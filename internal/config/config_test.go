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

	t.Run("parses whitelisted usernames", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USERNAMES", "alice,bob,charlie")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []string{"alice", "bob", "charlie"}, cfg.WhitelistedUsernames)
	})

	t.Run("handles whitespace in usernames", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USERNAMES", " alice , bob , charlie ")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []string{"alice", "bob", "charlie"}, cfg.WhitelistedUsernames)
	})

	t.Run("strips @ prefix from usernames", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USERNAMES", "@alice,@bob,charlie")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []string{"alice", "bob", "charlie"}, cfg.WhitelistedUsernames)
	})

	t.Run("handles empty username whitelist", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USERNAMES", "")

		cfg, err := Load()
		require.NoError(t, err)
		require.Empty(t, cfg.WhitelistedUsernames)
	})

	t.Run("loads both user IDs and usernames", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "123,456")
		t.Setenv("WHITELISTED_USERNAMES", "alice,bob")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []int64{123, 456}, cfg.WhitelistedUserIDs)
		require.Equal(t, []string{"alice", "bob"}, cfg.WhitelistedUsernames)
	})
}

func TestConfig_IsUserWhitelisted(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		WhitelistedUserIDs:   []int64{100, 200, 300},
		WhitelistedUsernames: []string{"alice", "bob", "charlie"},
	}

	t.Run("returns true for whitelisted user ID", func(t *testing.T) {
		t.Parallel()
		require.True(t, cfg.IsUserWhitelisted(100, ""))
		require.True(t, cfg.IsUserWhitelisted(200, ""))
		require.True(t, cfg.IsUserWhitelisted(300, ""))
	})

	t.Run("returns true for whitelisted username", func(t *testing.T) {
		t.Parallel()
		require.True(t, cfg.IsUserWhitelisted(999, "alice"))
		require.True(t, cfg.IsUserWhitelisted(888, "bob"))
		require.True(t, cfg.IsUserWhitelisted(777, "charlie"))
	})

	t.Run("returns true for whitelisted username with @ prefix", func(t *testing.T) {
		t.Parallel()
		require.True(t, cfg.IsUserWhitelisted(999, "@alice"))
		require.True(t, cfg.IsUserWhitelisted(888, "@bob"))
	})

	t.Run("username check is case insensitive", func(t *testing.T) {
		t.Parallel()
		require.True(t, cfg.IsUserWhitelisted(999, "ALICE"))
		require.True(t, cfg.IsUserWhitelisted(888, "Bob"))
		require.True(t, cfg.IsUserWhitelisted(777, "ChArLiE"))
	})

	t.Run("returns false for non-whitelisted user", func(t *testing.T) {
		t.Parallel()
		require.False(t, cfg.IsUserWhitelisted(999, "unknown"))
		require.False(t, cfg.IsUserWhitelisted(0, ""))
		require.False(t, cfg.IsUserWhitelisted(555, "notinlist"))
	})

	t.Run("returns false for empty whitelist", func(t *testing.T) {
		t.Parallel()
		emptyCfg := &Config{WhitelistedUserIDs: nil, WhitelistedUsernames: nil}
		require.False(t, emptyCfg.IsUserWhitelisted(100, "alice"))
	})

	t.Run("returns true if either user ID or username is whitelisted", func(t *testing.T) {
		t.Parallel()
		// User ID matches but username doesn't
		require.True(t, cfg.IsUserWhitelisted(100, "notinlist"))
		// Username matches but user ID doesn't
		require.True(t, cfg.IsUserWhitelisted(999, "alice"))
		// Both match
		require.True(t, cfg.IsUserWhitelisted(100, "alice"))
	})
}
