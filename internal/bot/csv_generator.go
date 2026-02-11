package bot

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"gitlab.com/yelinaung/expense-bot/internal/models"
)

const (
	periodWeek  = "week"
	periodMonth = "month"
)

// GenerateExpensesCSV generates a CSV file from a list of expenses.
func GenerateExpensesCSV(expenses []models.Expense) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{"ID", "Date", "Amount", "Currency", "Description", "Merchant", "Category"}
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write expense rows
	for i := range expenses {
		categoryName := "Uncategorized"
		if expenses[i].Category != nil {
			categoryName = expenses[i].Category.Name
		}

		row := []string{
			strconv.FormatInt(expenses[i].UserExpenseNumber, 10),
			expenses[i].CreatedAt.Format("2006-01-02 15:04:05"),
			expenses[i].Amount.StringFixed(2),
			expenses[i].Currency,
			expenses[i].Description,
			expenses[i].Merchant,
			categoryName,
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

// getWeekDateRange returns start and end dates for the current week.
func getWeekDateRange() (time.Time, time.Time) {
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	startOfWeek := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
	endOfWeek := startOfWeek.Add(7 * 24 * time.Hour)
	return startOfWeek, endOfWeek
}

// getMonthDateRange returns start and end dates for the current month.
func getMonthDateRange() (time.Time, time.Time) {
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, 0)
	return startOfMonth, endOfMonth
}

// generateReportFilename creates a descriptive filename for the CSV report.
func generateReportFilename(period string) string {
	now := time.Now()
	switch period {
	case periodWeek:
		start, _ := getWeekDateRange()
		return fmt.Sprintf("expenses_week_%s.csv", start.Format("2006-01-02"))
	case periodMonth:
		return fmt.Sprintf("expenses_month_%s.csv", now.Format("2006-01"))
	default:
		return fmt.Sprintf("expenses_%s.csv", now.Format("2006-01-02"))
	}
}
