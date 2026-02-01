package repository

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestExpenseRepository_GetByUserIDAndCategory(t *testing.T) {
	t.Parallel()

	tx := database.TestTx(t)
	repo := NewExpenseRepository(tx)
	catRepo := NewCategoryRepository(tx)
	userRepo := NewUserRepository(tx)
	ctx := context.Background()

	// Create test user
	userID := int64(800001)
	err := userRepo.UpsertUser(ctx, &models.User{
		ID:        userID,
		Username:  "testuser",
		FirstName: "Test",
	})
	require.NoError(t, err)

	// Create test category
	category, err := catRepo.Create(ctx, "Test Category Filter")
	require.NoError(t, err)

	// Create another category for control
	otherCategory, err := catRepo.Create(ctx, "Other Category")
	require.NoError(t, err)

	// Create expenses in target category
	for i := 1; i <= 5; i++ {
		expense := &models.Expense{
			UserID:      userID,
			Amount:      decimal.NewFromFloat(10.50),
			Currency:    "SGD",
			Description: "Test expense",
			CategoryID:  &category.ID,
			Status:      models.ExpenseStatusConfirmed,
		}
		err := repo.Create(ctx, expense)
		require.NoError(t, err)
	}

	// Create expenses in other category (should not be returned)
	for i := 1; i <= 3; i++ {
		expense := &models.Expense{
			UserID:      userID,
			Amount:      decimal.NewFromFloat(5.00),
			Currency:    "SGD",
			Description: "Other expense",
			CategoryID:  &otherCategory.ID,
			Status:      models.ExpenseStatusConfirmed,
		}
		err := repo.Create(ctx, expense)
		require.NoError(t, err)
	}

	// Create draft expense (should not be returned)
	draftExpense := &models.Expense{
		UserID:      userID,
		Amount:      decimal.NewFromFloat(1.00),
		Currency:    "SGD",
		Description: "Draft",
		CategoryID:  &category.ID,
		Status:      models.ExpenseStatusDraft,
	}
	err = repo.Create(ctx, draftExpense)
	require.NoError(t, err)

	t.Run("returns only expenses from target category", func(t *testing.T) {
		expenses, err := repo.GetByUserIDAndCategory(ctx, userID, category.ID, 10)
		require.NoError(t, err)
		require.Len(t, expenses, 5, "should return only 5 expenses from target category")

		for _, exp := range expenses {
			require.Equal(t, userID, exp.UserID)
			require.Equal(t, &category.ID, exp.CategoryID)
			require.Equal(t, models.ExpenseStatusConfirmed, exp.Status)
			require.NotNil(t, exp.Category)
			require.Equal(t, category.Name, exp.Category.Name)
		}
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		expenses, err := repo.GetByUserIDAndCategory(ctx, userID, category.ID, 3)
		require.NoError(t, err)
		require.Len(t, expenses, 3, "should respect limit of 3")
	})

	t.Run("returns empty list for category with no expenses", func(t *testing.T) {
		emptyCategory, err := catRepo.Create(ctx, "Empty Category")
		require.NoError(t, err)

		expenses, err := repo.GetByUserIDAndCategory(ctx, userID, emptyCategory.ID, 10)
		require.NoError(t, err)
		require.Empty(t, expenses)
	})

	t.Run("filters by user ID", func(t *testing.T) {
		differentUserID := int64(800002)
		expenses, err := repo.GetByUserIDAndCategory(ctx, differentUserID, category.ID, 10)
		require.NoError(t, err)
		require.Empty(t, expenses, "should return empty for different user")
	})
}

func TestExpenseRepository_GetTotalByUserIDAndCategory(t *testing.T) {
	t.Parallel()

	tx := database.TestTx(t)
	repo := NewExpenseRepository(tx)
	catRepo := NewCategoryRepository(tx)
	userRepo := NewUserRepository(tx)
	ctx := context.Background()

	// Create test user
	userID := int64(800003)
	err := userRepo.UpsertUser(ctx, &models.User{
		ID:        userID,
		Username:  "totaluser",
		FirstName: "Total",
	})
	require.NoError(t, err)

	// Create test category
	category, err := catRepo.Create(ctx, "Total Test Category")
	require.NoError(t, err)

	// Create expenses with known amounts
	amounts := []float64{10.50, 20.00, 5.25, 15.75}
	expectedTotal := 0.0
	for _, amount := range amounts {
		expectedTotal += amount
		expense := &models.Expense{
			UserID:      userID,
			Amount:      decimal.NewFromFloat(amount),
			Currency:    "SGD",
			Description: "Test",
			CategoryID:  &category.ID,
			Status:      models.ExpenseStatusConfirmed,
		}
		err := repo.Create(ctx, expense)
		require.NoError(t, err)
	}

	// Create draft expense (should not be counted)
	draftExpense := &models.Expense{
		UserID:      userID,
		Amount:      decimal.NewFromFloat(100.00),
		Currency:    "SGD",
		Description: "Draft",
		CategoryID:  &category.ID,
		Status:      models.ExpenseStatusDraft,
	}
	err = repo.Create(ctx, draftExpense)
	require.NoError(t, err)

	t.Run("calculates total correctly", func(t *testing.T) {
		total, err := repo.GetTotalByUserIDAndCategory(ctx, userID, category.ID)
		require.NoError(t, err)
		expected := decimal.NewFromFloat(expectedTotal)
		require.True(t, expected.Equal(total), "expected %s, got %s", expected, total)
	})

	t.Run("returns zero for empty category", func(t *testing.T) {
		emptyCategory, err := catRepo.Create(ctx, "Empty Total Category")
		require.NoError(t, err)

		total, err := repo.GetTotalByUserIDAndCategory(ctx, userID, emptyCategory.ID)
		require.NoError(t, err)
		require.True(t, total.IsZero())
	})

	t.Run("filters by user ID", func(t *testing.T) {
		differentUserID := int64(800004)
		total, err := repo.GetTotalByUserIDAndCategory(ctx, differentUserID, category.ID)
		require.NoError(t, err)
		require.True(t, total.IsZero(), "should return zero for different user")
	})
}

func TestExpenseRepository_CategoryFilterEdgeCases(t *testing.T) {
	t.Parallel()

	tx := database.TestTx(t)
	repo := NewExpenseRepository(tx)
	catRepo := NewCategoryRepository(tx)
	userRepo := NewUserRepository(tx)
	ctx := context.Background()

	// Create test user
	userID := int64(800005)
	err := userRepo.UpsertUser(ctx, &models.User{
		ID:        userID,
		Username:  "edgeuser",
		FirstName: "Edge",
	})
	require.NoError(t, err)

	category, err := catRepo.Create(ctx, "Edge Case Category")
	require.NoError(t, err)

	t.Run("handles non-existent category ID", func(t *testing.T) {
		nonExistentID := 999999
		expenses, err := repo.GetByUserIDAndCategory(ctx, userID, nonExistentID, 10)
		require.NoError(t, err)
		require.Empty(t, expenses)
	})

	t.Run("orders by created_at DESC", func(t *testing.T) {
		// Create expenses with slight time delay
		// Note: Within a transaction, NOW() returns the same value
		// So we check ordering by ID instead (which is SERIAL and auto-incrementing)
		var ids []int
		for i := 1; i <= 3; i++ {
			expense := &models.Expense{
				UserID:      userID,
				Amount:      decimal.NewFromInt(int64(i)),
				Currency:    "SGD",
				Description: "Test",
				CategoryID:  &category.ID,
				Status:      models.ExpenseStatusConfirmed,
			}
			err := repo.Create(ctx, expense)
			require.NoError(t, err)
			ids = append(ids, expense.ID)
		}

		expenses, err := repo.GetByUserIDAndCategory(ctx, userID, category.ID, 10)
		require.NoError(t, err)
		require.Len(t, expenses, 3)

		// Should be in reverse order by ID (newest first, since IDs are sequential)
		// Since created_at is the same in a transaction, ordering is by ID DESC as fallback
		require.Equal(t, ids[2], expenses[0].ID, "newest expense should be first")
		require.Equal(t, ids[1], expenses[1].ID, "middle expense should be second")
		require.Equal(t, ids[0], expenses[2].ID, "oldest expense should be last")
	})
}
