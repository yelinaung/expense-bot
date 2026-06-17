package bot

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestAnalyzeExpenseHabit(t *testing.T) {
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

	tests := []struct {
		name                         string
		totalCount                   int
		reviewed                     []appmodels.Expense
		loc                          *time.Location
		periodLabel                  string
		wantReviewedCount            int
		wantWorthItCount             int
		wantNotWorthItCount          int
		wantWorthItByCurrency        map[string]string
		wantNotWorthItByCurrency     map[string]string
		wantBestValueCategory        string
		wantMostRegrettedCategory    string
		wantTopDriver                spendingDriver
		wantBusiestNotWorthItWeekday time.Weekday
		wantHasBusiestWeekday        bool
	}{
		{
			name:                         "multi-currency and category rules",
			totalCount:                   6,
			reviewed:                     reviewed,
			loc:                          loc,
			periodLabel:                  "June 2026",
			wantReviewedCount:            5,
			wantWorthItCount:             2,
			wantNotWorthItCount:          3,
			wantWorthItByCurrency:        map[string]string{"SGD": "10.00", "USD": "12.00"},
			wantNotWorthItByCurrency:     map[string]string{"SGD": "75.00"},
			wantBestValueCategory:        "Food",
			wantMostRegrettedCategory:    "Travel",
			wantTopDriver:                spendingDriver("Necessity"),
			wantBusiestNotWorthItWeekday: time.Thursday,
			wantHasBusiestWeekday:        true,
		},
		{
			name:                     "no reviewed expenses",
			totalCount:               1,
			loc:                      time.UTC,
			periodLabel:              "This week",
			wantWorthItByCurrency:    map[string]string{},
			wantNotWorthItByCurrency: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			summary := analyzeExpenseHabit(tt.totalCount, tt.reviewed, tt.loc, tt.periodLabel)

			require.Equal(t, tt.periodLabel, summary.PeriodLabel)
			require.Equal(t, tt.totalCount, summary.TotalCount)
			require.Equal(t, tt.wantReviewedCount, summary.ReviewedCount)
			require.Equal(t, tt.wantWorthItCount, summary.WorthItCount)
			require.Equal(t, tt.wantNotWorthItCount, summary.NotWorthItCount)
			requireCurrencyTotals(t, tt.wantWorthItByCurrency, summary.WorthItByCurrency)
			requireCurrencyTotals(t, tt.wantNotWorthItByCurrency, summary.NotWorthItByCurrency)
			require.Equal(t, tt.wantBestValueCategory, summary.BestValueCategory)
			require.Equal(t, tt.wantMostRegrettedCategory, summary.MostRegrettedCategory)
			require.Equal(t, tt.wantTopDriver, summary.TopDriver)
			require.Equal(t, tt.wantHasBusiestWeekday, summary.HasBusiestNotWorthItWeekday)
			if tt.wantHasBusiestWeekday {
				require.Equal(t, tt.wantBusiestNotWorthItWeekday, summary.BusiestNotWorthItWeekday)
			}
		})
	}
}

func requireCurrencyTotals(t *testing.T, want map[string]string, got map[string]decimal.Decimal) {
	t.Helper()

	require.Len(t, got, len(want))
	for currency, amount := range want {
		require.True(t, decimal.RequireFromString(amount).Equal(got[currency]))
	}
}

func TestReviewCallbackDataAndKeyboards(t *testing.T) {
	t.Parallel()

	driverKeyboard := buildDriverKeyboard(123, true)
	require.NotEmpty(t, driverKeyboard.InlineKeyboard)
	first := driverKeyboard.InlineKeyboard[0][0]
	require.Equal(t, "Necessity", first.Text)
	require.LessOrEqual(t, len(first.CallbackData), 64)

	callback := parseDriverCallback(first.CallbackData)
	require.True(t, callback.ok)
	require.Equal(t, 123, callback.expenseID)
	require.True(t, callback.worthIt)
	require.Zero(t, callback.driverIndex)

	reflectionKeyboard := buildExpenseReflectionKeyboard(456)
	require.Len(t, reflectionKeyboard.InlineKeyboard, 2)
	require.Len(t, reflectionKeyboard.InlineKeyboard[1], 3)
	require.Equal(t, "review_later_456", reflectionKeyboard.InlineKeyboard[1][2].CallbackData)
}

func TestFormatHabitSummary_PerCurrencyTotals(t *testing.T) {
	t.Parallel()

	summary := habitSummary{
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
	}

	text := formatHabitSummary(&summary)

	require.Contains(t, text, "SGD: S$12.50")
	require.Contains(t, text, "USD: $8.25")
	require.Contains(t, text, "Best-value category: Food")
	require.Contains(t, text, "Most-regretted category: Travel")
	require.Contains(t, text, "not-worth-it spending showed up most often in Travel")
}
