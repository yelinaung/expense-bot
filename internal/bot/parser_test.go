package bot

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

const (
	invalidAmountFormatParserTest = "invalid amount format"
	fiveFiftyCoffeeParserTest     = "5.50 Coffee"
	addFiveFiftyCoffeeParserTest  = "/add 5.50 Coffee"
	foodDiningOutParserTest       = "Food - Dining Out"
	travelVacationParserTest      = "Travel & Vacation"
	oneEightyNineParserTest       = "189.00"
	ogAlbertParserTest            = "OG Albert"
)

func TestParseAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
		errMsg  string
	}{
		{
			name:  "simple integer",
			input: "25",
			want:  "25.00",
		},
		{
			name:  "decimal with dot",
			input: "25.50",
			want:  "25.50",
		},
		{
			name:  "decimal with comma",
			input: "25,50",
			want:  "25.50",
		},
		{
			name:  "single decimal place",
			input: "25.5",
			want:  "25.50",
		},
		{
			name:  "with leading whitespace",
			input: "  25.50",
			want:  "25.50",
		},
		{
			name:  "with trailing whitespace",
			input: "25.50  ",
			want:  "25.50",
		},
		{
			name:  "large amount",
			input: "9999.99",
			want:  "9999.99",
		},
		{
			name:  "small amount",
			input: "0.01",
			want:  "0.01",
		},
		{
			name:    "zero amount",
			input:   "0",
			wantErr: true,
			errMsg:  "amount must be greater than zero",
		},
		{
			name:    "negative amount",
			input:   "-25.50",
			wantErr: true,
			errMsg:  "amount must be greater than zero",
		},
		{
			name:    "invalid format letters",
			input:   "abc",
			wantErr: true,
			errMsg:  invalidAmountFormatParserTest,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
			errMsg:  invalidAmountFormatParserTest,
		},
		{
			name:    "only whitespace",
			input:   "   ",
			wantErr: true,
			errMsg:  invalidAmountFormatParserTest,
		},
		{
			name:    "mixed letters and numbers",
			input:   "25abc",
			wantErr: true,
			errMsg:  invalidAmountFormatParserTest,
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
			require.Equal(t, tt.want, result.StringFixed(2))
		})
	}
}

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
			input:    fiveFiftyCoffeeParserTest,
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
		{
			name:     "amount with single decimal",
			input:    "5.5 Coffee",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
		},
		{
			name:     "description with special characters",
			input:    "15.00 Coffee @ Starbucks",
			wantAmt:  "15.00",
			wantDesc: "Coffee @ Starbucks",
		},
		{
			name:     "description with numbers",
			input:    "20.00 Order 12345",
			wantAmt:  "20.00",
			wantDesc: "Order 12345",
		},
		{
			name:     "description with hyphen",
			input:    "10.00 Food - Dining",
			wantAmt:  "10.00",
			wantDesc: "Food - Dining",
		},
		{
			name:    "zero decimal",
			input:   "0.00 Coffee",
			wantNil: true,
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
			input:    addFiveFiftyCoffeeParserTest,
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
		{
			name:    "add command with only bot mention",
			input:   "/add@mybot",
			wantNil: true,
		},
		{
			name:     "add command with bot mention and space",
			input:    "/add@mybot 10.00 Snacks",
			wantAmt:  "10.00",
			wantDesc: "Snacks",
		},
		{
			name:    "add command with @ in middle no space after",
			input:   "/add@bot",
			wantNil: true,
		},
		{
			name:     "add command with only amount",
			input:    "/add 50.00",
			wantAmt:  "50.00",
			wantDesc: "",
		},
		{
			name:    "add command with whitespace only after prefix",
			input:   "/add   ",
			wantNil: true,
		},
		{
			name:     "add command comma decimal",
			input:    "/add 5,50 Coffee",
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
			require.Equal(t, tt.wantAmt, result.Amount.StringFixed(2))
			require.Equal(t, tt.wantDesc, result.Description)
		})
	}
}

func TestParseAddCommandWithCategories(t *testing.T) {
	t.Parallel()

	categories := []string{
		foodDiningOutParserTest,
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
			wantCatName: foodDiningOutParserTest,
		},
		{
			name:        "no category match",
			input:       addFiveFiftyCoffeeParserTest,
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
		foodDiningOutParserTest,
		"Transportation",
		travelVacationParserTest,
	}

	tests := []struct {
		name         string
		input        string
		wantNil      bool
		wantAmt      string
		wantDesc     string
		wantCatName  string
		wantCurrency string
	}{
		{
			name:        "free text with category",
			input:       "5.50 Coffee Food - Dining Out",
			wantAmt:     "5.50",
			wantDesc:    "Coffee",
			wantCatName: foodDiningOutParserTest,
		},
		{
			name:        "free text without category",
			input:       fiveFiftyCoffeeParserTest,
			wantAmt:     "5.50",
			wantDesc:    "Coffee",
			wantCatName: "",
		},
		{
			name:        "bracket category syntax",
			input:       "189.00 OG Albert [Travel & Vacation]",
			wantAmt:     oneEightyNineParserTest,
			wantDesc:    ogAlbertParserTest,
			wantCatName: travelVacationParserTest,
		},
		{
			name:         "currency prefix with bracket category",
			input:        "S$189.00 SGD - OG Albert [Travel & Vacation]",
			wantAmt:      oneEightyNineParserTest,
			wantDesc:     ogAlbertParserTest,
			wantCatName:  travelVacationParserTest,
			wantCurrency: "SGD",
		},
		{
			name:         "currency code after amount stripped from description",
			input:        "189.00 SGD OG Albert",
			wantAmt:      oneEightyNineParserTest,
			wantDesc:     ogAlbertParserTest,
			wantCurrency: "SGD",
		},
		{
			name:        "bracket category case insensitive",
			input:       "10.00 Lunch [food - dining out]",
			wantAmt:     "10.00",
			wantDesc:    "Lunch",
			wantCatName: foodDiningOutParserTest,
		},
		{
			name:        "bracket with unknown category falls back to suffix",
			input:       "10.00 Lunch [Unknown Cat] Food - Dining Out",
			wantAmt:     "10.00",
			wantDesc:    "Lunch [Unknown Cat]",
			wantCatName: foodDiningOutParserTest,
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
			if tt.wantCurrency != "" {
				require.Equal(t, tt.wantCurrency, result.Currency)
			}
		})
	}
}

func TestDecimalParsing(t *testing.T) {
	t.Parallel()

	result := ParseExpenseInput("5.50 Test")
	require.NotNil(t, result)
	require.True(t, result.Amount.Equal(decimal.NewFromFloat(5.50)))
}

func TestExtractDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple text",
			input: "Coffee",
			want:  "Coffee",
		},
		{
			name:  "text with leading whitespace",
			input: "  Coffee",
			want:  "Coffee",
		},
		{
			name:  "text with trailing whitespace",
			input: "Coffee  ",
			want:  "Coffee",
		},
		{
			name:  "text with both whitespace",
			input: "  Coffee  ",
			want:  "Coffee",
		},
		{
			name:  "multi-word text",
			input: "Lunch with friends",
			want:  "Lunch with friends",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only whitespace",
			input: "   ",
			want:  "",
		},
		{
			name:  "text with special characters",
			input: "Coffee @ Starbucks - Downtown",
			want:  "Coffee @ Starbucks - Downtown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := extractDescription(tt.input)
			require.Equal(t, tt.want, result)
		})
	}
}

func TestParseAddCommandWithCategoriesEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		categories  []string
		wantNil     bool
		wantAmt     string
		wantDesc    string
		wantCatName string
	}{
		{
			name:        "empty category list",
			input:       addFiveFiftyCoffeeParserTest,
			categories:  []string{},
			wantAmt:     "5.50",
			wantDesc:    "Coffee",
			wantCatName: "",
		},
		{
			name:        "nil category list",
			input:       addFiveFiftyCoffeeParserTest,
			categories:  nil,
			wantAmt:     "5.50",
			wantDesc:    "Coffee",
			wantCatName: "",
		},
		{
			name:        "amount only with categories",
			input:       "/add 5.50",
			categories:  []string{"Food"},
			wantAmt:     "5.50",
			wantDesc:    "",
			wantCatName: "",
		},
		{
			name:        "partial category match should not match",
			input:       "/add 5.50 Coffee Foo",
			categories:  []string{"Food"},
			wantAmt:     "5.50",
			wantDesc:    "Coffee Foo",
			wantCatName: "",
		},
		{
			name:       "invalid input returns nil",
			input:      "/add Coffee",
			categories: []string{"Food"},
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ParseAddCommandWithCategories(tt.input, tt.categories)

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

func TestParseExpenseInputWithCategoriesEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		categories  []string
		wantNil     bool
		wantAmt     string
		wantDesc    string
		wantCatName string
	}{
		{
			name:        "empty category list",
			input:       fiveFiftyCoffeeParserTest,
			categories:  []string{},
			wantAmt:     "5.50",
			wantDesc:    "Coffee",
			wantCatName: "",
		},
		{
			name:        "amount only with categories",
			input:       "5.50",
			categories:  []string{"Food"},
			wantAmt:     "5.50",
			wantDesc:    "",
			wantCatName: "",
		},
		{
			name:       "invalid input returns nil",
			input:      "Coffee",
			categories: []string{"Food"},
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ParseExpenseInputWithCategories(tt.input, tt.categories)

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
