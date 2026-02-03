// Package main is the entry point for the expense tracker Telegram bot.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"gitlab.com/yelinaung/expense-bot/internal/bot"
	"gitlab.com/yelinaung/expense-bot/internal/config"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/logger"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to load config")
	}

	logger.SetLevel(cfg.LogLevel)
	logger.InitHashSalt()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer pool.Close()

	if err := database.RunMigrations(ctx, pool); err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to run migrations")
	}

	if err := database.SeedCategories(ctx, pool); err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to seed categories")
	}

	logger.Log.Info().Msg("Database initialized successfully")

	telegramBot, err := bot.New(cfg, pool)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to create bot")
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		logger.Log.Info().Msg("Shutting down...")
		cancel()
	}()

	telegramBot.Start(ctx)
}
