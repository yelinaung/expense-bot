package mocks

import (
	"context"
	"errors"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
)

func TestMockBot_SendMessage(t *testing.T) {
	t.Parallel()

	t.Run("captures sent message", func(t *testing.T) {
		t.Parallel()

		mockBot := NewMockBot()
		ctx := context.Background()

		msg, err := mockBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    int64(12345),
			Text:      "Hello, World!",
			ParseMode: models.ParseModeHTML,
		})

		require.NoError(t, err)
		require.NotNil(t, msg)
		require.Equal(t, 1000, msg.ID)
		require.Equal(t, int64(12345), msg.Chat.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		last := mockBot.LastSentMessage()
		require.NotNil(t, last)
		require.Equal(t, int64(12345), last.ChatID)
		require.Equal(t, "Hello, World!", last.Text)
		require.Equal(t, models.ParseModeHTML, last.ParseMode)
	})

	t.Run("returns error when configured", func(t *testing.T) {
		t.Parallel()

		mockBot := NewMockBot()
		mockBot.SendMessageError = errors.New("send failed")

		_, err := mockBot.SendMessage(context.Background(), &bot.SendMessageParams{
			ChatID: int64(123),
			Text:   "test",
		})

		require.Error(t, err)
		require.Equal(t, "send failed", err.Error())
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("increments message ID", func(t *testing.T) {
		t.Parallel()

		mockBot := NewMockBot()
		ctx := context.Background()

		msg1, err := mockBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: int64(1), Text: "a"})
		require.NoError(t, err)
		msg2, err := mockBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: int64(1), Text: "b"})
		require.NoError(t, err)

		require.Equal(t, 1000, msg1.ID)
		require.Equal(t, 1001, msg2.ID)
	})
}

func TestMockBot_EditMessageText(t *testing.T) {
	t.Parallel()

	t.Run("captures edited message", func(t *testing.T) {
		t.Parallel()

		mockBot := NewMockBot()
		ctx := context.Background()

		keyboard := &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{{Text: "Button", CallbackData: "data"}},
			},
		}

		msg, err := mockBot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      int64(12345),
			MessageID:   100,
			Text:        "Updated text",
			ParseMode:   models.ParseModeHTML,
			ReplyMarkup: keyboard,
		})

		require.NoError(t, err)
		require.NotNil(t, msg)
		require.Equal(t, 100, msg.ID)

		last := mockBot.LastEditedMessage()
		require.NotNil(t, last)
		require.Equal(t, int64(12345), last.ChatID)
		require.Equal(t, 100, last.MessageID)
		require.Equal(t, "Updated text", last.Text)
		require.NotNil(t, last.ReplyMarkup)
	})

	t.Run("returns error when configured", func(t *testing.T) {
		t.Parallel()

		mockBot := NewMockBot()
		mockBot.EditMessageError = errors.New("edit failed")

		_, err := mockBot.EditMessageText(context.Background(), &bot.EditMessageTextParams{
			ChatID:    int64(123),
			MessageID: 1,
			Text:      "test",
		})

		require.Error(t, err)
	})
}

func TestMockBot_AnswerCallbackQuery(t *testing.T) {
	t.Parallel()

	mockBot := NewMockBot()

	ok, err := mockBot.AnswerCallbackQuery(context.Background(), &bot.AnswerCallbackQueryParams{
		CallbackQueryID: "query-123",
		Text:            "Done!",
		ShowAlert:       true,
	})

	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, mockBot.AnsweredCallbacks, 1)
	require.Equal(t, "query-123", mockBot.AnsweredCallbacks[0].CallbackQueryID)
	require.Equal(t, "Done!", mockBot.AnsweredCallbacks[0].Text)
	require.True(t, mockBot.AnsweredCallbacks[0].ShowAlert)
}

func TestMockBot_GetFile(t *testing.T) {
	t.Parallel()

	t.Run("returns default file", func(t *testing.T) {
		t.Parallel()

		mockBot := NewMockBot()

		file, err := mockBot.GetFile(context.Background(), &bot.GetFileParams{
			FileID: "some-file",
		})

		require.NoError(t, err)
		require.NotNil(t, file)
		require.Equal(t, "test-file-id", file.FileID)
	})

	t.Run("returns configured file", func(t *testing.T) {
		t.Parallel()

		mockBot := NewMockBot()
		mockBot.FileToReturn = &models.File{
			FileID:   "custom-id",
			FilePath: "custom/path.jpg",
		}

		file, err := mockBot.GetFile(context.Background(), nil)

		require.NoError(t, err)
		require.Equal(t, "custom-id", file.FileID)
	})

	t.Run("returns error when configured", func(t *testing.T) {
		t.Parallel()

		mockBot := NewMockBot()
		mockBot.GetFileError = errors.New("file not found")

		_, err := mockBot.GetFile(context.Background(), nil)

		require.Error(t, err)
	})
}

func TestMockBot_FileDownloadLink(t *testing.T) {
	t.Parallel()

	t.Run("returns default link", func(t *testing.T) {
		t.Parallel()

		mockBot := NewMockBot()
		link := mockBot.FileDownloadLink(nil)

		require.Contains(t, link, "api.telegram.org")
	})

	t.Run("returns configured link", func(t *testing.T) {
		t.Parallel()

		mockBot := NewMockBot()
		mockBot.FileDownloadLinkToReturn = "https://example.com/file.jpg"

		link := mockBot.FileDownloadLink(nil)

		require.Equal(t, "https://example.com/file.jpg", link)
	})
}

func TestMockBot_Reset(t *testing.T) {
	t.Parallel()

	mockBot := NewMockBot()
	ctx := context.Background()

	_, _ = mockBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: int64(1), Text: "a"})
	_, _ = mockBot.EditMessageText(ctx, &bot.EditMessageTextParams{ChatID: int64(1), MessageID: 1, Text: "b"})
	mockBot.SendMessageError = errors.New("error")

	mockBot.Reset()

	require.Empty(t, mockBot.SentMessages)
	require.Empty(t, mockBot.EditedMessages)
	require.NoError(t, mockBot.SendMessageError)
}

func TestMockBot_LastSentMessage_Empty(t *testing.T) {
	t.Parallel()

	mockBot := NewMockBot()
	require.Nil(t, mockBot.LastSentMessage())
}

func TestMockBot_LastEditedMessage_Empty(t *testing.T) {
	t.Parallel()

	mockBot := NewMockBot()
	require.Nil(t, mockBot.LastEditedMessage())
}

func TestChatIDToInt64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		expected int64
	}{
		{"int64", int64(12345), 12345},
		{"int", 12345, 12345},
		{"string", "12345", 0},
		{"nil", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := chatIDToInt64(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}
