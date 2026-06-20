package bot

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/models"
	"hegel.dev/go/hegel"
)

// hegelCategoryOrNilGen draws a *models.Category that exercises the
// empty-name boundary: nil, a non-nil Category with an empty Name, or a
// non-nil Category with a non-empty Name. The empty-Name case is the one
// that distinguishes a correct "fall back to Uncategorized" implementation
// from a buggy one that only checks Category != nil.
func hegelCategoryOrNilGen() hegel.Generator[*models.Category] {
	return hegel.Composite(func(tc hegel.TestCase) *models.Category {
		kind := hegel.Draw(tc, hegel.Integers(0, 2))
		switch kind {
		case 0:
			return nil
		case 1:
			return &models.Category{Name: ""}
		default:
			return &models.Category{Name: hegel.Draw(tc, hegelCategoryNameGen())}
		}
	})
}

// TestHegelAggregateByCategoryConsistentWithHabitCategoryName asserts that
// the key aggregateByCategory uses for an expense matches the canonical
// category name produced by habitCategoryName. Both functions classify an
// expense's category, so they must agree on what "no category" means: a nil
// Category and a Category with an empty Name should both map to
// "Uncategorized". aggregateByCategory only checks Category != nil, so an
// empty Name leaks through as the "" key instead of "Uncategorized".
func TestHegelAggregateByCategoryConsistentWithHabitCategoryName(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		cat := hegel.Draw(ht, hegelCategoryOrNilGen())
		expense := models.Expense{
			Amount:   hegel.Draw(ht, hegelAmountGen()),
			Currency: hegel.Draw(ht, hegel.SampledFrom(expenseTestCurrencies)),
			Category: cat,
		}
		result := aggregateByCategory([]models.Expense{expense})
		wantKey := habitCategoryName(&expense)
		_, ok := result[wantKey]
		require.True(ht, ok,
			"aggregateByCategory key %q missing (Category=%+v): got %v",
			wantKey, cat, result)
	})
}

// TestHegelAggregateByCategoryEmptyNameIsUncategorized is the focused
// counterexample: a non-nil Category with an empty Name must aggregate under
// "Uncategorized", exactly like a nil Category does.
func TestHegelAggregateByCategoryEmptyNameIsUncategorized(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		amount := hegel.Draw(ht, hegelAmountGen())
		nilResult := aggregateByCategory([]models.Expense{
			{Amount: amount, Currency: "SGD", Category: nil},
		})
		emptyResult := aggregateByCategory([]models.Expense{
			{Amount: amount, Currency: "SGD", Category: &models.Category{Name: ""}},
		})
		require.Equal(ht, nilResult, emptyResult,
			"nil Category and empty-name Category should aggregate identically")
		require.Contains(ht, emptyResult, categoryUncategorized,
			"empty-name Category should fall back to %q", categoryUncategorized)
	})
}
