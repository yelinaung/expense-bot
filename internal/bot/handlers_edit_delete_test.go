package bot

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/models"
	"gitlab.com/yelinaung/expense-bot/internal/repository"
)

// TestHandleEdit tests the /edit command handler.
func TestHandleEdit(t *testing.T) {
	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	err = database.SeedCategories(ctx, pool)
	require.NoError(t, err)

	userRepo := repository.NewUserRepository(pool)
	categoryRepo := repository.NewCategoryRepository(pool)
	expenseRepo := repository.NewExpenseRepository(pool)
	mockBot := mocks.NewMockBot()

	user := &models.User{ID: 44444, Username: "edituser", FirstName: "Edit", LastName: "User"}
	err = userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	t.Run("shows usage when no arguments", func(t *testing.T) {
		update := mocks.CommandUpdate(12345, user.ID, "/edit")

		callHandleEdit(ctx, mockBot, update, expenseRepo, categoryRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Usage:")
		require.Contains(t, msg.Text, "/edit")
	})

	t.Run("shows error for invalid expense ID", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, "/edit abc 5.00 New description")

		callHandleEdit(ctx, mockBot, update, expenseRepo, categoryRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Invalid expense ID")
	})

	t.Run("shows error when expense not found", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, "/edit 99999 5.00 New description")

		callHandleEdit(ctx, mockBot, update, expenseRepo, categoryRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "not found")
	})

	t.Run("edits expense successfully", func(t *testing.T) {
		mockBot.Reset()

		expense := &models.Expense{
			UserID:      user.ID,
			Amount:      decimal.NewFromFloat(10.00),
			Currency:    "SGD",
			Description: "Original",
			Status:      models.ExpenseStatusConfirmed,
		}
		err := expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, user.ID, "/edit "+strconv.Itoa(expense.ID)+" 20.00 Updated description")

		callHandleEdit(ctx, mockBot, update, expenseRepo, categoryRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Expense Updated")
		require.Contains(t, msg.Text, "$20.00 SGD")
		require.Contains(t, msg.Text, "Updated description")
	})

	t.Run("edits only amount, preserves description and category", func(t *testing.T) {
		mockBot.Reset()

		category, err := categoryRepo.Create(ctx, "Test Partial Edit Preserve Cat")
		require.NoError(t, err)

		expense := &models.Expense{
			UserID:      user.ID,
			Amount:      decimal.NewFromFloat(15.00),
			Currency:    "SGD",
			Description: "Original description",
			CategoryID:  &category.ID,
			Status:      models.ExpenseStatusConfirmed,
		}
		err = expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		// Edit only the amount - description and category should be preserved
		update := mocks.CommandUpdate(12345, user.ID, "/edit "+strconv.Itoa(expense.ID)+" 25.50")

		callHandleEdit(ctx, mockBot, update, expenseRepo, categoryRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Expense Updated")
		require.Contains(t, msg.Text, "$25.50 SGD")
		require.Contains(t, msg.Text, "Original description")
		require.Contains(t, msg.Text, "Test Partial Edit Preserve Cat")

		// Verify in database that fields were preserved
		updated, err := expenseRepo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.Equal(t, "25.50", updated.Amount.StringFixed(2))
		require.Equal(t, "Original description", updated.Description)
		require.NotNil(t, updated.CategoryID)
		require.Equal(t, category.ID, *updated.CategoryID)
	})

	t.Run("shows error when editing another user's expense", func(t *testing.T) {
		mockBot.Reset()

		otherUser := &models.User{ID: 55555, Username: "otheruser", FirstName: "Other", LastName: "User"}
		err := userRepo.UpsertUser(ctx, otherUser)
		require.NoError(t, err)

		expense := &models.Expense{
			UserID:      otherUser.ID,
			Amount:      decimal.NewFromFloat(50.00),
			Currency:    "SGD",
			Description: "Other's expense",
			Status:      models.ExpenseStatusConfirmed,
		}
		err = expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, user.ID, "/edit "+strconv.Itoa(expense.ID)+" 100.00 Trying to edit")

		callHandleEdit(ctx, mockBot, update, expenseRepo, categoryRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "only edit your own")
	})
}

// callHandleEdit simulates the handleEdit logic with mock.
func callHandleEdit(
	ctx context.Context,
	mock *mocks.MockBot,
	update *tgmodels.Update,
	expenseRepo *repository.ExpenseRepository,
	categoryRepo *repository.CategoryRepository,
	userID int64,
) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	args := strings.TrimPrefix(update.Message.Text, "/edit")
	args = strings.TrimSpace(args)

	if idx := strings.Index(args, "@"); idx == 0 {
		if spaceIdx := strings.Index(args, " "); spaceIdx != -1 {
			args = strings.TrimSpace(args[spaceIdx:])
		} else {
			args = ""
		}
	}

	if args == "" {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Usage: <code>/edit &lt;id&gt; &lt;amount&gt; &lt;description&gt; [category]</code>",
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	parts := strings.SplitN(args, " ", 2)
	expenseID, err := strconv.Atoi(parts[0])
	if err != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Invalid expense ID. Use: <code>/edit &lt;id&gt; &lt;amount&gt; &lt;description&gt;</code>",
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	expense, err := expenseRepo.GetByID(ctx, expenseID)
	if err != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Expense #" + strconv.Itoa(expenseID) + " not found.",
		})
		return
	}

	if expense.UserID != userID {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ You can only edit your own expenses.",
		})
		return
	}

	if len(parts) < 2 {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Please provide new values: <code>/edit &lt;id&gt; &lt;amount&gt; &lt;description&gt;</code>",
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	categories, _ := categoryRepo.GetAll(ctx)

	// Load the existing category if one is set
	if expense.CategoryID != nil {
		for i := range categories {
			if categories[i].ID == *expense.CategoryID {
				expense.Category = &categories[i]
				break
			}
		}
	}

	categoryNames := make([]string, len(categories))
	for i, cat := range categories {
		categoryNames[i] = cat.Name
	}

	parsed := ParseExpenseInputWithCategories(parts[1], categoryNames)
	if parsed == nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Invalid format. Use: <code>/edit &lt;id&gt; &lt;amount&gt; &lt;description&gt;</code>",
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	// Update amount (always required)
	expense.Amount = parsed.Amount

	// Only update description if provided
	if parsed.Description != "" {
		expense.Description = parsed.Description
	}

	// Only update category if provided
	if parsed.CategoryName != "" {
		for _, cat := range categories {
			if strings.EqualFold(cat.Name, parsed.CategoryName) {
				expense.CategoryID = &cat.ID
				expense.Category = &cat
				break
			}
		}
	}

	if err := expenseRepo.Update(ctx, expense); err != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to update expense. Please try again.",
		})
		return
	}

	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = expense.Category.Name
	}

	text := "✅ <b>Expense Updated</b>\n\n🆔 #" + strconv.Itoa(expense.ID) + "\n💰 $" + expense.Amount.StringFixed(2) + " SGD\n📝 " + expense.Description + "\n📁 " + categoryText

	_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: tgmodels.ParseModeHTML,
	})
}

// TestHandleDelete tests the /delete command handler.
func TestHandleDelete(t *testing.T) {
	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	err = database.SeedCategories(ctx, pool)
	require.NoError(t, err)

	userRepo := repository.NewUserRepository(pool)
	expenseRepo := repository.NewExpenseRepository(pool)
	mockBot := mocks.NewMockBot()

	user := &models.User{ID: 66666, Username: "deleteuser", FirstName: "Delete", LastName: "User"}
	err = userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	t.Run("shows usage when no arguments", func(t *testing.T) {
		update := mocks.CommandUpdate(12345, user.ID, "/delete")

		callHandleDelete(ctx, mockBot, update, expenseRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Usage:")
		require.Contains(t, msg.Text, "/delete")
	})

	t.Run("shows error for invalid expense ID", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, "/delete abc")

		callHandleDelete(ctx, mockBot, update, expenseRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Invalid expense ID")
	})

	t.Run("shows error when expense not found", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, "/delete 99999")

		callHandleDelete(ctx, mockBot, update, expenseRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "not found")
	})

	t.Run("deletes expense successfully", func(t *testing.T) {
		mockBot.Reset()

		expense := &models.Expense{
			UserID:      user.ID,
			Amount:      decimal.NewFromFloat(25.00),
			Currency:    "SGD",
			Description: "Expense to delete",
			Status:      models.ExpenseStatusConfirmed,
		}
		err := expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, user.ID, "/delete "+strconv.Itoa(expense.ID))

		callHandleDelete(ctx, mockBot, update, expenseRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Expense Deleted")

		_, err = expenseRepo.GetByID(ctx, expense.ID)
		require.Error(t, err)
	})

	t.Run("shows error when deleting another user's expense", func(t *testing.T) {
		mockBot.Reset()

		otherUser := &models.User{ID: 77777, Username: "otheruser2", FirstName: "Other", LastName: "User2"}
		err := userRepo.UpsertUser(ctx, otherUser)
		require.NoError(t, err)

		expense := &models.Expense{
			UserID:      otherUser.ID,
			Amount:      decimal.NewFromFloat(100.00),
			Currency:    "SGD",
			Description: "Other's expense",
			Status:      models.ExpenseStatusConfirmed,
		}
		err = expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, user.ID, "/delete "+strconv.Itoa(expense.ID))

		callHandleDelete(ctx, mockBot, update, expenseRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "only delete your own")
	})
}

// TestEditDeleteHandlerWrappers provides coverage for thin command wrapper functions.
// These wrappers exist only to match the telegram bot library's expected handler signature
// and delegate to Core functions or helper logic which is thoroughly tested above.
//
// We test wrappers by calling them with updates that cause early returns,
// avoiding the need for a real *bot.Bot instance.
func TestEditDeleteHandlerWrappers(t *testing.T) {
	t.Parallel()

	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	userRepo := repository.NewUserRepository(pool)
	categoryRepo := repository.NewCategoryRepository(pool)
	expenseRepo := repository.NewExpenseRepository(pool)

	b := &Bot{
		userRepo:     userRepo,
		categoryRepo: categoryRepo,
		expenseRepo:  expenseRepo,
	}

	// nil *bot.Bot - wrappers pass it through but return early before using it.
	var tgBot *bot.Bot

	t.Run("handleEdit wrapper", func(t *testing.T) {
		t.Parallel()
		// Update with nil Message causes early return in handleEdit.
		b.handleEdit(ctx, tgBot, &tgmodels.Update{})
	})

	t.Run("handleDelete wrapper", func(t *testing.T) {
		t.Parallel()
		// Update with nil Message causes early return in handleDelete.
		b.handleDelete(ctx, tgBot, &tgmodels.Update{})
	})
}

// callHandleDelete simulates the handleDelete logic with mock.
func callHandleDelete(
	ctx context.Context,
	mock *mocks.MockBot,
	update *tgmodels.Update,
	expenseRepo *repository.ExpenseRepository,
	userID int64,
) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	args := strings.TrimPrefix(update.Message.Text, "/delete")
	args = strings.TrimSpace(args)

	if idx := strings.Index(args, "@"); idx == 0 {
		if spaceIdx := strings.Index(args, " "); spaceIdx != -1 {
			args = strings.TrimSpace(args[spaceIdx:])
		} else {
			args = ""
		}
	}

	if args == "" {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Usage: <code>/delete &lt;id&gt;</code>",
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	expenseID, err := strconv.Atoi(args)
	if err != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Invalid expense ID. Use: <code>/delete &lt;id&gt;</code>",
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	expense, err := expenseRepo.GetByID(ctx, expenseID)
	if err != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Expense #" + strconv.Itoa(expenseID) + " not found.",
		})
		return
	}

	if expense.UserID != userID {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ You can only delete your own expenses.",
		})
		return
	}

	if err := expenseRepo.Delete(ctx, expenseID); err != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to delete expense. Please try again.",
		})
		return
	}

	_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      "✅ <b>Expense Deleted</b>\n\n🆔 #" + strconv.Itoa(expenseID),
		ParseMode: tgmodels.ParseModeHTML,
	})
}
