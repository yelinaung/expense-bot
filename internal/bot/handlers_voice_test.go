package bot

import (
	"context"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
)

func TestHandleVoiceCore(t *testing.T) {
	t.Parallel()

	t.Run("nil message returns early", func(t *testing.T) {
		t.Parallel()
		b := &Bot{}
		mockBot := mocks.NewMockBot()
		update := &models.Update{Message: nil}
		b.handleVoiceCore(context.Background(), mockBot, update)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("nil voice returns early", func(t *testing.T) {
		t.Parallel()
		b := &Bot{}
		mockBot := mocks.NewMockBot()
		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: 12345},
				From: &models.User{ID: 100},
			},
		}
		b.handleVoiceCore(context.Background(), mockBot, update)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("no gemini client sends error message", func(t *testing.T) {
		t.Parallel()
		b := &Bot{}
		mockBot := mocks.NewMockBot()
		update := mocks.VoiceUpdate(12345, 100, "voice-file-id", 5)
		b.handleVoiceCore(context.Background(), mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "Voice expense input is not configured")
	})
}

// TestVoiceHandlerWrappers provides coverage for voice handler wrapper functions.
func TestVoiceHandlerWrappers(t *testing.T) {
	t.Parallel()

	b := &Bot{}
	ctx := context.Background()
	var tgBot *bot.Bot

	t.Run("handleVoice wrapper - nil message", func(t *testing.T) {
		t.Parallel()
		b.handleVoice(ctx, tgBot, &models.Update{})
	})

	t.Run("handleVoice wrapper - nil voice", func(t *testing.T) {
		t.Parallel()
		b.handleVoice(ctx, tgBot, &models.Update{
			Message: &models.Message{
				Voice: nil,
			},
		})
	})
}
