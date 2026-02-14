package exchange

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type countingService struct {
	calls int
	rate  decimal.Decimal
	date  time.Time
	delay time.Duration
}

func (s *countingService) Convert(
	_ context.Context,
	amount decimal.Decimal,
	_, _ string,
) (ConversionResult, error) {
	s.calls++
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	return ConversionResult{
		Amount:   amount.Mul(s.rate).Round(2),
		Rate:     s.rate,
		RateDate: s.date,
	}, nil
}

func TestCachedService_Convert(t *testing.T) {
	t.Parallel()

	t.Run("uses cache for same pair", func(t *testing.T) {
		t.Parallel()
		upstream := &countingService{
			rate: decimal.RequireFromString("1.35"),
			date: time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
		}
		svc := NewCachedService(upstream, time.Hour)

		got1, err := svc.Convert(context.Background(), decimal.RequireFromString("10"), "USD", "SGD")
		require.NoError(t, err)
		require.Equal(t, decimal.RequireFromString("13.50"), got1.Amount)

		got2, err := svc.Convert(context.Background(), decimal.RequireFromString("20"), "USD", "SGD")
		require.NoError(t, err)
		require.Equal(t, decimal.RequireFromString("27.00"), got2.Amount)
		require.Equal(t, got1.Rate, got2.Rate)
		require.Equal(t, 1, upstream.calls)
	})

	t.Run("cache key is per pair", func(t *testing.T) {
		t.Parallel()
		upstream := &countingService{
			rate: decimal.RequireFromString("1.2"),
			date: time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
		}
		svc := NewCachedService(upstream, time.Hour)

		_, err := svc.Convert(context.Background(), decimal.RequireFromString("10"), "USD", "SGD")
		require.NoError(t, err)
		_, err = svc.Convert(context.Background(), decimal.RequireFromString("10"), "EUR", "SGD")
		require.NoError(t, err)
		require.Equal(t, 2, upstream.calls)
	})

	t.Run("expired entry triggers refresh", func(t *testing.T) {
		t.Parallel()
		upstream := &countingService{
			rate: decimal.RequireFromString("1.1"),
			date: time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
		}
		svc := NewCachedService(upstream, time.Nanosecond)

		_, err := svc.Convert(context.Background(), decimal.RequireFromString("10"), "USD", "SGD")
		require.NoError(t, err)
		time.Sleep(time.Millisecond)
		_, err = svc.Convert(context.Background(), decimal.RequireFromString("10"), "USD", "SGD")
		require.NoError(t, err)
		require.Equal(t, 2, upstream.calls)
	})

	t.Run("ttl starts after upstream fetch completes", func(t *testing.T) {
		t.Parallel()
		upstream := &countingService{
			rate:  decimal.RequireFromString("1.3"),
			date:  time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
			delay: 20 * time.Millisecond,
		}
		svc := NewCachedService(upstream, 10*time.Millisecond)

		_, err := svc.Convert(context.Background(), decimal.RequireFromString("10"), "USD", "SGD")
		require.NoError(t, err)
		_, err = svc.Convert(context.Background(), decimal.RequireFromString("10"), "USD", "SGD")
		require.NoError(t, err)

		// Should hit cache on second call despite slow upstream.
		require.Equal(t, 1, upstream.calls)
	})
}
