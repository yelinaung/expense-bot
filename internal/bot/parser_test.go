package bot

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestParseExpenseInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantNil  bool
		wantAmt  string
		wantDesc string
	}{
		{
			name:     "simple amount and description",
			input:    "5.50 Coffee",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
		},
		{
			name:     "integer amount",
			input:    "10 Lunch",
			wantAmt:  "10.00",
			wantDesc: "Lunch",
		},
		{
			name:     "comma decimal",
			input:    "5,50 Coffee",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
		},
		{
			name:     "amount only",
			input:    "5.50",
			wantAmt:  "5.50",
			wantDesc: "",
		},
		{
			name:     "multi-word description",
			input:    "25.00 Dinner with friends",
			wantAmt:  "25.00",
			wantDesc: "Dinner with friends",
		},
		{
			name:    "empty input",
			input:   "",
			wantNil: true,
		},
		{
			name:    "no amount",
			input:   "Coffee",
			wantNil: true,
		},
		{
			name:    "negative amount",
			input:   "-5.50 Coffee",
			wantNil: true,
		},
		{
			name:     "whitespace handling",
			input:    "  5.50   Coffee  ",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
		},
		{
			name:    "zero amount",
			input:   "0 Coffee",
			wantNil: true,
		},
		{
			name:     "large amount",
			input:    "1234.56 Big purchase",
			wantAmt:  "1234.56",
			wantDesc: "Big purchase",
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
			require.Equal(t, tt.wantAmt, result.Amount.StringFixed(2))
			require.Equal(t, tt.wantDesc, result.Description)
		})
	}
}

func TestParseAddCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantNil  bool
		wantAmt  string
		wantDesc string
	}{
		{
			name:     "simple add command",
			input:    "/add 5.50 Coffee",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
		},
		{
			name:     "add command with bot mention",
			input:    "/add@mybot 5.50 Coffee",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
		},
		{
			name:     "add command with multi-word description",
			input:    "/add 25.00 Dinner with friends Food - Dining Out",
			wantAmt:  "25.00",
			wantDesc: "Dinner with friends Food - Dining Out",
		},
		{
			name:    "add command no amount",
			input:   "/add Coffee",
			wantNil: true,
		},
		{
			name:    "empty add command",
			input:   "/add",
			wantNil: true,
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
			require.Equal(t, tt.wantAmt, result.Amount.StringFixed(2))
			require.Equal(t, tt.wantDesc, result.Description)
		})
	}
}

func TestParseAddCommandWithCategories(t *testing.T) {
	t.Parallel()

	categories := []string{
		"Food - Dining Out",
		"Food - Grocery",
		"Transportation",
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
			name:        "with category at end",
			input:       "/add 5.50 Coffee Food - Dining Out",
			wantAmt:     "5.50",
			wantDesc:    "Coffee",
			wantCatName: "Food - Dining Out",
		},
		{
			name:        "no category match",
			input:       "/add 5.50 Coffee",
			wantAmt:     "5.50",
			wantDesc:    "Coffee",
			wantCatName: "",
		},
		{
			name:        "case insensitive category",
			input:       "/add 10.00 Bus transportation",
			wantAmt:     "10.00",
			wantDesc:    "Bus",
			wantCatName: "Transportation",
		},
		{
			name:        "longer category takes precedence",
			input:       "/add 50.00 Weekly groceries Food - Grocery",
			wantAmt:     "50.00",
			wantDesc:    "Weekly groceries",
			wantCatName: "Food - Grocery",
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
			require.Equal(t, tt.wantAmt, result.Amount.StringFixed(2))
			require.Equal(t, tt.wantDesc, result.Description)
			require.Equal(t, tt.wantCatName, result.CategoryName)
		})
	}
}

func TestParseExpenseInputWithCategories(t *testing.T) {
	t.Parallel()

	categories := []string{
		"Food - Dining Out",
		"Transportation",
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
			name:        "free text with category",
			input:       "5.50 Coffee Food - Dining Out",
			wantAmt:     "5.50",
			wantDesc:    "Coffee",
			wantCatName: "Food - Dining Out",
		},
		{
			name:        "free text without category",
			input:       "5.50 Coffee",
			wantAmt:     "5.50",
			wantDesc:    "Coffee",
			wantCatName: "",
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
			require.Equal(t, tt.wantAmt, result.Amount.StringFixed(2))
			require.Equal(t, tt.wantDesc, result.Description)
			require.Equal(t, tt.wantCatName, result.CategoryName)
		})
	}
}

func TestDecimalParsing(t *testing.T) {
	t.Parallel()

	result := ParseExpenseInput("5.50 Test")
	require.NotNil(t, result)
	require.True(t, result.Amount.Equal(decimal.NewFromFloat(5.50)))
}
