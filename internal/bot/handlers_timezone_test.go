package bot

import (
	"context"
	"testing"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/models"
	"gitlab.com/yelinaung/expense-bot/internal/repository"
	"gitlab.com/yelinaung/expense-bot/internal/testutil/dbtest"
)

func TestHandleSetTimezoneCore(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.TestTx(ctx, t)

	userRepo := repository.NewUserRepository(tx)
	categoryRepo := repository.NewCategoryRepository(tx)
	expenseRepo := repository.NewExpenseRepository(tx)
	mockBot := mocks.NewMockBot()

	b := &Bot{
		userRepo:     userRepo,
		categoryRepo: categoryRepo,
		expenseRepo:  expenseRepo,
	}

	user := &models.User{ID: 12345, Username: "tzuser", FirstName: "Tz", LastName: "User"}
	err := userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	t.Run("sets valid timezone", func(t *testing.T) {
		update := mocks.CommandUpdate(12345, user.ID, "/settimezone America/New_York")

		b.handleSetTimezoneCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "America/New_York")
		require.Contains(t, msg.Text, "Timezone set to")

		// Verify in database
		tz, err := userRepo.GetTimezone(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, "America/New_York", tz)
	})

	t.Run("shows error for invalid timezone", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, "/settimezone Invalid/Zone")

		b.handleSetTimezoneCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Unknown timezone")
		require.Contains(t, msg.Text, "Invalid/Zone")
	})

	t.Run("shows usage when no arguments", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, "/settimezone")

		b.handleSetTimezoneCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Set Your Timezone")
		require.Contains(t, msg.Text, "Asia/Singapore")
		require.Contains(t, msg.Text, "Common timezones")
	})

	t.Run("handles @botname suffix", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, "/settimezone@expensebot Europe/London")

		b.handleSetTimezoneCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Timezone set to")
		require.Contains(t, msg.Text, "Europe/London")
	})

	t.Run("returns early for nil message", func(t *testing.T) {
		mockBot.Reset()

		update := &tgmodels.Update{Message: nil}

		b.handleSetTimezoneCore(ctx, mockBot, update)

		require.Equal(t, 0, mockBot.SentMessageCount())
	})
}

func TestHandleShowTimezoneCore(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.TestTx(ctx, t)

	userRepo := repository.NewUserRepository(tx)
	categoryRepo := repository.NewCategoryRepository(tx)
	expenseRepo := repository.NewExpenseRepository(tx)
	mockBot := mocks.NewMockBot()

	b := &Bot{
		userRepo:     userRepo,
		categoryRepo: categoryRepo,
		expenseRepo:  expenseRepo,
	}

	user := &models.User{ID: 54321, Username: "showtzuser", FirstName: "Show", LastName: "User"}
	err := userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	t.Run("shows default timezone for new user", func(t *testing.T) {
		update := mocks.CommandUpdate(12345, user.ID, "/timezone")

		b.handleShowTimezoneCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Timezone Settings")
		require.Contains(t, msg.Text, "Asia/Singapore")
		require.Contains(t, msg.Text, "/settimezone")
	})

	t.Run("shows fallback timezone when user not found", func(t *testing.T) {
		mockBot.Reset()

		// Use a From.ID that does not exist in the users table so GetTimezone returns an error.
		update := mocks.CommandUpdate(12345, 99999, "/timezone")

		b.handleShowTimezoneCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Timezone Settings")
		require.Contains(t, msg.Text, "Asia/Singapore")
		require.Contains(t, msg.Text, "/settimezone")
	})
	t.Run("shows updated timezone after setting", func(t *testing.T) {
		mockBot.Reset()

		err := userRepo.UpdateTimezone(ctx, user.ID, "Europe/Berlin")
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, user.ID, "/timezone")

		b.handleShowTimezoneCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Europe/Berlin")
	})

	t.Run("returns early for nil message", func(t *testing.T) {
		mockBot.Reset()

		update := &tgmodels.Update{Message: nil}

		b.handleShowTimezoneCore(ctx, mockBot, update)

		require.Equal(t, 0, mockBot.SentMessageCount())
	})
}

func TestTimezoneHandlerWrappers(t *testing.T) {
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

	var tgBot *bot.Bot

	t.Run("handleSetTimezone wrapper", func(t *testing.T) {
		b.handleSetTimezone(ctx, tgBot, &tgmodels.Update{})
	})

	t.Run("handleShowTimezone wrapper", func(t *testing.T) {
		b.handleShowTimezone(ctx, tgBot, &tgmodels.Update{})
	})
}
