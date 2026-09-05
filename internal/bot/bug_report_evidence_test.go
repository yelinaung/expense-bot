package bot

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

// Regression coverage for currency rendering in the bot's draft/expense edit
// sub-screens. These handlers previously hard-coded "$" and "SGD"; they must
// render via getCurrencyOrCodeSymbol(expense.Currency) + expense.Currency, the
// same pattern used by buildVoiceConfirmationText, handleConfirmReceiptCore,
// handleEditReceiptCore, and processDescriptionEditCore.

// supportedSymbol returns the expected symbol for a currency code, mirroring
// appmodels.SupportedCurrencies so the expected render can be built inline.
func supportedSymbol(code string) string {
	symbol := appmodels.SupportedCurrencies[code]
	if symbol == "" {
		return code
	}
	return symbol
}

// TestBugReportEvidence_PromptEditAmountCore_NonSGD verifies the Edit Amount
// prompt (promptEditAmountCore, handlers_callbacks.go) renders the expense's
// real currency instead of a hard-coded "$... SGD". Parameterized over
// SGD/USD/MYR/EUR to pin both halves of the bug: the symbol (S$ vs $) for SGD
// and the ISO code (USD/EUR/MYR vs SGD) for non-SGD currencies. This handler
// only mutates the in-memory pendingEdits map and edits a message, so no
// database is needed.
func TestBugReportEvidence_PromptEditAmountCore_NonSGD(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name     string
		currency string
		amount   string
	}{
		{"SGD", "SGD", "8.75"},
		{"USD", "USD", "12.40"},
		{"MYR", "MYR", "30.00"},
		{"EUR", "EUR", "5.20"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := &Bot{pendingEdits: make(map[int64]*pendingEdit)}
			mockBot := mocks.NewMockBot()

			expense := &appmodels.Expense{
				ID:       777,
				UserID:   9001,
				Amount:   mustParseDecimal(tc.amount),
				Currency: tc.currency,
				Merchant: "Test",
				Status:   appmodels.ExpenseStatusDraft,
			}

			b.promptEditAmountCore(ctx, mockBot, 4242, 100, expense)

			require.Len(t, mockBot.EditedMessages, 1)
			got := mockBot.EditedMessages[0].Text

			wantSymbol := supportedSymbol(tc.currency)
			wantLine := fmt.Sprintf("Current amount: %s%s %s", wantSymbol, tc.amount, tc.currency)
			require.Containsf(t, got, wantLine,
				"expected real currency render for %s; got %q", tc.currency, got)

			buggyLine := fmt.Sprintf("Current amount: $%s SGD", tc.amount)
			require.NotContainsf(t, got, buggyLine,
				"hard-coded $... SGD literal should be gone for %s; got %q", tc.currency, got)

			if tc.currency != "SGD" {
				require.NotContainsf(t, got, "$"+tc.amount+" SGD",
					"hard-coded SGD code should be gone for non-SGD %s; got %q", tc.currency, got)
			}
		})
	}
}

// TestBugReportEvidence_RenderChain_NonSGD walks the real draft-edit handler
// sequence for a non-SGD (MYR) expense — voice detection → edit picker → edit
// amount prompt → amount updated → set category → description updated →
// confirm — then the confirmed-expense inline-edit/inline-delete/back-to
// screens, asserting the currency renders as "RM30.00 MYR" (never "$30.00
// SGD") at every step. It also pins that the persisted expense.Currency is not
// mutated by the edits. This is the in-process equivalent of the manual UI
// smoke for the draft and confirmed-expense edit flows.
func TestBugReportEvidence_RenderChain_NonSGD(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)
	userID := int64(501007)

	require.NoError(t, b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "evidencechain",
		FirstName: "EvidenceChain",
	}))

	const (
		currency = "MYR"
		amount   = "30.00"
		symbol   = "RM"
		merchant = "Kedai"
	)

	categories, err := b.categoryRepo.GetAll(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, categories)

	expense := &appmodels.Expense{
		UserID:      userID,
		Amount:      mustParseDecimal(amount),
		Currency:    currency,
		Description: merchant,
		Merchant:    merchant,
		Status:      appmodels.ExpenseStatusDraft,
	}
	require.NoError(t, b.expenseRepo.Create(ctx, expense))

	// Step 1 — initial voice-detection screen (buildVoiceConfirmationText).
	voiceText := buildVoiceConfirmationText(expense)
	require.Contains(t, voiceText, fmt.Sprintf("💰 Amount: %s%s %s", symbol, amount, currency))
	require.NotContains(t, voiceText, fmt.Sprintf("💰 Amount: $%s SGD", amount))

	// Step 2 — "✏️ Edit" → handleEditReceiptCore picker.
	mockBot := mocks.NewMockBot()
	b.handleEditReceiptCore(ctx, mockBot, 55509, 100, expense)
	require.Len(t, mockBot.EditedMessages, 1)
	got := mockBot.LastEditedMessage().Text
	require.Contains(t, got, "Edit Expense")
	require.Contains(t, got, fmt.Sprintf("💰 Amount: %s%s %s", symbol, amount, currency))
	require.NotContains(t, got, fmt.Sprintf("💰 Amount: $%s SGD", amount))

	// Step 3 — "💰 Edit Amount" → promptEditAmountCore.
	mockBot.Reset()
	b.promptEditAmountCore(ctx, mockBot, 55510, 100, expense)
	require.Len(t, mockBot.EditedMessages, 1)
	got = mockBot.LastEditedMessage().Text
	require.Contains(t, got, fmt.Sprintf("Current amount: %s%s %s", symbol, amount, currency))
	require.NotContains(t, got, fmt.Sprintf("Current amount: $%s SGD", amount))

	// Step 4 — type a new amount → processAmountEditCore ("Amount Updated!").
	mockBot.Reset()
	pendingAmount := &pendingEdit{ExpenseID: expense.ID, EditType: editTypeAmountCB, MessageID: 100}
	require.True(t, b.processAmountEditCore(ctx, mockBot, 55511, userID, pendingAmount, "30.00"))
	require.Len(t, mockBot.EditedMessages, 1)
	got = mockBot.LastEditedMessage().Text
	require.Contains(t, got, "Amount Updated")
	require.Contains(t, got, fmt.Sprintf("💰 Amount: %s%s %s", symbol, amount, currency))
	require.NotContains(t, got, fmt.Sprintf("💰 Amount: $%s SGD", amount))

	// Step 5 — "📁 Category" → set_category_<id>_<catID> → handleSetCategoryCallbackCore.
	mockBot.Reset()
	update := &models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   "chain-set-cat",
			From: models.User{ID: userID},
			Data: fmt.Sprintf("set_category_%d_%d", expense.ID, categories[0].ID),
			Message: models.MaybeInaccessibleMessage{
				Message: &models.Message{ID: 100, Chat: models.Chat{ID: 55512}},
			},
		},
	}
	b.handleSetCategoryCallbackCore(ctx, mockBot, update)
	require.Len(t, mockBot.EditedMessages, 1)
	got = mockBot.LastEditedMessage().Text
	require.Contains(t, got, "Receipt Updated")
	require.Contains(t, got, fmt.Sprintf("💰 Amount: %s%s %s", symbol, amount, currency))
	require.NotContains(t, got, fmt.Sprintf("💰 Amount: $%s SGD", amount))

	// Step 6 — edit "📝 Description" → processDescriptionEditCore ("Description Updated!").
	mockBot.Reset()
	pendingDesc := &pendingEdit{ExpenseID: expense.ID, EditType: editTypeDescriptionCB, MessageID: 100}
	require.True(t, b.processDescriptionEditCore(ctx, mockBot, 55513, userID, pendingDesc, "Kedai Raya"))
	require.Len(t, mockBot.EditedMessages, 1)
	got = mockBot.LastEditedMessage().Text
	require.Contains(t, got, "Description Updated")
	require.Contains(t, got, fmt.Sprintf("💰 Amount: %s%s %s", symbol, amount, currency))
	require.NotContains(t, got, fmt.Sprintf("💰 Amount: $%s SGD", amount))

	// Step 7 — "✅ Confirm" → handleConfirmReceiptCore.
	mockBot.Reset()
	b.handleConfirmReceiptCore(ctx, mockBot, 55514, 100, expense)
	require.Len(t, mockBot.EditedMessages, 1)
	got = mockBot.LastEditedMessage().Text
	require.Contains(t, got, "Expense Confirmed")
	require.Contains(t, got, fmt.Sprintf("💰 Amount: %s%s %s", symbol, amount, currency))
	require.NotContains(t, got, fmt.Sprintf("💰 Amount: $%s SGD", amount))

	// Persisted expense.Currency is NOT mutated by the edits.
	refreshed, err := b.expenseRepo.GetByID(ctx, expense.ID)
	require.NoError(t, err)
	require.Equal(t, currency, refreshed.Currency, "expense.Currency must not be mutated by edits")

	// Confirmed-expense flow — inline edit, inline delete, and back-to-expense
	// all render the real currency.
	mockBot.Reset()
	b.handleInlineEditExpenseCore(ctx, mockBot, 55515, 100, refreshed)
	require.Len(t, mockBot.EditedMessages, 1)
	got = mockBot.LastEditedMessage().Text
	require.Contains(t, got, fmt.Sprintf("💰 Amount: %s%s %s", symbol, amount, currency))
	require.NotContains(t, got, fmt.Sprintf("💰 Amount: $%s SGD", amount))

	mockBot.Reset()
	b.handleInlineDeleteExpenseCore(ctx, mockBot, 55516, 100, refreshed)
	require.Len(t, mockBot.EditedMessages, 1)
	got = mockBot.LastEditedMessage().Text
	require.Contains(t, got, fmt.Sprintf("💰 %s%s %s", symbol, amount, currency))
	require.NotContains(t, got, fmt.Sprintf("💰 $%s SGD", amount))

	mockBot.Reset()
	backUpdate := &models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   "chain-back",
			From: models.User{ID: userID},
			Data: fmt.Sprintf("back_to_expense_%d", refreshed.ID),
			Message: models.MaybeInaccessibleMessage{
				Message: &models.Message{ID: 100, Chat: models.Chat{ID: 55517}},
			},
		},
	}
	b.handleBackToExpenseCallbackCore(ctx, mockBot, backUpdate)
	require.GreaterOrEqual(t, mockBot.EditedMessageCount(), 1)
	got = mockBot.LastEditedMessage().Text
	require.Contains(t, got, fmt.Sprintf("💰 %s%s %s", symbol, amount, currency))
	require.NotContains(t, got, fmt.Sprintf("💰 $%s SGD", amount))
}
