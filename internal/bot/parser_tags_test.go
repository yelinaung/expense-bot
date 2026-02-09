package bot

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractTags(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantTags    []string
		wantCleaned string
	}{
		{
			name:        "single tag",
			input:       "Coffee #work",
			wantTags:    []string{"work"},
			wantCleaned: "Coffee",
		},
		{
			name:        "multiple tags",
			input:       "Coffee #work #meeting",
			wantTags:    []string{"work", "meeting"},
			wantCleaned: "Coffee",
		},
		{
			name:        "deduplication",
			input:       "Coffee #work #work",
			wantTags:    []string{"work"},
			wantCleaned: "Coffee",
		},
		{
			name:        "no tags",
			input:       "Coffee",
			wantTags:    nil,
			wantCleaned: "Coffee",
		},
		{
			name:        "tag only",
			input:       "#work",
			wantTags:    []string{"work"},
			wantCleaned: "",
		},
		{
			name:        "invalid tag starting with digit",
			input:       "Coffee #123",
			wantTags:    nil,
			wantCleaned: "Coffee #123",
		},
		{
			name:        "mixed case tags are lowercased",
			input:       "Coffee #Work #MEETING",
			wantTags:    []string{"work", "meeting"},
			wantCleaned: "Coffee",
		},
		{
			name:        "tag in middle of text",
			input:       "Coffee #work today",
			wantTags:    []string{"work"},
			wantCleaned: "Coffee today",
		},
		{
			name:        "empty string",
			input:       "",
			wantTags:    nil,
			wantCleaned: "",
		},
		{
			name:        "tag with underscores",
			input:       "Lunch #client_meeting",
			wantTags:    []string{"client_meeting"},
			wantCleaned: "Lunch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags, cleaned := extractTags(tt.input)
			require.Equal(t, tt.wantTags, tags)
			require.Equal(t, tt.wantCleaned, cleaned)
		})
	}
}

func TestParseExpenseInputWithTags(t *testing.T) {
	t.Run("expense with inline tag", func(t *testing.T) {
		result := ParseExpenseInput("5.50 Coffee #work")
		require.NotNil(t, result)
		require.Equal(t, "5.50", result.Amount.StringFixed(2))
		require.Equal(t, "Coffee", result.Description)
		require.Equal(t, []string{"work"}, result.Tags)
	})

	t.Run("expense with multiple tags", func(t *testing.T) {
		result := ParseExpenseInput("10.00 Lunch #work #team")
		require.NotNil(t, result)
		require.Equal(t, "10.00", result.Amount.StringFixed(2))
		require.Equal(t, "Lunch", result.Description)
		require.Equal(t, []string{"work", "team"}, result.Tags)
	})

	t.Run("expense without tags", func(t *testing.T) {
		result := ParseExpenseInput("5.50 Coffee")
		require.NotNil(t, result)
		require.Equal(t, "Coffee", result.Description)
		require.Nil(t, result.Tags)
	})

	t.Run("amount only no tags", func(t *testing.T) {
		result := ParseExpenseInput("5.50")
		require.NotNil(t, result)
		require.Nil(t, result.Tags)
	})
}

func TestParseAddCommandWithTags(t *testing.T) {
	t.Run("add command with tag", func(t *testing.T) {
		result := ParseAddCommand("/add 5.50 Coffee #work")
		require.NotNil(t, result)
		require.Equal(t, "5.50", result.Amount.StringFixed(2))
		require.Equal(t, "Coffee", result.Description)
		require.Equal(t, []string{"work"}, result.Tags)
	})
}

func TestIsValidTagName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"simple", "work", true},
		{"with underscore", "client_meeting", true},
		{"with digits", "q2budget", true},
		{"starts with digit", "2024", false},
		{"empty", "", false},
		{"html injection", "<b>bold</b>", false},
		{"special chars", "work!", false},
		{"spaces", "two words", false},
		{"too long", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false}, // 33 chars
		{"max length", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true},   // 30 chars
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.valid, isValidTagName(tt.input))
		})
	}
}

func TestEscapeHTML(t *testing.T) {
	require.Equal(t, "hello", escapeHTML("hello"))
	require.Equal(t, "&lt;b&gt;bold&lt;/b&gt;", escapeHTML("<b>bold</b>"))
	require.Equal(t, "a &amp; b", escapeHTML("a & b"))
}
