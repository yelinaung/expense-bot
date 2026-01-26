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

func setupExpenseTest(t *testing.T) (*ExpenseRepository, *UserRepository, *CategoryRepository, context.Context) {
	t.Helper()

	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	return NewExpenseRepository(pool),
		NewUserRepository(pool),
		NewCategoryRepository(pool),
		ctx
}

func TestExpenseRepository_Create(t *testing.T) {
	expenseRepo, userRepo, categoryRepo, ctx := setupExpenseTest(t)

	user := &models.User{ID: 111, Username: "testuser", FirstName: "Test", LastName: "User"}
	err := userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	cat, err := categoryRepo.Create(ctx, "Food - Dining Out")
	require.NoError(t, err)

	t.Run("creates expense with category", func(t *testing.T) {
		expense := &models.Expense{
			UserID:      111,
			Amount:      decimal.NewFromFloat(25.50),
			Currency:    "SGD",
			Description: "Lunch at hawker",
			CategoryID:  &cat.ID,
		}

		err := expenseRepo.Create(ctx, expense)
		require.NoError(t, err)
		require.NotZero(t, expense.ID)
		require.False(t, expense.CreatedAt.IsZero())
	})

	t.Run("creates expense without category", func(t *testing.T) {
		expense := &models.Expense{
			UserID:      111,
			Amount:      decimal.NewFromFloat(10.00),
			Currency:    "SGD",
			Description: "Misc expense",
			CategoryID:  nil,
		}

		err := expenseRepo.Create(ctx, expense)
		require.NoError(t, err)
		require.NotZero(t, expense.ID)
	})
}

func TestExpenseRepository_GetByID(t *testing.T) {
	expenseRepo, userRepo, _, ctx := setupExpenseTest(t)

	user := &models.User{ID: 222, Username: "user2", FirstName: "Test", LastName: "User"}
	err := userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	expense := &models.Expense{
		UserID:      222,
		Amount:      decimal.NewFromFloat(15.00),
		Currency:    "SGD",
		Description: "Coffee",
	}
	err = expenseRepo.Create(ctx, expense)
	require.NoError(t, err)

	t.Run("retrieves existing expense", func(t *testing.T) {
		fetched, err := expenseRepo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.Equal(t, expense.ID, fetched.ID)
		require.True(t, expense.Amount.Equal(fetched.Amount))
		require.Equal(t, "Coffee", fetched.Description)
	})

	t.Run("returns error for non-existent expense", func(t *testing.T) {
		_, err := expenseRepo.GetByID(ctx, 99999)
		require.Error(t, err)
	})
}

func TestExpenseRepository_GetByUserID(t *testing.T) {
	expenseRepo, userRepo, categoryRepo, ctx := setupExpenseTest(t)

	user := &models.User{ID: 333, Username: "user3", FirstName: "Test", LastName: "User"}
	err := userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	cat, err := categoryRepo.Create(ctx, "Transportation")
	require.NoError(t, err)

	for i := range 5 {
		expense := &models.Expense{
			UserID:      333,
			Amount:      decimal.NewFromFloat(float64(i + 1)),
			Currency:    "SGD",
			Description: "Expense",
			CategoryID:  &cat.ID,
		}
		err := expenseRepo.Create(ctx, expense)
		require.NoError(t, err)
	}

	t.Run("retrieves expenses with limit", func(t *testing.T) {
		expenses, err := expenseRepo.GetByUserID(ctx, 333, 3)
		require.NoError(t, err)
		require.Len(t, expenses, 3)
		require.NotNil(t, expenses[0].Category)
		require.Equal(t, "Transportation", expenses[0].Category.Name)
	})

	t.Run("returns empty for user with no expenses", func(t *testing.T) {
		expenses, err := expenseRepo.GetByUserID(ctx, 999, 10)
		require.NoError(t, err)
		require.Empty(t, expenses)
	})
}

func TestExpenseRepository_GetByUserIDAndDateRange(t *testing.T) {
	expenseRepo, userRepo, _, ctx := setupExpenseTest(t)

	user := &models.User{ID: 444, Username: "user4", FirstName: "Test", LastName: "User"}
	err := userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	expense := &models.Expense{
		UserID:      444,
		Amount:      decimal.NewFromFloat(50.00),
		Currency:    "SGD",
		Description: "Today expense",
	}
	err = expenseRepo.Create(ctx, expense)
	require.NoError(t, err)

	t.Run("retrieves expenses within date range", func(t *testing.T) {
		now := time.Now()
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		endOfDay := startOfDay.Add(24 * time.Hour)

		expenses, err := expenseRepo.GetByUserIDAndDateRange(ctx, 444, startOfDay, endOfDay)
		require.NoError(t, err)
		require.Len(t, expenses, 1)
	})

	t.Run("returns empty for date range with no expenses", func(t *testing.T) {
		pastStart := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		pastEnd := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)

		expenses, err := expenseRepo.GetByUserIDAndDateRange(ctx, 444, pastStart, pastEnd)
		require.NoError(t, err)
		require.Empty(t, expenses)
	})
}

func TestExpenseRepository_Update(t *testing.T) {
	expenseRepo, userRepo, categoryRepo, ctx := setupExpenseTest(t)

	user := &models.User{ID: 555, Username: "user5", FirstName: "Test", LastName: "User"}
	err := userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	cat, err := categoryRepo.Create(ctx, "Entertainment Update Test")
	require.NoError(t, err)

	expense := &models.Expense{
		UserID:      555,
		Amount:      decimal.NewFromFloat(20.00),
		Currency:    "SGD",
		Description: "Original",
	}
	err = expenseRepo.Create(ctx, expense)
	require.NoError(t, err)

	t.Run("updates expense fields", func(t *testing.T) {
		expense.Amount = decimal.NewFromFloat(30.00)
		expense.Description = "Updated"
		expense.CategoryID = &cat.ID

		err := expenseRepo.Update(ctx, expense)
		require.NoError(t, err)

		fetched, err := expenseRepo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.True(t, decimal.NewFromFloat(30.00).Equal(fetched.Amount))
		require.Equal(t, "Updated", fetched.Description)
		require.NotNil(t, fetched.CategoryID)
	})
}

func TestExpenseRepository_Delete(t *testing.T) {
	expenseRepo, userRepo, _, ctx := setupExpenseTest(t)

	user := &models.User{ID: 666, Username: "user6", FirstName: "Test", LastName: "User"}
	err := userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	expense := &models.Expense{
		UserID:      666,
		Amount:      decimal.NewFromFloat(10.00),
		Currency:    "SGD",
		Description: "To delete",
	}
	err = expenseRepo.Create(ctx, expense)
	require.NoError(t, err)

	t.Run("deletes expense", func(t *testing.T) {
		err := expenseRepo.Delete(ctx, expense.ID)
		require.NoError(t, err)

		_, err = expenseRepo.GetByID(ctx, expense.ID)
		require.Error(t, err)
	})
}

func TestExpenseRepository_DeleteExpiredDrafts(t *testing.T) {
	expenseRepo, userRepo, _, ctx := setupExpenseTest(t)

	user := &models.User{ID: 888, Username: "user8", FirstName: "Test", LastName: "User"}
	err := userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	t.Run("deletes expired draft expenses", func(t *testing.T) {
		draftExpense := &models.Expense{
			UserID:      888,
			Amount:      decimal.NewFromFloat(10.00),
			Currency:    "SGD",
			Description: "Draft expense",
			Status:      models.ExpenseStatusDraft,
		}
		err := expenseRepo.Create(ctx, draftExpense)
		require.NoError(t, err)

		confirmedExpense := &models.Expense{
			UserID:      888,
			Amount:      decimal.NewFromFloat(20.00),
			Currency:    "SGD",
			Description: "Confirmed expense",
			Status:      models.ExpenseStatusConfirmed,
		}
		err = expenseRepo.Create(ctx, confirmedExpense)
		require.NoError(t, err)

		count, err := expenseRepo.DeleteExpiredDrafts(ctx, -1*time.Hour)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		_, err = expenseRepo.GetByID(ctx, draftExpense.ID)
		require.Error(t, err)

		fetched, err := expenseRepo.GetByID(ctx, confirmedExpense.ID)
		require.NoError(t, err)
		require.Equal(t, confirmedExpense.ID, fetched.ID)
	})

	t.Run("does not delete recent drafts", func(t *testing.T) {
		database.CleanupTables(t, expenseRepo.pool)

		user := &models.User{ID: 889, Username: "user9", FirstName: "Test", LastName: "User"}
		err := userRepo.UpsertUser(ctx, user)
		require.NoError(t, err)

		recentDraft := &models.Expense{
			UserID:      889,
			Amount:      decimal.NewFromFloat(15.00),
			Currency:    "SGD",
			Description: "Recent draft",
			Status:      models.ExpenseStatusDraft,
		}
		err = expenseRepo.Create(ctx, recentDraft)
		require.NoError(t, err)

		count, err := expenseRepo.DeleteExpiredDrafts(ctx, 10*time.Minute)
		require.NoError(t, err)
		require.Equal(t, 0, count)

		fetched, err := expenseRepo.GetByID(ctx, recentDraft.ID)
		require.NoError(t, err)
		require.Equal(t, recentDraft.ID, fetched.ID)
	})

	t.Run("returns zero when no expired drafts", func(t *testing.T) {
		database.CleanupTables(t, expenseRepo.pool)

		count, err := expenseRepo.DeleteExpiredDrafts(ctx, 10*time.Minute)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})
}

func TestExpenseRepository_GetTotalByUserIDAndDateRange(t *testing.T) {
	expenseRepo, userRepo, _, ctx := setupExpenseTest(t)

	user := &models.User{ID: 777, Username: "user7", FirstName: "Test", LastName: "User"}
	err := userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	amounts := []float64{10.50, 20.25, 30.75}
	for _, amt := range amounts {
		expense := &models.Expense{
			UserID:      777,
			Amount:      decimal.NewFromFloat(amt),
			Currency:    "SGD",
			Description: "Expense",
		}
		err := expenseRepo.Create(ctx, expense)
		require.NoError(t, err)
	}

	t.Run("calculates total correctly", func(t *testing.T) {
		now := time.Now()
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		endOfDay := startOfDay.Add(24 * time.Hour)

		total, err := expenseRepo.GetTotalByUserIDAndDateRange(ctx, 777, startOfDay, endOfDay)
		require.NoError(t, err)
		require.True(t, decimal.NewFromFloat(61.50).Equal(total))
	})

	t.Run("returns zero for empty range", func(t *testing.T) {
		pastStart := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		pastEnd := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)

		total, err := expenseRepo.GetTotalByUserIDAndDateRange(ctx, 777, pastStart, pastEnd)
		require.NoError(t, err)
		require.True(t, decimal.Zero.Equal(total))
	})
}
