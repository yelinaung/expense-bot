package bot

import (
	"regexp"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

func FuzzParseAmount(f *testing.F) {
	// Seed corpus with valid amounts.
	f.Add("5.50")
	f.Add("5,50")
	f.Add("100")
	f.Add("0.01")
	f.Add("999999999.99")
	f.Add("0.1")
	f.Add("1")

	// Seed corpus with invalid amounts.
	f.Add("0")
	f.Add("-10")
	f.Add("")
	f.Add("abc")
	f.Add("5.5.5")
	f.Add("NaN")
	f.Add("Inf")
	f.Add("-Inf")
	f.Add("1e10")
	f.Add("1.234567890123456789")
	f.Add("   5.50   ")
	f.Add("5..50")
	f.Add(",50")
	f.Add("50,")
	f.Add(".")
	f.Add(",")

	f.Fuzz(func(t *testing.T, input string) {
		amount, err := parseAmount(input)

		// Invariant 1: If no error, amount must be positive.
		if err == nil {
			if amount.LessThanOrEqual(decimal.Zero) {
				t.Errorf("parseAmount(%q) returned non-positive amount %v without error", input, amount)
			}
		}

		// Invariant 2: Must not return both valid amount and error.
		if err != nil && !amount.Equal(decimal.Zero) {
			t.Errorf("parseAmount(%q) returned non-zero amount %v with error: %v", input, amount, err)
		}
	})
}

func FuzzParseExpenseInput(f *testing.F) {
	// Valid expense formats.
	f.Add("5.50 Coffee")
	f.Add("$10 Lunch")
	f.Add("50 USD Coffee")
	f.Add("S$5.50 Taxi")
	f.Add("€100 Dinner")
	f.Add("¥1000 Ramen")
	f.Add("£50 Groceries")
	f.Add("100 SGD Taxi")
	f.Add("5,50 Coffee")
	f.Add("10.00")
	f.Add("5")

	// Edge cases.
	f.Add("")
	f.Add("Coffee")
	f.Add("$")
	f.Add("USD")
	f.Add("-5 Invalid")
	f.Add("0 Zero")
	f.Add("   ")
	f.Add("5.50")
	f.Add("$0")
	f.Add("$ 5.50 Coffee")
	f.Add("5.50 Coffee USD")
	f.Add("5.50 Coffee SGD extra words")

	// Unicode and special characters.
	f.Add("5.50 コーヒー")
	f.Add("₹500 Food")
	f.Add("₩10000 Korean BBQ")
	f.Add("฿100 Thai Food")

	// Potential injection attempts.
	f.Add("5.50 Coffee\nNew line")
	f.Add("5.50 Coffee\x00null")
	f.Add("5.50 \"quoted\"")

	// With tags.
	f.Add("5.50 Coffee #work")
	f.Add("10 Lunch #team #client")
	f.Add("5 #a #a") // Dedup.

	// Tag edge cases.
	f.Add("5.50 #123")           // Numeric start, rejected.
	f.Add("5.50 Coffee#nospace") // Not a tag.

	tagPattern := regexp.MustCompile(`^[a-z]\w{0,29}$`)

	f.Fuzz(func(t *testing.T, input string) {
		result := ParseExpenseInput(input)

		if result != nil {
			// Invariant 1: Amount must be positive.
			if result.Amount.LessThanOrEqual(decimal.Zero) {
				t.Errorf("ParseExpenseInput(%q) returned non-positive amount: %v", input, result.Amount)
			}

			// Invariant 2: Currency (if set) must be valid.
			if result.Currency != "" {
				if _, ok := models.SupportedCurrencies[result.Currency]; !ok {
					t.Errorf("ParseExpenseInput(%q) returned invalid currency: %s", input, result.Currency)
				}
			}

			// Invariant 3: Tags (if set) must each match ^[a-z]\w{0,29}$ (lowercase, letter-start).
			for _, tag := range result.Tags {
				if !tagPattern.MatchString(tag) {
					t.Errorf("ParseExpenseInput(%q) returned invalid tag: %q", input, tag)
				}
			}

			// Invariant 4: Tags must be deduplicated.
			seen := make(map[string]bool)
			for _, tag := range result.Tags {
				if seen[tag] {
					t.Errorf("ParseExpenseInput(%q) returned duplicate tag: %q", input, tag)
				}
				seen[tag] = true
			}
		}
	})
}

func FuzzParseAddCommand(f *testing.F) {
	// Valid /add command formats.
	f.Add("/add 5.50 Coffee")
	f.Add("/add $10 Lunch")
	f.Add("/add 50 USD Coffee")
	f.Add("/add@bot 5.50 Coffee")
	f.Add("/add@expensebot 10 Food")

	// Edge cases.
	f.Add("/add")
	f.Add("/add ")
	f.Add("/add@bot")
	f.Add("/add 0 Zero")
	f.Add("/add -5 Invalid")
	f.Add("/add Coffee")

	f.Fuzz(func(t *testing.T, input string) {
		result := ParseAddCommand(input)

		if result != nil {
			// Invariant 1: Amount must be positive.
			if result.Amount.LessThanOrEqual(decimal.Zero) {
				t.Errorf("ParseAddCommand(%q) returned non-positive amount: %v", input, result.Amount)
			}

			// Invariant 2: Currency (if set) must be valid.
			if result.Currency != "" {
				if _, ok := models.SupportedCurrencies[result.Currency]; !ok {
					t.Errorf("ParseAddCommand(%q) returned invalid currency: %s", input, result.Currency)
				}
			}
		}
	})
}

func FuzzExtractTags(f *testing.F) {
	// Single tag.
	f.Add("Coffee #work")

	// Multiple tags.
	f.Add("Coffee #work #meeting")
	f.Add("Lunch #team #client #project")

	// Deduplication.
	f.Add("Coffee #work #work")
	f.Add("#a #A") // Case-insensitive dedup.

	// No tags.
	f.Add("Coffee")
	f.Add("")
	f.Add("no hash here")

	// Tag only.
	f.Add("#work")
	f.Add("#a")

	// Invalid tags.
	f.Add("Coffee #123") // Digit start.
	f.Add("#")           // Empty.
	f.Add("Coffee#nospace")

	// Underscores.
	f.Add("Lunch #client_meeting")
	f.Add("#a_b_c")

	// Long tag (31 chars, exceeds 30).
	f.Add("Coffee #abcdefghijklmnopqrstuvwxyz1234")

	// Unicode and special.
	f.Add("5.50 #café")
	f.Add("#タグ")
	f.Add("#work\x00null")

	tagPattern := regexp.MustCompile(`^[a-z]\w{0,29}$`)

	f.Fuzz(func(t *testing.T, input string) {
		tags, cleaned := extractTags(input)

		for _, tag := range tags {
			// Invariant 1: Tags must be lowercase.
			if tag != strings.ToLower(tag) {
				t.Errorf("extractTags(%q) returned non-lowercase tag: %q", input, tag)
			}

			// Invariant 2: Each tag must match the valid pattern.
			if !tagPattern.MatchString(tag) {
				t.Errorf("extractTags(%q) returned invalid tag: %q", input, tag)
			}
		}

		// Invariant 3: Tags must be deduplicated.
		seen := make(map[string]bool)
		for _, tag := range tags {
			if seen[tag] {
				t.Errorf("extractTags(%q) returned duplicate tag: %q", input, tag)
			}
			seen[tag] = true
		}

		// Invariant 4: Cleaned text must not contain any extracted #tag tokens as standalone words.
		for _, tag := range tags {
			for _, word := range strings.Fields(cleaned) {
				if strings.EqualFold(word, "#"+tag) {
					t.Errorf("extractTags(%q) cleaned text still contains #%s: %q", input, tag, cleaned)
				}
			}
		}

		_ = cleaned // Must not panic.
	})
}

func FuzzExtractCommandArgs(f *testing.F) {
	// Standard commands.
	f.Add("/cmd arg", "/cmd")
	f.Add("/cmd@bot arg", "/cmd")
	f.Add("/cmd@bot", "/cmd")
	f.Add("/cmd", "/cmd")
	f.Add("", "/cmd")

	// Multi-word.
	f.Add("/cmd@bot_name My Category Name", "/cmd")

	// Extra spaces.
	f.Add("/cmd   arg  ", "/cmd")
	f.Add("/cmd@bot   ", "/cmd")

	// Rename syntax.
	f.Add("/renamecategory Old -> New", "/renamecategory")

	// Edge cases.
	f.Add("/cmd@", "/cmd")
	f.Add("/cmd@ arg", "/cmd")
	f.Add("/cmd @not_a_mention", "/cmd")

	// Tag commands.
	f.Add("/tag 1 work", "/tag")
	f.Add("/tag@bot 1 work meeting", "/tag")
	f.Add("/untag 1 work", "/untag")
	f.Add("/tags work", "/tags")

	f.Fuzz(func(t *testing.T, text, command string) {
		result := extractCommandArgs(text, command)

		// Invariant 1: Result must be trimmed (no leading/trailing spaces).
		if result != strings.TrimSpace(result) {
			t.Errorf("extractCommandArgs(%q, %q) result not trimmed: %q", text, command, result)
		}
	})
}

func FuzzParseExpenseInputWithCategories(f *testing.F) {
	categories := []string{
		"Food - Dining Out",
		"Food - Groceries",
		"Transportation",
		"Entertainment",
		"Shopping",
	}

	// Valid inputs with categories.
	f.Add("5.50 Coffee Food - Dining Out")
	f.Add("10 Uber Transportation")
	f.Add("100 Movie Entertainment")

	// Edge cases.
	f.Add("5.50 Coffee")
	f.Add("5.50")
	f.Add("")
	f.Add("5.50 Food - Dining Out") // Description matches category exactly.

	f.Fuzz(func(t *testing.T, input string) {
		result := ParseExpenseInputWithCategories(input, categories)

		if result != nil {
			// Invariant 1: Amount must be positive.
			if result.Amount.LessThanOrEqual(decimal.Zero) {
				t.Errorf("ParseExpenseInputWithCategories(%q) returned non-positive amount: %v", input, result.Amount)
			}

			// Invariant 2: CategoryName (if set) must be in the provided list.
			if result.CategoryName != "" {
				found := false
				for _, cat := range categories {
					if cat == result.CategoryName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ParseExpenseInputWithCategories(%q) returned invalid category: %s", input, result.CategoryName)
				}
			}
		}
	})
}
