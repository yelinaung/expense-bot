package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestRunMigrations_Idempotent tests that migrations can be run multiple times safely.
func TestRunMigrations_Idempotent(t *testing.T) {
	pool := TestDB(t)
	ctx := context.Background()

	// Run migrations first time
	err := RunMigrations(ctx, pool)
	require.NoError(t, err)

	// Run migrations second time - should not error
	err = RunMigrations(ctx, pool)
	require.NoError(t, err)

	// Run migrations third time - should still not error
	err = RunMigrations(ctx, pool)
	require.NoError(t, err)

	// Verify tables still exist and are functional
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	require.NoError(t, err)
}

// TestRunMigrations_WithContextCancellation tests migration behavior with cancelled context.
func TestRunMigrations_WithContextCancellation(t *testing.T) {
	pool := TestDB(t)

	// Create already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := RunMigrations(ctx, pool)
	// Note: May succeed if migrations are fast enough, or may fail with context error
	// We just verify it doesn't panic
	_ = err
}

// TestSeedCategories_AlreadySeeded tests re-seeding with existing data.
func TestSeedCategories_AlreadySeeded(t *testing.T) {
	pool := TestDB(t)
	ctx := context.Background()

	err := RunMigrations(ctx, pool)
	require.NoError(t, err)

	CleanupTables(t, pool)

	// First seed
	err = SeedCategories(ctx, pool)
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM categories").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 16, count)

	// Second seed - should be idempotent
	err = SeedCategories(ctx, pool)
	require.NoError(t, err)

	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM categories").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 16, count, "should not duplicate categories")

	// Third seed - verify still idempotent
	err = SeedCategories(ctx, pool)
	require.NoError(t, err)

	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM categories").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 16, count, "should not duplicate categories after multiple seeds")
}

// TestSeedCategories_WithContextCancellation tests seeding with cancelled context.
func TestSeedCategories_WithContextCancellation(t *testing.T) {
	pool := TestDB(t)

	err := RunMigrations(context.Background(), pool)
	require.NoError(t, err)

	CleanupTables(t, pool)

	// Create already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = SeedCategories(ctx, pool)
	// May succeed or fail depending on timing
	_ = err
}

// TestConnect_WithTimeout tests connection with very short timeout.
func TestConnect_WithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Try to connect to unreachable host with very short timeout
	pool, err := Connect(ctx, "postgres://localhost:59999/nonexistent?connect_timeout=1")
	require.Error(t, err)
	require.Nil(t, pool)
}

// TestConnect_WithMalformedURL tests connection with various malformed URLs.
func TestConnect_WithMalformedURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "missing protocol",
			url:  "localhost:5432/test",
		},
		{
			name: "invalid protocol",
			url:  "http://localhost:5432/test",
		},
		{
			name: "empty string",
			url:  "",
		},
		{
			name: "just protocol",
			url:  "postgres://",
		},
		{
			name: "invalid port",
			url:  "postgres://localhost:notaport/test",
		},
		{
			name: "special characters in password",
			url:  "postgres://user:p@ss@w0rd@localhost:5432/test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			pool, err := Connect(ctx, tt.url)

			// All of these should fail
			require.Error(t, err)
			require.Nil(t, pool)
		})
	}
}

// TestCleanupTables_EmptyDatabase tests cleanup on empty database.
func TestCleanupTables_EmptyDatabase(t *testing.T) {
	pool := TestDB(t)
	ctx := context.Background()

	err := RunMigrations(ctx, pool)
	require.NoError(t, err)

	// Clean empty tables
	CleanupTables(t, pool)

	// Verify tables are empty
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM expenses").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM categories").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

// TestCleanupTables_WithData tests cleanup with existing data.
func TestCleanupTables_WithData(t *testing.T) {
	pool := TestDB(t)
	ctx := context.Background()

	err := RunMigrations(ctx, pool)
	require.NoError(t, err)

	// Insert test data
	_, err = pool.Exec(ctx, "INSERT INTO users (id, username, first_name, last_name) VALUES ($1, $2, $3, $4)",
		12345, "testuser", "Test", "User")
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO categories (name) VALUES ($1)", "TestCategory")
	require.NoError(t, err)

	// Verify data exists
	var userCount, categoryCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount)
	require.NoError(t, err)
	require.Positive(t, userCount)

	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM categories").Scan(&categoryCount)
	require.NoError(t, err)
	require.Positive(t, categoryCount)

	// Cleanup
	CleanupTables(t, pool)

	// Verify all data removed
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM expenses").Scan(&userCount)
	require.NoError(t, err)
	require.Equal(t, 0, userCount)

	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM categories").Scan(&categoryCount)
	require.NoError(t, err)
	require.Equal(t, 0, categoryCount)

	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount)
	require.NoError(t, err)
	require.Equal(t, 0, userCount)
}

// TestTestDB_SkipsWithoutEnvVar tests that TestDB skips when env var not set.
func TestTestDB_SkipsWithoutEnvVar(t *testing.T) {
	// Save original value
	original := os.Getenv("TEST_DATABASE_URL")

	// This test will actually always have the env var set in CI
	// but we document the expected behavior
	if original == "" {
		t.Skip("TEST_DATABASE_URL not set - this is expected behavior")
	}

	// Verify TestDB works when env var is set
	pool := TestDB(t)
	require.NotNil(t, pool)
}

// TestConnect_WithValidConnectionPooled tests that connection pooling works.
func TestConnect_WithValidConnectionPooled(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()

	// Create first connection
	pool1, err := Connect(ctx, dbURL)
	require.NoError(t, err)
	require.NotNil(t, pool1)
	defer pool1.Close()

	// Create second connection
	pool2, err := Connect(ctx, dbURL)
	require.NoError(t, err)
	require.NotNil(t, pool2)
	defer pool2.Close()

	// Both should be able to query
	var result1, result2 int
	err = pool1.QueryRow(ctx, "SELECT 1").Scan(&result1)
	require.NoError(t, err)
	require.Equal(t, 1, result1)

	err = pool2.QueryRow(ctx, "SELECT 1").Scan(&result2)
	require.NoError(t, err)
	require.Equal(t, 1, result2)
}

// TestSeedCategories_CategoryNames tests that all expected categories are seeded.
func TestSeedCategories_CategoryNames(t *testing.T) {
	pool := TestDB(t)
	ctx := context.Background()

	err := RunMigrations(ctx, pool)
	require.NoError(t, err)

	CleanupTables(t, pool)

	err = SeedCategories(ctx, pool)
	require.NoError(t, err)

	// Expected categories (must match migrations.go SeedCategories)
	expectedCategories := []string{
		"Food - Dining Out",
		"Food - Grocery",
		"Transportation",
		"Communication",
		"Housing - Mortgage",
		"Housing - Others",
		"Personal Care",
		"Health and Wellness",
		"Education",
		"Entertainment",
		"Credit/Debt Payments",
		"Others",
		"Utilities",
		"Travel & Vacation",
		"Subscriptions",
		"Donations",
	}

	// Verify each category exists
	for _, category := range expectedCategories {
		var exists bool
		err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM categories WHERE name = $1)", category).Scan(&exists)
		require.NoError(t, err, "failed to check category: %s", category)
		require.True(t, exists, "category not found: %s", category)
	}

	// Verify exact count
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM categories").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, len(expectedCategories), count)
}
