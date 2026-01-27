package bot

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"gitlab.com/yelinaung/expense-bot/internal/config"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/repository"
)

// TestDB is a convenience wrapper around database.TestDB for bot tests.
func TestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := database.TestDB(t)

	// Run migrations
	ctx := context.Background()
	if err := database.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Seed categories
	if err := database.SeedCategories(ctx, pool); err != nil {
		t.Fatalf("failed to seed categories: %v", err)
	}

	// Cleanup after test
	t.Cleanup(func() {
		database.CleanupTables(t, pool)
	})

	return pool
}

// setupTestBot creates a Bot instance for testing with database.
//
//nolint:unused // Used in test files
func setupTestBot(t *testing.T, pool *pgxpool.Pool) *Bot {
	t.Helper()

	cfg := &config.Config{
		TelegramBotToken:   "test-token",
		DatabaseURL:        "test-url",
		WhitelistedUserIDs: []int64{123456},
		GeminiAPIKey:       "", // No Gemini for unit tests
	}

	b := &Bot{
		cfg:          cfg,
		userRepo:     repository.NewUserRepository(pool),
		categoryRepo: repository.NewCategoryRepository(pool),
		expenseRepo:  repository.NewExpenseRepository(pool),
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
