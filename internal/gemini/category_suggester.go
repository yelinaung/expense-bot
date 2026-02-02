package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gitlab.com/yelinaung/expense-bot/internal/logger"
	"google.golang.org/genai"
)

// CategorySuggestion represents a suggested category for an expense description.
type CategorySuggestion struct {
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
}

// SuggestCategory uses Gemini to suggest an appropriate category for an expense description.
func (c *Client) SuggestCategory(ctx context.Context, description string, availableCategories []string) (*CategorySuggestion, error) {
	logger.Log.Debug().
		Str("description", description).
		Int("category_count", len(availableCategories)).
		Msg("SuggestCategory called")

	if c.generator == nil {
		logger.Log.Error().Msg("SuggestCategory: gemini client not initialized")
		return nil, fmt.Errorf("gemini client not initialized")
	}

	if description == "" {
		logger.Log.Warn().Msg("SuggestCategory: empty description provided")
		return nil, fmt.Errorf("description is required")
	}

	if len(availableCategories) == 0 {
		logger.Log.Warn().Msg("SuggestCategory: no categories available")
		return nil, fmt.Errorf("no categories available")
	}

	prompt := buildCategorySuggestionPrompt(description, availableCategories)
	logger.Log.Debug().Str("prompt", prompt).Msg("SuggestCategory: sending prompt to Gemini")

	// Set timeout for API call
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	contents := []*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{
				{Text: prompt},
			},
		},
	}

	temp := float32(0.3)
	config := &genai.GenerateContentConfig{
		Temperature:     &temp,      // Lower temperature for more consistent categorization
		MaxOutputTokens: int32(500), // Increased to prevent truncation of reasoning text
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: "You are a JSON API. You MUST respond with ONLY valid JSON, no preamble or explanation. Output a single JSON object."},
			},
		},
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"category": {
					Type:        genai.TypeString,
					Description: "The most appropriate category from the provided list",
				},
				"confidence": {
					Type:        genai.TypeNumber,
					Description: "Confidence score between 0 and 1",
				},
				"reasoning": {
					Type:        genai.TypeString,
					Description: "Brief explanation for the categorization",
				},
			},
			Required: []string{"category", "confidence", "reasoning"},
		},
	}

	resp, err := c.generator.GenerateContent(timeoutCtx, ModelName, contents, config)
	if err != nil {
		logger.Log.Error().Err(err).
			Str("description", description).
			Msg("SuggestCategory: Gemini API call failed")
		return nil, fmt.Errorf("gemini API call failed: %w", err)
	}

	if resp == nil {
		logger.Log.Warn().
			Str("description", description).
			Msg("SuggestCategory: nil response from Gemini")
		return nil, fmt.Errorf("no response from Gemini")
	}

	// Use the built-in Text() method to get concatenated text from all parts
	fullText := resp.Text()

	logger.Log.Debug().
		Str("description", description).
		Str("raw_response", fullText).
		Msg("SuggestCategory: received Gemini response")

	if fullText == "" {
		logger.Log.Warn().
			Str("description", description).
			Msg("SuggestCategory: no text content in Gemini response")
		return nil, fmt.Errorf("no text content in response")
	}

	// Extract JSON from response - Gemini sometimes includes preamble text
	jsonText := extractJSON(fullText)
	if jsonText == "" {
		logger.Log.Warn().
			Str("description", description).
			Str("full_text", fullText).
			Msg("SuggestCategory: no JSON found in Gemini response")
		return nil, fmt.Errorf("no JSON found in response")
	}

	// Parse JSON response
	var suggestion CategorySuggestion
	if err := json.Unmarshal([]byte(jsonText), &suggestion); err != nil {
		logger.Log.Error().Err(err).
			Str("description", description).
			Str("json_text", jsonText).
			Msg("SuggestCategory: failed to parse JSON response")
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	logger.Log.Debug().
		Str("description", description).
		Str("suggested_category", suggestion.Category).
		Float64("confidence", suggestion.Confidence).
		Str("reasoning", suggestion.Reasoning).
		Msg("SuggestCategory: parsed Gemini suggestion")

	// Validate that suggested category is in the available list
	validCategory := false
	for _, cat := range availableCategories {
		if strings.EqualFold(cat, suggestion.Category) {
			suggestion.Category = cat // Use exact case from available list
			validCategory = true
			break
		}
	}

	if !validCategory {
		logger.Log.Warn().
			Str("description", description).
			Str("suggested_category", suggestion.Category).
			Strs("available_categories", availableCategories).
			Msg("SuggestCategory: suggested category not in available list")
		return nil, fmt.Errorf("suggested category '%s' not in available categories", suggestion.Category)
	}

	logger.Log.Debug().
		Str("description", description).
		Str("category", suggestion.Category).
		Float64("confidence", suggestion.Confidence).
		Str("reasoning", suggestion.Reasoning).
		Msg("SuggestCategory: successfully matched category")

	return &suggestion, nil
}

// buildCategorySuggestionPrompt creates the prompt for category suggestion.
func buildCategorySuggestionPrompt(description string, categories []string) string {
	categoriesList := strings.Join(categories, "\n- ")

	return fmt.Sprintf(`Categorize this expense: "%s"

Available categories:
- %s

Rules:
- Choose the MOST appropriate category from the list
- "Food - Dining Out" for restaurant/takeout meals, "Food - Grocery" for ingredients
- "Transportation" for taxi, uber, grab, bus, train
- Higher confidence (0.8-1.0) for obvious categories, lower (0.5-0.7) for ambiguous ones

Return JSON only:
{"category": "exact category name", "confidence": 0.0-1.0, "reasoning": "brief explanation"}`, description, categoriesList)
}

// extractJSON extracts a JSON object from text that may contain preamble.
// Gemini sometimes returns responses like "Here is the JSON:\n{...}" even
// when ResponseMIMEType is set to application/json.
func extractJSON(text string) string {
	text = strings.TrimSpace(text)

	// If it already starts with {, assume it's valid JSON
	if strings.HasPrefix(text, "{") {
		return text
	}

	// Find the first { and last } to extract JSON object
	start := strings.Index(text, "{")
	if start == -1 {
		return ""
	}

	end := strings.LastIndex(text, "}")
	if end == -1 || end < start {
		return ""
	}

	return text[start : end+1]
}
