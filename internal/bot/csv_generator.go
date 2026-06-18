package bot

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gitlab.com/yelinaung/expense-bot/internal/models"
)

const (
	periodWeek  = "week"
	periodMonth = "month"

	csvHeaderID          = "ID"
	csvHeaderDate        = "Date"
	csvHeaderAmount      = "Amount"
	csvHeaderCurrency    = "Currency"
	csvHeaderDescription = "Description"
	csvHeaderMerchant    = "Merchant"
	csvHeaderCategory    = "Category"
)

var csvExpenseHeader = []string{
	csvHeaderID,
	csvHeaderDate,
	csvHeaderAmount,
	csvHeaderCurrency,
	csvHeaderDescription,
	csvHeaderMerchant,
	csvHeaderCategory,
}

// sanitizeCSVCell prefixes cell values that could be interpreted as
// formulas by spreadsheet applications.
func sanitizeCSVCell(s string) string {
	if s == "" {
		return s
	}
	trimmed := strings.TrimLeft(s, " ")
	if trimmed == "" {
		return s
	}
	switch trimmed[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

// GenerateExpensesCSV generates a CSV file from a list of expenses.
func GenerateExpensesCSV(expenses []models.Expense) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	if err := writer.Write(csvExpenseHeader); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write expense rows
	for i := range expenses {
		categoryName := categoryUncategorized
		if expenses[i].Category != nil && expenses[i].Category.Name != "" {
			categoryName = expenses[i].Category.Name
		}

		row := []string{
			strconv.FormatInt(expenses[i].UserExpenseNumber, 10),
			expenses[i].CreatedAt.Format("2006-01-02 15:04:05"),
			expenses[i].Amount.StringFixed(2),
			expenses[i].Currency,
			sanitizeCSVCell(expenses[i].Description),
			sanitizeCSVCell(expenses[i].Merchant),
			sanitizeCSVCell(categoryName),
		}

		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("CSV writer error: %w", err)
	}

	return buf.Bytes(), nil
}

// generateReportFilename creates a descriptive filename for the CSV report.
func generateReportFilename(period string, loc *time.Location, now time.Time) string {
	safeLoc := normalizeLocation(loc)
	current := now.In(safeLoc)

	switch period {
	case periodWeek:
		start, _ := getWeekDateRangeAt(current)
		return fmt.Sprintf("expenses_week_%s.csv", start.Format("2006-01-02"))
	case periodMonth:
		start, _ := getMonthDateRangeAt(current)
		return fmt.Sprintf("expenses_month_%s.csv", start.Format("2006-01"))
	default:
		return fmt.Sprintf("expenses_%s.csv", current.Format("2006-01-02"))
	}
}
