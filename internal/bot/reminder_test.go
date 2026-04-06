package bot

import (
	"context"
	"errors"
	"testing"
	"time"

	tgmodels "github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestCheckAndSendReminders(t *testing.T) {
	loc := time.FixedZone("GMT+8", 8*60*60)
	// 14:30 in GMT+8 = 06:30 UTC
	nowUTC := time.Date(2026, 2, 11, 6, 30, 0, 0, time.UTC)
	todayStr := nowUTC.In(loc).Format("2006-01-02")

	t.Run("sends reminder when user has no expenses today", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
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
		err = b.userRepo.UpdateTimezone(ctx, 2001, "Etc/GMT-8")
		require.NoError(t, err)

		reminded := make(map[int64]string)
		b.checkAndSendReminders(ctx, reminded, nowUTC)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Equal(t, int64(2001), msg.ChatID)
		require.Contains(t, msg.Text, "Hey Alice!")
		require.Contains(t, msg.Text, "5.50 Coffee")
		require.Equal(t, todayStr, reminded[2001])
	})

	t.Run("sends reminder to approved user", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
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
		err = b.userRepo.UpdateTimezone(ctx, 2010, "Etc/GMT-8")
		require.NoError(t, err)

		err = b.approvedUserRepo.Approve(ctx, 2010, "approved", 1)
		require.NoError(t, err)

		reminded := make(map[int64]string)
		b.checkAndSendReminders(ctx, reminded, nowUTC)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Equal(t, int64(2010), msg.ChatID)
	})

	t.Run("skips unapproved user", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
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
		b.checkAndSendReminders(ctx, reminded, nowUTC)

		require.Equal(t, 0, mockBot.SentMessageCount(), "should not send reminder to unapproved user")
	})

	t.Run("sends summary to user with expenses today", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
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
		err = b.userRepo.UpdateTimezone(ctx, 2002, "Etc/GMT-8")
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
		// Set created_at to 14:30 GMT+8 = 06:30 UTC
		_, err = b.db.Exec(ctx, `UPDATE expenses SET created_at = $2 WHERE id = $1`, expense.ID, nowUTC)
		require.NoError(t, err)

		reminded := make(map[int64]string)
		b.checkAndSendReminders(ctx, reminded, nowUTC)

		require.Equal(t, 1, mockBot.SentMessageCount(), "should send daily summary to user with expenses")
		msg := mockBot.LastSentMessage()
		require.Equal(t, int64(2002), msg.ChatID)
		require.Contains(t, msg.Text, testTodayExpensesText)
		require.Contains(t, msg.Text, "Lunch")
		require.Equal(t, tgmodels.ParseModeHTML, msg.ParseMode)
		require.Equal(t, todayStr, reminded[2002])
	})

	t.Run("uses per-user timezone day window when deciding summary", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.ReminderHour = 0
		b.cfg.WhitelistedUserIDs = []int64{2012}

		// 00:30 GMT+8 = 16:30 UTC previous day
		nowAtBoundaryUTC := time.Date(2026, 2, 10, 16, 30, 0, 0, time.UTC)

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        2012,
			Username:  "gmtplus8user",
			FirstName: "Grace",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 2012, "Etc/GMT-8")
		require.NoError(t, err)

		expense := &models.Expense{
			UserID:      2012,
			Amount:      decimal.NewFromFloat(12.00),
			Currency:    "SGD",
			Description: "Breakfast",
			Status:      models.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		createdAtUTC := time.Date(2026, 2, 10, 16, 45, 0, 0, time.UTC) // 2026-02-11 00:45 GMT+8
		_, err = b.db.Exec(ctx, `UPDATE expenses SET created_at = $2 WHERE id = $1`, expense.ID, createdAtUTC)
		require.NoError(t, err)

		reminded := make(map[int64]string)
		b.checkAndSendReminders(ctx, reminded, nowAtBoundaryUTC)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, testTodayExpensesText)
		require.Contains(t, msg.Text, "Breakfast")
		expectedDate := nowAtBoundaryUTC.In(loc).Format("2006-01-02")
		require.Equal(t, expectedDate, reminded[2012])
	})

	t.Run("sends summary even when tag repository is nil", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.tagRepo = nil
		b.cfg.ReminderHour = 14
		b.cfg.WhitelistedUserIDs = []int64{2013}

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        2013,
			Username:  "niltagrepo",
			FirstName: "Nina",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 2013, "Etc/GMT-8")
		require.NoError(t, err)

		expense := &models.Expense{
			UserID:      2013,
			Amount:      decimal.NewFromFloat(7.25),
			Currency:    "SGD",
			Description: "Tea",
			Status:      models.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)
		_, err = b.db.Exec(ctx, `UPDATE expenses SET created_at = $2 WHERE id = $1`, expense.ID, nowUTC)
		require.NoError(t, err)

		reminded := make(map[int64]string)
		b.checkAndSendReminders(ctx, reminded, nowUTC)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, testTodayExpensesText)
		require.Contains(t, msg.Text, "Tea")
		require.Equal(t, todayStr, reminded[2013])
	})

	t.Run("skips user already reminded today", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
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
		err = b.userRepo.UpdateTimezone(ctx, 2003, "Etc/GMT-8")
		require.NoError(t, err)

		reminded := map[int64]string{
			2003: todayStr,
		}
		b.checkAndSendReminders(ctx, reminded, nowUTC)

		require.Equal(t, 0, mockBot.SentMessageCount(), "should not send reminder to already reminded user")
	})

	t.Run("skips when user local hour doesn't match ReminderHour", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.ReminderHour = 20 // user's local hour is 14, won't match
		b.cfg.WhitelistedUserIDs = []int64{2004}

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        2004,
			Username:  "wronghour",
			FirstName: "Diana",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 2004, "Etc/GMT-8")
		require.NoError(t, err)

		reminded := make(map[int64]string)
		b.checkAndSendReminders(ctx, reminded, nowUTC)

		require.Equal(t, 0, mockBot.SentMessageCount(), "should not send any reminders at wrong hour")
	})

	t.Run("handles send failure gracefully", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
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
		err = b.userRepo.UpdateTimezone(ctx, 2005, "Etc/GMT-8")
		require.NoError(t, err)

		reminded := make(map[int64]string)
		b.checkAndSendReminders(ctx, reminded, nowUTC)

		_, exists := reminded[2005]
		require.False(t, exists, "should not mark as reminded on send failure")
	})

	t.Run("prunes stale entries from reminded map", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.ReminderHour = 14

		reminded := map[int64]string{
			9001: "2026-02-09", // two days ago — should be pruned
			9002: "2026-01-01", // last month — should be pruned
			9003: todayStr,     // today — should survive
			9004: "2026-02-10", // yesterday — should survive (cutoff is yesterday)
		}

		b.checkAndSendReminders(ctx, reminded, nowUTC)

		_, has9001 := reminded[9001]
		_, has9002 := reminded[9002]
		require.False(t, has9001, "old entry should be pruned")
		require.False(t, has9002, "old entry should be pruned")
		require.Equal(t, todayStr, reminded[9003], "today's entry should survive")
		require.Equal(t, "2026-02-10", reminded[9004], "yesterday's entry should survive")
	})

	t.Run("users in different timezones get reminders at correct times", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = time.UTC
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.ReminderHour = 20
		b.cfg.WhitelistedUserIDs = []int64{3001, 3002}

		// User in Asia/Singapore (UTC+8): at 04:00 UTC, local time = 12:00 → not 20
		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        3001,
			Username:  "sguser",
			FirstName: "Sg",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 3001, "Etc/GMT-8")
		require.NoError(t, err)

		// User in America/New_York (UTC-5): at 04:00 UTC, local time = 23:00 → not 20
		err = b.userRepo.UpsertUser(ctx, &models.User{
			ID:        3002,
			Username:  "nyuser",
			FirstName: "Ny",
		})
		require.NoError(t, err)
		err = b.userRepo.UpdateTimezone(ctx, 3002, "Etc/GMT+5")
		require.NoError(t, err)

		// At 04:00 UTC: SG=12:00, NY=23:00 — neither is 20
		nowTest := time.Date(2026, 2, 11, 4, 0, 0, 0, time.UTC)
		reminded := make(map[int64]string)
		b.checkAndSendReminders(ctx, reminded, nowTest)
		require.Equal(t, 0, mockBot.SentMessageCount(), "neither user should be reminded")

		// At 12:00 UTC: SG=20:00, NY=07:00 — only SG matches
		mockBot.Reset()
		nowTest = time.Date(2026, 2, 11, 12, 0, 0, 0, time.UTC)
		b.checkAndSendReminders(ctx, reminded, nowTest)
		require.Equal(t, 1, mockBot.SentMessageCount(), "only SG user should be reminded")
		msg := mockBot.LastSentMessage()
		require.Equal(t, int64(3001), msg.ChatID)

		// At 01:00 UTC next day: SG=09:00, NY=20:00 — only NY matches
		mockBot.Reset()
		nowTest = time.Date(2026, 2, 12, 1, 0, 0, 0, time.UTC)
		b.checkAndSendReminders(ctx, reminded, nowTest)
		require.Equal(t, 1, mockBot.SentMessageCount(), "only NY user should be reminded")
		msg = mockBot.LastSentMessage()
		require.Equal(t, int64(3002), msg.ChatID)
	})

	t.Run("uses default DB timezone when not explicitly set", func(t *testing.T) {
		ctx := context.Background()
		pool := testDB(ctx, t)
		b := setupTestBot(t, pool)
		b.displayLocation = loc // GMT+8
		mockBot := mocks.NewMockBot()
		b.messageSender = mockBot
		b.cfg.ReminderHour = 14
		b.cfg.WhitelistedUserIDs = []int64{3010}

		err := b.userRepo.UpsertUser(ctx, &models.User{
			ID:        3010,
			Username:  "defaulttz",
			FirstName: "Default",
		})
		require.NoError(t, err)
		// Don't set timezone — DB default is Asia/Singapore which is also +8

		reminded := make(map[int64]string)
		b.checkAndSendReminders(ctx, reminded, nowUTC)

		require.Equal(t, 1, mockBot.SentMessageCount(), "should use default timezone and send reminder")
	})
}

func TestStartDailyReminderLoop_RunsImmediateCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)

	mockBot := mocks.NewMockBot()
	b.messageSender = mockBot
	b.cfg.DailyReminderEnabled = true
	b.cfg.WhitelistedUserIDs = []int64{2100}

	err := b.userRepo.UpsertUser(ctx, &models.User{
		ID:        2100,
		Username:  "startupcheck",
		FirstName: "Nora",
	})
	require.NoError(t, err)

	// Set the user's timezone and ReminderHour to match the current local time
	// so the immediate check fires.
	userTZ := "Asia/Singapore"
	err = b.userRepo.UpdateTimezone(ctx, 2100, userTZ)
	require.NoError(t, err)
	loc, _ := time.LoadLocation(userTZ)
	b.cfg.ReminderHour = time.Now().In(loc).Hour()

	done := make(chan struct{})
	go func() {
		defer close(done)
		b.startDailyReminderLoop(ctx)
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

	require.Equal(t, 1, mockBot.SentMessageCount(), "should send reminder on immediate startup check")
}

func TestSendReminderOrDailySummary_FetchError(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)

	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()

	err := b.sendReminderOrDailySummary(
		canceledCtx,
		&models.User{ID: 2200, FirstName: "Err"},
		time.Now().Add(-time.Hour),
		time.Now(),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to fetch today's expenses")
}

func TestSendNoExpenseReminder_EmptyFirstNameFallback(t *testing.T) {
	t.Parallel()

	mockBot := mocks.NewMockBot()
	b := &Bot{messageSender: mockBot}

	err := b.sendNoExpenseReminder(context.Background(), &models.User{ID: 2300, FirstName: ""})
	require.NoError(t, err)
	require.Equal(t, 1, mockBot.SentMessageCount())
	require.Contains(t, mockBot.LastSentMessage().Text, "Hey there!")
}

func TestUserLocation(t *testing.T) {
	t.Parallel()

	fallback := time.FixedZone("fallback", 3*60*60)
	b := &Bot{displayLocation: fallback}

	t.Run("valid timezone", func(t *testing.T) {
		t.Parallel()
		loc := b.userLocation("America/New_York")
		require.Equal(t, "America/New_York", loc.String())
	})

	t.Run("empty timezone falls back", func(t *testing.T) {
		t.Parallel()
		loc := b.userLocation("")
		require.Equal(t, fallback, loc)
	})

	t.Run("invalid timezone falls back", func(t *testing.T) {
		t.Parallel()
		loc := b.userLocation("Invalid/Zone")
		require.Equal(t, fallback, loc)
	})
}
