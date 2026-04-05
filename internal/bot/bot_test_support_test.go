package bot

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gitlab.com/yelinaung/expense-bot/internal/config"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/exchange"
	"gitlab.com/yelinaung/expense-bot/internal/repository"
	"gitlab.com/yelinaung/expense-bot/internal/testutil/dbtest"
)

// Shared test constants for bot package tests.
const (
	testCurrencySGD           = "SGD"
	testCoffeeDesc            = "Coffee"
	testAmount550             = "5.50"
	testAmount1000            = "10.00"
	testCategoryFood          = "Food"
	testCategoryTransport     = "Transport"
	testCategoryFoodDiningOut = "Food - Dining Out"
	testCategoryFoodGroceries = "Food - Groceries"
	testLunchDesc             = "Lunch"
	testAddCategoryCommand    = "/addcategory"
	testChartWeekCommand      = "/chart week"
	testReportWeekCommand     = "/report week"
	testReportMonthCommand    = "/report month"
	testTagUsageText          = "Usage:"
	testNotFoundText          = "not found"
	testControlCharactersText = "control characters"
	testVoiceFileID           = "voice-file-id"
	testPhotoFileID           = "photo-file-id"
	testProcessingVoiceText   = "Processing voice message"
	testProcessingReceiptText = "Processing receipt"
	testTodayExpensesText     = "Today's Expenses"
	testOriginalDescription   = "Original description"
	testEditCommandPrefix     = "/edit "
	testEditCommand           = "/edit"
	testDeleteCommand         = "/delete"
	testInlineNilCallbackName = "returns early for nil callback query"
	testUpdateExpenseTimeSQL  = "UPDATE expenses SET created_at = $1 WHERE id = $2"
	testExtractsFromMessage   = "extracts from message"
	testExtractsFromCallback  = "extracts from callback query"
	testExtractsFromEdited    = "extracts from edited message"
	testExpectedNonNilInput   = "expected non-nil result for input: %s"
)

func withBotMention(command string) string {
	return command + "@mybot"
}

func withCommandArg(command, arg string) string {
	if arg == "" {
		return command
	}

	return command + " " + arg
}

func bracketedCategory(category string) string {
	return "[" + category + "]"
}

// testDB is a convenience wrapper around dbtest.TestTx for bot tests.
func testDB(ctx context.Context, t *testing.T) database.PGXDB {
	t.Helper()
	return dbtest.TestTx(ctx, t)
}

// setupTestBot creates a Bot instance for testing with database.
func setupTestBot(t *testing.T, db database.PGXDB) *Bot {
	t.Helper()

	cfg := &config.Config{
		TelegramBotToken:   "test-token",
		DatabaseURL:        "test-url",
		WhitelistedUserIDs: []int64{123456},
		GeminiAPIKey:       "", // No Gemini for unit tests
	}

	b := &Bot{
		cfg:              cfg,
		db:               db,
		userRepo:         repository.NewUserRepository(db),
		categoryRepo:     repository.NewCategoryRepository(db),
		expenseRepo:      repository.NewExpenseRepository(db),
		tagRepo:          repository.NewTagRepository(db),
		approvedUserRepo: repository.NewApprovedUserRepository(db),
		geminiClient:     nil, // No Gemini client for cache tests
		exchangeService:  &testExchangeService{},
		messageSender:    nil, // Tests that need it will inject a mock
		displayLocation:  time.UTC,
		nowFunc:          time.Now,
		pendingEdits:     make(map[int64]*pendingEdit),
	}

	return b
}

// mustParseDecimal parses a decimal string or panics (for test data).
func mustParseDecimal(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic("invalid decimal in test: " + s)
	}
	return d
}

type testExchangeService struct{}

func (s *testExchangeService) Convert(
	_ context.Context,
	amount decimal.Decimal,
	fromCurrency, toCurrency string,
) (exchange.ConversionResult, error) {
	if fromCurrency == toCurrency {
		return exchange.ConversionResult{
			Amount:   amount,
			Rate:     decimal.NewFromInt(1),
			RateDate: time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
		}, nil
	}
	// Keep test helper deterministic and network-free; conversion logic itself
	// is validated by dedicated unit tests with explicit mocks.
	return exchange.ConversionResult{
		Amount:   amount,
		Rate:     decimal.NewFromInt(1),
		RateDate: time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
	}, nil
}
