package dbtest

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.com/yelinaung/expense-bot/internal/database"
)

var (
	testPool     *pgxpool.Pool
	testPoolOnce sync.Once
	testPoolErr  error
)

// TestDB returns a database connection pool for testing.
// Skips the test if TEST_DATABASE_URL is not set.
func TestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx, dbURL, false)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

// CleanupTables truncates all tables for a clean test state.
func CleanupTables(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	_, err := pool.Exec(context.WithoutCancel(ctx), "TRUNCATE TABLE expenses, users, categories CASCADE")
	if err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}
}

// TestPool returns a shared database connection pool for testing.
// The pool is created once and reused across all tests.
// Migrations are run once when the pool is first created.
// Skips the test if TEST_DATABASE_URL is not set.
func TestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	testPoolOnce.Do(func() {
		ctx := context.Background()
		testPool, testPoolErr = database.Connect(ctx, dbURL, false)
		if testPoolErr != nil {
			return
		}

		// Run migrations once for the shared pool.
		testPoolErr = database.RunMigrations(ctx, testPool)
		if testPoolErr != nil {
			return
		}

		// Seed categories once for the shared pool.
		testPoolErr = database.SeedCategories(ctx, testPool)
	})

	if testPoolErr != nil {
		t.Fatalf("failed to setup test database: %v", testPoolErr)
	}

	return testPool
}

// TestTx returns a database transaction for testing.
// The transaction is automatically rolled back when the test completes,
// ensuring test isolation without the need for table cleanup.
func TestTx(ctx context.Context, t *testing.T) database.PGXDB {
	t.Helper()

	pool := TestPool(t) //nolint:contextcheck // Shared pool setup is intentionally decoupled from caller context.

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	t.Cleanup(func() {
		_ = tx.Rollback(context.WithoutCancel(ctx))
	})

	return tx
}
