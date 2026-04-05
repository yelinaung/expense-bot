package main

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const testMainAppName = "expense-bot"

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

	os.Args = []string{testMainAppName, "version"}
	version = "v1.2.3"
	commit = "abc123"
	date = "2026-02-26"

	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	err = run(context.Background(), os.Args, os.Stdout)
	require.NoError(t, err)

	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, testMainAppName+" v1.2.3 (commit: abc123, built: 2026-02-26)\n", string(out))
}

func TestMainExitsWhenTelemetryInitFails(t *testing.T) {
	for _, kv := range []string{
		"TELEGRAM_BOT_TOKEN=test-token",
		"DATABASE_URL=postgres://user:pass@localhost:5432/db?sslmode=disable", //gitleaks:allow
		"WHITELISTED_USER_IDS=1",
		"LOG_HASH_SALT=test-salt-for-main-tests-1234567890",
		"OTEL_ENABLED=true",
		"OTEL_EXPORTER_TYPE=invalid-exporter",
	} {
		key, value, _ := strings.Cut(kv, "=")
		t.Setenv(key, value)
	}

	err := run(context.Background(), []string{testMainAppName}, io.Discard)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Failed to initialize OpenTelemetry")
}

func TestMainExitsWhenDatabaseConnectFails(t *testing.T) {
	for _, kv := range []string{
		"TELEGRAM_BOT_TOKEN=test-token",
		"DATABASE_URL=postgres://invalid-connection-string",
		"WHITELISTED_USER_IDS=1",
		"LOG_HASH_SALT=test-salt-for-main-tests-1234567890",
		"OTEL_ENABLED=false",
	} {
		key, value, _ := strings.Cut(kv, "=")
		t.Setenv(key, value)
	}

	err := run(context.Background(), []string{testMainAppName}, io.Discard)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Failed to connect to database")
}
