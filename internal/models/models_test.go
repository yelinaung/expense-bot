package models

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestUser(t *testing.T) {
	t.Parallel()

	t.Run("creates user with all fields", func(t *testing.T) {
		t.Parallel()
		now := time.Now()
		user := User{
			ID:        12345,
			Username:  "testuser",
			FirstName: "Test",
			LastName:  "User",
			CreatedAt: now,
			UpdatedAt: now,
		}

		require.Equal(t, int64(12345), user.ID)
		require.Equal(t, "testuser", user.Username)
		require.Equal(t, "Test", user.FirstName)
		require.Equal(t, "User", user.LastName)
	})
}

func TestCategory(t *testing.T) {
	t.Parallel()

	t.Run("creates category with all fields", func(t *testing.T) {
		t.Parallel()
		now := time.Now()
		cat := Category{
			ID:        1,
			Name:      "Food - Dining Out",
			CreatedAt: now,
		}

		require.Equal(t, 1, cat.ID)
		require.Equal(t, "Food - Dining Out", cat.Name)
	})
}

func TestExpense(t *testing.T) {
	t.Parallel()

	t.Run("creates expense with all fields", func(t *testing.T) {
		t.Parallel()
		now := time.Now()
		catID := 5
		expense := Expense{
			ID:            1,
			UserID:        12345,
			Amount:        decimal.NewFromFloat(25.50),
			Currency:      "SGD",
			Description:   "Lunch",
			CategoryID:    &catID,
			ReceiptFileID: "file123",
			CreatedAt:     now,
			UpdatedAt:     now,
		}

		require.Equal(t, 1, expense.ID)
		require.Equal(t, int64(12345), expense.UserID)
		require.True(t, decimal.NewFromFloat(25.50).Equal(expense.Amount))
		require.Equal(t, "SGD", expense.Currency)
		require.Equal(t, "Lunch", expense.Description)
		require.NotNil(t, expense.CategoryID)
		require.Equal(t, 5, *expense.CategoryID)
	})

	t.Run("creates expense without category", func(t *testing.T) {
		t.Parallel()
		expense := Expense{
			ID:         2,
			UserID:     12345,
			Amount:     decimal.NewFromFloat(10.00),
			Currency:   "SGD",
			CategoryID: nil,
		}

		require.Nil(t, expense.CategoryID)
	})

	t.Run("expense with category object", func(t *testing.T) {
		t.Parallel()
		cat := &Category{ID: 1, Name: "Food"}
		expense := Expense{
			ID:       3,
			Category: cat,
		}

		require.NotNil(t, expense.Category)
		require.Equal(t, "Food", expense.Category.Name)
	})
}
