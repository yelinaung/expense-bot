package bot

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/gemini"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
	"google.golang.org/genai"
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

type voiceRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f voiceRoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestHandleVoiceCore_DownloadError(t *testing.T) {
	t.Parallel()

	b := &Bot{
		geminiClient: gemini.NewClientWithGenerator(&botTestGenerator{}),
	}
	mockBot := mocks.NewMockBot()
	mockBot.GetFileError = errors.New("get file failed")
	update := mocks.VoiceUpdate(12345, 100, "voice-file-id", 5)

	b.handleVoiceCore(context.Background(), mockBot, update)

	require.Equal(t, 2, mockBot.SentMessageCount())
	require.Contains(t, mockBot.SentMessages[0].Text, "Processing voice message")
	require.Contains(t, mockBot.SentMessages[1].Text, "Failed to download voice message")
}

func TestHandleVoiceCore_ParseError(t *testing.T) {
	t.Parallel()

	b := &Bot{
		geminiClient: gemini.NewClientWithGenerator(&botTestGenerator{
			err: errors.New("voice parse failed"),
		}),
		categoryCache: []appmodels.Category{
			{ID: 1, Name: "Food"},
		},
		categoryCacheExpiry: time.Now().Add(time.Hour),
	}
	b.httpClient = &http.Client{
		Transport: voiceRoundTripperFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("fake-audio-bytes")),
				Header:     make(http.Header),
			}, nil
		}),
	}
	mockBot := mocks.NewMockBot()
	update := mocks.VoiceUpdate(12345, 100, "voice-file-id", 5)

	b.handleVoiceCore(context.Background(), mockBot, update)

	require.Equal(t, 2, mockBot.SentMessageCount())
	require.Contains(t, mockBot.SentMessages[0].Text, "Processing voice message")
	require.Contains(t, mockBot.SentMessages[1].Text, "Failed to process voice message")
}

func TestHandleVoiceCore_Success(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)
	require.NoError(t, b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        100,
		Username:  "voice-success-user",
		FirstName: "Voice",
	}))
	b.geminiClient = gemini.NewClientWithGenerator(&botTestGenerator{
		response: &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Parts: []*genai.Part{
							{
								Text: `{"amount":"8.75","currency":"SGD","description":"Taxi ride","suggested_category":"Transportation","confidence":0.9}`,
							},
						},
					},
				},
			},
		},
	})
	b.httpClient = &http.Client{
		Transport: voiceRoundTripperFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("fake-audio-bytes")),
				Header:     make(http.Header),
			}, nil
		}),
	}
	mockBot := mocks.NewMockBot()
	update := mocks.VoiceUpdate(12345, 100, "voice-file-id", 5)

	b.handleVoiceCore(ctx, mockBot, update)

	require.Equal(t, 2, mockBot.SentMessageCount())
	require.Contains(t, mockBot.SentMessages[0].Text, "Processing voice message")
	require.Contains(t, mockBot.SentMessages[1].Text, "Voice Expense Detected")
}
