package bot

import (
	"context"
	"errors"
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
	require.Contains(t, mockBot.LastSentMessage().Text, "not found")

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

func TestSendEmptyExpenseList(t *testing.T) {
	t.Parallel()

	b := &Bot{}
	mockBot := mocks.NewMockBot()

	b.sendEmptyExpenseList(context.Background(), mockBot, 99, "Header")
	require.Equal(t, 1, mockBot.SentMessageCount())
	require.Contains(t, mockBot.LastSentMessage().Text, "Header")
	require.Contains(t, mockBot.LastSentMessage().Text, "No expenses found")
}

func TestIsValidAutoCreatedCategoryName(t *testing.T) {
	t.Parallel()

	require.False(t, isValidAutoCreatedCategoryName(""))
	require.False(t, isValidAutoCreatedCategoryName(" \t "))
	require.False(t, isValidAutoCreatedCategoryName("bad\nname"))
	require.True(t, isValidAutoCreatedCategoryName("Travel"))
}

func TestApplyNewCategorySuggestion(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	expense := &appmodels.Expense{}
	ok := b.applyNewCategorySuggestion(ctx, expense, "desc", &gemini.CategorySuggestion{
		NewCategoryName: "bad\nname",
		Confidence:      0.95,
	}, nil)
	require.False(t, ok)
	require.Nil(t, expense.CategoryID)

	categories := []appmodels.Category{{ID: 42, Name: "Food"}}
	expense = &appmodels.Expense{}
	ok = b.applyNewCategorySuggestion(ctx, expense, "desc", &gemini.CategorySuggestion{
		NewCategoryName: "food",
		Confidence:      0.95,
	}, categories)
	require.True(t, ok)
	require.NotNil(t, expense.CategoryID)
	require.Equal(t, 42, *expense.CategoryID)

	expense = &appmodels.Expense{}
	ok = b.applyNewCategorySuggestion(ctx, expense, "desc", &gemini.CategorySuggestion{
		NewCategoryName: "NewCategory",
		Confidence:      0.95,
	}, nil)
	require.True(t, ok)
	require.NotNil(t, expense.Category)
	require.Equal(t, "NewCategory", expense.Category.Name)
}

func TestSaveInlineTags(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	const userID = int64(940001)
	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:              userID,
		Username:        "taguser",
		FirstName:       "Tag",
		DefaultCurrency: "SGD",
	})
	require.NoError(t, err)

	exp := &appmodels.Expense{
		UserID:      userID,
		Amount:      decimal.RequireFromString("3.50"),
		Currency:    "SGD",
		Description: "snack",
		Merchant:    "snack",
	}
	require.NoError(t, b.expenseRepo.Create(ctx, exp))

	b.saveInlineTags(ctx, exp.ID, []string{"food", "snack"})
	tags, err := b.tagRepo.GetByExpenseID(ctx, exp.ID)
	require.NoError(t, err)
	require.Len(t, tags, 2)
}

func TestHandlePhotoCore_GeminiNotConfigured(t *testing.T) {
	t.Parallel()

	b := &Bot{}
	mockBot := mocks.NewMockBot()
	update := mocks.PhotoUpdate(123, 456, "file-id")

	b.handlePhotoCore(context.Background(), mockBot, update)

	require.Equal(t, 1, mockBot.SentMessageCount())
	require.Contains(t, mockBot.LastSentMessage().Text, "Receipt OCR is not configured")
}

func TestHandleVoiceCore_GeminiNotConfigured(t *testing.T) {
	t.Parallel()

	b := &Bot{}
	mockBot := mocks.NewMockBot()
	update := mocks.VoiceUpdate(123, 456, "voice-file", 2)

	b.handleVoiceCore(context.Background(), mockBot, update)

	require.Equal(t, 1, mockBot.SentMessageCount())
	require.Contains(t, mockBot.LastSentMessage().Text, "Voice expense input is not configured")
}

func TestSaveInlineTags_NoTags(t *testing.T) {
	t.Parallel()
	b := &Bot{}
	b.saveInlineTags(context.Background(), 1, nil)
}

func TestApplyNewCategorySuggestion_CreateError(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	// Seed an existing category to force duplicate create path.
	_, err := b.categoryRepo.Create(ctx, "DupCat")
	require.NoError(t, err)

	expense := &appmodels.Expense{}
	ok := b.applyNewCategorySuggestion(ctx, expense, "desc", &gemini.CategorySuggestion{
		NewCategoryName: "DupCat",
		Confidence:      0.95,
	}, nil)
	require.False(t, ok)
	require.Nil(t, expense.Category)
	require.Nil(t, expense.CategoryID)
}

func TestSendVoiceParseError_Default(t *testing.T) {
	t.Parallel()

	mockBot := mocks.NewMockBot()
	sendVoiceParseError(context.Background(), mockBot, 123, errors.New("x"))
	require.Equal(t, 1, mockBot.SentMessageCount())
	require.Contains(t, mockBot.LastSentMessage().Text, "Failed to process voice message")
}
