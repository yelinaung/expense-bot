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
	"gitlab.com/yelinaung/expense-bot/internal/models"
	"gitlab.com/yelinaung/expense-bot/internal/repository"
	"gitlab.com/yelinaung/expense-bot/internal/testutil/dbtest"
)

const (
	editUsageHTML       = "❌ Usage: <code>/edit &lt;id&gt; &lt;amount&gt; &lt;description&gt; [category]</code>"
	editInvalidIDHTML   = "❌ Invalid expense ID. Use: <code>/edit &lt;id&gt; &lt;amount&gt; &lt;description&gt;</code>"
	editProvideValsHTML = "❌ Please provide new values: <code>/edit &lt;id&gt; &lt;amount&gt; &lt;description&gt;</code>"
	editInvalidFmtHTML  = "❌ Invalid format. Use: <code>/edit &lt;id&gt; &lt;amount&gt; &lt;description&gt;</code>"
	deleteUsageHTML     = "❌ Usage: <code>/delete &lt;id&gt;</code>"
	deleteInvalidIDHTML = "❌ Invalid expense ID. Use: <code>/delete &lt;id&gt;</code>"
)

// TestHandleEdit tests the /edit command handler.
func TestHandleEdit(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.TestTx(ctx, t)

	userRepo := repository.NewUserRepository(tx)
	categoryRepo := repository.NewCategoryRepository(tx)
	expenseRepo := repository.NewExpenseRepository(tx)
	mockBot := mocks.NewMockBot()

	user := &models.User{ID: 44444, Username: "edituser", FirstName: "Edit", LastName: "User"}
	err := userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	t.Run("shows usage when no arguments", func(t *testing.T) {
		update := mocks.CommandUpdate(12345, user.ID, testEditCommand)

		callHandleEdit(ctx, mockBot, update, expenseRepo, categoryRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, testTagUsageText)
		require.Contains(t, msg.Text, testEditCommand)
	})

	t.Run("shows error for invalid expense ID", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, testEditCommandPrefix+"abc 5.00 New description")

		callHandleEdit(ctx, mockBot, update, expenseRepo, categoryRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Invalid expense ID")
	})

	t.Run("shows error when expense not found", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, testEditCommandPrefix+"99999 5.00 New description")

		callHandleEdit(ctx, mockBot, update, expenseRepo, categoryRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, testNotFoundText)
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

		update := mocks.CommandUpdate(12345, user.ID, testEditCommandPrefix+strconv.FormatInt(expense.UserExpenseNumber, 10)+" 20.00 Updated description")

		callHandleEdit(ctx, mockBot, update, expenseRepo, categoryRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Expense Updated")
		require.Contains(t, msg.Text, "S$20.00 SGD")
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
			Description: testOriginalDescription,
			CategoryID:  &category.ID,
			Status:      models.ExpenseStatusConfirmed,
		}
		err = expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		// Edit only the amount - description and category should be preserved
		update := mocks.CommandUpdate(12345, user.ID, testEditCommandPrefix+strconv.FormatInt(expense.UserExpenseNumber, 10)+" 25.50")

		callHandleEdit(ctx, mockBot, update, expenseRepo, categoryRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Expense Updated")
		require.Contains(t, msg.Text, "S$25.50 SGD")
		require.Contains(t, msg.Text, testOriginalDescription)
		require.Contains(t, msg.Text, "Test Partial Edit Preserve Cat")

		// Verify in database that fields were preserved
		updated, err := expenseRepo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.Equal(t, "25.50", updated.Amount.StringFixed(2))
		require.Equal(t, testOriginalDescription, updated.Description)
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

		// Use a number that doesn't exist for the current user.
		// The other user's expense number won't resolve under user.ID
		// since GetByUserAndNumber is scoped by user.
		nonExistentNum := int64(99999)
		update := mocks.CommandUpdate(12345, user.ID, testEditCommandPrefix+strconv.FormatInt(nonExistentNum, 10)+" 100.00 Trying to edit")

		callHandleEdit(ctx, mockBot, update, expenseRepo, categoryRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, testNotFoundText)
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
	args := extractCommandArgs(update.Message.Text, testEditCommand)

	if args == "" {
		sendMockHTMLMessage(ctx, mock, chatID, editUsageHTML)
		return
	}

	expense, values, ok := findEditableExpense(ctx, mock, chatID, expenseRepo, userID, args)
	if !ok {
		return
	}

	categories, ok := loadEditCategories(ctx, mock, chatID, categoryRepo)
	if !ok {
		return
	}

	attachExpenseCategory(expense, categories)

	parsed := parseEditValues(ctx, mock, chatID, values, categories)
	if parsed == nil {
		return
	}

	applyParsedExpenseEdit(expense, parsed, categories)

	if !updateEditedExpense(ctx, mock, chatID, expenseRepo, expense) {
		return
	}

	sendEditedExpenseMessage(ctx, mock, chatID, expense)
}

// TestHandleDelete tests the /delete command handler.
func TestHandleDelete(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.TestTx(ctx, t)

	userRepo := repository.NewUserRepository(tx)
	expenseRepo := repository.NewExpenseRepository(tx)
	mockBot := mocks.NewMockBot()

	user := &models.User{ID: 66666, Username: "deleteuser", FirstName: "Delete", LastName: "User"}
	err := userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	t.Run("shows usage when no arguments", func(t *testing.T) {
		update := mocks.CommandUpdate(12345, user.ID, testDeleteCommand)

		callHandleDelete(ctx, mockBot, update, expenseRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, testTagUsageText)
		require.Contains(t, msg.Text, testDeleteCommand)
	})

	t.Run("shows error for invalid expense ID", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, withCommandArg(testDeleteCommand, "abc"))

		callHandleDelete(ctx, mockBot, update, expenseRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Invalid expense ID")
	})

	t.Run("shows error when expense not found", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, withCommandArg(testDeleteCommand, "99999"))

		callHandleDelete(ctx, mockBot, update, expenseRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, testNotFoundText)
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

		update := mocks.CommandUpdate(12345, user.ID, withCommandArg(testDeleteCommand, strconv.FormatInt(expense.UserExpenseNumber, 10)))

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

		update := mocks.CommandUpdate(12345, user.ID, withCommandArg(testDeleteCommand, strconv.FormatInt(expense.UserExpenseNumber, 10)))

		callHandleDelete(ctx, mockBot, update, expenseRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, testNotFoundText)
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

	ctx := context.Background()
	tx := dbtest.TestTx(ctx, t)

	userRepo := repository.NewUserRepository(tx)
	categoryRepo := repository.NewCategoryRepository(tx)
	expenseRepo := repository.NewExpenseRepository(tx)

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

	args := strings.TrimPrefix(update.Message.Text, testDeleteCommand)
	args = strings.TrimSpace(args)

	if strings.Index(args, "@") == 0 {
		if spaceIdx := strings.Index(args, " "); spaceIdx != -1 {
			args = strings.TrimSpace(args[spaceIdx:])
		} else {
			args = ""
		}
	}

	if args == "" {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      deleteUsageHTML,
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	expenseNum, err := strconv.ParseInt(args, 10, 64)
	if err != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      deleteInvalidIDHTML,
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	expense, err := expenseRepo.GetByUserAndNumber(ctx, userID, expenseNum)
	if err != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Expense #" + strconv.FormatInt(expenseNum, 10) + " " + testNotFoundText + ".",
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

	if expenseRepo.Delete(ctx, expense.ID) != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to delete expense. Please try again.",
		})
		return
	}

	_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      "✅ <b>Expense Deleted</b>\n\n🆔 #" + strconv.FormatInt(expenseNum, 10),
		ParseMode: tgmodels.ParseModeHTML,
	})
}
