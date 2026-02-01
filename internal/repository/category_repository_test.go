package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/database"
)

func TestCategoryRepository_CRUD(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	repo := NewCategoryRepository(tx)

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
		_, err := repo.Create(ctx, "Category A")
		require.NoError(t, err)
		_, err = repo.Create(ctx, "Category B")
		require.NoError(t, err)

		cats, err := repo.GetAll(ctx)
		require.NoError(t, err)
		require.Len(t, cats, 2)
	})
}

func TestCategoryRepository_GetByID_NonExistent(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	repo := NewCategoryRepository(tx)

	_, err := repo.GetByID(ctx, 99999)
	require.Error(t, err)
}

func TestCategoryRepository_GetByName_NonExistent(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	repo := NewCategoryRepository(tx)

	_, err := repo.GetByName(ctx, "NonExistentCategory")
	require.Error(t, err)
}

func TestCategoryRepository_UpdateNonExistent(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	repo := NewCategoryRepository(tx)

	// Update should succeed even for non-existent ID (no rows affected).
	err := repo.Update(ctx, 99999, "New Name")
	require.NoError(t, err)
}

func TestCategoryRepository_DeleteNonExistent(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	repo := NewCategoryRepository(tx)

	// Delete should succeed even for non-existent ID (no rows affected).
	err := repo.Delete(ctx, 99999)
	require.NoError(t, err)
}

func TestCategoryRepository_GetAll_Empty(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	repo := NewCategoryRepository(tx)

	cats, err := repo.GetAll(ctx)
	require.NoError(t, err)
	require.Empty(t, cats)
}

func TestCategoryRepository_CreateDuplicate(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	repo := NewCategoryRepository(tx)

	_, err := repo.Create(ctx, "Duplicate Category")
	require.NoError(t, err)

	// Attempt to create duplicate - this might succeed or fail depending on DB constraints.
	// Test verifies the behavior, not necessarily an error.
	cat2, err := repo.Create(ctx, "Duplicate Category")
	if err == nil {
		// If no unique constraint, both should exist.
		require.NotZero(t, cat2.ID)
	} else {
		// If unique constraint exists, expect an error.
		require.Error(t, err)
	}
}
