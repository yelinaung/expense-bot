package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/database"
)

func TestCategoryRepository_CRUD(t *testing.T) {
	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	repo := NewCategoryRepository(pool)

	t.Run("creates and retrieves category", func(t *testing.T) {
		cat, err := repo.Create(ctx, "Test Category")
		require.NoError(t, err)
		require.NotZero(t, cat.ID)
		require.Equal(t, "Test Category", cat.Name)

		fetched, err := repo.GetByID(ctx, cat.ID)
		require.NoError(t, err)
		require.Equal(t, cat.Name, fetched.Name)
	})

	t.Run("gets category by name case-insensitive", func(t *testing.T) {
		cat, err := repo.Create(ctx, "Food - Dining Out")
		require.NoError(t, err)

		fetched, err := repo.GetByName(ctx, "food - dining out")
		require.NoError(t, err)
		require.Equal(t, cat.ID, fetched.ID)
	})

	t.Run("updates category", func(t *testing.T) {
		cat, err := repo.Create(ctx, "Old Name")
		require.NoError(t, err)

		err = repo.Update(ctx, cat.ID, "New Name")
		require.NoError(t, err)

		fetched, err := repo.GetByID(ctx, cat.ID)
		require.NoError(t, err)
		require.Equal(t, "New Name", fetched.Name)
	})

	t.Run("deletes category", func(t *testing.T) {
		cat, err := repo.Create(ctx, "To Delete")
		require.NoError(t, err)

		err = repo.Delete(ctx, cat.ID)
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, cat.ID)
		require.Error(t, err)
	})

	t.Run("gets all categories", func(t *testing.T) {
		database.CleanupTables(t, pool)

		_, err := repo.Create(ctx, "Category A")
		require.NoError(t, err)
		_, err = repo.Create(ctx, "Category B")
		require.NoError(t, err)

		cats, err := repo.GetAll(ctx)
		require.NoError(t, err)
		require.Len(t, cats, 2)
	})
}
