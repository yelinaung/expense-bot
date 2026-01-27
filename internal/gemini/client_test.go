package gemini

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		apiKey  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty API key returns error",
			apiKey:  "",
			wantErr: true,
			errMsg:  "API key is required",
		},
		{
			name:    "whitespace-only API key is treated as valid input",
			apiKey:  "   ",
			wantErr: false, // The SDK will validate, we just check non-empty.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			client, err := NewClient(ctx, tt.apiKey)

			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, client)
				require.Contains(t, err.Error(), tt.errMsg)
			} else {
				// With a non-empty key, NewClient should succeed.
				// The actual API validation happens on first request.
				require.NoError(t, err)
				require.NotNil(t, client)
			}
		})
	}
}

func TestClient_GenerativeClient(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client, err := NewClient(ctx, "test-api-key")
	require.NoError(t, err)
	require.NotNil(t, client)

	genClient := client.GenerativeClient()
	require.NotNil(t, genClient)
}
