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
