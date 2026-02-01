package bot

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseExpenseInput_Currency(t *testing.T) {
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
			name:         "dollar sign prefix",
			input:        "$10 Coffee",
			wantAmt:      "10.00",
			wantDesc:     "Coffee",
			wantCurrency: "USD",
		},
		{
			name:         "euro sign prefix",
			input:        "€5.50 Lunch",
			wantAmt:      "5.50",
			wantDesc:     "Lunch",
			wantCurrency: "EUR",
		},
		{
			name:         "pound sign prefix",
			input:        "£20 Dinner",
			wantAmt:      "20.00",
			wantDesc:     "Dinner",
			wantCurrency: "GBP",
		},
		{
			name:         "SGD prefix",
			input:        "S$15 Taxi",
			wantAmt:      "15.00",
			wantDesc:     "Taxi",
			wantCurrency: "SGD",
		},
		{
			name:         "3-letter code prefix",
			input:        "SGD 25.50 Groceries",
			wantAmt:      "25.50",
			wantDesc:     "Groceries",
			wantCurrency: "SGD",
		},
		{
			name:         "3-letter code suffix",
			input:        "50 Taxi THB",
			wantAmt:      "50.00",
			wantDesc:     "Taxi",
			wantCurrency: "THB",
		},
		{
			name:         "no currency defaults to empty",
			input:        "10.50 Coffee",
			wantAmt:      "10.50",
			wantDesc:     "Coffee",
			wantCurrency: "",
		},
		{
			name:         "yen sign",
			input:        "¥1000 Ramen",
			wantAmt:      "1000.00",
			wantDesc:     "Ramen",
			wantCurrency: "JPY",
		},
		{
			name:         "amount only with currency",
			input:        "$50",
			wantAmt:      "50.00",
			wantDesc:     "",
			wantCurrency: "USD",
		},
		{
			name:         "Malaysian Ringgit",
			input:        "RM 25 Nasi Lemak",
			wantAmt:      "25.00",
			wantDesc:     "Nasi Lemak",
			wantCurrency: "MYR",
		},
		{
			name:         "currency suffix USD",
			input:        "100 Hotel USD",
			wantAmt:      "100.00",
			wantDesc:     "Hotel",
			wantCurrency: "USD",
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

			require.NotNil(t, result, "expected non-nil result for input: %s", tt.input)
			require.Equal(t, tt.wantAmt, result.Amount.StringFixed(2))
			require.Equal(t, tt.wantDesc, result.Description)
			require.Equal(t, tt.wantCurrency, result.Currency)
		})
	}
}

func TestCurrencySymbolToCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		symbol string
		code   string
	}{
		{"$", "USD"},
		{"€", "EUR"},
		{"£", "GBP"},
		{"¥", "JPY"},
		{"S$", "SGD"},
		{"A$", "AUD"},
		{"RM", "MYR"},
		{"Rp", "IDR"},
	}

	for _, tt := range tests {
		t.Run(tt.symbol, func(t *testing.T) {
			t.Parallel()
			code, ok := currencySymbolToCode[tt.symbol]
			require.True(t, ok, "symbol %s should be in map", tt.symbol)
			require.Equal(t, tt.code, code)
		})
	}
}
