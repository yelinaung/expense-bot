package bot

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

// isFormulaSafe reports whether a CSV cell cannot be interpreted as a
// spreadsheet formula: after stripping leading spaces it must not start
// with a formula trigger character.
func isFormulaSafe(cell string) bool {
	trimmed := strings.TrimLeft(cell, " ")
	if trimmed == "" {
		return true
	}
	switch trimmed[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return false
	}
	return true
}

func FuzzSanitizeCSVCell(f *testing.F) {
	f.Add("normal text")
	f.Add("")
	f.Add("=1+2")
	f.Add("+SUM(A1:A9)")
	f.Add("-2+3")
	f.Add("@cmd")
	f.Add("\t=1")
	f.Add("\r=1")
	f.Add("  =1")
	f.Add("   ")
	f.Add("'already quoted")
	f.Add("Café ☕")
	f.Add("multi\nline")

	f.Fuzz(func(t *testing.T, input string) {
		result := sanitizeCSVCell(input)

		// Invariant 1: output is either the input or the input with a quote prefix.
		if result != input && result != "'"+input {
			t.Errorf("sanitizeCSVCell(%q) = %q, want input or quote-prefixed input", input, result)
		}

		// Invariant 2: output is never a spreadsheet formula.
		if !isFormulaSafe(result) {
			t.Errorf("sanitizeCSVCell(%q) = %q, still formula-unsafe", input, result)
		}

		// Invariant 3: sanitizing is idempotent.
		if again := sanitizeCSVCell(result); again != result {
			t.Errorf("sanitizeCSVCell not idempotent: %q -> %q -> %q", input, result, again)
		}
	})
}

func FuzzGenerateExpensesCSV(f *testing.F) {
	f.Add("Coffee", "Starbucks", "USD", "Food", int64(1), int64(550), true, true)
	f.Add("", "", "", "", int64(0), int64(0), false, false)
	f.Add("=1+2", "@cmd", "-USD", "+Food", int64(42), int64(-100), true, false)
	f.Add("comma, in, field", "quote \" here", "SGD", "Food", int64(7), int64(1), false, true)
	f.Add("multi\nline", "tab\there", "THB", "Café", int64(9), int64(99999999), true, true)
	f.Add("\r\n", "\r", "\n", "'", int64(-1), int64(1), false, false)

	f.Fuzz(func(t *testing.T, description, merchant, currency, category string, expenseNum, amountCents int64, hasWorthIt, worthIt bool) {
		expense := models.Expense{
			UserExpenseNumber: expenseNum,
			Amount:            decimal.New(amountCents, -2),
			Currency:          currency,
			Description:       description,
			Merchant:          merchant,
			CreatedAt:         time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		}
		if category != "" {
			expense.Category = &models.Category{Name: category}
		}
		if hasWorthIt {
			expense.WorthIt = &worthIt
		}

		data, err := GenerateExpensesCSV([]models.Expense{expense})
		if err != nil {
			t.Fatalf("GenerateExpensesCSV failed: %v", err)
		}

		// Invariant 1: output must parse back as CSV with a header and one row,
		// each with the full set of columns.
		records, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
		if err != nil {
			t.Fatalf("generated CSV does not parse back: %v\ndata: %q", err, data)
		}
		if len(records) != 2 {
			t.Fatalf("got %d records, want 2 (header + row)", len(records))
		}
		for _, record := range records {
			if len(record) != len(csvExpenseHeader) {
				t.Errorf("record has %d fields, want %d: %q", len(record), len(csvExpenseHeader), record)
			}
		}

		// Invariant 2: user-controlled cells are formula-safe after parsing.
		row := records[1]
		for i, cell := range row {
			// Currency is written unsanitized by design; check the sanitized columns.
			if csvExpenseHeader[i] == csvHeaderDescription ||
				csvExpenseHeader[i] == csvHeaderMerchant ||
				csvExpenseHeader[i] == csvHeaderCategory {
				if !isFormulaSafe(cell) {
					t.Errorf("%s cell %q is formula-unsafe (input description=%q merchant=%q category=%q)",
						csvExpenseHeader[i], cell, description, merchant, category)
				}
			}
		}
	})
}

func FuzzGenerateReportFilename(f *testing.F) {
	f.Add("week", int64(1705312200))
	f.Add("month", int64(1705312200))
	f.Add("day", int64(1705312200))
	f.Add("", int64(0))
	f.Add("WEEK", int64(-1))
	f.Add("month; rm -rf /", int64(1<<40))
	f.Add("../../../etc/passwd", int64(1))
	f.Add("week", int64(-1<<40))

	f.Fuzz(func(t *testing.T, period string, sec int64) {
		name := generateReportFilename(period, time.UTC, time.Unix(sec, 0))

		// Invariant 1: filename shape is always expenses_*.csv.
		if !strings.HasPrefix(name, "expenses_") || !strings.HasSuffix(name, ".csv") {
			t.Errorf("generateReportFilename(%q, %d) = %q, want expenses_*.csv", period, sec, name)
		}

		// Invariant 2: no path traversal characters leak into the filename.
		if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
			t.Errorf("generateReportFilename(%q, %d) = %q contains path characters", period, sec, name)
		}
	})
}
