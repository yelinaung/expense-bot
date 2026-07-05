package bot

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/models"
	"gitlab.com/yelinaung/expense-bot/internal/telemetry"
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
		b.cfg.WeeklyReportDay = time.Monday
		b.cfg.WeeklyReportHour = 9
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
		require.Contains(t, msg.Text, "SGD: S$31.50")
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
		b.cfg.WeeklyReportDay = time.Monday
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
		b.cfg.WeeklyReportDay = time.Monday
		b.cfg.WeeklyReportHour = 9
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
		b.cfg.WeeklyReportDay = time.Monday
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
		b.cfg.WeeklyReportDay = time.Monday
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
		b.cfg.WeeklyReportDay = time.Monday
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
		b.cfg.WeeklyReportDay = time.Monday
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

		// Create an expense in the previous week.
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
		// No message was actually sent because the mock returns an
		// error before recording. This distinguishes "attempted but
		// failed" from "skipped entirely."
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("prunes stale entries from sent map", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.WeeklyReportEnabled = true
		b.cfg.WeeklyReportDay = time.Monday
		b.cfg.WeeklyReportHour = 9
		b.cfg.WhitelistedUserIDs = []int64{4008}

		sent := map[int64]string{
			9001: "2026-04-01", // A month ago — should be pruned.
			9002: "2026-04-20", // Two weeks ago — should survive (cutoff is 14 days).
			9003: "2026-04-27", // Last week — should survive.
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
		b.cfg.WeeklyReportDay = time.Monday
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

		// Create expense in the previous week.
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
		b.cfg.WeeklyReportDay = time.Monday
		b.cfg.WeeklyReportHour = 9
		b.cfg.WhitelistedUserIDs = []int64{4101, 4102}

		// Etc/GMT-8 means UTC+8. At 01:00 UTC Monday it is
		// 09:00 local Monday — matches the configured hour.
		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        4101,
			Username:  "sguser",
			FirstName: "Sg",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 4101, "Etc/GMT-8")
		require.NoError(t, err)

		// Etc/GMT+5 means UTC-5. At 01:00 UTC Monday it is
		// 20:00 (8 PM) Sunday local — does not match.
		err = b.userRepo.UpsertUser(ctx, &models.User{
			ID:        4102,
			Username:  "westuser",
			FirstName: "West",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 4102, "Etc/GMT+5")
		require.NoError(t, err)

		// Create expense for both users in the previous week.
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
		b.cfg.WeeklyReportDay = time.Monday
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

	t.Run("sends habit recap after weekly summary when recap enabled", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.WeeklyReportEnabled = true
		b.cfg.WeeklyHabitRecapEnabled = true
		b.cfg.WeeklyReportDay = time.Monday
		b.cfg.WeeklyReportHour = 9
		b.cfg.WhitelistedUserIDs = []int64{4104}

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        4104,
			Username:  "recapuser",
			FirstName: "Rita",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 4104, "Etc/GMT-8")
		require.NoError(t, err)

		// Create a reviewed expense in the previous week (Apr 27 - May 3).
		prevMonday := time.Date(2026, 4, 27, 10, 0, 0, 0, loc)
		expense := &models.Expense{
			UserID:      4104,
			Amount:      decimal.NewFromFloat(12.00),
			Currency:    "SGD",
			Description: "Taxi",
			Status:      models.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)
		_, err = b.db.Exec(ctx, testUpdateExpenseTimeSQL, prevMonday, expense.ID)
		require.NoError(t, err)
		worthIt := true
		err = b.expenseRepo.UpdateReflection(ctx, expense.ID, 4104, &worthIt, "Convenience")
		require.NoError(t, err)

		sent := make(map[int64]string)
		b.checkAndSendWeeklyReports(ctx, sent, monday9amUTC)

		require.Equal(t, 2, mockBot.SentMessageCount(), "should send weekly summary and habit recap")
		require.Contains(t, mockBot.SentMessages[0].Text, "Weekly Expenses")
		recap := mockBot.SentMessages[1]
		require.Equal(t, int64(4104), recap.ChatID)
		require.Contains(t, recap.Text, "Spending Reflection")
		require.Contains(t, recap.Text, "Apr 27 to May 3, 2026")
		require.Contains(t, recap.Text, "Reviewed: 1/1")
		require.Contains(t, recap.Text, "Worth it: 1")
		require.Contains(t, recap.Text, "Convenience")
		require.Equal(t, "2026-04-27", sent[4104])
	})

	t.Run("skips habit recap when no reviewed expenses", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.WeeklyReportEnabled = true
		b.cfg.WeeklyHabitRecapEnabled = true
		b.cfg.WeeklyReportDay = time.Monday
		b.cfg.WeeklyReportHour = 9
		b.cfg.WhitelistedUserIDs = []int64{4105}

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        4105,
			Username:  "unreviewed",
			FirstName: "Uma",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 4105, "Etc/GMT-8")
		require.NoError(t, err)

		// Expense in the previous week, but never reviewed.
		prevMonday := time.Date(2026, 4, 27, 10, 0, 0, 0, loc)
		expense := &models.Expense{
			UserID:      4105,
			Amount:      decimal.NewFromFloat(8.00),
			Currency:    "SGD",
			Description: "Snack",
			Status:      models.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)
		_, err = b.db.Exec(ctx, testUpdateExpenseTimeSQL, prevMonday, expense.ID)
		require.NoError(t, err)

		sent := make(map[int64]string)
		b.checkAndSendWeeklyReports(ctx, sent, monday9amUTC)

		require.Equal(t, 1, mockBot.SentMessageCount(), "should send only the weekly summary")
		require.Contains(t, mockBot.LastSentMessage().Text, "Weekly Expenses")
		require.Equal(t, "2026-04-27", sent[4105])
	})

	t.Run("does not send habit recap when recap disabled", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.WeeklyReportEnabled = true
		b.cfg.WeeklyHabitRecapEnabled = false
		b.cfg.WeeklyReportDay = time.Monday
		b.cfg.WeeklyReportHour = 9
		b.cfg.WhitelistedUserIDs = []int64{4106}

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        4106,
			Username:  "recapoff",
			FirstName: "Omar",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 4106, "Etc/GMT-8")
		require.NoError(t, err)

		// A reviewed expense exists, but the recap flag is off.
		prevMonday := time.Date(2026, 4, 27, 10, 0, 0, 0, loc)
		expense := &models.Expense{
			UserID:      4106,
			Amount:      decimal.NewFromFloat(15.00),
			Currency:    "SGD",
			Description: "Book",
			Status:      models.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)
		_, err = b.db.Exec(ctx, testUpdateExpenseTimeSQL, prevMonday, expense.ID)
		require.NoError(t, err)
		worthIt := false
		err = b.expenseRepo.UpdateReflection(ctx, expense.ID, 4106, &worthIt, "Impulse")
		require.NoError(t, err)

		sent := make(map[int64]string)
		b.checkAndSendWeeklyReports(ctx, sent, monday9amUTC)

		require.Equal(t, 1, mockBot.SentMessageCount(), "should send only the weekly summary")
		require.Contains(t, mockBot.LastSentMessage().Text, "Weekly Expenses")
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

func TestStartWeeklyReportLoop_RunsImmediateCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)

	loc := time.FixedZone("GMT+8", 8*60*60)
	b.displayLocation = loc
	// Fixed time: Monday 9 AM GMT+8 = 01:00 UTC.
	fixedNow := time.Date(2026, 5, 4, 1, 0, 0, 0, time.UTC)
	b.nowFunc = func() time.Time { return fixedNow }

	mockBot := mocks.NewMockBot()
	b.messageSender = mockBot
	b.cfg.WeeklyReportEnabled = true
	b.cfg.WeeklyReportDay = time.Monday
	b.cfg.WeeklyReportHour = 9
	b.cfg.WhitelistedUserIDs = []int64{5001}

	err := b.userRepo.UpsertUser(ctx, &models.User{
		ID:        5001,
		Username:  "loopcheck",
		FirstName: "Nora",
	})
	require.NoError(t, err)
	err = b.userRepo.UpdateTimezone(ctx, 5001, "Etc/GMT-8")
	require.NoError(t, err)

	// Create an expense in the previous week (Apr 27 - May 3).
	prevMonday := time.Date(2026, 4, 27, 10, 0, 0, 0, loc)
	expense := &models.Expense{
		UserID:      5001,
		Amount:      decimal.NewFromFloat(10.00),
		Currency:    "SGD",
		Description: "Lunch",
		Status:      models.ExpenseStatusConfirmed,
	}
	err = b.expenseRepo.Create(ctx, expense)
	require.NoError(t, err)
	_, err = b.db.Exec(ctx, testUpdateExpenseTimeSQL, prevMonday, expense.ID)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		b.startWeeklyReportLoop(ctx)
	}()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if mockBot.SentMessageCount() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	<-done

	require.Equal(t, 1, mockBot.SentMessageCount(), "should send weekly report on immediate startup check")
	msg := mockBot.LastSentMessage()
	require.Contains(t, msg.Text, "Weekly Expenses")
}

func TestSendWeeklySummary_FetchError(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)

	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()

	sent, err := b.sendWeeklySummary(
		canceledCtx,
		&models.User{ID: 5002, FirstName: "Err"},
		time.Now(),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to fetch weekly expenses")
	require.False(t, sent)
}

func TestSendWeeklyHabitRecap_FetchError(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)

	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()

	sent, err := b.sendWeeklyHabitRecap(
		canceledCtx,
		&models.User{ID: 5004, FirstName: "Err"},
		time.Now(),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to fetch expenses for habit recap")
	require.False(t, sent)
}

// createReviewedExpense inserts a confirmed worth-it expense for the user,
// backdates it to createdAt, and marks it reviewed.
func createReviewedExpense(
	ctx context.Context,
	t *testing.T,
	b *Bot,
	userID int64,
	createdAt time.Time,
) {
	t.Helper()
	expense := &models.Expense{
		UserID:      userID,
		Amount:      decimal.NewFromFloat(9.50),
		Currency:    "SGD",
		Description: "Coffee",
		Status:      models.ExpenseStatusConfirmed,
	}
	err := b.expenseRepo.Create(ctx, expense)
	require.NoError(t, err)
	_, err = b.db.Exec(ctx, testUpdateExpenseTimeSQL, createdAt, expense.ID)
	require.NoError(t, err)
	worthIt := true
	err = b.expenseRepo.UpdateReflection(ctx, expense.ID, userID, &worthIt, "Ritual")
	require.NoError(t, err)
}

// habitRecapRunCount returns the recorded background.job.runs counter
// value for the weekly_habit_recap job with the given status attribute.
func habitRecapRunCount(rm metricdata.ResourceMetrics, status string) int64 {
	for i := range rm.ScopeMetrics {
		for j := range rm.ScopeMetrics[i].Metrics {
			metric := rm.ScopeMetrics[i].Metrics[j]
			if metric.Name != "background.job.runs" {
				continue
			}
			sum, ok := metric.Data.(metricdata.Sum[int64])
			if !ok {
				return 0
			}
			for _, dp := range sum.DataPoints {
				jobVal, _ := dp.Attributes.Value(attribute.Key("job"))
				statusVal, _ := dp.Attributes.Value(attribute.Key("status"))
				if jobVal.AsString() == "weekly_habit_recap" && statusVal.AsString() == status {
					return dp.Value
				}
			}
		}
	}
	return 0
}

func TestSendWeeklyHabitRecap_SendError(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)

	loc := time.FixedZone("GMT+8", 8*60*60)
	b.displayLocation = loc
	mockBot := mocks.NewMockBot()
	mockBot.SendMessageError = errors.New("user blocked bot")
	b.messageSender = mockBot

	err := b.userRepo.UpsertUser(ctx, &models.User{
		ID:        5005,
		Username:  "recapsenderr",
		FirstName: "Sena",
	})
	require.NoError(t, err)

	userNow := time.Date(2026, 5, 4, 9, 0, 0, 0, loc)
	createReviewedExpense(ctx, t, b, 5005, time.Date(2026, 4, 28, 10, 0, 0, 0, loc))

	sent, err := b.sendWeeklyHabitRecap(ctx, &models.User{ID: 5005}, userNow)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to send weekly habit recap")
	require.False(t, sent)
}

func TestSendWeeklyHabitRecapForUser_Metrics(t *testing.T) {
	loc := time.FixedZone("GMT+8", 8*60*60)
	userNow := time.Date(2026, 5, 4, 9, 0, 0, 0, loc)
	prevWeekDay := time.Date(2026, 4, 28, 10, 0, 0, 0, loc)

	setupMetrics := func(t *testing.T, b *Bot) *sdkmetric.ManualReader {
		t.Helper()
		reader := sdkmetric.NewManualReader()
		meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		otel.SetMeterProvider(meterProvider)
		t.Cleanup(func() {
			otel.SetMeterProvider(noop.NewMeterProvider())
			_ = meterProvider.Shutdown(context.Background())
		})
		metrics, err := telemetry.NewBotMetrics()
		require.NoError(t, err)
		b.metrics = metrics
		return reader
	}

	t.Run("records ok metric on successful recap", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		reader := setupMetrics(t, b)

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        5006,
			Username:  "recapmetricok",
			FirstName: "Mel",
		})
		require.NoError(t, err)
		createReviewedExpense(ctx, t, b, 5006, prevWeekDay)

		b.sendWeeklyHabitRecapForUser(ctx, &models.User{ID: 5006}, userNow)

		require.Equal(t, 1, mockBot.SentMessageCount())
		var rm metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(ctx, &rm))
		require.Equal(t, int64(1), habitRecapRunCount(rm, "ok"))
	})

	t.Run("records error metric on send failure", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		mockBot.SendMessageError = errors.New("user blocked bot")
		b.messageSender = mockBot
		reader := setupMetrics(t, b)

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        5007,
			Username:  "recapmetricerr",
			FirstName: "Mia",
		})
		require.NoError(t, err)
		createReviewedExpense(ctx, t, b, 5007, prevWeekDay)

		b.sendWeeklyHabitRecapForUser(ctx, &models.User{ID: 5007}, userNow)

		require.Equal(t, 0, mockBot.SentMessageCount())
		var rm metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(ctx, &rm))
		require.Equal(t, int64(1), habitRecapRunCount(rm, "error"))
	})

	t.Run("records no metric when nothing to send", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		reader := setupMetrics(t, b)

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        5008,
			Username:  "recapmetricskip",
			FirstName: "Mo",
		})
		require.NoError(t, err)

		b.sendWeeklyHabitRecapForUser(ctx, &models.User{ID: 5008}, userNow)

		require.Equal(t, 0, mockBot.SentMessageCount())
		var rm metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(ctx, &rm))
		require.Equal(t, int64(0), habitRecapRunCount(rm, "ok"))
		require.Equal(t, int64(0), habitRecapRunCount(rm, "error"))
	})
}

func TestCheckAndSendWeeklyReports_FetchUsersError(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)
	b.cfg.WeeklyReportEnabled = true
	b.cfg.WeeklyReportDay = time.Monday
	b.cfg.WeeklyReportHour = 9
	b.cfg.WhitelistedUserIDs = []int64{5003}

	loc := time.FixedZone("GMT+8", 8*60*60)
	monday9amUTC := time.Date(2026, 5, 4, 1, 0, 0, 0, time.UTC)
	b.displayLocation = loc

	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()

	sent := make(map[int64]string)
	// Should not panic when fetching users fails.
	b.checkAndSendWeeklyReports(canceledCtx, sent, monday9amUTC)
	require.Empty(t, sent)
}
