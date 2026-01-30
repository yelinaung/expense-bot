package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
	if c.generator == nil {
		return nil, fmt.Errorf("gemini client not initialized")
	}

	if description == "" {
		return nil, fmt.Errorf("description is required")
	}

	if len(availableCategories) == 0 {
		return nil, fmt.Errorf("no categories available")
	}

	prompt := buildCategorySuggestionPrompt(description, availableCategories)

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
		Temperature:      &temp, // Lower temperature for more consistent categorization
		MaxOutputTokens:  int32(200),
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
		return nil, fmt.Errorf("gemini API call failed: %w", err)
	}

	if resp == nil || len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response content")
	}

	// Extract JSON from response
	var jsonText string
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			jsonText = part.Text
			break
		}
	}

	if jsonText == "" {
		return nil, fmt.Errorf("no text content in response")
	}

	// Parse JSON response
	var suggestion CategorySuggestion
	if err := json.Unmarshal([]byte(jsonText), &suggestion); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

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
		return nil, fmt.Errorf("suggested category '%s' not in available categories", suggestion.Category)
	}

	return &suggestion, nil
}

// buildCategorySuggestionPrompt creates the prompt for category suggestion.
func buildCategorySuggestionPrompt(description string, categories []string) string {
	categoriesList := strings.Join(categories, "\n- ")

	return fmt.Sprintf(`You are a helpful expense categorization assistant. Analyze the expense description and suggest the most appropriate category from the provided list.

Expense Description: "%s"

Available Categories:
- %s

Instructions:
1. Choose the MOST appropriate category from the list above
2. Provide a confidence score (0-1) indicating how certain you are
3. Give a brief reasoning for your choice
4. Consider common expense patterns (e.g., "coffee" → Food/Dining, "taxi" → Transportation)
5. If the description is ambiguous, choose the most likely category and indicate lower confidence

Return your response as a JSON object with:
- category: exact category name from the list
- confidence: number between 0 and 1
- reasoning: brief explanation (1-2 sentences)`, description, categoriesList)
}
