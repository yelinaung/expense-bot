package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunMigrations(t *testing.T) {
	pool := TestDB(t)
	ctx := context.Background()

	err := RunMigrations(ctx, pool)
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
	pool := TestDB(t)
	ctx := context.Background()

	err := RunMigrations(ctx, pool)
	require.NoError(t, err)

	CleanupTables(t, pool)

	err = SeedCategories(ctx, pool)
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM categories").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 16, count)

	err = SeedCategories(ctx, pool)
	require.NoError(t, err)

	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM categories").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 16, count, "should not duplicate categories on re-seed")
}
