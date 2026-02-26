package bot

import (
	"context"
	"strconv"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestHandleEditCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(910001)
	chatID := int64(910001)

	require.NoError(t, b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "editcore",
		FirstName: "EditCore",
	}))

	t.Run("invalid command format returns usage", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/edit")
		b.handleEditCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "Usage:")
	})

	t.Run("expense not found returns not found", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/edit 9999 1.00 coffee")
		b.handleEditCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "not found")
	})

	t.Run("updates expense successfully", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      decimal.RequireFromString("10.00"),
			Currency:    "SGD",
			Description: "before",
			Merchant:    "before",
			Status:      appmodels.ExpenseStatusConfirmed,
		}
		require.NoError(t, b.expenseRepo.Create(ctx, expense))

		cmd := "/edit " + strconv.FormatInt(expense.UserExpenseNumber, 10) + " 20.50 after"
		update := mocks.CommandUpdate(chatID, userID, cmd)
		b.handleEditCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "Expense Updated")
		require.Contains(t, mockBot.LastSentMessage().Text, "20.50")

		updated, err := b.expenseRepo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.Equal(t, "20.50", updated.Amount.StringFixed(2))
		require.Equal(t, "after", updated.Description)
	})

	t.Run("invalid edit values return format error", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      decimal.RequireFromString("8.00"),
			Currency:    "SGD",
			Description: "value",
			Merchant:    "value",
			Status:      appmodels.ExpenseStatusConfirmed,
		}
		require.NoError(t, b.expenseRepo.Create(ctx, expense))

		cmd := "/edit " + strconv.FormatInt(expense.UserExpenseNumber, 10) + " not-a-valid-input"
		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: chatID},
				From: &models.User{ID: userID},
				Text: cmd,
			},
		}
		b.handleEditCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "Invalid format")
	})
}

func TestHandleDeleteCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(910101)
	chatID := int64(910101)

	require.NoError(t, b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "deletecore",
		FirstName: "DeleteCore",
	}))

	t.Run("usage when no args", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/delete")
		b.handleDeleteCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "Usage")
	})

	t.Run("invalid id", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/delete abc")
		b.handleDeleteCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "Invalid expense ID")
	})

	t.Run("not found", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(chatID, userID, "/delete 99999")
		b.handleDeleteCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "not found")
	})

	t.Run("deletes successfully", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      decimal.RequireFromString("11.00"),
			Currency:    "SGD",
			Description: "delete me",
			Status:      appmodels.ExpenseStatusConfirmed,
		}
		require.NoError(t, b.expenseRepo.Create(ctx, expense))

		cmd := "/delete " + strconv.FormatInt(expense.UserExpenseNumber, 10)
		update := mocks.CommandUpdate(chatID, userID, cmd)
		b.handleDeleteCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "deleted")
		_, err := b.expenseRepo.GetByID(ctx, expense.ID)
		require.Error(t, err)
	})
}
