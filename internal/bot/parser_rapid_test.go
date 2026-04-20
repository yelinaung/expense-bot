package bot

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/models"
	"pgregory.net/rapid"
)

// genPositiveAmountString generates a positive decimal string with up to 2 fractional digits.
func genPositiveAmountString() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		whole := rapid.IntRange(0, 1_000_000).Draw(t, "whole")
		frac := rapid.IntRange(0, 99).Draw(t, "frac")
		hasFrac := rapid.Bool().Draw(t, "hasFrac")
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
	})
}

// genDescWord generates a single lowercase word with no digits / tag / bracket markers.
// Filters out words that collide with currency codes (e.g. "usd") or currency
// words (e.g. "baht") since the parser would extract those as the currency and
// shrink the description, breaking downstream assertions.
func genDescWord() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-z]{1,10}`).Filter(func(w string) bool {
		u := strings.ToUpper(w)
		if _, ok := models.SupportedCurrencies[u]; ok {
			return false
		}
		if _, ok := currencyWordToCode[u]; ok {
			return false
		}
		return true
	})
}

// genDescription generates a description of 1..4 words, no digits, no special chars.
func genDescription() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		n := rapid.IntRange(1, 4).Draw(t, "n")
		words := make([]string, n)
		for i := range n {
			words[i] = genDescWord().Draw(t, "word")
		}
		return strings.Join(words, " ")
	})
}

// TestParseAmountAcceptsPositiveDecimals: parseAmount accepts positive decimal strings.
func TestParseAmountAcceptsPositiveDecimals(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		s := genPositiveAmountString().Draw(t, "amountStr")
		amt, err := parseAmount(s)
		require.NoError(t, err, "parseAmount(%q)", s)
		require.True(t, amt.GreaterThan(decimal.Zero), "parseAmount(%q) = %s", s, amt)

		if strings.Contains(s, ".") {
			commaStr := strings.ReplaceAll(s, ".", ",")
			amt2, err2 := parseAmount(commaStr)
			require.NoError(t, err2, "parseAmount(%q)", commaStr)
			require.True(t, amt.Equal(amt2), "comma variant mismatch: %s vs %s", amt, amt2)
		}
	})
}

// TestParseAmountRejectsNonPositive: parseAmount rejects zero or negative.
func TestParseAmountRejectsNonPositive(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 1_000_000).Draw(t, "n")
		frac := rapid.IntRange(0, 99).Draw(t, "frac")
		negative := rapid.Bool().Draw(t, "neg")
		d := decimal.New(int64(n*100+frac), -2)
		if negative {
			d = d.Neg()
		} else if d.IsPositive() {
			t.Skip("positive — covered in other test")
		}
		_, err := parseAmount(d.String())
		require.Error(t, err, "parseAmount(%q) expected error", d.String())
	})
}

// TestExtractTagsIdempotent: second extraction from cleaned text yields no tags.
func TestExtractTagsIdempotent(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 5).Draw(t, "n")
		parts := make([]string, 0, 1+n)
		parts = append(parts, "lunch")
		for range n {
			tag := rapid.StringMatching(`[A-Za-z]{1,10}`).Draw(t, "tag")
			parts = append(parts, "#"+tag)
		}
		input := strings.Join(parts, " ")

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
	})
}

// TestParseExpenseInputAmountFirst: "AMOUNT DESCRIPTION" parses with matching amount + desc.
func TestParseExpenseInputAmountFirst(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		amtStr := genPositiveAmountString().Draw(t, "amount")
		desc := genDescription().Draw(t, "desc")
		input := amtStr + " " + desc

		parsed := ParseExpenseInput(input)
		require.NotNil(t, parsed, "ParseExpenseInput(%q)", input)

		wantAmt, err := decimal.NewFromString(amtStr)
		require.NoError(t, err)
		require.True(t, parsed.Amount.Equal(wantAmt),
			"amount mismatch: got %s, want %s (input=%q)", parsed.Amount, wantAmt, input)
		require.Equal(t, desc, parsed.Description, "input=%q", input)
	})
}

// TestParseExpenseInputNoAmount: pure letters/spaces with no numeric token
// must not parse. Both the leading-amount and reordered-amount paths require
// a parseable amount, so the result is nil.
func TestParseExpenseInputNoAmount(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		desc := genDescription().Draw(t, "desc")
		parsed := ParseExpenseInput(desc)
		require.Nil(t, parsed, "ParseExpenseInput(%q)", desc)
	})
}

// TestParseExpenseInputWhitespaceTolerantAmount: leading/trailing/extra inner
// spaces around "AMOUNT DESC" don't change the parsed amount. Description is
// not compared because parser may normalize whitespace inside the description.
func TestParseExpenseInputWhitespaceTolerantAmount(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		amtStr := genPositiveAmountString().Draw(t, "amount")
		desc := genDescription().Draw(t, "desc")
		lead := rapid.StringMatching(`[ \t]{0,4}`).Draw(t, "lead")
		trail := rapid.StringMatching(`[ \t]{0,4}`).Draw(t, "trail")
		gap := rapid.StringMatching(`[ \t]{1,4}`).Draw(t, "gap")
		base := amtStr + " " + desc
		noisy := lead + amtStr + gap + desc + trail

		parsedBase := ParseExpenseInput(base)
		parsedNoisy := ParseExpenseInput(noisy)
		require.NotNil(t, parsedBase, "base=%q", base)
		require.NotNil(t, parsedNoisy, "noisy=%q", noisy)
		require.True(t, parsedBase.Amount.Equal(parsedNoisy.Amount),
			"amount mismatch: base=%s noisy=%s (base=%q noisy=%q)",
			parsedBase.Amount, parsedNoisy.Amount, base, noisy)
	})
}

// TestParseExpenseInputWithCurrencyPrefix: "CODE AMOUNT DESC" detects currency.
func TestParseExpenseInputWithCurrencyPrefix(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		code := genSupportedCurrency().Draw(t, "code")
		amtStr := genPositiveAmountString().Draw(t, "amount")
		desc := genDescription().Draw(t, "desc")
		input := code + " " + amtStr + " " + desc

		parsed := ParseExpenseInput(input)
		require.NotNil(t, parsed, "ParseExpenseInput(%q)", input)
		// Leading CODE (e.g., "USD") stays set; leading "$" symbol is intentionally cleared.
		require.Equal(t, code, parsed.Currency, "input=%q", input)
	})
}

// TestContainsDigitMatchesStrings: containsDigit equals stdlib behavior.
func TestContainsDigitMatchesStrings(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		got := containsDigit(s)
		want := strings.ContainsAny(s, "0123456789")
		require.Equal(t, want, got, "containsDigit(%q)", s)
	})
}
