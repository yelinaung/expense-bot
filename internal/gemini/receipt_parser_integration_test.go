package gemini

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestParseReceipt_Integration(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping integration test")
	}

	ctx := context.Background()
	client, err := NewClient(ctx, apiKey)
	require.NoError(t, err)

	t.Run("parses sample receipt successfully", func(t *testing.T) {
		imageBytes, err := os.ReadFile("../../sample_receipt.jpeg")
		require.NoError(t, err)
		require.NotEmpty(t, imageBytes)

		receiptData, err := client.ParseReceipt(ctx, imageBytes, "image/jpeg")
		require.NoError(t, err)
		require.NotNil(t, receiptData)

		require.True(t, receiptData.HasAmount(), "should extract amount")
		require.True(t, receiptData.HasMerchant(), "should extract merchant")
		require.False(t, receiptData.IsEmpty(), "should not be empty")
		require.False(t, receiptData.IsPartial(), "should have both amount and merchant")

		expectedAmount := decimal.NewFromFloat(54.60)
		require.True(t, receiptData.Amount.Equal(expectedAmount),
			"expected amount 54.60, got %s", receiptData.Amount)

		require.True(t, strings.Contains(strings.ToLower(receiptData.Merchant), "swee choon"),
			"expected merchant to contain 'Swee Choon', got %s", receiptData.Merchant)

		require.Equal(t, "Food - Dining Out", receiptData.SuggestedCategory,
			"expected category 'Food - Dining Out', got %s", receiptData.SuggestedCategory)

		require.GreaterOrEqual(t, receiptData.Confidence, 0.7,
			"expected confidence >= 0.7, got %f", receiptData.Confidence)

		require.False(t, receiptData.Date.IsZero(), "should extract date")

		t.Logf("Extracted receipt data: Amount=%s, Merchant=%s, Date=%s, Category=%s, Confidence=%.2f",
			receiptData.Amount, receiptData.Merchant, receiptData.Date.Format("2006-01-02"),
			receiptData.SuggestedCategory, receiptData.Confidence)
	})

	t.Run("returns error for empty image", func(t *testing.T) {
		_, err := client.ParseReceipt(ctx, []byte{}, "image/jpeg")
		require.Error(t, err)
		require.Contains(t, err.Error(), "image data is required")
	})

	t.Run("handles invalid image gracefully", func(t *testing.T) {
		invalidImage := []byte("not a valid image")
		_, err := client.ParseReceipt(ctx, invalidImage, "image/jpeg")
		require.Error(t, err)
	})
}

func TestNewClient_Integration(t *testing.T) {
	t.Parallel()

	t.Run("fails with empty API key", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		client, err := NewClient(ctx, "")
		require.Error(t, err)
		require.Nil(t, client)
		require.Contains(t, err.Error(), "API key is required")
	})
}
