// Package config provides application configuration loading from environment.
package config

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application.
type Config struct {
	TelegramBotToken     string
	DatabaseURL          string
	GeminiAPIKey         string
	LogLevel             string
	WhitelistedUserIDs   []int64
	WhitelistedUsernames []string
	DailyReminderEnabled bool
	ReminderHour         int
	ReminderTimezone     string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		GeminiAPIKey:     os.Getenv("GEMINI_API_KEY"),
		LogLevel:         os.Getenv("LOG_LEVEL"),
	}

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

	whitelistStr := os.Getenv("WHITELISTED_USER_IDS")
	if whitelistStr != "" {
		for idStr := range strings.SplitSeq(whitelistStr, ",") {
			idStr = strings.TrimSpace(idStr)
			if idStr == "" {
				continue
			}
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				continue
			}
			cfg.WhitelistedUserIDs = append(cfg.WhitelistedUserIDs, id)
		}
	}

	whitelistUsernames := os.Getenv("WHITELISTED_USERNAMES")
	if whitelistUsernames != "" {
		for username := range strings.SplitSeq(whitelistUsernames, ",") {
			username = strings.TrimSpace(username)
			if username == "" {
				continue
			}
			// Remove @ prefix if present
			username = strings.TrimPrefix(username, "@")
			cfg.WhitelistedUsernames = append(cfg.WhitelistedUsernames, username)
		}
	}

	// Validate required configuration.
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
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

// IsSuperAdmin checks if a user is a superadmin (defined via environment variables).
func (c *Config) IsSuperAdmin(userID int64, username string) bool {
	return c.IsUserWhitelisted(userID, username)
}

// IsUserWhitelisted checks if a Telegram user ID or username is in the whitelist.
// Returns true if either the user ID or username is whitelisted.
func (c *Config) IsUserWhitelisted(userID int64, username string) bool {
	// Check user ID whitelist
	if slices.Contains(c.WhitelistedUserIDs, userID) {
		return true
	}

	// Check username whitelist (case-insensitive)
	if username != "" {
		username = strings.TrimPrefix(username, "@")
		for _, whitelisted := range c.WhitelistedUsernames {
			if strings.EqualFold(whitelisted, username) {
				return true
			}
		}
	}

	return false
}
