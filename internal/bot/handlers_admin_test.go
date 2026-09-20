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

// TestHandleApproveCore_SentinelInputs covers the approve/revoke symmetry
// hazard introduced by 3b09610: that commit taught /revoke to reject the
// sentinels user_id = 0 ("/revoke 0") and username = "" ("/revoke @") but
// left /approve unguarded. Because the combined sentinel (user_id = 0,
// username = "") falls outside both partial unique indexes on
// approved_users, each /approve 0 or /approve @ call used to INSERT a fresh
// (0, "") orphan row that (a) is never deduplicated (outside both indexes),
// (b) grants no access (IsApproved excludes sentinels), and (c) cannot be
// removed via the bot (the /revoke DELETE statements exclude the sentinels,
// and handleRevokeCore rejects the sentinel inputs). After the fix, the
// same sentinel inputs that /revoke rejects must be rejected by /approve as
// invalid usage and must write zero rows.
func TestHandleApproveCore_SentinelInputs(t *testing.T) {
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
	// orphanCount counts the unremovable (user_id = 0, username = "") rows
	// the bug report describes. It must stay zero after every rejected
	// sentinel approve call.
	orphanCount := func(t *testing.T) int {
		t.Helper()
		users, err := b.approvedUserRepo.GetAll(ctx)
		require.NoError(t, err)
		var n int
		for _, u := range users {
			if u.UserID == 0 && u.Username == "" {
				n++
			}
		}
		return n
	}

	t.Run("/approve 0 is usage error and writes no orphan row", func(t *testing.T) {
		seed(t)
		require.Equal(t, 6, count(t))
		require.Equal(t, 0, orphanCount(t))

		mockBot := mocks.NewMockBot()
		update := mocks.NewUpdateBuilder().
			WithMessage(1, 100, "/approve 0").
			WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
			Build()
		b.handleApproveCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "Usage")
		require.NotContains(t, mockBot.LastSentMessage().Text, "has been approved", "must not confirm a sentinel approve")
		require.Equal(t, 6, count(t), "/approve 0 must write zero rows")
		require.Equal(t, 0, orphanCount(t), "/approve 0 must not mint a (0, \"\") orphan row")

		approved, _, err := b.approvedUserRepo.IsApproved(ctx, 0, "")
		require.NoError(t, err)
		require.False(t, approved, "sentinel approve must grant no access")
		for _, id := range byIDs {
			ok, _, _ := b.approvedUserRepo.IsApproved(ctx, id, "")
			require.True(t, ok, "by-ID approval %d must survive /approve 0", id)
		}
		for _, u := range byUsernames {
			ok, _, _ := b.approvedUserRepo.IsApproved(ctx, 0, u)
			require.True(t, ok, "username-only approval %q must survive /approve 0", u)
		}
	})

	t.Run("/approve @ is usage error and writes no orphan row", func(t *testing.T) {
		seed(t)
		require.Equal(t, 6, count(t))
		require.Equal(t, 0, orphanCount(t))

		mockBot := mocks.NewMockBot()
		update := mocks.NewUpdateBuilder().
			WithMessage(1, 100, "/approve @").
			WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
			Build()
		b.handleApproveCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "Usage")
		require.NotContains(t, mockBot.LastSentMessage().Text, "has been approved", "must not confirm a sentinel approve")
		require.Equal(t, 6, count(t), "/approve @ must write zero rows")
		require.Equal(t, 0, orphanCount(t), "/approve @ must not mint a (0, \"\") orphan row")

		approved, _, err := b.approvedUserRepo.IsApproved(ctx, 0, "")
		require.NoError(t, err)
		require.False(t, approved, "sentinel approve must grant no access")
		for _, id := range byIDs {
			ok, _, _ := b.approvedUserRepo.IsApproved(ctx, id, "")
			require.True(t, ok, "by-ID approval %d must survive /approve @", id)
		}
		for _, u := range byUsernames {
			ok, _, _ := b.approvedUserRepo.IsApproved(ctx, 0, u)
			require.True(t, ok, "username-only approval %q must survive /approve @", u)
		}
	})

	// Repeated sentinel calls must not accumulate orphans even when issued
	// back to back: before the fix each call inserted another (0, "") row.
	t.Run("repeated sentinel calls do not accumulate orphans", func(t *testing.T) {
		seed(t)
		require.Equal(t, 6, count(t))
		require.Equal(t, 0, orphanCount(t))

		for _, cmd := range []string{"/approve 0", "/approve @", "/approve 0", "/approve @"} {
			mockBot := mocks.NewMockBot()
			update := mocks.NewUpdateBuilder().
				WithMessage(1, 100, cmd).
				WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
				Build()
			b.handleApproveCore(ctx, mockBot, update)
			require.Equal(t, 1, mockBot.SentMessageCount(), "%q should reply once", cmd)
			require.Contains(t, mockBot.LastSentMessage().Text, "Usage", "%q should be a usage error", cmd)
			require.NotContains(t, mockBot.LastSentMessage().Text, "has been approved", "%q must not confirm", cmd)
		}
		require.Equal(t, 6, count(t), "four sentinel approve calls must write zero rows")
		require.Equal(t, 0, orphanCount(t), "no (0, \"\") orphan rows after repeated sentinel approve calls")
	})

	// ParseInt collapses "00", "+0" and "-0" to the same 0 sentinel, and
	// negative IDs are never real Telegram users, so the whole non-positive
	// range must be rejected before reaching Approve — mirroring /revoke,
	// and avoiding a misleading "User <code>0</code> has been approved."
	// reply.
	for _, cmd := range []string{"/approve 00", "/approve +0", "/approve -0", "/approve -1", "/approve -99999"} {
		t.Run(cmd+" is usage error and writes no row", func(t *testing.T) {
			seed(t)
			require.Equal(t, 6, count(t))
			require.Equal(t, 0, orphanCount(t))

			mockBot := mocks.NewMockBot()
			update := mocks.NewUpdateBuilder().
				WithMessage(1, 100, cmd).
				WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
				Build()
			b.handleApproveCore(ctx, mockBot, update)

			require.Equal(t, 1, mockBot.SentMessageCount())
			require.Contains(t, mockBot.LastSentMessage().Text, "Usage")
			require.NotContains(t, mockBot.LastSentMessage().Text, "has been approved", "must not confirm %q", cmd)
			require.Equal(t, 6, count(t), "%q must write zero rows", cmd)
			require.Equal(t, 0, orphanCount(t), "%q must not mint a (0, \"\") orphan row", cmd)
		})
	}
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

	// ParseInt collapses "00", "+0" and "-0" to the same 0 sentinel, and
	// negative IDs are never real Telegram users, so the whole non-positive
	// range must be rejected before reaching Revoke.
	for _, cmd := range []string{"/revoke 00", "/revoke +0", "/revoke -0", "/revoke -1", "/revoke -99999"} {
		t.Run(cmd+" is usage error and mass-deletes nothing", func(t *testing.T) {
			seed(t)
			require.Equal(t, 6, count(t))

			mockBot := mocks.NewMockBot()
			update := mocks.NewUpdateBuilder().
				WithMessage(1, 100, cmd).
				WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
				Build()
			b.handleRevokeCore(ctx, mockBot, update)

			require.Equal(t, 1, mockBot.SentMessageCount())
			require.Contains(t, mockBot.LastSentMessage().Text, "Usage")
			require.NotContains(t, mockBot.LastSentMessage().Text, "has been revoked", "must not confirm %q", cmd)
			require.Equal(t, 6, count(t), "%q must delete zero rows", cmd)
		})
	}
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

	// End-to-end check for the /users symptom in the bug report: a rejected
	// sentinel /approve must not produce the blank "  @\n" ghost line that
	// handleUsersCore's default branch emits for an (0, "") orphan row. The
	// pre-fix bug minted one such row per /approve 0 call, each surfacing as a
	// blank @ entry here.
	t.Run("no blank @ ghost line after rejected sentinel approve", func(t *testing.T) {
		// A real, non-orphan by-username approval so the Approved Users block
		// is exercised but only with a non-empty username.
		require.NoError(t, b.approvedUserRepo.ApproveByUsername(ctx, "ghostcheck", 100))

		approveBot := mocks.NewMockBot()
		approveUpdate := mocks.NewUpdateBuilder().
			WithMessage(1, 100, "/approve 0").
			WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
			Build()
		b.handleApproveCore(ctx, approveBot, approveUpdate)
		require.Equal(t, 1, approveBot.SentMessageCount())
		require.Contains(t, approveBot.LastSentMessage().Text, "Usage")

		usersBot := mocks.NewMockBot()
		usersUpdate := mocks.NewUpdateBuilder().
			WithMessage(1, 100, usersCommandAdminTest).
			WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
			Build()
		b.handleUsersCore(ctx, usersBot, usersUpdate)
		require.Equal(t, 1, usersBot.SentMessageCount())
		msg := usersBot.LastSentMessage().Text
		require.Contains(t, msg, "@ghostcheck", "legitimate username approval must still be listed")
		require.NotContains(t, msg, "  @\n", "/users must not emit blank @ ghost lines from orphan rows")
	})
}
