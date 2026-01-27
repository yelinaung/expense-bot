// Package gemini provides a client for the Google Gemini API.
package gemini

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// ModelName is the Gemini model to use for receipt OCR.
const ModelName = "gemini-3-flash-preview"

// Client wraps the Gemini API client.
type Client struct {
	client *genai.Client
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

	return &Client{client: client}, nil
}

// GenerativeClient returns the underlying genai client for advanced usage.
func (c *Client) GenerativeClient() *genai.Client {
	return c.client
}
