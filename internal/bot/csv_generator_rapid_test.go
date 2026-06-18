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
	"hegel.dev/go/hegel"
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

// TestHegelSanitizeCSVCellIdempotent is the Hegel equivalent of the
// CSV-formula sanitizer idempotence contract: sanitizing twice yields the
// same string as sanitizing once, over full-Unicode input.
func TestHegelSanitizeCSVCellIdempotent(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		s := hegel.Draw(ht, hegel.Text())
		once := sanitizeCSVCell(s)
		twice := sanitizeCSVCell(once)
		require.Equal(ht, once, twice, "not idempotent (in=%q)", s)
	})
}

// TestHegelSanitizeCSVCellPrefixesFormulaStart is the Hegel equivalent: cells
// whose first non-space char is a formula char are prefixed with a single quote.
func TestHegelSanitizeCSVCellPrefixesFormulaStart(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		leadingSpaces := hegel.Draw(ht, hegel.FromRegex(` {0,5}`, true))
		first := hegel.Draw(ht, hegel.SampledFrom([]byte(csvFormulaChars)))
		tail := hegel.Draw(ht, hegel.FromRegex(`[A-Za-z0-9 ]{0,20}`, true))
		s := leadingSpaces + string(first) + tail

		got := sanitizeCSVCell(s)
		require.Equal(ht, "'"+s, got, "input=%q", s)
	})
}

// TestHegelSanitizeCSVCellSafeUnchanged is the Hegel equivalent: cells that
// start (after trim) with a safe character are returned unchanged.
func TestHegelSanitizeCSVCellSafeUnchanged(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		leadingSpaces := hegel.Draw(ht, hegel.FromRegex(` {0,5}`, true))
		first := hegel.Draw(ht, hegel.FromRegex(`[A-Za-z0-9!?#$%^&*()_.,;:"'/\\]`, true))
		ht.Assume(!strings.ContainsAny(first, csvFormulaChars))
		tail := hegel.Draw(ht, hegel.Text())
		s := leadingSpaces + first + tail
		ht.Assume(strings.TrimLeft(s, " ") != "")

		got := sanitizeCSVCell(s)
		require.Equal(ht, s, got, "input=%q", s)
	})
}

// TestHegelGenerateExpensesCSVStructure is the Hegel equivalent: output parses
// as CSV with N+1 rows (header+rows) and 7 fields per row.
func TestHegelGenerateExpensesCSVStructure(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		n := hegel.Draw(ht, hegel.Integers(0, 10))
		exps := make([]models.Expense, n)
		for i := range n {
			exps[i] = models.Expense{
				UserExpenseNumber: int64(i + 1),
				Amount:            decimal.NewFromInt(int64(i + 1)),
				Currency:          hegel.Draw(ht, hegel.SampledFrom(sortedSupportedCurrencyCodes())),
				Description:       hegel.Draw(ht, hegel.FromRegex(`[A-Za-z0-9 ]{0,20}`, true)),
				Merchant:          hegel.Draw(ht, hegel.FromRegex(`[A-Za-z0-9 ]{0,20}`, true)),
				CreatedAt:         hegel.Draw(ht, hegel.Datetimes()),
			}
		}

		data, err := GenerateExpensesCSV(exps)
		require.NoError(ht, err)

		reader := csv.NewReader(bytes.NewReader(data))
		rows, err := reader.ReadAll()
		require.NoError(ht, err)
		require.Len(ht, rows, n+1, "row count")
		for _, row := range rows {
			require.Len(ht, row, 7, "field count")
		}
		require.Equal(ht,
			[]string{"ID", "Date", "Amount", "Currency", "Description", "Merchant", "Category"},
			rows[0])
	})
}

// TestHegelGenerateExpensesCSVNeutralizesFormulas is the Hegel equivalent:
// formula-leading description/merchant fields appear prefixed with a single
// quote in the CSV output.
func TestHegelGenerateExpensesCSVNeutralizesFormulas(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		first := hegel.Draw(ht, hegel.SampledFrom([]byte(csvFormulaChars)))
		tail := hegel.Draw(ht, hegel.FromRegex(`[A-Za-z0-9 ]{0,10}`, true))
		injected := string(first) + tail

		exps := []models.Expense{{
			UserExpenseNumber: 1,
			Amount:            decimal.NewFromInt(1),
			Currency:          hegel.Draw(ht, hegel.SampledFrom(sortedSupportedCurrencyCodes())),
			Description:       injected,
			Merchant:          injected,
			CreatedAt:         hegel.Draw(ht, hegel.Datetimes()),
		}}

		data, err := GenerateExpensesCSV(exps)
		require.NoError(ht, err)

		reader := csv.NewReader(bytes.NewReader(data))
		rows, err := reader.ReadAll()
		require.NoError(ht, err)
		require.Len(ht, rows, 2)

		require.Equal(ht, "'"+injected, rows[1][4], "description cell")
		require.Equal(ht, "'"+injected, rows[1][5], "merchant cell")
	})
}

// csvCategoryColumn renders a single-expense CSV and returns the Category
// cell (the 7th column of the data row). Used by the empty-name property
// tests below. The expense is passed by pointer because models.Expense is
// large enough to trip gocritic's hugeParam check.
func csvCategoryColumn(t require.TestingT, expense *models.Expense) string {
	data, err := GenerateExpensesCSV([]models.Expense{*expense})
	require.NoError(t, err)
	reader := csv.NewReader(bytes.NewReader(data))
	rows, err := reader.ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Len(t, rows[1], 7)
	return rows[1][6]
}

// TestHegelGenerateExpensesCSVEmptyNameIsUncategorized is the focused
// counterexample: a non-nil Category with an empty Name must render the
// Category column as "Uncategorized", exactly like a nil Category does.
// GenerateExpensesCSV only checks Category != nil, so an empty Name leaks
// through as an empty cell instead of "Uncategorized".
func TestHegelGenerateExpensesCSVEmptyNameIsUncategorized(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		amount := hegel.Draw(ht, hegelAmountGen())
		cur := hegel.Draw(ht, hegel.SampledFrom(sortedSupportedCurrencyCodes()))
		base := models.Expense{
			UserExpenseNumber: 1,
			Amount:            amount,
			Currency:          cur,
			CreatedAt:         hegel.Draw(ht, hegel.Datetimes()),
		}

		nilCell := csvCategoryColumn(ht, func() *models.Expense {
			e := base
			e.Category = nil
			return &e
		}())
		emptyCell := csvCategoryColumn(ht, func() *models.Expense {
			e := base
			e.Category = &models.Category{Name: ""}
			return &e
		}())

		require.Equal(ht, nilCell, emptyCell,
			"nil Category and empty-name Category should render the same Category cell")
		require.Equal(ht, categoryUncategorized, emptyCell,
			"empty-name Category should render as %q, got %q", categoryUncategorized, emptyCell)
	})
}

// TestHegelGenerateExpensesCSVCategoryColumnConsistentWithHabitCategoryName
// asserts that the CSV Category column agrees with habitCategoryName's
// notion of an expense's category. Both code paths classify categories, so a
// nil Category and a Category with an empty Name must both yield
// "Uncategorized" rather than diverging.
func TestHegelGenerateExpensesCSVCategoryColumnConsistentWithHabitCategoryName(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		cat := hegel.Draw(ht, hegelCategoryOrNilGen())
		expense := models.Expense{
			UserExpenseNumber: 1,
			Amount:            hegel.Draw(ht, hegelAmountGen()),
			Currency:          hegel.Draw(ht, hegel.SampledFrom(sortedSupportedCurrencyCodes())),
			Category:          cat,
			CreatedAt:         hegel.Draw(ht, hegel.Datetimes()),
		}
		got := csvCategoryColumn(ht, &expense)
		require.Equal(ht, habitCategoryName(&expense), got,
			"CSV Category column disagrees with habitCategoryName (Category=%+v)", cat)
	})
}
