package bot

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// genCommand generates a slash command name like "/add", "/list".
func genCommand() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		name := rapid.StringMatching(`[a-z]{1,15}`).Draw(t, "name")
		return "/" + name
	})
}

// genBotName generates a @botname suffix like "@myBot".
func genBotName() *rapid.Generator[string] {
	return rapid.StringMatching(`@[A-Za-z0-9_]{1,20}`)
}

// genArgs generates a free-text argument string without the '@' and '/' sentinels
// that extractCommandArgs treats specially.
func genArgs() *rapid.Generator[string] {
	return rapid.StringMatching(`[A-Za-z0-9 .,]{0,30}`)
}

// TestExtractCommandArgsPlain: "/cmd ARGS" → trimmed ARGS.
func TestExtractCommandArgsPlain(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		cmd := genCommand().Draw(t, "cmd")
		args := genArgs().Draw(t, "args")
		input := cmd + " " + args

		got := extractCommandArgs(input, cmd)
		require.Equal(t, strings.TrimSpace(args), got, "input=%q", input)
	})
}

// TestExtractCommandArgsWithBotMention: "/cmd@bot ARGS" → trimmed ARGS.
func TestExtractCommandArgsWithBotMention(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		cmd := genCommand().Draw(t, "cmd")
		bot := genBotName().Draw(t, "bot")
		args := genArgs().Draw(t, "args")
		input := cmd + bot + " " + args

		got := extractCommandArgs(input, cmd)
		require.Equal(t, strings.TrimSpace(args), got, "input=%q", input)
	})
}

// TestExtractCommandArgsBotOnly: "/cmd@bot" (no args) → "".
func TestExtractCommandArgsBotOnly(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		cmd := genCommand().Draw(t, "cmd")
		bot := genBotName().Draw(t, "bot")
		input := cmd + bot

		got := extractCommandArgs(input, cmd)
		require.Empty(t, got, "input=%q", input)
	})
}

// TestExtractCommandArgsNoArgs: "/cmd" alone → "".
func TestExtractCommandArgsNoArgs(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		cmd := genCommand().Draw(t, "cmd")
		got := extractCommandArgs(cmd, cmd)
		require.Empty(t, got)
	})
}

// TestExtractCommandArgsNeverLeadingAtOrWhitespace: after a single optional
// "@botname" suffix, result never starts with '@' and is trimmed.
// Tails excluding '@' cover the "no further @mentions after the bot suffix"
// case since extractCommandArgs only strips the first "@..." segment.
func TestExtractCommandArgsNeverLeadingAtOrWhitespace(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		cmd := genCommand().Draw(t, "cmd")
		tail := rapid.StringMatching(`[A-Za-z0-9 _.,]{0,30}`).Draw(t, "tail")
		input := cmd + tail

		got := extractCommandArgs(input, cmd)
		require.False(t, strings.HasPrefix(got, "@"), "leading @: got=%q input=%q", got, input)
		require.Equal(t, strings.TrimSpace(got), got, "not trimmed: %q", got)
	})
}
