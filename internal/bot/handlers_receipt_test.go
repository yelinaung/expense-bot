package bot

import (
	"context"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

const (
	callbackIDReceipt = "callback123"
	testReceiptText   = "Test Receipt"
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
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
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
				Data: "receipt_confirm_" + string(rune(expense.ID+'0')),
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
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
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
			Amount:      mustParseDecimal("30.00"),
			Currency:    "USD",
			Description: "US Test Receipt",
			Merchant:    "US Test Receipt",
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
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
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
		require.Contains(t, mockBot.EditedMessages[0].Text, "cancelled")

		_, err = b.expenseRepo.GetByID(ctx, expense.ID)
		require.Error(t, err)
	})
}

func TestHandleEditReceiptCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
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
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
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
			Amount:      mustParseDecimal("30.00"),
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
