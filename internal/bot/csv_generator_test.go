package bot

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestGenerateExpensesCSV(t *testing.T) {
	t.Parallel()

	t.Run("generates CSV with header and rows", func(t *testing.T) {
		expenses := []models.Expense{
			{
				ID:                1,
				UserExpenseNumber: 1,
				Amount:            decimal.NewFromFloat(10.50),
				Currency:          "SGD",
				Description:       "Coffee",
				CreatedAt:         time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
				Category:          &models.Category{Name: "Food"},
			},
			{
				ID:                2,
				UserExpenseNumber: 2,
				Amount:            decimal.NewFromFloat(25.00),
				Currency:          "SGD",
				Description:       "Taxi",
				CreatedAt:         time.Date(2026, 1, 16, 14, 15, 0, 0, time.UTC),
				Category:          &models.Category{Name: "Transportation"},
			},
		}

		csvData, err := GenerateExpensesCSV(expenses)
		require.NoError(t, err)
		require.NotEmpty(t, csvData)

		// Parse CSV
		reader := csv.NewReader(strings.NewReader(string(csvData)))
		records, err := reader.ReadAll()
		require.NoError(t, err)
		require.Len(t, records, 3) // Header + 2 rows

		// Verify header
		header := records[0]
		require.Equal(t, []string{"ID", "Date", "Amount", "Currency", "Description", "Merchant", "Category"}, header)

		// Verify first row
		row1 := records[1]
		require.Equal(t, "1", row1[0])
		require.Equal(t, "2026-01-15 10:30:00", row1[1])
		require.Equal(t, "10.50", row1[2])
		require.Equal(t, "SGD", row1[3])
		require.Equal(t, "Coffee", row1[4])
		require.Equal(t, "", row1[5]) // Merchant
		require.Equal(t, "Food", row1[6])

		// Verify second row
		row2 := records[2]
		require.Equal(t, "2", row2[0])
		require.Equal(t, "2026-01-16 14:15:00", row2[1])
		require.Equal(t, "25.00", row2[2])
		require.Equal(t, "SGD", row2[3])
		require.Equal(t, "Taxi", row2[4])
		require.Equal(t, "", row2[5]) // Merchant
		require.Equal(t, "Transportation", row2[6])
	})

	t.Run("handles uncategorized expenses", func(t *testing.T) {
		expenses := []models.Expense{
			{
				ID:          1,
				Amount:      decimal.NewFromFloat(5.00),
				Currency:    "SGD",
				Description: "Misc",
				CreatedAt:   time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
				Category:    nil, // No category
			},
		}

		csvData, err := GenerateExpensesCSV(expenses)
		require.NoError(t, err)

		reader := csv.NewReader(strings.NewReader(string(csvData)))
		records, err := reader.ReadAll()
		require.NoError(t, err)
		require.Equal(t, "Uncategorized", records[1][6])
	})

	t.Run("handles empty expense list", func(t *testing.T) {
		expenses := []models.Expense{}

		csvData, err := GenerateExpensesCSV(expenses)
		require.NoError(t, err)

		reader := csv.NewReader(strings.NewReader(string(csvData)))
		records, err := reader.ReadAll()
		require.NoError(t, err)
		require.Len(t, records, 1) // Only header
	})

	t.Run("handles special characters in description", func(t *testing.T) {
		expenses := []models.Expense{
			{
				ID:          1,
				Amount:      decimal.NewFromFloat(10.00),
				Currency:    "SGD",
				Description: "Coffee, \"special\" & tea",
				CreatedAt:   time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
				Category:    &models.Category{Name: "Food"},
			},
		}

		csvData, err := GenerateExpensesCSV(expenses)
		require.NoError(t, err)

		reader := csv.NewReader(strings.NewReader(string(csvData)))
		records, err := reader.ReadAll()
		require.NoError(t, err)
		require.Equal(t, "Coffee, \"special\" & tea", records[1][4])
	})

	t.Run("formats decimal amounts correctly", func(t *testing.T) {
		expenses := []models.Expense{
			{
				ID:          1,
				Amount:      decimal.NewFromFloat(5.5),
				Currency:    "SGD",
				Description: "Test",
				CreatedAt:   time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
			},
			{
				ID:          2,
				Amount:      decimal.NewFromFloat(10.123),
				Currency:    "SGD",
				Description: "Test",
				CreatedAt:   time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
			},
		}

		csvData, err := GenerateExpensesCSV(expenses)
		require.NoError(t, err)

		reader := csv.NewReader(strings.NewReader(string(csvData)))
		records, err := reader.ReadAll()
		require.NoError(t, err)
		require.Equal(t, "5.50", records[1][2])
		require.Equal(t, "10.12", records[2][2]) // Truncated to 2 decimals
	})
}

func TestGetWeekDateRange(t *testing.T) {
	t.Run("returns Monday to Sunday range", func(t *testing.T) {
		start, end := getWeekDateRange()

		// Start should be Monday at 00:00:00
		require.Equal(t, time.Monday, start.Weekday())
		require.Equal(t, 0, start.Hour())
		require.Equal(t, 0, start.Minute())
		require.Equal(t, 0, start.Second())

		// End should be 7 days after start
		require.Equal(t, 7*24*time.Hour, end.Sub(start))
	})
}

func TestGetMonthDateRange(t *testing.T) {
	t.Run("returns first day of month to first day of next month", func(t *testing.T) {
		start, end := getMonthDateRange()

		// Start should be first day of month at 00:00:00
		require.Equal(t, 1, start.Day())
		require.Equal(t, 0, start.Hour())
		require.Equal(t, 0, start.Minute())
		require.Equal(t, 0, start.Second())

		// End should be first day of next month
		require.Equal(t, 1, end.Day())
		require.True(t, end.After(start))

		// Month should increment (or wrap to January)
		expectedMonth := start.Month() + 1
		if expectedMonth > 12 {
			expectedMonth = 1
		}
		require.Equal(t, expectedMonth, end.Month())
	})
}

func TestGenerateReportFilename(t *testing.T) {
	t.Parallel()

	t.Run("generates week filename with start date", func(t *testing.T) {
		filename := generateReportFilename("week")
		require.Contains(t, filename, "expenses_week_")
		require.Contains(t, filename, ".csv")
		require.Regexp(t, `expenses_week_\d{4}-\d{2}-\d{2}\.csv`, filename)
	})

	t.Run("generates month filename with year-month", func(t *testing.T) {
		filename := generateReportFilename("month")
		require.Contains(t, filename, "expenses_")
		require.Contains(t, filename, ".csv")
		require.Regexp(t, `expenses_month_\d{4}-\d{2}\.csv`, filename)
	})

	t.Run("generates default filename for unknown period", func(t *testing.T) {
		filename := generateReportFilename("unknown")
		require.Contains(t, filename, "expenses_")
		require.Contains(t, filename, ".csv")
	})
}
