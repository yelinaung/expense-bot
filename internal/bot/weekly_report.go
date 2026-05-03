package bot

import (
	"context"
	"fmt"
	"time"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
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
		Int("day", b.cfg.WeeklyReportDay).
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

	// Run one check immediately.
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

	// Prune entries older than 2 weeks.
	cutoff := now.UTC().AddDate(0, 0, -14).Format("2006-01-02")
	for uid, weekKey := range sent {
		if weekKey < cutoff {
			delete(sent, uid)
		}
	}

	users, err := b.userRepo.GetAuthorizedUsersForReminder(
		checkCtx,
		b.cfg.WhitelistedUserIDs,
		b.cfg.WhitelistedUsernames,
	)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch users for weekly report")
		if b.metrics != nil {
			b.metrics.BackgroundJobRuns.Add(ctx, 1, otelmetric.WithAttributes(
				attribute.String("job", "weekly_report"),
				attribute.String("status", "error"),
			))
			b.metrics.BackgroundJobDuration.Record(ctx, time.Since(start).Seconds(),
				otelmetric.WithAttributes(attribute.String("job", "weekly_report")))
		}
		return
	}

	for i := range users {
		user := &users[i]

		loc := b.userLocation(user.Timezone)
		userNow := now.In(loc)

		if int(userNow.Weekday()) != b.cfg.WeeklyReportDay {
			continue
		}

		if userNow.Hour() != b.cfg.WeeklyReportHour {
			continue
		}

		// Build a key from the previous week's Monday date.
		prevStart, _ := getPreviousWeekRangeAt(userNow)
		weekKey := prevStart.Format("2006-01-02")
		if sent[user.ID] == weekKey {
			continue
		}

		err = b.sendWeeklySummary(checkCtx, user, userNow)
		if err != nil {
			logger.Log.Warn().Err(err).Str("user_hash", logger.HashUserID(user.ID)).Msg("Failed to send weekly report")
			continue
		}

		sent[user.ID] = weekKey
		logger.Log.Debug().Str("user_hash", logger.HashUserID(user.ID)).Str("timezone", loc.String()).Msg("Sent weekly report")
	}

	if b.metrics != nil {
		b.metrics.BackgroundJobRuns.Add(ctx, 1, otelmetric.WithAttributes(
			attribute.String("job", "weekly_report"),
			attribute.String("status", "ok"),
		))
		b.metrics.BackgroundJobDuration.Record(ctx, time.Since(start).Seconds(),
			otelmetric.WithAttributes(attribute.String("job", "weekly_report")))
	}
}

func (b *Bot) sendWeeklySummary(
	ctx context.Context,
	user *appmodels.User,
	userNow time.Time,
) error {
	startOfWeek, endOfWeek := getPreviousWeekRangeAt(userNow)

	expenses, err := b.expenseRepo.GetByUserIDAndDateRange(ctx, user.ID, startOfWeek, endOfWeek)
	if err != nil {
		return fmt.Errorf("failed to fetch weekly expenses: %w", err)
	}

	if len(expenses) == 0 {
		return nil // No expenses this week, skip silently.
	}

	total := sumExpenseAmounts(expenses)
	header := fmt.Sprintf("📊 <b>Weekly Expenses</b> (%s to %s)\nTotal: $%s | %d expenses",
		startOfWeek.Format("Jan 2"),
		endOfWeek.AddDate(0, 0, -1).Format("Jan 2, 2006"),
		total.StringFixed(2),
		len(expenses),
	)

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
		return fmt.Errorf("failed to send weekly summary: %w", err)
	}
	return nil
}
