package bot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
)

func TestRegisterCommands_NoPanicOnAPIFailure(t *testing.T) {
	t.Parallel()

	// fakeHTTPClient returns {"ok":false}, causing SetMyCommands to error.
	// registerCommands must log a warning and return without panicking.
	b := &Bot{}
	tgBot, err := tgbot.New(
		"123:TESTTOKEN",
		tgbot.WithSkipGetMe(),
		tgbot.WithHTTPClient(time.Second, &fakeHTTPClient{}),
		tgbot.WithServerURL("http://example.com"),
	)
	require.NoError(t, err)
	b.bot = tgBot

	require.NotPanics(t, func() {
		b.registerCommands(context.Background())
	})
}

func TestRegisterCommands_CommandsAreValid(t *testing.T) {
	t.Parallel()

	// The go-telegram/bot library sends requests as multipart/form-data.
	// The "commands" field contains the JSON-encoded BotCommand array.
	var (
		mu               sync.Mutex
		capturedCommands string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "setMyCommands") {
			if r.ParseMultipartForm(1<<20) == nil {
				if vals, ok := r.MultipartForm.Value["commands"]; ok && len(vals) > 0 {
					mu.Lock()
					capturedCommands = vals[0]
					mu.Unlock()
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer server.Close()

	b := &Bot{}
	tgBot, err := tgbot.New(
		"123:TESTTOKEN",
		tgbot.WithSkipGetMe(),
		tgbot.WithHTTPClient(time.Second, http.DefaultClient),
		tgbot.WithServerURL(server.URL),
	)
	require.NoError(t, err)
	b.bot = tgBot

	b.registerCommands(context.Background())

	mu.Lock()
	raw := capturedCommands
	mu.Unlock()
	require.NotEmpty(t, raw, "expected SetMyCommands request to be sent")

	var commands []tgmodels.BotCommand
	require.NoError(t, json.Unmarshal([]byte(raw), &commands))
	require.NotEmpty(t, commands)

	seen := make(map[string]bool)
	for _, cmd := range commands {
		require.NotEmpty(t, cmd.Command, "command name must not be empty")
		require.NotEmpty(t, cmd.Description, "description must not be empty")
		require.LessOrEqual(t, len(cmd.Command), 32, "command %q exceeds Telegram 32-char limit", cmd.Command)
		require.LessOrEqual(t, len(cmd.Description), 256, "description for %q exceeds Telegram 256-char limit", cmd.Command)
		require.False(t, seen[cmd.Command], "duplicate command: %q", cmd.Command)
		seen[cmd.Command] = true
	}

	// Core user commands must be present.
	for _, required := range []string{"add", "list", "today", "week", "help"} {
		require.True(t, seen[required], "expected command %q to be registered", required)
	}

	// Admin-only commands must not be in the menu.
	for _, admin := range []string{"approve", "revoke", "users"} {
		require.False(t, seen[admin], "admin command %q should not be in the menu", admin)
	}
}
