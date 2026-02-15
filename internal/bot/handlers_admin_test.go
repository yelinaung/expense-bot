package bot

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/config"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/repository"
)

const (
	superadminUsername             = "superadmin"
	superadminFirstName            = "Super"
	superadminLastName             = "Admin"
	nonSuperadminRejectedAdminTest = "non-superadmin rejected"
	onlySuperadminsTextAdminTest   = "Only superadmins"
	usersCommandAdminTest          = "/users"
)

func TestHandleApproveCore(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

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
			WithMessage(1, 999, "/approve 12345").
			WithFrom(999, "regular", "Regular", "User").
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
			WithMessage(1, 100, "/approve 12345").
			WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
			Build()
		b.handleApproveCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "12345")
		require.Contains(t, mockBot.LastSentMessage().Text, "approved")

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
		require.Contains(t, mockBot.LastSentMessage().Text, "approved")

		approved, _, err := b.approvedUserRepo.IsApproved(ctx, 0, "newuser")
		require.NoError(t, err)
		require.True(t, approved)
	})
}

func TestHandleRevokeCore(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

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
			WithFrom(999, "regular", "Regular", "User").
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
		require.Contains(t, mockBot.LastSentMessage().Text, "revoked")

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
		require.Contains(t, mockBot.LastSentMessage().Text, "revoked")
	})

	t.Run("cannot revoke superadmin by ID", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.NewUpdateBuilder().
			WithMessage(1, 100, "/revoke 100").
			WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
			Build()
		b.handleRevokeCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "Superadmins cannot be revoked")
	})

	t.Run("cannot revoke superadmin by username", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := mocks.NewUpdateBuilder().
			WithMessage(1, 100, "/revoke @superadmin").
			WithFrom(100, superadminUsername, superadminFirstName, superadminLastName).
			Build()
		b.handleRevokeCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "Superadmins cannot be revoked")
	})
}

func TestHandleUsersCore(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

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
			WithFrom(999, "regular", "Regular", "User").
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
		require.Contains(t, msg, "Superadmins")
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
		require.Contains(t, msg, "Superadmins")
		require.Contains(t, msg, "Approved Users")
		require.Contains(t, msg, "55555")
		require.Contains(t, msg, "@frank")
	})
}
