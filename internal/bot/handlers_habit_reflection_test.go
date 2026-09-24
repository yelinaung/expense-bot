package bot

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tgmodels "github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

// TestReflectionPreservesReceiptConfirmation drives the REAL receipt
// confirmation renderer (handleReceiptCallbackCore -> handleConfirmReceiptCore),
// which produces the "Expense Confirmed!" template, then runs the
// confirmation-flow reflection sequence (review_cw_ -> review_driver_ with
// advance=0). The driver-prompt step overwrites the message, so the original
// confirmation text is replayed from the stash captured at the Worth-it tap.
// The receipt template (Merchant / Date / "has been saved." footer) must be
// preserved — it must NOT be swapped for the /add "Expense Added" template.
func TestReflectionPreservesReceiptConfirmation(t *testing.T) {
	ctx := context.Background()
	db := testDB(ctx, t)
	b := setupTestBot(t, db)

	mockBot := mocks.NewMockBot()
	userID := int64(73801)
	upsertHabitTestUser(t, ctx, b, userID)

	// Merchant differs from Description so the `🏪 Merchant:` line carries
	// information the /add template lacks.
	expense := &appmodels.Expense{
		UserID:      userID,
		Amount:      decimal.RequireFromString("5.25"),
		Currency:    testCurrencySGD,
		Description: "Latte",
		Merchant:    "Starbucks Cafe",
		Status:      appmodels.ExpenseStatusDraft,
	}
	require.NoError(t, b.expenseRepo.Create(ctx, expense))
	created := time.Date(2026, 6, 20, 8, 30, 0, 0, time.UTC)
	_, err := b.db.Exec(ctx, testUpdateExpenseTimeSQL, created, expense.ID)
	require.NoError(t, err)
	expense.CreatedAt = created

	// 1. Drive the REAL receipt confirmation handler — this is what produces
	//    the "Expense Confirmed!" message a user sees after confirming a
	//    receipt-scanned expense.
	confirmUpdate := mocks.CallbackQueryUpdate(
		habitTestChatID, userID, habitTestMessage,
		fmt.Sprintf("receipt_confirm_%d", expense.ID),
	)
	b.handleReceiptCallbackCore(ctx, mockBot, confirmUpdate)

	originalText := mockBot.LastEditedMessage().Text
	t.Logf("RECEIPT ORIGINAL:\n%s", originalText)
	for _, want := range []string{
		"Expense Confirmed!",
		"🏪 Merchant: Starbucks Cafe",
		"🗓️ Date:",
		fmt.Sprintf("Expense #%d has been saved.", expense.UserExpenseNumber),
	} {
		require.Contains(t, originalText, want, "receipt confirmation missing %q", want)
	}

	// 2. Tap "Worth it" on the confirmation keyboard the receipt path
	//    attached (buildExpenseReflectionKeyboard is the same one /add uses).
	//    Telegram delivers the tapped message's current text on the callback,
	//    so carry the original confirmation text the way the real update would.
	worthData := fmt.Sprintf("%s%d", reviewConfirmWorthPrefix, expense.ID)
	worthUpdate := reflectionCallbackUpdateWithText(
		habitTestChatID, userID, habitTestMessage, worthData, originalText,
	)
	b.handleReviewCallbackCore(ctx, mockBot, worthUpdate)

	// 3. Tap a driver button — advanceBit is 0 because the worth-it prompt in
	//    the confirmation flow encodes advance=false.
	driverKeyboard := requireInlineKeyboard(t, mockBot.LastEditedMessage().ReplyMarkup)
	driverData := driverKeyboard.InlineKeyboard[0][0].CallbackData
	require.Equal(t, fmt.Sprintf("%s%d_1_0_0", reviewDriverPrefix, expense.ID), driverData)
	driverUpdate := mocks.CallbackQueryUpdate(
		habitTestChatID, userID, habitTestMessage, driverData,
	)
	b.handleReviewCallbackCore(ctx, mockBot, driverUpdate)

	finalText := mockBot.LastEditedMessage().Text
	t.Logf("REFLECTION FINAL:\n%s", finalText)

	// 4. The receipt confirmation must be preserved verbatim — not swapped to
	//    the /add "Expense Added" template.
	require.Equal(t, originalText, finalText, "receipt confirmation should be preserved verbatim after reflection")
	require.Contains(t, finalText, "Expense Confirmed!")
	require.Contains(t, finalText, "🏪 Merchant: Starbucks Cafe")
	require.Contains(t, finalText, "🗓️ Date:")
	require.Contains(t, finalText, fmt.Sprintf("Expense #%d has been saved.", expense.UserExpenseNumber))
	require.NotContains(t, finalText, "Expense Added", "final text must not fall back to the /add template")
	// The /add template's 📝 description line was never part of the receipt
	// template; it must not be introduced by the reflection flow.
	require.NotContains(t, finalText, "📝 Latte")
	// Action keyboard (Edit/Delete) is re-attached; reflection buttons dismissed.
	require.Equal(t, buildExpenseActionKeyboard(expense.ID), mockBot.LastEditedMessage().ReplyMarkup)
}

// TestReflectionPreservesTagsInAddConfirmation drives the REAL /add
// confirmation renderer (saveExpenseCore -> SendMessage with
// buildExpenseAddedMessage(expense, parsedTags)) to produce a tagged
// "Expense Added ... 🏷️ grocery, weekly" message, then runs the
// confirmation-flow reflection sequence. The 🏷️ tags line must be preserved,
// not stripped by an untagged rebuild.
func TestReflectionPreservesTagsInAddConfirmation(t *testing.T) {
	ctx := context.Background()
	db := testDB(ctx, t)
	b := setupTestBot(t, db)

	mockBot := mocks.NewMockBot()
	userID := int64(73802)
	upsertHabitTestUser(t, ctx, b, userID)

	parsed := &ParsedExpense{
		Amount:      decimal.RequireFromString("40.00"),
		Description: "Groceries",
		Currency:    testCurrencySGD,
		Tags:        []string{"grocery", "weekly"},
	}
	categories, err := b.getCategoriesWithCache(ctx)
	require.NoError(t, err)

	// 1. Drive the REAL /add save path — this produces the tagged "Expense
	//    Added" message a user sees after /add.
	b.saveExpenseCore(ctx, mockBot, habitTestChatID, userID, parsed, categories)

	origMsg := mockBot.LastSentMessage()
	originalText := origMsg.Text
	t.Logf("ADD ORIGINAL:\n%s", originalText)
	require.Contains(t, originalText, "🏷️ grocery, weekly", "original /add confirmation should have tags line")

	// Recover the "Worth it" button from the attached reflection keyboard so
	// we drive the same message the user would tap on. The keyboard rows are
	// [Edit, Delete] and [Worth it, Not worth it, Later]; search by prefix.
	kbd := requireInlineKeyboard(t, origMsg.ReplyMarkup)
	worthData := findCallbackContaining(kbd, reviewConfirmWorthPrefix)
	require.NotEmpty(t, worthData, "reflection keyboard should have a Worth it button")
	expenseID, ok := parseReviewID(worthData, reviewConfirmWorthPrefix)
	require.True(t, ok)

	// 2. Tap "Worth it", carrying the original tagged confirmation text.
	worthUpdate := reflectionCallbackUpdateWithText(
		habitTestChatID, userID, habitTestMessage, worthData, originalText,
	)
	b.handleReviewCallbackCore(ctx, mockBot, worthUpdate)

	// 3. Tap a driver (advance=0).
	driverKeyboard := requireInlineKeyboard(t, mockBot.LastEditedMessage().ReplyMarkup)
	driverData := driverKeyboard.InlineKeyboard[0][0].CallbackData
	driverUpdate := mocks.CallbackQueryUpdate(
		habitTestChatID, userID, habitTestMessage, driverData,
	)
	b.handleReviewCallbackCore(ctx, mockBot, driverUpdate)

	finalText := mockBot.LastEditedMessage().Text
	t.Logf("REFLECTION FINAL:\n%s", finalText)

	// The 🏷️ tags line must be preserved, not stripped.
	require.Equal(t, originalText, finalText, "tagged /add confirmation should be preserved verbatim after reflection")
	require.Contains(t, finalText, "🏷️ grocery, weekly")
	for _, want := range []string{"Expense Added", "Groceries", "S$40.00"} {
		require.Contains(t, finalText, want, "final text should still contain %q", want)
	}
	require.Equal(t, buildExpenseActionKeyboard(expenseID), mockBot.LastEditedMessage().ReplyMarkup)
}

// TestReflectionUntaggedAddIsUnaffected verifies the unaffected case
// end-to-end: for an untagged /add expense, the message the /add handler
// renders (via saveExpenseCore) is preserved byte-for-byte by the reflection
// flow.
func TestReflectionUntaggedAddIsUnaffected(t *testing.T) {
	ctx := context.Background()
	db := testDB(ctx, t)
	b := setupTestBot(t, db)

	mockBot := mocks.NewMockBot()
	userID := int64(73803)
	upsertHabitTestUser(t, ctx, b, userID)

	parsed := &ParsedExpense{
		Amount:      decimal.RequireFromString("12.50"),
		Description: "Lunch",
		Currency:    testCurrencySGD,
		Tags:        nil, // untagged /add
	}
	categories, err := b.getCategoriesWithCache(ctx)
	require.NoError(t, err)

	// 1. Drive the REAL /add path (no tags).
	b.saveExpenseCore(ctx, mockBot, habitTestChatID, userID, parsed, categories)
	origMsg := mockBot.LastSentMessage()
	originalText := origMsg.Text
	t.Logf("UNTAGGED ORIGINAL:\n%s", originalText)
	require.NotContains(t, originalText, "🏷️", "untagged /add should have no tags line")

	kbd := requireInlineKeyboard(t, origMsg.ReplyMarkup)
	worthData := findCallbackContaining(kbd, reviewConfirmWorthPrefix)
	require.NotEmpty(t, worthData)
	expenseID, ok := parseReviewID(worthData, reviewConfirmWorthPrefix)
	require.True(t, ok)

	// 2. Run the reflection flow on it, carrying the original text.
	worthUpdate := reflectionCallbackUpdateWithText(
		habitTestChatID, userID, habitTestMessage, worthData, originalText,
	)
	b.handleReviewCallbackCore(ctx, mockBot, worthUpdate)
	driverKeyboard := requireInlineKeyboard(t, mockBot.LastEditedMessage().ReplyMarkup)
	driverData := driverKeyboard.InlineKeyboard[0][0].CallbackData
	driverUpdate := mocks.CallbackQueryUpdate(
		habitTestChatID, userID, habitTestMessage, driverData,
	)
	b.handleReviewCallbackCore(ctx, mockBot, driverUpdate)

	finalText := mockBot.LastEditedMessage().Text
	t.Logf("UNTAGGED FINAL:\n%s", finalText)
	require.Equal(t, originalText, finalText, "untagged /add message should be byte-identical after reflection")
	require.Equal(t, buildExpenseActionKeyboard(expenseID), mockBot.LastEditedMessage().ReplyMarkup)
}

// TestReflectionFallsBackToRebuildWhenOriginalTextMissing ensures that when
// the original confirmation text is unavailable on the worth/not-worth tap
// (e.g. a legacy client that omits the message text), editToConfirmation falls
// back to the /add rebuild rather than rendering an empty message — preserving
// the pre-fix behavior for the untagged /add case.
func TestReflectionFallsBackToRebuildWhenOriginalTextMissing(t *testing.T) {
	ctx := context.Background()
	db := testDB(ctx, t)
	b := setupTestBot(t, db)

	mockBot := mocks.NewMockBot()
	userID := int64(73804)
	upsertHabitTestUser(t, ctx, b, userID)

	// Build an expense and render its /add confirmation directly.
	expense := createHabitTestExpense(
		t, ctx, b, userID, "Fallback coffee", "3.75",
		time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC),
	)
	originalText := buildExpenseAddedMessage(expense, nil)

	// Worth-it tap carries NO message text (simulating a client that omits it).
	worthUpdate := mocks.CallbackQueryUpdate(
		habitTestChatID, userID, habitTestMessage,
		fmt.Sprintf("%s%d", reviewConfirmWorthPrefix, expense.ID),
	)
	b.handleReviewCallbackCore(ctx, mockBot, worthUpdate)

	driverKeyboard := requireInlineKeyboard(t, mockBot.LastEditedMessage().ReplyMarkup)
	driverData := driverKeyboard.InlineKeyboard[0][0].CallbackData
	driverUpdate := mocks.CallbackQueryUpdate(
		habitTestChatID, userID, habitTestMessage, driverData,
	)
	b.handleReviewCallbackCore(ctx, mockBot, driverUpdate)

	finalText := mockBot.LastEditedMessage().Text
	require.Equal(t, originalText, finalText, "fallback rebuild should match the /add untagged template")
	require.Equal(t, buildExpenseActionKeyboard(expense.ID), mockBot.LastEditedMessage().ReplyMarkup)
}

// reflectionCallbackUpdateWithText builds a callback-query update whose
// message carries the original confirmation text, mirroring how Telegram
// delivers the tapped message's current text on a callback. The worth/
// not-worth reflection step stashes this text so editToConfirmation can replay
// it after the driver-prompt step overwrites the message.
func reflectionCallbackUpdateWithText(
	chatID, userID int64, messageID int, data, text string,
) *tgmodels.Update {
	return &tgmodels.Update{
		CallbackQuery: &tgmodels.CallbackQuery{
			ID:   "callback-query-id",
			From: tgmodels.User{ID: userID},
			Message: tgmodels.MaybeInaccessibleMessage{
				Type: tgmodels.MaybeInaccessibleMessageTypeMessage,
				Message: &tgmodels.Message{
					ID:   messageID,
					Chat: tgmodels.Chat{ID: chatID, Type: "private"},
					Text: text,
				},
			},
			Data: data,
		},
	}
}

// findCallbackContaining returns the first callback data in the keyboard whose
// data starts with prefix, or "" if none.
func findCallbackContaining(kbd *tgmodels.InlineKeyboardMarkup, prefix string) string {
	if kbd == nil {
		return ""
	}
	for _, row := range kbd.InlineKeyboard {
		for i := range row {
			if strings.HasPrefix(row[i].CallbackData, prefix) {
				return row[i].CallbackData
			}
		}
	}
	return ""
}
