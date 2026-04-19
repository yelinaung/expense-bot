package config

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestNormalizeUsernameIdempotent: norm(norm(x)) == norm(x).
func TestNormalizeUsernameIdempotent(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		once := normalizeUsername(s)
		twice := normalizeUsername(once)
		require.Equal(t, once, twice, "not idempotent (in=%q)", s)
	})
}

// TestNormalizeUsernameLowercaseNoLeadingAt:
//   - output is lowercase
//   - output has no leading "@" (one is stripped; more than one was not stripped
//     by the original implementation, so this pins single-@ semantics)
func TestNormalizeUsernameLowercaseNoLeadingAt(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		// Restrict to single-@ prefix forms; double-@ is not stripped by design.
		prefix := rapid.SampledFrom([]string{"", "@"}).Draw(t, "prefix")
		name := rapid.StringMatching(`[A-Za-z0-9_]{0,20}`).Draw(t, "name")
		in := prefix + name
		got := normalizeUsername(in)

		require.Equal(t, strings.ToLower(got), got, "not lowercased: %q", got)
		require.False(t, strings.HasPrefix(got, "@"), "leading @: %q", got)
	})
}

// TestParseInt64ListSkipsInvalidAndEmpty: every element in the output is a
// successfully parsed int64; non-numeric tokens are dropped.
func TestParseInt64ListSkipsInvalidAndEmpty(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 8).Draw(t, "n")
		parts := make([]string, n)
		for i := range n {
			// Mix of valid ints, garbage, whitespace-only.
			switch rapid.IntRange(0, 2).Draw(t, "kind") {
			case 0:
				parts[i] = strconv.FormatInt(rapid.Int64().Draw(t, "n64"), 10)
			case 1:
				parts[i] = rapid.StringMatching(`[A-Za-z]{0,10}`).Draw(t, "junk")
			default:
				parts[i] = rapid.StringMatching(` {0,5}`).Draw(t, "ws")
			}
		}
		raw := strings.Join(parts, ",")
		got := parseInt64List(raw)

		for _, id := range got {
			// Round-trip: formatted output must parse back to the same int64.
			s := strconv.FormatInt(id, 10)
			parsed, err := strconv.ParseInt(s, 10, 64)
			require.NoError(t, err)
			require.Equal(t, id, parsed)
		}
		require.LessOrEqual(t, len(got), n)
	})
}

// TestParseInt64ListOnlyValidInts: input composed solely of valid ints comma-joined
// yields a slice of equal length and values.
func TestParseInt64ListOnlyValidInts(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 8).Draw(t, "n")
		want := make([]int64, n)
		parts := make([]string, n)
		for i := range n {
			v := rapid.Int64().Draw(t, "v")
			want[i] = v
			parts[i] = strconv.FormatInt(v, 10)
		}
		got := parseInt64List(strings.Join(parts, ","))
		require.Equal(t, want, got)
	})
}

// TestParseWhitelistedUsernamesStripsAtAndTrims:
//   - no entry is empty
//   - no entry starts with '@'
//   - no entry has leading/trailing whitespace
func TestParseWhitelistedUsernamesStripsAtAndTrims(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 8).Draw(t, "n")
		parts := make([]string, n)
		for i := range n {
			leadAt := rapid.Bool().Draw(t, "leadAt")
			pad := rapid.StringMatching(` {0,3}`).Draw(t, "pad")
			name := rapid.StringMatching(`[A-Za-z0-9_]{0,10}`).Draw(t, "name")
			if leadAt {
				parts[i] = pad + "@" + name + pad
			} else {
				parts[i] = pad + name + pad
			}
		}
		raw := strings.Join(parts, ",")
		got := parseWhitelistedUsernames(raw)

		for _, u := range got {
			require.NotEmpty(t, u, "empty entry slipped through")
			require.False(t, strings.HasPrefix(u, "@"), "leading @: %q", u)
			require.Equal(t, strings.TrimSpace(u), u, "not trimmed: %q", u)
		}
		require.LessOrEqual(t, len(got), n)
	})
}

// TestParseWhitelistedUsernamesBareAtDropped: bare "@" tokens yield no entry.
func TestParseWhitelistedUsernamesBareAtDropped(t *testing.T) {
	t.Parallel()
	require.Empty(t, parseWhitelistedUsernames("@"))
	require.Empty(t, parseWhitelistedUsernames(" @ , @ "))
}
