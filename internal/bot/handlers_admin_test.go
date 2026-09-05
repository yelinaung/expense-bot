package bot

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/config"
	"gitlab.com/yelinaung/expense-bot/internal/repository"
	"gitlab.com/yelinaung/expense-bot/internal/testutil/dbtest"
)

const (
	superadminUsername              = "superadmin"
	superadminFirstName             = "Super"
	superadminLastName              = "Admin"
	nonSuperadminRejectedAdminTest  = "non-superadmin rejected"
	onlySuperadminsTextAdminTest    = "Only superadmins"
	usersCommandAdminTest           = "/users"
	regularUsernameAdminTest        = "regular"
	regularFirstNameAdminTest       = "Regular"
	regularLastNameAdminTest        = "User"
	revokedTextAdminTest            = "revoked"
	approvedTextAdminTest           = "approved"
	superadminCannotRevokeAdminTest = "Superadmins cannot be revoked"
	superadminsTextAdminTest        = "Superadmins"
	approveByIDCmdAdminTest         = "/approve 12345"
)

func TestHandleApproveCore(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.TestTx(ctx, t)

	cfg := &config.Config{
		WhitelistedUserIDs:   []int64{100},
		WhitelistedUsernames: []string{superadminUsername},
	}
	b := &Bot{
		cfg:              cfg,
		approvedUserRepo: repository.NewApprovedUserRepository(tx),
		pendingEdits:     make(map[int64]*pendingEdit),
	}

	t.Run("nil message", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.NewUpdateBuilder().Build()
		b.handleApproveCore(ctx, mockBot, update)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run(nonSuperadminRejectedAdminTest, func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.NewUpdateBuilder().
			WithMessage(1, 999, approveByIDCmdAdminTest).
			WithFrom(999, regularUsernameAdminTest, regularFirstNameAdminTest, regularLastNameAdminTest).
			Build()
		b.handleApproveCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, onlySuperadminsTextAdminTest)
	})

	t.Run("no args shows usage", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.NewUpdateBuilder().
			WithMessage(1, 100, "/approve").
			WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
			Build()
		b.handleApproveCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "Usage")
	})

	t.Run("approve by ID", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.NewUpdateBuilder().
			WithMessage(1, 100, approveByIDCmdAdminTest).
			WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
			Build()
		b.handleApproveCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "12345")
		require.Contains(t, mockBot.LastSentMessage().Text, approvedTextAdminTest)

		approved, _, err := b.approvedUserRepo.IsApproved(ctx, 12345, "")
		require.NoError(t, err)
		require.True(t, approved)
	})

	t.Run("approve by @username", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.NewUpdateBuilder().
			WithMessage(1, 100, "/approve @newuser").
			WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
			Build()
		b.handleApproveCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "@newuser")
		require.Contains(t, mockBot.LastSentMessage().Text, approvedTextAdminTest)

		approved, _, err := b.approvedUserRepo.IsApproved(ctx, 0, "newuser")
		require.NoError(t, err)
		require.True(t, approved)
	})
}

func TestHandleRevokeCore(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.TestTx(ctx, t)

	cfg := &config.Config{
		WhitelistedUserIDs:   []int64{100},
		WhitelistedUsernames: []string{superadminUsername},
	}
	b := &Bot{
		cfg:              cfg,
		approvedUserRepo: repository.NewApprovedUserRepository(tx),
		pendingEdits:     make(map[int64]*pendingEdit),
	}

	t.Run(nonSuperadminRejectedAdminTest, func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.NewUpdateBuilder().
			WithMessage(1, 999, "/revoke 12345").
			WithFrom(999, regularUsernameAdminTest, regularFirstNameAdminTest, regularLastNameAdminTest).
			Build()
		b.handleRevokeCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, onlySuperadminsTextAdminTest)
	})

	t.Run("revoke by ID", func(t *testing.T) {
		// First approve a user.
		err := b.approvedUserRepo.Approve(ctx, 22222, "", 100)
		require.NoError(t, err)

		mockBot := mocks.NewMockBot()
		update := mocks.NewUpdateBuilder().
			WithMessage(1, 100, "/revoke 22222").
			WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
			Build()
		b.handleRevokeCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "22222")
		require.Contains(t, mockBot.LastSentMessage().Text, revokedTextAdminTest)

		approved, _, err := b.approvedUserRepo.IsApproved(ctx, 22222, "")
		require.NoError(t, err)
		require.False(t, approved)
	})

	t.Run("revoke by @username", func(t *testing.T) {
		err := b.approvedUserRepo.ApproveByUsername(ctx, "revokeuser", 100)
		require.NoError(t, err)

		mockBot := mocks.NewMockBot()
		update := mocks.NewUpdateBuilder().
			WithMessage(1, 100, "/revoke @revokeuser").
			WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
			Build()
		b.handleRevokeCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "@revokeuser")
		require.Contains(t, mockBot.LastSentMessage().Text, revokedTextAdminTest)
	})

	t.Run("cannot revoke superadmin by ID", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.NewUpdateBuilder().
			WithMessage(1, 100, "/revoke 100").
			WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
			Build()
		b.handleRevokeCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, superadminCannotRevokeAdminTest)
	})

	t.Run("cannot revoke superadmin by username", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.NewUpdateBuilder().
			WithMessage(1, 100, "/revoke @superadmin").
			WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
			Build()
		b.handleRevokeCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, superadminCannotRevokeAdminTest)
	})
}

// TestHandleRevokeCore_SentinelInputs covers the operator-error hazard where
// "/revoke 0" (userID sentinel 0) and "/revoke @" (username sentinel "") used
// to bulk-delete the opposite half of the approval list and reply with a
// misleading per-user confirmation. After the fix, both must be treated as
// invalid usage and must delete zero rows.
func TestHandleRevokeCore_SentinelInputs(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.TestTx(ctx, t)

	cfg := &config.Config{
		WhitelistedUserIDs:   []int64{100},
		WhitelistedUsernames: []string{superadminUsername},
	}
	b := &Bot{
		cfg:              cfg,
		approvedUserRepo: repository.NewApprovedUserRepository(tx),
		pendingEdits:     make(map[int64]*pendingEdit),
	}

	byIDs := []int64{11111, 22222, 33333}
	byUsernames := []string{"alice", "bob", "carol"}

	seed := func(t *testing.T) {
		t.Helper()
		for _, id := range byIDs {
			require.NoError(t, b.approvedUserRepo.Approve(ctx, id, "", 100))
		}
		for _, u := range byUsernames {
			require.NoError(t, b.approvedUserRepo.ApproveByUsername(ctx, u, 100))
		}
	}
	count := func(t *testing.T) int {
		t.Helper()
		users, err := b.approvedUserRepo.GetAll(ctx)
		require.NoError(t, err)
		return len(users)
	}

	t.Run("/revoke 0 is usage error and mass-deletes nothing", func(t *testing.T) {
		seed(t)
		require.Equal(t, 6, count(t))

		mockBot := mocks.NewMockBot()
		update := mocks.NewUpdateBuilder().
			WithMessage(1, 100, "/revoke 0").
			WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
			Build()
		b.handleRevokeCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "Usage")
		require.NotContains(t, mockBot.LastSentMessage().Text, "has been revoked", "must not confirm a sentinel revoke")
		require.Equal(t, 6, count(t), "/revoke 0 must delete zero rows")

		for _, u := range byUsernames {
			ok, _, _ := b.approvedUserRepo.IsApproved(ctx, 0, u)
			require.True(t, ok, "username-only approval %q must survive /revoke 0", u)
		}
		for _, id := range byIDs {
			ok, _, _ := b.approvedUserRepo.IsApproved(ctx, id, "")
			require.True(t, ok, "by-ID approval %d must survive /revoke 0", id)
		}
	})

	t.Run("/revoke @ is usage error and mass-deletes nothing", func(t *testing.T) {
		seed(t)
		require.Equal(t, 6, count(t))

		mockBot := mocks.NewMockBot()
		update := mocks.NewUpdateBuilder().
			WithMessage(1, 100, "/revoke @").
			WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
			Build()
		b.handleRevokeCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "Usage")
		require.NotContains(t, mockBot.LastSentMessage().Text, "has been revoked", "must not confirm a sentinel revoke")
		require.Equal(t, 6, count(t), "/revoke @ must delete zero rows")

		for _, id := range byIDs {
			ok, _, _ := b.approvedUserRepo.IsApproved(ctx, id, "")
			require.True(t, ok, "by-ID approval %d must survive /revoke @", id)
		}
		for _, u := range byUsernames {
			ok, _, _ := b.approvedUserRepo.IsApproved(ctx, 0, u)
			require.True(t, ok, "username-only approval %q must survive /revoke @", u)
		}
	})
}

func TestHandleUsersCore(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.TestTx(ctx, t)

	cfg := &config.Config{
		WhitelistedUserIDs:   []int64{100},
		WhitelistedUsernames: []string{superadminUsername},
	}
	b := &Bot{
		cfg:              cfg,
		approvedUserRepo: repository.NewApprovedUserRepository(tx),
		pendingEdits:     make(map[int64]*pendingEdit),
	}

	t.Run(nonSuperadminRejectedAdminTest, func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.NewUpdateBuilder().
			WithMessage(1, 999, usersCommandAdminTest).
			WithFrom(999, regularUsernameAdminTest, regularFirstNameAdminTest, regularLastNameAdminTest).
			Build()
		b.handleUsersCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, onlySuperadminsTextAdminTest)
	})

	t.Run("lists superadmins and empty approved", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.NewUpdateBuilder().
			WithMessage(1, 100, usersCommandAdminTest).
			WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
			Build()
		b.handleUsersCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage().Text
		require.Contains(t, msg, superadminsTextAdminTest)
		require.Contains(t, msg, "100")
		require.Contains(t, msg, "@superadmin")
		require.Contains(t, msg, "(none)")
	})

	t.Run("lists superadmins and approved users", func(t *testing.T) {
		err := b.approvedUserRepo.Approve(ctx, 55555, "frank", 100)
		require.NoError(t, err)

		mockBot := mocks.NewMockBot()
		update := mocks.NewUpdateBuilder().
			WithMessage(1, 100, usersCommandAdminTest).
			WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
			Build()
		b.handleUsersCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage().Text
		require.Contains(t, msg, superadminsTextAdminTest)
		require.Contains(t, msg, "Approved Users")
		require.Contains(t, msg, "55555")
		require.Contains(t, msg, "@frank")
	})
}
