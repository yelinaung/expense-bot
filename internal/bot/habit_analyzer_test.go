package bot

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestAnalyzeExpenseHabit_MultiCurrencyAndCategoryRules(t *testing.T) {
	t.Parallel()

	loc := time.FixedZone("SGT", 8*60*60)
	food := &appmodels.Category{Name: "Food"}
	travel := &appmodels.Category{Name: "Travel"}
	selfCare := "Self-care"
	necessity := "Necessity"
	worth := true
	notWorth := false
	reviewedAt := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	reviewed := []appmodels.Expense{
		{
			Amount:      decimal.RequireFromString("10.00"),
			Currency:    "SGD",
			Category:    food,
			WorthIt:     &worth,
			SpendDriver: &selfCare,
			ReviewedAt:  &reviewedAt,
			CreatedAt:   time.Date(2026, 6, 9, 10, 0, 0, 0, loc),
		},
		{
			Amount:      decimal.RequireFromString("12.00"),
			Currency:    "USD",
			Category:    food,
			WorthIt:     &worth,
			SpendDriver: &selfCare,
			ReviewedAt:  &reviewedAt,
			CreatedAt:   time.Date(2026, 6, 10, 10, 0, 0, 0, loc),
		},
		{
			Amount:      decimal.RequireFromString("30.00"),
			Currency:    "SGD",
			Category:    travel,
			WorthIt:     &notWorth,
			SpendDriver: &necessity,
			ReviewedAt:  &reviewedAt,
			CreatedAt:   time.Date(2026, 6, 11, 10, 0, 0, 0, loc),
		},
		{
			Amount:      decimal.RequireFromString("40.00"),
			Currency:    "SGD",
			Category:    travel,
			WorthIt:     &notWorth,
			SpendDriver: &necessity,
			ReviewedAt:  &reviewedAt,
			CreatedAt:   time.Date(2026, 6, 11, 16, 0, 0, 0, loc),
		},
		{
			Amount:     decimal.RequireFromString("5.00"),
			Currency:   "SGD",
			WorthIt:    &notWorth,
			ReviewedAt: &reviewedAt,
			CreatedAt:  time.Date(2026, 6, 12, 10, 0, 0, 0, loc),
		},
	}
	all := append([]appmodels.Expense{}, reviewed...)
	all = append(all, appmodels.Expense{Amount: decimal.RequireFromString("9.00"), Currency: "SGD"})

	summary := AnalyzeExpenseHabit(all, reviewed, loc, "June 2026")

	require.Equal(t, "June 2026", summary.PeriodLabel)
	require.Equal(t, 6, summary.TotalCount)
	require.Equal(t, 5, summary.ReviewedCount)
	require.Equal(t, 2, summary.WorthItCount)
	require.Equal(t, 3, summary.NotWorthItCount)
	require.True(t, decimal.RequireFromString("10.00").Equal(summary.WorthItByCurrency["SGD"]))
	require.True(t, decimal.RequireFromString("12.00").Equal(summary.WorthItByCurrency["USD"]))
	require.True(t, decimal.RequireFromString("75.00").Equal(summary.NotWorthItByCurrency["SGD"]))
	require.Equal(t, "Food", summary.BestValueCategory)
	require.Equal(t, "Travel", summary.MostRegrettedCategory)
	require.Equal(t, SpendingDriver("Necessity"), summary.TopDriver)
	require.True(t, summary.HasBusiestNotWorthItWeekday)
	require.Equal(t, time.Thursday, summary.BusiestNotWorthItWeekday)
	require.Contains(t, summary.Insight, "Travel")
	require.Contains(t, summary.NotWorthItByCategoryAndCurrency, categoryUncategorized)
}

func TestAnalyzeExpenseHabit_NoReviewedExpenses(t *testing.T) {
	t.Parallel()

	summary := AnalyzeExpenseHabit([]appmodels.Expense{{Currency: "SGD"}}, nil, time.UTC, "This week")

	require.Equal(t, 1, summary.TotalCount)
	require.Zero(t, summary.ReviewedCount)
	require.Empty(t, summary.BestValueCategory)
	require.Empty(t, summary.MostRegrettedCategory)
	require.False(t, summary.HasBusiestNotWorthItWeekday)
	require.Contains(t, summary.Insight, "/review")
}

func TestReviewCallbackDataAndKeyboards(t *testing.T) {
	t.Parallel()

	driverKeyboard := buildDriverKeyboard(123, true)
	require.NotEmpty(t, driverKeyboard.InlineKeyboard)
	first := driverKeyboard.InlineKeyboard[0][0]
	require.Equal(t, "Necessity", first.Text)
	require.LessOrEqual(t, len(first.CallbackData), 64)

	expenseID, worthIt, driverIndex, ok := parseDriverCallback(first.CallbackData)
	require.True(t, ok)
	require.Equal(t, 123, expenseID)
	require.True(t, worthIt)
	require.Zero(t, driverIndex)

	reflectionKeyboard := buildExpenseReflectionKeyboard(456)
	require.Len(t, reflectionKeyboard.InlineKeyboard, 2)
	require.Len(t, reflectionKeyboard.InlineKeyboard[1], 3)
	require.Equal(t, "review_later_456", reflectionKeyboard.InlineKeyboard[1][2].CallbackData)
}

func TestFormatHabitSummary_PerCurrencyTotals(t *testing.T) {
	t.Parallel()

	summary := HabitSummary{
		PeriodLabel:           "Last 90 days",
		TotalCount:            3,
		ReviewedCount:         2,
		WorthItCount:          1,
		NotWorthItCount:       1,
		WorthItByCurrency:     map[string]decimal.Decimal{"SGD": decimal.RequireFromString("12.50")},
		NotWorthItByCurrency:  map[string]decimal.Decimal{"USD": decimal.RequireFromString("8.25")},
		TopDriver:             "Convenience",
		BestValueCategory:     "Food",
		MostRegrettedCategory: "Travel",
		Insight:               "Keep going.",
	}

	text := formatHabitSummary(&summary)

	require.Contains(t, text, "SGD: S$12.50")
	require.Contains(t, text, "USD: $8.25")
	require.Contains(t, text, "Best-value category: Food")
	require.Contains(t, text, "Most-regretted category: Travel")
}
