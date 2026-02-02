package bot

import (
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
