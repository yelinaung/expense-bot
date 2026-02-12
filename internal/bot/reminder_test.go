package bot

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestCheckAndSendReminders(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 2, 11, 14, 30, 0, 0, loc)
	todayStr := now.Format("2006-01-02")

	t.Run("sends reminder when user has no expenses today", func(t *testing.T) {
		pool := TestDB(t)
		b := setupTestBot(t, pool)
		ctx := context.Background()
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.ReminderHour = 14
		b.cfg.WhitelistedUserIDs = []int64{2001}

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        2001,
			Username:  "noexpenses",
			FirstName: "Alice",
		})
		require.NoError(t, err)

		reminded := make(map[int64]string)
		b.checkAndSendReminders(ctx, loc, reminded, now)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Equal(t, int64(2001), msg.ChatID)
		require.Contains(t, msg.Text, "Hey Alice!")
		require.Contains(t, msg.Text, "5.50 Coffee")
		require.Equal(t, todayStr, reminded[2001])
	})

	t.Run("sends reminder to approved user", func(t *testing.T) {
		pool := TestDB(t)
		b := setupTestBot(t, pool)
		ctx := context.Background()
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.ReminderHour = 14
		b.cfg.WhitelistedUserIDs = nil // not a superadmin

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        2010,
			Username:  "approved",
			FirstName: "Frank",
		})
		require.NoError(t, err)

		err = b.approvedUserRepo.Approve(ctx, 2010, "approved", 1)
		require.NoError(t, err)

		reminded := make(map[int64]string)
		b.checkAndSendReminders(ctx, loc, reminded, now)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Equal(t, int64(2010), msg.ChatID)
	})

	t.Run("skips unapproved user", func(t *testing.T) {
		pool := TestDB(t)
		b := setupTestBot(t, pool)
		ctx := context.Background()
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.ReminderHour = 14
		b.cfg.WhitelistedUserIDs = nil

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        2011,
			Username:  "stranger",
			FirstName: "Ghost",
		})
		require.NoError(t, err)

		reminded := make(map[int64]string)
		b.checkAndSendReminders(ctx, loc, reminded, now)

		require.Equal(t, 0, mockBot.SentMessageCount(), "should not send reminder to unapproved user")
	})

	t.Run("skips user with expenses today", func(t *testing.T) {
		pool := TestDB(t)
		b := setupTestBot(t, pool)
		ctx := context.Background()
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.ReminderHour = 14
		b.cfg.WhitelistedUserIDs = []int64{2002}

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        2002,
			Username:  "hasexpenses",
			FirstName: "Bob",
		})
		require.NoError(t, err)

		expense := &models.Expense{
			UserID:      2002,
			Amount:      decimal.NewFromFloat(10.00),
			Currency:    "SGD",
			Description: "Lunch",
			Status:      models.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		reminded := make(map[int64]string)
		b.checkAndSendReminders(ctx, loc, reminded, now)

		require.Equal(t, 0, mockBot.SentMessageCount(), "should not send reminder to user with expenses")
	})

	t.Run("skips user already reminded today", func(t *testing.T) {
		pool := TestDB(t)
		b := setupTestBot(t, pool)
		ctx := context.Background()
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.ReminderHour = 14
		b.cfg.WhitelistedUserIDs = []int64{2003}

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        2003,
			Username:  "alreadyreminded",
			FirstName: "Charlie",
		})
		require.NoError(t, err)

		reminded := map[int64]string{
			2003: todayStr,
		}
		b.checkAndSendReminders(ctx, loc, reminded, now)

		require.Equal(t, 0, mockBot.SentMessageCount(), "should not send reminder to already reminded user")
	})

	t.Run("skips when current hour doesn't match ReminderHour", func(t *testing.T) {
		pool := TestDB(t)
		b := setupTestBot(t, pool)
		ctx := context.Background()
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.ReminderHour = 20 // now is hour 14, won't match
		b.cfg.WhitelistedUserIDs = []int64{2004}

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        2004,
			Username:  "wronghour",
			FirstName: "Diana",
		})
		require.NoError(t, err)

		reminded := make(map[int64]string)
		b.checkAndSendReminders(ctx, loc, reminded, now)

		require.Equal(t, 0, mockBot.SentMessageCount(), "should not send any reminders at wrong hour")
	})

	t.Run("handles send failure gracefully", func(t *testing.T) {
		pool := TestDB(t)
		b := setupTestBot(t, pool)
		ctx := context.Background()
		mockBot := mocks.NewMockBot()
		mockBot.SendMessageError = errors.New("user blocked bot")
		b.messageSender = mockBot
		b.cfg.ReminderHour = 14
		b.cfg.WhitelistedUserIDs = []int64{2005}

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        2005,
			Username:  "blockeduser",
			FirstName: "Eve",
		})
		require.NoError(t, err)

		reminded := make(map[int64]string)
		b.checkAndSendReminders(ctx, loc, reminded, now)

		_, exists := reminded[2005]
		require.False(t, exists, "should not mark as reminded on send failure")
	})

	t.Run("prunes stale entries from reminded map", func(t *testing.T) {
		pool := TestDB(t)
		b := setupTestBot(t, pool)
		ctx := context.Background()
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.ReminderHour = 14

		reminded := map[int64]string{
			9001: "2026-02-10", // yesterday
			9002: "2026-01-01", // last month
			9003: todayStr,     // today — should survive
		}

		b.checkAndSendReminders(ctx, loc, reminded, now)

		_, has9001 := reminded[9001]
		_, has9002 := reminded[9002]
		require.False(t, has9001, "yesterday's entry should be pruned")
		require.False(t, has9002, "old entry should be pruned")
		require.Equal(t, todayStr, reminded[9003], "today's entry should survive")
	})
}
