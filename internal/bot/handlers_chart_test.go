package bot

import (
	"context"
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

	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

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
	for i := 0; i < 3; i++ {
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
	for i := 0; i < 2; i++ {
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
		update := mocks.CommandUpdate(chatID, userID, "/chart week")

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
		require.Contains(t, msg.Text, "/chart week")
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
		update := mocks.CommandUpdate(chatID, newUserID, "/chart week")

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
}

func TestHandleChartWrapper(t *testing.T) {
	t.Parallel()

	b := &Bot{}
	ctx := context.Background()
	var tgBot *bot.Bot

	t.Run("wrapper delegates to core", func(t *testing.T) {
		update := &models.Update{}
		b.handleChart(ctx, tgBot, update)
		// Should not panic
	})
}
