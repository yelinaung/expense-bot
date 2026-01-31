package bot

import (
	"fmt"
	"time"

	"github.com/go-analyze/charts"
	"github.com/shopspring/decimal"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

// GenerateExpenseChart creates a pie chart showing expense breakdown by category.
// Returns PNG image as bytes.
func GenerateExpenseChart(expenses []models.Expense, period string) ([]byte, error) {
	if len(expenses) == 0 {
		return nil, fmt.Errorf("no expenses to chart")
	}

	// Aggregate expenses by category
	categoryTotals := aggregateByCategory(expenses)

	// Convert to chart values and names
	var values []float64
	var categoryNames []string

	for categoryName, total := range categoryTotals {
		categoryNames = append(categoryNames, categoryName)
		values = append(values, total.InexactFloat64())
	}

	opt := charts.NewPieChartOptionWithData(values)
	opt.Title = charts.TitleOption{
		Text:             fmt.Sprintf("Expense Breakdown %s\n\n", period),
		Offset:           charts.OffsetCenter,
		FontStyle:        charts.NewFontStyleWithSize(16),
		SubtextFontStyle: charts.NewFontStyleWithSize(10),
	}
	opt.Padding = charts.NewBoxEqual(5)
	opt.Legend = charts.LegendOption{
		SeriesNames: categoryNames,
		Vertical:    charts.Ptr(true),
		Offset: charts.OffsetStr{
			Left: "80%",
			Top:  charts.PositionBottom,
		},
		FontStyle: charts.NewFontStyleWithSize(8),
	}

	p := charts.NewPainter(charts.PainterOptions{
		OutputFormat: charts.ChartOutputPNG,
		Width:        600,
		Height:       400,
	})
	err := p.PieChart(opt)
	if err != nil {
		return nil, fmt.Errorf("failed to create chart: %w", err)
	}

	// Render to PNG bytes
	buf, err := p.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to render chart: %w", err)
	}

	return buf, nil
}

// aggregateByCategory groups expenses and returns category totals.
func aggregateByCategory(expenses []models.Expense) map[string]decimal.Decimal {
	categoryTotals := make(map[string]decimal.Decimal)

	for _, expense := range expenses {
		categoryName := "Uncategorized"
		if expense.Category != nil {
			categoryName = expense.Category.Name
		}

		if existing, ok := categoryTotals[categoryName]; ok {
			categoryTotals[categoryName] = existing.Add(expense.Amount)
		} else {
			categoryTotals[categoryName] = expense.Amount
		}
	}

	return categoryTotals
}

// generateChartFilename creates filename like "chart_week_2026-01-31.png".
func generateChartFilename(period string) string {
	now := time.Now()
	switch period {
	case periodWeek:
		start, _ := getWeekDateRange()
		return fmt.Sprintf("chart_week_%s.png", start.Format("2006-01-02"))
	case periodMonth:
		return fmt.Sprintf("chart_month_%s.png", now.Format("2006-01"))
	default:
		return fmt.Sprintf("chart_%s.png", now.Format("2006-01-02"))
	}
}
