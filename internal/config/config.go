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
	TelegramBotToken   string
	DatabaseURL        string
	GeminiAPIKey       string
	LogLevel           string
	WhitelistedUserIDs []int64
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

	return cfg, nil
}

// IsUserWhitelisted checks if a Telegram user ID is in the whitelist.
func (c *Config) IsUserWhitelisted(userID int64) bool {
	return slices.Contains(c.WhitelistedUserIDs, userID)
}
