package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	testDatabaseURLConfig                    = "postgres://localhost/test"
	testTokenConfig                          = "token"
	envTelegramKeyVarConfig                  = "TELEGRAM_BOT_TOKEN"
	envDatabaseURL                           = "DATABASE_URL"
	envWhitelistedUserIDs                    = "WHITELISTED_USER_IDS"
	envWhitelistedUsernames                  = "WHITELISTED_USERNAMES"
	envReminderHour                          = "REMINDER_HOUR"
	envReminderTimezone                      = "REMINDER_TIMEZONE"
	adminUsernameConfigTest                  = "admin"
	aliceUsernameConfigTest                  = "alice"
	charlieUsernameConfigTest                = "charlie"
	tzAmericaNewYork                         = "America/New_York"
	tzAsiaSingapore                          = "Asia/Singapore"
	errTelegramKeyRequiredConfig             = "TELEGRAM_BOT_TOKEN is required"
	errDatabaseURLRequired                   = "DATABASE_URL is required"
	errAtLeastOneWhitelisted                 = "at least one whitelisted user"
	testToken123Config                       = "test-token-123"
	testGeminiKeyConfig                      = "test-gemini-key"
	exchangeRateBaseURLConfig                = "https://rates.example.com"
	envExchangeRateTimeout                   = "EXCHANGE_RATE_TIMEOUT"
	notInListUsernameConfig                  = "notinlist"
	returnsTrueWhitelistedUserIDConfigTest   = "returns true for whitelisted user ID"
	returnsFalseNonWhitelistedUserConfigTest = "returns false for non-whitelisted user"
)

func TestLoad(t *testing.T) {
	t.Run("loads all config from env", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testToken123Config)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, "123")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, testToken123Config, cfg.TelegramBotToken)
		require.Equal(t, testDatabaseURLConfig, cfg.DatabaseURL)
	})

	t.Run("parses whitelisted user IDs", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, "123,456,789")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []int64{123, 456, 789}, cfg.WhitelistedUserIDs)
	})

	t.Run("handles whitespace in user IDs", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, " 123 , 456 , 789 ")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []int64{123, 456, 789}, cfg.WhitelistedUserIDs)
	})

	t.Run("skips invalid user IDs", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, "123,invalid,456")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []int64{123, 456}, cfg.WhitelistedUserIDs)
	})

	t.Run("skips empty entries from trailing commas", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, "123,,456,")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []int64{123, 456}, cfg.WhitelistedUserIDs)
	})

	t.Run("loads GeminiAPIKey from env", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv("GEMINI_API_KEY", testGeminiKeyConfig)
		t.Setenv(envWhitelistedUserIDs, "123")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, testGeminiKeyConfig, cfg.GeminiAPIKey)
	})

	t.Run("loads exchange config from env", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, "123")
		t.Setenv("EXCHANGE_RATE_BASE_URL", exchangeRateBaseURLConfig)
		t.Setenv(envExchangeRateTimeout, "3s")
		t.Setenv("EXCHANGE_RATE_CACHE_TTL", "1h")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, exchangeRateBaseURLConfig, cfg.ExchangeRateBaseURL)
		require.Equal(t, 3*time.Second, cfg.ExchangeRateTimeout)
		require.Equal(t, time.Hour, cfg.ExchangeRateCacheTTL)
	})

	t.Run("uses exchange defaults for invalid timeout", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, "123")
		t.Setenv(envExchangeRateTimeout, "invalid")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, "https://api.frankfurter.app", cfg.ExchangeRateBaseURL)
		require.Equal(t, 5*time.Second, cfg.ExchangeRateTimeout)
		require.Equal(t, 12*time.Hour, cfg.ExchangeRateCacheTTL)
	})

	t.Run("parses whitelisted usernames", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUsernames, "alice,bob,charlie")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []string{aliceUsernameConfigTest, "bob", charlieUsernameConfigTest}, cfg.WhitelistedUsernames)
	})

	t.Run("handles whitespace in usernames", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUsernames, " alice , bob , charlie ")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []string{aliceUsernameConfigTest, "bob", charlieUsernameConfigTest}, cfg.WhitelistedUsernames)
	})

	t.Run("strips @ prefix from usernames", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUsernames, "@alice,@bob,charlie")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []string{aliceUsernameConfigTest, "bob", charlieUsernameConfigTest}, cfg.WhitelistedUsernames)
	})

	t.Run("loads both user IDs and usernames", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, "123,456")
		t.Setenv(envWhitelistedUsernames, "alice,bob")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []int64{123, 456}, cfg.WhitelistedUserIDs)
		require.Equal(t, []string{aliceUsernameConfigTest, "bob"}, cfg.WhitelistedUsernames)
	})
}

func TestLoad_DailyReminder(t *testing.T) {
	t.Run("parses DAILY_REMINDER_ENABLED=true", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, "123")
		t.Setenv("DAILY_REMINDER_ENABLED", "true")

		cfg, err := Load()
		require.NoError(t, err)
		require.True(t, cfg.DailyReminderEnabled)
	})

	t.Run("defaults DAILY_REMINDER_ENABLED to false", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, "123")

		cfg, err := Load()
		require.NoError(t, err)
		require.False(t, cfg.DailyReminderEnabled)
	})

	t.Run("parses valid REMINDER_HOUR", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, "123")
		t.Setenv(envReminderHour, "9")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, 9, cfg.ReminderHour)
	})

	t.Run("defaults REMINDER_HOUR to 20 for invalid value", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, "123")
		t.Setenv(envReminderHour, "25")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, 20, cfg.ReminderHour)
	})

	t.Run("defaults REMINDER_HOUR to 20 for non-numeric value", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, "123")
		t.Setenv(envReminderHour, "abc")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, 20, cfg.ReminderHour)
	})

	t.Run("parses REMINDER_TIMEZONE", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, "123")
		t.Setenv(envReminderTimezone, tzAmericaNewYork)

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, tzAmericaNewYork, cfg.ReminderTimezone)
	})

	t.Run("defaults REMINDER_TIMEZONE to Asia/Singapore", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, "123")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, tzAsiaSingapore, cfg.ReminderTimezone)
	})

	t.Run("falls back to Asia/Singapore for invalid timezone", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, "123")
		t.Setenv(envReminderTimezone, "Invalid/Timezone")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, tzAsiaSingapore, cfg.ReminderTimezone)
	})
}

func TestLoad_Validation(t *testing.T) {
	t.Run("fails when TELEGRAM_BOT_TOKEN is missing", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, "")
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, "123")

		_, err := Load()
		require.Error(t, err)
		require.Contains(t, err.Error(), errTelegramKeyRequiredConfig)
	})

	t.Run("fails when DATABASE_URL is missing", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, "")
		t.Setenv(envWhitelistedUserIDs, "123")

		_, err := Load()
		require.Error(t, err)
		require.Contains(t, err.Error(), errDatabaseURLRequired)
	})

	t.Run("fails when no whitelisted users", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, "")
		t.Setenv(envWhitelistedUsernames, "")

		_, err := Load()
		require.Error(t, err)
		require.Contains(t, err.Error(), errAtLeastOneWhitelisted)
	})

	t.Run("fails with multiple validation errors", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, "")
		t.Setenv(envDatabaseURL, "")
		t.Setenv(envWhitelistedUserIDs, "")

		_, err := Load()
		require.Error(t, err)
		require.Contains(t, err.Error(), errTelegramKeyRequiredConfig)
		require.Contains(t, err.Error(), errDatabaseURLRequired)
		require.Contains(t, err.Error(), errAtLeastOneWhitelisted)
	})

	t.Run("succeeds with username whitelist only", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, "")
		t.Setenv(envWhitelistedUsernames, aliceUsernameConfigTest)

		cfg, err := Load()
		require.NoError(t, err)
		require.Empty(t, cfg.WhitelistedUserIDs)
		require.Equal(t, []string{aliceUsernameConfigTest}, cfg.WhitelistedUsernames)
	})

	t.Run("succeeds with user ID whitelist only", func(t *testing.T) {
		t.Setenv(envTelegramKeyVarConfig, testTokenConfig)
		t.Setenv(envDatabaseURL, testDatabaseURLConfig)
		t.Setenv(envWhitelistedUserIDs, "123")
		t.Setenv(envWhitelistedUsernames, "")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []int64{123}, cfg.WhitelistedUserIDs)
		require.Empty(t, cfg.WhitelistedUsernames)
	})
}

func TestConfig_IsSuperAdmin(t *testing.T) {
	t.Parallel()

	t.Run(returnsTrueWhitelistedUserIDConfigTest, func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUserIDs:   []int64{100, 200},
			WhitelistedUsernames: []string{adminUsernameConfigTest},
		}
		require.True(t, cfg.IsSuperAdmin(100, ""))
	})

	t.Run("returns true for whitelisted username and binds", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUserIDs:   []int64{100, 200},
			WhitelistedUsernames: []string{adminUsernameConfigTest},
		}
		require.True(t, cfg.IsSuperAdmin(999, adminUsernameConfigTest))
		require.True(t, cfg.IsSuperAdmin(999, ""), "bound user_id should work without username")
	})

	t.Run(returnsFalseNonWhitelistedUserConfigTest, func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUserIDs:   []int64{100, 200},
			WhitelistedUsernames: []string{adminUsernameConfigTest},
		}
		require.False(t, cfg.IsSuperAdmin(999, "nobody"))
	})

	t.Run("recycled username rejected after binding", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{adminUsernameConfigTest},
		}
		require.True(t, cfg.IsSuperAdmin(42, adminUsernameConfigTest), "bootstrap should succeed")
		require.True(t, cfg.IsSuperAdmin(42, adminUsernameConfigTest), "same user should still work")
		require.True(t, cfg.IsSuperAdmin(42, ""), "bound user_id alone should work")
		require.False(t, cfg.IsSuperAdmin(99, adminUsernameConfigTest), "different user_id with recycled username must be rejected")
	})

	t.Run("userID 0 does not create binding", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{adminUsernameConfigTest},
		}
		require.True(t, cfg.IsSuperAdmin(0, adminUsernameConfigTest), "userID=0 should still return true for lookup")
		require.True(t, cfg.IsSuperAdmin(42, adminUsernameConfigTest), "should still be able to bind after userID=0 call")
		require.False(t, cfg.IsSuperAdmin(99, adminUsernameConfigTest), "recycled username rejected after real binding")
	})

	t.Run("userID 0 lookup returns true after binding", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{adminUsernameConfigTest},
		}
		require.True(t, cfg.IsSuperAdmin(42, adminUsernameConfigTest), "bootstrap binds")
		require.True(t, cfg.IsSuperAdmin(0, adminUsernameConfigTest), "lookup-only call must still return true after binding")
		require.False(t, cfg.IsSuperAdmin(99, adminUsernameConfigTest), "attacker still rejected")
	})
}

func TestConfig_IsUserWhitelisted(t *testing.T) {
	t.Parallel()

	t.Run(returnsTrueWhitelistedUserIDConfigTest, func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUserIDs:   []int64{100, 200, 300},
			WhitelistedUsernames: []string{aliceUsernameConfigTest},
		}
		require.True(t, cfg.IsUserWhitelisted(100, ""))
		require.True(t, cfg.IsUserWhitelisted(200, ""))
		require.True(t, cfg.IsUserWhitelisted(300, ""))
	})

	t.Run("returns true for whitelisted username bootstrap", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{aliceUsernameConfigTest, "bob", charlieUsernameConfigTest},
		}
		require.True(t, cfg.IsUserWhitelisted(999, aliceUsernameConfigTest))
		require.True(t, cfg.IsUserWhitelisted(888, "bob"))
		require.True(t, cfg.IsUserWhitelisted(777, charlieUsernameConfigTest))
	})

	t.Run("returns true for whitelisted username with @ prefix", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{aliceUsernameConfigTest, "bob"},
		}
		require.True(t, cfg.IsUserWhitelisted(999, "@alice"))
		require.True(t, cfg.IsUserWhitelisted(888, "@bob"))
	})

	t.Run("username check is case insensitive", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{aliceUsernameConfigTest, "bob", charlieUsernameConfigTest},
		}
		require.True(t, cfg.IsUserWhitelisted(999, "ALICE"))
		require.True(t, cfg.IsUserWhitelisted(888, "Bob"))
		require.True(t, cfg.IsUserWhitelisted(777, "ChArLiE"))
	})

	t.Run(returnsFalseNonWhitelistedUserConfigTest, func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUserIDs:   []int64{100},
			WhitelistedUsernames: []string{aliceUsernameConfigTest},
		}
		require.False(t, cfg.IsUserWhitelisted(999, "unknown"))
		require.False(t, cfg.IsUserWhitelisted(0, ""))
		require.False(t, cfg.IsUserWhitelisted(555, notInListUsernameConfig))
	})

	t.Run("returns false for empty whitelist", func(t *testing.T) {
		t.Parallel()
		emptyCfg := &Config{WhitelistedUserIDs: nil, WhitelistedUsernames: nil}
		require.False(t, emptyCfg.IsUserWhitelisted(100, aliceUsernameConfigTest))
	})

	t.Run("user ID match works even with non-whitelisted username", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUserIDs:   []int64{100},
			WhitelistedUsernames: []string{aliceUsernameConfigTest},
		}
		require.True(t, cfg.IsUserWhitelisted(100, notInListUsernameConfig))
	})

	t.Run("username binds on first use then enforces user_id", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{aliceUsernameConfigTest},
		}
		require.True(t, cfg.IsUserWhitelisted(999, aliceUsernameConfigTest))
		require.True(t, cfg.IsUserWhitelisted(999, aliceUsernameConfigTest))
		require.False(t, cfg.IsUserWhitelisted(888, aliceUsernameConfigTest), "different user_id must be rejected after binding")
	})
}

func TestConfig_SuperadminBound(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		WhitelistedUsernames: []string{adminUsernameConfigTest},
	}

	_, bound := cfg.SuperadminBound(adminUsernameConfigTest)
	require.False(t, bound, "should not be bound before first use")

	cfg.IsSuperAdmin(42, adminUsernameConfigTest)

	id, bound := cfg.SuperadminBound(adminUsernameConfigTest)
	require.True(t, bound)
	require.Equal(t, int64(42), id)
}

func TestConfig_LoadSuperadminBindings(t *testing.T) {
	t.Parallel()

	t.Run("pre-loaded binding prevents recycled username", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{adminUsernameConfigTest},
		}
		cfg.LoadSuperadminBindings([]SuperadminBinding{
			{Username: adminUsernameConfigTest, UserID: 42},
		})
		require.True(t, cfg.IsSuperAdmin(42, adminUsernameConfigTest))
		require.True(t, cfg.IsSuperAdmin(42, ""))
		require.False(t, cfg.IsSuperAdmin(99, adminUsernameConfigTest), "recycled username must be rejected")
	})

	t.Run("ignores bindings for non-whitelisted usernames", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{adminUsernameConfigTest},
		}
		cfg.LoadSuperadminBindings([]SuperadminBinding{
			{Username: "removed_user", UserID: 55},
		})
		require.False(t, cfg.IsSuperAdmin(55, ""), "non-whitelisted binding should be ignored")
	})

	t.Run("survives restart simulation", func(t *testing.T) {
		t.Parallel()

		cfg1 := &Config{
			WhitelistedUsernames: []string{adminUsernameConfigTest},
		}
		cfg1.IsSuperAdmin(42, adminUsernameConfigTest)

		id, bound := cfg1.SuperadminBound(adminUsernameConfigTest)
		require.True(t, bound)

		cfg2 := &Config{
			WhitelistedUsernames: []string{adminUsernameConfigTest},
		}
		cfg2.LoadSuperadminBindings([]SuperadminBinding{
			{Username: adminUsernameConfigTest, UserID: id},
		})
		require.True(t, cfg2.IsSuperAdmin(42, adminUsernameConfigTest))
		require.False(t, cfg2.IsSuperAdmin(99, adminUsernameConfigTest), "attacker must be rejected after restart+reload")
	})
}

func TestConfig_CheckSuperAdmin(t *testing.T) {
	t.Parallel()

	t.Run("returns new binding on first username match", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{adminUsernameConfigTest},
		}
		ok, binding := cfg.CheckSuperAdmin(42, adminUsernameConfigTest)
		require.True(t, ok)
		require.NotNil(t, binding)
		require.Equal(t, int64(42), binding.UserID)
		require.Equal(t, adminUsernameConfigTest, binding.Username)
	})

	t.Run("returns nil binding on subsequent calls", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			WhitelistedUsernames: []string{adminUsernameConfigTest},
		}
		cfg.CheckSuperAdmin(42, adminUsernameConfigTest)

		ok, binding := cfg.CheckSuperAdmin(42, adminUsernameConfigTest)
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
			WhitelistedUsernames: []string{adminUsernameConfigTest},
		}
		cfg.LoadSuperadminBindings([]SuperadminBinding{
			{Username: adminUsernameConfigTest, UserID: 42},
		})
		ok, binding := cfg.CheckSuperAdmin(42, adminUsernameConfigTest)
		require.True(t, ok)
		require.Nil(t, binding, "pre-loaded binding should not produce a new binding")
	})
}
