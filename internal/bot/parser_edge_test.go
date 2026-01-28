package bot

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// TestParseAmount_EdgeCases tests additional edge cases for parseAmount.
func TestParseAmount_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
		errMsg  string
	}{
		{
			name:  "very large amount",
			input: "999999999.99",
			want:  "999999999.99",
		},
		{
			name:  "amount with three decimal places truncates",
			input: "10.999",
			want:  "10.999",
		},
		{
			name:  "amount with many decimal places",
			input: "10.12345678",
			want:  "10.12345678",
		},
		{
			name:  "tiny fractional amount",
			input: "0.001",
			want:  "0.001",
		},
		{
			name:  "comma with three decimals",
			input: "10,999",
			want:  "10.999",
		},
		{
			name:    "multiple commas",
			input:   "1,000,50",
			wantErr: true,
			errMsg:  "invalid amount format",
		},
		{
			name:    "multiple dots",
			input:   "10.50.25",
			wantErr: true,
			errMsg:  "invalid amount format",
		},
		{
			name:    "dot and comma mixed",
			input:   "10.50,25",
			wantErr: true,
			errMsg:  "invalid amount format",
		},
		{
			name:    "leading dot",
			input:   ".50",
			want:    "0.50",
			wantErr: false,
		},
		{
			name:    "trailing dot",
			input:   "10.",
			want:    "10",
			wantErr: false,
		},
		{
			name:    "amount with currency symbol",
			input:   "$10.50",
			wantErr: true,
			errMsg:  "invalid amount format",
		},
		{
			name:    "amount with spaces inside",
			input:   "10 50",
			wantErr: true,
			errMsg:  "invalid amount format",
		},
		{
			name:  "scientific notation",
			input: "1e2",
			want:  "100",
		},
		{
			name:  "scientific notation decimal",
			input: "1.5e2",
			want:  "150",
		},
		{
			name:    "special characters",
			input:   "10@50",
			wantErr: true,
			errMsg:  "invalid amount format",
		},
		{
			name:    "unicode digits",
			input:   "१२३",
			wantErr: true,
			errMsg:  "invalid amount format",
		},
		{
			name:    "negative zero",
			input:   "-0",
			wantErr: true,
			errMsg:  "amount must be greater than zero",
		},
		{
			name:    "negative zero decimal",
			input:   "-0.00",
			wantErr: true,
			errMsg:  "amount must be greater than zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := parseAmount(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			expected, _ := decimal.NewFromString(tt.want)
			require.True(t, expected.Equal(result), "expected %s, got %s", tt.want, result.String())
		})
	}
}

// TestParseExpenseInput_EdgeCases tests additional edge cases for ParseExpenseInput.
func TestParseExpenseInput_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantNil  bool
		wantAmt  string
		wantDesc string
	}{
		{
			name:     "amount with many decimals gets truncated by regex",
			input:    "10.12345 Coffee",
			wantAmt:  "10.12",      // Regex only captures \d{1,2} decimals
			wantDesc: "345 Coffee", // Rest becomes description
		},
		{
			name:     "description with emoji",
			input:    "5.50 Coffee ☕",
			wantAmt:  "5.50",
			wantDesc: "Coffee ☕",
		},
		{
			name:     "description with unicode",
			input:    "10.00 Café français",
			wantAmt:  "10.00",
			wantDesc: "Café français",
		},
		{
			name:     "description with newlines",
			input:    "5.50 Coffee\nwith friends",
			wantAmt:  "5.50",
			wantDesc: "Coffee\nwith friends",
		},
		{
			name:     "description with tabs",
			input:    "5.50 Coffee\twith\ttabs",
			wantAmt:  "5.50",
			wantDesc: "Coffee\twith\ttabs",
		},
		{
			name:     "very long description",
			input:    "5.50 " + string(make([]byte, 500)),
			wantAmt:  "5.50",
			wantDesc: string(make([]byte, 500)),
		},
		{
			name:     "description with multiple spaces",
			input:    "5.50   Coffee   with   spaces",
			wantAmt:  "5.50",
			wantDesc: "Coffee   with   spaces",
		},
		{
			name:     "amount with leading zeros",
			input:    "005.50 Coffee",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
		},
		{
			name:     "amount at start with no space",
			input:    "5.50Coffee",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
		},
		{
			name:     "amount with trailing zeros gets truncated by regex",
			input:    "5.5000 Coffee",
			wantAmt:  "5.50",      // Regex captures 5.50
			wantDesc: "00 Coffee", // Rest becomes description
		},
		{
			name:     "text starting with number but not amount",
			input:    "3 items for lunch",
			wantAmt:  "3.00",
			wantDesc: "items for lunch",
		},
		{
			name:     "description starting with number",
			input:    "5.50 123 Main Street",
			wantAmt:  "5.50",
			wantDesc: "123 Main Street",
		},
		{
			name:    "only whitespace input",
			input:   "     ",
			wantNil: true,
		},
		{
			name:     "amount with comma no decimals",
			input:    "5,50",
			wantAmt:  "5.50",
			wantDesc: "",
		},
		{
			name:    "invalid amount with text",
			input:   "abc 5.50 Coffee",
			wantNil: true,
		},
		{
			name:     "description with parentheses",
			input:    "10.00 Coffee (large)",
			wantAmt:  "10.00",
			wantDesc: "Coffee (large)",
		},
		{
			name:     "description with quotes",
			input:    "10.00 \"Special\" Coffee",
			wantAmt:  "10.00",
			wantDesc: "\"Special\" Coffee",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ParseExpenseInput(tt.input)

			if tt.wantNil {
				require.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			expected, _ := decimal.NewFromString(tt.wantAmt)
			require.True(t, expected.Equal(result.Amount), "expected %s, got %s", tt.wantAmt, result.Amount.String())
			require.Equal(t, tt.wantDesc, result.Description)
		})
	}
}

// TestParseAddCommand_EdgeCases tests additional edge cases for ParseAddCommand.
func TestParseAddCommand_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantNil  bool
		wantAmt  string
		wantDesc string
	}{
		{
			name:     "add command with extra spaces",
			input:    "/add     5.50     Coffee",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
		},
		{
			name:     "add command with bot mention and extra text",
			input:    "/add@mybot@extra 5.50 Coffee",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
		},
		{
			name:    "add command with @ but no space",
			input:   "/add@",
			wantNil: true,
		},
		{
			name:     "add command with @ at end",
			input:    "/add 5.50 Coffee@",
			wantAmt:  "5.50",
			wantDesc: "Coffee@",
		},
		{
			name:    "add command case sensitive prefix",
			input:   "/ADD 5.50 Coffee",
			wantNil: true, // "/ADD" doesn't match "/add" prefix
		},
		{
			name:     "add command with newline",
			input:    "/add\n5.50 Coffee",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
		},
		{
			name:     "add command with tab",
			input:    "/add\t5.50 Coffee",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
		},
		{
			name:     "add command with unicode space",
			input:    "/add 5.50 Coffee",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
		},
		{
			name:     "add with bot mention containing numbers",
			input:    "/add@bot123 10.00 Lunch",
			wantAmt:  "10.00",
			wantDesc: "Lunch",
		},
		{
			name:    "add with bot mention no amount after",
			input:   "/add@mybot Coffee",
			wantNil: true,
		},
		{
			name:     "add command with very long bot name",
			input:    "/add@verylongbotnamethatgoeson 5.50 Coffee",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ParseAddCommand(tt.input)

			if tt.wantNil {
				require.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			expected, _ := decimal.NewFromString(tt.wantAmt)
			require.True(t, expected.Equal(result.Amount), "expected %s, got %s", tt.wantAmt, result.Amount.String())
			require.Equal(t, tt.wantDesc, result.Description)
		})
	}
}

// TestParseAddCommandWithCategories_ComplexEdgeCases tests complex category matching scenarios.
func TestParseAddCommandWithCategories_ComplexEdgeCases(t *testing.T) {
	t.Parallel()

	categories := []string{
		"Food",
		"Food - Dining Out",
		"Food - Grocery",
		"Transportation - Bus",
		"Transportation - Taxi",
		"Entertainment",
	}

	tests := []struct {
		name        string
		input       string
		wantNil     bool
		wantAmt     string
		wantDesc    string
		wantCatName string
	}{
		{
			name:        "overlapping categories - longest wins",
			input:       "/add 10.00 Dinner Food - Dining Out",
			wantAmt:     "10.00",
			wantDesc:    "Dinner",
			wantCatName: "Food - Dining Out",
		},
		{
			name:        "category name in description but not at end",
			input:       "/add 10.00 Food from restaurant",
			wantAmt:     "10.00",
			wantDesc:    "Food from restaurant",
			wantCatName: "",
		},
		{
			name:        "multiple category names, last one matches",
			input:       "/add 10.00 Food and Entertainment",
			wantAmt:     "10.00",
			wantDesc:    "Food and",
			wantCatName: "Entertainment",
		},
		{
			name:        "category with special characters in name",
			input:       "/add 10.00 Bus fare Transportation - Bus",
			wantAmt:     "10.00",
			wantDesc:    "Bus fare",
			wantCatName: "Transportation - Bus",
		},
		{
			name:        "description ends with partial category",
			input:       "/add 10.00 Foo",
			wantAmt:     "10.00",
			wantDesc:    "Foo",
			wantCatName: "",
		},
		{
			name:        "whitespace before category",
			input:       "/add 10.00 Dinner   Food - Dining Out",
			wantAmt:     "10.00",
			wantDesc:    "Dinner",
			wantCatName: "Food - Dining Out",
		},
		{
			name:        "category only in description no amount text",
			input:       "/add 10.00 Food",
			wantAmt:     "10.00",
			wantDesc:    "",
			wantCatName: "Food",
		},
		{
			name:        "empty description after category extraction",
			input:       "/add 10.00   Food",
			wantAmt:     "10.00",
			wantDesc:    "",
			wantCatName: "Food",
		},
		{
			name:        "case mismatch in category",
			input:       "/add 10.00 Snacks FOOD",
			wantAmt:     "10.00",
			wantDesc:    "Snacks",
			wantCatName: "Food",
		},
		{
			name:        "category with trailing spaces in input",
			input:       "/add 10.00 Movie Entertainment   ",
			wantAmt:     "10.00",
			wantDesc:    "Movie",
			wantCatName: "Entertainment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ParseAddCommandWithCategories(tt.input, categories)

			if tt.wantNil {
				require.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			expected, _ := decimal.NewFromString(tt.wantAmt)
			require.True(t, expected.Equal(result.Amount), "expected %s, got %s", tt.wantAmt, result.Amount.String())
			require.Equal(t, tt.wantDesc, result.Description)
			require.Equal(t, tt.wantCatName, result.CategoryName)
		})
	}
}

// TestParseExpenseInputWithCategories_ComplexEdgeCases tests complex category matching in free text.
func TestParseExpenseInputWithCategories_ComplexEdgeCases(t *testing.T) {
	t.Parallel()

	categories := []string{
		"Food - Dining Out",
		"Food - Grocery",
		"Utilities",
		"Utilities - Electric",
	}

	tests := []struct {
		name        string
		input       string
		wantNil     bool
		wantAmt     string
		wantDesc    string
		wantCatName string
	}{
		{
			name:        "longer specific category matches over shorter",
			input:       "50.00 Monthly bill Utilities - Electric",
			wantAmt:     "50.00",
			wantDesc:    "Monthly bill",
			wantCatName: "Utilities - Electric",
		},
		{
			name:        "shorter category when longer not present",
			input:       "50.00 Water bill Utilities",
			wantAmt:     "50.00",
			wantDesc:    "Water bill",
			wantCatName: "Utilities",
		},
		{
			name:        "category name repeated in description",
			input:       "10.00 Food from food store Food - Grocery",
			wantAmt:     "10.00",
			wantDesc:    "Food from food store",
			wantCatName: "Food - Grocery",
		},
		{
			name:        "unicode in description with category",
			input:       "10.00 Café ☕ Food - Dining Out",
			wantAmt:     "10.00",
			wantDesc:    "Café ☕",
			wantCatName: "Food - Dining Out",
		},
		{
			name:        "number in description with category",
			input:       "10.00 Order #12345 Food - Dining Out",
			wantAmt:     "10.00",
			wantDesc:    "Order #12345",
			wantCatName: "Food - Dining Out",
		},
		{
			name:    "invalid amount with categories",
			input:   "Coffee Food - Dining Out",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ParseExpenseInputWithCategories(tt.input, categories)

			if tt.wantNil {
				require.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			expected, _ := decimal.NewFromString(tt.wantAmt)
			require.True(t, expected.Equal(result.Amount), "expected %s, got %s", tt.wantAmt, result.Amount.String())
			require.Equal(t, tt.wantDesc, result.Description)
			require.Equal(t, tt.wantCatName, result.CategoryName)
		})
	}
}
