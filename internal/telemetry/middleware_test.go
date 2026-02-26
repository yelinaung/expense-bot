package telemetry

import (
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/logger"
	"go.opentelemetry.io/otel/attribute"
)

func attrsToMap(attrs []attribute.KeyValue) map[string]string {
	out := make(map[string]string, len(attrs))
	for _, kv := range attrs {
		out[string(kv.Key)] = kv.Value.AsString()
	}
	return out
}

func TestClassifyUpdate(t *testing.T) {
	t.Parallel()

	t.Run("classifies command and strips mention", func(t *testing.T) {
		t.Parallel()
		update := &models.Update{
			Message: &models.Message{
				Text: "/add@mybot 10 coffee",
			},
		}
		require.Equal(t, "telegram.command /add", classifyUpdate(update))
	})

	t.Run("classifies callback prefix", func(t *testing.T) {
		t.Parallel()
		update := &models.Update{
			CallbackQuery: &models.CallbackQuery{Data: "receipt_confirm_123"},
		}
		require.Equal(t, "telegram.callback receipt_confirm", classifyUpdate(update))
	})
}

func TestUpdateAttributes(t *testing.T) {
	t.Parallel()
	logger.InitHashSaltForTesting("test-salt-for-telemetry-attributes-1234567890")

	t.Run("extracts callback user and chat attributes", func(t *testing.T) {
		t.Parallel()
		update := &models.Update{
			CallbackQuery: &models.CallbackQuery{
				From: models.User{ID: 42},
				Message: models.MaybeInaccessibleMessage{
					Message: &models.Message{
						Chat: models.Chat{ID: 99},
					},
				},
			},
		}

		attrs := attrsToMap(updateAttributes(update))
		require.Equal(t, "telegram", attrs["messaging.system"])
		require.Equal(t, logger.HashUserID(42), attrs["telegram.user_id"])
		require.Equal(t, logger.HashChatID(99), attrs["telegram.chat_id"])
	})

	t.Run("extracts message user and chat attributes", func(t *testing.T) {
		t.Parallel()
		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: 88},
				From: &models.User{ID: 77},
			},
		}

		attrs := attrsToMap(updateAttributes(update))
		require.Equal(t, "telegram", attrs["messaging.system"])
		require.Equal(t, logger.HashUserID(77), attrs["telegram.user_id"])
		require.Equal(t, logger.HashChatID(88), attrs["telegram.chat_id"])
	})

	t.Run("extracts edited message user and chat attributes", func(t *testing.T) {
		t.Parallel()
		update := &models.Update{
			EditedMessage: &models.Message{
				Chat: models.Chat{ID: 66},
				From: &models.User{ID: 55},
			},
		}

		attrs := attrsToMap(updateAttributes(update))
		require.Equal(t, "telegram", attrs["messaging.system"])
		require.Equal(t, logger.HashUserID(55), attrs["telegram.user_id"])
		require.Equal(t, logger.HashChatID(66), attrs["telegram.chat_id"])
	})
}
