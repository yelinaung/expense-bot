package bot

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/models"
	"gitlab.com/yelinaung/expense-bot/internal/repository"
)

// TestHandleFreeTextExpense tests free-text expense parsing and creation.
func TestHandleFreeTextExpense(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	userRepo := repository.NewUserRepository(tx)
	categoryRepo := repository.NewCategoryRepository(tx)
	expenseRepo := repository.NewExpenseRepository(tx)
	mockBot := mocks.NewMockBot()

	user := &models.User{ID: 88888, Username: "freetextuser", FirstName: "FreeText", LastName: "User"}
	err := userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	t.Run("parses valid free-text expense", func(t *testing.T) {
		update := mocks.MessageUpdate(12345, user.ID, "5.50 Coffee")

		categories, err := categoryRepo.GetAll(ctx)
		require.NoError(t, err)

		result := callHandleFreeTextExpense(ctx, mockBot, update, expenseRepo, categories)

		require.True(t, result)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "$5.50 SGD")
		require.Contains(t, msg.Text, "Coffee")
	})

	t.Run("returns false for commands", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.MessageUpdate(12345, user.ID, "/start")

		categories, err := categoryRepo.GetAll(ctx)
		require.NoError(t, err)

		result := callHandleFreeTextExpense(ctx, mockBot, update, expenseRepo, categories)

		require.False(t, result)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("returns false for invalid format", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.MessageUpdate(12345, user.ID, "hello world")

		categories, err := categoryRepo.GetAll(ctx)
		require.NoError(t, err)

		result := callHandleFreeTextExpense(ctx, mockBot, update, expenseRepo, categories)

		require.False(t, result)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("returns false for nil message", func(t *testing.T) {
		mockBot.Reset()

		update := &tgmodels.Update{Message: nil}

		result := callHandleFreeTextExpense(ctx, mockBot, update, expenseRepo, nil)

		require.False(t, result)
	})

	t.Run("returns false for empty text", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.MessageUpdate(12345, user.ID, "")

		result := callHandleFreeTextExpense(ctx, mockBot, update, expenseRepo, nil)

		require.False(t, result)
	})
}

// callHandleFreeTextExpense simulates the handleFreeTextExpense logic with mock.
func callHandleFreeTextExpense(
	ctx context.Context,
	mock *mocks.MockBot,
	update *tgmodels.Update,
	expenseRepo *repository.ExpenseRepository,
	categories []models.Category,
) bool {
	if update.Message == nil || update.Message.Text == "" {
		return false
	}

	text := update.Message.Text
	if strings.HasPrefix(text, "/") {
		return false
	}

	categoryNames := make([]string, len(categories))
	for i, cat := range categories {
		categoryNames[i] = cat.Name
	}

	parsed := ParseExpenseInputWithCategories(text, categoryNames)
	if parsed == nil {
		return false
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	expense := &models.Expense{
		UserID:      userID,
		Amount:      parsed.Amount,
		Currency:    "SGD",
		Description: parsed.Description,
	}

	if parsed.CategoryName != "" {
		for _, cat := range categories {
			if strings.EqualFold(cat.Name, parsed.CategoryName) {
				expense.CategoryID = &cat.ID
				expense.Category = &cat
				break
			}
		}
	}

	if err := expenseRepo.Create(ctx, expense); err != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to save expense. Please try again.",
		})
		return true
	}

	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = expense.Category.Name
	}

	descText := ""
	if expense.Description != "" {
		descText = "\n📝 " + expense.Description
	}

	msgText := "✅ <b>Expense Added</b>\n\n💰 $" + expense.Amount.StringFixed(2) + " SGD" + descText + "\n📁 " + categoryText + "\n🆔 #" + strconv.Itoa(expense.ID)

	_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      msgText,
		ParseMode: tgmodels.ParseModeHTML,
	})
	return true
}
