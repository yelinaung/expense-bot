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
	tests := []struct {
		name  string
		level Level
		want  zerolog.Level
	}{
		{"debug", LevelDebug, zerolog.DebugLevel},
		{"info", LevelInfo, zerolog.InfoLevel},
		{"warn", LevelWarn, zerolog.WarnLevel},
		{"error", LevelError, zerolog.ErrorLevel},
		{"unknown defaults to info", "unknown", zerolog.InfoLevel},
		{"zero value defaults to info", Level(""), zerolog.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLevel(tt.level)
			require.Equal(t, tt.want, zerolog.GlobalLevel())
		})
	}

	// Reset to debug for other tests.
	SetLevel(LevelDebug)
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    Level
		wantErr bool
	}{
		{"debug", "debug", LevelDebug, false},
		{"info", "info", LevelInfo, false},
		{"warn", "warn", LevelWarn, false},
		{"error", "error", LevelError, false},
		{"empty defaults to info", "", LevelInfo, false},
		{"uppercase normalized", "DEBUG", LevelDebug, false},
		{"mixed case normalized", "Warn", LevelWarn, false},
		{"whitespace trimmed", "  error  ", LevelError, false},
		{"unknown returns error", "trace", LevelInfo, true},
		{"typo returns error", "inf", LevelInfo, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLevel(tt.raw)
			require.Equal(t, tt.want, got)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
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
