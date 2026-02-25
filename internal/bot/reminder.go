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
)

const (
	// ReminderCheckInterval is how often the reminder loop checks whether to send reminders.
	ReminderCheckInterval = 30 * time.Minute
	// ReminderTimeout is the maximum time a single reminder check can take.
	ReminderTimeout = 2 * time.Minute
)

var reminderLocationGMTPlus8 = time.FixedZone("GMT+8", 8*60*60)

// startDailyReminderLoop runs a periodic loop that sends daily reminders to users
// who haven't logged any expenses for the current day.
func (b *Bot) startDailyReminderLoop(ctx context.Context) {
	if !b.cfg.DailyReminderEnabled {
		logger.Log.Info().Msg("Daily reminder is disabled")
		return
	}

	loc := reminderLocationGMTPlus8

	logger.Log.Info().
		Int("hour", b.cfg.ReminderHour).
		Str("timezone", "GMT+8").
		Msg("Daily reminder loop started")

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
	b.checkAndSendReminders(ctx, loc, reminded, b.now().In(loc))

	for {
		select {
		case <-ctx.Done():
			logger.Log.Info().Msg("Daily reminder loop stopped")
			return
		case <-ticker.C:
			b.checkAndSendReminders(ctx, loc, reminded, b.now().In(loc))
		}
	}
}

// checkAndSendReminders checks the current hour and sends reminders to users
// who haven't logged expenses today. The reminded map tracks which users have
// already been reminded today to avoid duplicate notifications.
func (b *Bot) checkAndSendReminders(ctx context.Context, loc *time.Location, reminded map[int64]string, now time.Time) {
	if now.Hour() != b.cfg.ReminderHour {
		return
	}

	checkCtx, cancel := context.WithTimeout(ctx, ReminderTimeout)
	defer cancel()

	todayStr := now.Format("2006-01-02")

	// Prune entries from previous days so the map doesn't grow unbounded.
	for uid, dateStr := range reminded {
		if dateStr != todayStr {
			delete(reminded, uid)
		}
	}

	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	endOfDay := startOfDay.AddDate(0, 0, 1)

	users, err := b.userRepo.GetAuthorizedUsersForReminder(
		checkCtx,
		b.cfg.WhitelistedUserIDs,
		b.cfg.WhitelistedUsernames,
	)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch users for daily reminder")
		return
	}

	for i := range users {
		user := &users[i]
		if reminded[user.ID] == todayStr {
			continue
		}

		err = b.sendReminderOrDailySummary(checkCtx, user, startOfDay, endOfDay)
		if err != nil {
			logger.Log.Warn().Err(err).Str("user_hash", logger.HashUserID(user.ID)).Msg("Failed to send daily reminder")
			continue
		}

		reminded[user.ID] = todayStr
		logger.Log.Debug().Str("user_hash", logger.HashUserID(user.ID)).Msg("Sent daily reminder")
	}
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
	header := fmt.Sprintf("📅 <b>Today's Expenses</b> (Total: $%s)", total.StringFixed(2))
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
