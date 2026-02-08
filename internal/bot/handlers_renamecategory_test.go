package bot

import (
	"context"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestHandleRenameCategoryCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	userID := int64(900001)
	chatID := int64(900001)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "renamecatuser",
		FirstName: "RenameCat",
	})
	require.NoError(t, err)

	t.Run("returns error when no args provided", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/renamecategory")

		b.handleRenameCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Old Name")
		require.Contains(t, msg.Text, "New Name")
	})

	t.Run("returns error when arrow separator is missing", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/renamecategory Food Dining")

		b.handleRenameCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Old Name")
	})

	t.Run("returns error when old name is empty", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/renamecategory  -> New Name")

		b.handleRenameCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Both old and new")
	})

	t.Run("returns error when new name is empty", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/renamecategory Old Name ->  ")

		b.handleRenameCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Both old and new")
	})

	t.Run("rejects new name with control characters", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/renamecategory Food -> Evil\nName")

		b.handleRenameCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "control characters")
	})

	t.Run("rejects new name that is too long", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		longName := strings.Repeat("x", appmodels.MaxCategoryNameLength+1)
		update := mocks.CommandUpdate(chatID, userID, "/renamecategory Food -> "+longName)

		b.handleRenameCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "too long")
	})

	t.Run("returns error when category not found", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/renamecategory Nonexistent -> New")

		b.handleRenameCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "not found")
	})

	t.Run("renames category successfully", func(t *testing.T) {
		_, err := b.categoryRepo.Create(ctx, "Rename Me 900")
		require.NoError(t, err)
		b.invalidateCategoryCache()

		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/renamecategory Rename Me 900 -> Renamed 900")

		b.handleRenameCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "renamed")
		require.Contains(t, msg.Text, "Renamed 900")
	})

	t.Run("returns error when new name already exists", func(t *testing.T) {
		_, err := b.categoryRepo.Create(ctx, "Existing A 900")
		require.NoError(t, err)
		_, err = b.categoryRepo.Create(ctx, "Existing B 900")
		require.NoError(t, err)
		b.invalidateCategoryCache()

		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/renamecategory Existing A 900 -> Existing B 900")

		b.handleRenameCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "already exists")
	})

	t.Run("handles bot mention in command", func(t *testing.T) {
		_, err := b.categoryRepo.Create(ctx, "Mention Cat 900")
		require.NoError(t, err)
		b.invalidateCategoryCache()

		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/renamecategory@mybot Mention Cat 900 -> Mentioned 900")

		b.handleRenameCategoryCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "renamed")
	})

	t.Run("returns early for nil message", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{}

		b.handleRenameCategoryCore(ctx, mockBot, update)

		require.Equal(t, 0, mockBot.SentMessageCount())
	})
}
