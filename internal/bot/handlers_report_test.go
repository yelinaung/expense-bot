package bot

import (
	"context"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestHandleReportCore(t *testing.T) {
	// Note: Not using t.Parallel() to avoid database cleanup conflicts

	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	userID := int64(800001)
	chatID := int64(800001)

	// Create user
	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "reportuser",
		FirstName: "Report",
	})
	require.NoError(t, err)

	// Create category
	category, err := b.categoryRepo.Create(ctx, "Test Report Category")
	require.NoError(t, err)

	// Create expenses for this week
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	startOfWeek := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 10, 0, 0, 0, now.Location())

	for i := 0; i < 3; i++ {
		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      decimal.NewFromFloat(10.50),
			Currency:    "SGD",
			Description: "Weekly expense",
			CategoryID:  &category.ID,
			Status:      appmodels.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		// Update created_at to be within this week
		_, err = b.expenseRepo.Pool().Exec(ctx,
			"UPDATE expenses SET created_at = $1 WHERE id = $2",
			startOfWeek.Add(time.Duration(i)*24*time.Hour), expense.ID)
		require.NoError(t, err)
	}

	// Create expenses for this month (but not this week)
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 10, 0, 0, 0, now.Location())
	for i := 0; i < 2; i++ {
		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      decimal.NewFromFloat(20.00),
			Currency:    "SGD",
			Description: "Monthly expense",
			CategoryID:  &category.ID,
			Status:      appmodels.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		// Update created_at to be within this month but before this week
		_, err = b.expenseRepo.Pool().Exec(ctx,
			"UPDATE expenses SET created_at = $1 WHERE id = $2",
			startOfMonth.Add(time.Duration(i)*24*time.Hour), expense.ID)
		require.NoError(t, err)
	}

	t.Run("generates weekly report CSV", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/report week")

		b.handleReportCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		// In real implementation, this would send a document
		// For now, we verify no error messages were sent
		msg := mockBot.LastSentMessage()
		require.NotContains(t, msg.Text, "❌")
	})

	t.Run("generates monthly report CSV", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/report month")

		b.handleReportCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotContains(t, msg.Text, "❌")
	})

	t.Run("returns error for invalid period", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/report invalid")

		b.handleReportCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "❌ Invalid report type")
		require.Contains(t, msg.Text, "week")
		require.Contains(t, msg.Text, "month")
	})

	t.Run("returns error when no period specified", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/report")

		b.handleReportCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "❌ Please specify report type")
		require.Contains(t, msg.Text, "/report week")
		require.Contains(t, msg.Text, "/report month")
	})

	t.Run("handles period with no expenses", func(t *testing.T) {
		// Create a new user with no expenses
		newUserID := int64(800002)
		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        newUserID,
			Username:  "emptyuser",
			FirstName: "Empty",
		})
		require.NoError(t, err)

		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, newUserID, "/report week")

		b.handleReportCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "No expenses found for week")
	})

	t.Run("returns early for nil message", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{}

		b.handleReportCore(ctx, mockBot, update)

		require.Equal(t, 0, mockBot.SentMessageCount())
	})
}

func TestHandleReportWrapper(t *testing.T) {
	t.Parallel()

	b := &Bot{}
	ctx := context.Background()
	var tgBot *bot.Bot

	t.Run("wrapper delegates to core", func(t *testing.T) {
		update := &models.Update{}
		b.handleReport(ctx, tgBot, update)
		// Should not panic
	})
}
