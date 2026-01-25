package database

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
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
	pool, err := Connect(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

// CleanupTables truncates all tables for a clean test state.
func CleanupTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	ctx := context.Background()
	tables := []string{"expenses", "users", "categories"}
	for _, table := range tables {
		_, err := pool.Exec(ctx, "TRUNCATE TABLE "+table+" CASCADE")
		if err != nil {
			t.Fatalf("failed to truncate table %s: %v", table, err)
		}
	}
}
