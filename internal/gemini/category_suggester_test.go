package gemini

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
)

func TestSuggestCategory(t *testing.T) {
	t.Parallel()

	categories := []string{
		"Food - Dining Out",
		"Food - Groceries",
		"Transportation",
		"Entertainment",
		"Shopping",
		"Health & Fitness",
		"Utilities",
	}

	t.Run("suggests category for coffee", func(t *testing.T) {
		mockGen := &mockGenerator{
			response: createMockCategoryResponse("Food - Dining Out", 0.95, "Coffee is typically a dining out expense"),
		}
		client := NewClientWithGenerator(mockGen)

		suggestion, err := client.SuggestCategory(context.Background(), "coffee", categories)
		require.NoError(t, err)
		require.NotNil(t, suggestion)
		require.Equal(t, "Food - Dining Out", suggestion.Category)
		require.Greater(t, suggestion.Confidence, 0.9)
		require.NotEmpty(t, suggestion.Reasoning)
	})

	t.Run("suggests category for taxi", func(t *testing.T) {
		mockGen := &mockGenerator{
			response: createMockCategoryResponse("Transportation", 0.98, "Taxi is a transportation expense"),
		}
		client := NewClientWithGenerator(mockGen)

		suggestion, err := client.SuggestCategory(context.Background(), "taxi to airport", categories)
		require.NoError(t, err)
		require.NotNil(t, suggestion)
		require.Equal(t, "Transportation", suggestion.Category)
	})

	t.Run("suggests category for groceries", func(t *testing.T) {
		mockGen := &mockGenerator{
			response: createMockCategoryResponse("Food - Groceries", 0.92, "Supermarket shopping is typically groceries"),
		}
		client := NewClientWithGenerator(mockGen)

		suggestion, err := client.SuggestCategory(context.Background(), "supermarket", categories)
		require.NoError(t, err)
		require.NotNil(t, suggestion)
		require.Equal(t, "Food - Groceries", suggestion.Category)
	})

	t.Run("handles case-insensitive category matching", func(t *testing.T) {
		mockGen := &mockGenerator{
			response: createMockCategoryResponse("transportation", 0.95, "Uber is transportation"),
		}
		client := NewClientWithGenerator(mockGen)

		suggestion, err := client.SuggestCategory(context.Background(), "uber ride", categories)
		require.NoError(t, err)
		require.NotNil(t, suggestion)
		// Should match exact case from available categories
		require.Equal(t, "Transportation", suggestion.Category)
	})

	t.Run("returns error for empty description", func(t *testing.T) {
		client := NewClientWithGenerator(&mockGenerator{})

		suggestion, err := client.SuggestCategory(context.Background(), "", categories)
		require.Error(t, err)
		require.Nil(t, suggestion)
		require.Contains(t, err.Error(), "description is required")
	})

	t.Run("returns error for empty categories list", func(t *testing.T) {
		client := NewClientWithGenerator(&mockGenerator{})

		suggestion, err := client.SuggestCategory(context.Background(), "coffee", []string{})
		require.Error(t, err)
		require.Nil(t, suggestion)
		require.Contains(t, err.Error(), "no categories available")
	})

	t.Run("returns error for nil generator", func(t *testing.T) {
		client := &Client{generator: nil}

		suggestion, err := client.SuggestCategory(context.Background(), "coffee", categories)
		require.Error(t, err)
		require.Nil(t, suggestion)
		require.Contains(t, err.Error(), "not initialized")
	})

	t.Run("returns error when suggested category not in list", func(t *testing.T) {
		mockGen := &mockGenerator{
			response: createMockCategoryResponse("Invalid Category", 0.95, "This category doesn't exist"),
		}
		client := NewClientWithGenerator(mockGen)

		suggestion, err := client.SuggestCategory(context.Background(), "coffee", categories)
		require.Error(t, err)
		require.Nil(t, suggestion)
		require.Contains(t, err.Error(), "not in available categories")
	})

	t.Run("handles API errors gracefully", func(t *testing.T) {
		mockGen := &mockGenerator{
			err: fmt.Errorf("API error"),
		}
		client := NewClientWithGenerator(mockGen)

		suggestion, err := client.SuggestCategory(context.Background(), "coffee", categories)
		require.Error(t, err)
		require.Nil(t, suggestion)
	})

	t.Run("handles empty response", func(t *testing.T) {
		mockGen := &mockGenerator{
			response: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{},
			},
		}
		client := NewClientWithGenerator(mockGen)

		suggestion, err := client.SuggestCategory(context.Background(), "coffee", categories)
		require.Error(t, err)
		require.Nil(t, suggestion)
		require.Contains(t, err.Error(), "no text content")
	})
}

func TestBuildCategorySuggestionPrompt(t *testing.T) {
	t.Parallel()

	categories := []string{"Food", "Transportation", "Shopping"}

	t.Run("includes description in prompt", func(t *testing.T) {
		prompt := buildCategorySuggestionPrompt("coffee at Starbucks", categories)
		require.Contains(t, prompt, "coffee at Starbucks")
	})

	t.Run("includes all categories in prompt", func(t *testing.T) {
		prompt := buildCategorySuggestionPrompt("test", categories)
		require.Contains(t, prompt, "Food")
		require.Contains(t, prompt, "Transportation")
		require.Contains(t, prompt, "Shopping")
	})

	t.Run("includes instructions", func(t *testing.T) {
		prompt := buildCategorySuggestionPrompt("test", categories)
		require.Contains(t, prompt, "Categorize")
		require.Contains(t, prompt, "confidence")
		require.Contains(t, prompt, "reasoning")
		require.Contains(t, prompt, "JSON")
	})
}

// Helper function to create mock category response
func createMockCategoryResponse(category string, confidence float64, reasoning string) *genai.GenerateContentResponse {
	jsonResponse := `{
		"category": "` + category + `",
		"confidence": ` + formatFloat(confidence) + `,
		"reasoning": "` + reasoning + `"
	}`

	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Parts: []*genai.Part{
						{Text: jsonResponse},
					},
				},
			},
		},
	}
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%.2f", f)
}
