package bot

import (
	"regexp"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/models"
	"hegel.dev/go/hegel"
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

func drawHegelPositiveAmountString(ht *hegel.T) string {
	whole := hegel.Draw(ht, hegel.Integers(0, 1_000_000))
	frac := hegel.Draw(ht, hegel.Integers(0, 99))
	hasFrac := hegel.Draw(ht, hegel.Booleans())
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

// hegelLowerLetters is the lowercase ASCII alphabet used to generate
// description words and tag names that won't collide with digit/symbol markers.
const hegelLowerLetters = "abcdefghijklmnopqrstuvwxyz"

// hegelDescWordGen is the Hegel analog of genDescWord: a single lowercase
// word that won't be mistaken for a currency code or currency word by the
// parser (which would otherwise shrink the description we assert against).
func hegelDescWordGen() hegel.Generator[string] {
	return hegel.Filter(
		hegel.Text().Alphabet(hegelLowerLetters).MinSize(1).MaxSize(10),
		func(w string) bool {
			u := strings.ToUpper(w)
			if _, ok := models.SupportedCurrencies[u]; ok {
				return false
			}
			if _, ok := currencyWordToCode[u]; ok {
				return false
			}
			return true
		},
	)
}

// hegelDescriptionGen is the Hegel analog of genDescription: 1..4 words
// joined by single spaces, no digits or special characters.
func hegelDescriptionGen() hegel.Generator[string] {
	return hegel.Composite(func(tc hegel.TestCase) string {
		n := hegel.Draw(tc, hegel.Integers(1, 4))
		words := make([]string, n)
		for i := range n {
			words[i] = hegel.Draw(tc, hegelDescWordGen())
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

// TestHegelParseAmountAcceptsPositiveDecimals is the Hegel equivalent of the
// positive amount contract: generated positive decimal strings parse as
// strictly positive decimal amounts.
func TestHegelParseAmountAcceptsPositiveDecimals(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		s := drawHegelPositiveAmountString(ht)
		amt, err := parseAmount(s)
		require.NoError(ht, err, "parseAmount(%q)", s)
		require.True(ht, amt.GreaterThan(decimal.Zero), "parseAmount(%q) = %s", s, amt)

		if strings.Contains(s, ".") {
			commaStr := strings.ReplaceAll(s, ".", ",")
			amt2, err2 := parseAmount(commaStr)
			require.NoError(ht, err2, "parseAmount(%q)", commaStr)
			require.True(ht, amt.Equal(amt2), "comma variant mismatch: %s vs %s", amt, amt2)
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

// TestHegelExtractTagsIdempotent is the Hegel equivalent of the tag-extraction
// idempotence contract: extracted tags are lowercased and de-duplicated, and a
// second extraction from the cleaned text yields no further tags.
func TestHegelExtractTagsIdempotent(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		n := hegel.Draw(ht, hegel.Integers(0, 5))
		parts := make([]string, 0, 1+n)
		parts = append(parts, "lunch")
		for range n {
			tag := hegel.Draw(ht, hegel.Text().Alphabet(hegelLowerLetters+strings.ToUpper(hegelLowerLetters)).MinSize(1).MaxSize(10))
			parts = append(parts, "#"+tag)
		}
		input := strings.Join(parts, " ")

		tags, cleaned := extractTags(input)

		for _, tag := range tags {
			require.Equal(ht, strings.ToLower(tag), tag, "tag not lowercased: %q", tag)
		}

		seen := map[string]bool{}
		for _, tag := range tags {
			require.False(ht, seen[tag], "duplicate tag: %q", tag)
			seen[tag] = true
		}

		tags2, _ := extractTags(cleaned)
		require.Empty(ht, tags2, "cleaned text still has tags (cleaned=%q)", cleaned)
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

// TestHegelParseExpenseInputAmountFirst is the Hegel equivalent of the
// amount-first roundtrip: constructing "AMOUNT DESCRIPTION" and parsing it back
// recovers the original amount and description.
func TestHegelParseExpenseInputAmountFirst(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		amtStr := drawHegelPositiveAmountString(ht)
		desc := hegel.Draw(ht, hegelDescriptionGen())
		input := amtStr + " " + desc

		parsed := ParseExpenseInput(input)
		require.NotNil(ht, parsed, "ParseExpenseInput(%q)", input)

		wantAmt, err := decimal.NewFromString(amtStr)
		require.NoError(ht, err)
		require.True(ht, parsed.Amount.Equal(wantAmt),
			"amount mismatch: got %s, want %s (input=%q)", parsed.Amount, wantAmt, input)
		require.Equal(ht, desc, parsed.Description, "input=%q", input)
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

// TestHegelParseExpenseInputInvariants checks the broad parser contract over
// generated Unicode text: if parsing succeeds, the parsed expense must remain
// internally valid.
func TestHegelParseExpenseInputInvariants(t *testing.T) {
	t.Parallel()
	tagPattern := regexp.MustCompile(`^[a-z]\w{0,29}$`)

	hegel.Test(t, func(ht *hegel.T) {
		input := hegel.Draw(ht, hegel.Text().MaxSize(200))
		parsed := ParseExpenseInput(input)
		if parsed == nil {
			return
		}

		require.True(
			ht,
			parsed.Amount.GreaterThan(decimal.Zero),
			"ParseExpenseInput(%q) returned non-positive amount: %s",
			input,
			parsed.Amount,
		)

		if parsed.Currency != "" {
			require.Contains(
				ht,
				models.SupportedCurrencies,
				parsed.Currency,
				"ParseExpenseInput(%q) returned invalid currency: %s",
				input,
				parsed.Currency,
			)
		}

		seen := map[string]bool{}
		for _, tag := range parsed.Tags {
			require.True(
				ht,
				tagPattern.MatchString(tag),
				"ParseExpenseInput(%q) returned invalid tag: %q",
				input,
				tag,
			)
			require.False(
				ht,
				seen[tag],
				"ParseExpenseInput(%q) returned duplicate tag: %q",
				input,
				tag,
			)
			seen[tag] = true
		}
	})
}

// containsASCIIDigit reports whether s contains any ASCII digit (0-9).
// Independent oracle for testing containsDigit via a rune range check
// rather than delegating to the same strings.ContainsAny call.
func containsASCIIDigit(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

// TestContainsDigitMatchesOracle: containsDigit matches an independent
// ASCII-digit check over arbitrary strings.
func TestContainsDigitMatchesOracle(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		got := containsDigit(s)
		require.Equal(t, containsASCIIDigit(s), got, "%q: containsDigit=%v, oracle=%v", s, got, containsASCIIDigit(s))
	})
}

// TestHegelContainsDigitMatchesOracle is the Hegel equivalent of the
// containsDigit oracle, run over full-Unicode text so non-ASCII digit
// characters (e.g. Arabic-Indic or fullwidth digits) exercise the boundary:
// containsDigit reports only ASCII digits, matching the independent
// rune-range oracle.
func TestHegelContainsDigitMatchesOracle(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		s := hegel.Draw(ht, hegel.Text().MaxSize(100))
		got := containsDigit(s)
		require.Equal(ht, containsASCIIDigit(s), got, "containsDigit(%q)", s)
	})
}
