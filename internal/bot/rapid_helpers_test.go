package bot

import (
	"sort"
	"strings"
	"sync"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/models"
	"pgregory.net/rapid"
)

// expenseTestCurrencies is the currency set shared by the rapid and Hegel
// expense generators so both frameworks aggregate over the same codes.
var expenseTestCurrencies = []string{"SGD", "USD", "EUR"}

// buildPositiveAmountString renders a strictly-positive decimal string (up to 2
// fractional digits) from drawn components. Shared by the rapid and Hegel
// positive-amount generators so the "never zero, never negative" shape is
// encoded once: a missing fraction floors the whole part to 1, and an all-zero
// draw bumps the fraction to 1.
func buildPositiveAmountString(whole, frac int, hasFrac bool) string {
	if !hasFrac {
		if whole == 0 {
			whole = 1
		}
		return decimal.NewFromInt(int64(whole)).String()
	}
	if whole == 0 && frac == 0 {
		frac = 1
	}
	return decimal.New(int64(whole*100+frac), -2).String()
}

// isUsableDescWord reports whether w is safe to use as a description word: it
// must not collide with a currency code (e.g. "usd") or currency word (e.g.
// "baht"), which the parser would otherwise extract as the currency and strip
// from the description, breaking downstream description assertions.
func isUsableDescWord(w string) bool {
	u := strings.ToUpper(w)
	if _, ok := models.SupportedCurrencies[u]; ok {
		return false
	}
	if _, ok := currencyWordToCode[u]; ok {
		return false
	}
	return true
}

// newExpenseWithCurrency builds an Expense from drawn components. Shared by the
// rapid and Hegel expense generators.
func newExpenseWithCurrency(v, exp int, cur string) models.Expense {
	return models.Expense{
		Amount:   decimal.New(int64(v), int32(exp)),
		Currency: cur,
	}
}

// taggedInput renders "lunch #tag1 #tag2 ..." from the given tag names. The
// leading non-tag word ensures the cleaned text is non-empty.
func taggedInput(tags []string) string {
	parts := make([]string, 0, 1+len(tags))
	parts = append(parts, "lunch")
	for _, tag := range tags {
		parts = append(parts, "#"+tag)
	}
	return strings.Join(parts, " ")
}

// requirePositiveAmountParses asserts parseAmount accepts s as a strictly
// positive decimal and that the comma decimal separator yields the same amount.
// The require.TestingT parameter lets both rapid (*rapid.T) and Hegel (*hegel.T)
// tests share this body.
func requirePositiveAmountParses(t require.TestingT, s string) {
	amt, err := parseAmount(s)
	require.NoError(t, err, "parseAmount(%q)", s)
	require.True(t, amt.GreaterThan(decimal.Zero), "parseAmount(%q) = %s", s, amt)

	if strings.Contains(s, ".") {
		commaStr := strings.ReplaceAll(s, ".", ",")
		amt2, err2 := parseAmount(commaStr)
		require.NoError(t, err2, "parseAmount(%q)", commaStr)
		require.True(t, amt.Equal(amt2), "comma variant mismatch: %s vs %s", amt, amt2)
	}
}

// requireTagExtractionInvariants asserts extractTags lowercases and
// de-duplicates tags, and that re-extracting from the cleaned text yields none.
func requireTagExtractionInvariants(t require.TestingT, input string) {
	tags, cleaned := extractTags(input)

	for _, tag := range tags {
		require.Equal(t, strings.ToLower(tag), tag, "tag not lowercased: %q", tag)
	}

	seen := map[string]bool{}
	for _, tag := range tags {
		require.False(t, seen[tag], "duplicate tag: %q", tag)
		seen[tag] = true
	}

	tags2, _ := extractTags(cleaned)
	require.Empty(t, tags2, "cleaned text still has tags (cleaned=%q)", cleaned)
}

// requireAmountFirstRoundtrip asserts that "AMOUNT DESCRIPTION" parses back into
// the original amount and description.
func requireAmountFirstRoundtrip(t require.TestingT, amtStr, desc string) {
	input := amtStr + " " + desc
	parsed := ParseExpenseInput(input)
	require.NotNil(t, parsed, "ParseExpenseInput(%q)", input)

	wantAmt, err := decimal.NewFromString(amtStr)
	require.NoError(t, err)
	require.True(t, parsed.Amount.Equal(wantAmt),
		"amount mismatch: got %s, want %s (input=%q)", parsed.Amount, wantAmt, input)
	require.Equal(t, desc, parsed.Description, "input=%q", input)
}

// sortedSupportedCurrencyCodes returns models.SupportedCurrencies keys sorted
// once. Caching keeps rapid.Check loops from rebuilding the slice per iteration
// while preserving deterministic order across runs.
var sortedSupportedCurrencyCodes = sync.OnceValue(func() []string {
	codes := make([]string, 0, len(models.SupportedCurrencies))
	for c := range models.SupportedCurrencies {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	return codes
})

// genSupportedCurrency draws a currency code from models.SupportedCurrencies.
func genSupportedCurrency() *rapid.Generator[string] {
	return rapid.SampledFrom(sortedSupportedCurrencyCodes())
}
