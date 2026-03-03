package telemetry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func metricDataPointCount(resourceMetrics metricdata.ResourceMetrics, metricName string) int {
	for i := range resourceMetrics.ScopeMetrics {
		for j := range resourceMetrics.ScopeMetrics[i].Metrics {
			metric := resourceMetrics.ScopeMetrics[i].Metrics[j]
			if metric.Name != metricName {
				continue
			}
			switch data := metric.Data.(type) {
			case metricdata.Sum[int64]:
				return len(data.DataPoints)
			case metricdata.Histogram[float64]:
				return len(data.DataPoints)
			}
		}
	}
	return 0
}

func TestTracingMiddlewareRecordsMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(meterProvider)
	defer otel.SetMeterProvider(noop.NewMeterProvider())
	defer func() {
		_ = meterProvider.Shutdown(context.Background())
	}()

	metrics, err := NewBotMetrics()
	require.NoError(t, err)

	mw := TracingMiddleware(metrics)
	called := false
	handler := mw(func(ctx context.Context, _ *bot.Bot, _ *models.Update) {
		called = true
		require.NotNil(t, ctx)
	})

	update := &models.Update{
		Message: &models.Message{
			Text: "/today",
			Chat: models.Chat{ID: 101},
			From: &models.User{ID: 202},
		},
	}
	handler(context.Background(), nil, update)
	require.True(t, called)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	require.Positive(t, metricDataPointCount(rm, "telegram.handler.count"))
	require.Positive(t, metricDataPointCount(rm, "telegram.handler.duration"))
}

func TestRecordHandlerMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(meterProvider)
	defer otel.SetMeterProvider(noop.NewMeterProvider())
	defer func() {
		_ = meterProvider.Shutdown(context.Background())
	}()

	metrics, err := NewBotMetrics()
	require.NoError(t, err)

	recordHandlerMetrics(context.Background(), metrics, "telegram.command /add", "ok", time.Now().Add(-time.Second))

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	require.Positive(t, metricDataPointCount(rm, "telegram.handler.count"))
	require.Positive(t, metricDataPointCount(rm, "telegram.handler.duration"))
}

func TestTracingMiddlewarePanicPathRecordsMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(meterProvider)
	defer otel.SetMeterProvider(noop.NewMeterProvider())
	defer func() {
		_ = meterProvider.Shutdown(context.Background())
	}()

	metrics, err := NewBotMetrics()
	require.NoError(t, err)

	mw := TracingMiddleware(metrics)
	handler := mw(func(context.Context, *bot.Bot, *models.Update) {
		panic(errors.New("boom"))
	})

	update := &models.Update{Message: &models.Message{Text: "/add", Chat: models.Chat{ID: 1}, From: &models.User{ID: 2}}}

	require.Panics(t, func() {
		handler(context.Background(), nil, update)
	})

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	require.Positive(t, metricDataPointCount(rm, "telegram.handler.count"))
}
