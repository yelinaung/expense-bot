// Package logger provides structured logging using zerolog.
package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
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
