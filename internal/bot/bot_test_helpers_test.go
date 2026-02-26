package bot

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestTestExchangeServiceConvert(t *testing.T) {
	t.Parallel()

	svc := &testExchangeService{}
	amount := decimal.RequireFromString("12.34")

	t.Run("same currency keeps amount with unit rate", func(t *testing.T) {
		t.Parallel()
		result, err := svc.Convert(context.Background(), amount, "SGD", "SGD")
		require.NoError(t, err)
		require.True(t, amount.Equal(result.Amount))
		require.True(t, decimal.NewFromInt(1).Equal(result.Rate))
	})

	t.Run("different currency still returns deterministic conversion", func(t *testing.T) {
		t.Parallel()
		result, err := svc.Convert(context.Background(), amount, "USD", "SGD")
		require.NoError(t, err)
		require.True(t, amount.Equal(result.Amount))
		require.True(t, decimal.NewFromInt(1).Equal(result.Rate))
	})
}
