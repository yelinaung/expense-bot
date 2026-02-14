package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Run("loads all config from env", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "test-token-123")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "123")

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
		t.Setenv("WHITELISTED_USER_IDS", "123")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, "test-gemini-key", cfg.GeminiAPIKey)
	})

	t.Run("loads exchange config from env", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "123")
		t.Setenv("EXCHANGE_RATE_BASE_URL", "https://rates.example.com")
		t.Setenv("EXCHANGE_RATE_TIMEOUT", "3s")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, "https://rates.example.com", cfg.ExchangeRateBaseURL)
		require.Equal(t, 3*time.Second, cfg.ExchangeRateTimeout)
	})

	t.Run("uses exchange defaults for invalid timeout", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "123")
		t.Setenv("EXCHANGE_RATE_TIMEOUT", "invalid")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, "https://api.frankfurter.app", cfg.ExchangeRateBaseURL)
		require.Equal(t, 5*time.Second, cfg.ExchangeRateTimeout)
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

func TestLoad_DailyReminder(t *testing.T) {
	t.Run("parses DAILY_REMINDER_ENABLED=true", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "123")
		t.Setenv("DAILY_REMINDER_ENABLED", "true")

		cfg, err := Load()
		require.NoError(t, err)
		require.True(t, cfg.DailyReminderEnabled)
	})

	t.Run("defaults DAILY_REMINDER_ENABLED to false", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "123")

		cfg, err := Load()
		require.NoError(t, err)
		require.False(t, cfg.DailyReminderEnabled)
	})

	t.Run("parses valid REMINDER_HOUR", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "123")
		t.Setenv("REMINDER_HOUR", "9")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, 9, cfg.ReminderHour)
	})

	t.Run("defaults REMINDER_HOUR to 20 for invalid value", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "123")
		t.Setenv("REMINDER_HOUR", "25")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, 20, cfg.ReminderHour)
	})

	t.Run("defaults REMINDER_HOUR to 20 for non-numeric value", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "123")
		t.Setenv("REMINDER_HOUR", "abc")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, 20, cfg.ReminderHour)
	})

	t.Run("parses REMINDER_TIMEZONE", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "123")
		t.Setenv("REMINDER_TIMEZONE", "America/New_York")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, "America/New_York", cfg.ReminderTimezone)
	})

	t.Run("defaults REMINDER_TIMEZONE to Asia/Singapore", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "123")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, "Asia/Singapore", cfg.ReminderTimezone)
	})

	t.Run("falls back to Asia/Singapore for invalid timezone", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "123")
		t.Setenv("REMINDER_TIMEZONE", "Invalid/Timezone")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, "Asia/Singapore", cfg.ReminderTimezone)
	})
}

func TestLoad_Validation(t *testing.T) {
	t.Run("fails when TELEGRAM_BOT_TOKEN is missing", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "123")

		_, err := Load()
		require.Error(t, err)
		require.Contains(t, err.Error(), "TELEGRAM_BOT_TOKEN is required")
	})

	t.Run("fails when DATABASE_URL is missing", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "")
		t.Setenv("WHITELISTED_USER_IDS", "123")

		_, err := Load()
		require.Error(t, err)
		require.Contains(t, err.Error(), "DATABASE_URL is required")
	})

	t.Run("fails when no whitelisted users", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "")
		t.Setenv("WHITELISTED_USERNAMES", "")

		_, err := Load()
		require.Error(t, err)
		require.Contains(t, err.Error(), "at least one whitelisted user")
	})

	t.Run("fails with multiple validation errors", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "")
		t.Setenv("DATABASE_URL", "")
		t.Setenv("WHITELISTED_USER_IDS", "")

		_, err := Load()
		require.Error(t, err)
		require.Contains(t, err.Error(), "TELEGRAM_BOT_TOKEN is required")
		require.Contains(t, err.Error(), "DATABASE_URL is required")
		require.Contains(t, err.Error(), "at least one whitelisted user")
	})

	t.Run("succeeds with username whitelist only", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "")
		t.Setenv("WHITELISTED_USERNAMES", "alice")

		cfg, err := Load()
		require.NoError(t, err)
		require.Empty(t, cfg.WhitelistedUserIDs)
		require.Equal(t, []string{"alice"}, cfg.WhitelistedUsernames)
	})

	t.Run("succeeds with user ID whitelist only", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "token")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("WHITELISTED_USER_IDS", "123")
		t.Setenv("WHITELISTED_USERNAMES", "")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []int64{123}, cfg.WhitelistedUserIDs)
		require.Empty(t, cfg.WhitelistedUsernames)
	})
}

func TestConfig_IsSuperAdmin(t *testing.T) {
	t.Parallel()

	t.Run("returns true for whitelisted user ID", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUserIDs:   []int64{100, 200},
			WhitelistedUsernames: []string{"admin"},
		}
		require.True(t, cfg.IsSuperAdmin(100, ""))
	})

	t.Run("returns true for whitelisted username and binds", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUserIDs:   []int64{100, 200},
			WhitelistedUsernames: []string{"admin"},
		}
		require.True(t, cfg.IsSuperAdmin(999, "admin"))
		require.True(t, cfg.IsSuperAdmin(999, ""), "bound user_id should work without username")
	})

	t.Run("returns false for non-whitelisted user", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUserIDs:   []int64{100, 200},
			WhitelistedUsernames: []string{"admin"},
		}
		require.False(t, cfg.IsSuperAdmin(999, "nobody"))
	})

	t.Run("recycled username rejected after binding", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{"admin"},
		}
		require.True(t, cfg.IsSuperAdmin(42, "admin"), "bootstrap should succeed")
		require.True(t, cfg.IsSuperAdmin(42, "admin"), "same user should still work")
		require.True(t, cfg.IsSuperAdmin(42, ""), "bound user_id alone should work")
		require.False(t, cfg.IsSuperAdmin(99, "admin"), "different user_id with recycled username must be rejected")
	})

	t.Run("userID 0 does not create binding", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{"admin"},
		}
		require.True(t, cfg.IsSuperAdmin(0, "admin"), "userID=0 should still return true for lookup")
		require.True(t, cfg.IsSuperAdmin(42, "admin"), "should still be able to bind after userID=0 call")
		require.False(t, cfg.IsSuperAdmin(99, "admin"), "recycled username rejected after real binding")
	})

	t.Run("userID 0 lookup returns true after binding", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{"admin"},
		}
		require.True(t, cfg.IsSuperAdmin(42, "admin"), "bootstrap binds")
		require.True(t, cfg.IsSuperAdmin(0, "admin"), "lookup-only call must still return true after binding")
		require.False(t, cfg.IsSuperAdmin(99, "admin"), "attacker still rejected")
	})
}

func TestConfig_IsUserWhitelisted(t *testing.T) {
	t.Parallel()

	t.Run("returns true for whitelisted user ID", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUserIDs:   []int64{100, 200, 300},
			WhitelistedUsernames: []string{"alice"},
		}
		require.True(t, cfg.IsUserWhitelisted(100, ""))
		require.True(t, cfg.IsUserWhitelisted(200, ""))
		require.True(t, cfg.IsUserWhitelisted(300, ""))
	})

	t.Run("returns true for whitelisted username bootstrap", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{"alice", "bob", "charlie"},
		}
		require.True(t, cfg.IsUserWhitelisted(999, "alice"))
		require.True(t, cfg.IsUserWhitelisted(888, "bob"))
		require.True(t, cfg.IsUserWhitelisted(777, "charlie"))
	})

	t.Run("returns true for whitelisted username with @ prefix", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{"alice", "bob"},
		}
		require.True(t, cfg.IsUserWhitelisted(999, "@alice"))
		require.True(t, cfg.IsUserWhitelisted(888, "@bob"))
	})

	t.Run("username check is case insensitive", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{"alice", "bob", "charlie"},
		}
		require.True(t, cfg.IsUserWhitelisted(999, "ALICE"))
		require.True(t, cfg.IsUserWhitelisted(888, "Bob"))
		require.True(t, cfg.IsUserWhitelisted(777, "ChArLiE"))
	})

	t.Run("returns false for non-whitelisted user", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUserIDs:   []int64{100},
			WhitelistedUsernames: []string{"alice"},
		}
		require.False(t, cfg.IsUserWhitelisted(999, "unknown"))
		require.False(t, cfg.IsUserWhitelisted(0, ""))
		require.False(t, cfg.IsUserWhitelisted(555, "notinlist"))
	})

	t.Run("returns false for empty whitelist", func(t *testing.T) {
		t.Parallel()
		emptyCfg := &Config{WhitelistedUserIDs: nil, WhitelistedUsernames: nil}
		require.False(t, emptyCfg.IsUserWhitelisted(100, "alice"))
	})

	t.Run("user ID match works even with non-whitelisted username", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUserIDs:   []int64{100},
			WhitelistedUsernames: []string{"alice"},
		}
		require.True(t, cfg.IsUserWhitelisted(100, "notinlist"))
	})

	t.Run("username binds on first use then enforces user_id", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{"alice"},
		}
		require.True(t, cfg.IsUserWhitelisted(999, "alice"))
		require.True(t, cfg.IsUserWhitelisted(999, "alice"))
		require.False(t, cfg.IsUserWhitelisted(888, "alice"), "different user_id must be rejected after binding")
	})
}

func TestConfig_SuperadminBound(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		WhitelistedUsernames: []string{"admin"},
	}

	_, bound := cfg.SuperadminBound("admin")
	require.False(t, bound, "should not be bound before first use")

	cfg.IsSuperAdmin(42, "admin")

	id, bound := cfg.SuperadminBound("admin")
	require.True(t, bound)
	require.Equal(t, int64(42), id)
}

func TestConfig_LoadSuperadminBindings(t *testing.T) {
	t.Parallel()

	t.Run("pre-loaded binding prevents recycled username", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{"admin"},
		}
		cfg.LoadSuperadminBindings([]SuperadminBinding{
			{Username: "admin", UserID: 42},
		})
		require.True(t, cfg.IsSuperAdmin(42, "admin"))
		require.True(t, cfg.IsSuperAdmin(42, ""))
		require.False(t, cfg.IsSuperAdmin(99, "admin"), "recycled username must be rejected")
	})

	t.Run("ignores bindings for non-whitelisted usernames", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{"admin"},
		}
		cfg.LoadSuperadminBindings([]SuperadminBinding{
			{Username: "removed_user", UserID: 55},
		})
		require.False(t, cfg.IsSuperAdmin(55, ""), "non-whitelisted binding should be ignored")
	})

	t.Run("survives restart simulation", func(t *testing.T) {
		t.Parallel()

		cfg1 := &Config{
			WhitelistedUsernames: []string{"admin"},
		}
		cfg1.IsSuperAdmin(42, "admin")

		id, bound := cfg1.SuperadminBound("admin")
		require.True(t, bound)

		cfg2 := &Config{
			WhitelistedUsernames: []string{"admin"},
		}
		cfg2.LoadSuperadminBindings([]SuperadminBinding{
			{Username: "admin", UserID: id},
		})
		require.True(t, cfg2.IsSuperAdmin(42, "admin"))
		require.False(t, cfg2.IsSuperAdmin(99, "admin"), "attacker must be rejected after restart+reload")
	})
}

func TestConfig_CheckSuperAdmin(t *testing.T) {
	t.Parallel()

	t.Run("returns new binding on first username match", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{"admin"},
		}
		ok, binding := cfg.CheckSuperAdmin(42, "admin")
		require.True(t, ok)
		require.NotNil(t, binding)
		require.Equal(t, int64(42), binding.UserID)
		require.Equal(t, "admin", binding.Username)
	})

	t.Run("returns nil binding on subsequent calls", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{"admin"},
		}
		cfg.CheckSuperAdmin(42, "admin")

		ok, binding := cfg.CheckSuperAdmin(42, "admin")
		require.True(t, ok)
		require.Nil(t, binding, "no new binding for already-bound username")
	})

	t.Run("returns nil binding for user ID match", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUserIDs: []int64{100},
		}
		ok, binding := cfg.CheckSuperAdmin(100, "")
		require.True(t, ok)
		require.Nil(t, binding)
	})

	t.Run("returns nil binding for pre-loaded binding", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{"admin"},
		}
		cfg.LoadSuperadminBindings([]SuperadminBinding{
			{Username: "admin", UserID: 42},
		})
		ok, binding := cfg.CheckSuperAdmin(42, "admin")
		require.True(t, ok)
		require.Nil(t, binding, "pre-loaded binding should not produce a new binding")
	})
}
