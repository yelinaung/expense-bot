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
