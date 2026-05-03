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

func TestCheckAndSendWeeklyReports(t *testing.T) {
	loc := time.FixedZone("GMT+8", 8*60*60)
	// 2026-05-04 is a Monday. 09:00 GMT+8 = 01:00 UTC.
	monday9amUTC := time.Date(2026, 5, 4, 1, 0, 0, 0, time.UTC)

	t.Run("sends weekly summary on Monday at configured hour", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.WeeklyReportEnabled = true
		b.cfg.WeeklyReportDay = 1  // Monday
		b.cfg.WeeklyReportHour = 9 // 9 AM
		b.cfg.WhitelistedUserIDs = []int64{4001}

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        4001,
			Username:  "weeklyuser",
			FirstName: "Alice",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 4001, "Etc/GMT-8")
		require.NoError(t, err)

		// Create expenses in the previous week (Apr 27 - May 3).
		prevMonday := time.Date(2026, 4, 27, 10, 0, 0, 0, loc)
		for i := range 3 {
			expense := &models.Expense{
				UserID:      4001,
				Amount:      decimal.NewFromFloat(10.50),
				Currency:    "SGD",
				Description: "Lunch",
				Status:      models.ExpenseStatusConfirmed,
			}
			err = b.expenseRepo.Create(ctx, expense)
			require.NoError(t, err)
			_, err = b.db.Exec(ctx, testUpdateExpenseTimeSQL,
				prevMonday.Add(time.Duration(i)*24*time.Hour), expense.ID)
			require.NoError(t, err)
		}

		sent := make(map[int64]string)
		b.checkAndSendWeeklyReports(ctx, sent, monday9amUTC)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Equal(t, int64(4001), msg.ChatID)
		require.Contains(t, msg.Text, "Weekly Expenses")
		require.Contains(t, msg.Text, "Apr 27")
		require.Contains(t, msg.Text, "May 3, 2026")
		require.Contains(t, msg.Text, "Lunch")
		require.Equal(t, "2026-04-27", sent[4001])
	})

	t.Run("skips when user has no expenses in previous week", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.WeeklyReportEnabled = true
		b.cfg.WeeklyReportDay = 1
		b.cfg.WeeklyReportHour = 9
		b.cfg.WhitelistedUserIDs = []int64{4002}

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        4002,
			Username:  "emptyweek",
			FirstName: "Bob",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 4002, "Etc/GMT-8")
		require.NoError(t, err)

		sent := make(map[int64]string)
		b.checkAndSendWeeklyReports(ctx, sent, monday9amUTC)

		require.Equal(t, 0, mockBot.SentMessageCount(), "should skip user with no expenses")
	})

	t.Run("skips when weekday does not match", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.WeeklyReportEnabled = true
		b.cfg.WeeklyReportDay = 1  // Monday
		b.cfg.WeeklyReportHour = 9 // 9 AM
		b.cfg.WhitelistedUserIDs = []int64{4003}

		tuesdayUTC := time.Date(2026, 5, 5, 1, 0, 0, 0, time.UTC) // Tuesday

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        4003,
			Username:  "wrongday",
			FirstName: "Charlie",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 4003, "Etc/GMT-8")
		require.NoError(t, err)

		sent := make(map[int64]string)
		b.checkAndSendWeeklyReports(ctx, sent, tuesdayUTC)

		require.Equal(t, 0, mockBot.SentMessageCount(), "should skip on Tuesday")
	})

	t.Run("skips when hour does not match", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.WeeklyReportEnabled = true
		b.cfg.WeeklyReportDay = 1
		b.cfg.WeeklyReportHour = 9
		b.cfg.WhitelistedUserIDs = []int64{4004}

		wrongHourUTC := time.Date(2026, 5, 4, 2, 0, 0, 0, time.UTC) // Monday 10 AM GMT+8

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        4004,
			Username:  "wronghour",
			FirstName: "Diana",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 4004, "Etc/GMT-8")
		require.NoError(t, err)

		sent := make(map[int64]string)
		b.checkAndSendWeeklyReports(ctx, sent, wrongHourUTC)

		require.Equal(t, 0, mockBot.SentMessageCount(), "should skip at wrong hour")
	})

	t.Run("skips user already sent for this week", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.WeeklyReportEnabled = true
		b.cfg.WeeklyReportDay = 1
		b.cfg.WeeklyReportHour = 9
		b.cfg.WhitelistedUserIDs = []int64{4005}

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        4005,
			Username:  "alreadysent",
			FirstName: "Eve",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 4005, "Etc/GMT-8")
		require.NoError(t, err)

		sent := map[int64]string{
			4005: "2026-04-27",
		}
		b.checkAndSendWeeklyReports(ctx, sent, monday9amUTC)

		require.Equal(t, 0, mockBot.SentMessageCount(), "should skip already sent user")
	})

	t.Run("skips unapproved user", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.WeeklyReportEnabled = true
		b.cfg.WeeklyReportDay = 1
		b.cfg.WeeklyReportHour = 9
		b.cfg.WhitelistedUserIDs = nil

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        4006,
			Username:  "stranger",
			FirstName: "Frank",
		})
		require.NoError(t, err)

		sent := make(map[int64]string)
		b.checkAndSendWeeklyReports(ctx, sent, monday9amUTC)

		require.Equal(t, 0, mockBot.SentMessageCount(), "should skip unapproved user")
	})

	t.Run("handles send failure gracefully", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		mockBot.SendMessageError = errors.New("user blocked bot")
		b.messageSender = mockBot
		b.cfg.WeeklyReportEnabled = true
		b.cfg.WeeklyReportDay = 1
		b.cfg.WeeklyReportHour = 9
		b.cfg.WhitelistedUserIDs = []int64{4007}

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        4007,
			Username:  "blockeduser",
			FirstName: "Grace",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 4007, "Etc/GMT-8")
		require.NoError(t, err)

		// Create an expense in previous week.
		prevMonday := time.Date(2026, 4, 27, 10, 0, 0, 0, loc)
		expense := &models.Expense{
			UserID:      4007,
			Amount:      decimal.NewFromFloat(5.00),
			Currency:    "SGD",
			Description: "Coffee",
			Status:      models.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)
		_, err = b.db.Exec(ctx, testUpdateExpenseTimeSQL, prevMonday, expense.ID)
		require.NoError(t, err)

		sent := make(map[int64]string)
		b.checkAndSendWeeklyReports(ctx, sent, monday9amUTC)

		_, exists := sent[4007]
		require.False(t, exists, "should not mark as sent on failure")
	})

	t.Run("prunes stale entries from sent map", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.WeeklyReportEnabled = true
		b.cfg.WeeklyReportDay = 1
		b.cfg.WeeklyReportHour = 9
		b.cfg.WhitelistedUserIDs = []int64{4008}

		sent := map[int64]string{
			9001: "2026-04-01", // month ago — should be pruned
			9002: "2026-04-20", // 2 weeks ago — should survive (cutoff is 14 days)
			9003: "2026-04-27", // last week — should survive
		}

		b.checkAndSendWeeklyReports(ctx, sent, monday9amUTC)

		_, has9001 := sent[9001]
		require.False(t, has9001, "old entry should be pruned")
		require.Equal(t, "2026-04-20", sent[9002], "entry within 14 days should survive")
		require.Equal(t, "2026-04-27", sent[9003], "last week's entry should survive")
	})

	t.Run("falls back to displayLocation when user timezone is empty", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc // GMT+8
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.WeeklyReportEnabled = true
		b.cfg.WeeklyReportDay = 1
		b.cfg.WeeklyReportHour = 9
		b.cfg.WhitelistedUserIDs = []int64{4009}

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        4009,
			Username:  "defaulttz",
			FirstName: "Hank",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 4009, "")
		require.NoError(t, err)

		// Create expense in previous week.
		prevMonday := time.Date(2026, 4, 27, 10, 0, 0, 0, loc)
		expense := &models.Expense{
			UserID:      4009,
			Amount:      decimal.NewFromFloat(5.00),
			Currency:    "SGD",
			Description: "Coffee",
			Status:      models.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)
		_, err = b.db.Exec(ctx, testUpdateExpenseTimeSQL, prevMonday, expense.ID)
		require.NoError(t, err)

		sent := make(map[int64]string)
		b.checkAndSendWeeklyReports(ctx, sent, monday9amUTC)

		require.Equal(t, 1, mockBot.SentMessageCount(), "should fall back to displayLocation and send report")
	})

	t.Run("per-user timezone: only matching user receives report", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = time.UTC
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.WeeklyReportEnabled = true
		b.cfg.WeeklyReportDay = 1  // Monday
		b.cfg.WeeklyReportHour = 9 // 9 AM
		b.cfg.WhitelistedUserIDs = []int64{4101, 4102}

		// User in GMT+8: at 01:00 UTC Monday = 09:00 local → matches
		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        4101,
			Username:  "sguser",
			FirstName: "Sg",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 4101, "Etc/GMT-8")
		require.NoError(t, err)

		// User in GMT+5: at 01:00 UTC Monday = 06:00 local → doesn't match
		err = b.userRepo.UpsertUser(ctx, &models.User{
			ID:        4102,
			Username:  "eastuser",
			FirstName: "East",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 4102, "Etc/GMT+5")
		require.NoError(t, err)

		// Create expense for both users in previous week.
		prevMonday := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
		for _, uid := range []int64{4101, 4102} {
			expense := &models.Expense{
				UserID:      uid,
				Amount:      decimal.NewFromFloat(10.00),
				Currency:    "SGD",
				Description: "Lunch",
				Status:      models.ExpenseStatusConfirmed,
			}
			err = b.expenseRepo.Create(ctx, expense)
			require.NoError(t, err)
			_, err = b.db.Exec(ctx, testUpdateExpenseTimeSQL, prevMonday, expense.ID)
			require.NoError(t, err)
		}

		sent := make(map[int64]string)
		b.checkAndSendWeeklyReports(ctx, sent, monday9amUTC)

		require.Equal(t, 1, mockBot.SentMessageCount(), "only GMT+8 user should receive report")
		msg := mockBot.LastSentMessage()
		require.Equal(t, int64(4101), msg.ChatID)
	})

	t.Run("sends summary even when tag repository is nil", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.tagRepo = nil
		b.cfg.WeeklyReportEnabled = true
		b.cfg.WeeklyReportDay = 1
		b.cfg.WeeklyReportHour = 9
		b.cfg.WhitelistedUserIDs = []int64{4103}

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        4103,
			Username:  "niltagrepo",
			FirstName: "Nina",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 4103, "Etc/GMT-8")
		require.NoError(t, err)

		prevMonday := time.Date(2026, 4, 27, 10, 0, 0, 0, loc)
		expense := &models.Expense{
			UserID:      4103,
			Amount:      decimal.NewFromFloat(7.25),
			Currency:    "SGD",
			Description: "Tea",
			Status:      models.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)
		_, err = b.db.Exec(ctx, testUpdateExpenseTimeSQL, prevMonday, expense.ID)
		require.NoError(t, err)

		sent := make(map[int64]string)
		b.checkAndSendWeeklyReports(ctx, sent, monday9amUTC)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Weekly Expenses")
		require.Contains(t, msg.Text, "Tea")
	})
}

func TestGetPreviousWeekRangeAt(t *testing.T) {
	t.Parallel()

	loc := time.UTC

	t.Run("Monday returns previous week", func(t *testing.T) {
		t.Parallel()
		monday := time.Date(2026, 5, 4, 10, 0, 0, 0, loc)
		start, end := getPreviousWeekRangeAt(monday)

		require.Equal(t, "2026-04-27", start.Format("2006-01-02"))
		require.Equal(t, "2026-05-04", end.Format("2006-01-02"))
	})

	t.Run("Wednesday returns same previous week as Monday", func(t *testing.T) {
		t.Parallel()
		wednesday := time.Date(2026, 5, 6, 10, 0, 0, 0, loc)
		monday := time.Date(2026, 5, 4, 10, 0, 0, 0, loc)

		wStart, wEnd := getPreviousWeekRangeAt(wednesday)
		mStart, mEnd := getPreviousWeekRangeAt(monday)

		require.Equal(t, mStart, wStart)
		require.Equal(t, mEnd, wEnd)
	})
}
