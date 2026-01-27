package mocks

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateBuilder_WithMessage(t *testing.T) {
	t.Parallel()

	update := NewUpdateBuilder().
		WithMessage(12345, 67890, "Hello").
		Build()

	require.NotNil(t, update.Message)
	require.Equal(t, int64(12345), update.Message.Chat.ID)
	require.Equal(t, int64(67890), update.Message.From.ID)
	require.Equal(t, "Hello", update.Message.Text)
	require.Equal(t, "testuser", update.Message.From.Username)
}

func TestUpdateBuilder_WithMessageID(t *testing.T) {
	t.Parallel()

	update := NewUpdateBuilder().
		WithMessage(1, 2, "text").
		WithMessageID(999).
		Build()

	require.Equal(t, 999, update.Message.ID)
}

func TestUpdateBuilder_WithFrom(t *testing.T) {
	t.Parallel()

	t.Run("on message", func(t *testing.T) {
		t.Parallel()

		update := NewUpdateBuilder().
			WithMessage(1, 2, "text").
			WithFrom(100, "alice", "Alice", "Smith").
			Build()

		require.Equal(t, int64(100), update.Message.From.ID)
		require.Equal(t, "alice", update.Message.From.Username)
		require.Equal(t, "Alice", update.Message.From.FirstName)
		require.Equal(t, "Smith", update.Message.From.LastName)
	})

	t.Run("on callback query", func(t *testing.T) {
		t.Parallel()

		update := NewUpdateBuilder().
			WithCallbackQuery("cb-1", 1, 2, 3, "data").
			WithFrom(200, "bob", "Bob", "Jones").
			Build()

		require.Equal(t, int64(200), update.CallbackQuery.From.ID)
		require.Equal(t, "bob", update.CallbackQuery.From.Username)
	})
}

func TestUpdateBuilder_WithCallbackQuery(t *testing.T) {
	t.Parallel()

	update := NewUpdateBuilder().
		WithCallbackQuery("callback-123", 100, 200, 50, "action_data").
		Build()

	require.NotNil(t, update.CallbackQuery)
	require.Equal(t, "callback-123", update.CallbackQuery.ID)
	require.Equal(t, int64(200), update.CallbackQuery.From.ID)
	require.Equal(t, "action_data", update.CallbackQuery.Data)

	msg := update.CallbackQuery.Message.Message
	require.NotNil(t, msg)
	require.Equal(t, 50, msg.ID)
	require.Equal(t, int64(100), msg.Chat.ID)
}

func TestUpdateBuilder_WithPhoto(t *testing.T) {
	t.Parallel()

	update := NewUpdateBuilder().
		WithMessage(1, 2, "").
		WithPhoto("photo-file-id").
		Build()

	require.Len(t, update.Message.Photo, 2)
	require.Equal(t, "photo-file-id_small", update.Message.Photo[0].FileID)
	require.Equal(t, "photo-file-id", update.Message.Photo[1].FileID)
	require.Equal(t, 1280, update.Message.Photo[1].Width)
}

func TestUpdateBuilder_WithPhoto_CreatesMessage(t *testing.T) {
	t.Parallel()

	update := NewUpdateBuilder().
		WithPhoto("photo-id").
		Build()

	require.NotNil(t, update.Message)
	require.Len(t, update.Message.Photo, 2)
}

func TestUpdateBuilder_WithDocument(t *testing.T) {
	t.Parallel()

	update := NewUpdateBuilder().
		WithMessage(1, 2, "").
		WithDocument("doc-id", "receipt.pdf", "application/pdf").
		Build()

	require.NotNil(t, update.Message.Document)
	require.Equal(t, "doc-id", update.Message.Document.FileID)
	require.Equal(t, "receipt.pdf", update.Message.Document.FileName)
	require.Equal(t, "application/pdf", update.Message.Document.MimeType)
}

func TestUpdateBuilder_WithDocument_CreatesMessage(t *testing.T) {
	t.Parallel()

	update := NewUpdateBuilder().
		WithDocument("doc-id", "file.pdf", "application/pdf").
		Build()

	require.NotNil(t, update.Message)
	require.NotNil(t, update.Message.Document)
}

func TestUpdateBuilder_WithEditedMessage(t *testing.T) {
	t.Parallel()

	update := NewUpdateBuilder().
		WithEditedMessage(100, 200, "Edited text").
		Build()

	require.NotNil(t, update.EditedMessage)
	require.Equal(t, int64(100), update.EditedMessage.Chat.ID)
	require.Equal(t, int64(200), update.EditedMessage.From.ID)
	require.Equal(t, "Edited text", update.EditedMessage.Text)
}

func TestUpdateBuilder_WithReplyToMessage(t *testing.T) {
	t.Parallel()

	update := NewUpdateBuilder().
		WithMessage(1, 2, "reply").
		WithReplyToMessage(99, "Original message").
		Build()

	require.NotNil(t, update.Message.ReplyToMessage)
	require.Equal(t, 99, update.Message.ReplyToMessage.ID)
	require.Equal(t, "Original message", update.Message.ReplyToMessage.Text)
}

func TestUpdateBuilder_WithReplyToMessage_NoMessage(t *testing.T) {
	t.Parallel()

	update := NewUpdateBuilder().
		WithReplyToMessage(99, "Original").
		Build()

	require.Nil(t, update.Message)
}

func TestMessageUpdate(t *testing.T) {
	t.Parallel()

	update := MessageUpdate(111, 222, "Test message")

	require.NotNil(t, update.Message)
	require.Equal(t, int64(111), update.Message.Chat.ID)
	require.Equal(t, int64(222), update.Message.From.ID)
	require.Equal(t, "Test message", update.Message.Text)
}

func TestCommandUpdate(t *testing.T) {
	t.Parallel()

	update := CommandUpdate(1, 2, "/start")

	require.NotNil(t, update.Message)
	require.Equal(t, "/start", update.Message.Text)
}

func TestCallbackQueryUpdate(t *testing.T) {
	t.Parallel()

	update := CallbackQueryUpdate(100, 200, 50, "receipt_confirm_123")

	require.NotNil(t, update.CallbackQuery)
	require.Equal(t, int64(200), update.CallbackQuery.From.ID)
	require.Equal(t, 50, update.CallbackQuery.Message.Message.ID)
	require.Equal(t, "receipt_confirm_123", update.CallbackQuery.Data)
}

func TestPhotoUpdate(t *testing.T) {
	t.Parallel()

	update := PhotoUpdate(100, 200, "photo-123")

	require.NotNil(t, update.Message)
	require.Equal(t, int64(100), update.Message.Chat.ID)
	require.Equal(t, int64(200), update.Message.From.ID)
	require.Len(t, update.Message.Photo, 2)
	require.Equal(t, "photo-123", update.Message.Photo[1].FileID)
}
