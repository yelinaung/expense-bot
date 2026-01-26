// Package main is the entry point for the expense tracker Telegram bot.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"gitlab.com/yelinaung/expense-bot/internal/bot"
	"gitlab.com/yelinaung/expense-bot/internal/config"
	"gitlab.com/yelinaung/expense-bot/internal/database"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := database.RunMigrations(ctx, pool); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	if err := database.SeedCategories(ctx, pool); err != nil {
		log.Fatalf("Failed to seed categories: %v", err)
	}

	log.Println("Database initialized successfully")

	telegramBot, err := bot.New(cfg, pool)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down...")
		cancel()
	}()

	telegramBot.Start(ctx)
}
