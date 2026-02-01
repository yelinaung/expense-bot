package database

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	testPool     *pgxpool.Pool
	testPoolOnce sync.Once
	testPoolErr  error
)

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
		testPool, testPoolErr = Connect(ctx, dbURL)
		if testPoolErr != nil {
			return
		}

		// Run migrations once for the shared pool.
		testPoolErr = RunMigrations(ctx, testPool)
		if testPoolErr != nil {
			return
		}

		// Seed categories once for the shared pool.
		testPoolErr = SeedCategories(ctx, testPool)
	})

	if testPoolErr != nil {
		t.Fatalf("failed to setup test database: %v", testPoolErr)
	}

	return testPool
}

// TestTx returns a database transaction for testing.
// The transaction is automatically rolled back when the test completes,
// ensuring test isolation without the need for table cleanup.
//
// This approach allows tests to run in parallel safely, as each test
// operates in its own transaction that doesn't affect others.
//
// Usage:
//
//	tx := database.TestTx(t)
//	userRepo := repository.NewUserRepository(tx)
//	// Use repositories normally - all operations are in a transaction
//	// Transaction is automatically rolled back after test completes
//
// Benefits:
//   - No need for CleanupTables() - automatic rollback
//   - Tests can run in parallel safely (each has own transaction)
//   - Faster than TRUNCATE-based cleanup
//   - Requires repositories to accept PGXDB interface
//
// Note: This function returns PGXDB interface. Repositories must be updated
// to accept PGXDB instead of *pgxpool.Pool.
func TestTx(t *testing.T) PGXDB {
	t.Helper()

	pool := TestPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	t.Cleanup(func() {
		_ = tx.Rollback(context.Background())
	})

	return tx
}
