package telemetry

import (
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/logger"
	"go.opentelemetry.io/otel/attribute"
)

func attrsToMap(attrs []attribute.KeyValue) map[string]string {
	out := make(map[string]string, len(attrs))
	for i := range attrs {
		out[string(attrs[i].Key)] = attrs[i].Value.AsString()
	}
	return out
}

func TestClassifyUpdate(t *testing.T) {
	t.Parallel()

	known := map[string]struct{}{"/add": {}, "/today": {}}

	t.Run("classifies command and strips mention", func(t *testing.T) {
		t.Parallel()
		update := &models.Update{
			Message: &models.Message{
				Text: "/add@mybot 10 coffee",
			},
		}
		require.Equal(t, "telegram.command /add", classifyUpdate(update, known))
	})

	t.Run("classifies callback prefix", func(t *testing.T) {
		t.Parallel()
		update := &models.Update{
			CallbackQuery: &models.CallbackQuery{Data: "receipt_confirm_123"},
		}
		require.Equal(t, "telegram.callback receipt_confirm", classifyUpdate(update, known))
	})

	t.Run("collapses unknown command to unknown", func(t *testing.T) {
		t.Parallel()
		update := &models.Update{Message: &models.Message{Text: "/nonexistent"}}
		require.Equal(t, "telegram.command unknown", classifyUpdate(update, known))
	})

	t.Run("collapses numeric command to unknown", func(t *testing.T) {
		t.Parallel()
		update := &models.Update{Message: &models.Message{Text: "/1"}}
		require.Equal(t, "telegram.command unknown", classifyUpdate(update, known))
	})

	t.Run("collapses each distinct numeric token to the same name", func(t *testing.T) {
		t.Parallel()
		for _, text := range []string{"/1", "/2", "/3", "/999999"} {
			update := &models.Update{Message: &models.Message{Text: text}}
			require.Equal(t, "telegram.command unknown", classifyUpdate(update, known))
		}
	})

	t.Run("collapses disallowed characters to unknown", func(t *testing.T) {
		t.Parallel()
		for _, text := range []string{
			"/héllo", "/foo bar", "/foo!bar", "/foo.bar", "/emoji😀",
		} {
			update := &models.Update{Message: &models.Message{Text: text}}
			require.Equal(t, "telegram.command unknown", classifyUpdate(update, known))
		}
	})

	t.Run("normalizes command case", func(t *testing.T) {
		t.Parallel()
		update := &models.Update{Message: &models.Message{Text: "/ADD args"}}
		require.Equal(t, "telegram.command /add", classifyUpdate(update, known))
	})

	t.Run("caps overly long command token to unknown", func(t *testing.T) {
		t.Parallel()
		long := "/" + strings.Repeat("a", commandTokenMaxLength*4)
		update := &models.Update{Message: &models.Message{Text: long}}
		require.Equal(t, "telegram.command unknown", classifyUpdate(update, known))
	})

	t.Run("bare slash collapses to unknown", func(t *testing.T) {
		t.Parallel()
		update := &models.Update{Message: &models.Message{Text: "/"}}
		require.Equal(t, "telegram.command unknown", classifyUpdate(update, known))
	})

	t.Run("slash mention only collapses to unknown", func(t *testing.T) {
		t.Parallel()
		update := &models.Update{Message: &models.Message{Text: "/@bot"}}
		require.Equal(t, "telegram.command unknown", classifyUpdate(update, known))
	})

	t.Run("non-command text classified as text", func(t *testing.T) {
		t.Parallel()
		update := &models.Update{Message: &models.Message{Text: "hello there"}}
		require.Equal(t, "telegram.text", classifyUpdate(update, known))
	})

	t.Run("empty known set collapses every command to unknown", func(t *testing.T) {
		t.Parallel()
		update := &models.Update{Message: &models.Message{Text: "/add"}}
		require.Equal(
			t,
			"telegram.command unknown",
			classifyUpdate(update, map[string]struct{}{}),
		)
	})

	t.Run("classifies non-text message types", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "telegram.voice", classifyUpdate(&models.Update{
			Message: &models.Message{Voice: &models.Voice{}},
		}, known))
		require.Equal(t, "telegram.photo", classifyUpdate(&models.Update{
			Message: &models.Message{Photo: []models.PhotoSize{{}}},
		}, known))
		require.Equal(t, "telegram.document", classifyUpdate(&models.Update{
			Message: &models.Message{Document: &models.Document{}},
		}, known))
		require.Equal(t, "telegram.message", classifyUpdate(&models.Update{
			Message: &models.Message{},
		}, known))
		require.Equal(t, "telegram.edited_message", classifyUpdate(&models.Update{
			EditedMessage: &models.Message{},
		}, known))
		require.Equal(t, "telegram.update", classifyUpdate(&models.Update{}, known))
	})
}

func TestExtractCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want string
	}{
		{"empty", "", ""},
		{"no leading slash", "add foo", ""},
		{"simple", "/add", "/add"},
		{"with args", "/add 10 coffee", "/add"},
		{"strips at bot mention", "/add@mybot 10 coffee", "/add"},
		{"lowercases", "/ADD", "/add"},
		{"strips mention then lowercases", "/ADD@MyBot", "/add"},
		{"bare slash", "/", "/"},
		{"mention only leaves bare slash", "/@bot", "/"},
		{"caps length", "/" + strings.Repeat("a", 100), "/" + strings.Repeat("a", commandTokenMaxLength)},
		{"keeps digits and underscore", "/add_category_1", "/add_category_1"},
		{"keeps unicode as is", "/héllo", "/héllo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, extractCommand(tt.text))
		})
	}
}

func TestResolveCommand(t *testing.T) {
	t.Parallel()

	known := map[string]struct{}{"/add": {}, "/today": {}}

	tests := []struct {
		name string
		cmd  string
		want string
	}{
		{"known command", "/add", "/add"},
		{"known command other", "/today", "/today"},
		{"unknown command", "/foo", "unknown"},
		{"numeric command", "/1", "unknown"},
		{"disallowed unicode", "/héllo", "unknown"},
		{"disallowed punctuation", "/foo!bar", "unknown"},
		{"disallowed space", "/foo bar", "unknown"},
		{"bare slash", "/", "unknown"},
		{"long valid unknown", "/" + strings.Repeat("a", commandTokenMaxLength), "unknown"},
		{"underscore only unknown", "/_", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, resolveCommand(tt.cmd, known))
		})
	}
}

// TestClassifyUpdateCardinalityBounded asserts that a broad range of
// attacker-controlled inputs maps onto a fixed, small vocabulary of span names,
// preventing the high-cardinality span-name growth described in the bug report.
func TestClassifyUpdateCardinalityBounded(t *testing.T) {
	t.Parallel()

	known := map[string]struct{}{"/add": {}, "/today": {}}

	inputs := []string{
		"/1", "/2", "/3", "/9999",
		"/foo", "/bar", "/baz",
		"/héllo", "/emoji😀", "/foo bar",
		"/" + strings.Repeat("a", 100),
		"/" + strings.Repeat("1", 100),
		"/add", "/today",
		"/ADD", "/Add@bot",
		"/", "/@bot",
	}

	got := make(map[string]struct{})
	for _, text := range inputs {
		update := &models.Update{Message: &models.Message{Text: text}}
		got[classifyUpdate(update, known)] = struct{}{}
	}

	allowed := map[string]struct{}{
		"telegram.command /add":    {},
		"telegram.command /today":  {},
		"telegram.command unknown": {},
	}
	for name := range got {
		require.Contains(t, allowed, name, "unexpected high-cardinality span name")
	}
	// The "unknown" bucket must be present: all attacker inputs collapse into it.
	require.Contains(t, got, "telegram.command unknown")
}

func TestUpdateAttributes(t *testing.T) {
	t.Parallel()
	logger.InitHashSaltForTesting("test-salt-for-telemetry-attributes-1234567890")

	t.Run("extracts callback user and chat attributes", func(t *testing.T) {
		t.Parallel()
		update := &models.Update{
			CallbackQuery: &models.CallbackQuery{
				From: models.User{ID: 42},
				Message: models.MaybeInaccessibleMessage{
					Message: &models.Message{
						Chat: models.Chat{ID: 99},
					},
				},
			},
		}

		attrs := attrsToMap(updateAttributes(update))
		require.Equal(t, "telegram", attrs[testMessagingSystemAttr])
		require.Equal(t, logger.HashUserID(42), attrs[testTelegramUserIDAttr])
		require.Equal(t, logger.HashChatID(99), attrs[testTelegramChatIDAttr])
	})

	t.Run("extracts message user and chat attributes", func(t *testing.T) {
		t.Parallel()
		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: 88},
				From: &models.User{ID: 77},
			},
		}

		attrs := attrsToMap(updateAttributes(update))
		require.Equal(t, "telegram", attrs[testMessagingSystemAttr])
		require.Equal(t, logger.HashUserID(77), attrs[testTelegramUserIDAttr])
		require.Equal(t, logger.HashChatID(88), attrs[testTelegramChatIDAttr])
	})

	t.Run("extracts edited message user and chat attributes", func(t *testing.T) {
		t.Parallel()
		update := &models.Update{
			EditedMessage: &models.Message{
				Chat: models.Chat{ID: 66},
				From: &models.User{ID: 55},
			},
		}

		attrs := attrsToMap(updateAttributes(update))
		require.Equal(t, "telegram", attrs[testMessagingSystemAttr])
		require.Equal(t, logger.HashUserID(55), attrs[testTelegramUserIDAttr])
		require.Equal(t, logger.HashChatID(66), attrs[testTelegramChatIDAttr])
	})
}

// FuzzClassifyUpdateCommand asserts the cardinality invariant that is the
// subject of this fix: for any user-supplied message text, the derived span
// name is always one of a fixed vocabulary (registered commands plus a single
// "unknown" bucket and the non-command classes), never a per-message value.
func FuzzClassifyUpdateCommand(f *testing.F) {
	known := map[string]struct{}{"/add": {}, "/today": {}}

	// Seeds cover the exploit scenario, known commands, and edge cases.
	f.Add("/1")
	f.Add("/add")
	f.Add("/add@mybot 10 coffee")
	f.Add("/nonexistent")
	f.Add("/héllo")
	f.Add("/")
	f.Add("/ADD")
	f.Add("hello there")
	f.Add("")

	allowed := map[string]struct{}{
		"telegram.command /add":    {},
		"telegram.command /today":  {},
		"telegram.command unknown": {},
		"telegram.text":            {},
		"telegram.message":         {},
	}
	f.Fuzz(func(t *testing.T, text string) {
		update := &models.Update{Message: &models.Message{Text: text}}
		name := classifyUpdate(update, known)
		require.Containsf(t, allowed, name, "span name must be within the fixed vocabulary, got %q", name)
	})
}
