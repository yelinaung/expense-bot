// Package mocks provides mock implementations for testing bot handlers.
package mocks

import (
	"context"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// SentMessage captures a message sent via MockBot.
type SentMessage struct {
	ChatID    any
	Text      string
	ParseMode models.ParseMode
}

// EditedMessage captures an edited message via MockBot.
type EditedMessage struct {
	ChatID      any
	MessageID   int
	Text        string
	ParseMode   models.ParseMode
	ReplyMarkup models.ReplyMarkup
}

// AnsweredCallback captures a callback query answer via MockBot.
type AnsweredCallback struct {
	CallbackQueryID string
	Text            string
	ShowAlert       bool
}

// MockBot simulates Telegram bot operations for testing.
type MockBot struct {
	mu sync.RWMutex

	SentMessages      []SentMessage
	EditedMessages    []EditedMessage
	AnsweredCallbacks []AnsweredCallback

	// SendMessageError allows simulating SendMessage failures.
	SendMessageError error
	// EditMessageError allows simulating EditMessageText failures.
	EditMessageError error
	// GetFileError allows simulating GetFile failures.
	GetFileError error

	// FileToReturn is returned by GetFile.
	FileToReturn *models.File
	// FileDownloadLinkToReturn is returned by FileDownloadLink.
	FileDownloadLinkToReturn string

	// NextMessageID is auto-incremented for each sent message.
	NextMessageID int
}

// NewMockBot creates a new MockBot instance.
func NewMockBot() *MockBot {
	return &MockBot{
		SentMessages:      make([]SentMessage, 0),
		EditedMessages:    make([]EditedMessage, 0),
		AnsweredCallbacks: make([]AnsweredCallback, 0),
		NextMessageID:     1000,
	}
}

// SendMessage simulates sending a message.
func (m *MockBot) SendMessage(_ context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.SendMessageError != nil {
		return nil, m.SendMessageError
	}

	m.SentMessages = append(m.SentMessages, SentMessage{
		ChatID:    params.ChatID,
		Text:      params.Text,
		ParseMode: params.ParseMode,
	})

	msgID := m.NextMessageID
	m.NextMessageID++

	return &models.Message{
		ID: msgID,
		Chat: models.Chat{
			ID: chatIDToInt64(params.ChatID),
		},
		Text: params.Text,
	}, nil
}

// EditMessageText simulates editing a message.
func (m *MockBot) EditMessageText(_ context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.EditMessageError != nil {
		return nil, m.EditMessageError
	}

	m.EditedMessages = append(m.EditedMessages, EditedMessage{
		ChatID:      params.ChatID,
		MessageID:   params.MessageID,
		Text:        params.Text,
		ParseMode:   params.ParseMode,
		ReplyMarkup: params.ReplyMarkup,
	})

	return &models.Message{
		ID: params.MessageID,
		Chat: models.Chat{
			ID: chatIDToInt64(params.ChatID),
		},
		Text: params.Text,
	}, nil
}

// AnswerCallbackQuery simulates answering a callback query.
func (m *MockBot) AnswerCallbackQuery(_ context.Context, params *bot.AnswerCallbackQueryParams) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.AnsweredCallbacks = append(m.AnsweredCallbacks, AnsweredCallback{
		CallbackQueryID: params.CallbackQueryID,
		Text:            params.Text,
		ShowAlert:       params.ShowAlert,
	})

	return true, nil
}

// GetFile simulates getting file info from Telegram.
func (m *MockBot) GetFile(_ context.Context, _ *bot.GetFileParams) (*models.File, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.GetFileError != nil {
		return nil, m.GetFileError
	}

	if m.FileToReturn != nil {
		return m.FileToReturn, nil
	}

	return &models.File{
		FileID:   "test-file-id",
		FilePath: "photos/test.jpg",
	}, nil
}

// FileDownloadLink returns a mock download URL.
func (m *MockBot) FileDownloadLink(_ *models.File) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.FileDownloadLinkToReturn != "" {
		return m.FileDownloadLinkToReturn
	}

	return "https://api.telegram.org/file/bot123/photos/test.jpg"
}

// Reset clears all recorded interactions.
func (m *MockBot) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SentMessages = make([]SentMessage, 0)
	m.EditedMessages = make([]EditedMessage, 0)
	m.AnsweredCallbacks = make([]AnsweredCallback, 0)
	m.SendMessageError = nil
	m.EditMessageError = nil
	m.GetFileError = nil
}

// LastSentMessage returns the most recently sent message, or nil if none.
func (m *MockBot) LastSentMessage() *SentMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.SentMessages) == 0 {
		return nil
	}
	return &m.SentMessages[len(m.SentMessages)-1]
}

// LastEditedMessage returns the most recently edited message, or nil if none.
func (m *MockBot) LastEditedMessage() *EditedMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.EditedMessages) == 0 {
		return nil
	}
	return &m.EditedMessages[len(m.EditedMessages)-1]
}

// SentMessageCount returns the number of messages sent.
func (m *MockBot) SentMessageCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.SentMessages)
}

// chatIDToInt64 converts a ChatID to int64.
func chatIDToInt64(chatID any) int64 {
	switch v := chatID.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		return 0
	default:
		return 0
	}
}
