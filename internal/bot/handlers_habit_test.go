package bot

import (
	"context"
	"fmt"
	"testing"
	"time"

	tgmodels "github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

const (
	habitTestChatID   = int64(73001)
	habitTestMessage  = 44
	habitTestTimezone = "UTC"
)

func TestHandleReviewCore(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)

	t.Run("no expenses", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(73101)
		upsertHabitTestUser(t, ctx, b, userID)

		b.handleReviewCore(ctx, mockBot, mocks.CommandUpdate(habitTestChatID, userID, "/review"))

		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "No expenses to review.")
	})

	t.Run("with unreviewed expense", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(73102)
		upsertHabitTestUser(t, ctx, b, userID)
		expense := createHabitTestExpense(
			t,
			ctx,
			b,
			userID,
			"Review coffee",
			"4.25",
			time.Date(2026, 6, 12, 9, 0, 0, 0, time.UTC),
		)

		b.handleReviewCore(ctx, mockBot, mocks.CommandUpdate(habitTestChatID, userID, "/review"))

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Was this worth it?")
		require.Contains(t, msg.Text, expense.Description)
		keyboard := requireInlineKeyboard(t, msg.ReplyMarkup)
		require.Equal(t, fmt.Sprintf("%s%d", reviewWorthPrefix, expense.ID), keyboard.InlineKeyboard[0][0].CallbackData)
		require.Equal(t, fmt.Sprintf("%s%d", reviewNotWorthPrefix, expense.ID), keyboard.InlineKeyboard[0][1].CallbackData)
		require.Equal(t, fmt.Sprintf("%s%d", reviewSkipPrefix, expense.ID), keyboard.InlineKeyboard[1][0].CallbackData)
	})
}

func TestHandleReviewCallbackCore(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)

	t.Run("worth-it callback asks for driver", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(73201)
		upsertHabitTestUser(t, ctx, b, userID)
		expense := createHabitTestExpense(
			t,
			ctx,
			b,
			userID,
			"Driver prompt coffee",
			"6.20",
			time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC),
		)

		update := mocks.CallbackQueryUpdate(
			habitTestChatID,
			userID,
			habitTestMessage,
			fmt.Sprintf("%s%d", reviewWorthPrefix, expense.ID),
		)
		b.handleReviewCallbackCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.AnsweredCallbackCount())
		require.Equal(t, 1, mockBot.EditedMessageCount())
		msg := mockBot.LastEditedMessage()
		require.Contains(t, msg.Text, "What drove this spend?")
		keyboard := requireInlineKeyboard(t, msg.ReplyMarkup)
		require.Equal(t, fmt.Sprintf("%s%d_1_0", reviewDriverPrefix, expense.ID), keyboard.InlineKeyboard[0][0].CallbackData)
	})

	t.Run("driver callback saves reflection and advances", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(73202)
		upsertHabitTestUser(t, ctx, b, userID)
		newer := createHabitTestExpense(
			t,
			ctx,
			b,
			userID,
			"Newer reviewed coffee",
			"8.00",
			time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC),
		)
		older := createHabitTestExpense(
			t,
			ctx,
			b,
			userID,
			"Older next lunch",
			"12.00",
			time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC),
		)

		update := mocks.CallbackQueryUpdate(
			habitTestChatID,
			userID,
			habitTestMessage,
			fmt.Sprintf("%s%d_1_0", reviewDriverPrefix, newer.ID),
		)
		b.handleReviewCallbackCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.AnsweredCallbackCount())
		require.Equal(t, 1, mockBot.EditedMessageCount())
		require.Contains(t, mockBot.LastEditedMessage().Text, older.Description)

		reviewed, err := b.expenseRepo.GetReviewedByUserIDAndDateRange(
			ctx,
			userID,
			time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		)
		require.NoError(t, err)
		require.Len(t, reviewed, 1)
		require.Equal(t, newer.ID, reviewed[0].ID)
		require.NotNil(t, reviewed[0].WorthIt)
		require.True(t, *reviewed[0].WorthIt)
		require.NotNil(t, reviewed[0].SpendDriver)
		require.Equal(t, string(spendingDrivers[0]), *reviewed[0].SpendDriver)
	})

	t.Run("skip advances without marking reviewed", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(73203)
		upsertHabitTestUser(t, ctx, b, userID)
		newer := createHabitTestExpense(
			t,
			ctx,
			b,
			userID,
			"Skipped newer taxi",
			"15.00",
			time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
		)
		older := createHabitTestExpense(
			t,
			ctx,
			b,
			userID,
			"Older next train",
			"3.00",
			time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC),
		)

		update := mocks.CallbackQueryUpdate(
			habitTestChatID,
			userID,
			habitTestMessage,
			fmt.Sprintf("%s%d", reviewSkipPrefix, newer.ID),
		)
		b.handleReviewCallbackCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.AnsweredCallbackCount())
		require.Equal(t, 1, mockBot.EditedMessageCount())
		require.Contains(t, mockBot.LastEditedMessage().Text, older.Description)

		unreviewed, err := b.expenseRepo.GetUnreviewedByUserID(ctx, userID, 10)
		require.NoError(t, err)
		require.Len(t, unreviewed, 2)
		require.Equal(t, newer.ID, unreviewed[0].ID)
	})

	t.Run("skip last expense shows done", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(73204)
		upsertHabitTestUser(t, ctx, b, userID)
		expense := createHabitTestExpense(
			t,
			ctx,
			b,
			userID,
			"Only unreviewed",
			"2.00",
			time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC),
		)

		update := mocks.CallbackQueryUpdate(
			habitTestChatID,
			userID,
			habitTestMessage,
			fmt.Sprintf("%s%d", reviewSkipPrefix, expense.ID),
		)
		b.handleReviewCallbackCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.AnsweredCallbackCount())
		require.Equal(t, 1, mockBot.EditedMessageCount())
		require.Equal(t, "No more expenses to review.", mockBot.LastEditedMessage().Text)
	})
}

func TestHandleHabitCore(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)
	b.nowFunc = func() time.Time {
		return time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	}

	t.Run("default period summarizes month", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(73301)
		upsertHabitTestUser(t, ctx, b, userID)
		reviewedExpense := createHabitTestExpense(
			t,
			ctx,
			b,
			userID,
			"Reviewed June coffee",
			"9.50",
			time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC),
		)
		createHabitTestExpense(
			t,
			ctx,
			b,
			userID,
			"Unreviewed June lunch",
			"11.00",
			time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC),
		)
		worthIt := true
		require.NoError(t, b.expenseRepo.UpdateReflection(ctx, reviewedExpense.ID, userID, &worthIt, "Necessity"))

		b.handleHabitCore(ctx, mockBot, mocks.CommandUpdate(habitTestChatID, userID, "/habit"))

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Spending Reflection")
		require.Contains(t, msg.Text, "June 2026")
		require.Contains(t, msg.Text, "Reviewed: 1/2")
		require.Contains(t, msg.Text, "Worth it: 1")
		require.Contains(t, msg.Text, "SGD: S$9.50")
	})

	t.Run("invalid period", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(73302)
		upsertHabitTestUser(t, ctx, b, userID)

		b.handleHabitCore(ctx, mockBot, mocks.CommandUpdate(habitTestChatID, userID, "/habit yearly"))

		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "Invalid habit period")
	})

	t.Run("no data", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(73303)
		upsertHabitTestUser(t, ctx, b, userID)

		b.handleHabitCore(ctx, mockBot, mocks.CommandUpdate(habitTestChatID, userID, "/habit"))

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Reviewed: 0/0")
		require.Contains(t, msg.Text, "Worth-it spend:\n  None")
		require.Contains(t, msg.Text, "Best-value category: Not enough data")
	})
}

func upsertHabitTestUser(t *testing.T, ctx context.Context, b *Bot, userID int64) {
	t.Helper()

	require.NoError(t, b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:              userID,
		Username:        fmt.Sprintf("habit%d", userID),
		FirstName:       "Habit",
		Timezone:        habitTestTimezone,
		DefaultCurrency: testCurrencySGD,
	}))
}

func createHabitTestExpense(
	t *testing.T,
	ctx context.Context,
	b *Bot,
	userID int64,
	description string,
	amount string,
	createdAt time.Time,
) *appmodels.Expense {
	t.Helper()

	expense := &appmodels.Expense{
		UserID:      userID,
		Amount:      decimal.RequireFromString(amount),
		Currency:    testCurrencySGD,
		Description: description,
		Merchant:    description,
		Status:      appmodels.ExpenseStatusConfirmed,
	}
	require.NoError(t, b.expenseRepo.Create(ctx, expense))
	_, err := b.db.Exec(ctx, testUpdateExpenseTimeSQL, createdAt, expense.ID)
	require.NoError(t, err)
	expense.CreatedAt = createdAt

	return expense
}

func requireInlineKeyboard(t *testing.T, markup tgmodels.ReplyMarkup) *tgmodels.InlineKeyboardMarkup {
	t.Helper()

	keyboard, ok := markup.(*tgmodels.InlineKeyboardMarkup)
	require.True(t, ok, "expected inline keyboard markup")
	require.NotNil(t, keyboard)
	return keyboard
}
