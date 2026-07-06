package bot

import (
	"slices"
	"strconv"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"hegel.dev/go/hegel"

	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

// hegelReviewedExpenseGen draws an expense in the shape
// GetReviewedByUserIDAndDateRange feeds to analyzeExpenseHabit. WorthIt is
// drawn as nil/true/false because the analyzer must skip nil entries, and
// SpendDriver covers nil, empty, and known drivers because the analyzer
// only counts non-empty ones.
func hegelReviewedExpenseGen() hegel.Generator[appmodels.Expense] {
	return hegel.Composite(func(tc hegel.TestCase) appmodels.Expense {
		expense := appmodels.Expense{
			Amount:    hegel.Draw(tc, hegelAmountGen()),
			Currency:  hegel.Draw(tc, hegel.SampledFrom(expenseTestCurrencies)),
			Category:  hegel.Draw(tc, hegelCategoryOrNilGen()),
			WorthIt:   hegel.Draw(tc, hegel.Optional(hegel.Booleans())),
			CreatedAt: hegel.Draw(tc, hegelTimeInLocationGen()),
		}
		switch hegel.Draw(tc, hegel.Integers(0, 2)) {
		case 0:
			// No driver recorded.
		case 1:
			empty := ""
			expense.SpendDriver = &empty
		default:
			idx := hegel.Draw(tc, hegel.Integers(0, len(spendingDrivers)-1))
			driver := string(spendingDrivers[idx])
			expense.SpendDriver = &driver
		}
		return expense
	})
}

// hegelReviewedExpensesGen draws a slice of reviewed expenses. The size is
// drawn separately so large slices are actually produced.
func hegelReviewedExpensesGen() hegel.Generator[[]appmodels.Expense] {
	return hegel.Composite(func(tc hegel.TestCase) []appmodels.Expense {
		n := hegel.Draw(tc, hegel.Integers(0, 60))
		return hegel.Draw(tc, hegel.Lists(hegelReviewedExpenseGen()).MinSize(n))
	})
}

// requireSameCurrencyTotals asserts two currency→amount maps hold the same
// keys with decimal-equal values, ignoring internal decimal representation.
func requireSameCurrencyTotals(t require.TestingT, want, got map[string]decimal.Decimal) {
	require.Len(t, got, len(want))
	for currency, amount := range want {
		require.True(t, amount.Equal(got[currency]),
			"currency %s: want %s, got %s", currency, amount, got[currency])
	}
}

// TestHegelAnalyzeExpenseHabitCountConservation asserts the summary's counts
// are conserved: every expense with a non-nil WorthIt is counted exactly
// once as either worth-it or not-worth-it, and TotalCount/PeriodLabel pass
// through unchanged.
func TestHegelAnalyzeExpenseHabitCountConservation(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		expenses := hegel.Draw(ht, hegelReviewedExpensesGen())
		extra := hegel.Draw(ht, hegel.Integers(0, 50))
		totalCount := len(expenses) + extra
		loc := hegel.Draw(ht, hegelLocationGen())

		summary := analyzeExpenseHabit(totalCount, expenses, loc, "label")

		wantReviewed, wantWorth := 0, 0
		for i := range expenses {
			if expenses[i].WorthIt == nil {
				continue
			}
			wantReviewed++
			if *expenses[i].WorthIt {
				wantWorth++
			}
		}
		require.Equal(ht, "label", summary.PeriodLabel)
		require.Equal(ht, totalCount, summary.TotalCount)
		require.Equal(ht, wantReviewed, summary.ReviewedCount)
		require.Equal(ht, wantWorth, summary.WorthItCount)
		require.Equal(ht, wantReviewed-wantWorth, summary.NotWorthItCount)
	})
}

// TestHegelAnalyzeExpenseHabitCurrencyTotalsOracle asserts the per-currency
// worth-it and not-worth-it totals against an independently computed oracle,
// and that they reconcile with sumExpenseAmountsByCurrency over the reviewed
// subset — the helper the weekly report uses for its own totals.
func TestHegelAnalyzeExpenseHabitCurrencyTotalsOracle(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		expenses := hegel.Draw(ht, hegelReviewedExpensesGen())
		loc := hegel.Draw(ht, hegelLocationGen())

		summary := analyzeExpenseHabit(len(expenses), expenses, loc, "label")

		wantWorth := map[string]decimal.Decimal{}
		wantNotWorth := map[string]decimal.Decimal{}
		var reviewed []appmodels.Expense
		for i := range expenses {
			e := expenses[i]
			if e.WorthIt == nil {
				continue
			}
			reviewed = append(reviewed, e)
			if *e.WorthIt {
				wantWorth[e.Currency] = wantWorth[e.Currency].Add(e.Amount)
			} else {
				wantNotWorth[e.Currency] = wantNotWorth[e.Currency].Add(e.Amount)
			}
		}
		requireSameCurrencyTotals(ht, wantWorth, summary.WorthItByCurrency)
		requireSameCurrencyTotals(ht, wantNotWorth, summary.NotWorthItByCurrency)

		// Worth-it plus not-worth-it must reconcile with the weekly
		// report's own aggregation of the reviewed subset.
		combined := map[string]decimal.Decimal{}
		for currency, amount := range summary.WorthItByCurrency {
			combined[currency] = combined[currency].Add(amount)
		}
		for currency, amount := range summary.NotWorthItByCurrency {
			combined[currency] = combined[currency].Add(amount)
		}
		requireSameCurrencyTotals(ht, sumExpenseAmountsByCurrency(reviewed), combined)
	})
}

// TestHegelAnalyzeExpenseHabitOrderInvariance asserts the summary does not
// depend on input slice order. The analyzer aggregates through Go maps and
// breaks ties explicitly (alphabetical, weekday order), so a reversed input
// must produce an identical summary; a violation would mean nondeterministic
// map-iteration order leaks into the result.
func TestHegelAnalyzeExpenseHabitOrderInvariance(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		expenses := hegel.Draw(ht, hegelReviewedExpensesGen())
		loc := hegel.Draw(ht, hegelLocationGen())

		reversed := slices.Clone(expenses)
		slices.Reverse(reversed)

		got := analyzeExpenseHabit(len(expenses), expenses, loc, "label")
		gotReversed := analyzeExpenseHabit(len(expenses), reversed, loc, "label")

		require.Equal(ht, got.ReviewedCount, gotReversed.ReviewedCount)
		require.Equal(ht, got.WorthItCount, gotReversed.WorthItCount)
		require.Equal(ht, got.NotWorthItCount, gotReversed.NotWorthItCount)
		require.Equal(ht, got.TopDriver, gotReversed.TopDriver)
		require.Equal(ht, got.BestValueCategory, gotReversed.BestValueCategory)
		require.Equal(ht, got.MostRegrettedCategory, gotReversed.MostRegrettedCategory)
		require.Equal(ht, got.HasBusiestNotWorthItWeekday, gotReversed.HasBusiestNotWorthItWeekday)
		require.Equal(ht, got.BusiestNotWorthItWeekday, gotReversed.BusiestNotWorthItWeekday)
		requireSameCurrencyTotals(ht, got.WorthItByCurrency, gotReversed.WorthItByCurrency)
		requireSameCurrencyTotals(ht, got.NotWorthItByCurrency, gotReversed.NotWorthItByCurrency)
	})
}

// TestHegelAnalyzeExpenseHabitBusiestWeekdayIffNotWorthIt asserts the
// weekday-pattern flag is set exactly when at least one not-worth-it expense
// exists, since only not-worth-it expenses feed the weekday counts.
func TestHegelAnalyzeExpenseHabitBusiestWeekdayIffNotWorthIt(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		expenses := hegel.Draw(ht, hegelReviewedExpensesGen())
		loc := hegel.Draw(ht, hegelLocationGen())

		summary := analyzeExpenseHabit(len(expenses), expenses, loc, "label")

		require.Equal(ht, summary.NotWorthItCount > 0, summary.HasBusiestNotWorthItWeekday)
	})
}

// TestHegelAnalyzeExpenseHabitMinSampleGuard asserts the documented
// minimum-sample rule: a category may only be named best-value or
// most-regretted when it has at least minCategoryReflectionSample reviewed
// expenses.
func TestHegelAnalyzeExpenseHabitMinSampleGuard(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		expenses := hegel.Draw(ht, hegelReviewedExpensesGen())
		loc := hegel.Draw(ht, hegelLocationGen())

		summary := analyzeExpenseHabit(len(expenses), expenses, loc, "label")

		reviewedPerCategory := map[string]int{}
		for i := range expenses {
			if expenses[i].WorthIt == nil {
				continue
			}
			reviewedPerCategory[habitCategoryName(&expenses[i])]++
		}
		for _, category := range []string{summary.BestValueCategory, summary.MostRegrettedCategory} {
			if category == "" {
				continue
			}
			require.GreaterOrEqual(ht, reviewedPerCategory[category], minCategoryReflectionSample,
				"category %q named with fewer than %d reviewed expenses",
				category, minCategoryReflectionSample)
		}
	})
}

// TestHegelFormatHabitSummaryRobustness asserts the recap renderer never
// panics on analyzer output for arbitrary expenses and Unicode period
// labels, always reports the counts, and HTML-escapes the label. Both
// /habit and the weekly habit recap render through this function.
func TestHegelFormatHabitSummaryRobustness(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		expenses := hegel.Draw(ht, hegelReviewedExpensesGen())
		loc := hegel.Draw(ht, hegelLocationGen())
		label := hegel.Draw(ht, hegel.Text().MaxSize(50))

		summary := analyzeExpenseHabit(len(expenses), expenses, loc, label)
		out := formatHabitSummary(&summary)

		require.Contains(ht, out, escapeHTML(label))
		require.Contains(ht, out,
			"Reviewed: "+strconv.Itoa(summary.ReviewedCount)+"/"+strconv.Itoa(summary.TotalCount))
	})
}
