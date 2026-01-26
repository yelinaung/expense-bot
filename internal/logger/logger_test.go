package logger

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestSetLevel(t *testing.T) {
	t.Run("sets debug level", func(t *testing.T) {
		SetLevel("debug")
		require.Equal(t, zerolog.DebugLevel, zerolog.GlobalLevel())
	})

	t.Run("sets info level", func(t *testing.T) {
		SetLevel("info")
		require.Equal(t, zerolog.InfoLevel, zerolog.GlobalLevel())
	})

	t.Run("sets warn level", func(t *testing.T) {
		SetLevel("warn")
		require.Equal(t, zerolog.WarnLevel, zerolog.GlobalLevel())
	})

	t.Run("sets error level", func(t *testing.T) {
		SetLevel("error")
		require.Equal(t, zerolog.ErrorLevel, zerolog.GlobalLevel())
	})

	t.Run("defaults to info for unknown level", func(t *testing.T) {
		SetLevel("unknown")
		require.Equal(t, zerolog.InfoLevel, zerolog.GlobalLevel())
	})

	// Reset to debug for other tests.
	SetLevel("debug")
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
