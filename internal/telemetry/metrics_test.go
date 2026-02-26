package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestNewBotMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(meterProvider)
	defer otel.SetMeterProvider(noop.NewMeterProvider())
	defer func() {
		_ = meterProvider.Shutdown(context.Background())
	}()

	metrics, err := NewBotMetrics()
	require.NoError(t, err)
	require.NotNil(t, metrics)
}
