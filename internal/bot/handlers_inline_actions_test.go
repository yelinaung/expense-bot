package bot

import (
	"context"
	"strconv"
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

func TestHandleExpenseActionCallbackCore(t *testing.T) {
	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	userRepo := repository.NewUserRepository(pool)
	categoryRepo := repository.NewCategoryRepository(pool)
	expenseRepo := repository.NewExpenseRepository(pool)
	mockBot := mocks.NewMockBot()

	b := &Bot{
		userRepo:     userRepo,
		categoryRepo: categoryRepo,
		expenseRepo:  expenseRepo,
	}

	user := &models.User{ID: 12345, Username: "inlineuser", FirstName: "Inline", LastName: "User"}
	err = userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	category, err := categoryRepo.Create(ctx, "Food")
	require.NoError(t, err)

	expense := &models.Expense{
		UserID:      user.ID,
		Amount:      decimal.NewFromFloat(10.50),
		Currency:    "USD",
		Description: "Test expense",
		CategoryID:  &category.ID,
		Status:      "confirmed",
	}
	err = expenseRepo.Create(ctx, expense)
	require.NoError(t, err)

	t.Run("returns early for nil callback query", func(t *testing.T) {
		mockBot.Reset()

		update := &tgmodels.Update{CallbackQuery: nil}

		b.handleExpenseActionCallbackCore(ctx, mockBot, update)

		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("handles edit_expense action", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.NewUpdateBuilder().
			WithCallbackQuery("callback123", 100, user.ID, 200, "edit_expense_"+strconv.Itoa(expense.ID)).
			Build()

		b.handleExpenseActionCallbackCore(ctx, mockBot, update)

		// Should have answered the callback and edited the message.
		require.GreaterOrEqual(t, mockBot.SentMessageCount(), 1)
	})

	t.Run("handles delete_expense action", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.NewUpdateBuilder().
			WithCallbackQuery("callback124", 100, user.ID, 200, "delete_expense_"+strconv.Itoa(expense.ID)).
			Build()

		b.handleExpenseActionCallbackCore(ctx, mockBot, update)

		// Should have answered the callback and edited the message with delete confirmation.
		require.GreaterOrEqual(t, mockBot.SentMessageCount(), 1)
	})

	t.Run("handles invalid callback data format", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.NewUpdateBuilder().
			WithCallbackQuery("callback125", 100, user.ID, 200, "invalid").
			Build()

		b.handleExpenseActionCallbackCore(ctx, mockBot, update)

		// Should answer callback but not send additional messages.
		require.Equal(t, 1, mockBot.SentMessageCount())
	})

	t.Run("handles non-existent expense", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.NewUpdateBuilder().
			WithCallbackQuery("callback126", 100, user.ID, 200, "edit_expense_99999").
			Build()

		b.handleExpenseActionCallbackCore(ctx, mockBot, update)

		// Should have sent error message.
		require.GreaterOrEqual(t, mockBot.SentMessageCount(), 1)
	})

	t.Run("handles user mismatch", func(t *testing.T) {
		mockBot.Reset()

		// Different user trying to edit the expense.
		update := mocks.NewUpdateBuilder().
			WithCallbackQuery("callback127", 100, 99999, 200, "edit_expense_"+strconv.Itoa(expense.ID)).
			Build()

		b.handleExpenseActionCallbackCore(ctx, mockBot, update)

		// Should have answered callback with error.
		require.GreaterOrEqual(t, mockBot.SentMessageCount(), 1)
	})
}

func TestHandleConfirmDeleteCallbackCore(t *testing.T) {
	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	userRepo := repository.NewUserRepository(pool)
	categoryRepo := repository.NewCategoryRepository(pool)
	expenseRepo := repository.NewExpenseRepository(pool)
	mockBot := mocks.NewMockBot()

	b := &Bot{
		userRepo:     userRepo,
		categoryRepo: categoryRepo,
		expenseRepo:  expenseRepo,
	}

	user := &models.User{ID: 12345, Username: "deleteuser", FirstName: "Delete", LastName: "User"}
	err = userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	category, err := categoryRepo.Create(ctx, "Food")
	require.NoError(t, err)

	expense := &models.Expense{
		UserID:      user.ID,
		Amount:      decimal.NewFromFloat(15.75),
		Currency:    "USD",
		Description: "Delete me",
		CategoryID:  &category.ID,
		Status:      "confirmed",
	}
	err = expenseRepo.Create(ctx, expense)
	require.NoError(t, err)

	t.Run("returns early for nil callback query", func(t *testing.T) {
		mockBot.Reset()

		update := &tgmodels.Update{CallbackQuery: nil}

		b.handleConfirmDeleteCallbackCore(ctx, mockBot, update)

		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("confirms delete", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.NewUpdateBuilder().
			WithCallbackQuery("callback128", 100, user.ID, 200, "confirm_delete_"+strconv.Itoa(expense.ID)).
			Build()

		b.handleConfirmDeleteCallbackCore(ctx, mockBot, update)

		// Should have deleted the expense and sent confirmation.
		require.GreaterOrEqual(t, mockBot.SentMessageCount(), 1)

		// Verify expense was deleted.
		_, err := expenseRepo.GetByID(ctx, expense.ID)
		require.Error(t, err) // Should not find the expense anymore.
	})
}

func TestHandleBackToExpenseCallbackCore(t *testing.T) {
	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	userRepo := repository.NewUserRepository(pool)
	categoryRepo := repository.NewCategoryRepository(pool)
	expenseRepo := repository.NewExpenseRepository(pool)
	mockBot := mocks.NewMockBot()

	b := &Bot{
		userRepo:     userRepo,
		categoryRepo: categoryRepo,
		expenseRepo:  expenseRepo,
	}

	user := &models.User{ID: 12345, Username: "backuser", FirstName: "Back", LastName: "User"}
	err = userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	category, err := categoryRepo.Create(ctx, "Food")
	require.NoError(t, err)

	expense := &models.Expense{
		UserID:      user.ID,
		Amount:      decimal.NewFromFloat(20.00),
		Currency:    "USD",
		Description: "Back test",
		CategoryID:  &category.ID,
		Status:      "confirmed",
	}
	err = expenseRepo.Create(ctx, expense)
	require.NoError(t, err)

	t.Run("returns early for nil callback query", func(t *testing.T) {
		mockBot.Reset()

		update := &tgmodels.Update{CallbackQuery: nil}

		b.handleBackToExpenseCallbackCore(ctx, mockBot, update)

		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("navigates back to expense", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.NewUpdateBuilder().
			WithCallbackQuery("callback129", 100, user.ID, 200, "back_to_expense_"+strconv.Itoa(expense.ID)).
			Build()

		b.handleBackToExpenseCallbackCore(ctx, mockBot, update)

		// Should have edited message to show expense details again.
		require.GreaterOrEqual(t, mockBot.SentMessageCount(), 1)
	})
}

// TestInlineActionWrappers provides coverage for callback wrapper functions.
func TestInlineActionWrappers(t *testing.T) {
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

	var tgBot *bot.Bot

	t.Run("handleExpenseActionCallback wrapper", func(t *testing.T) {
		t.Parallel()
		// Update with nil CallbackQuery causes early return.
		b.handleExpenseActionCallback(ctx, tgBot, &tgmodels.Update{})
	})

	t.Run("handleConfirmDeleteCallback wrapper", func(t *testing.T) {
		t.Parallel()
		// Update with nil CallbackQuery causes early return.
		b.handleConfirmDeleteCallback(ctx, tgBot, &tgmodels.Update{})
	})

	t.Run("handleBackToExpenseCallback wrapper", func(t *testing.T) {
		t.Parallel()
		// Update with nil CallbackQuery causes early return.
		b.handleBackToExpenseCallback(ctx, tgBot, &tgmodels.Update{})
	})
}
