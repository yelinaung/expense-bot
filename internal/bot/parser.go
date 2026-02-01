package bot

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/shopspring/decimal"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

// errInvalidAmount is returned when the amount is zero or negative.
var errInvalidAmount = errors.New("amount must be greater than zero")

// ParsedExpense represents a parsed expense from user input.
type ParsedExpense struct {
	Amount       decimal.Decimal
	Description  string
	CategoryName string
	Currency     string // Detected currency code (e.g., "USD", "SGD"), empty if not specified
}

// amountRegex matches amounts like "5", "5.50", "5,50".
var amountRegex = regexp.MustCompile(`^(\d+(?:[.,]\d{1,2})?)`)

// currencySymbolToCode maps currency symbols to currency codes.
var currencySymbolToCode = map[string]string{
	"$":  "USD", // Default $ to USD; user can override with explicit code
	"€":  "EUR",
	"£":  "GBP",
	"¥":  "JPY",
	"฿":  "THB",
	"₱":  "PHP",
	"₫":  "VND",
	"₩":  "KRW",
	"₹":  "INR",
	"S$": "SGD",
	"A$": "AUD",
	"RM": "MYR",
	"Rp": "IDR",
}

// currencyPrefixRegex matches currency symbols or codes at the start.
// Matches: $, €, £, ¥, S$, A$, RM, Rp, or 3-letter codes like USD, SGD.
var currencyPrefixRegex = regexp.MustCompile(`^(S\$|A\$|HK\$|NZ\$|NT\$|RM|Rp|[$€£¥฿₱₫₩₹]|[A-Z]{3})\s*`)

// currencySuffixRegex matches 3-letter currency codes at the end (e.g., "50 USD").
var currencySuffixRegex = regexp.MustCompile(`\s+([A-Z]{3})$`)

// parseAmount parses a string into a decimal amount.
func parseAmount(input string) (decimal.Decimal, error) {
	input = strings.TrimSpace(input)
	input = strings.ReplaceAll(input, ",", ".")

	amount, err := decimal.NewFromString(input)
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid amount format: %w", err)
	}

	if amount.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, errInvalidAmount
	}

	return amount, nil
}

// ParseExpenseInput parses free-text expense input like "5.50 Coffee", "$10 Lunch", or "50 USD Coffee".
// Returns nil if the input cannot be parsed as an expense.
func ParseExpenseInput(input string) *ParsedExpense {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	var detectedCurrency string

	// Check for currency prefix (e.g., "$5.50", "SGD 10", "S$5")
	if prefixMatch := currencyPrefixRegex.FindStringSubmatch(input); len(prefixMatch) > 1 {
		symbol := prefixMatch[1]
		// Convert symbol to code if it's a symbol
		if code, ok := currencySymbolToCode[symbol]; ok {
			detectedCurrency = code
		} else if _, ok := models.SupportedCurrencies[symbol]; ok {
			// It's already a valid currency code
			detectedCurrency = symbol
		}
		if detectedCurrency != "" {
			input = strings.TrimSpace(input[len(prefixMatch[0]):])
		}
	}

	match := amountRegex.FindString(input)
	if match == "" {
		return nil
	}

	match = strings.ReplaceAll(match, ",", ".")
	amount, err := decimal.NewFromString(match)
	if err != nil {
		return nil
	}

	if amount.LessThanOrEqual(decimal.Zero) {
		return nil
	}

	rest := strings.TrimSpace(input[len(match):])

	// Check for currency suffix in description (e.g., "Coffee USD")
	if detectedCurrency == "" && rest != "" {
		upperRest := strings.ToUpper(rest)
		if suffixMatch := currencySuffixRegex.FindStringSubmatch(upperRest); len(suffixMatch) > 1 {
			code := suffixMatch[1]
			if _, ok := models.SupportedCurrencies[code]; ok {
				detectedCurrency = code
				// Remove the currency code from description
				rest = strings.TrimSpace(rest[:len(rest)-len(suffixMatch[0])])
			}
		}
	}

	if rest == "" {
		return &ParsedExpense{
			Amount:      amount,
			Description: "",
			Currency:    detectedCurrency,
		}
	}

	return &ParsedExpense{
		Amount:      amount,
		Description: extractDescription(rest),
		Currency:    detectedCurrency,
	}
}

// extractDescription extracts the description from the input.
// Category matching is handled separately in ParseAddCommandWithCategories.
func extractDescription(input string) string {
	return strings.TrimSpace(input)
}

// ParseAddCommand parses the /add command format: /add <amount> <description> [category].
// Category can be multi-word like "Food - Dining Out".
func ParseAddCommand(input string) *ParsedExpense {
	input = strings.TrimPrefix(input, "/add")
	input = strings.TrimSpace(input)

	idx := strings.Index(input, "@")
	if idx != -1 {
		spaceIdx := strings.Index(input, " ")
		if spaceIdx != -1 && spaceIdx > idx {
			input = strings.TrimSpace(input[spaceIdx:])
		} else if spaceIdx == -1 {
			return nil
		}
	}

	return ParseExpenseInput(input)
}

// ParseAddCommandWithCategories parses /add with category matching.
// It tries to match the longest category name from the end of the input.
func ParseAddCommandWithCategories(input string, categoryNames []string) *ParsedExpense {
	parsed := ParseAddCommand(input)
	if parsed == nil {
		return nil
	}

	if parsed.Description == "" {
		return parsed
	}

	descLower := strings.ToLower(parsed.Description)
	var matchedCategory string
	var matchedLen int

	for _, catName := range categoryNames {
		catLower := strings.ToLower(catName)
		if strings.HasSuffix(descLower, catLower) {
			if len(catName) > matchedLen {
				matchedCategory = catName
				matchedLen = len(catName)
			}
		}
	}

	if matchedCategory != "" {
		descWithoutCat := strings.TrimSpace(parsed.Description[:len(parsed.Description)-matchedLen])
		parsed.Description = descWithoutCat
		parsed.CategoryName = matchedCategory
	}

	return parsed
}

// ParseExpenseInputWithCategories parses free-text with category matching.
func ParseExpenseInputWithCategories(input string, categoryNames []string) *ParsedExpense {
	parsed := ParseExpenseInput(input)
	if parsed == nil {
		return nil
	}

	if parsed.Description == "" {
		return parsed
	}

	descLower := strings.ToLower(parsed.Description)
	var matchedCategory string
	var matchedLen int

	for _, catName := range categoryNames {
		catLower := strings.ToLower(catName)
		if strings.HasSuffix(descLower, catLower) {
			if len(catName) > matchedLen {
				matchedCategory = catName
				matchedLen = len(catName)
			}
		}
	}

	if matchedCategory != "" {
		descWithoutCat := strings.TrimSpace(parsed.Description[:len(parsed.Description)-matchedLen])
		parsed.Description = descWithoutCat
		parsed.CategoryName = matchedCategory
	}

	return parsed
}
