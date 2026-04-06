package bot

import (
	"context"
	"fmt"
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
	// ReminderCheckInterval is how often the reminder loop checks whether to send reminders.
	ReminderCheckInterval = 30 * time.Minute
	// ReminderTimeout is the maximum time a single reminder check can take.
	ReminderTimeout = 2 * time.Minute
)

// startDailyReminderLoop runs a periodic loop that sends daily reminders to users
// who haven't logged any expenses for the current day.
func (b *Bot) startDailyReminderLoop(ctx context.Context) {
	if !b.cfg.DailyReminderEnabled {
		logger.Log.Info().Msg("Daily reminder is disabled")
		return
	}

	logger.Log.Info().
		Int("hour", b.cfg.ReminderHour).
		Msg("Daily reminder loop started (per-user timezone)")

	reminded := make(map[int64]string)
	ticker := time.NewTicker(ReminderCheckInterval)
	defer ticker.Stop()

	select {
	case <-ctx.Done():
		logger.Log.Info().Msg("Daily reminder loop stopped")
		return
	default:
	}

	// Run one check immediately so reminders aren't skipped when the process
	// starts during the configured reminder hour.
	b.checkAndSendReminders(ctx, reminded, b.now())

	for {
		select {
		case <-ctx.Done():
			logger.Log.Info().Msg("Daily reminder loop stopped")
			return
		case <-ticker.C:
			b.checkAndSendReminders(ctx, reminded, b.now())
		}
	}
}

// checkAndSendReminders sends reminders to authorized users whose local hour
// matches ReminderHour. Each user's timezone is read from their profile;
// the global displayLocation is used as fallback.
func (b *Bot) checkAndSendReminders(ctx context.Context, reminded map[int64]string, now time.Time) {
	ctx, span := otel.Tracer("expense-bot/background").Start(ctx, "background.reminder_check")
	defer span.End()
	start := time.Now()

	checkCtx, cancel := context.WithTimeout(ctx, ReminderTimeout)
	defer cancel()

	// Prune entries older than yesterday (safe for all timezone offsets).
	cutoff := now.UTC().AddDate(0, 0, -1).Format("2006-01-02")
	for uid, dateStr := range reminded {
		if dateStr < cutoff {
			delete(reminded, uid)
		}
	}

	users, err := b.userRepo.GetAuthorizedUsersForReminder(
		checkCtx,
		b.cfg.WhitelistedUserIDs,
		b.cfg.WhitelistedUsernames,
	)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch users for daily reminder")
		if b.metrics != nil {
			b.metrics.BackgroundJobRuns.Add(ctx, 1, otelmetric.WithAttributes(attribute.String("job", "reminder"), attribute.String("status", "error")))
			b.metrics.BackgroundJobDuration.Record(ctx, time.Since(start).Seconds(), otelmetric.WithAttributes(attribute.String("job", "reminder")))
		}
		return
	}

	for i := range users {
		user := &users[i]

		loc := b.userLocation(user.Timezone)
		userNow := now.In(loc)

		if userNow.Hour() != b.cfg.ReminderHour {
			continue
		}

		todayStr := userNow.Format("2006-01-02")
		if reminded[user.ID] == todayStr {
			continue
		}

		startOfDay := time.Date(userNow.Year(), userNow.Month(), userNow.Day(), 0, 0, 0, 0, loc)
		endOfDay := startOfDay.AddDate(0, 0, 1)

		err = b.sendReminderOrDailySummary(checkCtx, user, startOfDay, endOfDay)
		if err != nil {
			logger.Log.Warn().Err(err).Str("user_hash", logger.HashUserID(user.ID)).Msg("Failed to send daily reminder")
			continue
		}

		reminded[user.ID] = todayStr
		logger.Log.Debug().Str("user_hash", logger.HashUserID(user.ID)).Str("timezone", loc.String()).Msg("Sent daily reminder")
	}

	if b.metrics != nil {
		b.metrics.BackgroundJobRuns.Add(ctx, 1, otelmetric.WithAttributes(attribute.String("job", "reminder"), attribute.String("status", "ok")))
		b.metrics.BackgroundJobDuration.Record(ctx, time.Since(start).Seconds(), otelmetric.WithAttributes(attribute.String("job", "reminder")))
	}
}

// userLocation resolves a user's timezone string to a *time.Location,
// falling back to the bot's global displayLocation on error.
func (b *Bot) userLocation(tz string) *time.Location {
	if tz == "" {
		return b.displayLocation
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		logger.Log.Warn().Err(err).Str("timezone", tz).Msg("Invalid user timezone, using global default")
		return b.displayLocation
	}
	return loc
}

func (b *Bot) sendReminderOrDailySummary(
	ctx context.Context,
	user *appmodels.User,
	startOfDay, endOfDay time.Time,
) error {
	expenses, err := b.expenseRepo.GetByUserIDAndDateRange(ctx, user.ID, startOfDay, endOfDay)
	if err != nil {
		return fmt.Errorf("failed to fetch today's expenses: %w", err)
	}

	if len(expenses) == 0 {
		return b.sendNoExpenseReminder(ctx, user)
	}

	total := sumExpenseAmounts(expenses)
	header := fmt.Sprintf("\U0001f4c5 <b>Today's Expenses</b> (Total: $%s)", total.StringFixed(2))
	return b.sendTodaySummary(ctx, user.ID, expenses, header)
}

func (b *Bot) sendNoExpenseReminder(ctx context.Context, user *appmodels.User) error {
	firstName := user.FirstName
	if firstName == "" {
		firstName = "there"
	}

	text := fmt.Sprintf(
		"Hey %s! You haven't recorded any expenses today. Don't forget to track your spending!\n\nSend an expense like `5.50 Coffee` to get started.",
		firstName,
	)

	_, err := b.messageSender.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: user.ID,
		Text:   text,
	})
	if err != nil {
		return fmt.Errorf("failed to send no-expense reminder: %w", err)
	}
	return nil
}

func (b *Bot) sendTodaySummary(
	ctx context.Context,
	userID int64,
	expenses []appmodels.Expense,
	header string,
) error {
	expenseIDs := make([]int, len(expenses))
	for i := range expenses {
		expenseIDs[i] = expenses[i].ID
	}

	tagsByExpense := map[int][]appmodels.Tag{}
	if b.tagRepo != nil {
		var err error
		tagsByExpense, err = b.tagRepo.GetByExpenseIDs(ctx, expenseIDs)
		if err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to batch-load tags for daily summary")
		}
	}

	text := b.buildExpenseListMessage(header, expenses, tagsByExpense)
	_, err := b.messageSender.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    userID,
		Text:      text,
		ParseMode: tgmodels.ParseModeHTML,
	})
	if err != nil {
		return fmt.Errorf("failed to send daily summary: %w", err)
	}
	return nil
}

func sumExpenseAmounts(expenses []appmodels.Expense) decimal.Decimal {
	total := decimal.Zero
	for i := range expenses {
		total = total.Add(expenses[i].Amount)
	}
	return total
}
