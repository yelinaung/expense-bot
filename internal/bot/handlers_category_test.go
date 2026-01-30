package bot

import (
	"context"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestHandleCategoryCore(t *testing.T) {
	// Note: Not using t.Parallel() to avoid database cleanup conflicts

	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	userID := int64(700999)
	chatID := int64(700999)

	// Create user
	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "categoryuser",
		FirstName: "Category",
	})
	require.NoError(t, err)

	// Verify user exists
	user, err := b.userRepo.GetUserByID(ctx, userID)
	require.NoError(t, err, "user should exist after upsert")
	require.Equal(t, userID, user.ID)

	// Create unique categories for this test
	foodCategory, err := b.categoryRepo.Create(ctx, "Test Food Category 999")
	require.NoError(t, err)

	transportCategory, err := b.categoryRepo.Create(ctx, "Test Transport Category 999")
	require.NoError(t, err)

	// Create expenses in food category
	for i := 1; i <= 3; i++ {
		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      decimal.NewFromFloat(10.50),
			Currency:    "SGD",
			Description: "Lunch",
			CategoryID:  &foodCategory.ID,
			Status:      appmodels.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)
	}

	// Create expenses in transport category
	expense := &appmodels.Expense{
		UserID:      userID,
		Amount:      decimal.NewFromFloat(5.00),
		Currency:    "SGD",
		Description: "Bus",
		CategoryID:  &transportCategory.ID,
		Status:      appmodels.ExpenseStatusConfirmed,
	}
	err = b.expenseRepo.Create(ctx, expense)
	require.NoError(t, err)

	t.Run("filters expenses by category - exact match", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/category Test Food Category 999")

		b.handleCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Test Food Category 999 Expenses")
		require.Contains(t, msg.Text, "Total: $31.50") // 3 * 10.50
		require.Contains(t, msg.Text, "#")             // Should contain expense IDs
		require.Contains(t, msg.Text, "Lunch")
	})

	t.Run("filters expenses by category - case insensitive", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/category test transport category 999")

		b.handleCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Test Transport Category 999 Expenses")
		require.Contains(t, msg.Text, "Total: $5.00")
		require.Contains(t, msg.Text, "Bus")
	})

	t.Run("returns error for non-existent category", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/category NonExistent")

		b.handleCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "❌ Category 'NonExistent' not found")
		require.Contains(t, msg.Text, "Use /categories to see all available categories")
	})

	t.Run("returns error when no category name provided", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/category")

		b.handleCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "❌ Please provide a category name")
		require.Contains(t, msg.Text, "/category")
	})

	t.Run("handles category with no expenses", func(t *testing.T) {
		_, err := b.categoryRepo.Create(ctx, "Empty Category 999")
		require.NoError(t, err)

		// Invalidate cache so new category is found
		b.categoryCacheMu.Lock()
		b.categoryCache = nil
		b.categoryCacheMu.Unlock()

		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/category Empty Category 999")

		b.handleCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Empty Category 999 Expenses")
		require.Contains(t, msg.Text, "Total: $0.00")
		require.Contains(t, msg.Text, "No expenses found")
	})

	t.Run("returns early for nil message", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{}

		b.handleCategoryCore(ctx, mockBot, update)

		require.Equal(t, 0, mockBot.SentMessageCount(), "should not send message for nil update")
	})
}

func TestHandleCategoryWrapper(t *testing.T) {
	t.Parallel()

	// Minimal bot instance - wrapper returns early so we don't need full setup
	b := &Bot{}
	ctx := context.Background()
	var tgBot *bot.Bot

	t.Run("wrapper delegates to core", func(t *testing.T) {
		update := &models.Update{}
		// This should not panic even with nil bot
		b.handleCategory(ctx, tgBot, update)
	})
}
