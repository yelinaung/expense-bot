package bot

import (
	"context"
	"strconv"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestHandleTagCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	userID := int64(700001)
	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "taguser",
		FirstName: "Tag",
	})
	require.NoError(t, err)

	t.Run("nil message returns early", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{Message: nil}
		b.handleTagCore(ctx, mockBot, update)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("no args shows usage", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(12345, userID, "/tag")
		b.handleTagCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Usage:")
	})

	t.Run("missing tag name shows usage", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(12345, userID, "/tag 1")
		b.handleTagCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Usage:")
	})

	t.Run("invalid expense ID shows error", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(12345, userID, "/tag abc work")
		b.handleTagCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Invalid expense ID")
	})

	t.Run("non-existent expense shows error", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(12345, userID, "/tag 99999 work")
		b.handleTagCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "not found")
	})

	t.Run("adds single tag successfully", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("5.50"),
			Currency:    "SGD",
			Description: "Coffee",
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, userID, "/tag "+itoa(expense.UserExpenseNumber)+" work")
		b.handleTagCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "#work")
		require.Contains(t, msg.Text, "Added")
	})

	t.Run("adds multiple tags", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("10.00"),
			Currency:    "SGD",
			Description: "Lunch",
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, userID, "/tag "+itoa(expense.UserExpenseNumber)+" work meeting")
		b.handleTagCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "#work")
		require.Contains(t, msg.Text, "#meeting")
	})

	t.Run("with bot mention", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("7.00"),
			Currency:    "SGD",
			Description: "Snack",
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, userID, "/tag@mybot "+itoa(expense.UserExpenseNumber)+" snack")
		b.handleTagCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "#snack")
	})

	t.Run("rejects invalid tag name with special chars", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("5.00"),
			Currency:    "SGD",
			Description: "Test",
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, userID, "/tag "+itoa(expense.UserExpenseNumber)+" <b>bold</b>")
		b.handleTagCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Invalid tag name")
	})

	t.Run("rejects tag starting with digit", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("5.00"),
			Currency:    "SGD",
			Description: "Test",
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, userID, "/tag "+itoa(expense.UserExpenseNumber)+" 2024")
		b.handleTagCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Invalid tag name")
	})

	t.Run("rejects too many tags", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("5.00"),
			Currency:    "SGD",
			Description: "Test",
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		tags := "a b c d e f g h i j k" // 11 tags
		update := mocks.CommandUpdate(12345, userID, "/tag "+itoa(expense.UserExpenseNumber)+" "+tags)
		b.handleTagCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Too many tags")
	})
}

func TestHandleUntagCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	userID := int64(700002)
	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "untaguser",
		FirstName: "Untag",
	})
	require.NoError(t, err)

	t.Run("nil message returns early", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{Message: nil}
		b.handleUntagCore(ctx, mockBot, update)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("no args shows usage", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(12345, userID, "/untag")
		b.handleUntagCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Usage:")
	})

	t.Run("removes tag successfully", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("5.50"),
			Currency:    "SGD",
			Description: "Coffee",
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		// Add a tag first.
		tag, err := b.tagRepo.GetOrCreate(ctx, "removeme")
		require.NoError(t, err)
		err = b.tagRepo.AddTagsToExpense(ctx, expense.ID, []int{tag.ID})
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, userID, "/untag "+itoa(expense.UserExpenseNumber)+" removeme")
		b.handleUntagCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Removed")
		require.Contains(t, msg.Text, "#removeme")
	})

	t.Run("non-existent tag shows error", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("3.00"),
			Currency:    "SGD",
			Description: "Water",
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, userID, "/untag "+itoa(expense.UserExpenseNumber)+" nonexistent")
		b.handleUntagCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "not found")
	})
}

func TestHandleTagsCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	userID := int64(700003)
	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "tagsuser",
		FirstName: "Tags",
	})
	require.NoError(t, err)

	t.Run("nil message returns early", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{Message: nil}
		b.handleTagsCore(ctx, mockBot, update)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("lists all tags", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		// Create a tag.
		_, err := b.tagRepo.GetOrCreate(ctx, "listtag")
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, userID, "/tags")
		b.handleTagsCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Tags")
		require.Contains(t, msg.Text, "#listtag")
	})

	t.Run("filters expenses by tag", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("8.00"),
			Currency:    "SGD",
			Description: "Tagged Expense",
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		tag, err := b.tagRepo.GetOrCreate(ctx, "filtertag")
		require.NoError(t, err)
		err = b.tagRepo.AddTagsToExpense(ctx, expense.ID, []int{tag.ID})
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, userID, "/tags filtertag")
		b.handleTagsCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Tagged Expense")
		require.Contains(t, msg.Text, "#filtertag")
	})

	t.Run("non-existent tag shows error", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(12345, userID, "/tags nonexistenttag")
		b.handleTagsCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "not found")
	})
}

func TestInlineTagsOnExpenseCreation(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	userID := int64(700004)
	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "inlinetaguser",
		FirstName: "Inline",
	})
	require.NoError(t, err)

	t.Run("expense with inline tag shows tag in confirmation", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: 12345},
				From: &models.User{ID: userID},
				Text: "/add 5.50 Coffee #work",
			},
		}

		b.handleAddCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Expense Added")
		require.Contains(t, msg.Text, "Coffee")
		require.Contains(t, msg.Text, "work")
	})
}

// itoa is a helper to convert int64 to string for test readability.
func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
