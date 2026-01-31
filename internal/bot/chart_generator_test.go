package bot

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestGenerateExpenseChart(t *testing.T) {
	tests := []struct {
		name        string
		expenses    []models.Expense
		period      string
		expectError bool
	}{
		{
			name: "generates chart with multiple categories",
			expenses: []models.Expense{
				{
					ID:          1,
					Amount:      decimal.NewFromFloat(50.00),
					Description: "Groceries",
					Category:    &models.Category{ID: 1, Name: "Food - Groceries"},
				},
				{
					ID:          2,
					Amount:      decimal.NewFromFloat(30.00),
					Description: "Lunch",
					Category:    &models.Category{ID: 2, Name: "Food - Dining Out"},
				},
				{
					ID:          3,
					Amount:      decimal.NewFromFloat(20.00),
					Description: "Coffee",
					Category:    &models.Category{ID: 2, Name: "Food - Dining Out"},
				},
			},
			period:      "Week",
			expectError: false,
		},
		{
			name: "handles single category",
			expenses: []models.Expense{
				{
					ID:          1,
					Amount:      decimal.NewFromFloat(100.00),
					Description: "Groceries",
					Category:    &models.Category{ID: 1, Name: "Food - Groceries"},
				},
			},
			period:      "Month",
			expectError: false,
		},
		{
			name: "handles uncategorized expenses",
			expenses: []models.Expense{
				{
					ID:          1,
					Amount:      decimal.NewFromFloat(50.00),
					Description: "Random expense",
					Category:    nil,
				},
				{
					ID:          2,
					Amount:      decimal.NewFromFloat(30.00),
					Description: "Another expense",
					Category:    nil,
				},
			},
			period:      "Week",
			expectError: false,
		},
		{
			name:        "handles empty expense list",
			expenses:    []models.Expense{},
			period:      "Week",
			expectError: true,
		},
		{
			name: "formats decimal amounts correctly",
			expenses: []models.Expense{
				{
					ID:          1,
					Amount:      decimal.NewFromFloat(12.50),
					Description: "Coffee",
					Category:    &models.Category{ID: 1, Name: "Food - Dining Out"},
				},
				{
					ID:          2,
					Amount:      decimal.NewFromFloat(7.75),
					Description: "Snack",
					Category:    &models.Category{ID: 1, Name: "Food - Dining Out"},
				},
			},
			period:      "Week",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, err := GenerateExpenseChart(tt.expenses, tt.period)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(buf) == 0 {
				t.Errorf("expected non-empty PNG data")
			}

			// PNG files start with magic bytes: 89 50 4E 47
			if len(buf) >= 4 && (buf[0] != 0x89 || buf[1] != 0x50 || buf[2] != 0x4E || buf[3] != 0x47) {
				t.Errorf("output does not appear to be a PNG file")
			}
		})
	}
}

func TestAggregateByCategory(t *testing.T) {
	tests := []struct {
		name     string
		expenses []models.Expense
		expected map[string]string // category -> amount (as string for comparison)
	}{
		{
			name: "aggregates by category name",
			expenses: []models.Expense{
				{
					Amount:   decimal.NewFromFloat(50.00),
					Category: &models.Category{Name: "Food - Groceries"},
				},
				{
					Amount:   decimal.NewFromFloat(30.00),
					Category: &models.Category{Name: "Food - Dining Out"},
				},
				{
					Amount:   decimal.NewFromFloat(20.00),
					Category: &models.Category{Name: "Food - Groceries"},
				},
			},
			expected: map[string]string{
				"Food - Groceries":  "70",
				"Food - Dining Out": "30",
			},
		},
		{
			name: "handles nil categories as Uncategorized",
			expenses: []models.Expense{
				{
					Amount:   decimal.NewFromFloat(25.00),
					Category: nil,
				},
				{
					Amount:   decimal.NewFromFloat(15.00),
					Category: nil,
				},
			},
			expected: map[string]string{
				"Uncategorized": "40",
			},
		},
		{
			name: "sums amounts correctly with decimals",
			expenses: []models.Expense{
				{
					Amount:   decimal.NewFromFloat(12.50),
					Category: &models.Category{Name: "Food - Dining Out"},
				},
				{
					Amount:   decimal.NewFromFloat(7.75),
					Category: &models.Category{Name: "Food - Dining Out"},
				},
			},
			expected: map[string]string{
				"Food - Dining Out": "20.25",
			},
		},
		{
			name:     "handles empty expense list",
			expenses: []models.Expense{},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aggregateByCategory(tt.expenses)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d categories, got %d", len(tt.expected), len(result))
			}

			for category, expectedAmount := range tt.expected {
				actualAmount, ok := result[category]
				if !ok {
					t.Errorf("expected category %s not found in result", category)
					continue
				}

				if actualAmount.String() != expectedAmount {
					t.Errorf("category %s: expected amount %s, got %s",
						category, expectedAmount, actualAmount.String())
				}
			}
		})
	}
}

func TestGenerateChartFilename(t *testing.T) {
	tests := []struct {
		name     string
		period   string
		contains string // substring that should be in the filename
	}{
		{
			name:     "generates week filename with date",
			period:   periodWeek,
			contains: "chart_week_",
		},
		{
			name:     "generates month filename with year-month",
			period:   periodMonth,
			contains: "chart_month_",
		},
		{
			name:     "handles unknown period",
			period:   "unknown",
			contains: "chart_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := generateChartFilename(tt.period)

			if filename == "" {
				t.Errorf("expected non-empty filename")
			}

			if len(filename) < 10 {
				t.Errorf("filename seems too short: %s", filename)
			}

			if filename[len(filename)-4:] != ".png" {
				t.Errorf("expected .png extension, got: %s", filename)
			}

			// Verify it contains the expected prefix
			if len(filename) < len(tt.contains) || filename[:len(tt.contains)] != tt.contains {
				t.Errorf("expected filename to start with %s, got: %s", tt.contains, filename)
			}
		})
	}
}

func TestGenerateChartFilenameFormat(t *testing.T) {
	// Test that filenames follow the expected format

	t.Run("week format", func(t *testing.T) {
		filename := generateChartFilename(periodWeek)
		// Should be like: chart_week_2026-01-27.png
		start, _ := getWeekDateRange()
		expected := "chart_week_" + start.Format("2006-01-02") + ".png"
		if filename != expected {
			t.Errorf("expected %s, got %s", expected, filename)
		}
	})

	t.Run("month format", func(t *testing.T) {
		filename := generateChartFilename(periodMonth)
		// Should be like: chart_month_2026-01.png
		now := time.Now()
		expected := "chart_month_" + now.Format("2006-01") + ".png"
		if filename != expected {
			t.Errorf("expected %s, got %s", expected, filename)
		}
	})
}
