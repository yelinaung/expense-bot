package bot

import (
	"context"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// TestReceiptHandlerWrappers provides coverage for receipt handler wrapper functions.
// These wrappers exist only to match the telegram bot library's expected handler signature.
// These tests verify that wrappers can be called and delegate to Core functions properly.
func TestReceiptHandlerWrappers(t *testing.T) {
	t.Parallel()

	// Minimal bot instance - wrappers return early so we don't need full setup
	b := &Bot{}
	ctx := context.Background()

	// nil *bot.Bot - wrappers pass it through but Core functions return early before using it
	var tgBot *bot.Bot

	t.Run("handlePhoto wrapper - nil message", func(t *testing.T) {
		t.Parallel()
		// Update with nil Message causes early return in handlePhotoCore
		b.handlePhoto(ctx, tgBot, &models.Update{})
	})

	t.Run("handlePhoto wrapper - empty photo array", func(t *testing.T) {
		t.Parallel()
		// Update with empty photo array causes early return in handlePhotoCore
		b.handlePhoto(ctx, tgBot, &models.Update{
			Message: &models.Message{
				Photo: []models.PhotoSize{}, // Empty array
			},
		})
	})

	t.Run("handleReceiptCallback wrapper", func(t *testing.T) {
		t.Parallel()
		// Update with nil CallbackQuery causes early return in handleReceiptCallbackCore
		b.handleReceiptCallback(ctx, tgBot, &models.Update{})
	})
}
