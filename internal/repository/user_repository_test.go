package repository

import (
	"context"
	"testing"

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
