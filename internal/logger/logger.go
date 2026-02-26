// Package logger provides structured logging using zerolog.
package logger

import (
	"context"
	"os"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
)

// Log is the global logger instance.
var Log zerolog.Logger

func init() {
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}

	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	Log = zerolog.New(output).
		With().
		Timestamp().
		Caller().
		Logger()
}

// SetLevel sets the global log level.
func SetLevel(level string) {
	switch level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

// SetJSON switches to JSON output (for production).
func SetJSON() {
	Log = zerolog.New(os.Stdout).
		With().
		Timestamp().
		Logger()
}

// WithTraceContext returns a logger enriched with trace_id and span_id from
// the active span in ctx. If there is no active span, the base Log is returned.
func WithTraceContext(ctx context.Context) zerolog.Logger {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return Log
	}
	return Log.With().
		Str("trace_id", sc.TraceID().String()).
		Str("span_id", sc.SpanID().String()).
		Logger()
}
