package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnect(t *testing.T) {
	t.Run("fails with invalid connection string", func(t *testing.T) {
		ctx := context.Background()
		pool, err := Connect(ctx, "invalid://connection")
		require.Error(t, err)
		require.Nil(t, pool)
	})

	t.Run("fails with unreachable host", func(t *testing.T) {
		ctx := context.Background()
		pool, err := Connect(ctx, "postgres://localhost:59999/nonexistent?connect_timeout=1")
		require.Error(t, err)
		require.Nil(t, pool)
	})
}
