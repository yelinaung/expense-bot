package telemetry

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
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
	require.Positive(t, metricDataPointCount(rm, testTelegramHandlerCount))
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
	require.Positive(t, metricDataPointCount(rm, testTelegramHandlerCount))
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
	require.Positive(t, metricDataPointCount(rm, testTelegramHandlerCount))
}

// TestTracingMiddleware_SpanBehavior asserts the middleware emits
// low-cardinality span names. It swaps the package-level tracer for one
// backed by an in-memory span recorder, restoring it on cleanup, so it does
// not touch the global tracer provider (whose one-time delegation is relied
// on by TestTelegramTransport_SpanBehavior). It is intentionally not
// parallel: it mutates the package-level tracer var shared with the transport
// tests.
func TestTracingMiddleware_SpanBehavior(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	prevTracer := tracer
	tracer = tp.Tracer("expense-bot/telegram")
	t.Cleanup(func() {
		tracer = prevTracer
		_ = tp.Shutdown(context.Background())
	})

	known := []string{"/add", "/today"}
	// metrics is nil: only span behavior is asserted here.
	mw := TracingMiddleware(nil, known...)
	handler := mw(func(context.Context, *bot.Bot, *models.Update) {})

	run := func(text string) sdktrace.ReadOnlySpan {
		t.Helper()
		sr.Reset()
		update := &models.Update{
			Message: &models.Message{
				Text: text,
				Chat: models.Chat{ID: 1},
				From: &models.User{ID: 2},
			},
		}
		handler(context.Background(), nil, update)
		ended := sr.Ended()
		require.Len(t, ended, 1)
		return ended[0]
	}

	t.Run("known command produces a named server span", func(t *testing.T) {
		span := run("/add 10 coffee")
		require.Equal(t, "telegram.command /add", span.Name())
		require.Equal(t, trace.SpanKindServer, span.SpanKind())
	})

	t.Run("numeric command collapses to unknown", func(t *testing.T) {
		require.Equal(t, "telegram.command unknown", run("/1").Name())
	})

	t.Run("unknown command collapses to unknown", func(t *testing.T) {
		require.Equal(t, "telegram.command unknown", run("/nonexistent").Name())
	})

	t.Run("distinct attacker inputs share a single span name", func(t *testing.T) {
		names := make(map[string]struct{})
		for _, text := range []string{"/1", "/2", "/3", "/foo", "/bar"} {
			names[run(text).Name()] = struct{}{}
		}
		require.Len(t, names, 1)
		require.Contains(t, names, "telegram.command unknown")
	})

	t.Run("disallowed characters collapse to unknown", func(t *testing.T) {
		for _, text := range []string{"/héllo", "/foo bar", "/emoji😀"} {
			require.Equal(t, "telegram.command unknown", run(text).Name())
		}
	})

	t.Run("oversized command collapses to a bounded name", func(t *testing.T) {
		long := "/" + strings.Repeat("a", commandTokenMaxLength*4)
		span := run(long)
		require.Equal(t, "telegram.command unknown", span.Name())
	})

	t.Run("non command text classified as text span", func(t *testing.T) {
		require.Equal(t, "telegram.text", run("hello there").Name())
	})
}
