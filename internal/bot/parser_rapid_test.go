package bot

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"
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
func genDescWord() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-z]{1,10}`)
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

// genSupportedCurrencyCode draws a code from models.SupportedCurrencies.
func genSupportedCurrencyCode() *rapid.Generator[string] {
	codes := make([]string, 0, len(models.SupportedCurrencies))
	for c := range models.SupportedCurrencies {
		codes = append(codes, c)
	}
	return rapid.SampledFrom(codes)
}

// TestParseAmountAcceptsPositiveDecimals: parseAmount accepts positive decimal strings.
func TestParseAmountAcceptsPositiveDecimals(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := genPositiveAmountString().Draw(t, "amountStr")
		amt, err := parseAmount(s)
		if err != nil {
			t.Fatalf("parseAmount(%q) err: %v", s, err)
		}
		if !amt.GreaterThan(decimal.Zero) {
			t.Fatalf("parseAmount(%q) = %s, want > 0", s, amt)
		}

		// Comma variant parses to same value.
		if strings.Contains(s, ".") {
			commaStr := strings.ReplaceAll(s, ".", ",")
			amt2, err2 := parseAmount(commaStr)
			if err2 != nil {
				t.Fatalf("parseAmount(%q) err: %v", commaStr, err2)
			}
			if !amt.Equal(amt2) {
				t.Fatalf("comma variant mismatch: %s vs %s", amt, amt2)
			}
		}
	})
}

// TestParseAmountRejectsNonPositive: parseAmount rejects zero or negative.
func TestParseAmountRejectsNonPositive(t *testing.T) {
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
		if err == nil {
			t.Fatalf("parseAmount(%q) expected error", d.String())
		}
	})
}

// TestExtractTagsIdempotent: second extraction from cleaned text yields no tags.
func TestExtractTagsIdempotent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 5).Draw(t, "n")
		parts := make([]string, 0, 1+n)
		parts = append(parts, "lunch")
		for range n {
			tag := rapid.StringMatching(`[a-z]{1,10}`).Draw(t, "tag")
			parts = append(parts, "#"+tag)
		}
		input := strings.Join(parts, " ")

		tags, cleaned := extractTags(input)

		// All tags lowercased.
		for _, tag := range tags {
			if tag != strings.ToLower(tag) {
				t.Fatalf("tag not lowercased: %q", tag)
			}
		}

		// Tags are deduplicated.
		seen := map[string]bool{}
		for _, tag := range tags {
			if seen[tag] {
				t.Fatalf("duplicate tag: %q", tag)
			}
			seen[tag] = true
		}

		// Idempotence: cleaned text has no more tags to extract.
		tags2, _ := extractTags(cleaned)
		if len(tags2) != 0 {
			t.Fatalf("cleaned text still has tags: %v (cleaned=%q)", tags2, cleaned)
		}
	})
}

// TestParseExpenseInputAmountFirst: "AMOUNT DESCRIPTION" parses with matching amount + desc.
func TestParseExpenseInputAmountFirst(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		amtStr := genPositiveAmountString().Draw(t, "amount")
		desc := genDescription().Draw(t, "desc")
		input := amtStr + " " + desc

		parsed := ParseExpenseInput(input)
		if parsed == nil {
			t.Fatalf("ParseExpenseInput(%q) = nil", input)
			return
		}

		wantAmt, err := decimal.NewFromString(amtStr)
		if err != nil {
			t.Fatalf("bad amount: %v", err)
		}
		if !parsed.Amount.Equal(wantAmt) {
			t.Fatalf("amount mismatch: got %s, want %s (input=%q)", parsed.Amount, wantAmt, input)
		}
		if parsed.Description != desc {
			t.Fatalf("desc mismatch: got %q, want %q (input=%q)", parsed.Description, desc, input)
		}
	})
}

// TestParseExpenseInputWithCurrencyPrefix: "CODE AMOUNT DESC" detects currency.
func TestParseExpenseInputWithCurrencyPrefix(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		code := genSupportedCurrencyCode().Draw(t, "code")
		amtStr := genPositiveAmountString().Draw(t, "amount")
		desc := genDescription().Draw(t, "desc")
		input := code + " " + amtStr + " " + desc

		parsed := ParseExpenseInput(input)
		if parsed == nil {
			t.Fatalf("ParseExpenseInput(%q) = nil", input)
			return
		}
		// USD via leading "$" symbol is ambiguous and intentionally cleared,
		// but leading currency CODE (e.g., "USD") stays set. Check code path.
		if parsed.Currency != code {
			t.Fatalf("currency mismatch: got %q, want %q (input=%q)", parsed.Currency, code, input)
		}
	})
}

// TestContainsDigitMatchesStrings: containsDigit equals stdlib behavior.
func TestContainsDigitMatchesStrings(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		got := containsDigit(s)
		want := strings.ContainsAny(s, "0123456789")
		if got != want {
			t.Fatalf("containsDigit(%q) = %v, want %v", s, got, want)
		}
	})
}
