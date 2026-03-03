package bot

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/gemini"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
	"google.golang.org/genai"
)

const (
	callbackIDReceipt   = "callback123"
	testReceiptText     = "Test Receipt"
	amount30ReceiptTest = "30.00"
	usTestReceiptText   = "US Test Receipt"
)

func TestBuildReceiptConfirmationKeyboard(t *testing.T) {
	t.Parallel()

	t.Run("creates keyboard with correct buttons", func(t *testing.T) {
		t.Parallel()
		keyboard := buildReceiptConfirmationKeyboard(123)

		require.NotNil(t, keyboard)
		require.Len(t, keyboard.InlineKeyboard, 1)
		require.Len(t, keyboard.InlineKeyboard[0], 3)

		require.Equal(t, "✅ Confirm", keyboard.InlineKeyboard[0][0].Text)
		require.Equal(t, "receipt_confirm_123", keyboard.InlineKeyboard[0][0].CallbackData)

		require.Equal(t, "✏️ Edit", keyboard.InlineKeyboard[0][1].Text)
		require.Equal(t, "receipt_edit_123", keyboard.InlineKeyboard[0][1].CallbackData)

		require.Equal(t, "❌ Cancel", keyboard.InlineKeyboard[0][2].Text)
		require.Equal(t, "receipt_cancel_123", keyboard.InlineKeyboard[0][2].CallbackData)
	})
}

func TestHandleReceiptCallbackCore(t *testing.T) {
	ctx := context.Background()
	pool := TestDB(ctx, t)
	b := setupTestBot(t, pool)
	userID := int64(400001)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "receiptuser",
		FirstName: "Receipt",
	})
	require.NoError(t, err)

	t.Run("nil callback query returns early", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{CallbackQuery: nil}
		b.handleReceiptCallbackCore(ctx, mockBot, update)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("invalid callback data format returns early", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:   callbackIDReceipt,
				From: models.User{ID: userID},
				Data: "invalid",
				Message: models.MaybeInaccessibleMessage{
					Message: &models.Message{
						ID:   100,
						Chat: models.Chat{ID: 12345},
					},
				},
			},
		}
		b.handleReceiptCallbackCore(ctx, mockBot, update)
		require.Len(t, mockBot.AnsweredCallbacks, 1)
	})

	t.Run("expense not found shows error", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:   callbackIDReceipt,
				From: models.User{ID: userID},
				Data: "receipt_confirm_99999",
				Message: models.MaybeInaccessibleMessage{
					Message: &models.Message{
						ID:   100,
						Chat: models.Chat{ID: 12345},
					},
				},
			},
		}
		b.handleReceiptCallbackCore(ctx, mockBot, update)
		require.Len(t, mockBot.EditedMessages, 1)
		require.Contains(t, mockBot.EditedMessages[0].Text, "Expense not found")
	})

	t.Run("invalid expense id returns early", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:   callbackIDReceipt,
				From: models.User{ID: userID},
				Data: "receipt_confirm_not-a-number",
				Message: models.MaybeInaccessibleMessage{
					Message: &models.Message{
						ID:   100,
						Chat: models.Chat{ID: 12345},
					},
				},
			},
		}
		b.handleReceiptCallbackCore(ctx, mockBot, update)
		require.Len(t, mockBot.AnsweredCallbacks, 1)
		require.Zero(t, mockBot.EditedMessageCount())
	})

	t.Run("user mismatch returns early", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		otherUserID := userID + 100

		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        otherUserID,
			Username:  "otherreceiptuser",
			FirstName: "Other",
		})
		require.NoError(t, err)

		expense := &appmodels.Expense{
			UserID:      otherUserID,
			Amount:      mustParseDecimal("10.00"),
			Currency:    "SGD",
			Description: "Test",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:   callbackIDReceipt,
				From: models.User{ID: userID},
				Data: "receipt_confirm_" + strconv.Itoa(expense.ID),
				Message: models.MaybeInaccessibleMessage{
					Message: &models.Message{
						ID:   100,
						Chat: models.Chat{ID: 12345},
					},
				},
			},
		}
		b.handleReceiptCallbackCore(ctx, mockBot, update)
	})
}

func TestHandleConfirmReceiptCore(t *testing.T) {
	ctx := context.Background()
	pool := TestDB(ctx, t)
	b := setupTestBot(t, pool)
	userID := int64(400002)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "confirmuser",
		FirstName: "Confirm",
	})
	require.NoError(t, err)

	t.Run("confirms expense and shows success message", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("25.50"),
			Currency:    "SGD",
			Description: testReceiptText,
			Merchant:    testReceiptText,
			Status:      appmodels.ExpenseStatusDraft,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		b.handleConfirmReceiptCore(ctx, mockBot, 12345, 100, expense)

		require.Len(t, mockBot.EditedMessages, 1)
		msg := mockBot.EditedMessages[0].Text
		require.Contains(t, msg, "Expense Confirmed")
		require.Contains(t, msg, "S$25.50 SGD")
		require.Contains(t, msg, testReceiptText)

		// Verify date is formatted as "DD Mon YYYY" not raw timestamp
		require.Contains(t, msg, "🗓️ Date:")
		require.Regexp(t, `🗓️ Date: \d{2} \w{3} \d{4}`, msg, "Date should be formatted as 'DD Mon YYYY'")
		require.NotContains(t, msg, "+08", "Date should not contain timezone offset")

		updated, err := b.expenseRepo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.Equal(t, appmodels.ExpenseStatusConfirmed, updated.Status)
	})

	t.Run("uses expense currency in confirmation message", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal(amount30ReceiptTest),
			Currency:    "USD",
			Description: usTestReceiptText,
			Merchant:    usTestReceiptText,
			Status:      appmodels.ExpenseStatusDraft,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		b.handleConfirmReceiptCore(ctx, mockBot, 12345, 101, expense)

		require.Len(t, mockBot.EditedMessages, 1)
		msg := mockBot.EditedMessages[0].Text
		require.Contains(t, msg, "$30.00 USD")
	})
}

func TestHandleCancelReceiptCore(t *testing.T) {
	ctx := context.Background()
	pool := TestDB(ctx, t)
	b := setupTestBot(t, pool)
	userID := int64(400003)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "canceluser",
		FirstName: "Cancel",
	})
	require.NoError(t, err)

	t.Run("deletes expense and shows cancellation message", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("15.00"),
			Currency:    "SGD",
			Description: "To Cancel",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		b.handleCancelReceiptCore(ctx, mockBot, 12345, 100, expense)

		require.Len(t, mockBot.EditedMessages, 1)
		require.Contains(t, mockBot.EditedMessages[0].Text, "canceled")

		_, err = b.expenseRepo.GetByID(ctx, expense.ID)
		require.Error(t, err)
	})
}

func TestHandleEditReceiptCore(t *testing.T) {
	ctx := context.Background()
	pool := TestDB(ctx, t)
	b := setupTestBot(t, pool)
	userID := int64(400004)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "edituser",
		FirstName: "Edit",
	})
	require.NoError(t, err)

	t.Run("shows edit options", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("20.00"),
			Currency:    "SGD",
			Description: "To Edit",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		b.handleEditReceiptCore(ctx, mockBot, 12345, 100, expense)

		require.Len(t, mockBot.EditedMessages, 1)
		require.Contains(t, mockBot.EditedMessages[0].Text, "Edit Expense")
		require.Contains(t, mockBot.EditedMessages[0].Text, "$20.00 SGD")
		require.NotNil(t, mockBot.EditedMessages[0].ReplyMarkup)
	})
}

func TestHandleBackToReceiptCore(t *testing.T) {
	ctx := context.Background()
	pool := TestDB(ctx, t)
	b := setupTestBot(t, pool)
	userID := int64(400005)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "backuser",
		FirstName: "Back",
	})
	require.NoError(t, err)

	t.Run("shows receipt confirmation view", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal(amount30ReceiptTest),
			Currency:    "SGD",
			Description: "Back Test",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		b.handleBackToReceiptCore(ctx, mockBot, 12345, 100, expense)

		require.Len(t, mockBot.EditedMessages, 1)
		require.Contains(t, mockBot.EditedMessages[0].Text, "Receipt Scanned")
		require.Contains(t, mockBot.EditedMessages[0].Text, "$30.00 SGD")
		require.NotNil(t, mockBot.EditedMessages[0].ReplyMarkup)
	})

	t.Run("shows category when set", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		categories, err := b.categoryRepo.GetAll(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, categories)

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("35.00"),
			Currency:    "SGD",
			Description: "With Category",
			CategoryID:  &categories[0].ID,
			Category:    &categories[0],
			Status:      appmodels.ExpenseStatusDraft,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		b.handleBackToReceiptCore(ctx, mockBot, 12345, 100, expense)

		require.Len(t, mockBot.EditedMessages, 1)
		require.Contains(t, mockBot.EditedMessages[0].Text, categories[0].Name)
	})
}

type receiptRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f receiptRoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestHandlePhotoCore_DownloadError(t *testing.T) {
	t.Parallel()

	b := &Bot{
		geminiClient: gemini.NewClientWithGenerator(&botTestGenerator{}),
	}
	mockBot := mocks.NewMockBot()
	mockBot.GetFileError = errors.New("get file failed")

	update := mocks.PhotoUpdate(12345, 100, "photo-file-id")

	b.handlePhotoCore(context.Background(), mockBot, update)

	require.Equal(t, 2, mockBot.SentMessageCount())
	require.Contains(t, mockBot.SentMessages[0].Text, "Processing receipt")
	require.Contains(t, mockBot.SentMessages[1].Text, "Failed to download photo")
}

func TestHandlePhotoCore_ParseError(t *testing.T) {
	t.Parallel()

	b := &Bot{
		geminiClient: gemini.NewClientWithGenerator(&botTestGenerator{
			err: errors.New("parse failed"),
		}),
		httpClient: &http.Client{
			Transport: receiptRoundTripperFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("fake-image-bytes")),
					Header:     make(http.Header),
				}, nil
			}),
		},
	}
	mockBot := mocks.NewMockBot()
	update := mocks.PhotoUpdate(12345, 100, "photo-file-id")

	b.handlePhotoCore(context.Background(), mockBot, update)

	require.Equal(t, 2, mockBot.SentMessageCount())
	require.Contains(t, mockBot.SentMessages[0].Text, "Processing receipt")
	require.Contains(t, mockBot.SentMessages[1].Text, "Could not read this receipt")
}

func TestHandlePhotoCore_Success(t *testing.T) {
	ctx := context.Background()
	pool := TestDB(ctx, t)
	b := setupTestBot(t, pool)
	require.NoError(t, b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        100,
		Username:  "photo-success-user",
		FirstName: "Photo",
	}))
	b.geminiClient = gemini.NewClientWithGenerator(&botTestGenerator{
		response: &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Parts: []*genai.Part{
							{
								Text: `{"amount":"12.50","currency":"SGD","merchant":"Cafe","date":"2026-02-26","suggested_category":"Food - Dining Out","confidence":0.95}`,
							},
						},
					},
				},
			},
		},
	})
	b.httpClient = &http.Client{
		Transport: receiptRoundTripperFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("fake-image-bytes")),
				Header:     make(http.Header),
			}, nil
		}),
	}
	mockBot := mocks.NewMockBot()
	update := mocks.PhotoUpdate(12345, 100, "photo-file-id")

	b.handlePhotoCore(ctx, mockBot, update)

	require.Equal(t, 2, mockBot.SentMessageCount())
	require.Contains(t, mockBot.SentMessages[0].Text, "Processing receipt")
	require.Contains(t, mockBot.SentMessages[1].Text, "Receipt Scanned")
}
