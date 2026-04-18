package bot

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/models"
	"pgregory.net/rapid"
)

// genCreatedAt draws a UTC time between 2000 and 2040.
func genCreatedAt() *rapid.Generator[time.Time] {
	return rapid.Custom(func(t *rapid.T) time.Time {
		year := rapid.IntRange(2000, 2040).Draw(t, "year")
		month := rapid.IntRange(1, 12).Draw(t, "month")
		day := rapid.IntRange(1, 28).Draw(t, "day")
		hour := rapid.IntRange(0, 23).Draw(t, "hour")
		minute := rapid.IntRange(0, 59).Draw(t, "minute")
		second := rapid.IntRange(0, 59).Draw(t, "second")
		return time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC)
	})
}

// csvFormulaChars lists the leading characters sanitizeCSVCell must neutralize.
const csvFormulaChars = "=+-@\t\r"

// TestSanitizeCSVCellEmptyStays: empty in → empty out.
func TestSanitizeCSVCellEmptyStays(t *testing.T) {
	t.Parallel()
	require.Empty(t, sanitizeCSVCell(""))
}

// TestSanitizeCSVCellIdempotent: sanitizing twice equals sanitizing once.
func TestSanitizeCSVCellIdempotent(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		once := sanitizeCSVCell(s)
		twice := sanitizeCSVCell(once)
		require.Equal(t, once, twice, "not idempotent (in=%q)", s)
	})
}

// TestSanitizeCSVCellPrefixesFormulaStart: cells whose first non-space char is
// a formula char are prefixed with a single quote.
func TestSanitizeCSVCellPrefixesFormulaStart(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		leadingSpaces := rapid.StringMatching(` {0,5}`).Draw(t, "spaces")
		first := rapid.SampledFrom([]byte(csvFormulaChars)).Draw(t, "first")
		tail := rapid.StringMatching(`[A-Za-z0-9 ]{0,20}`).Draw(t, "tail")
		s := leadingSpaces + string(first) + tail

		got := sanitizeCSVCell(s)
		require.Equal(t, "'"+s, got, "input=%q", s)
	})
}

// TestSanitizeCSVCellSafeUnchanged: cells that start (after trim) with a safe
// character are returned unchanged.
func TestSanitizeCSVCellSafeUnchanged(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		leadingSpaces := rapid.StringMatching(` {0,5}`).Draw(t, "spaces")
		// First non-space char must be safe.
		first := rapid.StringMatching(`[A-Za-z0-9!?#$%^&*()_.,;:"'/\\]`).Draw(t, "first")
		if strings.ContainsAny(first, csvFormulaChars) {
			t.Skip("first char is a formula char")
		}
		tail := rapid.String().Draw(t, "tail")
		s := leadingSpaces + first + tail
		// Reject if the whole thing becomes empty after trim (covered elsewhere).
		if strings.TrimLeft(s, " ") == "" {
			t.Skip("empty after trim")
		}

		got := sanitizeCSVCell(s)
		require.Equal(t, s, got, "input=%q", s)
	})
}

// TestGenerateExpensesCSVStructure: output parses as CSV with N+1 rows (header+rows)
// and 7 fields per row.
func TestGenerateExpensesCSVStructure(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 10).Draw(t, "n")
		exps := make([]models.Expense, n)
		for i := range n {
			exps[i] = models.Expense{
				UserExpenseNumber: int64(i + 1),
				Amount:            decimal.NewFromInt(int64(i + 1)),
				Currency:          genSupportedCurrency().Draw(t, "currency"),
				Description:       rapid.StringMatching(`[A-Za-z0-9 ]{0,20}`).Draw(t, "desc"),
				Merchant:          rapid.StringMatching(`[A-Za-z0-9 ]{0,20}`).Draw(t, "merch"),
				CreatedAt:         genCreatedAt().Draw(t, "createdAt"),
			}
		}

		data, err := GenerateExpensesCSV(exps)
		require.NoError(t, err)

		reader := csv.NewReader(bytes.NewReader(data))
		rows, err := reader.ReadAll()
		require.NoError(t, err)
		require.Len(t, rows, n+1, "row count")
		for _, row := range rows {
			require.Len(t, row, 7, "field count")
		}
		// Header fixed.
		require.Equal(t,
			[]string{"ID", "Date", "Amount", "Currency", "Description", "Merchant", "Category"},
			rows[0])
	})
}

// TestGenerateExpensesCSVNeutralizesFormulas: formula-leading description/merchant
// fields appear prefixed with a single quote in the CSV output.
func TestGenerateExpensesCSVNeutralizesFormulas(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		first := rapid.SampledFrom([]byte(csvFormulaChars)).Draw(t, "first")
		tail := rapid.StringMatching(`[A-Za-z0-9 ]{0,10}`).Draw(t, "tail")
		injected := string(first) + tail

		exps := []models.Expense{{
			UserExpenseNumber: 1,
			Amount:            decimal.NewFromInt(1),
			Currency:          genSupportedCurrency().Draw(t, "currency"),
			Description:       injected,
			Merchant:          injected,
			CreatedAt:         genCreatedAt().Draw(t, "createdAt"),
		}}

		data, err := GenerateExpensesCSV(exps)
		require.NoError(t, err)

		reader := csv.NewReader(bytes.NewReader(data))
		rows, err := reader.ReadAll()
		require.NoError(t, err)
		require.Len(t, rows, 2)

		require.Equal(t, "'"+injected, rows[1][4], "description cell")
		require.Equal(t, "'"+injected, rows[1][5], "merchant cell")
	})
}
