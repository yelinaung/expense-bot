package bot

import (
	"context"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
)

// TestHabitHandlerWrappers covers the thin handler wrappers that exist only to
// match the telegram bot library's expected signature. They delegate to the
// Core functions, which return early on empty updates, so no bot/DB is needed.
func TestHabitHandlerWrappers(t *testing.T) {
	t.Parallel()

	b := &Bot{}
	ctx := context.Background()

	var tgBot *bot.Bot

	t.Run("handleReview wrapper - nil message", func(t *testing.T) {
		t.Parallel()
		b.handleReview(ctx, tgBot, &models.Update{})
	})

	t.Run("handleHabit wrapper - nil message", func(t *testing.T) {
		t.Parallel()
		b.handleHabit(ctx, tgBot, &models.Update{})
	})

	t.Run("handleReviewCallback wrapper - nil callback", func(t *testing.T) {
		t.Parallel()
		b.handleReviewCallback(ctx, tgBot, &models.Update{})
	})
}

// TestParseDriverCallback_Malformed covers the parse failure branches that the
// happy-path keyboard test does not reach.
func TestParseDriverCallback_Malformed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		data string
	}{
		{"too few parts", "review_driver_123_1"},
		{"too many parts", "review_driver_123_1_0_4"},
		{"non-numeric expense id", "review_driver_abc_1_0"},
		{"invalid worth bit", "review_driver_123_2_0"},
		{"non-numeric worth bit", "review_driver_123_x_0"},
		{"non-numeric driver index", "review_driver_123_1_z"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.False(t, parseDriverCallback(tc.data).ok)
		})
	}
}
