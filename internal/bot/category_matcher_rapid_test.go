package bot

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/models"
	"hegel.dev/go/hegel"
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
// Guarantees at least one category so draws like IntRange(0, len(cats)-1) are safe.
func genCategories() *rapid.Generator[[]models.Category] {
	return rapid.Custom(func(t *rapid.T) []models.Category {
		n := rapid.IntRange(1, 6).Draw(t, "n")
		seen := map[string]bool{}
		cats := make([]models.Category, 0, n)
		for len(cats) < n {
			name := genCategoryName().Draw(t, "name")
			key := strings.ToLower(name)
			if seen[key] {
				continue
			}
			seen[key] = true
			cats = append(cats, models.Category{ID: len(cats) + 1, Name: name})
		}
		return cats
	})
}

// TestMatchCategoryEmptyInputReturnsNil: blank suggested → nil.
func TestMatchCategoryEmptyInputReturnsNil(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		cats := genCategories().Draw(t, "cats")
		spaces := rapid.StringMatching(`[ \t]*`).Draw(t, "spaces")
		got := MatchCategory(spaces, cats)
		require.Nil(t, got, "MatchCategory(%q)", spaces)
	})
}

// TestMatchCategoryEmptyCategoriesReturnsNil: nil or empty-slice → nil.
func TestMatchCategoryEmptyCategoriesReturnsNil(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		categories []models.Category
	}{
		{"nil", nil},
		{"empty", []models.Category{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rapid.Check(t, func(t *rapid.T) {
				s := rapid.StringMatching(`[A-Za-z ]{1,20}`).Draw(t, "s")
				got := MatchCategory(s, tc.categories)
				require.Nil(t, got, "MatchCategory(%q, %s)", s, tc.name)
			})
		})
	}
}

// TestMatchCategoryExactCaseInsensitive: exact (case-insensitive) name always matches.
func TestMatchCategoryExactCaseInsensitive(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		cats := genCategories().Draw(t, "cats")
		idx := rapid.IntRange(0, len(cats)-1).Draw(t, "idx")
		target := cats[idx].Name
		upper := rapid.Bool().Draw(t, "upper")
		var suggested string
		if upper {
			suggested = strings.ToUpper(target)
		} else {
			suggested = strings.ToLower(target)
		}

		got := MatchCategory(suggested, cats)
		require.NotNil(t, got, "MatchCategory(%q)", suggested)
		require.True(t, strings.EqualFold(got.Name, target),
			"MatchCategory(%q) = %q, want equalfold %q", suggested, got.Name, target)
	})
}

// TestMatchCategoryDeterministic: same inputs → same result.
func TestMatchCategoryDeterministic(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		cats := genCategories().Draw(t, "cats")
		s := rapid.StringMatching(`[A-Za-z ]{1,20}`).Draw(t, "s")
		a := MatchCategory(s, cats)
		b := MatchCategory(s, cats)
		switch {
		case a == nil && b == nil:
			return
		case a == nil || b == nil:
			require.FailNowf(t, "nondeterministic nil",
				"a=%v b=%v", a, b)
		default:
			require.Equal(t, a.ID, b.ID, "nondeterministic match id")
		}
	})
}

// TestExtractSignificantWordsFiltersStopAndShort: all words ≥3 chars, none are stop words,
// contain no separator chars.
func TestExtractSignificantWordsFiltersStopAndShort(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.StringMatching(`[A-Za-z &/\- ]{0,40}`).Draw(t, "s")
		words := extractSignificantWords(s)
		for _, w := range words {
			require.GreaterOrEqual(t, len(w), 3, "short word: %q (input=%q)", w, s)
			require.False(t, isStopWord(w), "stop word returned: %q", w)
			require.False(t, strings.ContainsAny(w, "-/&"), "word contains separator: %q", w)
			require.Equal(t, strings.ToLower(w), w, "word not lowercased: %q", w)
		}
	})
}

// TestIsStopWordFixedSet pins the current stop-word set as a contract: only
// "and", "the", "for" are recognized. Update this test when the stop-word list
// changes — the pin exists so additions or removals are a deliberate, visible
// change rather than a silent behavior shift.
func TestIsStopWordFixedSet(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.StringMatching(`[a-z]{0,10}`).Draw(t, "s")
		want := s == "and" || s == "the" || s == "for"
		got := isStopWord(s)
		require.Equal(t, want, got, "isStopWord(%q)", s)
	})
}

// TestFindWordBasedCategoryMatchNoSignificantWordsReturnsNil: when the
// suggested input contains only stop words or sub-3-character tokens,
// extractSignificantWords returns an empty slice and the word-based matcher
// must not fall back to any accidental hit.
//
// Scoped to findWordBasedCategoryMatch specifically because
// findShortestContainingCategoryMatch can legitimately match on a 1-char
// substring of a cat name (e.g. "a" is a substring of "aaa") — that's not a
// spurious hit, so end-to-end MatchCategory can't assert nil here.
func TestFindWordBasedCategoryMatchNoSignificantWordsReturnsNil(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		cats := genCategories().Draw(t, "cats")
		kind := rapid.IntRange(0, 1).Draw(t, "kind")
		var suggested string
		switch kind {
		case 0:
			// Only stop words, space-separated.
			n := rapid.IntRange(1, 5).Draw(t, "n")
			toks := make([]string, n)
			for i := range n {
				toks[i] = rapid.SampledFrom([]string{"and", "the", "for"}).Draw(t, "stop")
			}
			suggested = strings.Join(toks, " ")
		default:
			// Only sub-3-character letter tokens.
			n := rapid.IntRange(1, 5).Draw(t, "n")
			toks := make([]string, n)
			for i := range n {
				toks[i] = rapid.StringMatching(`[A-Za-z]{1,2}`).Draw(t, "tok")
			}
			suggested = strings.Join(toks, " ")
		}

		require.Empty(t, extractSignificantWords(suggested),
			"precondition: expected no significant words in %q", suggested)
		got := findWordBasedCategoryMatch(suggested, cats)
		require.Nil(t, got, "findWordBasedCategoryMatch(%q)", suggested)
	})
}

// TestMatchCategorySubstringFinds asserts the substring-containment contract:
// when suggested is a non-empty substring of any category name,
// MatchCategory returns some category (not necessarily the target — the
// shortest containing category or first word-based hit is acceptable). This
// pins the "never miss a clear containment hit" behavior without constraining
// which category wins when multiple candidates contain the substring.
func TestMatchCategorySubstringFinds(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		cats := genCategories().Draw(t, "cats")
		idx := rapid.IntRange(0, len(cats)-1).Draw(t, "idx")
		target := cats[idx].Name
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
		require.NotNil(t, got,
			"MatchCategory(%q) expected match (target=%q)", sub, target)
	})
}

// hegelCategoryNameGen is the Hegel analog of genCategoryName: a category name
// of 1..3 words from letters.
func hegelCategoryNameGen() hegel.Generator[string] {
	return hegel.Composite(func(tc hegel.TestCase) string {
		n := hegel.Draw(tc, hegel.Integers(1, 3))
		words := make([]string, n)
		for i := range n {
			words[i] = hegel.Draw(tc, hegel.FromRegex(`[A-Za-z]{3,10}`, true))
		}
		return strings.Join(words, " ")
	})
}

// hegelCategoriesGen is the Hegel analog of genCategories: 1..6 unique-name
// categories. Guarantees at least one category so draws like
// hegel.Integers(0, len(cats)-1) are safe.
func hegelCategoriesGen() hegel.Generator[[]models.Category] {
	return hegel.Composite(func(tc hegel.TestCase) []models.Category {
		n := hegel.Draw(tc, hegel.Integers(1, 6))
		seen := map[string]bool{}
		cats := make([]models.Category, 0, n)
		for len(cats) < n {
			name := hegel.Draw(tc, hegelCategoryNameGen())
			key := strings.ToLower(name)
			if seen[key] {
				continue
			}
			seen[key] = true
			cats = append(cats, models.Category{ID: len(cats) + 1, Name: name})
		}
		return cats
	})
}

// TestHegelMatchCategoryEmptyInputReturnsNil is the Hegel equivalent: blank
// suggested returns nil.
func TestHegelMatchCategoryEmptyInputReturnsNil(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		cats := hegel.Draw(ht, hegelCategoriesGen())
		spaces := hegel.Draw(ht, hegel.FromRegex(`[ \t]*`, true))
		got := MatchCategory(spaces, cats)
		require.Nil(ht, got, "MatchCategory(%q)", spaces)
	})
}

// TestHegelMatchCategoryEmptyCategoriesReturnsNil is the Hegel equivalent: nil
// or empty-slice categories returns nil for any suggested string.
func TestHegelMatchCategoryEmptyCategoriesReturnsNil(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		s := hegel.Draw(ht, hegel.FromRegex(`[A-Za-z ]{1,20}`, true))
		require.Nil(ht, MatchCategory(s, nil), "MatchCategory(%q, nil)", s)
		require.Nil(ht, MatchCategory(s, []models.Category{}), "MatchCategory(%q, empty)", s)
	})
}

// TestHegelMatchCategoryExactCaseInsensitive is the Hegel equivalent: exact
// (case-insensitive) name always matches.
func TestHegelMatchCategoryExactCaseInsensitive(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		cats := hegel.Draw(ht, hegelCategoriesGen())
		idx := hegel.Draw(ht, hegel.Integers(0, len(cats)-1))
		target := cats[idx].Name
		upper := hegel.Draw(ht, hegel.Booleans())
		var suggested string
		if upper {
			suggested = strings.ToUpper(target)
		} else {
			suggested = strings.ToLower(target)
		}

		got := MatchCategory(suggested, cats)
		require.NotNil(ht, got, "MatchCategory(%q)", suggested)
		require.True(ht, strings.EqualFold(got.Name, target),
			"MatchCategory(%q) = %q, want equalfold %q", suggested, got.Name, target)
	})
}

// TestHegelMatchCategoryDeterministic is the Hegel equivalent: same inputs
// yield the same result.
func TestHegelMatchCategoryDeterministic(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		cats := hegel.Draw(ht, hegelCategoriesGen())
		s := hegel.Draw(ht, hegel.FromRegex(`[A-Za-z ]{1,20}`, true))
		a := MatchCategory(s, cats)
		b := MatchCategory(s, cats)
		switch {
		case a == nil && b == nil:
			return
		case a == nil || b == nil:
			ht.Fatalf("nondeterministic nil: a=%v b=%v", a, b)
		default:
			require.Equal(ht, a.ID, b.ID, "nondeterministic match id")
		}
	})
}

// TestHegelExtractSignificantWordsFiltersStopAndShort is the Hegel equivalent:
// all words are >=3 chars, none are stop words, none contain separator chars,
// and all are lowercased.
func TestHegelExtractSignificantWordsFiltersStopAndShort(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		s := hegel.Draw(ht, hegel.FromRegex(`[A-Za-z &/\- ]{0,40}`, true))
		words := extractSignificantWords(s)
		for _, w := range words {
			require.GreaterOrEqual(ht, len(w), 3, "short word: %q (input=%q)", w, s)
			require.False(ht, isStopWord(w), "stop word returned: %q", w)
			require.False(ht, strings.ContainsAny(w, "-/&"), "word contains separator: %q", w)
			require.Equal(ht, strings.ToLower(w), w, "word not lowercased: %q", w)
		}
	})
}

// TestHegelIsStopWordFixedSet is the Hegel equivalent: pins the current
// stop-word set ("and", "the", "for") as a contract.
func TestHegelIsStopWordFixedSet(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		s := hegel.Draw(ht, hegel.FromRegex(`[a-z]{0,10}`, true))
		want := s == "and" || s == "the" || s == "for"
		got := isStopWord(s)
		require.Equal(ht, want, got, "isStopWord(%q)", s)
	})
}

// TestHegelFindWordBasedCategoryMatchNoSignificantWordsReturnsNil is the Hegel
// equivalent: when the suggested input contains only stop words or sub-3-char
// tokens, findWordBasedCategoryMatch returns nil.
func TestHegelFindWordBasedCategoryMatchNoSignificantWordsReturnsNil(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		cats := hegel.Draw(ht, hegelCategoriesGen())
		kind := hegel.Draw(ht, hegel.Integers(0, 1))
		var suggested string
		switch kind {
		case 0:
			n := hegel.Draw(ht, hegel.Integers(1, 5))
			toks := make([]string, n)
			for i := range n {
				toks[i] = hegel.Draw(ht, hegel.SampledFrom([]string{"and", "the", "for"}))
			}
			suggested = strings.Join(toks, " ")
		default:
			n := hegel.Draw(ht, hegel.Integers(1, 5))
			toks := make([]string, n)
			for i := range n {
				toks[i] = hegel.Draw(ht, hegel.FromRegex(`[A-Za-z]{1,2}`, true))
			}
			suggested = strings.Join(toks, " ")
		}

		require.Empty(ht, extractSignificantWords(suggested),
			"precondition: expected no significant words in %q", suggested)
		got := findWordBasedCategoryMatch(suggested, cats)
		require.Nil(ht, got, "findWordBasedCategoryMatch(%q)", suggested)
	})
}

// TestHegelMatchCategorySubstringFinds is the Hegel equivalent: when suggested
// is a non-empty substring of any category name, MatchCategory returns some
// category.
func TestHegelMatchCategorySubstringFinds(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		cats := hegel.Draw(ht, hegelCategoriesGen())
		idx := hegel.Draw(ht, hegel.Integers(0, len(cats)-1))
		target := cats[idx].Name
		ht.Assume(strings.TrimSpace(target) != "")
		start := hegel.Draw(ht, hegel.Integers(0, len(target)-1))
		end := hegel.Draw(ht, hegel.Integers(start+1, len(target)))
		sub := strings.TrimSpace(target[start:end])
		ht.Assume(sub != "")

		got := MatchCategory(sub, cats)
		require.NotNil(ht, got,
			"MatchCategory(%q) expected match (target=%q)", sub, target)
	})
}
