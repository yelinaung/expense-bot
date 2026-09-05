package bot

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestHandleChartCore(t *testing.T) {
	// Note: Not using t.Parallel() to avoid database cleanup conflicts

	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)

	userID := int64(900001)
	chatID := int64(900001)

	// Create user
	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "chartuser",
		FirstName: "Chart",
	})
	require.NoError(t, err)

	// Create categories with unique names to avoid conflicts with seed data
	foodCategory, err := b.categoryRepo.Create(ctx, "Test Chart Category Food")
	require.NoError(t, err)

	transportCategory, err := b.categoryRepo.Create(ctx, "Test Chart Category Transport")
	require.NoError(t, err)

	// All expenses are placed on "today" to ensure they fall within both
	// the current week AND current month, avoiding edge case failures
	now := time.Now()
	loc := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, loc)

	// Create 3 food expenses (all dated today)
	for i := range 3 {
		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      decimal.NewFromFloat(15.50),
			Currency:    "SGD",
			Description: "Weekly food expense",
			CategoryID:  &foodCategory.ID,
			Status:      appmodels.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		// All placed today with different hours to differentiate
		expenseDate := today.Add(time.Duration(i) * time.Hour)
		_, err = b.expenseRepo.Pool().Exec(ctx,
			"UPDATE expenses SET created_at = $1 WHERE id = $2",
			expenseDate, expense.ID)
		require.NoError(t, err)
	}

	// Create 2 transport expenses (all dated today)
	for i := range 2 {
		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      decimal.NewFromFloat(5.00),
			Currency:    "SGD",
			Description: "Weekly transport expense",
			CategoryID:  &transportCategory.ID,
			Status:      appmodels.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		expenseDate := today.Add(time.Duration(i+3) * time.Hour)
		_, err = b.expenseRepo.Pool().Exec(ctx,
			"UPDATE expenses SET created_at = $1 WHERE id = $2",
			expenseDate, expense.ID)
		require.NoError(t, err)
	}

	// All 5 expenses are in both current week and current month
	weeklyExpenseCount := 5
	totalMonthlyExpenseCount := 5

	t.Run("generates weekly chart", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, testChartWeekCommand)

		b.handleChartCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentDocumentCount())
		doc := mockBot.LastSentDocument()
		require.NotNil(t, doc)
		require.Contains(t, doc.Filename, "chart_week_")
		require.Contains(t, doc.Filename, ".png")
		require.Contains(t, doc.Caption, "Weekly Expenses")
		require.Contains(t, doc.Caption, "Total:")
		require.Contains(t, doc.Caption, "Count:")
		require.Contains(t, doc.Caption, fmt.Sprintf("%d expenses", weeklyExpenseCount))
	})

	t.Run("generates monthly chart", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/chart month")

		b.handleChartCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentDocumentCount())
		doc := mockBot.LastSentDocument()
		require.NotNil(t, doc)
		require.Contains(t, doc.Filename, "chart_month_")
		require.Contains(t, doc.Filename, ".png")
		require.Contains(t, doc.Caption, "Monthly Expenses")
		require.Contains(t, doc.Caption, "Total:")
		require.Contains(t, doc.Caption, "Count:")
		require.Contains(t, doc.Caption, fmt.Sprintf("%d expenses", totalMonthlyExpenseCount))
	})

	t.Run("sends failure message when document send fails", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		mockBot.SendDocumentError = errors.New("telegram send failed")
		update := mocks.CommandUpdate(chatID, userID, testChartWeekCommand)

		b.handleChartCore(ctx, mockBot, update)

		// SendDocument failed, so the handler falls back to an error message.
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "❌ Failed to send chart")
	})

	t.Run("returns error for invalid period", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/chart invalid")

		b.handleChartCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "❌ Invalid chart type")
		require.Contains(t, msg.Text, "week")
		require.Contains(t, msg.Text, "month")
	})

	t.Run("returns error when no period specified", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/chart")

		b.handleChartCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "❌ Please specify chart type")
		require.Contains(t, msg.Text, testChartWeekCommand)
		require.Contains(t, msg.Text, "/chart month")
	})

	t.Run("handles period with no expenses", func(t *testing.T) {
		// Create a new user with no expenses
		newUserID := int64(900002)
		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        newUserID,
			Username:  "emptyuser",
			FirstName: "Empty",
		})
		require.NoError(t, err)

		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, newUserID, testChartWeekCommand)

		b.handleChartCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "📊 No expenses found for week")
	})

	t.Run("returns early for nil message", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{}

		b.handleChartCore(ctx, mockBot, update)

		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("sends failure message when chart generation fails", func(t *testing.T) {
		// Inject a failing chart generator.
		b.generateChart = func(_ []appmodels.Expense, _ string) ([]byte, error) {
			return nil, errors.New("render error")
		}
		defer func() { b.generateChart = nil }()

		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, testChartWeekCommand)

		b.handleChartCore(ctx, mockBot, update)

		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, failedGenerateChartMsg)
	})
}

func TestHandleChartCoreCurrency(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)

	now := time.Now()
	loc := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 11, 0, 0, 0, loc)

	mkExpense := func(userID int64, amt float64, cur string) {
		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      decimal.NewFromFloat(amt),
			Currency:    cur,
			Description: "chart currency test",
			Status:      appmodels.ExpenseStatusConfirmed,
		}
		require.NoError(t, b.expenseRepo.Create(ctx, expense))
		_, err := b.expenseRepo.Pool().Exec(ctx, testUpdateExpenseTimeSQL, today, expense.ID)
		require.NoError(t, err)
	}

	chart := func(userID, chatID int64) *mocks.MockBot {
		mb := mocks.NewMockBot()
		b.handleChartCore(ctx, mb, mocks.CommandUpdate(chatID, userID, testChartWeekCommand))
		return mb
	}

	t.Run("non-SGD default labels caption with default currency", func(t *testing.T) {
		uid := int64(910001)
		require.NoError(t, b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID: uid, Username: "chartusd", FirstName: "U",
		}))
		require.NoError(t, b.userRepo.UpdateDefaultCurrency(ctx, uid, "USD"))
		mkExpense(uid, 10, "USD")
		mkExpense(uid, 20, "USD")

		mb := chart(uid, uid)

		require.Equal(t, 1, mb.SentDocumentCount(), "single currency should produce a single document")
		doc := mb.LastSentDocument()
		require.Contains(t, doc.Caption, "Weekly Expenses")
		require.Contains(t, doc.Caption, "Total:")
		require.Contains(t, doc.Caption, "USD: $30.00")
		require.NotContains(t, doc.Caption, "SGD", "caption must not hardcode SGD for USD default")
		require.Contains(t, doc.Caption, "Count: 2 expenses")
	})

	t.Run("mixed currencies render one pie per currency with no cross-currency sum", func(t *testing.T) {
		uid := int64(910002)
		require.NoError(t, b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID: uid, Username: "chartmix", FirstName: "M",
		}))
		mkExpense(uid, 20, "SGD")
		mkExpense(uid, 5000, "JPY")

		mb := chart(uid, uid)

		require.Equal(t, 2, mb.SentDocumentCount(), "two currencies should produce two documents")
		require.Len(t, mb.SentDocuments, 2)

		// sortedCurrencyKeys orders JPY before SGD, so the first document
		// carries the full per-currency caption and the JPY pie.
		first := mb.SentDocuments[0]
		second := mb.SentDocuments[1]

		require.Contains(t, first.Filename, "chart_week_")
		require.Contains(t, first.Filename, "_JPY.png")
		require.Contains(t, second.Filename, "_SGD.png")

		require.Contains(t, first.Caption, "JPY: ¥5000.00")
		require.Contains(t, first.Caption, "SGD: S$20.00")
		require.NotContains(t, first.Caption, "5020", "caption must not mix numeric amounts across currencies")
		require.Contains(t, first.Caption, "Count: 2 expenses")

		require.Contains(t, second.Caption, "(SGD)")
		require.NotContains(t, second.Caption, "Total:", "subsequent documents carry only a short tag")
	})
}

func TestHandleChartWrapper(t *testing.T) {
	t.Parallel()

	b := &Bot{}
	ctx := context.Background()
	var tgBot *bot.Bot

	t.Run("wrapper delegates to core", func(t *testing.T) {
		t.Parallel()
		update := &models.Update{}
		b.handleChart(ctx, tgBot, update)
		// Should not panic
	})
}
