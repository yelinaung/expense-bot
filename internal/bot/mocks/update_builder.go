package mocks

import (
	"github.com/go-telegram/bot/models"
)

// UpdateBuilder helps construct test Update objects.
type UpdateBuilder struct {
	update *models.Update
}

// NewUpdateBuilder creates a new UpdateBuilder.
func NewUpdateBuilder() *UpdateBuilder {
	return &UpdateBuilder{
		update: &models.Update{},
	}
}

// WithMessage sets a message on the update.
func (b *UpdateBuilder) WithMessage(chatID, userID int64, text string) *UpdateBuilder {
	b.update.Message = &models.Message{
		ID: 1,
		Chat: models.Chat{
			ID:   chatID,
			Type: "private",
		},
		From: &models.User{
			ID:        userID,
			FirstName: "Test",
			LastName:  "User",
			Username:  "testuser",
		},
		Text: text,
	}
	return b
}

// WithMessageID sets a custom message ID.
func (b *UpdateBuilder) WithMessageID(messageID int) *UpdateBuilder {
	if b.update.Message != nil {
		b.update.Message.ID = messageID
	}
	return b
}

// WithFrom sets custom user details on the message.
func (b *UpdateBuilder) WithFrom(userID int64, username, firstName, lastName string) *UpdateBuilder {
	user := &models.User{
		ID:        userID,
		Username:  username,
		FirstName: firstName,
		LastName:  lastName,
	}
	if b.update.Message != nil {
		b.update.Message.From = user
	}
	if b.update.CallbackQuery != nil {
		b.update.CallbackQuery.From = *user
	}
	return b
}

// WithCallbackQuery sets a callback query on the update.
func (b *UpdateBuilder) WithCallbackQuery(
	callbackID string,
	chatID, userID int64,
	messageID int,
	data string,
) *UpdateBuilder {
	b.update.CallbackQuery = &models.CallbackQuery{
		ID: callbackID,
		From: models.User{
			ID:        userID,
			FirstName: "Test",
			LastName:  "User",
			Username:  "testuser",
		},
		Message: models.MaybeInaccessibleMessage{
			Message: &models.Message{
				ID: messageID,
				Chat: models.Chat{
					ID:   chatID,
					Type: "private",
				},
			},
		},
		Data: data,
	}
	return b
}

// WithPhoto adds a photo to the message.
func (b *UpdateBuilder) WithPhoto(fileID string) *UpdateBuilder {
	if b.update.Message == nil {
		b.WithMessage(0, 0, "")
	}
	b.update.Message.Photo = []models.PhotoSize{
		{
			FileID:       fileID + "_small",
			FileUniqueID: fileID + "_small_unique",
			Width:        320,
			Height:       240,
		},
		{
			FileID:       fileID,
			FileUniqueID: fileID + "_unique",
			Width:        1280,
			Height:       960,
		},
	}
	return b
}

// WithDocument adds a document to the message.
func (b *UpdateBuilder) WithDocument(fileID, fileName, mimeType string) *UpdateBuilder {
	if b.update.Message == nil {
		b.WithMessage(0, 0, "")
	}
	b.update.Message.Document = &models.Document{
		FileID:       fileID,
		FileUniqueID: fileID + "_unique",
		FileName:     fileName,
		MimeType:     mimeType,
	}
	return b
}

// WithVoice adds a voice message to the update.
func (b *UpdateBuilder) WithVoice(fileID string, duration int) *UpdateBuilder {
	if b.update.Message == nil {
		b.WithMessage(0, 0, "")
	}
	b.update.Message.Voice = &models.Voice{
		FileID:       fileID,
		FileUniqueID: fileID + "_unique",
		Duration:     duration,
		MimeType:     "audio/ogg",
	}
	return b
}

// WithEditedMessage sets an edited message on the update.
func (b *UpdateBuilder) WithEditedMessage(chatID, userID int64, text string) *UpdateBuilder {
	b.update.EditedMessage = &models.Message{
		ID: 1,
		Chat: models.Chat{
			ID:   chatID,
			Type: "private",
		},
		From: &models.User{
			ID:        userID,
			FirstName: "Test",
			LastName:  "User",
			Username:  "testuser",
		},
		Text: text,
	}
	return b
}

// WithReplyToMessage sets a reply-to message.
func (b *UpdateBuilder) WithReplyToMessage(messageID int, text string) *UpdateBuilder {
	if b.update.Message == nil {
		return b
	}
	b.update.Message.ReplyToMessage = &models.Message{
		ID:   messageID,
		Text: text,
	}
	return b
}

// Build returns the constructed Update.
func (b *UpdateBuilder) Build() *models.Update {
	return b.update
}

// MessageUpdate creates a simple message update.
func MessageUpdate(chatID, userID int64, text string) *models.Update {
	return NewUpdateBuilder().
		WithMessage(chatID, userID, text).
		Build()
}

// CommandUpdate creates a command message update.
func CommandUpdate(chatID, userID int64, command string) *models.Update {
	return MessageUpdate(chatID, userID, command)
}

// CallbackQueryUpdate creates a callback query update.
func CallbackQueryUpdate(chatID, userID int64, messageID int, data string) *models.Update {
	return NewUpdateBuilder().
		WithCallbackQuery("callback-query-id", chatID, userID, messageID, data).
		Build()
}

// PhotoUpdate creates a photo message update.
func PhotoUpdate(chatID, userID int64, fileID string) *models.Update {
	return NewUpdateBuilder().
		WithMessage(chatID, userID, "").
		WithPhoto(fileID).
		Build()
}

// VoiceUpdate creates a voice message update.
func VoiceUpdate(chatID, userID int64, fileID string, duration int) *models.Update {
	return NewUpdateBuilder().
		WithMessage(chatID, userID, "").
		WithVoice(fileID, duration).
		Build()
}
