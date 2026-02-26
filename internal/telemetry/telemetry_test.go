package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateEndpoint(t *testing.T) {
	t.Parallel()

	t.Run("accepts grpc host_port", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, validateEndpoint(ExporterOTLPGRPC, "localhost:4317"))
	})

	t.Run("rejects grpc endpoint with scheme", func(t *testing.T) {
		t.Parallel()
		err := validateEndpoint(ExporterOTLPGRPC, "http://localhost:4317")
		require.Error(t, err)
	})

	t.Run("accepts http endpoint with scheme", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, validateEndpoint(ExporterOTLPHTTP, "http://localhost:4318"))
		require.NoError(t, validateEndpoint(ExporterOTLPHTTP, "https://collector:4318"))
	})

	t.Run("rejects http endpoint without scheme", func(t *testing.T) {
		t.Parallel()
		err := validateEndpoint(ExporterOTLPHTTP, "localhost:4318")
		require.Error(t, err)
	})
}

func TestBuildSampler(t *testing.T) {
	t.Parallel()

	require.Equal(t, "AlwaysOnSampler", buildSampler(1).Description())
	require.Equal(t, "AlwaysOffSampler", buildSampler(0).Description())
	require.Equal(t, "AlwaysOffSampler", buildSampler(-1).Description())
	require.Contains(t, buildSampler(0.5).Description(), "TraceIDRatioBased")
}

func TestInitDisabledReturnsNoopProviders(t *testing.T) {
	t.Parallel()

	providers, err := Init(context.Background(), &Config{Enabled: false})
	require.NoError(t, err)
	require.NotNil(t, providers)
	require.NoError(t, providers.Shutdown(context.Background()))
}
