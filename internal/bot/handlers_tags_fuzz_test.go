package bot

import (
	"strings"
	"testing"

	"gitlab.com/yelinaung/expense-bot/internal/models"
)

func FuzzExtractTags_Standalone(f *testing.F) {
	// This is a focused fuzz test just for extractTags tag validation.
	f.Add("#work")
	f.Add("Coffee #work")
	f.Add("")

	f.Fuzz(func(t *testing.T, input string) {
		tags, _ := extractTags(input)
		for _, tag := range tags {
			if !isValidTagName(tag) {
				t.Errorf("extractTags(%q) produced tag that fails isValidTagName: %q", input, tag)
			}
		}
	})
}

func FuzzIsValidTagName(f *testing.F) {
	// Valid names.
	f.Add("work")
	f.Add("a")
	f.Add("client_meeting")
	f.Add("q2budget")
	f.Add("A")
	f.Add("Work")

	// Invalid names.
	f.Add("123")
	f.Add("")
	f.Add("a b")
	f.Add("-tag")
	f.Add("tag!")
	f.Add("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") // 33 chars, over limit.

	// Control chars.
	f.Add("tag\x00")
	f.Add("tag\n")
	f.Add("tag\t")

	// Unicode.
	f.Add("café")
	f.Add("タグ")

	f.Fuzz(func(t *testing.T, name string) {
		result := isValidTagName(name)

		if result {
			// Invariant 1: If valid, length must be ≤ MaxTagNameLength.
			if len(name) > models.MaxTagNameLength {
				t.Errorf("isValidTagName(%q) = true, but len=%d > MaxTagNameLength=%d", name, len(name), models.MaxTagNameLength)
			}

			// Invariant 2: If valid, must match the regex.
			if !validTagNameRegex.MatchString(name) {
				t.Errorf("isValidTagName(%q) = true, but doesn't match regex", name)
			}
		} else if validTagNameRegex.MatchString(name) && len(name) <= models.MaxTagNameLength {
			// Invariant 3: If invalid, must NOT match the regex OR exceeds length.
			t.Errorf("isValidTagName(%q) = false, but matches regex and within length", name)
		}
	})
}

func FuzzEscapeHTML(f *testing.F) {
	// Normal text.
	f.Add("hello")
	f.Add("work")
	f.Add("")
	f.Add("café")

	// HTML special chars.
	f.Add("<script>alert(1)</script>")
	f.Add("<b>bold</b>")
	f.Add("a&b")
	f.Add("a > b")
	f.Add("a < b")

	// Nested/pre-escaped.
	f.Add("&amp;")
	f.Add("&lt;script&gt;")
	f.Add("&&&&")

	// Edge cases.
	f.Add("<")
	f.Add(">")
	f.Add("&")
	f.Add("<<<>>>")
	f.Add(strings.Repeat("<script>", 100))

	// Unicode.
	f.Add("コーヒー")
	f.Add("☕🍕")

	f.Fuzz(func(t *testing.T, input string) {
		result := escapeHTML(input)

		// Invariant 1: Result must not contain unescaped < or > characters.
		// After escaping, any literal < or > means the function failed.
		temp := strings.ReplaceAll(result, "&lt;", "")
		temp = strings.ReplaceAll(temp, "&gt;", "")
		temp = strings.ReplaceAll(temp, "&amp;", "")
		if strings.ContainsAny(temp, "<>") {
			t.Errorf("escapeHTML(%q) contains unescaped < or >: %q", input, result)
		}

		// Invariant 2: Result must not contain bare & that is not part of &amp;, &lt;, or &gt;.
		check := result
		check = strings.ReplaceAll(check, "&amp;", "\x00")
		check = strings.ReplaceAll(check, "&lt;", "\x00")
		check = strings.ReplaceAll(check, "&gt;", "\x00")
		if strings.Contains(check, "&") {
			t.Errorf("escapeHTML(%q) contains bare &: %q", input, result)
		}
	})
}
