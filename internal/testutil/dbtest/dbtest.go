package dbtest

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
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

	rows, err := pool.Query(context.WithoutCancel(ctx), `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		ORDER BY tablename
	`)
	if err != nil {
		t.Fatalf("failed to list tables for cleanup: %v", err)
	}
	defer rows.Close()

	tables := make([]string, 0, 16)
	for rows.Next() {
		var table string
		if scanErr := rows.Scan(&table); scanErr != nil {
			t.Fatalf("failed to scan table name for cleanup: %v", scanErr)
		}
		tables = append(tables, pgx.Identifier{table}.Sanitize())
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		t.Fatalf("failed to iterate tables for cleanup: %v", rowsErr)
	}
	if len(tables) == 0 {
		return
	}

	// nosemgrep: gosec.G202-1 // Table names come from pg_tables (trusted) and are sanitized via pgx.Identifier.Sanitize().
	_, err = pool.Exec(
		context.WithoutCancel(ctx),
		"TRUNCATE TABLE "+strings.Join(tables, ", ")+" RESTART IDENTITY CASCADE",
	)
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
//
// This approach allows tests to run in parallel safely, as each test
// operates in its own transaction that doesn't affect others.
//
// # Usage
//
//	tx := dbtest.TestTx(ctx, t)
//	userRepo := repository.NewUserRepository(tx)
//	// Use repositories normally - all operations are in a transaction.
//	// The transaction is automatically rolled back after the test completes.
//
// # Benefits
//   - No need for CleanupTables() - automatic rollback via t.Cleanup.
//   - Tests can run in parallel safely (each has its own transaction).
//   - Faster than TRUNCATE-based cleanup.
//   - Works with any repository that accepts the database.PGXDB interface.
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
