package bot

import (
	"context"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

// TestCallbackHandlerWrappers provides coverage for thin callback wrapper functions.
// These wrappers exist only to match the telegram bot library's expected handler signature
// and delegate to Core functions which are thoroughly tested in handlers_callbacks_test.go.
//
// We test wrappers by calling them with updates that cause early returns in Core functions,
// avoiding the need for a real *bot.Bot instance.
func TestCallbackHandlerWrappers(t *testing.T) {
	t.Parallel()

	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	// Create a user for tests that need it
	userID := int64(900002)
	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "callbackwrapperuser",
		FirstName: "CallbackWrapper",
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// nil *bot.Bot - wrappers pass it through but Core returns early before using it
	var tgBot *bot.Bot

	t.Run("handleEditCallback wrapper", func(t *testing.T) {
		t.Parallel()
		// Update with nil CallbackQuery causes early return in handleEditCallbackCore
		b.handleEditCallback(ctx, tgBot, &models.Update{})
	})

	t.Run("handlePendingEdit wrapper", func(t *testing.T) {
		t.Parallel()
		// Update with nil Message causes early return in handlePendingEditCore
		result := b.handlePendingEdit(ctx, tgBot, &models.Update{})
		if result {
			t.Error("expected false for nil message")
		}
	})

	t.Run("handleCancelEditCallback wrapper", func(t *testing.T) {
		t.Parallel()
		// Update with nil CallbackQuery causes early return in handleCancelEditCallbackCore
		b.handleCancelEditCallback(ctx, tgBot, &models.Update{})
	})

	t.Run("handleSetCategoryCallback wrapper", func(t *testing.T) {
		t.Parallel()
		// Update with nil CallbackQuery causes early return in handleSetCategoryCallbackCore
		b.handleSetCategoryCallback(ctx, tgBot, &models.Update{})
	})

	t.Run("handleCreateCategoryCallback wrapper", func(t *testing.T) {
		t.Parallel()
		// Update with nil CallbackQuery causes early return in handleCreateCategoryCallbackCore
		b.handleCreateCategoryCallback(ctx, tgBot, &models.Update{})
	})
}
