// Package config provides application configuration loading from environment.
package config

import (
	"os"
	"slices"
	"strconv"
	"strings"

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

	return cfg, nil
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
