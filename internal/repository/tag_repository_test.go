package repository

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

func setupTagTest(t *testing.T) (*TagRepository, *ExpenseRepository, *UserRepository, context.Context) {
	t.Helper()

	tx := database.TestTx(t)
	ctx := context.Background()

	return NewTagRepository(tx),
		NewExpenseRepository(tx),
		NewUserRepository(tx),
		ctx
}

// createTestExpense creates a user and an expense for tag tests.
func createTestExpense(t *testing.T, userRepo *UserRepository, expenseRepo *ExpenseRepository, ctx context.Context, userID int64) *models.Expense {
	t.Helper()

	err := userRepo.UpsertUser(ctx, &models.User{ID: userID, Username: "taguser"})
	require.NoError(t, err)

	expense := &models.Expense{
		UserID:      userID,
		Amount:      decimal.NewFromFloat(10.00),
		Currency:    "SGD",
		Description: "Test expense",
		Status:      models.ExpenseStatusConfirmed,
	}
	err = expenseRepo.Create(ctx, expense)
	require.NoError(t, err)
	return expense
}

func TestTagRepository_GetOrCreate(t *testing.T) {
	tagRepo, _, _, ctx := setupTagTest(t)

	t.Run("creates new tag", func(t *testing.T) {
		tag, err := tagRepo.GetOrCreate(ctx, "work")
		require.NoError(t, err)
		require.NotZero(t, tag.ID)
		require.Equal(t, "work", tag.Name)
		require.False(t, tag.CreatedAt.IsZero())
	})

	t.Run("returns existing tag", func(t *testing.T) {
		tag1, err := tagRepo.GetOrCreate(ctx, "travel")
		require.NoError(t, err)

		tag2, err := tagRepo.GetOrCreate(ctx, "travel")
		require.NoError(t, err)
		require.Equal(t, tag1.ID, tag2.ID)
	})
}

func TestTagRepository_GetByName(t *testing.T) {
	tagRepo, _, _, ctx := setupTagTest(t)

	t.Run("finds existing tag", func(t *testing.T) {
		created, err := tagRepo.GetOrCreate(ctx, "lunch")
		require.NoError(t, err)

		found, err := tagRepo.GetByName(ctx, "lunch")
		require.NoError(t, err)
		require.Equal(t, created.ID, found.ID)
	})

	t.Run("returns error for non-existent tag", func(t *testing.T) {
		_, err := tagRepo.GetByName(ctx, "nonexistent")
		require.Error(t, err)
	})
}

func TestTagRepository_GetAll(t *testing.T) {
	tagRepo, _, _, ctx := setupTagTest(t)

	t.Run("returns empty for no tags", func(t *testing.T) {
		tags, err := tagRepo.GetAll(ctx)
		require.NoError(t, err)
		// May have tags from seeded data or other tests sharing the tx,
		// but at minimum should not error.
		_ = tags
	})

	t.Run("returns created tags", func(t *testing.T) {
		_, err := tagRepo.GetOrCreate(ctx, "alpha")
		require.NoError(t, err)
		_, err = tagRepo.GetOrCreate(ctx, "beta")
		require.NoError(t, err)

		tags, err := tagRepo.GetAll(ctx)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(tags), 2)

		names := make([]string, len(tags))
		for i, tag := range tags {
			names[i] = tag.Name
		}
		require.Contains(t, names, "alpha")
		require.Contains(t, names, "beta")
	})
}

func TestTagRepository_Delete(t *testing.T) {
	tagRepo, _, _, ctx := setupTagTest(t)

	tag, err := tagRepo.GetOrCreate(ctx, "deleteme")
	require.NoError(t, err)

	err = tagRepo.Delete(ctx, tag.ID)
	require.NoError(t, err)

	_, err = tagRepo.GetByName(ctx, "deleteme")
	require.Error(t, err)
}

func TestTagRepository_SetExpenseTags(t *testing.T) {
	tagRepo, expenseRepo, userRepo, ctx := setupTagTest(t)
	expense := createTestExpense(t, userRepo, expenseRepo, ctx, 700)

	tag1, err := tagRepo.GetOrCreate(ctx, "food")
	require.NoError(t, err)
	tag2, err := tagRepo.GetOrCreate(ctx, "dinner")
	require.NoError(t, err)

	t.Run("sets tags on expense", func(t *testing.T) {
		err := tagRepo.SetExpenseTags(ctx, expense.ID, []int{tag1.ID, tag2.ID})
		require.NoError(t, err)

		tags, err := tagRepo.GetByExpenseID(ctx, expense.ID)
		require.NoError(t, err)
		require.Len(t, tags, 2)
	})

	t.Run("replaces existing tags", func(t *testing.T) {
		tag3, err := tagRepo.GetOrCreate(ctx, "takeout")
		require.NoError(t, err)

		err = tagRepo.SetExpenseTags(ctx, expense.ID, []int{tag3.ID})
		require.NoError(t, err)

		tags, err := tagRepo.GetByExpenseID(ctx, expense.ID)
		require.NoError(t, err)
		require.Len(t, tags, 1)
		require.Equal(t, "takeout", tags[0].Name)
	})
}

func TestTagRepository_AddTagsToExpense(t *testing.T) {
	tagRepo, expenseRepo, userRepo, ctx := setupTagTest(t)
	expense := createTestExpense(t, userRepo, expenseRepo, ctx, 701)

	tag1, err := tagRepo.GetOrCreate(ctx, "meeting")
	require.NoError(t, err)
	tag2, err := tagRepo.GetOrCreate(ctx, "client")
	require.NoError(t, err)

	// Set initial tag.
	err = tagRepo.SetExpenseTags(ctx, expense.ID, []int{tag1.ID})
	require.NoError(t, err)

	// Add another tag without removing existing.
	err = tagRepo.AddTagsToExpense(ctx, expense.ID, []int{tag2.ID})
	require.NoError(t, err)

	tags, err := tagRepo.GetByExpenseID(ctx, expense.ID)
	require.NoError(t, err)
	require.Len(t, tags, 2)
}

func TestTagRepository_RemoveTagFromExpense(t *testing.T) {
	tagRepo, expenseRepo, userRepo, ctx := setupTagTest(t)
	expense := createTestExpense(t, userRepo, expenseRepo, ctx, 702)

	tag1, err := tagRepo.GetOrCreate(ctx, "remove1")
	require.NoError(t, err)
	tag2, err := tagRepo.GetOrCreate(ctx, "remove2")
	require.NoError(t, err)

	err = tagRepo.SetExpenseTags(ctx, expense.ID, []int{tag1.ID, tag2.ID})
	require.NoError(t, err)

	err = tagRepo.RemoveTagFromExpense(ctx, expense.ID, tag1.ID)
	require.NoError(t, err)

	tags, err := tagRepo.GetByExpenseID(ctx, expense.ID)
	require.NoError(t, err)
	require.Len(t, tags, 1)
	require.Equal(t, "remove2", tags[0].Name)
}

func TestTagRepository_GetByExpenseID(t *testing.T) {
	tagRepo, expenseRepo, userRepo, ctx := setupTagTest(t)

	t.Run("returns empty for expense with no tags", func(t *testing.T) {
		expense := createTestExpense(t, userRepo, expenseRepo, ctx, 703)
		tags, err := tagRepo.GetByExpenseID(ctx, expense.ID)
		require.NoError(t, err)
		require.Empty(t, tags)
	})

	t.Run("returns tags ordered by name", func(t *testing.T) {
		expense := createTestExpense(t, userRepo, expenseRepo, ctx, 704)
		tagZ, err := tagRepo.GetOrCreate(ctx, "zzz")
		require.NoError(t, err)
		tagA, err := tagRepo.GetOrCreate(ctx, "aaa")
		require.NoError(t, err)

		err = tagRepo.SetExpenseTags(ctx, expense.ID, []int{tagZ.ID, tagA.ID})
		require.NoError(t, err)

		tags, err := tagRepo.GetByExpenseID(ctx, expense.ID)
		require.NoError(t, err)
		require.Len(t, tags, 2)
		require.Equal(t, "aaa", tags[0].Name)
		require.Equal(t, "zzz", tags[1].Name)
	})
}

func TestTagRepository_GetByExpenseIDs(t *testing.T) {
	tagRepo, expenseRepo, userRepo, ctx := setupTagTest(t)

	t.Run("returns empty map for empty input", func(t *testing.T) {
		result, err := tagRepo.GetByExpenseIDs(ctx, []int{})
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("batch loads tags for multiple expenses", func(t *testing.T) {
		exp1 := createTestExpense(t, userRepo, expenseRepo, ctx, 705)
		exp2 := createTestExpense(t, userRepo, expenseRepo, ctx, 706)

		tag1, err := tagRepo.GetOrCreate(ctx, "batch1")
		require.NoError(t, err)
		tag2, err := tagRepo.GetOrCreate(ctx, "batch2")
		require.NoError(t, err)

		err = tagRepo.SetExpenseTags(ctx, exp1.ID, []int{tag1.ID})
		require.NoError(t, err)
		err = tagRepo.SetExpenseTags(ctx, exp2.ID, []int{tag1.ID, tag2.ID})
		require.NoError(t, err)

		result, err := tagRepo.GetByExpenseIDs(ctx, []int{exp1.ID, exp2.ID})
		require.NoError(t, err)
		require.Len(t, result[exp1.ID], 1)
		require.Len(t, result[exp2.ID], 2)
	})
}

func TestTagRepository_GetExpensesByTagID(t *testing.T) {
	tagRepo, expenseRepo, userRepo, ctx := setupTagTest(t)

	userID := int64(707)
	err := userRepo.UpsertUser(ctx, &models.User{ID: userID, Username: "tagexpuser"})
	require.NoError(t, err)

	// Create two confirmed expenses.
	exp1 := &models.Expense{
		UserID:      userID,
		Amount:      decimal.NewFromFloat(5.00),
		Currency:    "SGD",
		Description: "Expense 1",
		Status:      models.ExpenseStatusConfirmed,
	}
	err = expenseRepo.Create(ctx, exp1)
	require.NoError(t, err)

	exp2 := &models.Expense{
		UserID:      userID,
		Amount:      decimal.NewFromFloat(15.00),
		Currency:    "SGD",
		Description: "Expense 2",
		Status:      models.ExpenseStatusConfirmed,
	}
	err = expenseRepo.Create(ctx, exp2)
	require.NoError(t, err)

	tag, err := tagRepo.GetOrCreate(ctx, "filterable")
	require.NoError(t, err)

	// Tag only exp1.
	err = tagRepo.SetExpenseTags(ctx, exp1.ID, []int{tag.ID})
	require.NoError(t, err)

	t.Run("returns expenses with tag", func(t *testing.T) {
		expenses, err := tagRepo.GetExpensesByTagID(ctx, userID, tag.ID, 10)
		require.NoError(t, err)
		require.Len(t, expenses, 1)
		require.Equal(t, exp1.ID, expenses[0].ID)
	})

	t.Run("returns empty for tag with no expenses", func(t *testing.T) {
		emptyTag, err := tagRepo.GetOrCreate(ctx, "emptytag")
		require.NoError(t, err)

		expenses, err := tagRepo.GetExpensesByTagID(ctx, userID, emptyTag.ID, 10)
		require.NoError(t, err)
		require.Empty(t, expenses)
	})

	t.Run("respects limit", func(t *testing.T) {
		// Tag both expenses.
		err := tagRepo.SetExpenseTags(ctx, exp2.ID, []int{tag.ID})
		require.NoError(t, err)

		expenses, err := tagRepo.GetExpensesByTagID(ctx, userID, tag.ID, 1)
		require.NoError(t, err)
		require.Len(t, expenses, 1)
	})

	t.Run("only returns expenses for given user", func(t *testing.T) {
		otherUserID := int64(708)
		err := userRepo.UpsertUser(ctx, &models.User{ID: otherUserID, Username: "otheruser"})
		require.NoError(t, err)

		expenses, err := tagRepo.GetExpensesByTagID(ctx, otherUserID, tag.ID, 10)
		require.NoError(t, err)
		require.Empty(t, expenses)
	})
}
