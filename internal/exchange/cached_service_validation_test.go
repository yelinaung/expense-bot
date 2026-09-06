package exchange

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// validatingConverter mirrors FrankfurterClient's input contract: it rejects
// non-positive amounts and otherwise returns amount*rate. It is used to verify
// that CachedService enforces the same input invariants regardless of cache
// warmth, so identical inputs yield identical observable results.
type validatingConverter struct {
	rate  decimal.Decimal
	calls atomic.Int32
}

func (v *validatingConverter) Convert(
	_ context.Context,
	amount decimal.Decimal,
	_, _ string,
) (ConversionResult, error) {
	v.calls.Add(1)
	if amount.IsNegative() || amount.IsZero() {
		return ConversionResult{}, errors.New("amount must be positive")
	}
	return ConversionResult{
		Amount:   amount.Mul(v.rate).Round(2),
		Rate:     v.rate,
		RateDate: time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
	}, nil
}

// TestCachedService_AmountValidationIsCacheIndependent pins the fix for the
// hit/miss validation divergence: a non-positive amount must be rejected
// identically whether the currency pair is cached (hit) or not (miss), so the
// wrapper's observable behavior does not depend on its internal cache state.
func TestCachedService_AmountValidationIsCacheIndependent(t *testing.T) {
	t.Parallel()

	inner := &validatingConverter{rate: decimal.RequireFromString("1.35")}
	svc := NewCachedService(inner, time.Hour, nil)

	// Warm the cache for USD->SGD with a positive amount.
	posRes, err := svc.Convert(context.Background(), decimal.RequireFromString("10"), "USD", "SGD")
	require.NoError(t, err)
	require.Equal(t, decimal.RequireFromString("13.50"), posRes.Amount)

	// Cache hit: zero amount must now be rejected (inner validation enforced
	// before consulting the cache), instead of returning {Amount: 0}.
	hitResHitErr, err := svc.Convert(context.Background(), decimal.Zero, "USD", "SGD")
	require.Error(t, err, "cache hit must reject zero amount")
	require.Equal(t, ConversionResult{}, hitResHitErr)
	require.Contains(t, err.Error(), "amount must be positive")

	// Cache miss: zero amount is rejected via the same top-level guard, so the
	// miss path no longer differs from the hit path.
	missRes, missErr := svc.Convert(context.Background(), decimal.Zero, "EUR", "SGD")
	require.Error(t, missErr, "cache miss must reject zero amount")
	require.Equal(t, ConversionResult{}, missRes)
	require.Contains(t, missErr.Error(), "amount must be positive")

	// Parity: identical inputs produce identical (non-usable) results and the
	// same error message regardless of cache warmth.
	require.Equal(t, missErr.Error(), err.Error())

	// The inner converter must not be consulted for the rejected amounts on the
	// hit path: only the warming call reached it.
	require.Equal(t, 1, int(inner.calls.Load()))
}

// TestCachedService_RejectsNegativeAmountOnHitAndMiss ensures negative
// amounts are rejected on both cache paths and never reach the inner converter.
func TestCachedService_RejectsNegativeAmountOnHitAndMiss(t *testing.T) {
	t.Parallel()

	inner := &validatingConverter{rate: decimal.RequireFromString("1.35")}
	svc := NewCachedService(inner, time.Hour, nil)

	// Warm USD->SGD.
	_, err := svc.Convert(context.Background(), decimal.RequireFromString("10"), "USD", "SGD")
	require.NoError(t, err)

	_, err = svc.Convert(context.Background(), decimal.RequireFromString("-5"), "USD", "SGD")
	require.Error(t, err, "cache hit must reject negative amount")
	require.Contains(t, err.Error(), "amount must be positive")

	_, err = svc.Convert(context.Background(), decimal.RequireFromString("-5"), "EUR", "SGD")
	require.Error(t, err, "cache miss must reject negative amount")
	require.Contains(t, err.Error(), "amount must be positive")

	require.Equal(t, 1, int(inner.calls.Load()))
}

// TestCachedService_PositiveAmountsStillWorkOnHitAndMiss guards against a
// regression where the new guard accidentally rejects valid positive amounts.
func TestCachedService_PositiveAmountsStillWorkOnHitAndMiss(t *testing.T) {
	t.Parallel()

	inner := &validatingConverter{rate: decimal.RequireFromString("1.35")}
	svc := NewCachedService(inner, time.Hour, nil)

	// Miss path: positive amount fetches and caches.
	missRes, err := svc.Convert(context.Background(), decimal.RequireFromString("10"), "USD", "SGD")
	require.NoError(t, err)
	require.Equal(t, decimal.RequireFromString("13.50"), missRes.Amount)

	// Hit path: positive amount serves from cache with the same rate.
	hitRes, err := svc.Convert(context.Background(), decimal.RequireFromString("20"), "USD", "SGD")
	require.NoError(t, err)
	require.Equal(t, decimal.RequireFromString("27.00"), hitRes.Amount)
	require.Equal(t, missRes.Rate, hitRes.Rate)
	require.Equal(t, missRes.RateDate, hitRes.RateDate)
}
