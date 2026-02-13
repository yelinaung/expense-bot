package bot

import (
	"context"
	"fmt"
	"time"

	tgbot "github.com/go-telegram/bot"
	"gitlab.com/yelinaung/expense-bot/internal/logger"
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

	loc, err := time.LoadLocation(b.cfg.ReminderTimezone)
	if err != nil {
		logger.Log.Error().Err(err).Str("timezone", b.cfg.ReminderTimezone).Msg("Failed to load reminder timezone, disabling reminders")
		return
	}

	logger.Log.Info().
		Int("hour", b.cfg.ReminderHour).
		Str("timezone", b.cfg.ReminderTimezone).
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
	b.checkAndSendReminders(ctx, loc, reminded, time.Now().In(loc))

	for {
		select {
		case <-ctx.Done():
			logger.Log.Info().Msg("Daily reminder loop stopped")
			return
		case <-ticker.C:
			b.checkAndSendReminders(ctx, loc, reminded, time.Now().In(loc))
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

	users, err := b.userRepo.GetUsersNeedingReminder(
		checkCtx,
		b.cfg.WhitelistedUserIDs,
		b.cfg.WhitelistedUsernames,
		startOfDay, endOfDay,
	)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch users for daily reminder")
		return
	}

	for _, user := range users {
		if reminded[user.ID] == todayStr {
			continue
		}

		firstName := user.FirstName
		if firstName == "" {
			firstName = "there"
		}

		text := fmt.Sprintf(
			"Hey %s! You haven't recorded any expenses today. Don't forget to track your spending!\n\nSend an expense like `5.50 Coffee` to get started.",
			firstName,
		)

		_, err = b.messageSender.SendMessage(checkCtx, &tgbot.SendMessageParams{
			ChatID: user.ID,
			Text:   text,
		})
		if err != nil {
			logger.Log.Warn().Err(err).Str("user_hash", logger.HashUserID(user.ID)).Msg("Failed to send daily reminder")
			continue
		}

		reminded[user.ID] = todayStr
		logger.Log.Debug().Str("user_hash", logger.HashUserID(user.ID)).Msg("Sent daily reminder")
	}
}
