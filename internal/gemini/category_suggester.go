package gemini

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gitlab.com/yelinaung/expense-bot/internal/logger"
	"google.golang.org/genai"
)

// MaxDescriptionLength is the maximum allowed length for expense descriptions.
const MaxDescriptionLength = 200

// MaxCategoryNameLength is the maximum allowed length for category names.
const MaxCategoryNameLength = 50

// CategorySuggestion represents a suggested category for an expense description.
type CategorySuggestion struct {
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
}

// SuggestCategory uses Gemini to suggest an appropriate category for an expense description.
func (c *Client) SuggestCategory(ctx context.Context, description string, availableCategories []string) (*CategorySuggestion, error) {
	descHash := hashDescription(description)
	logger.Log.Debug().
		Str("description_hash", descHash).
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

	// Sanitize description to prevent prompt injection attacks.
	sanitizedDescription := sanitizeDescription(description)

	prompt := buildCategorySuggestionPrompt(sanitizedDescription, availableCategories)
	logger.Log.Debug().
		Str("description_hash", descHash).
		Int("category_count", len(availableCategories)).
		Msg("SuggestCategory: sending prompt to Gemini")

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
					Enum:        availableCategories, // Restrict to allowed values.
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
			Str("description_hash", descHash).
			Msg("SuggestCategory: Gemini API call failed")
		return nil, fmt.Errorf("gemini API call failed: %w", err)
	}

	if resp == nil {
		logger.Log.Warn().
			Str("description_hash", descHash).
			Msg("SuggestCategory: nil response from Gemini")
		return nil, fmt.Errorf("no response from Gemini")
	}

	// Use the built-in Text() method to get concatenated text from all parts.
	fullText := resp.Text()

	logger.Log.Debug().
		Str("description_hash", descHash).
		Msg("SuggestCategory: received Gemini response")

	if fullText == "" {
		logger.Log.Warn().
			Str("description_hash", descHash).
			Msg("SuggestCategory: no text content in Gemini response")
		return nil, fmt.Errorf("no text content in response")
	}

	// Extract JSON from response - Gemini sometimes includes preamble text.
	jsonText := extractJSON(fullText)
	if jsonText == "" {
		logger.Log.Warn().
			Str("description_hash", descHash).
			Msg("SuggestCategory: no JSON found in Gemini response")
		return nil, fmt.Errorf("no JSON found in response")
	}

	// Parse JSON response.
	var suggestion CategorySuggestion
	if err := json.Unmarshal([]byte(jsonText), &suggestion); err != nil {
		logger.Log.Error().Err(err).
			Str("description_hash", descHash).
			Msg("SuggestCategory: failed to parse JSON response")
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	logger.Log.Debug().
		Str("description_hash", descHash).
		Str("suggested_category", suggestion.Category).
		Float64("confidence", suggestion.Confidence).
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
			Str("description_hash", descHash).
			Str("suggested_category", suggestion.Category).
			Strs("available_categories", availableCategories).
			Msg("SuggestCategory: suggested category not in available list")
		return nil, fmt.Errorf("suggested category '%s' not in available categories", suggestion.Category)
	}

	// Validate confidence range.
	if suggestion.Confidence < 0.0 || suggestion.Confidence > 1.0 {
		logger.Log.Warn().
			Float64("confidence", suggestion.Confidence).
			Msg("SuggestCategory: confidence out of valid range")
		return nil, fmt.Errorf("confidence out of range: %f", suggestion.Confidence)
	}

	// Sanitize reasoning field before returning.
	suggestion.Reasoning = sanitizeReasoning(suggestion.Reasoning)

	logger.Log.Debug().
		Str("description_hash", descHash).
		Str("category", suggestion.Category).
		Float64("confidence", suggestion.Confidence).
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

	// Find the first { and last } to extract JSON object.
	start := strings.Index(text, "{")
	if start == -1 {
		return ""
	}

	end := strings.LastIndex(text, "}")
	if end == -1 || end <= start {
		return ""
	}

	return text[start : end+1]
}

// SanitizeForPrompt sanitizes user input to prevent prompt injection attacks.
// It removes or escapes characters that could break prompt structure,
// and truncates to the given maxLength.
func SanitizeForPrompt(input string, maxLength int) string {
	// Remove or escape quotes that could break prompt structure.
	input = strings.ReplaceAll(input, `"`, `'`)
	input = strings.ReplaceAll(input, "`", "'")

	// Remove null bytes and other control characters.
	input = strings.ReplaceAll(input, "\x00", "")

	// Normalize whitespace: splits on any whitespace (spaces, tabs, newlines)
	// and rejoins with single spaces. This handles newline injection and
	// collapses multiple spaces in one efficient operation.
	input = strings.Join(strings.Fields(input), " ")

	// Limit length to prevent prompt stuffing attacks.
	// Trim after truncation to avoid trailing whitespace from mid-word cuts.
	if len(input) > maxLength {
		input = strings.TrimSpace(input[:maxLength])
	}

	return input
}

// SanitizeCategoryName sanitizes a category name for safe embedding in prompts.
// It applies the same rules as SanitizeForPrompt but with a shorter length limit
// appropriate for category names.
func SanitizeCategoryName(name string) string {
	return SanitizeForPrompt(name, MaxCategoryNameLength)
}

// sanitizeDescription sanitizes user input to prevent prompt injection attacks.
// It removes or escapes characters that could break prompt structure.
func sanitizeDescription(description string) string {
	return SanitizeForPrompt(description, MaxDescriptionLength)
}

// sanitizeReasoning sanitizes the reasoning field from LLM response.
// This prevents potentially malicious content from being persisted or displayed.
func sanitizeReasoning(reasoning string) string {
	// Normalize whitespace: handles newlines, tabs, and collapses multiple spaces.
	reasoning = strings.Join(strings.Fields(reasoning), " ")

	// Limit length and trim any trailing whitespace from truncation.
	const maxReasoningLength = 500
	if len(reasoning) > maxReasoningLength {
		reasoning = strings.TrimSpace(reasoning[:maxReasoningLength])
	}

	return reasoning
}

// hashDescription creates a SHA256 hash of the description for secure logging.
func hashDescription(description string) string {
	hash := sha256.Sum256([]byte(description))
	return hex.EncodeToString(hash[:8]) // First 8 bytes for brevity.
}
