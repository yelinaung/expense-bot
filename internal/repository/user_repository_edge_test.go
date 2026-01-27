package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

// TestUserRepository_UpsertUserEdgeCases tests edge cases for user upsert.
func TestUserRepository_UpsertUserEdgeCases(t *testing.T) {
	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	repo := NewUserRepository(pool)

	t.Run("upsert with very long username", func(t *testing.T) {
		longUsername := string(make([]byte, 500))
		for i := range longUsername {
			longUsername = longUsername[:i] + "x" + longUsername[i+1:]
		}

		user := &models.User{
			ID:        123,
			Username:  longUsername,
			FirstName: "Test",
			LastName:  "User",
		}

		err := repo.UpsertUser(ctx, user)
		require.NoError(t, err)

		// Verify it was stored
		retrieved, err := repo.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, longUsername, retrieved.Username)
	})

	t.Run("upsert with very long first name", func(t *testing.T) {
		longFirstName := string(make([]byte, 500))
		for i := range longFirstName {
			longFirstName = longFirstName[:i] + "y" + longFirstName[i+1:]
		}

		user := &models.User{
			ID:        456,
			Username:  "testuser",
			FirstName: longFirstName,
			LastName:  "User",
		}

		err := repo.UpsertUser(ctx, user)
		require.NoError(t, err)

		retrieved, err := repo.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, longFirstName, retrieved.FirstName)
	})

	t.Run("upsert with very long last name", func(t *testing.T) {
		longLastName := string(make([]byte, 500))
		for i := range longLastName {
			longLastName = longLastName[:i] + "z" + longLastName[i+1:]
		}

		user := &models.User{
			ID:        789,
			Username:  "testuser",
			FirstName: "Test",
			LastName:  longLastName,
		}

		err := repo.UpsertUser(ctx, user)
		require.NoError(t, err)

		retrieved, err := repo.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, longLastName, retrieved.LastName)
	})

	t.Run("upsert with special characters", func(t *testing.T) {
		user := &models.User{
			ID:        111,
			Username:  "user_☕🍔",
			FirstName: "Café",
			LastName:  "O'Brien-Smith",
		}

		err := repo.UpsertUser(ctx, user)
		require.NoError(t, err)

		// Verify special characters preserved
		retrieved, err := repo.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, user.Username, retrieved.Username)
		require.Equal(t, user.FirstName, retrieved.FirstName)
		require.Equal(t, user.LastName, retrieved.LastName)
	})

	t.Run("upsert with empty fields", func(t *testing.T) {
		user := &models.User{
			ID:        222,
			Username:  "",
			FirstName: "",
			LastName:  "",
		}

		err := repo.UpsertUser(ctx, user)
		require.NoError(t, err) // Empty fields allowed

		retrieved, err := repo.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, "", retrieved.Username)
		require.Equal(t, "", retrieved.FirstName)
		require.Equal(t, "", retrieved.LastName)
	})

	t.Run("upsert with unicode characters", func(t *testing.T) {
		user := &models.User{
			ID:        333,
			Username:  "用户名",
			FirstName: "名前",
			LastName:  "Фамилия",
		}

		err := repo.UpsertUser(ctx, user)
		require.NoError(t, err)

		retrieved, err := repo.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, user.Username, retrieved.Username)
		require.Equal(t, user.FirstName, retrieved.FirstName)
		require.Equal(t, user.LastName, retrieved.LastName)
	})

	t.Run("update existing user - all fields change", func(t *testing.T) {
		// Create initial user
		user := &models.User{
			ID:        444,
			Username:  "original",
			FirstName: "Original",
			LastName:  "Name",
		}
		err := repo.UpsertUser(ctx, user)
		require.NoError(t, err)

		// Update all fields
		user.Username = "updated"
		user.FirstName = "Updated"
		user.LastName = "Names"
		err = repo.UpsertUser(ctx, user)
		require.NoError(t, err)

		// Verify update
		retrieved, err := repo.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, "updated", retrieved.Username)
		require.Equal(t, "Updated", retrieved.FirstName)
		require.Equal(t, "Names", retrieved.LastName)
	})

	t.Run("update existing user - all fields to empty", func(t *testing.T) {
		// Create initial user
		user := &models.User{
			ID:        555,
			Username:  "tobeemptied",
			FirstName: "ToBeEmptied",
			LastName:  "Last",
		}
		err := repo.UpsertUser(ctx, user)
		require.NoError(t, err)

		// Update all fields to empty
		user.Username = ""
		user.FirstName = ""
		user.LastName = ""
		err = repo.UpsertUser(ctx, user)
		require.NoError(t, err)

		// Verify all empty
		retrieved, err := repo.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, "", retrieved.Username)
		require.Equal(t, "", retrieved.FirstName)
		require.Equal(t, "", retrieved.LastName)
	})

	t.Run("upsert with newlines in fields", func(t *testing.T) {
		user := &models.User{
			ID:        666,
			Username:  "user\nname",
			FirstName: "First\nName",
			LastName:  "Last\nName",
		}

		err := repo.UpsertUser(ctx, user)
		require.NoError(t, err)

		retrieved, err := repo.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, user.Username, retrieved.Username)
	})

	t.Run("upsert with leading/trailing spaces", func(t *testing.T) {
		user := &models.User{
			ID:        777,
			Username:  "  spaced  ",
			FirstName: "  First  ",
			LastName:  "  Last  ",
		}

		err := repo.UpsertUser(ctx, user)
		require.NoError(t, err)

		// Verify spaces preserved
		retrieved, err := repo.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, "  spaced  ", retrieved.Username)
		require.Equal(t, "  First  ", retrieved.FirstName)
		require.Equal(t, "  Last  ", retrieved.LastName)
	})
}

// TestUserRepository_GetUserByIDEdgeCases tests edge cases for GetUserByID.
func TestUserRepository_GetUserByIDEdgeCases(t *testing.T) {
	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	repo := NewUserRepository(pool)

	t.Run("get non-existent user", func(t *testing.T) {
		user, err := repo.GetUserByID(ctx, 99999)
		require.Error(t, err)
		require.Nil(t, user)
		require.Contains(t, err.Error(), "failed to get user")
	})

	t.Run("get with zero ID", func(t *testing.T) {
		user, err := repo.GetUserByID(ctx, 0)
		require.Error(t, err)
		require.Nil(t, user)
	})

	t.Run("get with negative ID", func(t *testing.T) {
		user, err := repo.GetUserByID(ctx, -1)
		require.Error(t, err)
		require.Nil(t, user)
	})

	t.Run("get with very large ID", func(t *testing.T) {
		user, err := repo.GetUserByID(ctx, 9223372036854775807) // Max int64
		require.Error(t, err)
		require.Nil(t, user)
	})

	t.Run("get existing user", func(t *testing.T) {
		// Create user first
		testUser := &models.User{
			ID:        888,
			Username:  "existing",
			FirstName: "Existing",
			LastName:  "User",
		}
		err := repo.UpsertUser(ctx, testUser)
		require.NoError(t, err)

		// Retrieve
		retrieved, err := repo.GetUserByID(ctx, 888)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		require.Equal(t, testUser.ID, retrieved.ID)
		require.Equal(t, testUser.Username, retrieved.Username)
	})
}
