package bot

import (
	"strings"
	"testing"

	"gitlab.com/yelinaung/expense-bot/internal/models"
	"pgregory.net/rapid"
)

// genCategoryName generates a category name of 1..3 words from letters.
func genCategoryName() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		n := rapid.IntRange(1, 3).Draw(t, "n")
		words := make([]string, n)
		for i := range n {
			words[i] = rapid.StringMatching(`[A-Za-z]{3,10}`).Draw(t, "word")
		}
		return strings.Join(words, " ")
	})
}

// genCategories generates 1..6 unique-name categories.
func genCategories() *rapid.Generator[[]models.Category] {
	return rapid.Custom(func(t *rapid.T) []models.Category {
		n := rapid.IntRange(1, 6).Draw(t, "n")
		seen := map[string]bool{}
		var cats []models.Category
		attempts := 0
		for len(cats) < n && attempts < n*4 {
			name := genCategoryName().Draw(t, "name")
			key := strings.ToLower(name)
			if seen[key] {
				attempts++
				continue
			}
			seen[key] = true
			cats = append(cats, models.Category{ID: len(cats) + 1, Name: name})
			attempts++
		}
		return cats
	})
}

// TestMatchCategoryEmptyInputReturnsNil: blank suggested → nil.
func TestMatchCategoryEmptyInputReturnsNil(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cats := genCategories().Draw(t, "cats")
		spaces := rapid.StringMatching(`[ \t]*`).Draw(t, "spaces")
		got := MatchCategory(spaces, cats)
		if got != nil {
			t.Fatalf("MatchCategory(%q) = %v, want nil", spaces, got)
		}
	})
}

// TestMatchCategoryEmptyCategoriesReturnsNil: no categories → nil.
func TestMatchCategoryEmptyCategoriesReturnsNil(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.StringMatching(`[A-Za-z ]{1,20}`).Draw(t, "s")
		got := MatchCategory(s, nil)
		if got != nil {
			t.Fatalf("MatchCategory(%q, nil) = %v, want nil", s, got)
		}
	})
}

// TestMatchCategoryExactCaseInsensitive: exact (case-insensitive) name always matches.
func TestMatchCategoryExactCaseInsensitive(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cats := genCategories().Draw(t, "cats")
		idx := rapid.IntRange(0, len(cats)-1).Draw(t, "idx")
		target := cats[idx].Name
		upper := rapid.Bool().Draw(t, "upper")
		suggested := target
		if upper {
			suggested = strings.ToUpper(target)
		} else {
			suggested = strings.ToLower(target)
		}

		got := MatchCategory(suggested, cats)
		if got == nil {
			t.Fatalf("MatchCategory(%q) = nil, want match", suggested)
		}
		if !strings.EqualFold(got.Name, target) {
			t.Fatalf("MatchCategory(%q) = %q, want equalfold %q", suggested, got.Name, target)
		}
	})
}

// TestMatchCategoryDeterministic: same inputs → same result.
func TestMatchCategoryDeterministic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cats := genCategories().Draw(t, "cats")
		s := rapid.StringMatching(`[A-Za-z ]{1,20}`).Draw(t, "s")
		a := MatchCategory(s, cats)
		b := MatchCategory(s, cats)
		switch {
		case a == nil && b == nil:
			return
		case a == nil || b == nil:
			t.Fatalf("nondeterministic: %v vs %v", a, b)
		case a.ID != b.ID:
			t.Fatalf("nondeterministic match id: %d vs %d", a.ID, b.ID)
		}
	})
}

// TestExtractSignificantWordsFiltersStopAndShort: all words ≥3 chars, none are stop words,
// contain no separator chars.
func TestExtractSignificantWordsFiltersStopAndShort(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.StringMatching(`[A-Za-z &/\- ]{0,40}`).Draw(t, "s")
		words := extractSignificantWords(s)
		for _, w := range words {
			if len(w) < 3 {
				t.Fatalf("short word: %q (input=%q)", w, s)
			}
			if isStopWord(w) {
				t.Fatalf("stop word returned: %q", w)
			}
			if strings.ContainsAny(w, "-/&") {
				t.Fatalf("word contains separator: %q", w)
			}
			if w != strings.ToLower(w) {
				t.Fatalf("word not lowercased: %q", w)
			}
		}
	})
}

// TestIsStopWordFixedSet: only "and", "the", "for" are stop words.
func TestIsStopWordFixedSet(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.StringMatching(`[a-z]{0,10}`).Draw(t, "s")
		want := s == "and" || s == "the" || s == "for"
		got := isStopWord(s)
		if got != want {
			t.Fatalf("isStopWord(%q) = %v, want %v", s, got, want)
		}
	})
}

// TestMatchCategorySubstringFinds: when suggested is substring of some category
// name (case-insensitive), Match returns non-nil.
func TestMatchCategorySubstringFinds(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cats := genCategories().Draw(t, "cats")
		idx := rapid.IntRange(0, len(cats)-1).Draw(t, "idx")
		target := cats[idx].Name
		// Use a non-empty case-insensitive substring of target.
		if strings.TrimSpace(target) == "" {
			t.Skip("empty target")
		}
		start := rapid.IntRange(0, len(target)-1).Draw(t, "start")
		end := rapid.IntRange(start+1, len(target)).Draw(t, "end")
		sub := strings.TrimSpace(target[start:end])
		if sub == "" {
			t.Skip("empty substring")
		}

		got := MatchCategory(sub, cats)
		if got == nil {
			t.Fatalf("MatchCategory(%q) = nil, expected some category containing it (target=%q)", sub, target)
		}
	})
}
