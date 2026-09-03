package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/testutil/dbtest"
)

func TestApprovedUserRepository_ApproveAndRevoke(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.TestTx(ctx, t)

	repo := NewApprovedUserRepository(tx)

	t.Run("approve by ID and revoke", func(t *testing.T) {
		err := repo.Approve(ctx, 11111, "alice", 99999)
		require.NoError(t, err)

		approved, _, err := repo.IsApproved(ctx, 11111, "")
		require.NoError(t, err)
		require.True(t, approved)

		err = repo.Revoke(ctx, 11111)
		require.NoError(t, err)

		approved, _, err = repo.IsApproved(ctx, 11111, "")
		require.NoError(t, err)
		require.False(t, approved)
	})

	t.Run("approve by username and revoke", func(t *testing.T) {
		err := repo.ApproveByUsername(ctx, "bob", 99999)
		require.NoError(t, err)

		approved, _, err := repo.IsApproved(ctx, 0, "bob")
		require.NoError(t, err)
		require.True(t, approved)

		err = repo.RevokeByUsername(ctx, "bob")
		require.NoError(t, err)

		approved, _, err = repo.IsApproved(ctx, 0, "bob")
		require.NoError(t, err)
		require.False(t, approved)
	})
}

func TestApprovedUserRepository_IsApproved(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.TestTx(ctx, t)

	repo := NewApprovedUserRepository(tx)

	t.Run("by user ID", func(t *testing.T) {
		err := repo.Approve(ctx, 22222, "", 99999)
		require.NoError(t, err)

		approved, needsBackfill, err := repo.IsApproved(ctx, 22222, "")
		require.NoError(t, err)
		require.True(t, approved)
		require.False(t, needsBackfill)
	})

	t.Run("by username needs backfill", func(t *testing.T) {
		err := repo.ApproveByUsername(ctx, "charlie", 99999)
		require.NoError(t, err)

		approved, needsBackfill, err := repo.IsApproved(ctx, 0, "charlie")
		require.NoError(t, err)
		require.True(t, approved)
		require.True(t, needsBackfill)
	})

	t.Run("case insensitive username", func(t *testing.T) {
		err := repo.ApproveByUsername(ctx, "Dave", 99999)
		require.NoError(t, err)

		approved, _, err := repo.IsApproved(ctx, 0, "dave")
		require.NoError(t, err)
		require.True(t, approved)

		approved, _, err = repo.IsApproved(ctx, 0, "DAVE")
		require.NoError(t, err)
		require.True(t, approved)
	})

	t.Run("returns false for unknown user", func(t *testing.T) {
		approved, needsBackfill, err := repo.IsApproved(ctx, 77777, "unknownuser")
		require.NoError(t, err)
		require.False(t, approved)
		require.False(t, needsBackfill)
	})
}

func TestApprovedUserRepository_GetAll(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.TestTx(ctx, t)

	repo := NewApprovedUserRepository(tx)

	t.Run("empty list", func(t *testing.T) {
		users, err := repo.GetAll(ctx)
		require.NoError(t, err)
		require.Empty(t, users)
	})

	t.Run("returns all entries", func(t *testing.T) {
		err := repo.Approve(ctx, 44444, "frank", 99999)
		require.NoError(t, err)
		err = repo.ApproveByUsername(ctx, "grace", 99999)
		require.NoError(t, err)

		users, err := repo.GetAll(ctx)
		require.NoError(t, err)
		require.Len(t, users, 2)
	})
}

func TestApprovedUserRepository_RecycledUsernameDoesNotInheritAccess(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.TestTx(ctx, t)

	repo := NewApprovedUserRepository(tx)

	// 1. Admin approves user 40001 (who currently owns @origuser) by user ID.
	err := repo.Approve(ctx, 40001, "origuser", 99999)
	require.NoError(t, err)

	// 2. Original user is still approved by their immutable user_id.
	approved, _, err := repo.IsApproved(ctx, 40001, "origuser")
	require.NoError(t, err)
	require.True(t, approved)

	// 3. An attacker claims @origuser (different user_id 40002).
	//    They must NOT inherit access from the row bound to user_id 40001.
	approved, _, err = repo.IsApproved(ctx, 40002, "origuser")
	require.NoError(t, err)
	require.False(t, approved, "recycled username must not inherit access when row is bound to a different user ID")
}

func TestApprovedUserRepository_ApproveDuplicate(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.TestTx(ctx, t)

	repo := NewApprovedUserRepository(tx)

	// Approve same user ID twice — should upsert, not error.
	err := repo.Approve(ctx, 55555, "hank", 99999)
	require.NoError(t, err)

	err = repo.Approve(ctx, 55555, "hank_updated", 99999)
	require.NoError(t, err)

	users, err := repo.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, users, 1)
	require.Equal(t, "hank_updated", users[0].Username)
}
