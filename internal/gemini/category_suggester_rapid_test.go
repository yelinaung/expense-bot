package gemini

import (
	"encoding/hex"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/models"
	"pgregory.net/rapid"
)

// TestSanitizeForPromptRemovesQuotesAndNulls: output contains no `"`, backtick, or NUL.
func TestSanitizeForPromptRemovesQuotesAndNulls(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		in := rapid.String().Draw(t, "in")
		maxLen := rapid.IntRange(1, 500).Draw(t, "maxLen")
		got := SanitizeForPrompt(in, maxLen)
		require.NotContains(t, got, `"`, "double quote in %q", got)
		require.NotContains(t, got, "`", "backtick in %q", got)
		require.NotContains(t, got, "\x00", "nul in %q", got)
	})
}

// TestSanitizeForPromptLengthCapped: len(output) <= maxLen.
func TestSanitizeForPromptLengthCapped(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		in := rapid.String().Draw(t, "in")
		maxLen := rapid.IntRange(1, 500).Draw(t, "maxLen")
		got := SanitizeForPrompt(in, maxLen)
		require.LessOrEqual(t, len(got), maxLen, "len=%d maxLen=%d got=%q", len(got), maxLen, got)
	})
}

// TestSanitizeForPromptNoInternalWhitespaceRuns: no consecutive whitespace and no
// leading/trailing whitespace. Normalization collapses runs via strings.Fields.
func TestSanitizeForPromptNoInternalWhitespaceRuns(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		in := rapid.String().Draw(t, "in")
		maxLen := rapid.IntRange(10, 500).Draw(t, "maxLen")
		got := SanitizeForPrompt(in, maxLen)
		require.Equal(t, strings.TrimSpace(got), got, "not trimmed: %q", got)
		prevSpace := false
		for _, r := range got {
			isSpace := unicode.IsSpace(r)
			require.False(t, prevSpace && isSpace, "consecutive whitespace in %q", got)
			prevSpace = isSpace
		}
	})
}

// TestSanitizeCategoryNameLengthCap: len(output) <= MaxCategoryNameLength.
func TestSanitizeCategoryNameLengthCap(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		in := rapid.String().Draw(t, "in")
		got := SanitizeCategoryName(in)
		require.LessOrEqual(t, len(got), models.MaxCategoryNameLength)
	})
}

// TestExtractJSONMissingBraceEmpty: input with no '{' yields "".
func TestExtractJSONMissingBraceEmpty(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		in := rapid.StringMatching(`[A-Za-z0-9 ]{0,40}`).Draw(t, "in")
		got := extractJSON(in)
		require.Empty(t, got, "in=%q", in)
	})
}

// TestExtractJSONRoundsToBraces: when '{' exists and '}' exists after it, result
// starts with '{' and ends with '}'.
func TestExtractJSONRoundsToBraces(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		prefix := rapid.StringMatching(`[A-Za-z ]{0,10}`).Draw(t, "prefix")
		body := rapid.StringMatching(`[A-Za-z0-9 :",]{0,40}`).Draw(t, "body")
		suffix := rapid.StringMatching(`[A-Za-z ]{0,10}`).Draw(t, "suffix")
		in := prefix + "{" + body + "}" + suffix

		got := extractJSON(in)
		require.NotEmpty(t, got)
		require.Equal(t, byte('{'), got[0])
		require.Equal(t, byte('}'), got[len(got)-1])
	})
}

// TestHashDescriptionDeterministic: same input → same output; output is 16 hex chars.
func TestHashDescriptionDeterministic(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		in := rapid.String().Draw(t, "in")
		a := hashDescription(in)
		b := hashDescription(in)
		require.Equal(t, a, b, "not deterministic")
		require.Len(t, a, 16, "expected 16 hex chars")
		_, err := hex.DecodeString(a)
		require.NoError(t, err, "not valid hex: %q", a)
	})
}

// TestSanitizeAvailableCategoriesDeduplicatesCaseInsensitively:
// output has no two entries equal after ToLower.
func TestSanitizeAvailableCategoriesDeduplicatesCaseInsensitively(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 10).Draw(t, "n")
		input := make([]string, n)
		for i := range n {
			input[i] = rapid.StringMatching(`[A-Za-z0-9 \-]{0,20}`).Draw(t, "cat")
		}
		out := sanitizeAvailableCategories(input)

		seen := map[string]bool{}
		for _, c := range out {
			require.NotEmpty(t, c, "empty entry slipped through")
			key := strings.ToLower(c)
			require.False(t, seen[key], "duplicate: %q", c)
			seen[key] = true
		}
		require.LessOrEqual(t, len(out), len(input))
	})
}
