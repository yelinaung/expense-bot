package bot

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

const (
	foodDiningOutCatEdge     = "Food - Dining Out"
	foodGroceryCatEdge       = "Food - Grocery"
	transportationBusCatEdge = "Transportation - Bus"
	travelVacationCatEdge    = "Travel & Vacation"
)

// TestMatchCategory_EdgeCases tests additional edge cases for category matching.
func TestMatchCategory_EdgeCases(t *testing.T) {
	t.Parallel()

	categories := []models.Category{
		{ID: 1, Name: foodDiningOutCatEdge},
		{ID: 2, Name: foodGroceryCatEdge},
		{ID: 3, Name: transportationBusCatEdge},
		{ID: 4, Name: "Transportation - Taxi"},
		{ID: 5, Name: "Entertainment"},
		{ID: 6, Name: "Others"},
		{ID: 7, Name: travelVacationCatEdge},
		{ID: 8, Name: "Health and Wellness"},
	}

	tests := []struct {
		name      string
		suggested string
		wantNil   bool
		wantCatID int
		wantName  string
	}{
		{
			name:      "suggested with leading spaces",
			suggested: "   Food - Dining Out",
			wantCatID: 1,
			wantName:  foodDiningOutCatEdge,
		},
		{
			name:      "suggested with trailing spaces",
			suggested: "Food - Dining Out   ",
			wantCatID: 1,
			wantName:  foodDiningOutCatEdge,
		},
		{
			name:      "suggested with mixed case and spaces",
			suggested: "  EnTeRtAiNmEnT  ",
			wantCatID: 5,
			wantName:  "Entertainment",
		},
		{
			name:      "suggested with unicode characters",
			suggested: "Café Dining",
			wantCatID: 1, // Matches "Dining" in foodDiningOutCatEdge
			wantName:  foodDiningOutCatEdge,
		},
		{
			name:      "suggested with emoji",
			suggested: "🚌 Transportation",
			wantCatID: 3, // Matches transportationBusCatEdge
			wantName:  transportationBusCatEdge,
		},
		{
			name:      "very long suggested category",
			suggested: "This is a very long category name that doesn't match anything",
			wantNil:   true,
		},
		{
			name:      "suggested with numbers no match",
			suggested: "Food123",
			wantNil:   true, // "Food123" doesn't match "food" exactly
		},
		{
			name:      "suggested is substring of multiple categories",
			suggested: "Transportation",
			wantCatID: 3, // Should match transportationBusCatEdge (first/shortest)
			wantName:  transportationBusCatEdge,
		},
		{
			name:      "suggested with multiple spaces",
			suggested: "Food    Dining    Out",
			wantCatID: 1,
			wantName:  foodDiningOutCatEdge,
		},
		{
			name:      "two characters matches via substring",
			suggested: "Fo",
			wantCatID: 2, // Matches "Food" in foodGroceryCatEdge
			wantName:  foodGroceryCatEdge,
		},
		{
			name:      "three characters matches",
			suggested: "bus",
			wantCatID: 3,
			wantName:  transportationBusCatEdge,
		},
		{
			name:      "exact match takes precedence over contains",
			suggested: "Others",
			wantCatID: 6,
			wantName:  "Others",
		},
		{
			name:      "suggested with parentheses",
			suggested: "Food (Dining)",
			wantCatID: 1,
			wantName:  foodDiningOutCatEdge,
		},
		{
			name:      "suggested with brackets",
			suggested: "Food [Dining]",
			wantCatID: 1,
			wantName:  foodDiningOutCatEdge,
		},
		{
			name:      "reverse match - longer suggested",
			suggested: "Going to Transportation - Bus station",
			wantCatID: 3,
			wantName:  transportationBusCatEdge,
		},
		{
			name:      "word match with multiple words",
			suggested: "health care wellness",
			wantCatID: 8, // Matches "health" and "wellness"
			wantName:  "Health and Wellness",
		},
		{
			name:      "suggested has only stop words",
			suggested: "and the for",
			wantNil:   true,
		},
		{
			name:      "suggested with ampersand matches",
			suggested: "Travel&Vacation",
			wantCatID: 7, // Ampersand gets normalized
			wantName:  travelVacationCatEdge,
		},
		{
			name:      "suggested with slash",
			suggested: "Travel/Vacation",
			wantCatID: 7,
			wantName:  travelVacationCatEdge,
		},
		{
			name:      "multiple category matches - picks first",
			suggested: "food",
			wantCatID: 2, // Should pick foodGroceryCatEdge (shorter)
			wantName:  foodGroceryCatEdge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := MatchCategory(tt.suggested, categories)

			if tt.wantNil {
				require.Nil(t, result, "expected nil match for %q", tt.suggested)
				return
			}

			require.NotNil(t, result, "expected match for %q", tt.suggested)
			require.Equal(t, tt.wantCatID, result.ID, "category ID mismatch for %q", tt.suggested)
			require.Equal(t, tt.wantName, result.Name, "category name mismatch for %q", tt.suggested)
		})
	}
}

// TestExtractSignificantWords_EdgeCases tests additional edge cases for word extraction.
func TestExtractSignificantWords_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "only spaces",
			input: "     ",
			want:  nil,
		},
		{
			name:  "only separators",
			input: "- / & -",
			want:  nil,
		},
		{
			name:  "only stop words",
			input: "and the for",
			want:  nil,
		},
		{
			name:  "mix of stop words and significant",
			input: "the food and the dining",
			want:  []string{"food", "dining"},
		},
		{
			name:  "single significant word",
			input: "food",
			want:  []string{"food"},
		},
		{
			name:  "words with numbers",
			input: "food123 dining456",
			want:  []string{"food123", "dining456"},
		},
		{
			name:  "unicode characters",
			input: "café français",
			want:  []string{"café", "français"},
		},
		{
			name:  "mixed separators",
			input: "food-dining/out&more",
			want:  []string{"food", "dining", "out", "more"},
		},
		{
			name:  "multiple consecutive separators",
			input: "food---dining///out&&&more",
			want:  []string{"food", "dining", "out", "more"},
		},
		{
			name:  "words shorter than 3 chars filtered",
			input: "a ab abc food",
			want:  []string{"abc", "food"},
		},
		{
			name:  "exactly 3 chars included",
			input: "foo bar baz",
			want:  []string{"foo", "bar", "baz"},
		},
		{
			name:  "with tabs and newlines",
			input: "food\tdining\nout",
			want:  []string{"food", "dining", "out"},
		},
		{
			name:  "uppercase gets lowercased",
			input: "FOOD DINING OUT",
			want:  []string{"food", "dining", "out"},
		},
		{
			name:  "mixed case preserved as lowercase",
			input: "FoOd DiNiNg OuT",
			want:  []string{"food", "dining", "out"},
		},
		{
			name:  "special characters in words",
			input: "food! dining? out.",
			want:  []string{"food!", "dining?", "out."},
		},
		{
			name:  "parentheses and brackets",
			input: "food (dining) [out]",
			want:  []string{"food", "(dining)", "[out]"},
		},
		{
			name:  "with multiple spaces between words",
			input: "food    dining    out",
			want:  []string{"food", "dining", "out"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := extractSignificantWords(tt.input)
			require.Equal(t, tt.want, result, "mismatch for input %q", tt.input)
		})
	}
}

// TestIsStopWord_EdgeCases tests additional edge cases for stop word detection.
func TestIsStopWord_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		word string
		want bool
	}{
		{
			name: "empty string",
			word: "",
			want: false,
		},
		{
			name: "uppercase AND",
			word: "AND",
			want: false, // Case sensitive
		},
		{
			name: "uppercase THE",
			word: "THE",
			want: false,
		},
		{
			name: "mixed case And",
			word: "And",
			want: false,
		},
		{
			name: "with spaces",
			word: " and ",
			want: false, // Won't match due to spaces
		},
		{
			name: "similar but not stop word",
			word: "andy",
			want: false,
		},
		{
			name: "similar but not stop word",
			word: "them",
			want: false,
		},
		{
			name: "single letter",
			word: "a",
			want: false,
		},
		{
			name: "two letters",
			word: "an",
			want: false,
		},
		{
			name: "actual stop word and",
			word: "and",
			want: true,
		},
		{
			name: "actual stop word the",
			word: "the",
			want: true,
		},
		{
			name: "actual stop word for",
			word: "for",
			want: true,
		},
		{
			name: "not a stop word",
			word: "food",
			want: false,
		},
		{
			name: "not a stop word",
			word: "transportation",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isStopWord(tt.word)
			require.Equal(t, tt.want, result, "mismatch for word %q", tt.word)
		})
	}
}

// TestMatchCategory_MultipleMatches tests scenarios with multiple potential matches.
func TestMatchCategory_MultipleMatches(t *testing.T) {
	t.Parallel()

	categories := []models.Category{
		{ID: 1, Name: "Food"},
		{ID: 2, Name: "Food - Dining"},
		{ID: 3, Name: foodDiningOutCatEdge},
		{ID: 4, Name: "Food - Dining Out - Restaurant"},
	}

	tests := []struct {
		name      string
		suggested string
		wantCatID int
		wantName  string
	}{
		{
			name:      "exact match preferred",
			suggested: "Food",
			wantCatID: 1,
			wantName:  "Food",
		},
		{
			name:      "exact match with spaces",
			suggested: foodDiningOutCatEdge,
			wantCatID: 3,
			wantName:  foodDiningOutCatEdge,
		},
		{
			name:      "shortest contains match",
			suggested: "dining",
			wantCatID: 2, // "Food - Dining" is shortest containing "dining"
			wantName:  "Food - Dining",
		},
		{
			name:      "partial word matches shortest",
			suggested: "food",
			wantCatID: 1, // Exact match to shortest "Food"
			wantName:  "Food",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := MatchCategory(tt.suggested, categories)
			require.NotNil(t, result)
			require.Equal(t, tt.wantCatID, result.ID)
			require.Equal(t, tt.wantName, result.Name)
		})
	}
}
