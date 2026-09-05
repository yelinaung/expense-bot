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
	"github.com/shopspring/decimal"
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
	pool := testDB(ctx, t)
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
		require.Contains(t, mockBot.EditedMessages[0].Text, "no longer available")
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
	pool := testDB(ctx, t)
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
	pool := testDB(ctx, t)
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

	t.Run("does not delete a confirmed expense", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("20.00"),
			Currency:    "SGD",
			Description: "Already Saved",
			Status:      appmodels.ExpenseStatusConfirmed,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		b.handleCancelReceiptCore(ctx, mockBot, 12345, 100, expense)

		require.Len(t, mockBot.EditedMessages, 1)
		require.Contains(t, mockBot.EditedMessages[0].Text, "already saved")

		_, err = b.expenseRepo.GetByID(ctx, expense.ID)
		require.NoError(t, err, "confirmed expense must not be deleted on cancel")
	})
}

func TestHandleEditReceiptCore(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
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
	pool := testDB(ctx, t)
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

	update := mocks.PhotoUpdate(12345, 100, testPhotoFileID)

	b.handlePhotoCore(context.Background(), mockBot, update)

	require.Equal(t, 2, mockBot.SentMessageCount())
	require.Contains(t, mockBot.SentMessages[0].Text, testProcessingReceiptText)
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
	update := mocks.PhotoUpdate(12345, 100, testPhotoFileID)

	b.handlePhotoCore(context.Background(), mockBot, update)

	require.Equal(t, 2, mockBot.SentMessageCount())
	require.Contains(t, mockBot.SentMessages[0].Text, testProcessingReceiptText)
	require.Contains(t, mockBot.SentMessages[1].Text, "Could not read this receipt")
}

func TestHandlePhotoCore_Success(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
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
	update := mocks.PhotoUpdate(12345, 100, testPhotoFileID)

	b.handlePhotoCore(ctx, mockBot, update)

	require.Equal(t, 2, mockBot.SentMessageCount())
	require.Contains(t, mockBot.SentMessages[0].Text, testProcessingReceiptText)
	require.Contains(t, mockBot.SentMessages[1].Text, "Receipt Scanned")
}

// TestHandleConfirmReceiptCore_RejectsNonPositiveAmount verifies the positivity
// guard added to handleConfirmReceiptCore. A partial-extraction draft may carry
// a zero amount, and a draft may also hold a negative value; both must be
// rejected at confirm time (mirroring parseAmount's "amount > 0" invariant)
// rather than promoted to confirmed.
func TestHandleConfirmReceiptCore_RejectsNonPositiveAmount(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)
	userID := int64(400010)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "nonpositive-confirm-user",
		FirstName: "NonPositive",
	})
	require.NoError(t, err)

	cases := []struct {
		name   string
		amount string
	}{
		{name: "zero amount", amount: "0"},
		{name: "negative amount", amount: "-5.00"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockBot := mocks.NewMockBot()

			expense := &appmodels.Expense{
				UserID:      userID,
				Amount:      mustParseDecimal(tc.amount),
				Currency:    "SGD",
				Description: testReceiptText,
				Merchant:    testReceiptText,
				Status:      appmodels.ExpenseStatusDraft,
			}
			require.NoError(t, b.expenseRepo.Create(ctx, expense))

			b.handleConfirmReceiptCore(ctx, mockBot, 12345, 100, expense)

			// Rejection is a new message so the editable draft and its inline
			// keyboard stay intact, not an edit replacing the confirmation.
			require.Equal(t, 0, mockBot.EditedMessageCount(),
				"no edit should replace the editable draft confirmation")
			require.Equal(t, 1, mockBot.SentMessageCount())
			require.Contains(t, mockBot.SentMessages[0].Text, "Amount must be greater than zero")

			// The draft must remain unchanged in the database.
			updated, err := b.expenseRepo.GetByID(ctx, expense.ID)
			require.NoError(t, err)
			require.Equal(t, appmodels.ExpenseStatusDraft, updated.Status,
				"non-positive draft must not be promoted to confirmed")

			// Confirmed-only readers must not surface the rejected draft.
			confirmed, err := b.expenseRepo.GetByUserID(ctx, userID, 50)
			require.NoError(t, err)
			for i := range confirmed {
				require.NotEqual(t, expense.ID, confirmed[i].ID,
					"non-positive expense must not leak into confirmed read path")
			}
		})
	}
}

// TestHandlePhotoCore_ZeroAmountReceiptBug is the end-to-end regression test
// for the partial-extraction confirm path. Gemini returns merchant but not the
// total (amount "0"), so handlePhotoCore drafts a zero-amount expense, and
// confirming it must be rejected instead of persisting a confirmed 0.00 row.
func TestHandlePhotoCore_ZeroAmountReceiptBug(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)
	const (
		userID = 100
		chatID = 12345
	)

	require.NoError(t, b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "zero-receipt-bug-user",
		FirstName: "Zero",
	}))
	b.geminiClient = gemini.NewClientWithGenerator(&botTestGenerator{
		response: &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{{
				Content: &genai.Content{Parts: []*genai.Part{{
					Text: `{"amount":"0","currency":"SGD","merchant":"Cafe","date":"2026-02-26","suggested_category":"Food - Dining Out","confidence":0.4}`,
				}}},
			}},
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
	update := mocks.PhotoUpdate(chatID, userID, testPhotoFileID)

	b.handlePhotoCore(ctx, mockBot, update)

	// 1. A partial extraction (merchant present, amount absent) drafts a
	//    zero-amount expense. GetByUserID filters status='confirmed', so
	//    query directly to reach the draft row.
	require.Equal(t, 2, mockBot.SentMessageCount())
	require.Contains(t, mockBot.SentMessages[0].Text, testProcessingReceiptText)
	require.Contains(t, mockBot.SentMessages[1].Text, "Partial Extraction")

	var (
		draftID     int
		draftAmount decimal.Decimal
		draftStatus string
	)
	err := pool.QueryRow(ctx, `
		SELECT id, amount, status FROM expenses
		WHERE user_id = $1 ORDER BY id DESC LIMIT 1`,
		userID).Scan(&draftID, &draftAmount, &draftStatus)
	require.NoError(t, err, "zero-amount DRAFT should be created by handlePhotoCore")
	require.True(t, draftAmount.IsZero(), "draft amount should be zero for partial extraction")
	require.Equal(t, string(appmodels.ExpenseStatusDraft), draftStatus,
		"partial-extraction receipt should remain a DRAFT until confirmed")

	// 2. Confirming the zero-amount draft must be rejected, not promoted.
	zeroDraft, err := b.expenseRepo.GetByID(ctx, draftID)
	require.NoError(t, err)

	confirmBot := mocks.NewMockBot()
	b.handleConfirmReceiptCore(ctx, confirmBot, chatID, 100, zeroDraft)

	require.Equal(t, 0, confirmBot.EditedMessageCount(),
		"no edit should replace the editable draft confirmation")
	require.Equal(t, 1, confirmBot.SentMessageCount())
	require.Contains(t, confirmBot.SentMessages[0].Text, "Amount must be greater than zero")

	stillDraft, err := b.expenseRepo.GetByID(ctx, draftID)
	require.NoError(t, err)
	require.Equal(t, appmodels.ExpenseStatusDraft, stillDraft.Status,
		"zero-amount draft must not be promoted to CONFIRMED")

	// 3. No confirmed zero-amount expense leaks into confirmed-only readers.
	confirmed, err := b.expenseRepo.GetByUserID(ctx, userID, 50)
	require.NoError(t, err)
	for i := range confirmed {
		require.NotEqual(t, draftID, confirmed[i].ID,
			"zero-amount expense must not appear in confirmed read path (bug closed)")
	}
}
