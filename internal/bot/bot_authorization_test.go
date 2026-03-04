package bot

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestIsAuthorized(t *testing.T) {
	ctx := context.Background()
	pool := testDB(ctx, t)
	b := setupTestBot(t, pool)

	t.Run("allows configured superadmin", func(t *testing.T) {
		b.cfg.WhitelistedUserIDs = []int64{9001}
		require.True(t, b.isAuthorized(ctx, 9001, "superadmin"))
	})

	t.Run("allows approved user", func(t *testing.T) {
		userID := int64(9002)
		require.NoError(t, b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        userID,
			Username:  "approved-user",
			FirstName: "Approved",
		}))
		require.NoError(t, b.approvedUserRepo.Approve(ctx, userID, "approved-user", 1))
		require.True(t, b.isAuthorized(ctx, userID, "approved-user"))
	})

	t.Run("denies unapproved user", func(t *testing.T) {
		require.False(t, b.isAuthorized(ctx, 9999, "not-approved"))
	})

	t.Run("returns false when context is canceled", func(t *testing.T) {
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()
		require.False(t, b.isAuthorized(canceledCtx, 9002, "approved-user"))
	})

	t.Run("username-only approval is accepted and backfilled", func(t *testing.T) {
		username := "username-only"
		userID := int64(9010)
		require.NoError(t, b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        userID,
			Username:  username,
			FirstName: "Backfill",
		}))
		require.NoError(t, b.approvedUserRepo.ApproveByUsername(ctx, username, 1))

		require.True(t, b.isAuthorized(ctx, userID, username))
	})
}
