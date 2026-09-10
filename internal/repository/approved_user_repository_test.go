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

// TestApprovedUserRepository_RevokeSentinelGuard ensures the Revoke delete paths
// apply the same sentinel exclusion the schema's partial unique indexes and
// IsApproved apply. The sentinel user_id = 0 marks every "approved by @username
// only" row and the sentinel empty username marks every "approved by ID only"
// row; deleting with either sentinel must NOT bulk-remove the matching half of
// the approval list.
func TestApprovedUserRepository_RevokeSentinelGuard(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.TestTx(ctx, t)

	repo := NewApprovedUserRepository(tx)

	byIDs := []int64{11111, 22222, 33333}
	byUsernames := []string{"alice", "bob", "carol"}

	for _, id := range byIDs {
		require.NoError(t, repo.Approve(ctx, id, "", 100))
	}
	for _, u := range byUsernames {
		require.NoError(t, repo.ApproveByUsername(ctx, u, 100))
	}

	count := func(t *testing.T) int {
		t.Helper()
		users, err := repo.GetAll(ctx)
		require.NoError(t, err)
		return len(users)
	}
	require.Equal(t, 6, count(t), "seeded approval list should hold 6 rows")

	t.Run("Revoke(0) does not delete username-only approvals", func(t *testing.T) {
		require.NoError(t, repo.Revoke(ctx, 0))
		require.Equal(t, 6, count(t), "Revoke(0) must not delete any rows")

		for _, u := range byUsernames {
			ok, _, err := repo.IsApproved(ctx, 0, u)
			require.NoError(t, err)
			require.True(t, ok, "username-only approval %q must survive Revoke(0)", u)
		}
		for _, id := range byIDs {
			ok, _, err := repo.IsApproved(ctx, id, "")
			require.NoError(t, err)
			require.True(t, ok, "by-ID approval %d must survive Revoke(0)", id)
		}
	})

	t.Run("RevokeByUsername(\"\") does not delete by-ID approvals", func(t *testing.T) {
		require.NoError(t, repo.RevokeByUsername(ctx, ""))
		require.Equal(t, 6, count(t), `RevokeByUsername("") must not delete any rows`)

		for _, id := range byIDs {
			ok, _, err := repo.IsApproved(ctx, id, "")
			require.NoError(t, err)
			require.True(t, ok, "by-ID approval %d must survive RevokeByUsername(\"\")", id)
		}
		for _, u := range byUsernames {
			ok, _, err := repo.IsApproved(ctx, 0, u)
			require.NoError(t, err)
			require.True(t, ok, "username-only approval %q must survive RevokeByUsername(\"\")", u)
		}
	})
}
