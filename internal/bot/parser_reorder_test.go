package bot

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseExpenseInput_DescriptionFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		wantNil      bool
		wantAmt      string
		wantDesc     string
		wantCurrency string
	}{
		{
			name:     "simple description then amount",
			input:    "Coffee 5.50",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
		},
		{
			name:     "multi-word description then amount",
			input:    "Lunch with friends 25.00",
			wantAmt:  "25.00",
			wantDesc: "Lunch with friends",
		},
		{
			name:         "description then amount then currency code",
			input:        "Coffee 5.50 SGD",
			wantAmt:      "5.50",
			wantDesc:     "Coffee",
			wantCurrency: "SGD",
		},
		{
			name:         "description then currency symbol and amount",
			input:        "Taxi S$15",
			wantAmt:      "15.00",
			wantDesc:     "Taxi",
			wantCurrency: "SGD",
		},
		{
			name:         "description then euro amount",
			input:        "Dinner €30",
			wantAmt:      "30.00",
			wantDesc:     "Dinner",
			wantCurrency: "EUR",
		},
		{
			name:     "description then comma decimal",
			input:    "Groceries 12,50",
			wantAmt:  "12.50",
			wantDesc: "Groceries",
		},
		{
			name:     "description then integer amount",
			input:    "Bus 5",
			wantAmt:  "5.00",
			wantDesc: "Bus",
		},
		{
			name:         "description then amount then THB",
			input:        "Street food 100 THB",
			wantAmt:      "100.00",
			wantDesc:     "Street food",
			wantCurrency: "THB",
		},
		{
			name:         "multi-word description then RM attached to amount",
			input:        "Grab taxi RM25",
			wantAmt:      "25.00",
			wantDesc:     "Grab taxi",
			wantCurrency: "MYR",
		},
		{
			name:    "only description no amount",
			input:   "Coffee",
			wantNil: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantNil: true,
		},
		{
			name:    "only whitespace",
			input:   "   ",
			wantNil: true,
		},
		{
			name:    "failed command not reordered",
			input:   "/ADD 5.50 Coffee",
			wantNil: true,
		},
		{
			name:    "prefix with digits not reordered",
			input:   "Order123 5.50",
			wantNil: true,
		},
		{
			name:    "amount in middle of sentence rejected",
			input:   "I have 2 meetings today",
			wantNil: true,
		},
		{
			name:    "number in chat sentence rejected",
			input:   "bought 3 items at the store",
			wantNil: true,
		},
		{
			name:    "amount not at end rejected",
			input:   "abc 5.50 Coffee",
			wantNil: true,
		},
		{
			name:     "leading amount still works",
			input:    "5.50 Coffee",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
		},
		{
			name:         "leading currency still works",
			input:        "SGD 5.50 Coffee",
			wantAmt:      "5.50",
			wantDesc:     "Coffee",
			wantCurrency: "SGD",
		},
		{
			name:     "description then large amount",
			input:    "Rent 1500",
			wantAmt:  "1500.00",
			wantDesc: "Rent",
		},
		{
			name:         "description then amount with suffix USD",
			input:        "Hotel 200 USD",
			wantAmt:      "200.00",
			wantDesc:     "Hotel",
			wantCurrency: "USD",
		},
		{
			name:         "description then yen amount",
			input:        "Ramen ¥1000",
			wantAmt:      "1000.00",
			wantDesc:     "Ramen",
			wantCurrency: "JPY",
		},
		{
			name:         "description then pound amount",
			input:        "Fish and chips £12",
			wantAmt:      "12.00",
			wantDesc:     "Fish and chips",
			wantCurrency: "GBP",
		},
		{
			name:         "trailing dollar sign is ambiguous not USD",
			input:        "Coffee $5.50",
			wantAmt:      "5.50",
			wantDesc:     "Coffee",
			wantCurrency: "",
		},
		{
			name:         "trailing currency symbol after amount",
			input:        "Lunch 10€",
			wantAmt:      "10.00",
			wantDesc:     "Lunch",
			wantCurrency: "EUR",
		},
		{
			name:         "lowercase currency code suffix",
			input:        "Coffee 5.50 sgd",
			wantAmt:      "5.50",
			wantDesc:     "Coffee",
			wantCurrency: "SGD",
		},
		{
			name:         "quantity in description with trailing currency amount",
			input:        "2 Prawn noodles, 10$",
			wantAmt:      "10.00",
			wantDesc:     "2 Prawn noodles,",
			wantCurrency: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ParseExpenseInput(tt.input)

			if tt.wantNil {
				require.Nil(t, result, "expected nil for input: %s", tt.input)
				return
			}

			require.NotNil(t, result, "expected non-nil result for input: %s", tt.input)
			require.Equal(t, tt.wantAmt, result.Amount.StringFixed(2))
			require.Equal(t, tt.wantDesc, result.Description)
			require.Equal(t, tt.wantCurrency, result.Currency)
		})
	}
}

func TestParseExpenseInputWithCategories_DescriptionFirst(t *testing.T) {
	t.Parallel()

	categories := []string{
		"Food - Dining Out",
		"Transportation",
		"Travel & Vacation",
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
			name:        "description first with bracket category",
			input:       "Coffee 5.50 [Food - Dining Out]",
			wantAmt:     "5.50",
			wantDesc:    "Coffee",
			wantCatName: "Food - Dining Out",
		},
		{
			name:         "description first with currency and bracket category",
			input:        "Taxi S$15 [Transportation]",
			wantAmt:      "15.00",
			wantDesc:     "Taxi",
			wantCatName:  "Transportation",
			wantCurrency: "SGD",
		},
		{
			name:         "description first with lowercase currency code and bracket category",
			input:        "Coffee 5.50 sgd [Food - Dining Out]",
			wantAmt:      "5.50",
			wantDesc:     "Coffee",
			wantCatName:  "Food - Dining Out",
			wantCurrency: "SGD",
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

			require.NotNil(t, result, "expected non-nil result for input: %s", tt.input)
			require.Equal(t, tt.wantAmt, result.Amount.StringFixed(2))
			require.Equal(t, tt.wantDesc, result.Description)
			require.Equal(t, tt.wantCatName, result.CategoryName)
			if tt.wantCurrency != "" {
				require.Equal(t, tt.wantCurrency, result.Currency)
			}
		})
	}
}

func TestParseAddCommand_DescriptionFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		wantNil      bool
		wantAmt      string
		wantDesc     string
		wantCurrency string
	}{
		{
			name:     "add command with description first",
			input:    "/add Coffee 5.50",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
		},
		{
			name:         "add command with description first and currency",
			input:        "/add Lunch 10 SGD",
			wantAmt:      "10.00",
			wantDesc:     "Lunch",
			wantCurrency: "SGD",
		},
		{
			name:         "add command with description first and lowercase currency",
			input:        "/add Lunch 10 sgd",
			wantAmt:      "10.00",
			wantDesc:     "Lunch",
			wantCurrency: "SGD",
		},
		{
			name:    "add command with no amount still fails",
			input:   "/add Coffee",
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

			require.NotNil(t, result, "expected non-nil result for input: %s", tt.input)
			require.Equal(t, tt.wantAmt, result.Amount.StringFixed(2))
			require.Equal(t, tt.wantDesc, result.Description)
			if tt.wantCurrency != "" {
				require.Equal(t, tt.wantCurrency, result.Currency)
			}
		})
	}
}

func TestParseExpenseInput_DescriptionFirst_Tags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		wantNil      bool
		wantAmt      string
		wantDesc     string
		wantCurrency string
		wantTags     []string
	}{
		{
			name:     "tag in prefix before amount",
			input:    "Coffee #food 5.50",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
			wantTags: []string{"food"},
		},
		{
			name:     "tag after trailing amount",
			input:    "Coffee 5.50 #snack",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
			wantTags: []string{"snack"},
		},
		{
			name:         "tag in prefix with currency suffix",
			input:        "Lunch #work 10 SGD",
			wantAmt:      "10.00",
			wantDesc:     "Lunch",
			wantCurrency: "SGD",
			wantTags:     []string{"work"},
		},
		{
			name:     "multiple tags in prefix before amount",
			input:    "Coffee #food #morning 5.50",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
			wantTags: []string{"food", "morning"},
		},
		{
			name:     "multiple tags after trailing amount",
			input:    "Coffee 5.50 #snack #office",
			wantAmt:  "5.50",
			wantDesc: "Coffee",
			wantTags: []string{"snack", "office"},
		},
		{
			name:     "tags with bracket category in reordered input",
			input:    "Coffee #food 5.50 [Food - Dining Out]",
			wantAmt:  "5.50",
			wantDesc: "Coffee [Food - Dining Out]",
			wantTags: []string{"food"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ParseExpenseInput(tt.input)

			if tt.wantNil {
				require.Nil(t, result, "expected nil for input: %s", tt.input)
				return
			}

			require.NotNil(t, result, "expected non-nil result for input: %s", tt.input)
			require.Equal(t, tt.wantAmt, result.Amount.StringFixed(2))
			require.Equal(t, tt.wantDesc, result.Description)
			require.Equal(t, tt.wantCurrency, result.Currency)
			if tt.wantTags != nil {
				require.Equal(t, tt.wantTags, result.Tags)
			} else {
				require.Nil(t, result.Tags)
			}
		})
	}
}

func TestContainsDigit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		{"abc", false},
		{"abc123", true},
		{"123", true},
		{"", false},
		{"Coffee", false},
		{"Order1", true},
		{"café", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, containsDigit(tt.input))
		})
	}
}
