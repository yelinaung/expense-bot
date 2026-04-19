package bot

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestParseTagCommandValidInputsRoundTrip: well-formed "/tag <id> #tag1 #tag2..."
// returns the id, the tag slice verbatim, and an empty error message.
func TestParseTagCommandValidInputsRoundTrip(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		id := rapid.Int64Range(1, 1_000_000).Draw(t, "id")
		n := rapid.IntRange(1, 5).Draw(t, "n")
		tags := make([]string, n)
		for i := range n {
			name := rapid.StringMatching(`[A-Za-z][A-Za-z0-9_]{0,9}`).Draw(t, "tag")
			tags[i] = "#" + name
		}
		text := "/tag " + strconv.FormatInt(id, 10) + " " + strings.Join(tags, " ")

		gotID, gotTags, gotErr := parseTagCommand(text)
		require.Empty(t, gotErr, "text=%q", text)
		require.Equal(t, id, gotID)
		require.Equal(t, tags, gotTags)
	})
}

// TestParseTagCommandMissingArgsErrors: "/tag" alone or with whitespace only
// returns an error message and zero id.
func TestParseTagCommandMissingArgsErrors(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		pad := rapid.StringMatching(`[ \t]*`).Draw(t, "pad")
		text := "/tag" + pad

		gotID, gotTags, gotErr := parseTagCommand(text)
		require.Zero(t, gotID)
		require.Nil(t, gotTags)
		require.NotEmpty(t, gotErr)
	})
}

// TestParseTagCommandMissingTagsErrors: "/tag <id>" without any tag tokens
// returns an error and zero id.
func TestParseTagCommandMissingTagsErrors(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		id := rapid.Int64Range(1, 1_000_000).Draw(t, "id")
		text := "/tag " + strconv.FormatInt(id, 10)

		gotID, gotTags, gotErr := parseTagCommand(text)
		require.Zero(t, gotID)
		require.Nil(t, gotTags)
		require.NotEmpty(t, gotErr)
	})
}

// TestParseTagCommandInvalidIDErrors: first arg not an int64 → error, zero id.
func TestParseTagCommandInvalidIDErrors(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		// non-numeric first token; require at least one non-digit char
		garbage := rapid.StringMatching(`[A-Za-z]{1,8}`).Draw(t, "garbage")
		text := "/tag " + garbage + " #foo"

		gotID, gotTags, gotErr := parseTagCommand(text)
		require.Zero(t, gotID)
		require.Nil(t, gotTags)
		require.NotEmpty(t, gotErr)
	})
}
