package repository

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

// TestExpenseRepository_CreateEdgeCases tests edge cases for expense creation.
func TestExpenseRepository_CreateEdgeCases(t *testing.T) {
	pool := database.TestDB(t)
	ctx := context.Background()

	// Run migrations
	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)

	// Cleanup
	database.CleanupTables(t, pool)

	// Create test user for foreign key constraint
	userRepo := NewUserRepository(pool)
	err = userRepo.UpsertUser(ctx, &models.User{
		ID:        123,
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
	})
	require.NoError(t, err)

	repo := NewExpenseRepository(pool)

	t.Run("create with very large amount", func(t *testing.T) {
		expense := &models.Expense{
			UserID:      123,
			Amount:      decimal.NewFromFloat(999999999.99),
			Currency:    "SGD",
			Description: "Very large expense",
			Status:      models.ExpenseStatusConfirmed,
		}

		err := repo.Create(ctx, expense)
		require.NoError(t, err)
		require.NotZero(t, expense.ID)

		// Verify it was stored correctly
		retrieved, err := repo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.True(t, expense.Amount.Equal(retrieved.Amount))
	})

	t.Run("create with very small amount", func(t *testing.T) {
		expense := &models.Expense{
			UserID:      123,
			Amount:      decimal.NewFromFloat(0.01),
			Currency:    "SGD",
			Description: "Very small expense",
			Status:      models.ExpenseStatusConfirmed,
		}

		err := repo.Create(ctx, expense)
		require.NoError(t, err)
		require.NotZero(t, expense.ID)
	})

	t.Run("create with empty description", func(t *testing.T) {
		expense := &models.Expense{
			UserID:      123,
			Amount:      decimal.NewFromFloat(10.00),
			Currency:    "SGD",
			Description: "", // Empty description
			Status:      models.ExpenseStatusConfirmed,
		}

		err := repo.Create(ctx, expense)
		require.NoError(t, err)
		require.NotZero(t, expense.ID)
	})

	t.Run("create with very long description", func(t *testing.T) {
		longDesc := string(make([]byte, 1000))
		for i := range longDesc {
			longDesc = longDesc[:i] + "x" + longDesc[i+1:]
		}

		expense := &models.Expense{
			UserID:      123,
			Amount:      decimal.NewFromFloat(10.00),
			Currency:    "SGD",
			Description: longDesc,
			Status:      models.ExpenseStatusConfirmed,
		}

		err := repo.Create(ctx, expense)
		require.NoError(t, err)
		require.NotZero(t, expense.ID)
	})

	t.Run("create with special characters in description", func(t *testing.T) {
		expense := &models.Expense{
			UserID:      123,
			Amount:      decimal.NewFromFloat(10.00),
			Currency:    "SGD",
			Description: "Coffee ☕ & Cake 🍰 @ Café",
			Status:      models.ExpenseStatusConfirmed,
		}

		err := repo.Create(ctx, expense)
		require.NoError(t, err)
		require.NotZero(t, expense.ID)

		// Verify special chars preserved
		retrieved, err := repo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.Equal(t, expense.Description, retrieved.Description)
	})

	t.Run("create draft expense", func(t *testing.T) {
		expense := &models.Expense{
			UserID:      123,
			Amount:      decimal.NewFromFloat(10.00),
			Currency:    "SGD",
			Description: "Draft",
			Status:      models.ExpenseStatusDraft,
		}

		err := repo.Create(ctx, expense)
		require.NoError(t, err)
		require.NotZero(t, expense.ID)
		require.Equal(t, models.ExpenseStatusDraft, expense.Status)
	})
}

// TestExpenseRepository_UpdateEdgeCases tests edge cases for expense updates.
func TestExpenseRepository_UpdateEdgeCases(t *testing.T) {
	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	// Create test user
	userRepo := NewUserRepository(pool)
	err = userRepo.UpsertUser(ctx, &models.User{
		ID:        123,
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
	})
	require.NoError(t, err)

	repo := NewExpenseRepository(pool)

	t.Run("update non-existent expense", func(t *testing.T) {
		expense := &models.Expense{
			ID:          99999, // Non-existent ID
			UserID:      123,
			Amount:      decimal.NewFromFloat(10.00),
			Currency:    "SGD",
			Description: "Test",
			Status:      models.ExpenseStatusConfirmed,
		}

		// Update doesn't check rows affected, so it succeeds silently
		err := repo.Update(ctx, expense)
		require.NoError(t, err) // No error, just no rows affected
	})

	t.Run("update to empty description", func(t *testing.T) {
		// Create expense first
		expense := &models.Expense{
			UserID:      123,
			Amount:      decimal.NewFromFloat(10.00),
			Currency:    "SGD",
			Description: "Original",
			Status:      models.ExpenseStatusConfirmed,
		}
		err := repo.Create(ctx, expense)
		require.NoError(t, err)

		// Update to empty description
		expense.Description = ""
		err = repo.Update(ctx, expense)
		require.NoError(t, err)

		// Verify
		retrieved, err := repo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.Equal(t, "", retrieved.Description)
	})

	t.Run("update status from draft to confirmed", func(t *testing.T) {
		// Create draft
		expense := &models.Expense{
			UserID:      123,
			Amount:      decimal.NewFromFloat(10.00),
			Currency:    "SGD",
			Description: "Test",
			Status:      models.ExpenseStatusDraft,
		}
		err := repo.Create(ctx, expense)
		require.NoError(t, err)

		// Update to confirmed
		expense.Status = models.ExpenseStatusConfirmed
		err = repo.Update(ctx, expense)
		require.NoError(t, err)

		// Verify
		retrieved, err := repo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.Equal(t, models.ExpenseStatusConfirmed, retrieved.Status)
	})
}

// TestExpenseRepository_DeleteEdgeCases tests edge cases for expense deletion.
func TestExpenseRepository_DeleteEdgeCases(t *testing.T) {
	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	// Create test user
	userRepo := NewUserRepository(pool)
	err = userRepo.UpsertUser(ctx, &models.User{
		ID:        123,
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
	})
	require.NoError(t, err)

	repo := NewExpenseRepository(pool)

	t.Run("delete non-existent expense", func(t *testing.T) {
		// Delete doesn't check rows affected, so it succeeds silently
		err := repo.Delete(ctx, 99999)
		require.NoError(t, err) // No error, just no rows affected
	})

	t.Run("delete already deleted expense", func(t *testing.T) {
		// Create and delete
		expense := &models.Expense{
			UserID:      123,
			Amount:      decimal.NewFromFloat(10.00),
			Currency:    "SGD",
			Description: "Test",
			Status:      models.ExpenseStatusConfirmed,
		}
		err := repo.Create(ctx, expense)
		require.NoError(t, err)

		err = repo.Delete(ctx, expense.ID)
		require.NoError(t, err)

		// Try to delete again - succeeds but affects 0 rows
		err = repo.Delete(ctx, expense.ID)
		require.NoError(t, err)
	})
}

// TestExpenseRepository_GetByIDEdgeCases tests edge cases for GetByID.
func TestExpenseRepository_GetByIDEdgeCases(t *testing.T) {
	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	repo := NewExpenseRepository(pool)

	t.Run("get with invalid ID", func(t *testing.T) {
		expense, err := repo.GetByID(ctx, 99999)
		require.Error(t, err)
		require.Nil(t, expense)
	})

	t.Run("get with zero ID", func(t *testing.T) {
		expense, err := repo.GetByID(ctx, 0)
		require.Error(t, err)
		require.Nil(t, expense)
	})

	t.Run("get with negative ID", func(t *testing.T) {
		expense, err := repo.GetByID(ctx, -1)
		require.Error(t, err)
		require.Nil(t, expense)
	})
}

// TestExpenseRepository_GetByUserIDAndDateRangeEdgeCases tests date range edge cases.
func TestExpenseRepository_GetByUserIDAndDateRangeEdgeCases(t *testing.T) {
	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	// Create test user
	userRepo := NewUserRepository(pool)
	err = userRepo.UpsertUser(ctx, &models.User{
		ID:        123,
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
	})
	require.NoError(t, err)

	repo := NewExpenseRepository(pool)

	now := time.Now()

	// Create test expense
	expense := &models.Expense{
		UserID:      123,
		Amount:      decimal.NewFromFloat(10.00),
		Currency:    "SGD",
		Description: "Test",
		Status:      models.ExpenseStatusConfirmed,
	}
	err = repo.Create(ctx, expense)
	require.NoError(t, err)

	t.Run("end time before start time", func(t *testing.T) {
		start := now
		end := now.Add(-24 * time.Hour) // End before start

		expenses, err := repo.GetByUserIDAndDateRange(ctx, 123, start, end)
		require.NoError(t, err)
		require.Empty(t, expenses) // Should return empty, not error
	})

	t.Run("very wide date range", func(t *testing.T) {
		start := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2100, 12, 31, 23, 59, 59, 0, time.UTC)

		expenses, err := repo.GetByUserIDAndDateRange(ctx, 123, start, end)
		require.NoError(t, err)
		require.NotEmpty(t, expenses)
	})

	t.Run("exact timestamp range", func(t *testing.T) {
		// Query with exact created timestamp
		start := expense.CreatedAt.Add(-1 * time.Second)
		end := expense.CreatedAt.Add(1 * time.Second)

		expenses, err := repo.GetByUserIDAndDateRange(ctx, 123, start, end)
		require.NoError(t, err)
		require.NotEmpty(t, expenses)
	})

	t.Run("user with no expenses", func(t *testing.T) {
		start := now.Add(-24 * time.Hour)
		end := now

		expenses, err := repo.GetByUserIDAndDateRange(ctx, 999999, start, end)
		require.NoError(t, err)
		require.Empty(t, expenses)
	})
}

// TestExpenseRepository_DeleteExpiredDraftsEdgeCases tests draft cleanup edge cases.
func TestExpenseRepository_DeleteExpiredDraftsEdgeCases(t *testing.T) {
	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	// Create test user
	userRepo := NewUserRepository(pool)
	err = userRepo.UpsertUser(ctx, &models.User{
		ID:        123,
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
	})
	require.NoError(t, err)

	repo := NewExpenseRepository(pool)

	t.Run("no drafts to delete", func(t *testing.T) {
		count, err := repo.DeleteExpiredDrafts(ctx, 10*time.Minute)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	t.Run("only recent drafts exist", func(t *testing.T) {
		// Create recent draft
		expense := &models.Expense{
			UserID:      123,
			Amount:      decimal.NewFromFloat(10.00),
			Currency:    "SGD",
			Description: "Recent draft",
			Status:      models.ExpenseStatusDraft,
		}
		err := repo.Create(ctx, expense)
		require.NoError(t, err)

		// Try to delete with short timeout
		count, err := repo.DeleteExpiredDrafts(ctx, 1*time.Nanosecond)
		require.NoError(t, err)
		require.Equal(t, 1, count) // Should delete it

		// Verify deleted
		_, err = repo.GetByID(ctx, expense.ID)
		require.Error(t, err)
	})

	t.Run("confirmed expenses not affected", func(t *testing.T) {
		// Create confirmed expense
		expense := &models.Expense{
			UserID:      123,
			Amount:      decimal.NewFromFloat(10.00),
			Currency:    "SGD",
			Description: "Confirmed",
			Status:      models.ExpenseStatusConfirmed,
		}
		err := repo.Create(ctx, expense)
		require.NoError(t, err)

		// Try to delete drafts
		count, err := repo.DeleteExpiredDrafts(ctx, 1*time.Nanosecond)
		require.NoError(t, err)
		require.Equal(t, 0, count) // Should not delete confirmed

		// Verify still exists
		retrieved, err := repo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
	})
}
