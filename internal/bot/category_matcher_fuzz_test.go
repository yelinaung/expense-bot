package bot

import (
	"strings"
	"testing"

	"gitlab.com/yelinaung/expense-bot/internal/models"
)

func FuzzMatchCategory(f *testing.F) {
	// suggested, plus three category names.
	f.Add("Food", "Food", "Transport", "Shopping")
	f.Add("dining", "Food - Dining Out", "Transport", "Entertainment")
	f.Add("food and drinks", "Food", "Drinks", "Groceries")
	f.Add("", "Food", "Transport", "Shopping")
	f.Add("   ", "Food", "", "")
	f.Add("FOOD", "food", "Food", "FOOD")
	f.Add("café", "Café", "Coffee", "Restaurants")
	f.Add("the for and", "Theater", "Fortune", "Android")
	f.Add("a/b-c&d", "b", "c", "d")
	f.Add("\x00", "\x00", "a", "b")

	f.Fuzz(func(t *testing.T, suggested, cat1, cat2, cat3 string) {
		categories := []models.Category{
			{ID: 1, Name: cat1},
			{ID: 2, Name: cat2},
			{ID: 3, Name: cat3},
		}

		result := MatchCategory(suggested, categories)

		// Invariant 1: whitespace-only suggestions never match.
		if strings.TrimSpace(suggested) == "" {
			if result != nil {
				t.Errorf("MatchCategory(%q) = %v, want nil for blank input", suggested, result)
			}
			return
		}

		// Invariant 2: a result must point into the input slice, unmodified.
		if result != nil {
			found := false
			for i := range categories {
				if result == &categories[i] {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("MatchCategory(%q) returned a category not from the input slice: %+v", suggested, result)
			}
		}

		// Invariant 3: an exact (case-insensitive) name match always wins.
		for i := range categories {
			if strings.EqualFold(categories[i].Name, suggested) {
				if result == nil || !strings.EqualFold(result.Name, suggested) {
					t.Errorf("MatchCategory(%q) = %v, want exact match %q", suggested, result, categories[i].Name)
				}
				break
			}
		}
	})
}

func FuzzExtractSignificantWords(f *testing.F) {
	f.Add("Food - Dining Out")
	f.Add("food and drinks")
	f.Add("the for and")
	f.Add("a/b-c&d")
	f.Add("")
	f.Add("   ")
	f.Add("Café & Restaurants")
	f.Add("one two-three/four&five")
	f.Add("UPPER lower MiXeD")
	f.Add("\ttabs\nnewlines\r")

	f.Fuzz(func(t *testing.T, input string) {
		words := extractSignificantWords(input)

		for _, w := range words {
			// Invariant 1: words are at least 3 bytes long.
			if len(w) < 3 {
				t.Errorf("extractSignificantWords(%q) produced short word %q", input, w)
			}
			// Invariant 2: stop words are filtered out.
			if isStopWord(w) {
				t.Errorf("extractSignificantWords(%q) produced stop word %q", input, w)
			}
			// Invariant 3: words are lowercased.
			if w != strings.ToLower(w) {
				t.Errorf("extractSignificantWords(%q) produced non-lowercase word %q", input, w)
			}
			// Invariant 4: separators and whitespace are never inside a word.
			if strings.ContainsAny(w, "-/& \t\n\r") {
				t.Errorf("extractSignificantWords(%q) produced word with separator: %q", input, w)
			}
		}
	})
}
