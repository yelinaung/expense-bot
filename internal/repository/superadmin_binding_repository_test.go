package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/database"
)

func TestSuperadminBindingRepository_SaveAndLoad(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	repo := NewSuperadminBindingRepository(tx)

	t.Run("empty on start", func(t *testing.T) {
		bindings, err := repo.LoadAll(ctx)
		require.NoError(t, err)
		require.Empty(t, bindings)
	})

	t.Run("save and load", func(t *testing.T) {
		err := repo.Save(ctx, "admin", 12345)
		require.NoError(t, err)

		bindings, err := repo.LoadAll(ctx)
		require.NoError(t, err)
		require.Len(t, bindings, 1)
		require.Equal(t, "admin", bindings[0].Username)
		require.Equal(t, int64(12345), bindings[0].UserID)
	})

	t.Run("upsert overwrites", func(t *testing.T) {
		err := repo.Save(ctx, "admin", 99999)
		require.NoError(t, err)

		bindings, err := repo.LoadAll(ctx)
		require.NoError(t, err)
		require.Len(t, bindings, 1)
		require.Equal(t, int64(99999), bindings[0].UserID)
	})

	t.Run("multiple bindings", func(t *testing.T) {
		err := repo.Save(ctx, "alice", 11111)
		require.NoError(t, err)

		bindings, err := repo.LoadAll(ctx)
		require.NoError(t, err)
		require.Len(t, bindings, 2)
	})
}
