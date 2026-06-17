package bot

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"

	"gitlab.com/yelinaung/expense-bot/internal/logger"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

const (
	// WeeklyReportCheckInterval is how often the weekly report loop runs.
	WeeklyReportCheckInterval = 30 * time.Minute
	// WeeklyReportTimeout is the maximum time a single check can take.
	WeeklyReportTimeout = 2 * time.Minute
)

// startWeeklyReportLoop runs a periodic loop that sends weekly expense
// summaries to users on the configured day and hour.
func (b *Bot) startWeeklyReportLoop(ctx context.Context) {
	if !b.cfg.WeeklyReportEnabled {
		logger.Log.Info().Msg("Weekly report is disabled")
		return
	}

	logger.Log.Info().
		Str("day", b.cfg.WeeklyReportDay.String()).
		Int("hour", b.cfg.WeeklyReportHour).
		Msg("Weekly report loop started (per-user timezone)")

	sent := make(map[int64]string)
	ticker := time.NewTicker(WeeklyReportCheckInterval)
	defer ticker.Stop()

	select {
	case <-ctx.Done():
		logger.Log.Info().Msg("Weekly report loop stopped")
		return
	default:
	}

	// Run one check immediately so reports aren't skipped when the
	// process starts during the configured window.
	b.checkAndSendWeeklyReports(ctx, sent, b.now())

	for {
		select {
		case <-ctx.Done():
			logger.Log.Info().Msg("Weekly report loop stopped")
			return
		case <-ticker.C:
			b.checkAndSendWeeklyReports(ctx, sent, b.now())
		}
	}
}

// checkAndSendWeeklyReports sends weekly summaries to authorized users
// whose local weekday and hour match the configured WeeklyReportDay and
// WeeklyReportHour.
func (b *Bot) checkAndSendWeeklyReports(ctx context.Context, sent map[int64]string, now time.Time) {
	ctx, span := otel.Tracer("expense-bot/background").Start(ctx, "background.weekly_report_check")
	defer span.End()
	start := time.Now()

	checkCtx, cancel := context.WithTimeout(ctx, WeeklyReportTimeout)
	defer cancel()

	pruneWeeklyReportSent(sent, now)

	users, err := b.userRepo.GetAuthorizedUsersForReminder(
		checkCtx,
		b.cfg.WhitelistedUserIDs,
		b.cfg.WhitelistedUsernames,
	)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch users for weekly report")
		b.recordWeeklyReportMetrics(ctx, start, "error")
		return
	}

	for i := range users {
		b.processWeeklyReportUser(checkCtx, &users[i], sent, now)
	}

	b.recordWeeklyReportMetrics(ctx, start, "ok")
}

// pruneWeeklyReportSent removes entries older than 2 weeks from the sent map.
func pruneWeeklyReportSent(sent map[int64]string, now time.Time) {
	cutoff := now.UTC().AddDate(0, 0, -14).Format("2006-01-02")
	for uid, weekKey := range sent {
		if weekKey < cutoff {
			delete(sent, uid)
		}
	}
}

// processWeeklyReportUser checks whether a user should receive a weekly
// report and sends one if needed.
func (b *Bot) processWeeklyReportUser(
	ctx context.Context,
	user *appmodels.User,
	sent map[int64]string,
	now time.Time,
) {
	loc := b.userLocation(user.Timezone)
	userNow := now.In(loc)

	if userNow.Weekday() != b.cfg.WeeklyReportDay {
		return
	}
	if userNow.Hour() != b.cfg.WeeklyReportHour {
		return
	}

	prevStart, _ := getPreviousWeekRangeAt(userNow)
	weekKey := prevStart.Format("2006-01-02")
	if sent[user.ID] == weekKey {
		return
	}

	sentOK, err := b.sendWeeklySummary(ctx, user, userNow)
	if err != nil {
		logger.Log.Warn().Err(err).
			Str("user_hash", logger.HashUserID(user.ID)).
			Msg("Failed to send weekly report")
		return
	}
	if !sentOK {
		logger.Log.Debug().
			Str("user_hash", logger.HashUserID(user.ID)).
			Msg("No weekly expenses; skipping report")
		return
	}

	sent[user.ID] = weekKey
	logger.Log.Debug().
		Str("user_hash", logger.HashUserID(user.ID)).
		Str("timezone", loc.String()).
		Msg("Sent weekly report")
}

// recordWeeklyReportMetrics records background job metrics for the
// weekly report run.
func (b *Bot) recordWeeklyReportMetrics(ctx context.Context, start time.Time, status string) {
	if b.metrics == nil {
		return
	}
	b.metrics.BackgroundJobRuns.Add(ctx, 1, otelmetric.WithAttributes(
		attribute.String("job", "weekly_report"),
		attribute.String("status", status),
	))
	b.metrics.BackgroundJobDuration.Record(ctx, time.Since(start).Seconds(),
		otelmetric.WithAttributes(attribute.String("job", "weekly_report")))
}

// sendWeeklySummary sends a weekly expense summary to the user.
// The returned bool indicates whether a message was actually sent.
// When there are no expenses, (false, nil) is returned.
func (b *Bot) sendWeeklySummary(
	ctx context.Context,
	user *appmodels.User,
	userNow time.Time,
) (bool, error) {
	startOfWeek, endOfWeek := getPreviousWeekRangeAt(userNow)

	expenses, err := b.expenseRepo.GetByUserIDAndDateRange(ctx, user.ID, startOfWeek, endOfWeek)
	if err != nil {
		return false, fmt.Errorf("failed to fetch weekly expenses: %w", err)
	}

	if len(expenses) == 0 {
		return false, nil
	}

	totalsByCurrency := sumExpenseAmountsByCurrency(expenses)
	currencies := sortedCurrencyKeys(totalsByCurrency)
	var sb strings.Builder
	fmt.Fprintf(
		&sb, "📊 <b>Weekly Expenses</b> (%s to %s)\n%d expenses",
		startOfWeek.Format("Jan 2"),
		endOfWeek.AddDate(0, 0, -1).Format("Jan 2, 2006"),
		len(expenses),
	)
	for _, cur := range currencies {
		fmt.Fprintf(&sb, "\n  %s: %s%s",
			escapeHTML(cur),
			escapeHTML(currencySymbol(cur)),
			totalsByCurrency[cur].StringFixed(2))
	}
	header := sb.String()

	expenseIDs := make([]int, len(expenses))
	for i := range expenses {
		expenseIDs[i] = expenses[i].ID
	}

	tagsByExpense := map[int][]appmodels.Tag{}
	if b.tagRepo != nil {
		var tagErr error
		tagsByExpense, tagErr = b.tagRepo.GetByExpenseIDs(ctx, expenseIDs)
		if tagErr != nil {
			logger.Log.Warn().Err(tagErr).Msg("Failed to batch-load tags for weekly summary")
		}
	}

	text := b.buildExpenseListMessage(header, expenses, tagsByExpense)
	_, err = b.messageSender.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    user.ID,
		Text:      text,
		ParseMode: tgmodels.ParseModeHTML,
	})
	if err != nil {
		return false, fmt.Errorf("failed to send weekly summary: %w", err)
	}
	return true, nil
}

// sumExpenseAmountsByCurrency returns expense totals grouped by currency.
func sumExpenseAmountsByCurrency(expenses []appmodels.Expense) map[string]decimal.Decimal {
	totals := make(map[string]decimal.Decimal)
	for i := range expenses {
		e := expenses[i]
		totals[e.Currency] = totals[e.Currency].Add(e.Amount)
	}
	return totals
}

// currencySymbol returns the display symbol for a currency code.
func currencySymbol(code string) string {
	if s := appmodels.SupportedCurrencies[code]; s != "" {
		return s
	}
	return code
}

// sortedCurrencyKeys returns the keys of a currency→amount map sorted
// alphabetically for deterministic output ordering.
func sortedCurrencyKeys(totals map[string]decimal.Decimal) []string {
	keys := make([]string, 0, len(totals))
	for k := range totals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
