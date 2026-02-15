// Package config provides application configuration loading from environment.
package config

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application.
type Config struct {
	TelegramBotToken     string
	DatabaseURL          string
	GeminiAPIKey         string
	ExchangeRateBaseURL  string
	ExchangeRateTimeout  time.Duration
	ExchangeRateCacheTTL time.Duration
	LogLevel             string
	WhitelistedUserIDs   []int64
	WhitelistedUsernames []string
	DailyReminderEnabled bool
	ReminderHour         int
	ReminderTimezone     string

	// resolvedSuperadmins maps normalized username → bound user_id.
	// Once a whitelisted username is seen with a real user_id, the
	// binding is recorded and only that user_id is accepted for the
	// username going forward. This prevents recycled-username attacks.
	resolvedSuperadmins   map[string]int64
	resolvedSuperadminIDs map[int64]struct{}
	resolvedMu            sync.RWMutex
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := newDefaultConfig()
	if err := applyExchangeRateConfig(cfg); err != nil {
		return nil, err
	}
	applyReminderConfig(cfg)
	cfg.WhitelistedUserIDs = parseWhitelistedUserIDs(os.Getenv("WHITELISTED_USER_IDS"))
	cfg.WhitelistedUsernames = parseWhitelistedUsernames(os.Getenv("WHITELISTED_USERNAMES"))

	// Validate required configuration.
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func newDefaultConfig() *Config {
	return &Config{
		TelegramBotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		GeminiAPIKey:          os.Getenv("GEMINI_API_KEY"),
		ExchangeRateBaseURL:   "https://api.frankfurter.app",
		ExchangeRateTimeout:   5 * time.Second,
		ExchangeRateCacheTTL:  12 * time.Hour,
		LogLevel:              os.Getenv("LOG_LEVEL"),
		resolvedSuperadmins:   make(map[string]int64),
		resolvedSuperadminIDs: make(map[int64]struct{}),
	}
}

func applyExchangeRateConfig(cfg *Config) error {
	if baseURL := strings.TrimSpace(os.Getenv("EXCHANGE_RATE_BASE_URL")); baseURL != "" {
		// Validate URL scheme to prevent SSRF.
		if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
			return errors.New("EXCHANGE_RATE_BASE_URL must use http:// or https:// scheme")
		}
		cfg.ExchangeRateBaseURL = baseURL
	}

	if timeout := strings.TrimSpace(os.Getenv("EXCHANGE_RATE_TIMEOUT")); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil && d > 0 {
			cfg.ExchangeRateTimeout = d
		}
	}

	if cacheTTL := strings.TrimSpace(os.Getenv("EXCHANGE_RATE_CACHE_TTL")); cacheTTL != "" {
		if d, err := time.ParseDuration(cacheTTL); err == nil && d > 0 {
			cfg.ExchangeRateCacheTTL = d
		}
	}
	return nil
}

func applyReminderConfig(cfg *Config) {
	cfg.DailyReminderEnabled = os.Getenv("DAILY_REMINDER_ENABLED") == "true"
	cfg.ReminderHour = 20
	if hourStr := os.Getenv("REMINDER_HOUR"); hourStr != "" {
		if h, err := strconv.Atoi(hourStr); err == nil && h >= 0 && h <= 23 {
			cfg.ReminderHour = h
		}
	}
	cfg.ReminderTimezone = "Asia/Singapore"
	if tz := os.Getenv("REMINDER_TIMEZONE"); tz != "" {
		if _, err := time.LoadLocation(tz); err == nil {
			cfg.ReminderTimezone = tz
		}
	}
}

func parseWhitelistedUserIDs(raw string) []int64 {
	var ids []int64
	for idStr := range strings.SplitSeq(raw, ",") {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func parseWhitelistedUsernames(raw string) []string {
	var usernames []string
	for username := range strings.SplitSeq(raw, ",") {
		username = strings.TrimSpace(username)
		if username == "" {
			continue
		}
		usernames = append(usernames, strings.TrimPrefix(username, "@"))
	}
	return usernames
}

// validate checks that all required configuration is present.
func (c *Config) validate() error {
	var errs []string

	if c.TelegramBotToken == "" {
		errs = append(errs, "TELEGRAM_BOT_TOKEN is required")
	}

	if c.DatabaseURL == "" {
		errs = append(errs, "DATABASE_URL is required")
	}

	if len(c.WhitelistedUserIDs) == 0 && len(c.WhitelistedUsernames) == 0 {
		errs = append(errs, "at least one whitelisted user (WHITELISTED_USER_IDS or WHITELISTED_USERNAMES) is required")
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}

// normalizeUsername returns a lowercase, @-stripped username for map keys.
func normalizeUsername(u string) string {
	return strings.ToLower(strings.TrimPrefix(u, "@"))
}

// ensureResolvedMaps lazily initializes the resolved maps under a
// write lock. This is needed for Config structs created via literal
// in tests. Must be called before any read-lock access.
func (c *Config) ensureResolvedMaps() {
	c.resolvedMu.Lock()
	if c.resolvedSuperadmins == nil {
		c.resolvedSuperadmins = make(map[string]int64)
	}
	if c.resolvedSuperadminIDs == nil {
		c.resolvedSuperadminIDs = make(map[int64]struct{})
	}
	c.resolvedMu.Unlock()
}

// SuperadminBinding represents a username → user_id binding that
// should be persisted by the caller.
type SuperadminBinding struct {
	Username string
	UserID   int64
}

// LoadSuperadminBindings pre-loads persisted username → user_id
// bindings into the in-memory maps. Call this at startup after
// loading bindings from the database. Only bindings whose username
// matches a current WHITELISTED_USERNAMES entry are loaded.
func (c *Config) LoadSuperadminBindings(bindings []SuperadminBinding) {
	c.ensureResolvedMaps()
	c.resolvedMu.Lock()
	defer c.resolvedMu.Unlock()

	for _, b := range bindings {
		norm := normalizeUsername(b.Username)
		if !c.isWhitelistedUsername(norm) {
			continue
		}
		c.resolvedSuperadmins[norm] = b.UserID
		c.resolvedSuperadminIDs[b.UserID] = struct{}{}
	}
}

// IsSuperAdmin checks if a user is a superadmin (defined via environment
// variables). On the first call with a whitelisted username and a non-zero
// userID, the username is bound to that userID. Subsequent calls with the
// same username but a different userID are rejected, preventing
// recycled-username attacks.
func (c *Config) IsSuperAdmin(userID int64, username string) bool {
	_, ok := c.checkWhitelist(userID, username)
	return ok
}

// checkWhitelist is the internal implementation of IsUserWhitelisted.
// It returns a non-nil *SuperadminBinding when a new binding was just
// created (needs persistence) and whether the user is whitelisted.
func (c *Config) checkWhitelist(userID int64, username string) (*SuperadminBinding, bool) {
	if slices.Contains(c.WhitelistedUserIDs, userID) {
		return nil, true
	}

	c.ensureResolvedMaps()

	c.resolvedMu.RLock()
	if userID != 0 {
		if _, ok := c.resolvedSuperadminIDs[userID]; ok {
			c.resolvedMu.RUnlock()
			return nil, true
		}
	}
	c.resolvedMu.RUnlock()

	if username == "" {
		return nil, false
	}

	norm := normalizeUsername(username)
	if !c.isWhitelistedUsername(norm) {
		return nil, false
	}

	c.resolvedMu.Lock()
	defer c.resolvedMu.Unlock()

	if boundID, bound := c.resolvedSuperadmins[norm]; bound {
		// When userID is 0 this is a lookup-only call (e.g. the
		// /revoke guard checking whether a target is a superadmin).
		// Return true so the caller knows the username belongs to a
		// superadmin, but do not bind.
		if userID == 0 {
			return nil, true
		}
		return nil, userID == boundID
	}

	if userID != 0 {
		c.resolvedSuperadmins[norm] = userID
		c.resolvedSuperadminIDs[userID] = struct{}{}
		return &SuperadminBinding{Username: norm, UserID: userID}, true
	}
	return nil, true
}

// IsUserWhitelisted checks if a Telegram user ID or username is in the
// whitelist. Usernames are treated as bootstrap-only: once a username is
// seen with a real user_id, the binding is recorded and only that
// user_id is accepted for the username going forward.
func (c *Config) IsUserWhitelisted(userID int64, username string) bool {
	_, ok := c.checkWhitelist(userID, username)
	return ok
}

// CheckSuperAdmin is like IsSuperAdmin but also returns a non-nil
// *SuperadminBinding when a new username → user_id binding was just
// created and should be persisted by the caller.
func (c *Config) CheckSuperAdmin(userID int64, username string) (isSuperAdmin bool, newBinding *SuperadminBinding) {
	binding, ok := c.checkWhitelist(userID, username)
	return ok, binding
}

// isWhitelistedUsername checks whether norm (already lowered, @-stripped)
// matches any entry in WhitelistedUsernames.
func (c *Config) isWhitelistedUsername(norm string) bool {
	for _, w := range c.WhitelistedUsernames {
		if normalizeUsername(w) == norm {
			return true
		}
	}
	return false
}

// SuperadminBound reports whether the given username has already been
// bound to a specific user_id via the bootstrap mechanism.
func (c *Config) SuperadminBound(username string) (userID int64, bound bool) {
	norm := normalizeUsername(username)
	c.ensureResolvedMaps()
	c.resolvedMu.RLock()
	defer c.resolvedMu.RUnlock()
	id, ok := c.resolvedSuperadmins[norm]
	return id, ok
}
