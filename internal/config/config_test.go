package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Run("loads telegram token from env", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "test-token-123")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, "test-token-123", cfg.TelegramBotToken)
		require.Equal(t, "postgres://localhost/test", cfg.DatabaseURL)
	})

	t.Run("parses whitelisted user IDs", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "123,456,789")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []int64{123, 456, 789}, cfg.WhitelistedUserIDs)
	})

	t.Run("handles whitespace in user IDs", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", " 123 , 456 , 789 ")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []int64{123, 456, 789}, cfg.WhitelistedUserIDs)
	})

	t.Run("skips invalid user IDs", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "123,invalid,456")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []int64{123, 456}, cfg.WhitelistedUserIDs)
	})

	t.Run("handles empty whitelist", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "")

		cfg, err := Load()
		require.NoError(t, err)
		require.Empty(t, cfg.WhitelistedUserIDs)
	})

	t.Run("skips empty entries from trailing commas", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "123,,456,")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []int64{123, 456}, cfg.WhitelistedUserIDs)
	})

	t.Run("loads GeminiAPIKey from env", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("GEMINI_API_KEY", "test-gemini-key")
		t.Setenv("WHITELISTED_USER_IDS", "")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, "test-gemini-key", cfg.GeminiAPIKey)
	})
}

func TestConfig_IsUserWhitelisted(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		WhitelistedUserIDs: []int64{100, 200, 300},
	}

	t.Run("returns true for whitelisted user", func(t *testing.T) {
		t.Parallel()
		require.True(t, cfg.IsUserWhitelisted(100))
		require.True(t, cfg.IsUserWhitelisted(200))
		require.True(t, cfg.IsUserWhitelisted(300))
	})

	t.Run("returns false for non-whitelisted user", func(t *testing.T) {
		t.Parallel()
		require.False(t, cfg.IsUserWhitelisted(999))
		require.False(t, cfg.IsUserWhitelisted(0))
	})

	t.Run("returns false for empty whitelist", func(t *testing.T) {
		t.Parallel()
		emptyCfg := &Config{WhitelistedUserIDs: nil}
		require.False(t, emptyCfg.IsUserWhitelisted(100))
	})
}
