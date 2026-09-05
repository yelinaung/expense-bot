package bot

import (
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestHandleReportCore(t *testing.T) {
	// Note: Not using t.Parallel() to avoid database cleanup conflicts

	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)

	userID := int64(800001)
	chatID := int64(800001)

	// Create user
	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "reportuser",
		FirstName: "Report",
	})
	require.NoError(t, err)

	// Create category
	category, err := b.categoryRepo.Create(ctx, "Test Report Category")
	require.NoError(t, err)

	// Create expenses for this week
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	startOfWeek := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 10, 0, 0, 0, now.Location())

	for i := range 3 {
		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      decimal.NewFromFloat(10.50),
			Currency:    "SGD",
			Description: "Weekly expense",
			CategoryID:  &category.ID,
			Status:      appmodels.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		// Update created_at to be within this week
		_, err = b.expenseRepo.Pool().Exec(ctx,
			testUpdateExpenseTimeSQL,
			startOfWeek.Add(time.Duration(i)*24*time.Hour), expense.ID)
		require.NoError(t, err)
	}

	// Create expenses for this month (but not this week)
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 10, 0, 0, 0, now.Location())
	for i := range 2 {
		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      decimal.NewFromFloat(20.00),
			Currency:    "SGD",
			Description: "Monthly expense",
			CategoryID:  &category.ID,
			Status:      appmodels.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		// Update created_at to be within this month but before this week
		_, err = b.expenseRepo.Pool().Exec(ctx,
			testUpdateExpenseTimeSQL,
			startOfMonth.Add(time.Duration(i)*24*time.Hour), expense.ID)
		require.NoError(t, err)
	}

	t.Run("generates weekly report CSV", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, testReportWeekCommand)

		b.handleReportCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentDocumentCount())
		doc := mockBot.LastSentDocument()
		require.NotNil(t, doc)
		require.Contains(t, doc.Filename, "expenses_week_")
		require.Contains(t, doc.Caption, "Weekly Expenses")
	})

	t.Run("generates monthly report CSV", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, testReportMonthCommand)

		b.handleReportCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentDocumentCount())
		doc := mockBot.LastSentDocument()
		require.NotNil(t, doc)
		require.Contains(t, doc.Filename, "expenses_month_")
		require.Contains(t, doc.Caption, "Monthly Expenses")
	})

	t.Run("uses display timezone boundaries for weekly report", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		originalDisplayLocation := b.displayLocation
		b.displayLocation = time.FixedZone("GMT+8", 8*60*60)
		t.Cleanup(func() {
			b.displayLocation = originalDisplayLocation
		})
		fixedNow := time.Date(2026, 2, 25, 10, 0, 0, 0, b.displayLocation)
		originalNowFunc := b.nowFunc
		b.nowFunc = func() time.Time {
			return fixedNow
		}
		t.Cleanup(func() {
			b.nowFunc = originalNowFunc
		})

		tzUserID := int64(800003)
		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        tzUserID,
			Username:  "reportweektz",
			FirstName: "ReportWeekTZ",
		})
		require.NoError(t, err)

		makeExpense := func(amount string, desc string, ts time.Time) {
			expense := &appmodels.Expense{
				UserID:      tzUserID,
				Amount:      decimal.RequireFromString(amount),
				Currency:    "SGD",
				Description: desc,
				CategoryID:  &category.ID,
				Status:      appmodels.ExpenseStatusConfirmed,
			}
			err = b.expenseRepo.Create(ctx, expense)
			require.NoError(t, err)
			_, err = b.expenseRepo.Pool().Exec(ctx,
				testUpdateExpenseTimeSQL,
				ts, expense.ID)
			require.NoError(t, err)
		}

		makeExpense("1.00", "Prev Sunday", time.Date(2026, 2, 22, 23, 59, 0, 0, b.displayLocation))
		makeExpense("2.00", "Monday Start", time.Date(2026, 2, 23, 0, 1, 0, 0, b.displayLocation))
		makeExpense("3.00", "Sunday End", time.Date(2026, 3, 1, 23, 59, 0, 0, b.displayLocation))
		makeExpense("4.00", "Next Monday", time.Date(2026, 3, 2, 0, 1, 0, 0, b.displayLocation))

		update := mocks.CommandUpdate(chatID, tzUserID, testReportWeekCommand)
		b.handleReportCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentDocumentCount())
		doc := mockBot.LastSentDocument()
		require.NotNil(t, doc)
		require.Equal(t, "expenses_week_2026-02-23.csv", doc.Filename)
		require.Contains(t, doc.Caption, "Weekly Expenses (Feb 23 to Mar 1, 2026)")
		require.Contains(t, doc.Caption, "Total Expenses: $5.00 SGD")
		require.Contains(t, doc.Caption, "Count: 2")
	})

	t.Run("uses display timezone boundaries for monthly report", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		originalDisplayLocation := b.displayLocation
		b.displayLocation = time.FixedZone("GMT+8", 8*60*60)
		t.Cleanup(func() {
			b.displayLocation = originalDisplayLocation
		})
		fixedNow := time.Date(2026, 2, 25, 10, 0, 0, 0, b.displayLocation)
		originalNowFunc := b.nowFunc
		b.nowFunc = func() time.Time {
			return fixedNow
		}
		t.Cleanup(func() {
			b.nowFunc = originalNowFunc
		})

		tzUserID := int64(800004)
		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        tzUserID,
			Username:  "reportmonthtz",
			FirstName: "ReportMonthTZ",
		})
		require.NoError(t, err)

		makeExpense := func(amount string, desc string, ts time.Time) {
			expense := &appmodels.Expense{
				UserID:      tzUserID,
				Amount:      decimal.RequireFromString(amount),
				Currency:    "SGD",
				Description: desc,
				CategoryID:  &category.ID,
				Status:      appmodels.ExpenseStatusConfirmed,
			}
			err = b.expenseRepo.Create(ctx, expense)
			require.NoError(t, err)
			_, err = b.expenseRepo.Pool().Exec(ctx,
				testUpdateExpenseTimeSQL,
				ts, expense.ID)
			require.NoError(t, err)
		}

		makeExpense("10.00", "Prev Month", time.Date(2026, 1, 31, 23, 59, 0, 0, b.displayLocation))
		makeExpense("20.00", "Month Start", time.Date(2026, 2, 1, 0, 1, 0, 0, b.displayLocation))
		makeExpense("30.00", "Month End", time.Date(2026, 2, 28, 23, 59, 0, 0, b.displayLocation))
		makeExpense("40.00", "Next Month", time.Date(2026, 3, 1, 0, 1, 0, 0, b.displayLocation))

		update := mocks.CommandUpdate(chatID, tzUserID, testReportMonthCommand)
		b.handleReportCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentDocumentCount())
		doc := mockBot.LastSentDocument()
		require.NotNil(t, doc)
		require.Equal(t, "expenses_month_2026-02.csv", doc.Filename)
		require.Contains(t, doc.Caption, "Monthly Expenses (February 2026)")
		require.Contains(t, doc.Caption, "Total Expenses: $50.00 SGD")
		require.Contains(t, doc.Caption, "Count: 2")
	})

	t.Run("returns error for invalid period", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/report invalid")

		b.handleReportCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "❌ Invalid report type")
		require.Contains(t, msg.Text, "week")
		require.Contains(t, msg.Text, "month")
	})

	t.Run("returns error when no period specified", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/report")

		b.handleReportCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "❌ Please specify report type")
		require.Contains(t, msg.Text, testReportWeekCommand)
		require.Contains(t, msg.Text, testReportMonthCommand)
	})

	t.Run("handles period with no expenses", func(t *testing.T) {
		// Create a new user with no expenses
		newUserID := int64(800002)
		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        newUserID,
			Username:  "emptyuser",
			FirstName: "Empty",
		})
		require.NoError(t, err)

		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, newUserID, testReportWeekCommand)

		b.handleReportCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "No expenses found for week")
	})

	t.Run("returns early for nil message", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{}

		b.handleReportCore(ctx, mockBot, update)

		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	// Regresses CSV export "Worth It" column being permanently blank.
	// handleReportCore fetches via GetByUserIDAndDateRange, whose SELECT
	// historically omitted e.worth_it; the CSV then rendered an empty cell
	// via worthItCSVCell even for expenses the user explicitly reviewed.
	t.Run("renders Worth It column from persisted reflection in CSV body", func(t *testing.T) {
		// Fixed clock + UTC so the weekly window is deterministic and isolated
		// from the outer-scoped expenses (which use time.Now()).
		originalNowFunc := b.nowFunc
		originalDisplayLocation := b.displayLocation
		b.displayLocation = time.UTC
		fixedNow := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC) // Thursday
		b.nowFunc = func() time.Time { return fixedNow }
		t.Cleanup(func() {
			b.nowFunc = originalNowFunc
			b.displayLocation = originalDisplayLocation
		})

		reflectUserID := int64(800010)
		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        reflectUserID,
			Username:  "reflectcsvuser",
			FirstName: "ReflectCSV",
		})
		require.NoError(t, err)

		// Monday of the fixed week: 2026-06-15 00:00..2026-06-22 00:00 UTC.
		weekStart := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)

		makeBackdated := func(amount, desc string, ts time.Time) *appmodels.Expense {
			expense := &appmodels.Expense{
				UserID:      reflectUserID,
				Amount:      decimal.RequireFromString(amount),
				Currency:    "SGD",
				Description: desc,
				CategoryID:  &category.ID,
				Status:      appmodels.ExpenseStatusConfirmed,
			}
			err = b.expenseRepo.Create(ctx, expense)
			require.NoError(t, err)
			_, err = b.expenseRepo.Pool().Exec(ctx,
				testUpdateExpenseTimeSQL, ts, expense.ID)
			require.NoError(t, err)
			return expense
		}

		worthItExp := makeBackdated("10.00", "Worth it row", weekStart.Add(1*time.Hour))
		notWorthItExp := makeBackdated("20.00", "Not worth it row", weekStart.Add(2*time.Hour))
		unreviewedExp := makeBackdated("30.00", "Unreviewed row", weekStart.Add(3*time.Hour))

		worth := true
		err = b.expenseRepo.UpdateReflection(ctx, worthItExp.ID, reflectUserID, &worth, "Necessity")
		require.NoError(t, err)
		notWorth := false
		err = b.expenseRepo.UpdateReflection(ctx, notWorthItExp.ID, reflectUserID, &notWorth, "")
		require.NoError(t, err)

		// Confirm reflection actually persisted in DB (sanity check independent
		// of the report query).
		var dbWorthIt *bool
		err = b.expenseRepo.Pool().QueryRow(ctx,
			`SELECT worth_it FROM expenses WHERE id = $1`, worthItExp.ID).Scan(&dbWorthIt)
		require.NoError(t, err)
		require.NotNil(t, dbWorthIt)
		require.True(t, *dbWorthIt)

		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, reflectUserID, testReportWeekCommand)
		b.handleReportCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentDocumentCount())
		doc := mockBot.LastSentDocument()
		require.NotNil(t, doc)
		require.NotEmpty(t, doc.Body, "mock should capture the CSV body bytes")

		reader := csv.NewReader(bytes.NewReader(doc.Body))
		records, err := reader.ReadAll()
		require.NoError(t, err)
		require.Len(t, records, 4, "header + 3 expense rows")

		require.Equal(t,
			[]string{"ID", "Date", "Amount", "Currency", "Description", "Merchant", "Category", "Worth It"},
			records[0])

		byNumber := make(map[string]string, len(records)-1)
		for _, row := range records[1:] {
			require.Len(t, row, 8)
			byNumber[row[0]] = row[7]
		}
		require.Equal(t, "Worth it",
			byNumber[strconv.FormatInt(worthItExp.UserExpenseNumber, 10)],
			"reviewed-as-worth-it row must render 'Worth it' in the Worth It column")
		require.Equal(t, "Not worth it",
			byNumber[strconv.FormatInt(notWorthItExp.UserExpenseNumber, 10)],
			"reviewed-as-not-worth-it row must render 'Not worth it' in the Worth It column")
		// Unreviewed rows keep a nil WorthIt, which the CSV renders as an empty cell
		// (see worthItCSVCell). Use Empty instead of Equal(t, "") per testifylint.
		require.Empty(t,
			byNumber[strconv.FormatInt(unreviewedExp.UserExpenseNumber, 10)],
			"unreviewed row must render an empty Worth It cell")
	})
}

func TestHandleReportWrapper(t *testing.T) {
	t.Parallel()

	b := &Bot{}
	ctx := context.Background()
	var tgBot *bot.Bot

	t.Run("wrapper delegates to core", func(t *testing.T) {
		t.Parallel()
		update := &models.Update{}
		b.handleReport(ctx, tgBot, update)
		// Should not panic
	})
}
