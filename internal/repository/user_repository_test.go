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

func TestUserRepository_UpsertUser(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	repo := NewUserRepository(tx)

	t.Run("creates new user", func(t *testing.T) {
		user := &models.User{
			ID:        12345,
			Username:  "testuser",
			FirstName: "Test",
			LastName:  "User",
		}

		err := repo.UpsertUser(ctx, user)
		require.NoError(t, err)

		fetched, err := repo.GetUserByID(ctx, 12345)
		require.NoError(t, err)
		require.Equal(t, "testuser", fetched.Username)
		require.Equal(t, "Test", fetched.FirstName)
		require.Equal(t, "User", fetched.LastName)
	})

	t.Run("updates existing user", func(t *testing.T) {
		user := &models.User{
			ID:        12345,
			Username:  "updateduser",
			FirstName: "Updated",
			LastName:  "Name",
		}

		err := repo.UpsertUser(ctx, user)
		require.NoError(t, err)

		fetched, err := repo.GetUserByID(ctx, 12345)
		require.NoError(t, err)
		require.Equal(t, "updateduser", fetched.Username)
		require.Equal(t, "Updated", fetched.FirstName)
	})
}

func TestUserRepository_GetAllUsers(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	repo := NewUserRepository(tx)

	t.Run("returns empty when no users exist", func(t *testing.T) {
		users, err := repo.GetAllUsers(ctx)
		require.NoError(t, err)
		require.Empty(t, users)
	})

	t.Run("returns all created users", func(t *testing.T) {
		err := repo.UpsertUser(ctx, &models.User{ID: 1001, Username: "alice", FirstName: "Alice", LastName: "A"})
		require.NoError(t, err)
		err = repo.UpsertUser(ctx, &models.User{ID: 1002, Username: "bob", FirstName: "Bob", LastName: "B"})
		require.NoError(t, err)

		users, err := repo.GetAllUsers(ctx)
		require.NoError(t, err)
		require.Len(t, users, 2)

		ids := []int64{users[0].ID, users[1].ID}
		require.Contains(t, ids, int64(1001))
		require.Contains(t, ids, int64(1002))
	})
}

func TestUserRepository_GetUsersNeedingReminder(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	userRepo := NewUserRepository(tx)
	expenseRepo := NewExpenseRepository(tx)
	approvedRepo := NewApprovedUserRepository(tx)

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.AddDate(0, 0, 1)

	t.Run("returns superadmin without expenses", func(t *testing.T) {
		err := userRepo.UpsertUser(ctx, &models.User{ID: 3001, Username: "admin", FirstName: "Admin"})
		require.NoError(t, err)

		users, err := userRepo.GetUsersNeedingReminder(ctx, []int64{3001}, nil, startOfDay, endOfDay)
		require.NoError(t, err)
		require.Len(t, users, 1)
		require.Equal(t, int64(3001), users[0].ID)
	})

	t.Run("excludes superadmin with expenses", func(t *testing.T) {
		err := userRepo.UpsertUser(ctx, &models.User{ID: 3002, Username: "busyadmin", FirstName: "Busy"})
		require.NoError(t, err)

		expense := &models.Expense{
			UserID:      3002,
			Amount:      decimal.NewFromFloat(5.00),
			Currency:    "SGD",
			Description: "Coffee",
			Status:      models.ExpenseStatusConfirmed,
		}
		err = expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		users, err := userRepo.GetUsersNeedingReminder(ctx, []int64{3002}, nil, startOfDay, endOfDay)
		require.NoError(t, err)
		require.Empty(t, users)
	})

	t.Run("returns approved user without expenses", func(t *testing.T) {
		err := userRepo.UpsertUser(ctx, &models.User{ID: 3003, Username: "approveduser", FirstName: "Approved"})
		require.NoError(t, err)
		err = approvedRepo.Approve(ctx, 3003, "approveduser", 1)
		require.NoError(t, err)

		users, err := userRepo.GetUsersNeedingReminder(ctx, nil, nil, startOfDay, endOfDay)
		require.NoError(t, err)

		var found bool
		for _, u := range users {
			if u.ID == 3003 {
				found = true
				break
			}
		}
		require.True(t, found, "approved user should be included")
	})

	t.Run("excludes unapproved user", func(t *testing.T) {
		err := userRepo.UpsertUser(ctx, &models.User{ID: 3004, Username: "stranger", FirstName: "Stranger"})
		require.NoError(t, err)

		users, err := userRepo.GetUsersNeedingReminder(ctx, nil, nil, startOfDay, endOfDay)
		require.NoError(t, err)

		for _, u := range users {
			require.NotEqual(t, int64(3004), u.ID, "unapproved user should not be included")
		}
	})

	t.Run("matches superadmin by username", func(t *testing.T) {
		err := userRepo.UpsertUser(ctx, &models.User{ID: 3005, Username: "UsernameAdmin", FirstName: "UAdmin"})
		require.NoError(t, err)

		users, err := userRepo.GetUsersNeedingReminder(ctx, nil, []string{"UsernameAdmin"}, startOfDay, endOfDay)
		require.NoError(t, err)

		var found bool
		for _, u := range users {
			if u.ID == 3005 {
				found = true
				break
			}
		}
		require.True(t, found, "username-whitelisted user should be included")
	})
}

func TestUserRepository_GetUserByID(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	repo := NewUserRepository(tx)

	t.Run("returns error for non-existent user", func(t *testing.T) {
		_, err := repo.GetUserByID(ctx, 99999)
		require.Error(t, err)
	})
}

func TestUserRepository_UpsertUser_WithEmptyFields(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	repo := NewUserRepository(tx)

	// Create user with minimal fields.
	user := &models.User{
		ID:        54321,
		Username:  "",
		FirstName: "",
		LastName:  "",
	}

	err := repo.UpsertUser(ctx, user)
	require.NoError(t, err)

	fetched, err := repo.GetUserByID(ctx, 54321)
	require.NoError(t, err)
	require.Empty(t, fetched.Username)
	require.Empty(t, fetched.FirstName)
	require.Empty(t, fetched.LastName)
}

func TestUserRepository_UpsertUser_UpdateToEmpty(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	repo := NewUserRepository(tx)

	// Create user with values.
	user := &models.User{
		ID:        65432,
		Username:  "originaluser",
		FirstName: "Original",
		LastName:  "User",
	}
	err := repo.UpsertUser(ctx, user)
	require.NoError(t, err)

	// Update to empty values.
	user.Username = ""
	user.FirstName = ""
	user.LastName = ""
	err = repo.UpsertUser(ctx, user)
	require.NoError(t, err)

	fetched, err := repo.GetUserByID(ctx, 65432)
	require.NoError(t, err)
	require.Empty(t, fetched.Username)
	require.Empty(t, fetched.FirstName)
	require.Empty(t, fetched.LastName)
}

func TestUserRepository_UpdateDefaultCurrency(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	repo := NewUserRepository(tx)

	// Create a user.
	user := &models.User{
		ID:        12345,
		Username:  "currencyuser",
		FirstName: "Currency",
		LastName:  "User",
	}
	err := repo.UpsertUser(ctx, user)
	require.NoError(t, err)

	t.Run("updates currency successfully", func(t *testing.T) {
		err := repo.UpdateDefaultCurrency(ctx, user.ID, "USD")
		require.NoError(t, err)

		currency, err := repo.GetDefaultCurrency(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, "USD", currency)
	})

	t.Run("updates currency to EUR", func(t *testing.T) {
		err := repo.UpdateDefaultCurrency(ctx, user.ID, "EUR")
		require.NoError(t, err)

		currency, err := repo.GetDefaultCurrency(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, "EUR", currency)
	})

	t.Run("succeeds silently for non-existent user", func(t *testing.T) {
		// Update doesn't fail for non-existent users, similar to other repository methods.
		err := repo.UpdateDefaultCurrency(ctx, 99999, "GBP")
		require.NoError(t, err)
	})
}

func TestUserRepository_GetDefaultCurrency(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	repo := NewUserRepository(tx)

	t.Run("returns SGD for new user", func(t *testing.T) {
		user := &models.User{
			ID:        54321,
			Username:  "newuser",
			FirstName: "New",
			LastName:  "User",
		}
		err := repo.UpsertUser(ctx, user)
		require.NoError(t, err)

		currency, err := repo.GetDefaultCurrency(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, "SGD", currency)
	})

	t.Run("returns error for non-existent user", func(t *testing.T) {
		_, err := repo.GetDefaultCurrency(ctx, 99999)
		require.Error(t, err)
	})
}
