package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestUserRepository_UpsertUser(t *testing.T) {
	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	repo := NewUserRepository(pool)

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

func TestUserRepository_GetUserByID(t *testing.T) {
	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	repo := NewUserRepository(pool)

	t.Run("returns error for non-existent user", func(t *testing.T) {
		_, err := repo.GetUserByID(ctx, 99999)
		require.Error(t, err)
	})
}

func TestUserRepository_UpsertUser_WithEmptyFields(t *testing.T) {
	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	repo := NewUserRepository(pool)

	// Create user with minimal fields.
	user := &models.User{
		ID:        54321,
		Username:  "",
		FirstName: "",
		LastName:  "",
	}

	err = repo.UpsertUser(ctx, user)
	require.NoError(t, err)

	fetched, err := repo.GetUserByID(ctx, 54321)
	require.NoError(t, err)
	require.Equal(t, "", fetched.Username)
	require.Equal(t, "", fetched.FirstName)
	require.Equal(t, "", fetched.LastName)
}

func TestUserRepository_UpsertUser_UpdateToEmpty(t *testing.T) {
	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	repo := NewUserRepository(pool)

	// Create user with values.
	user := &models.User{
		ID:        65432,
		Username:  "originaluser",
		FirstName: "Original",
		LastName:  "User",
	}
	err = repo.UpsertUser(ctx, user)
	require.NoError(t, err)

	// Update to empty values.
	user.Username = ""
	user.FirstName = ""
	user.LastName = ""
	err = repo.UpsertUser(ctx, user)
	require.NoError(t, err)

	fetched, err := repo.GetUserByID(ctx, 65432)
	require.NoError(t, err)
	require.Equal(t, "", fetched.Username)
	require.Equal(t, "", fetched.FirstName)
	require.Equal(t, "", fetched.LastName)
}

func TestUserRepository_UpdateDefaultCurrency(t *testing.T) {
	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	repo := NewUserRepository(pool)

	// Create a user.
	user := &models.User{
		ID:        12345,
		Username:  "currencyuser",
		FirstName: "Currency",
		LastName:  "User",
	}
	err = repo.UpsertUser(ctx, user)
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
	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	repo := NewUserRepository(pool)

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
