package gemini

import (
	"encoding/hex"
	"math"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/models"
	"hegel.dev/go/hegel"
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

// TestExtractJSONOpenBraceNoCloseEmpty: input with '{' but no matching '}'
// returns "". This pins the LLM-output safety contract: truncated or
// malformed responses must not slip past extractJSON as a valid payload.
func TestExtractJSONOpenBraceNoCloseEmpty(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		prefix := rapid.StringMatching(`[A-Za-z ]{0,10}`).Draw(t, "prefix")
		body := rapid.StringMatching(`[A-Za-z0-9 :",]{0,40}`).Draw(t, "body")
		in := prefix + "{" + body
		got := extractJSON(in)
		require.Empty(t, got, "in=%q", in)
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

// TestHegelSanitizeForPromptRemovesQuotesAndNulls is the Hegel equivalent: the
// output contains no double quote, backtick, or NUL, over full-Unicode input.
// maxLength is drawn from the full non-negative contract domain (negatives
// would panic on input[:maxLength] and are out of the documented contract).
func TestHegelSanitizeForPromptRemovesQuotesAndNulls(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		in := hegel.Draw(ht, hegel.Text())
		maxLen := hegel.Draw(ht, hegel.Integers(0, math.MaxInt))
		got := SanitizeForPrompt(in, maxLen)
		require.NotContains(ht, got, `"`, "double quote in %q", got)
		require.NotContains(ht, got, "`", "backtick in %q", got)
		require.NotContains(ht, got, "\x00", "nul in %q", got)
	})
}

// TestHegelSanitizeForPromptLengthCapped is the Hegel equivalent: len(output)
// <= maxLen.
func TestHegelSanitizeForPromptLengthCapped(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		in := hegel.Draw(ht, hegel.Text())
		maxLen := hegel.Draw(ht, hegel.Integers(0, math.MaxInt))
		got := SanitizeForPrompt(in, maxLen)
		require.LessOrEqual(ht, len(got), maxLen, "len=%d maxLen=%d got=%q", len(got), maxLen, got)
	})
}

// TestHegelSanitizeForPromptNoInternalWhitespaceRuns is the Hegel equivalent:
// no consecutive whitespace and no leading/trailing whitespace.
func TestHegelSanitizeForPromptNoInternalWhitespaceRuns(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		in := hegel.Draw(ht, hegel.Text())
		maxLen := hegel.Draw(ht, hegel.Integers(0, math.MaxInt))
		got := SanitizeForPrompt(in, maxLen)
		require.Equal(ht, strings.TrimSpace(got), got, "not trimmed: %q", got)
		prevSpace := false
		for _, r := range got {
			isSpace := unicode.IsSpace(r)
			require.False(ht, prevSpace && isSpace, "consecutive whitespace in %q", got)
			prevSpace = isSpace
		}
	})
}

// TestHegelSanitizeCategoryNameLengthCap is the Hegel equivalent: len(output)
// <= MaxCategoryNameLength.
func TestHegelSanitizeCategoryNameLengthCap(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		in := hegel.Draw(ht, hegel.Text())
		got := SanitizeCategoryName(in)
		require.LessOrEqual(ht, len(got), models.MaxCategoryNameLength)
	})
}

// TestHegelExtractJSONMissingBraceEmpty is the Hegel equivalent: input with no
// '{' yields "".
func TestHegelExtractJSONMissingBraceEmpty(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		in := hegel.Draw(ht, hegel.FromRegex(`[A-Za-z0-9 ]{0,40}`, true))
		got := extractJSON(in)
		require.Empty(ht, got, "in=%q", in)
	})
}

// TestHegelExtractJSONRoundsToBraces is the Hegel equivalent: when '{' exists
// and '}' exists after it, result starts with '{' and ends with '}'.
func TestHegelExtractJSONRoundsToBraces(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		prefix := hegel.Draw(ht, hegel.FromRegex(`[A-Za-z ]{0,10}`, true))
		body := hegel.Draw(ht, hegel.FromRegex(`[A-Za-z0-9 :",]{0,40}`, true))
		suffix := hegel.Draw(ht, hegel.FromRegex(`[A-Za-z ]{0,10}`, true))
		in := prefix + "{" + body + "}" + suffix

		got := extractJSON(in)
		require.NotEmpty(ht, got)
		require.Equal(ht, byte('{'), got[0])
		require.Equal(ht, byte('}'), got[len(got)-1])
	})
}

// TestHegelExtractJSONOpenBraceNoCloseEmpty is the Hegel equivalent: input with
// '{' but no matching '}' returns "".
func TestHegelExtractJSONOpenBraceNoCloseEmpty(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		prefix := hegel.Draw(ht, hegel.FromRegex(`[A-Za-z ]{0,10}`, true))
		body := hegel.Draw(ht, hegel.FromRegex(`[A-Za-z0-9 :",]{0,40}`, true))
		in := prefix + "{" + body
		got := extractJSON(in)
		require.Empty(ht, got, "in=%q", in)
	})
}

// TestHegelHashDescriptionDeterministic is the Hegel equivalent: same input
// yields the same 16-hex-char output.
func TestHegelHashDescriptionDeterministic(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		in := hegel.Draw(ht, hegel.Text())
		a := hashDescription(in)
		b := hashDescription(in)
		require.Equal(ht, a, b, "not deterministic")
		require.Len(ht, a, 16, "expected 16 hex chars")
		_, err := hex.DecodeString(a)
		require.NoError(ht, err, "not valid hex: %q", a)
	})
}

// TestHegelSanitizeAvailableCategoriesDeduplicatesCaseInsensitively is the Hegel
// equivalent: output has no two entries equal after ToLower.
func TestHegelSanitizeAvailableCategoriesDeduplicatesCaseInsensitively(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		n := hegel.Draw(ht, hegel.Integers(0, 10))
		input := make([]string, n)
		for i := range n {
			input[i] = hegel.Draw(ht, hegel.FromRegex(`[A-Za-z0-9 \-]{0,20}`, true))
		}
		out := sanitizeAvailableCategories(input)

		seen := map[string]bool{}
		for _, c := range out {
			require.NotEmpty(ht, c, "empty entry slipped through")
			key := strings.ToLower(c)
			require.False(ht, seen[key], "duplicate: %q", c)
			seen[key] = true
		}
		require.LessOrEqual(ht, len(out), len(input))
	})
}
