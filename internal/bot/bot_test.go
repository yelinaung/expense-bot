package bot

import (
	"testing"

	tgmodels "github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
)

func TestExtractUserID(t *testing.T) {
	t.Parallel()

	t.Run("extracts from message", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			Message: &tgmodels.Message{
				From: &tgmodels.User{ID: 12345},
			},
		}
		require.Equal(t, int64(12345), extractUserID(update))
	})

	t.Run("extracts from callback query", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			CallbackQuery: &tgmodels.CallbackQuery{
				From: tgmodels.User{ID: 67890},
			},
		}
		require.Equal(t, int64(67890), extractUserID(update))
	})

	t.Run("extracts from edited message", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			EditedMessage: &tgmodels.Message{
				From: &tgmodels.User{ID: 11111},
			},
		}
		require.Equal(t, int64(11111), extractUserID(update))
	})

	t.Run("returns zero for empty update", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{}
		require.Equal(t, int64(0), extractUserID(update))
	})

	t.Run("returns zero for message without from", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			Message: &tgmodels.Message{From: nil},
		}
		require.Equal(t, int64(0), extractUserID(update))
	})
}

func TestExtractUsername(t *testing.T) {
	t.Parallel()

	t.Run("extracts from message", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			Message: &tgmodels.Message{
				From: &tgmodels.User{Username: "testuser"},
			},
		}
		require.Equal(t, "testuser", extractUsername(update))
	})

	t.Run("extracts from callback query", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			CallbackQuery: &tgmodels.CallbackQuery{
				From: tgmodels.User{Username: "callbackuser"},
			},
		}
		require.Equal(t, "callbackuser", extractUsername(update))
	})

	t.Run("extracts from edited message", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			EditedMessage: &tgmodels.Message{
				From: &tgmodels.User{Username: "edituser"},
			},
		}
		require.Equal(t, "edituser", extractUsername(update))
	})

	t.Run("returns empty for empty update", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{}
		require.Equal(t, "", extractUsername(update))
	})

	t.Run("returns empty for message without from", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			Message: &tgmodels.Message{From: nil},
		}
		require.Equal(t, "", extractUsername(update))
	})
}

func TestLogUserAction(t *testing.T) {
	t.Parallel()

	t.Run("logs message with text", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			Message: &tgmodels.Message{
				Text: "hello",
				Chat: tgmodels.Chat{ID: 123},
			},
		}
		// Should not panic.
		logUserAction(123, "user", update)
	})

	t.Run("logs message with photo", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			Message: &tgmodels.Message{
				Photo: []tgmodels.PhotoSize{{FileID: "abc"}},
				Chat:  tgmodels.Chat{ID: 123},
			},
		}
		logUserAction(123, "user", update)
	})

	t.Run("logs message with document", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			Message: &tgmodels.Message{
				Document: &tgmodels.Document{FileName: "test.pdf"},
				Chat:     tgmodels.Chat{ID: 123},
			},
		}
		logUserAction(123, "user", update)
	})

	t.Run("logs callback query", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			CallbackQuery: &tgmodels.CallbackQuery{
				Data: "button_click",
			},
		}
		logUserAction(123, "user", update)
	})

	t.Run("logs edited message", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			EditedMessage: &tgmodels.Message{
				Text: "edited text",
			},
		}
		logUserAction(123, "user", update)
	})

	t.Run("handles empty update", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{}
		logUserAction(123, "user", update)
	})
}
