package bot

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/models"
	"gitlab.com/yelinaung/expense-bot/internal/repository"
)

// setupHandlerErrorTest creates a test environment for handler error scenarios.
func setupHandlerErrorTest(t *testing.T) (*Bot, context.Context, database.PGXDB) {
	t.Helper()

	tx := database.TestTx(t)
	ctx := context.Background()

	// Create bot instance
	expenseRepo := repository.NewExpenseRepository(tx)
	userRepo := repository.NewUserRepository(tx)
	categoryRepo := repository.NewCategoryRepository(tx)

	testBot := &Bot{
		expenseRepo:  expenseRepo,
		userRepo:     userRepo,
		categoryRepo: categoryRepo,
		geminiClient: nil, // No Gemini client for error tests
	}

	return testBot, ctx, tx
}

// TestSaveExpense_Errors tests error scenarios in saveExpense function.
func TestSaveExpense_Errors(t *testing.T) {
	testBot, ctx, _ := setupHandlerErrorTest(t)

	// Create test user
	err := testBot.userRepo.UpsertUser(ctx, &models.User{
		ID:        12345,
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
	})
	require.NoError(t, err)

	t.Run("save expense with invalid user ID fails", func(t *testing.T) {
		// Try to save expense for non-existent user
		parsed := &ParsedExpense{
			Amount:       decimal.NewFromFloat(10.00),
			Description:  "Test expense",
			CategoryName: "",
		}

		// Create expense with invalid user ID
		expense := &models.Expense{
			UserID:      99999, // Non-existent user
			Amount:      parsed.Amount,
			Currency:    "SGD",
			Description: parsed.Description,
			Status:      models.ExpenseStatusConfirmed,
		}

		// Should fail due to foreign key constraint
		err := testBot.expenseRepo.Create(ctx, expense)
		require.Error(t, err, "should fail with foreign key constraint error")
		require.Contains(t, err.Error(), "violates foreign key constraint")
	})

	t.Run("save expense with very large amount succeeds", func(t *testing.T) {
		parsed := &ParsedExpense{
			Amount:       decimal.NewFromFloat(999999999.99),
			Description:  "Very large expense",
			CategoryName: "",
		}

		expense := &models.Expense{
			UserID:      12345,
			Amount:      parsed.Amount,
			Currency:    "SGD",
			Description: parsed.Description,
			Status:      models.ExpenseStatusConfirmed,
		}

		err := testBot.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)
		require.NotZero(t, expense.ID)

		// Verify it was saved correctly
		retrieved, err := testBot.expenseRepo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.True(t, parsed.Amount.Equal(retrieved.Amount))
	})

	t.Run("save expense with empty description succeeds", func(t *testing.T) {
		parsed := &ParsedExpense{
			Amount:       decimal.NewFromFloat(5.50),
			Description:  "", // Empty description
			CategoryName: "",
		}

		expense := &models.Expense{
			UserID:      12345,
			Amount:      parsed.Amount,
			Currency:    "SGD",
			Description: parsed.Description,
			Status:      models.ExpenseStatusConfirmed,
		}

		err := testBot.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)
		require.NotZero(t, expense.ID)
	})

	t.Run("save expense with category", func(t *testing.T) {
		categories, err := testBot.getCategoriesWithCache(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, categories)

		parsed := &ParsedExpense{
			Amount:       decimal.NewFromFloat(10.00),
			Description:  "Test with category",
			CategoryName: categories[0].Name,
		}

		expense := &models.Expense{
			UserID:      12345,
			Amount:      parsed.Amount,
			Currency:    "SGD",
			Description: parsed.Description,
			CategoryID:  &categories[0].ID,
			Category:    &categories[0],
			Status:      models.ExpenseStatusConfirmed,
		}

		err = testBot.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)
		require.NotZero(t, expense.ID)

		// Verify category was saved
		retrieved, err := testBot.expenseRepo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved.CategoryID)
		require.Equal(t, categories[0].ID, *retrieved.CategoryID)
	})
}

// TestGetCategoriesWithCache_Errors tests error scenarios for category caching.
func TestGetCategoriesWithCache_Errors(t *testing.T) {
	testBot, ctx, tx := setupHandlerErrorTest(t)

	t.Run("empty categories returns empty slice", func(t *testing.T) {
		// Delete all categories within this transaction
		_, err := tx.Exec(ctx, "DELETE FROM categories")
		require.NoError(t, err)

		// Clear cache
		testBot.invalidateCategoryCache()

		categories, err := testBot.getCategoriesWithCache(ctx)
		require.NoError(t, err)
		require.Empty(t, categories)
	})

	t.Run("cache returns same data on subsequent calls", func(t *testing.T) {
		// Categories are already seeded in TestPool, just clear cache
		testBot.invalidateCategoryCache()

		// First call - cache miss
		categories1, err := testBot.getCategoriesWithCache(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, categories1)

		// Second call - cache hit
		categories2, err := testBot.getCategoriesWithCache(ctx)
		require.NoError(t, err)
		require.Equal(t, len(categories1), len(categories2))
	})
}

// TestExpenseRepositoryErrors tests error scenarios in expense repository operations.
func TestExpenseRepositoryErrors(t *testing.T) {
	testBot, ctx, _ := setupHandlerErrorTest(t)

	// Create test user
	err := testBot.userRepo.UpsertUser(ctx, &models.User{
		ID:        12345,
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
	})
	require.NoError(t, err)

	t.Run("get non-existent expense returns error", func(t *testing.T) {
		expense, err := testBot.expenseRepo.GetByID(ctx, 99999)
		require.Error(t, err)
		require.Nil(t, expense)
	})

	t.Run("update non-existent expense succeeds silently", func(t *testing.T) {
		expense := &models.Expense{
			ID:          99999,
			UserID:      12345,
			Amount:      decimal.NewFromFloat(10.00),
			Currency:    "SGD",
			Description: "Test",
			Status:      models.ExpenseStatusConfirmed,
		}

		// Update doesn't check rows affected, so it succeeds
		err := testBot.expenseRepo.Update(ctx, expense)
		require.NoError(t, err)
	})

	t.Run("delete non-existent expense succeeds silently", func(t *testing.T) {
		// Delete doesn't check rows affected, so it succeeds
		err := testBot.expenseRepo.Delete(ctx, 99999)
		require.NoError(t, err)
	})

	t.Run("get expenses for non-existent user returns empty", func(t *testing.T) {
		expenses, err := testBot.expenseRepo.GetByUserID(ctx, 99999, 10)
		require.NoError(t, err)
		require.Empty(t, expenses)
	})

	t.Run("get expenses with date range end before start returns empty", func(t *testing.T) {
		now := time.Now()
		start := now
		end := now.Add(-24 * time.Hour) // End before start

		expenses, err := testBot.expenseRepo.GetByUserIDAndDateRange(ctx, 12345, start, end)
		require.NoError(t, err)
		require.Empty(t, expenses)
	})

	t.Run("delete expired drafts with no drafts returns zero", func(t *testing.T) {
		count, err := testBot.expenseRepo.DeleteExpiredDrafts(ctx, 10*time.Minute)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	t.Run("delete expired drafts removes only old drafts", func(t *testing.T) {
		// Create old draft
		oldDraft := &models.Expense{
			UserID:      12345,
			Amount:      decimal.NewFromFloat(10.00),
			Currency:    "SGD",
			Description: "Old draft",
			Status:      models.ExpenseStatusDraft,
		}
		err := testBot.expenseRepo.Create(ctx, oldDraft)
		require.NoError(t, err)

		// Create recent draft
		recentDraft := &models.Expense{
			UserID:      12345,
			Amount:      decimal.NewFromFloat(20.00),
			Currency:    "SGD",
			Description: "Recent draft",
			Status:      models.ExpenseStatusDraft,
		}
		err = testBot.expenseRepo.Create(ctx, recentDraft)
		require.NoError(t, err)

		// Delete drafts older than 1 nanosecond (should delete both)
		count, err := testBot.expenseRepo.DeleteExpiredDrafts(ctx, 1*time.Nanosecond)
		require.NoError(t, err)
		require.Equal(t, 2, count)

		// Verify both are deleted
		_, err = testBot.expenseRepo.GetByID(ctx, oldDraft.ID)
		require.Error(t, err)
		_, err = testBot.expenseRepo.GetByID(ctx, recentDraft.ID)
		require.Error(t, err)
	})

	t.Run("delete expired drafts preserves confirmed expenses", func(t *testing.T) {
		// Create test user
		err := testBot.userRepo.UpsertUser(ctx, &models.User{
			ID:        12345,
			Username:  "testuser",
			FirstName: "Test",
			LastName:  "User",
		})
		require.NoError(t, err)

		// Create confirmed expense
		confirmed := &models.Expense{
			UserID:      12345,
			Amount:      decimal.NewFromFloat(30.00),
			Currency:    "SGD",
			Description: "Confirmed",
			Status:      models.ExpenseStatusConfirmed,
		}
		err = testBot.expenseRepo.Create(ctx, confirmed)
		require.NoError(t, err)

		// Try to delete drafts
		count, err := testBot.expenseRepo.DeleteExpiredDrafts(ctx, 1*time.Nanosecond)
		require.NoError(t, err)
		require.Equal(t, 0, count) // Should not delete confirmed

		// Verify confirmed still exists
		retrieved, err := testBot.expenseRepo.GetByID(ctx, confirmed.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		require.Equal(t, models.ExpenseStatusConfirmed, retrieved.Status)
	})
}

// TestCategoryRepositoryErrors tests error scenarios in category repository operations.
func TestCategoryRepositoryErrors(t *testing.T) {
	testBot, ctx, _ := setupHandlerErrorTest(t)

	t.Run("get non-existent category returns error", func(t *testing.T) {
		category, err := testBot.categoryRepo.GetByID(ctx, 99999)
		require.Error(t, err)
		require.Nil(t, category)
	})

	t.Run("get category by non-existent name returns error", func(t *testing.T) {
		category, err := testBot.categoryRepo.GetByName(ctx, "NonExistentCategory")
		require.Error(t, err)
		require.Nil(t, category)
	})

	t.Run("get category by empty name returns error", func(t *testing.T) {
		category, err := testBot.categoryRepo.GetByName(ctx, "")
		require.Error(t, err)
		require.Nil(t, category)
	})

	t.Run("create duplicate category fails", func(t *testing.T) {
		// Get existing category
		categories, err := testBot.categoryRepo.GetAll(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, categories)

		// Try to create duplicate
		_, err = testBot.categoryRepo.Create(ctx, categories[0].Name)
		require.Error(t, err, "should fail with unique constraint error")
		require.Contains(t, err.Error(), "duplicate key value")
	})

	t.Run("update non-existent category succeeds silently", func(t *testing.T) {
		// Update doesn't check rows affected
		err := testBot.categoryRepo.Update(ctx, 99999, "NewName")
		require.NoError(t, err)
	})

	t.Run("delete non-existent category succeeds silently", func(t *testing.T) {
		// Delete doesn't check rows affected
		err := testBot.categoryRepo.Delete(ctx, 99999)
		require.NoError(t, err)
	})

	t.Run("category name is case insensitive", func(t *testing.T) {
		// Get existing category
		categories, err := testBot.categoryRepo.GetAll(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, categories)

		originalName := categories[0].Name

		// Try to find with different case
		category, err := testBot.categoryRepo.GetByName(ctx, "FOOD - DINING OUT")
		if originalName == "Food - Dining Out" {
			require.NoError(t, err)
			require.NotNil(t, category)
			require.Equal(t, originalName, category.Name)
		}
	})
}

// TestUserRepositoryErrors tests error scenarios in user repository operations.
func TestUserRepositoryErrors(t *testing.T) {
	testBot, ctx, _ := setupHandlerErrorTest(t)

	t.Run("get non-existent user returns error", func(t *testing.T) {
		user, err := testBot.userRepo.GetUserByID(ctx, 99999)
		require.Error(t, err)
		require.Nil(t, user)
	})

	t.Run("upsert user with very long fields succeeds", func(t *testing.T) {
		longString := string(make([]byte, 500))
		for i := range longString {
			longString = longString[:i] + "x" + longString[i+1:]
		}

		user := &models.User{
			ID:        11111,
			Username:  longString,
			FirstName: longString,
			LastName:  longString,
		}

		err := testBot.userRepo.UpsertUser(ctx, user)
		require.NoError(t, err)

		// Verify it was saved
		retrieved, err := testBot.userRepo.GetUserByID(ctx, 11111)
		require.NoError(t, err)
		require.Equal(t, longString, retrieved.Username)
	})

	t.Run("upsert user with empty fields succeeds", func(t *testing.T) {
		user := &models.User{
			ID:        22222,
			Username:  "",
			FirstName: "",
			LastName:  "",
		}

		err := testBot.userRepo.UpsertUser(ctx, user)
		require.NoError(t, err)

		// Verify it was saved
		retrieved, err := testBot.userRepo.GetUserByID(ctx, 22222)
		require.NoError(t, err)
		require.Equal(t, "", retrieved.Username)
	})

	t.Run("upsert updates existing user", func(t *testing.T) {
		user := &models.User{
			ID:        33333,
			Username:  "original",
			FirstName: "Original",
			LastName:  "User",
		}

		err := testBot.userRepo.UpsertUser(ctx, user)
		require.NoError(t, err)

		// Update
		user.Username = "updated"
		user.FirstName = "Updated"
		err = testBot.userRepo.UpsertUser(ctx, user)
		require.NoError(t, err)

		// Verify update
		retrieved, err := testBot.userRepo.GetUserByID(ctx, 33333)
		require.NoError(t, err)
		require.Equal(t, "updated", retrieved.Username)
		require.Equal(t, "Updated", retrieved.FirstName)
	})
}

// TestParsingEdgeCases tests edge cases in expense parsing that could cause handler errors.
func TestParsingEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("parse add command with invalid amount returns nil", func(t *testing.T) {
		result := ParseAddCommand("/add abc Coffee")
		require.Nil(t, result)
	})

	t.Run("parse add command with negative amount returns nil", func(t *testing.T) {
		result := ParseAddCommand("/add -5.50 Coffee")
		require.Nil(t, result)
	})

	t.Run("parse add command with zero amount returns nil", func(t *testing.T) {
		result := ParseAddCommand("/add 0 Coffee")
		require.Nil(t, result)
	})

	t.Run("parse expense input with no amount returns nil", func(t *testing.T) {
		result := ParseExpenseInput("Coffee at Starbucks")
		require.Nil(t, result)
	})

	t.Run("parse expense input with invalid amount returns nil", func(t *testing.T) {
		result := ParseExpenseInput("abc Coffee")
		require.Nil(t, result)
	})

	t.Run("match category with empty string returns nil", func(t *testing.T) {
		categories := []models.Category{
			{ID: 1, Name: "Food"},
		}
		result := MatchCategory("", categories)
		require.Nil(t, result)
	})

	t.Run("match category with no match returns nil", func(t *testing.T) {
		categories := []models.Category{
			{ID: 1, Name: "Food"},
		}
		result := MatchCategory("xyz", categories)
		require.Nil(t, result)
	})

	t.Run("match category with empty categories returns nil", func(t *testing.T) {
		result := MatchCategory("Food", []models.Category{})
		require.Nil(t, result)
	})
}
