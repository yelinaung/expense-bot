package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/database"
)

// TestCategoryRepository_CreateEdgeCases tests edge cases for category creation.
func TestCategoryRepository_CreateEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("create duplicate category", func(t *testing.T) {
		tx := database.TestTx(t)
		repo := NewCategoryRepository(tx)
		// Create first category with unique name
		cat1, err := repo.Create(ctx, "Unique Duplicate Test Category")
		require.NoError(t, err)
		require.NotNil(t, cat1)

		// Try to create duplicate
		cat2, err := repo.Create(ctx, "Unique Duplicate Test Category")
		require.Error(t, err)
		require.Nil(t, cat2)
		require.Contains(t, err.Error(), "failed to create category")
	})

	t.Run("create with empty name", func(t *testing.T) {
		tx := database.TestTx(t)
		repo := NewCategoryRepository(tx)

		cat, err := repo.Create(ctx, "")
		require.NoError(t, err) // Empty string is technically allowed
		require.NotNil(t, cat)
		require.Equal(t, "", cat.Name)
	})

	t.Run("create with very long name", func(t *testing.T) {
		tx := database.TestTx(t)
		repo := NewCategoryRepository(tx)

		longName := string(make([]byte, 500))
		for i := range longName {
			longName = longName[:i] + "x" + longName[i+1:]
		}

		cat, err := repo.Create(ctx, longName)
		require.NoError(t, err)
		require.NotNil(t, cat)
		require.Equal(t, longName, cat.Name)
	})

	t.Run("create with special characters", func(t *testing.T) {
		tx := database.TestTx(t)
		repo := NewCategoryRepository(tx)

		specialName := "Food & Drink ☕🍔 (café)"

		cat, err := repo.Create(ctx, specialName)
		require.NoError(t, err)
		require.NotNil(t, cat)
		require.Equal(t, specialName, cat.Name)
	})

	t.Run("create with leading/trailing spaces", func(t *testing.T) {
		tx := database.TestTx(t)
		repo := NewCategoryRepository(tx)

		cat, err := repo.Create(ctx, "  Spaced  ")
		require.NoError(t, err)
		require.NotNil(t, cat)
		require.Equal(t, "  Spaced  ", cat.Name) // Spaces preserved
	})

	t.Run("create with newlines", func(t *testing.T) {
		tx := database.TestTx(t)
		repo := NewCategoryRepository(tx)

		cat, err := repo.Create(ctx, "Line1\nLine2")
		require.NoError(t, err)
		require.NotNil(t, cat)
	})
}

// TestCategoryRepository_GetByIDEdgeCases tests edge cases for GetByID.
func TestCategoryRepository_GetByIDEdgeCases(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	repo := NewCategoryRepository(tx)

	t.Run("get non-existent category", func(t *testing.T) {
		cat, err := repo.GetByID(ctx, 99999)
		require.Error(t, err)
		require.Nil(t, cat)
	})

	t.Run("get with zero ID", func(t *testing.T) {
		cat, err := repo.GetByID(ctx, 0)
		require.Error(t, err)
		require.Nil(t, cat)
	})

	t.Run("get with negative ID", func(t *testing.T) {
		cat, err := repo.GetByID(ctx, -1)
		require.Error(t, err)
		require.Nil(t, cat)
	})
}

// TestCategoryRepository_GetByNameEdgeCases tests edge cases for GetByName.
func TestCategoryRepository_GetByNameEdgeCases(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	repo := NewCategoryRepository(tx)

	// Create test category
	created, err := repo.Create(ctx, "TestCategory")
	require.NoError(t, err)

	t.Run("get non-existent name", func(t *testing.T) {
		cat, err := repo.GetByName(ctx, "NonExistent")
		require.Error(t, err)
		require.Nil(t, cat)
	})

	t.Run("get with empty name", func(t *testing.T) {
		cat, err := repo.GetByName(ctx, "")
		require.Error(t, err)
		require.Nil(t, cat)
	})

	t.Run("get with exact match", func(t *testing.T) {
		cat, err := repo.GetByName(ctx, "TestCategory")
		require.NoError(t, err)
		require.NotNil(t, cat)
		require.Equal(t, created.ID, cat.ID)
	})

	t.Run("get is case insensitive", func(t *testing.T) {
		// GetByName uses LOWER() so it's case-insensitive
		cat, err := repo.GetByName(ctx, "testcategory")
		require.NoError(t, err)
		require.NotNil(t, cat)
		require.Equal(t, created.ID, cat.ID)
	})
}

// TestCategoryRepository_UpdateEdgeCases tests edge cases for category updates.
func TestCategoryRepository_UpdateEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("update non-existent category", func(t *testing.T) {
		tx := database.TestTx(t)
		repo := NewCategoryRepository(tx)

		// Update doesn't check rows affected, so it succeeds silently
		err := repo.Update(ctx, 99999, "NewName")
		require.NoError(t, err) // No error, just no rows affected
	})

	t.Run("update to duplicate name", func(t *testing.T) {
		tx := database.TestTx(t)
		repo := NewCategoryRepository(tx)

		// Create two categories
		cat1, err := repo.Create(ctx, "Category1")
		require.NoError(t, err)
		_, err = repo.Create(ctx, "Category2")
		require.NoError(t, err)

		// Try to rename cat1 to cat2's name
		err = repo.Update(ctx, cat1.ID, "Category2")
		require.Error(t, err)
	})

	t.Run("update to empty name", func(t *testing.T) {
		tx := database.TestTx(t)
		repo := NewCategoryRepository(tx)

		cat, err := repo.Create(ctx, "ToBeEmptied")
		require.NoError(t, err)

		err = repo.Update(ctx, cat.ID, "")
		require.NoError(t, err) // Empty name allowed

		// Verify update
		updated, err := repo.GetByID(ctx, cat.ID)
		require.NoError(t, err)
		require.Equal(t, "", updated.Name)
	})

	t.Run("update to same name", func(t *testing.T) {
		tx := database.TestTx(t)
		repo := NewCategoryRepository(tx)

		cat, err := repo.Create(ctx, "SameName")
		require.NoError(t, err)

		// Update to same name (should succeed)
		err = repo.Update(ctx, cat.ID, "SameName")
		require.NoError(t, err)
	})
}

// TestCategoryRepository_DeleteEdgeCases tests edge cases for category deletion.
func TestCategoryRepository_DeleteEdgeCases(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	repo := NewCategoryRepository(tx)

	t.Run("delete non-existent category", func(t *testing.T) {
		// Delete doesn't check rows affected, so it succeeds silently
		err := repo.Delete(ctx, 99999)
		require.NoError(t, err) // No error, just no rows affected
	})

	t.Run("delete already deleted category", func(t *testing.T) {
		cat, err := repo.Create(ctx, "ToBeDeleted")
		require.NoError(t, err)

		// Delete once
		err = repo.Delete(ctx, cat.ID)
		require.NoError(t, err)

		// Try to delete again - succeeds but affects 0 rows
		err = repo.Delete(ctx, cat.ID)
		require.NoError(t, err)
	})

	t.Run("delete with zero ID", func(t *testing.T) {
		// Deletes nothing but doesn't error
		err := repo.Delete(ctx, 0)
		require.NoError(t, err)
	})

	t.Run("delete with negative ID", func(t *testing.T) {
		// Deletes nothing but doesn't error
		err := repo.Delete(ctx, -1)
		require.NoError(t, err)
	})
}

// TestCategoryRepository_GetAllEdgeCases tests edge cases for GetAll.
func TestCategoryRepository_GetAllEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("get all when empty", func(t *testing.T) {
		tx := database.TestTx(t)
		repo := NewCategoryRepository(tx)

		// Delete all categories to test empty state
		_, err := tx.Exec(ctx, "DELETE FROM categories")
		require.NoError(t, err)

		categories, err := repo.GetAll(ctx)
		require.NoError(t, err)
		require.Empty(t, categories) // Should return empty slice, not error
	})

	t.Run("get all with many categories", func(t *testing.T) {
		tx := database.TestTx(t)
		repo := NewCategoryRepository(tx)

		// Get initial count (includes seeded categories)
		initialCats, err := repo.GetAll(ctx)
		require.NoError(t, err)
		initialCount := len(initialCats)

		// Create 100 categories
		for i := 0; i < 100; i++ {
			_, err := repo.Create(ctx, fmt.Sprintf("Category%d", i))
			require.NoError(t, err)
		}

		categories, err := repo.GetAll(ctx)
		require.NoError(t, err)
		require.Len(t, categories, initialCount+100)
	})
}
