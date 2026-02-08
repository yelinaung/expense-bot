package bot

import (
	"context"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestHandleDeleteCategoryCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	userID := int64(910001)
	chatID := int64(910001)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "delcatuser",
		FirstName: "DelCat",
	})
	require.NoError(t, err)

	t.Run("returns error when no args provided", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/deletecategory")

		b.handleDeleteCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Please provide a category name")
	})

	t.Run("returns error when category not found", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/deletecategory Nonexistent Cat")

		b.handleDeleteCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "not found")
	})

	t.Run("deletes category with no expenses", func(t *testing.T) {
		cat, err := b.categoryRepo.Create(ctx, "Delete Me 910")
		require.NoError(t, err)
		require.NotNil(t, cat)
		b.invalidateCategoryCache()

		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/deletecategory Delete Me 910")

		b.handleDeleteCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "deleted")
		require.NotContains(t, msg.Text, "uncategorized")
	})

	t.Run("deletes category and uncategorizes expenses", func(t *testing.T) {
		cat, err := b.categoryRepo.Create(ctx, "Has Expenses 910")
		require.NoError(t, err)
		b.invalidateCategoryCache()

		// Create an expense referencing this category.
		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("10.00"),
			Currency:    "SGD",
			Description: "test expense for delete cat",
			CategoryID:  &cat.ID,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/deletecategory Has Expenses 910")

		b.handleDeleteCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "deleted")
		require.Contains(t, msg.Text, "1 expense(s) have been uncategorized")

		// Verify the expense's category was nullified.
		updated, err := b.expenseRepo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.Nil(t, updated.CategoryID)
	})

	t.Run("handles bot mention in command", func(t *testing.T) {
		cat, err := b.categoryRepo.Create(ctx, "Mention Del 910")
		require.NoError(t, err)
		require.NotNil(t, cat)
		b.invalidateCategoryCache()

		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/deletecategory@mybot Mention Del 910")

		b.handleDeleteCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "deleted")
	})

	t.Run("returns early for nil message", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{}

		b.handleDeleteCategoryCore(ctx, mockBot, update)

		require.Equal(t, 0, mockBot.SentMessageCount())
	})
}
