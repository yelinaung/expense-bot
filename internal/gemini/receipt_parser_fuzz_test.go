package gemini

import (
	"testing"

	"github.com/shopspring/decimal"
)

func FuzzParseReceiptResponse(f *testing.F) {
	// Valid JSON responses.
	f.Add(`{"amount": "5.50", "merchant": "Coffee Shop", "date": "2024-01-15", "suggested_category": "Food", "confidence": 0.95}`)
	f.Add(`{"amount": "10", "merchant": "Shop"}`)
	f.Add(`{"amount": "0", "merchant": ""}`)

	// Markdown-wrapped (common LLM output).
	f.Add("```json\n{\"amount\": \"10\", \"merchant\": \"Shop\"}\n```")
	f.Add("```\n{\"amount\": \"5.50\"}\n```")

	// Invalid/edge cases.
	f.Add(`{"amount": "abc"}`)
	f.Add(`{}`)
	f.Add(`not json`)
	f.Add(``)
	f.Add(`   `)
	f.Add(`{"amount": "-5.00"}`)
	f.Add(`{"amount": "0", "merchant": "Test", "date": "invalid-date"}`)
	f.Add(`{"amount": "999999999999.99", "merchant": "Big"}`)

	// Prompt injection in fields.
	f.Add(`{"amount": "5.50", "merchant": "Shop\"; DROP TABLE expenses;--"}`)
	f.Add(`{"amount": "5.50", "merchant": "<script>alert(1)</script>"}`)

	// Unicode.
	f.Add(`{"amount": "5.50", "merchant": "コーヒー"}`)
	f.Add(`{"amount": "5.50", "merchant": "Café ☕"}`)

	f.Fuzz(func(t *testing.T, input string) {
		result, err := parseReceiptResponse(input)

		if err == nil && result != nil {
			// Invariant 1: If no error, amount must be non-negative.
			if result.Amount.LessThan(decimal.Zero) {
				t.Errorf("parseReceiptResponse(%q) returned negative amount: %v", input, result.Amount)
			}
		}
	})
}
