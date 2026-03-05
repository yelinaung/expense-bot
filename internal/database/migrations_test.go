package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/testutil/dbtest"
)

func TestRunMigrations(t *testing.T) {
	pool := dbtest.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)

	var tableExists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_name = 'users'
		)
	`).Scan(&tableExists)
	require.NoError(t, err)
	require.True(t, tableExists)

	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_name = 'categories'
		)
	`).Scan(&tableExists)
	require.NoError(t, err)
	require.True(t, tableExists)

	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_name = 'expenses'
		)
	`).Scan(&tableExists)
	require.NoError(t, err)
	require.True(t, tableExists)
}

func TestSeedCategories(t *testing.T) {
	pool := dbtest.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)

	dbtest.CleanupTables(ctx, t, pool)

	err = database.SeedCategories(ctx, pool)
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM categories").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 16, count)

	err = database.SeedCategories(ctx, pool)
	require.NoError(t, err)

	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM categories").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 16, count, "should not duplicate categories on re-seed")
}
