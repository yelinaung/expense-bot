package logger

import (
	"testing"
	"unicode/utf8"

	"hegel.dev/go/hegel"
)

// TestHegelSanitizeTextOutputIsValidUTF8 asserts that SanitizeText never
// returns an invalid UTF-8 string for any input. A sanitizer that slices
// the first three *bytes* of user text can split a multi-byte rune and
// emit a dangling continuation byte, which violates this contract.
func TestHegelSanitizeTextOutputIsValidUTF8(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		in := hegel.Draw(ht, hegel.Text())
		out := SanitizeText(in)
		if !utf8.ValidString(out) {
			ht.Fatalf("SanitizeText produced invalid UTF-8: input=%q output=%q", in, out)
		}
	})
}

// TestHegelSanitizeTextLongInputValidUTF8 focuses generation on inputs
// longer than 10 bytes, where SanitizeText switches to its prefix-slicing
// branch (the branch that can split a multi-byte rune).
func TestHegelSanitizeTextLongInputValidUTF8(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		in := hegel.Draw(ht, hegel.Text().MinSize(11))
		out := SanitizeText(in)
		if !utf8.ValidString(out) {
			ht.Fatalf("SanitizeText produced invalid UTF-8: input=%q output=%q", in, out)
		}
	})
}

// TestHegelSanitizeTextPrefixIsRuneAligned asserts that, when the long-input
// branch exposes a prefix, that prefix is a whole number of runes. This is
// the sharper property behind the UTF-8 contract: the slice must not cut a
// rune in half.
func TestHegelSanitizeTextPrefixIsRuneAligned(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		in := hegel.Draw(ht, hegel.Text().MinSize(11))
		out := SanitizeText(in)
		// The long branch formats as "<3-byte prefix>...<N chars>". Locate the
		// prefix between the start of the output and the "..." separator.
		dotIdx := indexOf(out, "...")
		if dotIdx < 0 {
			return // short-input branch took over; nothing to check here.
		}
		prefix := out[:dotIdx]
		if !utf8.ValidString(prefix) {
			ht.Fatalf("SanitizeText prefix splits a rune: input=%q prefix=%q", in, prefix)
		}
	})
}

// indexOf returns the byte index of the first occurrence of substr in s, or
// -1 if absent.
func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
