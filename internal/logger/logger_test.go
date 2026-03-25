package logger

import (
	"bytes"
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestSetLevel(t *testing.T) {
	t.Run("sets debug level", func(t *testing.T) {
		SetLevel(LevelDebug)
		require.Equal(t, zerolog.DebugLevel, zerolog.GlobalLevel())
	})

	t.Run("sets info level", func(t *testing.T) {
		SetLevel(LevelInfo)
		require.Equal(t, zerolog.InfoLevel, zerolog.GlobalLevel())
	})

	t.Run("sets warn level", func(t *testing.T) {
		SetLevel(LevelWarn)
		require.Equal(t, zerolog.WarnLevel, zerolog.GlobalLevel())
	})

	t.Run("sets error level", func(t *testing.T) {
		SetLevel(LevelError)
		require.Equal(t, zerolog.ErrorLevel, zerolog.GlobalLevel())
	})

	t.Run("defaults to info for unknown level", func(t *testing.T) {
		SetLevel("unknown")
		require.Equal(t, zerolog.InfoLevel, zerolog.GlobalLevel())
	})

	// Reset to debug for other tests.
	SetLevel(LevelDebug)
}

func TestSetJSON(t *testing.T) {
	t.Run("switches to JSON output", func(t *testing.T) {
		SetJSON()
		require.NotNil(t, Log)
	})
}

func TestLoggerInit(t *testing.T) {
	t.Run("logger is initialized", func(t *testing.T) {
		require.NotNil(t, Log)
	})

	t.Run("can log info message", func(t *testing.T) {
		Log.Info().Msg("test message")
	})

	t.Run("can log with fields", func(t *testing.T) {
		Log.Info().
			Str("key", "value").
			Int("count", 42).
			Msg("test with fields")
	})
}

func TestWithTraceContext(t *testing.T) {
	t.Run("returns base logger when no active span", func(t *testing.T) {
		var buf bytes.Buffer
		originalLog := Log
		Log = zerolog.New(&buf)
		t.Cleanup(func() {
			Log = originalLog
		})

		l := WithTraceContext(context.Background())
		l.Info().Msg("no span")

		output := buf.String()
		require.NotContains(t, output, `"trace_id"`)
		require.NotContains(t, output, `"span_id"`)
	})

	t.Run("adds trace and span fields when span is active", func(t *testing.T) {
		var buf bytes.Buffer
		originalLog := Log
		Log = zerolog.New(&buf)
		t.Cleanup(func() {
			Log = originalLog
		})

		traceProvider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
		otel.SetTracerProvider(traceProvider)
		t.Cleanup(func() {
			otel.SetTracerProvider(noop.NewTracerProvider())
			_ = traceProvider.Shutdown(context.Background())
		})

		ctx, span := traceProvider.Tracer("logger-test").Start(context.Background(), "span")
		defer span.End()

		l := WithTraceContext(ctx)
		l.Info().Msg("with span")

		output := buf.String()
		require.Contains(t, output, `"trace_id"`)
		require.Contains(t, output, `"span_id"`)
	})
}
