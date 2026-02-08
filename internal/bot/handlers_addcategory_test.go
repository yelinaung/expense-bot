package bot

import (
	"context"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestHandleAddCategoryCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	userID := int64(800001)
	chatID := int64(800001)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "addcatuser",
		FirstName: "AddCat",
	})
	require.NoError(t, err)

	t.Run("returns error when no name provided", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/addcategory")

		b.handleAddCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Please provide a category name")
		require.Contains(t, msg.Text, "/addcategory")
	})

	t.Run("creates category successfully", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/addcategory Test New Cat 800")

		b.handleAddCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Test New Cat 800")
		require.Contains(t, msg.Text, "created")
	})

	t.Run("handles bot mention in command", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/addcategory@mybot My Category 800")

		b.handleAddCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "My Category 800")
		require.Contains(t, msg.Text, "created")
	})

	t.Run("returns early for nil message", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{}

		b.handleAddCategoryCore(ctx, mockBot, update)

		require.Equal(t, 0, mockBot.SentMessageCount())
	})
}

func TestHandleAddCategoryCoreDuplicate(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	userID := int64(800002)
	chatID := int64(800002)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "addcatdup",
		FirstName: "AddCatDup",
	})
	require.NoError(t, err)

	_, err = b.categoryRepo.Create(ctx, "Duplicate Cat 800")
	require.NoError(t, err)

	b.invalidateCategoryCache()

	mockBot := mocks.NewMockBot()
	update := mocks.CommandUpdate(chatID, userID, "/addcategory Duplicate Cat 800")

	b.handleAddCategoryCore(ctx, mockBot, update)

	require.Equal(t, 1, mockBot.SentMessageCount())
	msg := mockBot.LastSentMessage()
	require.Contains(t, msg.Text, "Failed to create category")
}

func TestHandleAddCategoryWrapper(t *testing.T) {
	t.Parallel()

	b := &Bot{}
	ctx := context.Background()
	var tgBot *bot.Bot

	t.Run("wrapper delegates to core", func(t *testing.T) {
		update := &models.Update{}
		b.handleAddCategory(ctx, tgBot, update)
	})
}
