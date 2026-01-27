package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"google.golang.org/genai"
)

// ParseReceiptTimeout is the timeout for Gemini API calls.
const ParseReceiptTimeout = 30 * time.Second

// ErrParseTimeout indicates the Gemini API call timed out.
var ErrParseTimeout = errors.New("receipt parsing timed out")

// ErrNoData indicates no usable data could be extracted from the receipt.
var ErrNoData = errors.New("no usable data extracted from receipt")

// DefaultCategories is the list of expense categories to suggest from.
var DefaultCategories = []string{
	"Food - Dining Out",
	"Food - Grocery",
	"Transportation",
	"Communication",
	"Housing - Mortgage",
	"Housing - Others",
	"Personal Care",
	"Health and Wellness",
	"Education",
	"Entertainment",
	"Credit/Debt Payments",
	"Others",
	"Utilities",
	"Travel & Vacation",
	"Subscriptions",
	"Donations",
}

// ReceiptData contains the extracted data from a receipt image.
type ReceiptData struct {
	Amount            decimal.Decimal
	Merchant          string
	Date              time.Time
	SuggestedCategory string
	Confidence        float64
}

// HasAmount returns true if the amount was extracted.
func (r *ReceiptData) HasAmount() bool {
	return !r.Amount.IsZero()
}

// HasMerchant returns true if the merchant was extracted.
func (r *ReceiptData) HasMerchant() bool {
	return r.Merchant != ""
}

// IsPartial returns true if only some data was extracted.
func (r *ReceiptData) IsPartial() bool {
	return r.HasAmount() != r.HasMerchant()
}

// IsEmpty returns true if no usable data was extracted.
func (r *ReceiptData) IsEmpty() bool {
	return !r.HasAmount() && !r.HasMerchant()
}

// receiptResponse is the JSON structure returned by Gemini.
type receiptResponse struct {
	Amount            string  `json:"amount"`
	Merchant          string  `json:"merchant"`
	Date              string  `json:"date"`
	SuggestedCategory string  `json:"suggested_category"`
	Confidence        float64 `json:"confidence"`
}

// ParseReceipt extracts expense data from a receipt image using Gemini.
// It applies a 30-second timeout to the API call.
func (c *Client) ParseReceipt(ctx context.Context, imageBytes []byte, mimeType string) (*ReceiptData, error) {
	if len(imageBytes) == 0 {
		return nil, fmt.Errorf("image data is required")
	}

	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	// Apply timeout to the Gemini API call.
	timeoutCtx, cancel := context.WithTimeout(ctx, ParseReceiptTimeout)
	defer cancel()

	prompt := buildReceiptPrompt(DefaultCategories)

	resp, err := c.generator.GenerateContent(timeoutCtx, ModelName, []*genai.Content{
		{
			Parts: []*genai.Part{
				{InlineData: &genai.Blob{MIMEType: mimeType, Data: imageBytes}},
				{Text: prompt},
			},
		},
	}, nil)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrParseTimeout
		}
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, fmt.Errorf("no response from Gemini")
	}

	var textContent string
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Text != "" {
			textContent += part.Text
		}
	}

	if textContent == "" {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	data, err := parseReceiptResponse(textContent)
	if err != nil {
		return nil, err
	}

	// Return error if no usable data was extracted.
	if data.IsEmpty() {
		return nil, ErrNoData
	}

	return data, nil
}

func buildReceiptPrompt(categories []string) string {
	categoryList := strings.Join(categories, ", ")
	return fmt.Sprintf(`Analyze this receipt image and extract the following information.
Return ONLY a JSON object with no additional text or markdown formatting.

Required fields:
- amount: The total amount paid (numeric string, e.g., "54.60")
- merchant: The merchant/store name
- date: The date of purchase in YYYY-MM-DD format
- suggested_category: One of these categories that best matches: %s
- confidence: Your confidence in the extraction accuracy (0.0 to 1.0)

If a field cannot be determined, use an empty string for text fields, "0" for amount, or 0.0 for confidence.

Example response:
{"amount": "54.60", "merchant": "Restaurant Name", "date": "2024-01-15", "suggested_category": "Food - Dining Out", "confidence": 0.95}`, categoryList)
}

func parseReceiptResponse(response string) (*ReceiptData, error) {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var rr receiptResponse
	if err := json.Unmarshal([]byte(response), &rr); err != nil {
		return nil, fmt.Errorf("failed to parse receipt response: %w", err)
	}

	data := &ReceiptData{
		Merchant:          rr.Merchant,
		SuggestedCategory: rr.SuggestedCategory,
		Confidence:        rr.Confidence,
	}

	if rr.Amount != "" && rr.Amount != "0" {
		amount, err := decimal.NewFromString(rr.Amount)
		if err != nil {
			return nil, fmt.Errorf("failed to parse amount %q: %w", rr.Amount, err)
		}
		data.Amount = amount
	}

	if rr.Date != "" {
		date, err := time.Parse("2006-01-02", rr.Date)
		if err == nil {
			data.Date = date
		}
	}

	return data, nil
}
