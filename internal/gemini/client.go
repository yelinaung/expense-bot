// Package gemini provides a client for the Google Gemini API.
package gemini

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// ModelName is the Gemini model to use for receipt OCR and categorization.
const ModelName = "gemini-2.5-flash"

// ContentGenerator defines the interface for generating content via Gemini.
// This abstraction enables testing without making actual API calls.
type ContentGenerator interface {
	GenerateContent(
		ctx context.Context,
		model string,
		contents []*genai.Content,
		config *genai.GenerateContentConfig,
	) (*genai.GenerateContentResponse, error)
}

// modelsAdapter wraps *genai.Models to implement ContentGenerator.
type modelsAdapter struct {
	models *genai.Models
}

func (m *modelsAdapter) GenerateContent(
	ctx context.Context,
	model string,
	contents []*genai.Content,
	config *genai.GenerateContentConfig,
) (*genai.GenerateContentResponse, error) {
	resp, err := m.models.GenerateContent(ctx, model, contents, config)
	if err != nil {
		return nil, fmt.Errorf("genai.GenerateContent: %w", err)
	}
	return resp, nil
}

// Client wraps the Gemini API client.
type Client struct {
	client    *genai.Client
	generator ContentGenerator
}

// NewClient creates a new Gemini client with the provided API key.
func NewClient(ctx context.Context, apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("gemini API key is required")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &Client{
		client:    client,
		generator: &modelsAdapter{models: client.Models},
	}, nil
}

// NewClientWithGenerator creates a Client with a custom ContentGenerator.
// This is primarily used for testing with mock generators.
func NewClientWithGenerator(generator ContentGenerator) *Client {
	return &Client{
		generator: generator,
	}
}

// GenerativeClient returns the underlying genai client for advanced usage.
func (c *Client) GenerativeClient() *genai.Client {
	return c.client
}
