package bot

import (
	"context"
	"fmt"
	"testing"
	"time"

	tgmodels "github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
)

// TestHandleHabitReflectionExtraCoverage exercises branches not covered by
// TestHandleReviewCore / TestHandleReviewCallbackCore / TestHandleHabitCore:
// the "Later" dismissal, ownership rejection, malformed driver payloads, the
// week/90d period branches, and the nil-update early returns.
func TestHandleHabitReflectionExtraCoverage(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)
	b.nowFunc = func() time.Time {
		return time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	}

	createdAt := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)

	t.Run("habit nil message returns early", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		b.handleHabitCore(ctx, mockBot, &tgmodels.Update{})
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("review callback nil callback returns early", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		b.handleReviewCallbackCore(ctx, mockBot, &tgmodels.Update{})
		require.Equal(t, 0, mockBot.AnsweredCallbackCount())
		require.Equal(t, 0, mockBot.EditedMessageCount())
	})

	t.Run("later dismisses reflection buttons without reviewing", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(73401)
		upsertHabitTestUser(t, ctx, b, userID)
		expense := createHabitTestExpense(t, ctx, b, userID, "Later beans", "7.00", createdAt)

		update := mocks.CallbackQueryUpdate(
			habitTestChatID, userID, habitTestMessage,
			fmt.Sprintf("%s%d", reviewLaterPrefix, expense.ID),
		)
		b.handleReviewCallbackCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.AnsweredCallbackCount())
		require.Equal(t, 1, mockBot.EditedMessageCount())
		require.NotNil(t, mockBot.LastEditedMessage().ReplyMarkup)

		unreviewed, err := b.expenseRepo.GetUnreviewedByUserID(ctx, userID, 10)
		require.NoError(t, err)
		require.Len(t, unreviewed, 1)
	})

	t.Run("callback on another user's expense is rejected", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		ownerID := int64(73402)
		otherID := int64(73403)
		upsertHabitTestUser(t, ctx, b, ownerID)
		upsertHabitTestUser(t, ctx, b, otherID)
		expense := createHabitTestExpense(t, ctx, b, ownerID, "Owned lunch", "9.00", createdAt)

		update := mocks.CallbackQueryUpdate(
			habitTestChatID, otherID, habitTestMessage,
			fmt.Sprintf("%s%d", reviewWorthPrefix, expense.ID),
		)
		b.handleReviewCallbackCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.EditedMessageCount())
		require.Equal(t, expenseNotFoundMsgCB, mockBot.LastEditedMessage().Text)
	})

	t.Run("driver callback with out-of-range index is ignored", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(73404)
		upsertHabitTestUser(t, ctx, b, userID)
		expense := createHabitTestExpense(t, ctx, b, userID, "Snack", "2.50", createdAt)

		update := mocks.CallbackQueryUpdate(
			habitTestChatID, userID, habitTestMessage,
			fmt.Sprintf("%s%d_1_999", reviewDriverPrefix, expense.ID),
		)
		b.handleReviewCallbackCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.AnsweredCallbackCount())
		require.Equal(t, 0, mockBot.EditedMessageCount())
	})

	t.Run("week period works", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(73405)
		upsertHabitTestUser(t, ctx, b, userID)

		b.handleHabitCore(ctx, mockBot, mocks.CommandUpdate(habitTestChatID, userID, "/habit week"))

		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "This week")
	})

	t.Run("90d period summarizes reviewed expense", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(73406)
		upsertHabitTestUser(t, ctx, b, userID)
		expense := createHabitTestExpense(t, ctx, b, userID, "Quarterly taxi", "20.00", createdAt)
		worth := true
		require.NoError(t, b.expenseRepo.UpdateReflection(ctx, expense.ID, userID, &worth, "Necessity"))

		b.handleHabitCore(ctx, mockBot, mocks.CommandUpdate(habitTestChatID, userID, "/habit 90d"))

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Last 90 days")
		require.Contains(t, msg.Text, "Worth it: 1")
		require.Contains(t, msg.Text, "Necessity")
	})
}
