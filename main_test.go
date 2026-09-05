package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

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

func TestWatchSignalsGracefulThenForcedExit(t *testing.T) {
	t.Parallel()

	sigChan := make(chan os.Signal, 2)
	canceled := make(chan struct{})
	cancel := func() { close(canceled) }
	exitCode := make(chan int, 1)
	exit := func(code int) { exitCode <- code }

	go watchSignals(sigChan, cancel, exit)

	sigChan <- syscall.SIGTERM

	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("first signal did not trigger graceful cancel")
	}

	select {
	case code := <-exitCode:
		t.Fatalf("exit called before second signal (code=%d)", code)
	default:
	}

	sigChan <- syscall.SIGTERM

	select {
	case code := <-exitCode:
		require.Equal(t, 1, code)
	case <-time.After(time.Second):
		t.Fatal("second signal did not trigger forced exit")
	}
}

// TestWatchSignalsReceivesSecondRealSignal exercises the real os/signal
// runtime to confirm the signal.Notify registration stays active after the
// first signal, so a second SIGTERM is delivered and forces exit instead of
// being swallowed. This is the regression test for the swallowed-second-signal
// bug.
func TestWatchSignalsReceivesSecondRealSignal(t *testing.T) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	t.Cleanup(func() { signal.Stop(sigChan) })

	canceled := make(chan struct{})
	cancel := func() { close(canceled) }
	exitCode := make(chan int, 1)
	exit := func(code int) { exitCode <- code }

	go watchSignals(sigChan, cancel, exit)

	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))

	select {
	case <-canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("first SIGTERM was not delivered to watchSignals")
	}

	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))

	select {
	case code := <-exitCode:
		require.Equal(t, 1, code)
	case <-time.After(2 * time.Second):
		t.Fatal("second SIGTERM was swallowed; forced exit not triggered")
	}
}
