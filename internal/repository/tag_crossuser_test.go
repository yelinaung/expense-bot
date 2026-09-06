package repository

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

// TestTagRepository_GetByNameForUser verifies that the user-scoped name lookup
// cannot serve as a cross-user tag-name existence oracle. Commit 78c3e36
// ("sec: harden bot output handling and user-scoped tags") user-scoped the
// /tags listing (GetAll -> GetAllByUserID) but left the /tags <name> and
// /untag <id> <name> handlers resolving tag names via the global GetByName,
// leaking whether a tag name is used by any other user. GetByNameForUser
// closes that leak: a tag name used only by another user is indistinguishable
// from a globally-unused name.
func TestTagRepository_GetByNameForUser(t *testing.T) {
	tagRepo, expenseRepo, userRepo, ctx := setupTagTest(t)

	const alice int64 = 72001
	const bob int64 = 72002
	require.NoError(t, userRepo.UpsertUser(ctx, &models.User{ID: alice, Username: "alice"}))
	require.NoError(t, userRepo.UpsertUser(ctx, &models.User{ID: bob, Username: "bob"}))

	// Alice creates an expense and tags it with a name she would prefer
	// others not know she uses.
	aliceExp := &models.Expense{
		UserID:      alice,
		Amount:      decimal.NewFromFloat(42.00),
		Currency:    testCurrencySGD,
		Description: "alice expense",
		Status:      models.ExpenseStatusConfirmed,
	}
	require.NoError(t, expenseRepo.Create(ctx, aliceExp))
	secret, err := tagRepo.GetOrCreate(ctx, "acmeacquisition")
	require.NoError(t, err)
	require.NoError(t, tagRepo.AddTagsToExpense(ctx, aliceExp.ID, []int{secret.ID}))

	t.Run("returns tag for owner who uses it", func(t *testing.T) {
		got, err := tagRepo.GetByNameForUser(ctx, alice, "acmeacquisition")
		require.NoError(t, err)
		require.Equal(t, secret.ID, got.ID)
		require.Equal(t, "acmeacquisition", got.Name)
		require.False(t, got.CreatedAt.IsZero())
	})

	t.Run("resolves tag attached only to a draft expense of the owner", func(t *testing.T) {
		// GetByNameForUser matches the listing's trust boundary: any of the
		// user's expenses (confirmed or draft) counts as "uses the tag". The
		// status filter lives only in GetExpensesByTagID, preserving current
		// behavior for the owner.
		aliceDraft := &models.Expense{
			UserID:      alice,
			Amount:      decimal.NewFromFloat(7.00),
			Currency:    testCurrencySGD,
			Description: "alice draft",
			Status:      models.ExpenseStatusDraft,
		}
		require.NoError(t, expenseRepo.Create(ctx, aliceDraft))
		draftOnly, err := tagRepo.GetOrCreate(ctx, "draftonlytag")
		require.NoError(t, err)
		require.NoError(t, tagRepo.AddTagsToExpense(ctx, aliceDraft.ID, []int{draftOnly.ID}))

		got, err := tagRepo.GetByNameForUser(ctx, alice, "draftonlytag")
		require.NoError(t, err)
		require.Equal(t, draftOnly.ID, got.ID)

		// GetExpensesByTagID still filters by confirmed status, so a tag
		// attached only to a draft expense yields no expenses for the owner —
		// identical to the pre-fix behavior.
		expenses, err := tagRepo.GetExpensesByTagID(ctx, alice, draftOnly.ID, 20)
		require.NoError(t, err)
		require.Empty(t, expenses)
	})

	t.Run("does not leak other users tag existence", func(t *testing.T) {
		// Bob has NO expenses and NO tags of his own. Alice's tag name must
		// NOT be visible to Bob's request path — the core security invariant.
		// A tag name used only by another user and a globally-unused name both
		// error here, so the user-facing handlers render identical "not found"
		// replies and cannot serve as an existence oracle.
		_, err := tagRepo.GetByNameForUser(ctx, bob, "acmeacquisition")
		require.Error(t, err, "another user's tag name must not be resolved for Bob")
	})
}
