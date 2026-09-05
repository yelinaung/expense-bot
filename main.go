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

	if re, ok := errors.AsType[*runError](err); ok {
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

	logLevel, err := logger.ParseLevel(cfg.LogLevel)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Invalid LOG_LEVEL; defaulting to info")
	}
	logger.SetLevel(logLevel)
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

	telegramBot, err := bot.New(runCtx, cfg, pool)
	if err != nil {
		return wrapRunError("Failed to create bot", err)
	}

	// The first SIGINT/SIGTERM triggers graceful shutdown via cancel; a second
	// one forces an immediate exit so a stall (e.g. a slow OpenTelemetry flush)
	// can be interrupted instead of swallowing the signal. The goroutine is
	// detached: on the normal path run returns and the process exits while this
	// blocks on the second read, which is harmless.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go watchSignals(sigChan, cancel, os.Exit)

	telegramBot.Start(runCtx)
	return nil
}

// watchSignals translates the first signal on sigChan into graceful shutdown
// via cancel, then forces an exit via exit on a second signal so a slow
// shutdown can be interrupted.
func watchSignals(sigChan <-chan os.Signal, cancel context.CancelFunc, exit func(int)) {
	<-sigChan
	logger.Log.Info().Msg("Shutting down...")
	cancel()
	<-sigChan
	logger.Log.Warn().Msg("Forcing exit")
	exit(1)
}
