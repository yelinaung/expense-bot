package gemini

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestBuildReceiptPrompt(t *testing.T) {
	t.Parallel()

	categories := []string{"Food - Dining Out", "Transportation"}
	prompt := buildReceiptPrompt(categories)

	require.Contains(t, prompt, "Food - Dining Out")
	require.Contains(t, prompt, "Transportation")
	require.Contains(t, prompt, "amount")
	require.Contains(t, prompt, "merchant")
	require.Contains(t, prompt, "date")
	require.Contains(t, prompt, "suggested_category")
	require.Contains(t, prompt, "confidence")
}

func TestParseReceiptResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response string
		want     *ReceiptData
		wantErr  bool
	}{
		{
			name:     "valid complete response",
			response: `{"amount": "54.60", "merchant": "Swee Choon Tim Sum Restaurant", "date": "2019-04-21", "suggested_category": "Food - Dining Out", "confidence": 0.95}`,
			want: &ReceiptData{
				Amount:            decimal.NewFromFloat(54.60),
				Merchant:          "Swee Choon Tim Sum Restaurant",
				Date:              time.Date(2019, 4, 21, 0, 0, 0, 0, time.UTC),
				SuggestedCategory: "Food - Dining Out",
				Confidence:        0.95,
			},
			wantErr: false,
		},
		{
			name:     "response with markdown code block",
			response: "```json\n{\"amount\": \"10.50\", \"merchant\": \"Store\", \"date\": \"2024-01-15\", \"suggested_category\": \"Others\", \"confidence\": 0.8}\n```",
			want: &ReceiptData{
				Amount:            decimal.NewFromFloat(10.50),
				Merchant:          "Store",
				Date:              time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				SuggestedCategory: "Others",
				Confidence:        0.8,
			},
			wantErr: false,
		},
		{
			name:     "partial response - missing date",
			response: `{"amount": "25.00", "merchant": "Coffee Shop", "date": "", "suggested_category": "Food - Dining Out", "confidence": 0.7}`,
			want: &ReceiptData{
				Amount:            decimal.NewFromFloat(25.00),
				Merchant:          "Coffee Shop",
				Date:              time.Time{},
				SuggestedCategory: "Food - Dining Out",
				Confidence:        0.7,
			},
			wantErr: false,
		},
		{
			name:     "partial response - zero amount",
			response: `{"amount": "0", "merchant": "Unknown", "date": "2024-01-01", "suggested_category": "", "confidence": 0.3}`,
			want: &ReceiptData{
				Amount:            decimal.Zero,
				Merchant:          "Unknown",
				Date:              time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				SuggestedCategory: "",
				Confidence:        0.3,
			},
			wantErr: false,
		},
		{
			name:     "invalid json",
			response: `not valid json`,
			want:     nil,
			wantErr:  true,
		},
		{
			name:     "invalid amount format",
			response: `{"amount": "not-a-number", "merchant": "Store", "date": "2024-01-01", "suggested_category": "Others", "confidence": 0.5}`,
			want:     nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseReceiptResponse(tt.response)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.True(t, tt.want.Amount.Equal(got.Amount), "amount mismatch: want %s, got %s", tt.want.Amount, got.Amount)
			require.Equal(t, tt.want.Merchant, got.Merchant)
			require.Equal(t, tt.want.Date, got.Date)
			require.Equal(t, tt.want.SuggestedCategory, got.SuggestedCategory)
			require.InDelta(t, tt.want.Confidence, got.Confidence, 0.001)
		})
	}
}

func TestDefaultCategories(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, DefaultCategories)
	require.Contains(t, DefaultCategories, "Food - Dining Out")
	require.Contains(t, DefaultCategories, "Food - Grocery")
	require.Contains(t, DefaultCategories, "Transportation")
}

func TestReceiptData_HasAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		amount decimal.Decimal
		want   bool
	}{
		{"zero amount", decimal.Zero, false},
		{"non-zero amount", decimal.NewFromFloat(10.50), true},
		{"negative amount", decimal.NewFromFloat(-5.00), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &ReceiptData{Amount: tt.amount}
			require.Equal(t, tt.want, r.HasAmount())
		})
	}
}

func TestReceiptData_HasMerchant(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		merchant string
		want     bool
	}{
		{"empty merchant", "", false},
		{"non-empty merchant", "Coffee Shop", true},
		{"whitespace only", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &ReceiptData{Merchant: tt.merchant}
			require.Equal(t, tt.want, r.HasMerchant())
		})
	}
}

func TestReceiptData_IsPartial(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		amount   decimal.Decimal
		merchant string
		want     bool
	}{
		{"both present", decimal.NewFromFloat(10.50), "Shop", false},
		{"both missing", decimal.Zero, "", false},
		{"only amount", decimal.NewFromFloat(10.50), "", true},
		{"only merchant", decimal.Zero, "Shop", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &ReceiptData{Amount: tt.amount, Merchant: tt.merchant}
			require.Equal(t, tt.want, r.IsPartial())
		})
	}
}

func TestReceiptData_IsEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		amount   decimal.Decimal
		merchant string
		want     bool
	}{
		{"both present", decimal.NewFromFloat(10.50), "Shop", false},
		{"both missing", decimal.Zero, "", true},
		{"only amount", decimal.NewFromFloat(10.50), "", false},
		{"only merchant", decimal.Zero, "Shop", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &ReceiptData{Amount: tt.amount, Merchant: tt.merchant}
			require.Equal(t, tt.want, r.IsEmpty())
		})
	}
}

func TestParseReceiptResponse_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response string
		want     *ReceiptData
		wantErr  bool
	}{
		{
			name:     "response with only json prefix",
			response: "```json\n{\"amount\": \"15.00\", \"merchant\": \"Cafe\", \"date\": \"\", \"suggested_category\": \"Food - Dining Out\", \"confidence\": 0.8}\n```",
			want: &ReceiptData{
				Amount:            decimal.NewFromFloat(15.00),
				Merchant:          "Cafe",
				Date:              time.Time{},
				SuggestedCategory: "Food - Dining Out",
				Confidence:        0.8,
			},
			wantErr: false,
		},
		{
			name:     "response with extra whitespace",
			response: "  \n\n  {\"amount\": \"20.00\", \"merchant\": \"Store\", \"date\": \"2024-06-15\", \"suggested_category\": \"Others\", \"confidence\": 0.9}  \n\n  ",
			want: &ReceiptData{
				Amount:            decimal.NewFromFloat(20.00),
				Merchant:          "Store",
				Date:              time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
				SuggestedCategory: "Others",
				Confidence:        0.9,
			},
			wantErr: false,
		},
		{
			name:     "empty amount string treated as zero",
			response: `{"amount": "", "merchant": "Shop", "date": "2024-01-01", "suggested_category": "Others", "confidence": 0.5}`,
			want: &ReceiptData{
				Amount:            decimal.Zero,
				Merchant:          "Shop",
				Date:              time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				SuggestedCategory: "Others",
				Confidence:        0.5,
			},
			wantErr: false,
		},
		{
			name:     "invalid date format is ignored",
			response: `{"amount": "30.00", "merchant": "Restaurant", "date": "not-a-date", "suggested_category": "Food - Dining Out", "confidence": 0.75}`,
			want: &ReceiptData{
				Amount:            decimal.NewFromFloat(30.00),
				Merchant:          "Restaurant",
				Date:              time.Time{},
				SuggestedCategory: "Food - Dining Out",
				Confidence:        0.75,
			},
			wantErr: false,
		},
		{
			name:     "decimal amount with many places",
			response: `{"amount": "99.999", "merchant": "Market", "date": "2024-03-20", "suggested_category": "Food - Grocery", "confidence": 0.88}`,
			want: &ReceiptData{
				Amount:            decimal.NewFromFloat(99.999),
				Merchant:          "Market",
				Date:              time.Date(2024, 3, 20, 0, 0, 0, 0, time.UTC),
				SuggestedCategory: "Food - Grocery",
				Confidence:        0.88,
			},
			wantErr: false,
		},
		{
			name:     "empty json object",
			response: `{}`,
			want: &ReceiptData{
				Amount:            decimal.Zero,
				Merchant:          "",
				Date:              time.Time{},
				SuggestedCategory: "",
				Confidence:        0,
			},
			wantErr: false,
		},
		{
			name:     "merchant with special characters",
			response: `{"amount": "45.00", "merchant": "Café & Bar - O'Brien's", "date": "2024-05-10", "suggested_category": "Food - Dining Out", "confidence": 0.92}`,
			want: &ReceiptData{
				Amount:            decimal.NewFromFloat(45.00),
				Merchant:          "Café & Bar - O'Brien's",
				Date:              time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
				SuggestedCategory: "Food - Dining Out",
				Confidence:        0.92,
			},
			wantErr: false,
		},
		{
			name:     "zero confidence",
			response: `{"amount": "10.00", "merchant": "Unknown", "date": "", "suggested_category": "", "confidence": 0}`,
			want: &ReceiptData{
				Amount:            decimal.NewFromFloat(10.00),
				Merchant:          "Unknown",
				Date:              time.Time{},
				SuggestedCategory: "",
				Confidence:        0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseReceiptResponse(tt.response)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.True(t, tt.want.Amount.Equal(got.Amount), "amount mismatch: want %s, got %s", tt.want.Amount, got.Amount)
			require.Equal(t, tt.want.Merchant, got.Merchant)
			require.Equal(t, tt.want.Date, got.Date)
			require.Equal(t, tt.want.SuggestedCategory, got.SuggestedCategory)
			require.InDelta(t, tt.want.Confidence, got.Confidence, 0.001)
		})
	}
}
