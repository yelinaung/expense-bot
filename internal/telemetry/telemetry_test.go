package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

func TestValidateEndpoint(t *testing.T) {
	t.Parallel()

	t.Run("accepts grpc host_port", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, validateEndpoint(ExporterOTLPGRPC, testLocalhost4317))
	})

	t.Run("rejects grpc endpoint with scheme", func(t *testing.T) {
		t.Parallel()
		err := validateEndpoint(ExporterOTLPGRPC, testHTTPLocalhost4317)
		require.Error(t, err)
	})

	t.Run("accepts http endpoint with scheme", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, validateEndpoint(ExporterOTLPHTTP, testHTTPLocalhost4318))
		require.NoError(t, validateEndpoint(ExporterOTLPHTTP, "https://collector:4318"))
	})

	t.Run("rejects http endpoint without scheme", func(t *testing.T) {
		t.Parallel()
		err := validateEndpoint(ExporterOTLPHTTP, testLocalhost4318)
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
	prevTP := otel.GetTracerProvider()
	prevMP := otel.GetMeterProvider()
	prevProp := otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetMeterProvider(prevMP)
		otel.SetTextMapPropagator(prevProp)
	})

	providers, err := Init(context.Background(), &Config{Enabled: false})
	require.NoError(t, err)
	require.NotNil(t, providers)
	require.NoError(t, providers.Shutdown(context.Background()))
}

func TestInitEnabled(t *testing.T) {
	prevTP := otel.GetTracerProvider()
	prevMP := otel.GetMeterProvider()
	prevProp := otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetMeterProvider(prevMP)
		otel.SetTextMapPropagator(prevProp)
	})

	t.Run("initializes stdout providers", func(t *testing.T) {
		providers, err := Init(context.Background(), &Config{
			Enabled:         true,
			ServiceName:     testExpenseBotService,
			ServiceVersion:  "test",
			Environment:     "test",
			ExporterType:    ExporterStdout,
			TraceSampleRate: 1.0,
		})
		require.NoError(t, err)
		require.NotNil(t, providers)
		require.NoError(t, providers.Shutdown(context.Background()))
	})

	t.Run("rejects invalid otlp grpc endpoint", func(t *testing.T) {
		providers, err := Init(context.Background(), &Config{
			Enabled:         true,
			ServiceName:     testExpenseBotService,
			ServiceVersion:  "test",
			Environment:     "test",
			ExporterType:    ExporterOTLPGRPC,
			Endpoint:        testHTTPLocalhost4317,
			TraceSampleRate: 1.0,
		})
		require.Error(t, err)
		require.Nil(t, providers)
	})

	t.Run("rejects invalid otlp http endpoint", func(t *testing.T) {
		providers, err := Init(context.Background(), &Config{
			Enabled:         true,
			ServiceName:     testExpenseBotService,
			ServiceVersion:  "test",
			Environment:     "test",
			ExporterType:    ExporterOTLPHTTP,
			Endpoint:        testLocalhost4318,
			TraceSampleRate: 1.0,
		})
		require.Error(t, err)
		require.Nil(t, providers)
	})

	t.Run("uses default endpoint for otlp grpc when endpoint is empty", func(t *testing.T) {
		providers, err := Init(context.Background(), &Config{
			Enabled:         true,
			ServiceName:     testExpenseBotService,
			ServiceVersion:  "test",
			Environment:     "test",
			ExporterType:    ExporterOTLPGRPC,
			Endpoint:        "",
			Insecure:        true,
			TraceSampleRate: 1.0,
		})
		require.NoError(t, err)
		require.NotNil(t, providers)
		_ = providers.Shutdown(context.Background())
	})

	t.Run("uses default endpoint for otlp http when endpoint is empty", func(t *testing.T) {
		providers, err := Init(context.Background(), &Config{
			Enabled:         true,
			ServiceName:     testExpenseBotService,
			ServiceVersion:  "test",
			Environment:     "test",
			ExporterType:    ExporterOTLPHTTP,
			Endpoint:        "",
			Insecure:        true,
			TraceSampleRate: 1.0,
		})
		require.NoError(t, err)
		require.NotNil(t, providers)
		_ = providers.Shutdown(context.Background())
	})
}

func TestNewTraceExporter(t *testing.T) {
	t.Parallel()

	t.Run("creates stdout exporter", func(t *testing.T) {
		t.Parallel()
		exp, err := newTraceExporter(context.Background(), ExporterStdout, "", false)
		require.NoError(t, err)
		require.NotNil(t, exp)
	})

	t.Run("returns error for unsupported exporter", func(t *testing.T) {
		t.Parallel()
		exp, err := newTraceExporter(context.Background(), "invalid", "", false)
		require.Error(t, err)
		require.Nil(t, exp)
	})

	t.Run("creates otlp grpc exporter", func(t *testing.T) {
		t.Parallel()
		exp, err := newTraceExporter(context.Background(), ExporterOTLPGRPC, testLocalhost4317, true)
		require.NoError(t, err)
		require.NotNil(t, exp)
	})

	t.Run("creates otlp http exporter", func(t *testing.T) {
		t.Parallel()
		exp, err := newTraceExporter(context.Background(), ExporterOTLPHTTP, testHTTPLocalhost4318, true)
		require.NoError(t, err)
		require.NotNil(t, exp)
	})
}

func TestNewMetricExporter(t *testing.T) {
	t.Parallel()

	t.Run("creates stdout exporter", func(t *testing.T) {
		t.Parallel()
		exp, err := newMetricExporter(context.Background(), ExporterStdout, "", false)
		require.NoError(t, err)
		require.NotNil(t, exp)
	})

	t.Run("returns error for unsupported exporter", func(t *testing.T) {
		t.Parallel()
		exp, err := newMetricExporter(context.Background(), "invalid", "", false)
		require.Error(t, err)
		require.Nil(t, exp)
	})

	t.Run("creates otlp grpc exporter", func(t *testing.T) {
		t.Parallel()
		exp, err := newMetricExporter(context.Background(), ExporterOTLPGRPC, testLocalhost4317, true)
		require.NoError(t, err)
		require.NotNil(t, exp)
	})

	t.Run("creates otlp http exporter", func(t *testing.T) {
		t.Parallel()
		exp, err := newMetricExporter(context.Background(), ExporterOTLPHTTP, testHTTPLocalhost4318, true)
		require.NoError(t, err)
		require.NotNil(t, exp)
	})
}
