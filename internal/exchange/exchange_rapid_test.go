package exchange

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"hegel.dev/go/hegel"
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

// hegelPositiveDecimalGen is the Hegel analog of genPositiveDecimal: a bounded
// strictly-positive decimal. The v/exp bounds prevent decimal.New overflow in
// test arithmetic (decimal.New takes int64 mantissa + int32 exp); the decimal
// domain itself is the contract's full valid positive range.
func hegelPositiveDecimalGen() hegel.Generator[decimal.Decimal] {
	return hegel.Composite(func(tc hegel.TestCase) decimal.Decimal {
		v := hegel.Draw(tc, hegel.Integers(1, 1_000_000))
		exp := hegel.Draw(tc, hegel.Integers(-4, 2))
		return decimal.New(int64(v), int32(exp))
	})
}

// TestHegelNormalizePairContainsArrow is the Hegel equivalent: output always
// contains the "->" separator, over full-Unicode input.
func TestHegelNormalizePairContainsArrow(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		from := hegel.Draw(ht, hegel.Text())
		to := hegel.Draw(ht, hegel.Text())
		got := normalizePair(from, to)
		require.Contains(ht, got, "->")
	})
}

// TestHegelNormalizePairUppercase is the Hegel equivalent: both sides of "->"
// are uppercased and trimmed, over full-Unicode input.
func TestHegelNormalizePairUppercase(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		from := hegel.Draw(ht, hegel.Text())
		to := hegel.Draw(ht, hegel.Text())
		got := normalizePair(from, to)
		parts := strings.SplitN(got, "->", 2)
		require.Len(ht, parts, 2)
		require.Equal(ht, strings.ToUpper(parts[0]), parts[0])
		require.Equal(ht, strings.ToUpper(parts[1]), parts[1])
		require.Equal(ht, strings.TrimSpace(parts[0]), parts[0])
		require.Equal(ht, strings.TrimSpace(parts[1]), parts[1])
	})
}

// TestHegelNormalizePairIdempotentOnCleanInput is the Hegel equivalent.
// Note this is NOT f(f(x)) == f(x): normalizePair joins its two arguments with
// "->", so feeding the whole result back in would re-append "->". The property
// holds when the output is split back into its two parts and renormalized —
// that is the operation the cache performs (it keys by normalizePair(from,to)
// and never normalizes the combined key as a single input).
func TestHegelNormalizePairIdempotentOnCleanInput(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		from := hegel.Draw(ht, hegel.FromRegex(`[A-Z]{3}`, true))
		to := hegel.Draw(ht, hegel.FromRegex(`[A-Z]{3}`, true))
		once := normalizePair(from, to)
		parts := strings.SplitN(once, "->", 2)
		twice := normalizePair(parts[0], parts[1])
		require.Equal(ht, once, twice)
	})
}

// TestHegelValidateConversionRatePositiveAccepts is the Hegel equivalent: any
// strictly-positive rate is accepted.
func TestHegelValidateConversionRatePositiveAccepts(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		rate := hegel.Draw(ht, hegelPositiveDecimalGen())
		require.NoError(ht, validateConversionRate(rate))
	})
}

// TestHegelValidateConversionRateZeroOrNegativeRejects is the Hegel equivalent:
// zero and negative rates are rejected.
func TestHegelValidateConversionRateZeroOrNegativeRejects(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		v := hegel.Draw(ht, hegel.Integers(-1_000_000, 0))
		exp := hegel.Draw(ht, hegel.Integers(-4, 2))
		rate := decimal.New(int64(v), int32(exp))
		ht.Assume(!rate.IsPositive())
		require.Error(ht, validateConversionRate(rate))
	})
}

// TestHegelApplyCachedRatePreservesRateAndDate is the Hegel equivalent: rate +
// rateDate flow through unchanged. The rateDate is drawn across the full int64
// Unix range since applyCachedRate's contract places no restriction on the date.
func TestHegelApplyCachedRatePreservesRateAndDate(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		amount := hegel.Draw(ht, hegelPositiveDecimalGen())
		rate := hegel.Draw(ht, hegelPositiveDecimalGen())
		unix := hegel.Draw(ht, hegel.Integers[int64](math.MinInt64, math.MaxInt64))
		entry := cachedRateEntry{
			Rate:     rate,
			RateDate: time.Unix(unix, 0).UTC(),
		}
		got := applyCachedRate(amount, entry)
		require.True(ht, got.Rate.Equal(rate))
		require.Equal(ht, entry.RateDate, got.RateDate)
	})
}

// TestHegelApplyCachedRateAmountRoundedTo2 is the Hegel equivalent: output
// amount equals amount*rate rounded to 2 places.
func TestHegelApplyCachedRateAmountRoundedTo2(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		amount := hegel.Draw(ht, hegelPositiveDecimalGen())
		rate := hegel.Draw(ht, hegelPositiveDecimalGen())
		entry := cachedRateEntry{Rate: rate, RateDate: time.Unix(0, 0)}
		got := applyCachedRate(amount, entry)
		want := amount.Mul(rate).Round(2)
		require.True(ht, got.Amount.Equal(want),
			"amount=%s rate=%s got=%s want=%s", amount, rate, got.Amount, want)
	})
}
