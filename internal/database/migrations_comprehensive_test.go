package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMigrations_SchemaDetails verifies the complete database schema.
func TestMigrations_SchemaDetails(t *testing.T) {
	pool := TestDB(t)
	ctx := context.Background()

	err := RunMigrations(ctx, pool)
	require.NoError(t, err)

	t.Run("users table has correct columns", func(t *testing.T) {
		var exists bool

		// Check primary key column
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.columns
				WHERE table_name = 'users'
				AND column_name = 'id'
				AND data_type = 'bigint'
			)
		`).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "users.id should be bigint")

		// Check username column
		err = pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.columns
				WHERE table_name = 'users'
				AND column_name = 'username'
				AND data_type = 'text'
			)
		`).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "users.username should exist")

		// Check timestamps
		err = pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.columns
				WHERE table_name = 'users'
				AND column_name = 'created_at'
				AND data_type = 'timestamp with time zone'
			)
		`).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "users.created_at should be timestamptz")
	})

	t.Run("categories table has unique constraint on name", func(t *testing.T) {
		var exists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.table_constraints
				WHERE table_name = 'categories'
				AND constraint_type = 'UNIQUE'
			)
		`).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "categories should have unique constraint")
	})

	t.Run("expenses table has foreign keys", func(t *testing.T) {
		// Check foreign key to users
		var exists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.table_constraints
				WHERE table_name = 'expenses'
				AND constraint_type = 'FOREIGN KEY'
			)
		`).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "expenses should have foreign key constraints")

		// Verify user_id column references users
		err = pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.constraint_column_usage
				WHERE table_name = 'users'
				AND column_name = 'id'
			)
		`).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "expenses.user_id should reference users.id")
	})

	t.Run("expenses table has correct decimal precision", func(t *testing.T) {
		var numericPrecision, numericScale int
		err := pool.QueryRow(ctx, `
			SELECT numeric_precision, numeric_scale
			FROM information_schema.columns
			WHERE table_name = 'expenses'
			AND column_name = 'amount'
		`).Scan(&numericPrecision, &numericScale)
		require.NoError(t, err)
		require.Equal(t, 12, numericPrecision, "amount should have precision 12")
		require.Equal(t, 2, numericScale, "amount should have scale 2")
	})

	t.Run("expenses table has status column", func(t *testing.T) {
		var exists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.columns
				WHERE table_name = 'expenses'
				AND column_name = 'status'
				AND data_type = 'text'
			)
		`).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "expenses.status should exist as text")
	})
}

// TestMigrations_Indexes verifies all indexes are created.
func TestMigrations_Indexes(t *testing.T) {
	pool := TestDB(t)
	ctx := context.Background()

	err := RunMigrations(ctx, pool)
	require.NoError(t, err)

	expectedIndexes := []string{
		"idx_expenses_user_id",
		"idx_expenses_created_at",
		"idx_expenses_category_id",
		"idx_expenses_status",
	}

	for _, indexName := range expectedIndexes {
		t.Run(indexName, func(t *testing.T) {
			var exists bool
			err := pool.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT FROM pg_indexes
					WHERE indexname = $1
				)
			`, indexName).Scan(&exists)
			require.NoError(t, err)
			require.True(t, exists, "index %s should exist", indexName)
		})
	}
}

// TestMigrations_ForeignKeyConstraints tests that foreign key constraints work.
func TestMigrations_ForeignKeyConstraints(t *testing.T) {
	pool := TestDB(t)
	ctx := context.Background()

	err := RunMigrations(ctx, pool)
	require.NoError(t, err)

	CleanupTables(t, pool)

	t.Run("cannot insert expense without user", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			INSERT INTO expenses (user_id, amount, currency, description)
			VALUES (999999, 10.00, 'SGD', 'Test')
		`)
		require.Error(t, err, "should fail due to foreign key constraint")
		require.Contains(t, err.Error(), "violates foreign key constraint")
	})

	t.Run("can insert expense with valid user", func(t *testing.T) {
		// First insert user
		_, err := pool.Exec(ctx, `
			INSERT INTO users (id, username, first_name, last_name)
			VALUES (123, 'testuser', 'Test', 'User')
		`)
		require.NoError(t, err)

		// Then insert expense
		_, err = pool.Exec(ctx, `
			INSERT INTO expenses (user_id, amount, currency, description)
			VALUES (123, 10.00, 'SGD', 'Test')
		`)
		require.NoError(t, err, "should succeed with valid user")
	})

	t.Run("can insert expense with valid category", func(t *testing.T) {
		CleanupTables(t, pool)

		// Insert user
		_, err := pool.Exec(ctx, `
			INSERT INTO users (id, username, first_name, last_name)
			VALUES (123, 'testuser', 'Test', 'User')
		`)
		require.NoError(t, err)

		// Insert category
		var categoryID int
		err = pool.QueryRow(ctx, `
			INSERT INTO categories (name) VALUES ('Food') RETURNING id
		`).Scan(&categoryID)
		require.NoError(t, err)

		// Insert expense with category
		_, err = pool.Exec(ctx, `
			INSERT INTO expenses (user_id, amount, currency, description, category_id)
			VALUES (123, 10.00, 'SGD', 'Test', $1)
		`, categoryID)
		require.NoError(t, err, "should succeed with valid category")
	})
}

// TestMigrations_DefaultValues tests that default values are set correctly.
func TestMigrations_DefaultValues(t *testing.T) {
	pool := TestDB(t)
	ctx := context.Background()

	err := RunMigrations(ctx, pool)
	require.NoError(t, err)

	CleanupTables(t, pool)

	t.Run("expenses.status defaults to 'confirmed'", func(t *testing.T) {
		// Insert user
		_, err := pool.Exec(ctx, `
			INSERT INTO users (id, username, first_name, last_name)
			VALUES (123, 'testuser', 'Test', 'User')
		`)
		require.NoError(t, err)

		// Insert expense without status
		var expenseID int
		err = pool.QueryRow(ctx, `
			INSERT INTO expenses (user_id, amount, currency, description)
			VALUES (123, 10.00, 'SGD', 'Test')
			RETURNING id
		`).Scan(&expenseID)
		require.NoError(t, err)

		// Verify default status
		var status string
		err = pool.QueryRow(ctx, `
			SELECT status FROM expenses WHERE id = $1
		`, expenseID).Scan(&status)
		require.NoError(t, err)
		require.Equal(t, "confirmed", status)
	})

	t.Run("expenses.currency defaults to 'SGD'", func(t *testing.T) {
		CleanupTables(t, pool)

		// Insert user
		_, err := pool.Exec(ctx, `
			INSERT INTO users (id, username, first_name, last_name)
			VALUES (123, 'testuser', 'Test', 'User')
		`)
		require.NoError(t, err)

		// Insert expense without currency
		var expenseID int
		err = pool.QueryRow(ctx, `
			INSERT INTO expenses (user_id, amount, description)
			VALUES (123, 10.00, 'Test')
			RETURNING id
		`).Scan(&expenseID)
		require.NoError(t, err)

		// Verify default currency
		var currency string
		err = pool.QueryRow(ctx, `
			SELECT currency FROM expenses WHERE id = $1
		`, expenseID).Scan(&currency)
		require.NoError(t, err)
		require.Equal(t, "SGD", currency)
	})

	t.Run("timestamps are automatically set", func(t *testing.T) {
		CleanupTables(t, pool)

		// Insert user
		var userID int64
		err := pool.QueryRow(ctx, `
			INSERT INTO users (id, username, first_name, last_name)
			VALUES (123, 'testuser', 'Test', 'User')
			RETURNING id
		`).Scan(&userID)
		require.NoError(t, err)

		// Verify timestamps are set
		var exists bool
		err = pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM users
				WHERE id = $1
				AND created_at IS NOT NULL
				AND updated_at IS NOT NULL
			)
		`, userID).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "timestamps should be automatically set")
	})
}

// TestSeedCategories_DuplicateHandling tests ON CONFLICT handling.
func TestSeedCategories_DuplicateHandling(t *testing.T) {
	pool := TestDB(t)
	ctx := context.Background()

	err := RunMigrations(ctx, pool)
	require.NoError(t, err)

	CleanupTables(t, pool)

	// Manually insert one of the categories
	_, err = pool.Exec(ctx, `INSERT INTO categories (name) VALUES ('Food - Dining Out')`)
	require.NoError(t, err)

	var countBefore int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM categories").Scan(&countBefore)
	require.NoError(t, err)
	require.Equal(t, 1, countBefore)

	// Run seed - should not error and should not duplicate
	err = SeedCategories(ctx, pool)
	require.NoError(t, err)

	var countAfter int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM categories").Scan(&countAfter)
	require.NoError(t, err)
	require.Equal(t, 16, countAfter, "should have all categories")

	// Verify the manually inserted category wasn't duplicated
	var foodDiningOutCount int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM categories WHERE name = 'Food - Dining Out'`).Scan(&foodDiningOutCount)
	require.NoError(t, err)
	require.Equal(t, 1, foodDiningOutCount, "should not duplicate existing category")
}

// TestMigrations_MigrationOrder tests that migrations run in correct order.
func TestMigrations_MigrationOrder(t *testing.T) {
	pool := TestDB(t)
	ctx := context.Background()

	// Drop all tables to test from scratch
	_, err := pool.Exec(ctx, "DROP TABLE IF EXISTS expenses CASCADE")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "DROP TABLE IF EXISTS categories CASCADE")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "DROP TABLE IF EXISTS users CASCADE")
	require.NoError(t, err)

	// Run migrations
	err = RunMigrations(ctx, pool)
	require.NoError(t, err)

	// Verify all tables were created in correct order
	// (if order was wrong, foreign keys would fail)
	var tableCount int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_type = 'BASE TABLE'
		AND table_name IN ('users', 'categories', 'expenses')
	`).Scan(&tableCount)
	require.NoError(t, err)
	require.Equal(t, 3, tableCount, "all three tables should be created")
}

// TestSeedCategories_CategoryOrder tests categories are inserted in expected order.
func TestSeedCategories_CategoryOrder(t *testing.T) {
	pool := TestDB(t)
	ctx := context.Background()

	err := RunMigrations(ctx, pool)
	require.NoError(t, err)

	CleanupTables(t, pool)

	err = SeedCategories(ctx, pool)
	require.NoError(t, err)

	// Verify first category
	var firstName string
	err = pool.QueryRow(ctx, `
		SELECT name FROM categories ORDER BY id LIMIT 1
	`).Scan(&firstName)
	require.NoError(t, err)
	require.Equal(t, "Food - Dining Out", firstName, "first category should be Food - Dining Out")

	// Verify last category
	var lastName string
	err = pool.QueryRow(ctx, `
		SELECT name FROM categories ORDER BY id DESC LIMIT 1
	`).Scan(&lastName)
	require.NoError(t, err)
	require.Equal(t, "Donations", lastName, "last category should be Donations")
}
