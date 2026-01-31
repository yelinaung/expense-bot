package bot

import (
	"context"
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

	// Create categories
	foodCategory, err := b.categoryRepo.Create(ctx, "Food - Dining Out")
	require.NoError(t, err)

	transportCategory, err := b.categoryRepo.Create(ctx, "Transport")
	require.NoError(t, err)

	// Create expenses for this week
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	startOfWeek := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 10, 0, 0, 0, now.Location())

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

		// Update created_at to be within this week
		_, err = b.expenseRepo.Pool().Exec(ctx,
			"UPDATE expenses SET created_at = $1 WHERE id = $2",
			startOfWeek.Add(time.Duration(i)*24*time.Hour), expense.ID)
		require.NoError(t, err)
	}

	// Add transport expenses
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

		// Update created_at to be within this week
		_, err = b.expenseRepo.Pool().Exec(ctx,
			"UPDATE expenses SET created_at = $1 WHERE id = $2",
			startOfWeek.Add(time.Duration(i)*24*time.Hour+12*time.Hour), expense.ID)
		require.NoError(t, err)
	}

	// Create expenses for this month (but not this week)
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 10, 0, 0, 0, now.Location())
	for i := 0; i < 2; i++ {
		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      decimal.NewFromFloat(25.00),
			Currency:    "SGD",
			Description: "Monthly expense",
			CategoryID:  &foodCategory.ID,
			Status:      appmodels.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		// Update created_at to be within this month but before this week
		_, err = b.expenseRepo.Pool().Exec(ctx,
			"UPDATE expenses SET created_at = $1 WHERE id = $2",
			startOfMonth.Add(time.Duration(i)*24*time.Hour), expense.ID)
		require.NoError(t, err)
	}

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
		require.Contains(t, doc.Caption, "5 expenses") // 3 food + 2 transport
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
		require.Contains(t, doc.Caption, "7 expenses") // All expenses are in current month
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
