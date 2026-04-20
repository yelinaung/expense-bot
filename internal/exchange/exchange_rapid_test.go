package exchange

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// genPositiveDecimal produces a bounded strictly-positive decimal.
func genPositiveDecimal() *rapid.Generator[decimal.Decimal] {
	return rapid.Custom(func(t *rapid.T) decimal.Decimal {
		v := rapid.IntRange(1, 1_000_000).Draw(t, "v")
		exp := rapid.IntRange(-4, 2).Draw(t, "exp")
		return decimal.New(int64(v), int32(exp))
	})
}

// TestNormalizePairContainsArrow: output always contains the "->" separator.
func TestNormalizePairContainsArrow(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		from := rapid.String().Draw(t, "from")
		to := rapid.String().Draw(t, "to")
		got := normalizePair(from, to)
		require.Contains(t, got, "->")
	})
}

// TestNormalizePairUppercase: both sides of "->" are uppercased and trimmed.
func TestNormalizePairUppercase(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		from := rapid.String().Draw(t, "from")
		to := rapid.String().Draw(t, "to")
		got := normalizePair(from, to)
		parts := strings.SplitN(got, "->", 2)
		require.Len(t, parts, 2)
		require.Equal(t, strings.ToUpper(parts[0]), parts[0])
		require.Equal(t, strings.ToUpper(parts[1]), parts[1])
		require.Equal(t, strings.TrimSpace(parts[0]), parts[0])
		require.Equal(t, strings.TrimSpace(parts[1]), parts[1])
	})
}

// TestNormalizePairIdempotentOnCleanInput: already-normalized currency codes
// pass through unchanged when split back and renormalized.
func TestNormalizePairIdempotentOnCleanInput(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		from := rapid.StringMatching(`[A-Z]{3}`).Draw(t, "from")
		to := rapid.StringMatching(`[A-Z]{3}`).Draw(t, "to")
		once := normalizePair(from, to)
		parts := strings.SplitN(once, "->", 2)
		twice := normalizePair(parts[0], parts[1])
		require.Equal(t, once, twice)
	})
}

// TestValidateConversionRatePositiveAccepts: any strictly-positive rate is accepted.
func TestValidateConversionRatePositiveAccepts(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		rate := genPositiveDecimal().Draw(t, "rate")
		require.NoError(t, validateConversionRate(rate))
	})
}

// TestValidateConversionRateZeroOrNegativeRejects: zero and negative rates are rejected.
func TestValidateConversionRateZeroOrNegativeRejects(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		v := rapid.IntRange(-1_000_000, 0).Draw(t, "v")
		exp := rapid.IntRange(-4, 2).Draw(t, "exp")
		rate := decimal.New(int64(v), int32(exp))
		if rate.IsPositive() {
			t.Skip("positive")
		}
		require.Error(t, validateConversionRate(rate))
	})
}

// TestApplyCachedRatePreservesRateAndDate: rate + rateDate flow through unchanged.
func TestApplyCachedRatePreservesRateAndDate(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		amount := genPositiveDecimal().Draw(t, "amount")
		rate := genPositiveDecimal().Draw(t, "rate")
		unix := rapid.Int64Range(0, 4_000_000_000).Draw(t, "unix")
		entry := cachedRateEntry{
			Rate:     rate,
			RateDate: time.Unix(unix, 0).UTC(),
		}
		got := applyCachedRate(amount, entry)
		require.True(t, got.Rate.Equal(rate))
		require.Equal(t, entry.RateDate, got.RateDate)
	})
}

// TestApplyCachedRateAmountRoundedTo2: output amount equals amount*rate rounded to 2 places.
func TestApplyCachedRateAmountRoundedTo2(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		amount := genPositiveDecimal().Draw(t, "amount")
		rate := genPositiveDecimal().Draw(t, "rate")
		entry := cachedRateEntry{Rate: rate, RateDate: time.Unix(0, 0)}
		got := applyCachedRate(amount, entry)
		want := amount.Mul(rate).Round(2)
		require.True(t, got.Amount.Equal(want),
			"amount=%s rate=%s got=%s want=%s", amount, rate, got.Amount, want)
	})
}
