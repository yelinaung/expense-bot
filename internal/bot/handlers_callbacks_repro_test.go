package bot

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/exchange"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

// These tests guard against the cross-field conflation bug where editing one of
// {Merchant, Description} overwrote the other. Description and Merchant are
// distinct columns by design: for a cross-currency expense, convertExpenseCurrency
// appends an "[orig: ...]" FX note to Description while leaving Merchant as the
// clean merchant name. processMerchantEditCore (handlers_callbacks.go) edits only
// Merchant; applyParsedEdit (handlers_commands.go) edits only Description. The
// previous implementation assigned one field's new value to the other field,
// destroying the divergent value.

func TestProcessMerchantEditCore_PreservesDivergentDescription(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)

	const userID = int64(500301)
	require.NoError(t, b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:              userID,
		Username:        "merchanteditfx301",
		FirstName:       "MerchantEditFX",
		DefaultCurrency: testCurrencySGD,
	}))

	const merchant = "Coffee Shop"
	const descriptionWithFX = "Coffee Shop [orig: 15.00 USD -> 20.00 SGD @ 1.3333 (2026-01-01)]"

	expense := &appmodels.Expense{
		UserID:      userID,
		Amount:      mustParseDecimal(amount20CBT),
		Currency:    testCurrencySGD,
		Description: descriptionWithFX,
		Merchant:    merchant,
		Status:      appmodels.ExpenseStatusDraft,
	}
	require.NoError(t, b.expenseRepo.Create(ctx, expense))

	mockBot := mocks.NewMockBot()
	pending := &pendingEdit{ExpenseID: expense.ID, EditType: merchantTypeCBT, MessageID: 100}
	require.True(t,
		b.processMerchantEditCore(ctx, mockBot, 33340, userID, pending, newRestaurantTextCBT))

	require.Len(t, mockBot.EditedMessages, 1)
	require.Contains(t, mockBot.EditedMessages[0].Text, "Merchant Updated")
	require.Contains(t, mockBot.EditedMessages[0].Text, newRestaurantTextCBT)

	updated, err := b.expenseRepo.GetByID(ctx, expense.ID)
	require.NoError(t, err)
	require.Equal(t, newRestaurantTextCBT, updated.Merchant, "merchant should be updated to the new name")
	require.Equal(t, descriptionWithFX, updated.Description,
		"editing merchant must not overwrite the divergent Description / FX note")
}

// TestMerchantEdit_DraftEditMerchantThenConfirm_PreservesFXNote drives the actual
// reported trigger sequence: a cross-currency receipt creates a draft carrying an
// "[orig: ...]" FX note in Description; the user edits the merchant before tapping
// Confirm; the draft edit persists; Confirm's subsequent Update must not bake in
// any corruption. This mirrors handlers_receipt.go (create draft) ->
// processMerchantEditCore (edit) -> handleConfirmReceiptCore (confirm).
func TestMerchantEdit_DraftEditMerchantThenConfirm_PreservesFXNote(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)

	const userID = int64(500302)
	require.NoError(t, b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:              userID,
		Username:        "merchanteditconfirm",
		FirstName:       "MerchantEditConfirm",
		DefaultCurrency: testCurrencySGD,
	}))

	b.exchangeService = &mockExchangeService{
		result: exchange.ConversionResult{
			Amount:   mustParseDecimal(amount20CBT),
			Rate:     decimal.RequireFromString("1.3333"),
			RateDate: time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
		},
	}

	const merchant = "Coffee Shop"
	convertedAmount, finalCurrency, finalDescription := b.convertExpenseCurrency(
		ctx, userID, mustParseDecimal(amount15CBT), "USD", merchant,
	)
	const expectedDescription = "Coffee Shop [orig: 15.00 USD -> 20.00 SGD @ 1.3333 (2026-02-14)]"
	require.Equal(t, expectedDescription, finalDescription)
	require.True(t, mustParseDecimal(amount20CBT).Equal(convertedAmount))
	require.Equal(t, testCurrencySGD, finalCurrency)

	expense := &appmodels.Expense{
		UserID:      userID,
		Amount:      convertedAmount,
		Currency:    finalCurrency,
		Description: finalDescription,
		Merchant:    merchant,
		Status:      appmodels.ExpenseStatusDraft,
	}
	require.NoError(t, b.expenseRepo.Create(ctx, expense))

	mockBot := mocks.NewMockBot()
	pending := &pendingEdit{ExpenseID: expense.ID, EditType: merchantTypeCBT, MessageID: 100}
	require.True(t,
		b.processMerchantEditCore(ctx, mockBot, 33342, userID, pending, newRestaurantTextCBT))

	draftAfterEdit, err := b.expenseRepo.GetByID(ctx, expense.ID)
	require.NoError(t, err)
	require.Equal(t, newRestaurantTextCBT, draftAfterEdit.Merchant,
		"merchant should be updated by the draft-time edit")
	require.Equal(t, expectedDescription, draftAfterEdit.Description,
		"draft-time merchant edit must not overwrite the FX note before confirm")

	// Replay handleConfirmReceiptCore's confirm step: re-fetch, set Status=Confirmed, Update.
	draftAfterEdit.Status = appmodels.ExpenseStatusConfirmed
	require.NoError(t, b.expenseRepo.Update(ctx, draftAfterEdit))

	confirmed, err := b.expenseRepo.GetByID(ctx, expense.ID)
	require.NoError(t, err)
	require.Equal(t, appmodels.ExpenseStatusConfirmed, confirmed.Status)
	require.Equal(t, newRestaurantTextCBT, confirmed.Merchant)
	require.Equal(t, expectedDescription, confirmed.Description,
		"confirm must preserve the FX note (no corruption baked in from the prior edit)")
	require.Contains(t, confirmed.Description, "15.00 USD",
		"the original amount must remain recoverable from the confirmed row")
}

// TestApplyParsedEdit_DescriptionEditPreservesMerchant is the focused unit guard for
// the /edit sibling defect: editing the description must not assign the new
// description text to Merchant.
func TestApplyParsedEdit_DescriptionEditPreservesMerchant(t *testing.T) {
	t.Parallel()

	const merchant = "Coffee Shop"
	const descriptionWithFX = "Coffee Shop [orig: 15.00 USD -> 20.00 SGD @ 1.3333 (2026-01-01)]"
	expense := &appmodels.Expense{
		Amount:      mustParseDecimal(amount20CBT),
		Currency:    testCurrencySGD,
		Description: descriptionWithFX,
		Merchant:    merchant,
		Status:      appmodels.ExpenseStatusConfirmed,
	}
	categories := []appmodels.Category{
		{ID: 1, Name: testCategoryFood},
		{ID: 2, Name: testCategoryTransport},
	}
	parsed := &ParsedExpense{
		Amount:      mustParseDecimal(amount20CBT),
		Currency:    testCurrencySGD,
		Description: newRestaurantTextCBT,
	}

	applyParsedEdit(expense, parsed, categories)

	require.Equal(t, newRestaurantTextCBT, expense.Description,
		"the supplied description should be applied")
	require.Equal(t, merchant, expense.Merchant,
		"editing description must not clobber the clean merchant name")
}

// TestApplyParsedEdit_ConfirmedRowPersistsPreservedMerchant drives the /edit path
// against a confirmed cross-currency row end to end (parse -> apply -> Update ->
// reload) and asserts the clean Merchant survives while the user's new
// Description is persisted.
func TestApplyParsedEdit_ConfirmedRowPersistsPreservedMerchant(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)

	const userID = int64(500303)
	require.NoError(t, b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:              userID,
		Username:        "editdescmerchant303",
		FirstName:       "EditDescMerchant",
		DefaultCurrency: testCurrencySGD,
	}))

	const merchant = "Coffee Shop"
	const descriptionWithFX = "Coffee Shop [orig: 15.00 USD -> 20.00 SGD @ 1.3333 (2026-03-01)]"
	expense := &appmodels.Expense{
		UserID:      userID,
		Amount:      mustParseDecimal(amount15CBT),
		Currency:    "USD",
		Description: descriptionWithFX,
		Merchant:    merchant,
		Status:      appmodels.ExpenseStatusConfirmed,
	}
	require.NoError(t, b.expenseRepo.Create(ctx, expense))

	categories, err := b.categoryRepo.GetAll(ctx)
	require.NoError(t, err)

	parsed := parseEditExpenseValues(amount20CBT+" "+newRestaurantTextCBT, expense, categories)
	require.NotNil(t, parsed)
	applyParsedEdit(expense, parsed, categories)
	require.NoError(t, b.expenseRepo.Update(ctx, expense))

	updated, err := b.expenseRepo.GetByID(ctx, expense.ID)
	require.NoError(t, err)
	require.Equal(t, appmodels.ExpenseStatusConfirmed, updated.Status, "row remains confirmed")
	require.True(t, mustParseDecimal(amount20CBT).Equal(updated.Amount), "amount should be updated")
	require.Equal(t, newRestaurantTextCBT, updated.Description,
		"the supplied description should be applied")
	require.Equal(t, merchant, updated.Merchant,
		"/edit assigning the new description must not clobber the clean merchant name")
}
