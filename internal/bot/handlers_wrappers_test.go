package bot

import (
	"context"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

// TestCommandHandlerWrappers provides coverage for thin wrapper functions.
// These wrappers exist only to match the telegram bot library's expected handler signature
// and delegate to Core functions which are thoroughly tested in handlers_core_test.go.
//
// We test wrappers by calling them with updates that cause early returns in Core functions,
// avoiding the need for a real *bot.Bot instance.
func TestCommandHandlerWrappers(t *testing.T) {
	t.Parallel()

	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	// Create a user for tests that need it
	userID := int64(900001)
	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "wrapperuser",
		FirstName: "Wrapper",
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// nil *bot.Bot - wrappers pass it through but Core returns early before using it
	var tgBot *bot.Bot

	t.Run("handleStart wrapper", func(t *testing.T) {
		t.Parallel()
		// Update with nil Message causes early return in handleStartCore
		b.handleStart(ctx, tgBot, &models.Update{})
	})

	t.Run("handleHelp wrapper", func(t *testing.T) {
		t.Parallel()
		// Update with nil Message causes early return in handleHelpCore
		b.handleHelp(ctx, tgBot, &models.Update{})
	})

	t.Run("handleCategories wrapper", func(t *testing.T) {
		t.Parallel()
		// Update with nil Message causes early return in handleCategoriesCore
		b.handleCategories(ctx, tgBot, &models.Update{})
	})

	t.Run("handleAddCategory wrapper", func(t *testing.T) {
		t.Parallel()
		b.handleAddCategory(ctx, tgBot, &models.Update{})
	})

	t.Run("handleAdd wrapper", func(t *testing.T) {
		t.Parallel()
		// Update with nil Message causes early return in handleAddCore
		b.handleAdd(ctx, tgBot, &models.Update{})
	})

	t.Run("handleList wrapper", func(t *testing.T) {
		t.Parallel()
		// Update with nil Message causes early return in handleListCore
		b.handleList(ctx, tgBot, &models.Update{})
	})

	t.Run("handleToday wrapper", func(t *testing.T) {
		t.Parallel()
		// Update with nil Message causes early return in handleTodayCore
		b.handleToday(ctx, tgBot, &models.Update{})
	})

	t.Run("handleWeek wrapper", func(t *testing.T) {
		t.Parallel()
		// Update with nil Message causes early return in handleWeekCore
		b.handleWeek(ctx, tgBot, &models.Update{})
	})

	t.Run("handleFreeTextExpense wrapper - nil message", func(t *testing.T) {
		t.Parallel()
		// Update with nil Message causes early return
		result := b.handleFreeTextExpense(ctx, tgBot, &models.Update{})
		if result {
			t.Error("expected false for nil message")
		}
	})

	t.Run("handleFreeTextExpense wrapper - empty text", func(t *testing.T) {
		t.Parallel()
		// Update with empty text causes early return
		result := b.handleFreeTextExpense(ctx, tgBot, &models.Update{
			Message: &models.Message{Text: ""},
		})
		if result {
			t.Error("expected false for empty text")
		}
	})

	t.Run("handleFreeTextExpense wrapper - command text", func(t *testing.T) {
		t.Parallel()
		// Update with command (starts with /) causes early return
		result := b.handleFreeTextExpense(ctx, tgBot, &models.Update{
			Message: &models.Message{Text: "/start"},
		})
		if result {
			t.Error("expected false for command text")
		}
	})

	t.Run("handleFreeTextExpense wrapper - invalid expense", func(t *testing.T) {
		t.Parallel()
		// Update with text that doesn't parse as expense
		result := b.handleFreeTextExpense(ctx, tgBot, &models.Update{
			Message: &models.Message{Text: "just random text"},
		})
		if result {
			t.Error("expected false for invalid expense text")
		}
	})

	t.Run("handleFreeTextExpense wrapper - valid expense", func(t *testing.T) {
		t.Parallel()
		// Valid expense text will call saveExpense which panics with nil bot
		defer func() {
			if recover() == nil {
				t.Error("expected panic when calling saveExpense with nil bot")
			}
		}()
		b.handleFreeTextExpense(ctx, tgBot, &models.Update{
			Message: &models.Message{
				Text: "5.50 Coffee",
				Chat: models.Chat{ID: 12345},
				From: &models.User{ID: userID},
			},
		})
	})

	t.Run("saveExpense wrapper", func(t *testing.T) {
		t.Parallel()
		// saveExpense has no early return, so we accept a panic
		defer func() {
			if recover() == nil {
				t.Error("expected panic with nil bot")
			}
		}()
		b.saveExpense(ctx, tgBot, 0, 0, nil, nil)
	})
}
