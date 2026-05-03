package bot

import (
	"maps"
	"math/rand"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
	"pgregory.net/rapid"
)

// genExpenseWithCurrency draws an Expense with a bounded positive amount
// and a random currency code.
func genExpenseWithCurrency() *rapid.Generator[appmodels.Expense] {
	return rapid.Custom(func(t *rapid.T) appmodels.Expense {
		v := rapid.IntRange(0, 1_000_000).Draw(t, "v")
		exp := rapid.IntRange(-4, 2).Draw(t, "exp")
		cur := rapid.SampledFrom([]string{"SGD", "USD", "EUR"}).Draw(t, "cur")
		return appmodels.Expense{
			Amount:   decimal.New(int64(v), int32(exp)),
			Currency: cur,
		}
	})
}

// TestSumExpenseAmountsByCurrencyEmptyIsEmpty: empty or nil slice
// returns an empty map.
func TestSumExpenseAmountsByCurrencyEmptyIsEmpty(t *testing.T) {
	t.Parallel()
	require.Empty(t, sumExpenseAmountsByCurrency(nil))
	require.Empty(t, sumExpenseAmountsByCurrency([]appmodels.Expense{}))
}

// TestSumExpenseAmountsByCurrencySingletonIdentity: sum of [x] equals
// a map with x.Amount for x.Currency.
func TestSumExpenseAmountsByCurrencySingletonIdentity(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		e := genExpenseWithCurrency().Draw(t, "e")
		result := sumExpenseAmountsByCurrency([]appmodels.Expense{e})
		require.Len(t, result, 1)
		require.True(t, result[e.Currency].Equal(e.Amount))
	})
}

// TestSumExpenseAmountsByCurrencyOrderInvariant: shuffling the slice
// doesn't change the per-currency totals.
func TestSumExpenseAmountsByCurrencyOrderInvariant(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 12).Draw(t, "n")
		xs := make([]appmodels.Expense, n)
		for i := range n {
			xs[i] = genExpenseWithCurrency().Draw(t, "x")
		}
		seed := rapid.Int64().Draw(t, "seed")
		ys := make([]appmodels.Expense, n)
		copy(ys, xs)
		r := rand.New(rand.NewSource(seed))
		r.Shuffle(n, func(i, j int) { ys[i], ys[j] = ys[j], ys[i] })

		require.Equal(t, sumExpenseAmountsByCurrency(xs), sumExpenseAmountsByCurrency(ys))
	})
}

// TestSumExpenseAmountsByCurrencyAssociativeSplit: merging per-currency
// totals of split slices equals the total of the whole slice.
func TestSumExpenseAmountsByCurrencyAssociativeSplit(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 12).Draw(t, "n")
		xs := make([]appmodels.Expense, n)
		for i := range n {
			xs[i] = genExpenseWithCurrency().Draw(t, "x")
		}
		k := rapid.IntRange(0, n).Draw(t, "k")
		whole := sumExpenseAmountsByCurrency(xs)
		left := sumExpenseAmountsByCurrency(xs[:k])
		right := sumExpenseAmountsByCurrency(xs[k:])
		merged := mergeCurrencyTotals(left, right)
		require.Equal(t, whole, merged)
	})
}

// mergeCurrencyTotals adds the values from right into left and returns
// the combined map.
func mergeCurrencyTotals(
	left, right map[string]decimal.Decimal,
) map[string]decimal.Decimal {
	result := make(map[string]decimal.Decimal, len(left)+len(right))
	maps.Copy(result, left)
	for k, v := range right {
		result[k] = result[k].Add(v)
	}
	return result
}
