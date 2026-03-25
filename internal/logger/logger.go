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

// Level represents a log level.
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// SetLevel sets the global log level.
func SetLevel(level Level) {
	switch level {
	case LevelDebug:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case LevelInfo:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case LevelWarn:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case LevelError:
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
