package bot

import (
	"context"
	"testing"

	tgmodels "github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/models"
	"gitlab.com/yelinaung/expense-bot/internal/repository"
)

func TestHandleSetCurrencyCore(t *testing.T) {
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

	user := &models.User{ID: 12345, Username: "currencyuser", FirstName: "Currency", LastName: "User"}
	err = userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	t.Run("sets valid currency", func(t *testing.T) {
		update := mocks.CommandUpdate(12345, user.ID, "/setcurrency USD")

		b.handleSetCurrencyCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "✅")
		require.Contains(t, msg.Text, "USD")
		require.Contains(t, msg.Text, "$")

		// Verify in database
		currency, err := userRepo.GetDefaultCurrency(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, "USD", currency)
	})

	t.Run("shows error for invalid currency", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, "/setcurrency XYZ")

		b.handleSetCurrencyCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "❌")
		require.Contains(t, msg.Text, "Unknown currency")
		require.Contains(t, msg.Text, "XYZ")
	})

	t.Run("shows currency list when no arguments", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, "/setcurrency")

		b.handleSetCurrencyCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Set Default Currency")
		require.Contains(t, msg.Text, "USD")
		require.Contains(t, msg.Text, "EUR")
		require.Contains(t, msg.Text, "SGD")
		require.Contains(t, msg.Text, "Supported currencies")
	})

	t.Run("handles @botname suffix", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, "/setcurrency@expensebot EUR")

		b.handleSetCurrencyCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "✅")
		require.Contains(t, msg.Text, "EUR")
	})

	t.Run("normalizes currency to uppercase", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, "/setcurrency gbp")

		b.handleSetCurrencyCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "✅")
		require.Contains(t, msg.Text, "GBP")

		// Verify in database
		currency, err := userRepo.GetDefaultCurrency(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, "GBP", currency)
	})

	t.Run("returns early for nil message", func(t *testing.T) {
		mockBot.Reset()

		update := &tgmodels.Update{Message: nil}

		b.handleSetCurrencyCore(ctx, mockBot, update)

		require.Equal(t, 0, mockBot.SentMessageCount())
	})
}

func TestHandleShowCurrencyCore(t *testing.T) {
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

	user := &models.User{ID: 54321, Username: "showuser", FirstName: "Show", LastName: "User"}
	err = userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	t.Run("shows default currency SGD for new user", func(t *testing.T) {
		update := mocks.CommandUpdate(12345, user.ID, "/currency")

		b.handleShowCurrencyCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Currency Settings")
		require.Contains(t, msg.Text, "SGD")
		require.Contains(t, msg.Text, "S$")
		require.Contains(t, msg.Text, "/setcurrency")
	})

	t.Run("shows updated currency after setting", func(t *testing.T) {
		mockBot.Reset()

		// Set currency to EUR
		err := userRepo.UpdateDefaultCurrency(ctx, user.ID, "EUR")
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, user.ID, "/currency")

		b.handleShowCurrencyCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "EUR")
		require.Contains(t, msg.Text, "€")
	})

	t.Run("returns early for nil message", func(t *testing.T) {
		mockBot.Reset()

		update := &tgmodels.Update{Message: nil}

		b.handleShowCurrencyCore(ctx, mockBot, update)

		require.Equal(t, 0, mockBot.SentMessageCount())
	})
}

func TestBuildCurrencyListMessage(t *testing.T) {
	b := &Bot{}

	message := b.buildCurrencyListMessage()

	// Check structure
	require.Contains(t, message, "Set Default Currency")
	require.Contains(t, message, "Usage:")
	require.Contains(t, message, "/setcurrency USD")
	require.Contains(t, message, "Supported currencies:")

	// Check all currencies are listed
	for code, symbol := range models.SupportedCurrencies {
		require.Contains(t, message, code, "Currency code %s should be in list", code)
		require.Contains(t, message, symbol, "Currency symbol %s should be in list", symbol)
	}

	// Check examples
	require.Contains(t, message, "$10 Coffee")
	require.Contains(t, message, "€5.50 Lunch")
	require.Contains(t, message, "THB")
}
