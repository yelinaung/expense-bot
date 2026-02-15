package bot

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/shopspring/decimal"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

// currencySymbolsByLenDesc is currencySymbolToCode keys sorted by length
// descending so that multi-char symbols (e.g. "S$") are matched before
// single-char ones (e.g. "$").
var currencySymbolsByLenDesc []string

func init() {
	for sym := range currencySymbolToCode {
		currencySymbolsByLenDesc = append(currencySymbolsByLenDesc, sym)
	}
	sort.Slice(currencySymbolsByLenDesc, func(i, j int) bool {
		return len(currencySymbolsByLenDesc[i]) > len(currencySymbolsByLenDesc[j])
	})
}

// errInvalidAmount is returned when the amount is zero or negative.
var errInvalidAmount = errors.New("amount must be greater than zero")

// ParsedExpense represents a parsed expense from user input.
type ParsedExpense struct {
	Amount       decimal.Decimal
	Description  string
	CategoryName string
	Currency     string // Detected currency code (e.g., "USD", "SGD"), empty if not specified
	Tags         []string
}

// amountRegex matches amounts like "5", "5.50", "5,50".
var amountRegex = regexp.MustCompile(`^(\d+(?:[.,]\d{1,2})?)`)

// bracketCategoryRegex matches a trailing [Category Name] in the input.
var bracketCategoryRegex = regexp.MustCompile(`\s*\[([^\]]+)\]\s*$`)

// currencySymbolToCode maps currency symbols to currency codes.
var currencySymbolToCode = map[string]string{
	"$":   "USD", // Default $ to USD; user can override with explicit code
	"€":   "EUR",
	"£":   "GBP",
	"¥":   "JPY",
	"฿":   "THB",
	"₱":   "PHP",
	"₫":   "VND",
	"₩":   "KRW",
	"₹":   "INR",
	"S$":  "SGD",
	"A$":  "AUD",
	"HK$": "HKD",
	"NZ$": "NZD",
	"NT$": "TWD",
	"RM":  "MYR",
	"Rp":  "IDR",
}

var currencyWordToCode = map[string]string{
	"BAHT": "THB",
}

// currencyPrefixRegex matches currency symbols or codes at the start.
// Matches: $, €, £, ¥, S$, A$, RM, Rp, or 3-letter codes like USD, SGD.
var currencyPrefixRegex = regexp.MustCompile(`^(S\$|A\$|HK\$|NZ\$|NT\$|RM|Rp|[$€£¥฿₱₫₩₹]|[A-Z]{3})\s*`)

// currencySuffixRegex matches 3-letter currency codes at the end (e.g., "50 USD").
var currencySuffixRegex = regexp.MustCompile(`\s+([A-Z]{3})$`)

// tagTokenRegex matches a single #tag token (letter start, up to 30 word chars).
var tagTokenRegex = regexp.MustCompile(`^#([a-zA-Z]\w{0,29})$`)

// extractTags extracts #tag tokens from text, removes them, deduplicates, and lowercases.
// It preserves original whitespace in the remaining text.
func extractTags(text string) (tags []string, cleaned string) {
	if !strings.Contains(text, "#") {
		return nil, text
	}

	// Find all tags using word splitting to handle consecutive tags.
	words := strings.Fields(text)
	seen := make(map[string]bool)
	tagWords := make(map[int]bool)

	for i, word := range words {
		if m := tagTokenRegex.FindStringSubmatch(word); len(m) > 1 {
			name := strings.ToLower(m[1])
			if !seen[name] {
				seen[name] = true
				tags = append(tags, name)
			}
			tagWords[i] = true
		}
	}

	if len(tags) == 0 {
		return nil, text
	}

	// Build cleaned string by removing tag words, preserving spacing between non-tag words.
	var remaining []string
	for i, word := range words {
		if !tagWords[i] {
			remaining = append(remaining, word)
		}
	}

	cleaned = strings.Join(remaining, " ")
	return tags, cleaned
}

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

	detectedCurrency, input := parseCurrencyPrefix(input)
	amount, rest := parseAmountAndRest(input)
	if !amount.GreaterThan(decimal.Zero) {
		return nil
	}
	detectedCurrency, rest = parseCurrencyAfterAmount(detectedCurrency, rest)

	// Extract tags from the remaining text.
	var tags []string
	if rest != "" {
		tags, rest = extractTags(rest)
	}

	return &ParsedExpense{
		Amount:      amount,
		Description: extractDescription(rest),
		Currency:    detectedCurrency,
		Tags:        tags,
	}
}

func parseCurrencyPrefix(input string) (currency string, remaining string) {
	prefixMatch := currencyPrefixRegex.FindStringSubmatch(input)
	if len(prefixMatch) <= 1 {
		return "", input
	}

	prefix := prefixMatch[1]
	if code, ok := currencySymbolToCode[prefix]; ok {
		currency = code
	} else if _, ok := models.SupportedCurrencies[prefix]; ok {
		currency = prefix
	}
	if currency == "" {
		return "", input
	}
	return currency, strings.TrimSpace(input[len(prefixMatch[0]):])
}

func parseAmountAndRest(input string) (decimal.Decimal, string) {
	match := amountRegex.FindString(input)
	if match == "" {
		return decimal.Zero, ""
	}
	match = strings.ReplaceAll(match, ",", ".")
	amount, err := decimal.NewFromString(match)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, ""
	}
	return amount, strings.TrimSpace(input[len(match):])
}

func parseCurrencyAfterAmount(detectedCurrency, rest string) (string, string) {
	detectedCurrency, rest = parseTrailingCurrencySymbol(detectedCurrency, rest)
	detectedCurrency, rest = parseImmediateCurrencyCode(detectedCurrency, rest)
	return parseSuffixCurrencyCode(detectedCurrency, rest)
}

func parseTrailingCurrencySymbol(detectedCurrency, rest string) (string, string) {
	if detectedCurrency != "" || rest == "" {
		return detectedCurrency, rest
	}
	for _, symbol := range currencySymbolsByLenDesc {
		if !strings.HasPrefix(rest, symbol) {
			continue
		}
		// Treat trailing "$" as ambiguous: strip it from description but keep
		// currency unset so user default currency applies.
		if symbol != "$" {
			detectedCurrency = currencySymbolToCode[symbol]
		}
		return detectedCurrency, strings.TrimSpace(rest[len(symbol):])
	}
	return detectedCurrency, rest
}

func parseImmediateCurrencyCode(detectedCurrency, rest string) (string, string) {
	if rest == "" {
		return detectedCurrency, rest
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return detectedCurrency, rest
	}

	token := strings.Trim(fields[0], ".,;:")
	code := strings.ToUpper(token)
	if _, ok := models.SupportedCurrencies[code]; !ok {
		wordCode, wordOK := currencyWordToCode[code]
		if !wordOK {
			return detectedCurrency, rest
		}
		code = wordCode
	}
	if detectedCurrency != "" && detectedCurrency != code {
		return detectedCurrency, rest
	}
	if detectedCurrency == "" {
		detectedCurrency = code
	}
	trimmed := strings.TrimSpace(strings.TrimPrefix(rest, fields[0]))
	trimmed = strings.TrimPrefix(trimmed, "-")
	return detectedCurrency, strings.TrimSpace(trimmed)
}

func parseSuffixCurrencyCode(detectedCurrency, rest string) (string, string) {
	if detectedCurrency != "" || rest == "" {
		return detectedCurrency, rest
	}
	upperRest := strings.ToUpper(rest)
	suffixMatch := currencySuffixRegex.FindStringSubmatch(upperRest)
	if len(suffixMatch) <= 1 {
		return detectedCurrency, rest
	}
	code := suffixMatch[1]
	if _, ok := models.SupportedCurrencies[code]; !ok {
		return detectedCurrency, rest
	}
	trimmed := strings.TrimSpace(rest[:len(rest)-len(suffixMatch[0])])
	return code, trimmed
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
// It tries bracket syntax first, then longest suffix match.
func ParseAddCommandWithCategories(input string, categoryNames []string) *ParsedExpense {
	parsed := ParseAddCommand(input)
	if parsed == nil {
		return nil
	}

	if parsed.Description == "" {
		return parsed
	}

	matchBracketCategory(parsed, categoryNames)

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

	matchBracketCategory(parsed, categoryNames)

	return parsed
}

// matchBracketCategory extracts a [Category] from the description, falling
// back to longest-suffix matching against known category names.
func matchBracketCategory(parsed *ParsedExpense, categoryNames []string) {
	if bracketMatch := bracketCategoryRegex.FindStringSubmatch(parsed.Description); len(bracketMatch) > 1 {
		bracketName := bracketMatch[1]
		for _, catName := range categoryNames {
			if strings.EqualFold(catName, bracketName) {
				parsed.Description = strings.TrimSpace(parsed.Description[:len(parsed.Description)-len(bracketMatch[0])])
				parsed.CategoryName = catName
				return
			}
		}
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
		parsed.Description = strings.TrimSpace(parsed.Description[:len(parsed.Description)-matchedLen])
		parsed.CategoryName = matchedCategory
	}
}
