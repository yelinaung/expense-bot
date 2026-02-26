package exchange

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestNewCachedService_DefaultTTLWhenNonPositive(t *testing.T) {
	t.Parallel()

	upstream := &countingService{
		rate: decimal.RequireFromString("1.2"),
		date: time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
	}
	svc := NewCachedService(upstream, 0, nil)
	require.Equal(t, 12*time.Hour, svc.ttl)
}

func TestCachedService_Convert_InnerNil(t *testing.T) {
	t.Parallel()

	svc := NewCachedService(nil, time.Hour, nil)
	_, err := svc.Convert(context.Background(), decimal.RequireFromString("10"), "USD", "SGD")
	require.Error(t, err)
	require.Contains(t, err.Error(), "inner exchange service is required")
}
