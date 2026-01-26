package bot

import (
	"regexp"
	"strings"

	"github.com/shopspring/decimal"
)

// ParsedExpense represents a parsed expense from user input.
type ParsedExpense struct {
	Amount       decimal.Decimal
	Description  string
	CategoryName string
}

// amountRegex matches amounts like "5", "5.50", "5,50".
var amountRegex = regexp.MustCompile(`^(\d+(?:[.,]\d{1,2})?)`)

// ParseExpenseInput parses free-text expense input like "5.50 Coffee" or "10 Lunch Food - Dining Out".
// Returns nil if the input cannot be parsed as an expense.
func ParseExpenseInput(input string) *ParsedExpense {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
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
	if rest == "" {
		return &ParsedExpense{
			Amount:      amount,
			Description: "",
		}
	}

	return &ParsedExpense{
		Amount:      amount,
		Description: extractDescription(rest),
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
