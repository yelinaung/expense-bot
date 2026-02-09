package bot

import (
	"testing"

	"github.com/shopspring/decimal"
	"gitlab.com/yelinaung/expense-bot/internal/config"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/repository"
)

// TestDB is a convenience wrapper around database.TestTx for bot tests.
func TestDB(t *testing.T) database.PGXDB {
	t.Helper()
	return database.TestTx(t)
}

// setupTestBot creates a Bot instance for testing with database.
//
//nolint:unused // Used in test files
func setupTestBot(t *testing.T, db database.PGXDB) *Bot {
	t.Helper()

	cfg := &config.Config{
		TelegramBotToken:   "test-token",
		DatabaseURL:        "test-url",
		WhitelistedUserIDs: []int64{123456},
		GeminiAPIKey:       "", // No Gemini for unit tests
	}

	b := &Bot{
		cfg:          cfg,
		db:           db,
		userRepo:     repository.NewUserRepository(db),
		categoryRepo: repository.NewCategoryRepository(db),
		expenseRepo:  repository.NewExpenseRepository(db),
		tagRepo:      repository.NewTagRepository(db),
		geminiClient: nil, // No Gemini client for cache tests
		pendingEdits: make(map[int64]*pendingEdit),
	}

	return b
}

// mustParseDecimal parses a decimal string or panics (for test data).
//
//nolint:unused // Used in test files
func mustParseDecimal(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic("invalid decimal in test: " + s)
	}
	return d
}
