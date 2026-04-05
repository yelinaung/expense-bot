package bot

import (
	"testing"
)

func TestExtractCommandArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		text    string
		command string
		want    string
	}{
		{
			name:    "simple command with args",
			text:    withCommandArg(testAddCategoryCommand, "Food"),
			command: testAddCategoryCommand,
			want:    "Food",
		},
		{
			name:    "command with no args",
			text:    testAddCategoryCommand,
			command: testAddCategoryCommand,
			want:    "",
		},
		{
			name:    "command with bot mention and args",
			text:    withCommandArg(withBotMention(testAddCategoryCommand), "Food"),
			command: testAddCategoryCommand,
			want:    "Food",
		},
		{
			name:    "command with bot mention and no args",
			text:    withBotMention(testAddCategoryCommand),
			command: testAddCategoryCommand,
			want:    "",
		},
		{
			name:    "command with multi-word args",
			text:    withCommandArg(testAddCategoryCommand, testCategoryFoodDiningOut),
			command: testAddCategoryCommand,
			want:    testCategoryFoodDiningOut,
		},
		{
			name:    "command with bot mention and multi-word args",
			text:    "/deletecategory@bot_name My Category Name",
			command: "/deletecategory",
			want:    "My Category Name",
		},
		{
			name:    "command with extra spaces",
			text:    testAddCategoryCommand + "   Food  ",
			command: testAddCategoryCommand,
			want:    "Food",
		},
		{
			name:    "edit command",
			text:    "/edit 1 5.50 Coffee",
			command: "/edit",
			want:    "1 5.50 Coffee",
		},
		{
			name:    "delete command with bot mention",
			text:    "/delete@mybot 42",
			command: "/delete",
			want:    "42",
		},
		{
			name:    "setcurrency command",
			text:    "/setcurrency USD",
			command: "/setcurrency",
			want:    "USD",
		},
		{
			name:    "rename with arrow syntax",
			text:    "/renamecategory Old -> New",
			command: "/renamecategory",
			want:    "Old -> New",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractCommandArgs(tt.text, tt.command)
			if got != tt.want {
				t.Errorf("extractCommandArgs(%q, %q) = %q, want %q", tt.text, tt.command, got, tt.want)
			}
		})
	}
}
