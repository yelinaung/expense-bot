package exchange

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
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

	t.Run("coalesces concurrent refreshes for same expired key", func(t *testing.T) {
		t.Parallel()

		upstream := &countingService{
			rate:  decimal.RequireFromString("1.4"),
			date:  time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
			delay: 20 * time.Millisecond,
		}
		svc := NewCachedService(upstream, time.Nanosecond)

		_, err := svc.Convert(context.Background(), decimal.RequireFromString("10"), "USD", "SGD")
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				_, _ = svc.Convert(context.Background(), decimal.RequireFromString("10"), "USD", "SGD")
			})
		}
		wg.Wait()

		require.Equal(t, 2, upstream.calls)
	})

	t.Run("prunes expired entries to prevent unbounded growth", func(t *testing.T) {
		t.Parallel()

		upstream := &countingService{
			rate: decimal.RequireFromString("1.1"),
			date: time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
		}
		svc := NewCachedService(upstream, time.Nanosecond)

		_, err := svc.Convert(context.Background(), decimal.RequireFromString("10"), "USD", "SGD")
		require.NoError(t, err)
		_, err = svc.Convert(context.Background(), decimal.RequireFromString("10"), "EUR", "SGD")
		require.NoError(t, err)

		time.Sleep(time.Millisecond)

		_, err = svc.Convert(context.Background(), decimal.RequireFromString("10"), "GBP", "SGD")
		require.NoError(t, err)

		svc.mu.RLock()
		cachedEntries := len(svc.rates)
		svc.mu.RUnlock()
		require.LessOrEqual(t, cachedEntries, 1)
	})
}

func TestCachedService_RejectsNonPositiveRate(t *testing.T) {
	t.Parallel()

	upstream := &negativeRateService{}
	svc := NewCachedService(upstream, time.Hour)
	_, err := svc.Convert(context.Background(), decimal.RequireFromString("10"), "USD", "SGD")
	require.ErrorIs(t, err, errInvalidNonPositiveRate)
}

func TestCachedService_CoalescingContextSemantics(t *testing.T) {
	t.Parallel()

	t.Run("short caller timeout does not fail concurrent longer caller", func(t *testing.T) {
		t.Parallel()

		upstream := &countingService{
			rate:  decimal.RequireFromString("1.4"),
			date:  time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
			delay: 40 * time.Millisecond,
		}
		svc := NewCachedService(upstream, time.Nanosecond)

		_, err := svc.Convert(context.Background(), decimal.RequireFromString("10"), "USD", "SGD")
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		shortCtx, cancelShort := context.WithTimeout(context.Background(), 5*time.Millisecond)
		defer cancelShort()
		longCtx, cancelLong := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancelLong()

		errCh := make(chan error, 2)
		go func() {
			_, convErr := svc.Convert(shortCtx, decimal.RequireFromString("10"), "USD", "SGD")
			errCh <- convErr
		}()
		go func() {
			_, convErr := svc.Convert(longCtx, decimal.RequireFromString("10"), "USD", "SGD")
			errCh <- convErr
		}()

		err1 := <-errCh
		err2 := <-errCh

		require.True(t, errors.Is(err1, context.DeadlineExceeded) || errors.Is(err2, context.DeadlineExceeded))
		require.True(t, (err1 == nil && errors.Is(err2, context.DeadlineExceeded)) ||
			(err2 == nil && errors.Is(err1, context.DeadlineExceeded)))
		require.Equal(t, 2, upstream.calls)
	})

	t.Run("waiting caller respects its own cancellation", func(t *testing.T) {
		t.Parallel()

		upstream := &countingService{
			rate:  decimal.RequireFromString("1.3"),
			date:  time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
			delay: 100 * time.Millisecond,
		}
		svc := NewCachedService(upstream, time.Nanosecond)

		_, err := svc.Convert(context.Background(), decimal.RequireFromString("10"), "USD", "SGD")
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		longCtx, cancelLong := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancelLong()
		go func() {
			_, _ = svc.Convert(longCtx, decimal.RequireFromString("10"), "USD", "SGD")
		}()

		shortCtx, cancelShort := context.WithTimeout(context.Background(), 5*time.Millisecond)
		defer cancelShort()
		_, err = svc.Convert(shortCtx, decimal.RequireFromString("10"), "USD", "SGD")
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
}

type negativeRateService struct {
	calls atomic.Int32
}

func (s *negativeRateService) Convert(
	_ context.Context,
	amount decimal.Decimal,
	_, _ string,
) (ConversionResult, error) {
	s.calls.Add(1)
	return ConversionResult{
		Amount:   amount,
		Rate:     decimal.Zero,
		RateDate: time.Now().UTC(),
	}, nil
}
