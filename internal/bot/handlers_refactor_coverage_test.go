package bot

import (
	"context"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/gemini"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestParseEditCommand(t *testing.T) {
	t.Parallel()

	_, _, errText := parseEditCommand("/edit")
	require.Contains(t, errText, "Usage")

	_, _, errText = parseEditCommand("/edit abc 10 coffee")
	require.Contains(t, errText, "Invalid expense ID")

	_, _, errText = parseEditCommand("/edit 1")
	require.Contains(t, errText, "Please provide new values")

	id, values, errText := parseEditCommand("/edit 12 5.50 coffee")
	require.Empty(t, errText)
	require.EqualValues(t, 12, id)
	require.Equal(t, "5.50 coffee", values)
}

func TestApplyParsedEdit(t *testing.T) {
	t.Parallel()

	expense := &appmodels.Expense{
		Amount:      decimal.RequireFromString("1.00"),
		Currency:    "USD",
		Description: "old",
		Merchant:    "old",
	}
	categories := []appmodels.Category{
		{ID: 1, Name: "Food"},
		{ID: 2, Name: "Transport"},
	}
	parsed := &ParsedExpense{
		Amount:       decimal.RequireFromString("9.99"),
		Currency:     "SGD",
		Description:  "Lunch",
		CategoryName: "food",
	}

	applyParsedEdit(expense, parsed, categories)

	require.True(t, decimal.RequireFromString("9.99").Equal(expense.Amount))
	require.Equal(t, "SGD", expense.Currency)
	require.Equal(t, "Lunch", expense.Description)
	require.Equal(t, "Lunch", expense.Merchant)
	require.NotNil(t, expense.CategoryID)
	require.Equal(t, 1, *expense.CategoryID)
	require.NotNil(t, expense.Category)
	require.Equal(t, "Food", expense.Category.Name)
}

func TestFindCategoryByName(t *testing.T) {
	t.Parallel()

	categories := []appmodels.Category{
		{ID: 10, Name: "Food"},
		{ID: 20, Name: "Transport"},
	}

	id, cat := findCategoryByName(categories, "transport")
	require.NotNil(t, id)
	require.NotNil(t, cat)
	require.Equal(t, 20, *id)
	require.Equal(t, "Transport", cat.Name)

	id, cat = findCategoryByName(categories, "unknown")
	require.Nil(t, id)
	require.Nil(t, cat)
}

func TestBuildReceiptConfirmationText(t *testing.T) {
	t.Parallel()

	expense := &appmodels.Expense{
		Amount:   decimal.RequireFromString("24.30"),
		Currency: "SGD",
		Merchant: "Cafe",
		Category: &appmodels.Category{Name: "Food"},
	}
	date := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)

	partial := buildReceiptConfirmationText(expense, date, true)
	require.Contains(t, partial, "Partial Extraction")
	require.Contains(t, partial, "24.30")
	require.Contains(t, partial, "Food")

	full := buildReceiptConfirmationText(expense, date, false)
	require.Contains(t, full, "Receipt Scanned")
	require.Contains(t, full, "15 Feb 2026")
}

func TestSendReceiptParseError(t *testing.T) {
	t.Parallel()

	mockBot := mocks.NewMockBot()
	ctx := context.Background()

	sendReceiptParseError(ctx, mockBot, 123, gemini.ErrParseTimeout)
	require.Equal(t, 1, mockBot.SentMessageCount())
	require.Contains(t, mockBot.LastSentMessage().Text, "timed out")

	sendReceiptParseError(ctx, mockBot, 123, gemini.ErrNoData)
	require.Equal(t, 2, mockBot.SentMessageCount())
	require.Contains(t, mockBot.LastSentMessage().Text, "Could not read this receipt")
}

func TestSendVoiceParseError(t *testing.T) {
	t.Parallel()

	mockBot := mocks.NewMockBot()
	ctx := context.Background()

	sendVoiceParseError(ctx, mockBot, 123, gemini.ErrVoiceParseTimeout)
	require.Equal(t, 1, mockBot.SentMessageCount())
	require.Contains(t, mockBot.LastSentMessage().Text, "timed out")

	sendVoiceParseError(ctx, mockBot, 123, gemini.ErrNoVoiceData)
	require.Equal(t, 2, mockBot.SentMessageCount())
	require.Contains(t, mockBot.LastSentMessage().Text, "Could not extract expense information")
}

func TestBuildVoiceConfirmationText(t *testing.T) {
	t.Parallel()

	expense := &appmodels.Expense{
		Amount:      decimal.RequireFromString("8.90"),
		Currency:    "SGD",
		Description: "Taxi",
		Category:    &appmodels.Category{Name: "Transport"},
	}
	text := buildVoiceConfirmationText(expense)
	require.Contains(t, text, "Voice Expense Detected")
	require.Contains(t, text, "Taxi")
	require.Contains(t, text, "Transport")
}

func TestApplyMatchedSuggestion(t *testing.T) {
	t.Parallel()

	b := &Bot{}
	expense := &appmodels.Expense{}
	categories := []appmodels.Category{
		{ID: 1, Name: "Food"},
		{ID: 2, Name: "Transport"},
	}

	ok := b.applyMatchedSuggestion(expense, "lunch", &gemini.CategorySuggestion{
		Category:   "food",
		Confidence: 0.9,
		Reasoning:  "restaurant expense",
	}, categories)
	require.True(t, ok)
	require.NotNil(t, expense.CategoryID)
	require.Equal(t, 1, *expense.CategoryID)

	expense = &appmodels.Expense{}
	ok = b.applyMatchedSuggestion(expense, "lunch", &gemini.CategorySuggestion{
		Category: "unknown",
	}, categories)
	require.False(t, ok)
	require.Nil(t, expense.CategoryID)
}

func TestSendEditConfirmation(t *testing.T) {
	t.Parallel()

	mockBot := mocks.NewMockBot()
	expense := &appmodels.Expense{
		UserExpenseNumber: 7,
		Amount:            decimal.RequireFromString("11.50"),
		Currency:          "SGD",
		Description:       "Breakfast",
		Category:          &appmodels.Category{Name: "Food"},
	}

	sendEditConfirmation(context.Background(), mockBot, 100, expense)

	require.Equal(t, 1, mockBot.SentMessageCount())
	msg := mockBot.LastSentMessage()
	require.NotNil(t, msg)
	require.Equal(t, models.ParseModeHTML, msg.ParseMode)
	require.Contains(t, msg.Text, "Expense Updated")
	require.Contains(t, msg.Text, "#7")
	require.Contains(t, msg.Text, "Breakfast")
	require.Contains(t, msg.Text, "Food")
}

func TestGetEditableExpense(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	const ownerID = int64(930001)
	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:              ownerID,
		Username:        "owner",
		FirstName:       "Owner",
		DefaultCurrency: "SGD",
	})
	require.NoError(t, err)

	exp := &appmodels.Expense{
		UserID:      ownerID,
		Amount:      decimal.RequireFromString("5.50"),
		Currency:    "SGD",
		Description: "Coffee",
		Merchant:    "Coffee",
	}
	require.NoError(t, b.expenseRepo.Create(ctx, exp))

	mockBot := mocks.NewMockBot()
	got, ok := b.getEditableExpense(ctx, mockBot, 100, ownerID, exp.UserExpenseNumber)
	require.True(t, ok)
	require.NotNil(t, got)
	require.Equal(t, exp.ID, got.ID)
	require.Equal(t, 0, mockBot.SentMessageCount())

	got, ok = b.getEditableExpense(ctx, mockBot, 100, ownerID+1, exp.UserExpenseNumber)
	require.False(t, ok)
	require.Nil(t, got)
	require.Equal(t, 1, mockBot.SentMessageCount())
	require.Contains(t, mockBot.LastSentMessage().Text, "You can only edit your own expenses")

	got, ok = b.getEditableExpense(ctx, mockBot, 100, ownerID, 99999)
	require.False(t, ok)
	require.Nil(t, got)
	require.Equal(t, 2, mockBot.SentMessageCount())
	require.Contains(t, mockBot.LastSentMessage().Text, "not found")
}

func TestParseEditExpenseValues(t *testing.T) {
	t.Parallel()

	categories := []appmodels.Category{
		{ID: 1, Name: "Food"},
		{ID: 2, Name: "Transport"},
	}
	categoryID := 2
	expense := &appmodels.Expense{
		CategoryID: &categoryID,
	}

	parsed := parseEditExpenseValues("18.25 Lunch [Food]", expense, categories)
	require.NotNil(t, parsed)
	require.Equal(t, "Lunch", parsed.Description)
	require.Equal(t, "Food", parsed.CategoryName)
	require.NotNil(t, expense.Category)
	require.Equal(t, "Transport", expense.Category.Name)
}
