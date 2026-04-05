package gemini

import "fmt"

const (
	testGeminiCategoryFoodDiningOut = "Food - Dining Out"
	testGeminiCategoryFoodGroceries = "Food - Groceries"
	testGeminiCategoryTransport     = "Transportation"
	testGeminiCoffeeShop            = "Coffee Shop"
	testGeminiFakeImage             = "fake-image"
	testGeminiImageJPEG             = "image/jpeg"
	testGeminiFakeAudio             = "fake-audio"
	testGeminiAudioOGG              = "audio/ogg"
	testGeminiNoResponseText        = "no response from Gemini"
	testGeminiRemovesNewlines       = "removes newlines"
	testGeminiRemovesNullBytes      = "removes null bytes"
	testGeminiReasoningTestText     = "This is a test"
)

func receiptJSON(amount, merchant, date string, confidence float64) string {
	return fmt.Sprintf(
		`{"amount": %q, "merchant": %q, "date": %q, "suggested_category": %q, "confidence": %.2f}`,
		amount,
		merchant,
		date,
		testGeminiCategoryFoodDiningOut,
		confidence,
	)
}

func voiceExpenseJSON(amount, description, currency string, confidence float64) string {
	return fmt.Sprintf(
		`{"amount": %q, "description": %q, "currency": %q, "suggested_category": %q, "confidence": %.2f}`,
		amount,
		description,
		currency,
		testGeminiCategoryFoodDiningOut,
		confidence,
	)
}

func markdownJSON(payload string) string {
	return "```json\n" + payload + "\n```"
}
