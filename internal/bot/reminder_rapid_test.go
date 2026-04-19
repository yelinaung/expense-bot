package bot

import (
	"math/rand"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
	"pgregory.net/rapid"
)

// genExpense draws an Expense with a bounded positive amount.
func genExpense() *rapid.Generator[appmodels.Expense] {
	return rapid.Custom(func(t *rapid.T) appmodels.Expense {
		v := rapid.IntRange(0, 1_000_000).Draw(t, "v")
		exp := rapid.IntRange(-4, 2).Draw(t, "exp")
		return appmodels.Expense{Amount: decimal.New(int64(v), int32(exp))}
	})
}

// TestSumExpenseAmountsEmptyIsZero: sum of empty or nil slice is zero.
func TestSumExpenseAmountsEmptyIsZero(t *testing.T) {
	t.Parallel()
	require.True(t, sumExpenseAmounts(nil).IsZero())
	require.True(t, sumExpenseAmounts([]appmodels.Expense{}).IsZero())
}

// TestSumExpenseAmountsSingletonIdentity: sum of [x] equals x.
func TestSumExpenseAmountsSingletonIdentity(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		e := genExpense().Draw(t, "e")
		require.True(t, sumExpenseAmounts([]appmodels.Expense{e}).Equal(e.Amount))
	})
}

// TestSumExpenseAmountsOrderInvariant: shuffling the slice doesn't change the sum.
func TestSumExpenseAmountsOrderInvariant(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 12).Draw(t, "n")
		xs := make([]appmodels.Expense, n)
		for i := range n {
			xs[i] = genExpense().Draw(t, "x")
		}
		seed := rapid.Int64().Draw(t, "seed")
		ys := make([]appmodels.Expense, n)
		copy(ys, xs)
		// Deterministic shuffle driven by rapid-chosen seed for reproducibility.
		r := rand.New(rand.NewSource(seed))
		r.Shuffle(n, func(i, j int) { ys[i], ys[j] = ys[j], ys[i] })

		require.True(t, sumExpenseAmounts(xs).Equal(sumExpenseAmounts(ys)))
	})
}

// TestSumExpenseAmountsAssociativeSplit: sum(xs) == sum(xs[:k]) + sum(xs[k:]).
func TestSumExpenseAmountsAssociativeSplit(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 12).Draw(t, "n")
		xs := make([]appmodels.Expense, n)
		for i := range n {
			xs[i] = genExpense().Draw(t, "x")
		}
		k := rapid.IntRange(0, n).Draw(t, "k")
		whole := sumExpenseAmounts(xs)
		split := sumExpenseAmounts(xs[:k]).Add(sumExpenseAmounts(xs[k:]))
		require.True(t, whole.Equal(split))
	})
}
