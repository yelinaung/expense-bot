package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMainVersionCommand(t *testing.T) {
	originalArgs := os.Args
	originalStdout := os.Stdout
	originalVersion := version
	originalCommit := commit
	originalDate := date
	t.Cleanup(func() {
		os.Args = originalArgs
		os.Stdout = originalStdout
		version = originalVersion
		commit = originalCommit
		date = originalDate
	})

	os.Args = []string{"expense-bot", "version"}
	version = "v1.2.3"
	commit = "abc123"
	date = "2026-02-26"

	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	main()

	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "expense-bot v1.2.3 (commit: abc123, built: 2026-02-26)\n", string(out))
}

func TestMainExitsWhenTelemetryInitFails(t *testing.T) {
	if os.Getenv("TEST_MAIN_TELEMETRY_FAIL") == "1" {
		main()
		return
	}

	cmd := exec.CommandContext( //nolint:gosec // Controlled test harness invocation.
		context.Background(),
		os.Args[0],
		"-test.run=TestMainExitsWhenTelemetryInitFails",
	)
	cmd.Env = append(
		os.Environ(),
		"TEST_MAIN_TELEMETRY_FAIL=1",
		"TELEGRAM_BOT_TOKEN=test-token",
		"DATABASE_URL=postgres://user:pass@localhost:5432/db?sslmode=disable",
		"WHITELISTED_USER_IDS=1",
		"LOG_HASH_SALT=test-salt-for-main-tests-1234567890",
		"OTEL_ENABLED=true",
		"OTEL_EXPORTER_TYPE=invalid-exporter",
	)
	out, err := cmd.CombinedOutput()
	require.Error(t, err)
	require.Contains(t, string(out), "Failed to initialize OpenTelemetry")
}

func TestMainExitsWhenDatabaseConnectFails(t *testing.T) {
	if os.Getenv("TEST_MAIN_DB_FAIL") == "1" {
		main()
		return
	}

	cmd := exec.CommandContext( //nolint:gosec // Controlled test harness invocation.
		context.Background(),
		os.Args[0],
		"-test.run=TestMainExitsWhenDatabaseConnectFails",
	)
	cmd.Env = append(
		os.Environ(),
		"TEST_MAIN_DB_FAIL=1",
		"TELEGRAM_BOT_TOKEN=test-token",
		"DATABASE_URL=postgres://invalid-connection-string",
		"WHITELISTED_USER_IDS=1",
		"LOG_HASH_SALT=test-salt-for-main-tests-1234567890",
		"OTEL_ENABLED=false",
	)
	out, err := cmd.CombinedOutput()
	require.Error(t, err)
	require.Contains(t, string(out), "Failed to connect to database")
}
