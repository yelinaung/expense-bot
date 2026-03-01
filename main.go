// Package main is the entry point for the expense tracker Telegram bot.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gitlab.com/yelinaung/expense-bot/internal/bot"
	"gitlab.com/yelinaung/expense-bot/internal/config"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/logger"
	"gitlab.com/yelinaung/expense-bot/internal/telemetry"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type runError struct {
	logMessage string
	err        error
}

func (e *runError) Error() string {
	return fmt.Sprintf("%s: %v", e.logMessage, e.err)
}

func (e *runError) Unwrap() error {
	return e.err
}

func wrapRunError(logMessage string, err error) error {
	return &runError{
		logMessage: logMessage,
		err:        err,
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := run(ctx, os.Args, os.Stdout)
	if err == nil {
		return
	}

	var re *runError
	if errors.As(err, &re) {
		logger.Log.Fatal().Err(err).Msg(re.logMessage)
	}
	logger.Log.Fatal().Err(err).Msg("Application failed")
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) > 1 && args[1] == "version" {
		_, _ = fmt.Fprintf(stdout, "expense-bot %s (commit: %s, built: %s)\n", version, commit, date)
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		return wrapRunError("Failed to load config", err)
	}

	logger.SetLevel(cfg.LogLevel)
	logger.InitHashSalt()

	otelProviders, err := telemetry.Init(runCtx, &telemetry.Config{
		Enabled:         cfg.OTelEnabled,
		ServiceName:     cfg.OTelServiceName,
		ServiceVersion:  version,
		Environment:     cfg.OTelEnvironment,
		ExporterType:    cfg.OTelExporterType,
		Endpoint:        cfg.OTelEndpoint,
		Insecure:        cfg.OTelInsecure,
		TraceSampleRate: cfg.OTelTraceSampleRate,
	})
	if err != nil {
		return wrapRunError("Failed to initialize OpenTelemetry", err)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
		defer shutdownCancel()
		if err := otelProviders.Shutdown(shutdownCtx); err != nil {
			logger.Log.Error().Err(err).Msg("Failed to shutdown OpenTelemetry")
		}
	}()

	pool, err := database.Connect(runCtx, cfg.DatabaseURL, cfg.OTelEnabled)
	if err != nil {
		return wrapRunError("Failed to connect to database", err)
	}
	defer pool.Close()

	if err := database.RunMigrations(runCtx, pool); err != nil {
		return wrapRunError("Failed to run migrations", err)
	}

	if err := database.SeedCategories(runCtx, pool); err != nil {
		return wrapRunError("Failed to seed categories", err)
	}

	logger.Log.Info().Msg("Database initialized successfully")

	telegramBot, err := bot.New(cfg, pool)
	if err != nil {
		return wrapRunError("Failed to create bot", err)
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		logger.Log.Info().Msg("Shutting down...")
		cancel()
	}()

	telegramBot.Start(runCtx)
	return nil
}
