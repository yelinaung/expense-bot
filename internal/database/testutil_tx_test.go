package database

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTestPool_ReturnsSharedPool(t *testing.T) {
	if os.Getenv("TEST_DATABASE_URL") == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	p1 := TestPool(t)
	p2 := TestPool(t)

	require.NotNil(t, p1)
	require.Same(t, p1, p2)
}

func TestTestTx_ReturnsUsableTx(t *testing.T) {
	if os.Getenv("TEST_DATABASE_URL") == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	db := TestTx(t)
	require.NotNil(t, db)

	var n int
	err := db.QueryRow(context.Background(), "SELECT 1").Scan(&n)
	require.NoError(t, err)
	require.Equal(t, 1, n)
}
