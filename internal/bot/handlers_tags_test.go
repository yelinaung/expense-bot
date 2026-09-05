package bot

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

// normalizeProbeTag replaces the user-supplied probed tag name in a response
// with a placeholder. After normalization, probing another user's tag name
// and a globally-unused name must produce byte-identical responses, proving
// the existence oracle (divergent "not found" vs "Removed"/"No expenses
// found") is closed. The echoed name is the caller's own input, not a leak.
func normalizeProbeTag(s, probedName string) string {
	return strings.ReplaceAll(s, probedName, "<name>")
}

const (
	nilMessageReturnsEarlyTags = "nil message returns early"
	notFoundTextTags           = "not found"
)

func TestHandleTagCore(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)

	userID := int64(700001)
	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "taguser",
		FirstName: "Tag",
	})
	require.NoError(t, err)

	t.Run(nilMessageReturnsEarlyTags, func(t *testing.T) {
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
		require.Contains(t, msg.Text, testTagUsageText)
	})

	t.Run("missing tag name shows usage", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.CommandUpdate(12345, userID, "/tag 1")
		b.handleTagCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, testTagUsageText)
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
		require.Contains(t, msg.Text, notFoundTextTags)
	})

	t.Run("adds single tag successfully", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("5.50"),
			Currency:    testCurrencySGD,
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
			Currency:    testCurrencySGD,
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
			Currency:    testCurrencySGD,
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
			Currency:    testCurrencySGD,
			Description: "Test",
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, userID, "/tag "+itoa(expense.UserExpenseNumber)+" <b>bold</b>")
		b.handleTagCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Invalid tag name")
		require.NotContains(t, msg.Text, "<b>bold</b>")
		require.Contains(t, msg.Text, "&lt;b&gt;bold&lt;/b&gt;")
	})

	t.Run("rejects tag starting with digit", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("5.00"),
			Currency:    testCurrencySGD,
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
			Currency:    testCurrencySGD,
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
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)

	userID := int64(700002)
	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "untaguser",
		FirstName: "Untag",
	})
	require.NoError(t, err)

	t.Run(nilMessageReturnsEarlyTags, func(t *testing.T) {
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
		require.Contains(t, msg.Text, testTagUsageText)
	})

	t.Run("removes tag successfully", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("5.50"),
			Currency:    testCurrencySGD,
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
			Currency:    testCurrencySGD,
			Description: "Water",
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, userID, "/untag "+itoa(expense.UserExpenseNumber)+" nonexistent")
		b.handleUntagCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, notFoundTextTags)
	})

	t.Run("does not leak other users tag existence", func(t *testing.T) {
		// /untag <my-expense> <name> must not distinguish "name used by another
		// user" from "name unused by anyone". The buggy global GetByName
		// resolved another user's tag, then the no-op RemoveTagFromExpense
		// still printed "✅ Removed ...", leaking that the name is used.
		otherUserID := int64(700097)
		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:       otherUserID,
			Username: "untagoracleother",
		})
		require.NoError(t, err)

		otherExpense := &appmodels.Expense{
			UserID:      otherUserID,
			Amount:      mustParseDecimal("4.00"),
			Currency:    testCurrencySGD,
			Description: "untag oracle other expense",
			Status:      appmodels.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, otherExpense)
		require.NoError(t, err)

		otherTag, err := b.tagRepo.GetOrCreate(ctx, "untagoracleother")
		require.NoError(t, err)
		require.NoError(t, b.tagRepo.AddTagsToExpense(ctx, otherExpense.ID, []int{otherTag.ID}))

		mine := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("1.00"),
			Currency:    testCurrencySGD,
			Description: "untag oracle mine",
			Status:      appmodels.ExpenseStatusConfirmed,
		}
		require.NoError(t, b.expenseRepo.Create(ctx, mine))

		// Another user's tag name: must report "not found", never "Removed".
		mockBotA := mocks.NewMockBot()
		b.handleUntagCore(ctx, mockBotA, mocks.CommandUpdate(12345, userID, "/untag "+itoa(mine.UserExpenseNumber)+" untagoracleother"))
		require.Equal(t, 1, mockBotA.SentMessageCount())
		msgA := mockBotA.LastSentMessage()
		require.Contains(t, msgA.Text, notFoundTextTags)
		require.NotContains(t, msgA.Text, "Removed")

		// A globally-unused name: also "not found".
		mockBotB := mocks.NewMockBot()
		b.handleUntagCore(ctx, mockBotB, mocks.CommandUpdate(12345, userID, "/untag "+itoa(mine.UserExpenseNumber)+" globallyunuseduntag"))
		require.Equal(t, 1, mockBotB.SentMessageCount())
		msgB := mockBotB.LastSentMessage()
		require.Contains(t, msgB.Text, notFoundTextTags)
		require.NotContains(t, msgB.Text, "Removed")

		// No divergence modulo the echoed tag name: the only allowed
		// difference is the name the caller themselves typed. The outcome
		// class ("not found" vs "Removed") must be identical.
		require.Equal(t,
			normalizeProbeTag(msgA.Text, "untagoracleother"),
			normalizeProbeTag(msgB.Text, "globallyunuseduntag"),
			"other user's tag must be indistinguishable from an unused name",
		)
	})
}

func TestHandleTagsCore(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)

	userID := int64(700003)
	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "tagsuser",
		FirstName: "Tags",
	})
	require.NoError(t, err)

	t.Run(nilMessageReturnsEarlyTags, func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{Message: nil}
		b.handleTagsCore(ctx, mockBot, update)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("lists all tags", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		// Create an expense owned by the user and tag it.
		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("1.00"),
			Currency:    testCurrencySGD,
			Description: "tag list test",
			Status:      appmodels.ExpenseStatusConfirmed,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		tag, err := b.tagRepo.GetOrCreate(ctx, "listtag")
		require.NoError(t, err)
		err = b.tagRepo.AddTagsToExpense(ctx, expense.ID, []int{tag.ID})
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, userID, "/tags")
		b.handleTagsCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Tags")
		require.Contains(t, msg.Text, "#listtag")
	})

	t.Run("does not leak other users tags", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		otherUserID := int64(700099)
		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        otherUserID,
			Username:  "othertagsuser",
			FirstName: "Other",
		})
		require.NoError(t, err)

		otherExpense := &appmodels.Expense{
			UserID:      otherUserID,
			Amount:      mustParseDecimal("2.00"),
			Currency:    testCurrencySGD,
			Description: "other tag list test",
			Status:      appmodels.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, otherExpense)
		require.NoError(t, err)

		otherTag, err := b.tagRepo.GetOrCreate(ctx, "secretothertag")
		require.NoError(t, err)
		err = b.tagRepo.AddTagsToExpense(ctx, otherExpense.ID, []int{otherTag.ID})
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, userID, "/tags")
		b.handleTagsCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotContains(t, msg.Text, "#secretothertag")
	})

	t.Run("filters expenses by tag", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("8.00"),
			Currency:    testCurrencySGD,
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
		require.Contains(t, msg.Text, notFoundTextTags)
	})

	t.Run("does not leak other users tag existence", func(t *testing.T) {
		// /tags <name> must not distinguish "name used by another user" from
		// "name unused by anyone". The buggy global GetByName resolved another
		// user's tag, then the user-scoped GetExpensesByTagID returned empty,
		// rendering "No expenses found." instead of "not found".
		otherUserID := int64(700098)
		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:       otherUserID,
			Username: "tagsoracleother",
		})
		require.NoError(t, err)

		otherExpense := &appmodels.Expense{
			UserID:      otherUserID,
			Amount:      mustParseDecimal("3.00"),
			Currency:    testCurrencySGD,
			Description: "oracle other expense",
			Status:      appmodels.ExpenseStatusConfirmed,
		}
		err = b.expenseRepo.Create(ctx, otherExpense)
		require.NoError(t, err)

		otherTag, err := b.tagRepo.GetOrCreate(ctx, "oracleothertag")
		require.NoError(t, err)
		require.NoError(t, b.tagRepo.AddTagsToExpense(ctx, otherExpense.ID, []int{otherTag.ID}))

		// Another user's tag name: must report "not found", never render the
		// expense list ("No expenses found." / "Expenses tagged #...").
		mockBotA := mocks.NewMockBot()
		b.handleTagsCore(ctx, mockBotA, mocks.CommandUpdate(12345, userID, "/tags oracleothertag"))
		require.Equal(t, 1, mockBotA.SentMessageCount())
		msgA := mockBotA.LastSentMessage()
		require.Contains(t, msgA.Text, notFoundTextTags)
		require.NotContains(t, msgA.Text, "No expenses found.")
		require.NotContains(t, msgA.Text, "Expenses tagged")

		// A globally-unused name: also "not found".
		mockBotB := mocks.NewMockBot()
		b.handleTagsCore(ctx, mockBotB, mocks.CommandUpdate(12345, userID, "/tags globallyunusedname"))
		require.Equal(t, 1, mockBotB.SentMessageCount())
		msgB := mockBotB.LastSentMessage()
		require.Contains(t, msgB.Text, notFoundTextTags)
		require.NotContains(t, msgB.Text, "No expenses found.")

		// No divergence modulo the echoed tag name: the only allowed
		// difference is the name the caller themselves typed. The outcome
		// class ("not found" vs "No expenses found") must be identical.
		require.Equal(t,
			normalizeProbeTag(msgA.Text, "oracleothertag"),
			normalizeProbeTag(msgB.Text, "globallyunusedname"),
			"other user's tag must be indistinguishable from an unused name",
		)
	})
}

func TestInlineTagsOnExpenseCreation(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)

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
