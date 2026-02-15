package bot

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

const (
	foodDiningOutCatMatch  = "Food - Dining Out"
	foodGroceryCatMatch    = "Food - Grocery"
	healthWellnessCatMatch = "Health and Wellness"
	travelVacationCatMatch = "Travel & Vacation"
)

func TestMatchCategory(t *testing.T) {
	t.Parallel()

	categories := []models.Category{
		{ID: 1, Name: foodDiningOutCatMatch},
		{ID: 2, Name: foodGroceryCatMatch},
		{ID: 3, Name: "Transportation"},
		{ID: 4, Name: "Communication"},
		{ID: 5, Name: "Housing - Mortgage"},
		{ID: 6, Name: "Housing - Others"},
		{ID: 7, Name: "Personal Care"},
		{ID: 8, Name: healthWellnessCatMatch},
		{ID: 9, Name: "Education"},
		{ID: 10, Name: "Entertainment"},
		{ID: 11, Name: "Credit/Debt Payments"},
		{ID: 12, Name: "Others"},
		{ID: 13, Name: "Utilities"},
		{ID: 14, Name: travelVacationCatMatch},
		{ID: 15, Name: "Subscriptions"},
		{ID: 16, Name: "Donations"},
	}

	tests := []struct {
		name      string
		suggested string
		wantNil   bool
		wantCatID int
		wantName  string
	}{
		{
			name:      "exact match",
			suggested: foodDiningOutCatMatch,
			wantCatID: 1,
			wantName:  foodDiningOutCatMatch,
		},
		{
			name:      "exact match case insensitive",
			suggested: "food - dining out",
			wantCatID: 1,
			wantName:  foodDiningOutCatMatch,
		},
		{
			name:      "exact match uppercase",
			suggested: "TRANSPORTATION",
			wantCatID: 3,
			wantName:  "Transportation",
		},
		{
			name:      "contains match - dining",
			suggested: "dining",
			wantCatID: 1,
			wantName:  foodDiningOutCatMatch,
		},
		{
			name:      "contains match - grocery",
			suggested: "grocery",
			wantCatID: 2,
			wantName:  foodGroceryCatMatch,
		},
		{
			name:      "contains match - transport",
			suggested: "transport",
			wantCatID: 3,
			wantName:  "Transportation",
		},
		{
			name:      "contains match - wellness",
			suggested: "wellness",
			wantCatID: 8,
			wantName:  healthWellnessCatMatch,
		},
		{
			name:      "contains match - mortgage",
			suggested: "mortgage",
			wantCatID: 5,
			wantName:  "Housing - Mortgage",
		},
		{
			name:      "word match - travel",
			suggested: "travel",
			wantCatID: 14,
			wantName:  travelVacationCatMatch,
		},
		{
			name:      "word match - vacation",
			suggested: "vacation",
			wantCatID: 14,
			wantName:  travelVacationCatMatch,
		},
		{
			name:      "word match - health",
			suggested: "health",
			wantCatID: 8,
			wantName:  healthWellnessCatMatch,
		},
		{
			name:      "word match - entertainment",
			suggested: "entertainment",
			wantCatID: 10,
			wantName:  "Entertainment",
		},
		{
			name:      "no match - unknown category",
			suggested: "Insurance",
			wantNil:   true,
		},
		{
			name:      "empty string",
			suggested: "",
			wantNil:   true,
		},
		{
			name:      "whitespace only",
			suggested: "   ",
			wantNil:   true,
		},
		{
			name:      "contains match - food",
			suggested: "food",
			wantCatID: 2, // foodGroceryCatMatch is shorter than foodDiningOutCatMatch.
			wantName:  foodGroceryCatMatch,
		},
		{
			name:      "contains match - housing",
			suggested: "housing",
			wantCatID: 6, // "Housing - Others" is shorter than "Housing - Mortgage".
			wantName:  "Housing - Others",
		},
		{
			name:      "reverse contains - suggested has category name",
			suggested: "Restaurant Food - Dining Out expenses",
			wantCatID: 1,
			wantName:  foodDiningOutCatMatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := MatchCategory(tt.suggested, categories)

			if tt.wantNil {
				require.Nil(t, result)
				return
			}

			require.NotNil(t, result, "expected match for %q", tt.suggested)
			require.Equal(t, tt.wantCatID, result.ID)
			require.Equal(t, tt.wantName, result.Name)
		})
	}
}

func TestMatchCategory_EmptyCategories(t *testing.T) {
	t.Parallel()

	result := MatchCategory("Food", nil)
	require.Nil(t, result)

	result = MatchCategory("Food", []models.Category{})
	require.Nil(t, result)
}

func TestExtractSignificantWords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple words",
			input: "Food Dining Out",
			want:  []string{"food", "dining", "out"},
		},
		{
			name:  "with dash separator",
			input: foodDiningOutCatMatch,
			want:  []string{"food", "dining", "out"},
		},
		{
			name:  "with slash separator",
			input: "Credit/Debt Payments",
			want:  []string{"credit", "debt", "payments"},
		},
		{
			name:  "with ampersand",
			input: travelVacationCatMatch,
			want:  []string{"travel", "vacation"},
		},
		{
			name:  "filters stop words",
			input: healthWellnessCatMatch,
			want:  []string{"health", "wellness"},
		},
		{
			name:  "filters short words",
			input: "a to be",
			want:  nil,
		},
		{
			name:  "mixed case",
			input: "PERSONAL Care",
			want:  []string{"personal", "care"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := extractSignificantWords(tt.input)
			require.Equal(t, tt.want, result)
		})
	}
}

func TestIsStopWord(t *testing.T) {
	t.Parallel()

	require.True(t, isStopWord("and"))
	require.True(t, isStopWord("the"))
	require.True(t, isStopWord("for"))
	require.False(t, isStopWord("food"))
	require.False(t, isStopWord("travel"))
}
