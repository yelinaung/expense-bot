package bot

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/config"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
	"gitlab.com/yelinaung/expense-bot/internal/repository"
)

func TestBotDefaultHandler_NilMessage(t *testing.T) {
	t.Parallel()

	b := &Bot{pendingEdits: make(map[int64]*pendingEdit)}
	b.defaultHandler(context.Background(), nil, &tgmodels.Update{})
}

func TestBotDownloadFile(t *testing.T) {
	t.Parallel()
	const fileID = "file-1"

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("ok-bytes"))
		}))
		defer server.Close()

		mockBot := mocks.NewMockBot()
		mockBot.FileDownloadLinkToReturn = server.URL

		b := &Bot{}
		data, err := b.downloadFile(context.Background(), mockBot, fileID)
		require.NoError(t, err)
		require.Equal(t, []byte("ok-bytes"), data)
	})

	t.Run("get file error", func(t *testing.T) {
		t.Parallel()

		mockBot := mocks.NewMockBot()
		mockBot.GetFileError = errors.New("boom")

		b := &Bot{}
		data, err := b.downloadFile(context.Background(), mockBot, fileID)
		require.Error(t, err)
		require.Nil(t, data)
		require.Contains(t, err.Error(), "failed to get file info")
	})

	t.Run("non 200 status", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer server.Close()

		mockBot := mocks.NewMockBot()
		mockBot.FileDownloadLinkToReturn = server.URL

		b := &Bot{}
		data, err := b.downloadFile(context.Background(), mockBot, fileID)
		require.Error(t, err)
		require.Nil(t, data)
		require.Contains(t, err.Error(), "download failed with status")
	})

	t.Run("too large", func(t *testing.T) {
		t.Parallel()

		oversized := strings.Repeat("a", maxDownloadBytes+1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(oversized))
		}))
		defer server.Close()

		mockBot := mocks.NewMockBot()
		mockBot.FileDownloadLinkToReturn = server.URL

		b := &Bot{}
		data, err := b.downloadFile(context.Background(), mockBot, fileID)
		require.Error(t, err)
		require.Nil(t, data)
		require.Contains(t, err.Error(), "exceeds size limit")
	})
}

func TestBotStartDraftCleanupLoop_CanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		(&Bot{}).startDraftCleanupLoop(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("startDraftCleanupLoop did not stop on canceled context")
	}
}

func TestBotRegisterHandlers_PanicsWithNilBot(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		(&Bot{}).registerHandlers()
	})
}

func TestBotStart_PanicsWithNilBot(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		(&Bot{}).Start(context.Background())
	})
}

func TestBotStart_CanceledContext_NoPanic(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)

	client := &fakeHTTPClient{}
	tgBot, err := tgbot.New(
		"123:TESTTOKEN",
		tgbot.WithSkipGetMe(),
		tgbot.WithHTTPClient(time.Second, client),
		tgbot.WithServerURL("http://example.com"),
	)
	require.NoError(t, err)

	b.bot = tgBot
	b.messageSender = tgBot
	b.cfg.DailyReminderEnabled = false

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NotPanics(t, func() {
		b.Start(ctx)
	})
}

func TestCleanupExpiredDrafts_DeletesOnlyExpired(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	user := &appmodels.User{
		ID:        123456,
		Username:  "cleanup-user",
		FirstName: "Cleanup",
		LastName:  "User",
	}
	err := b.userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	expired := &appmodels.Expense{
		UserID:      123456,
		Amount:      decimal.RequireFromString("10.00"),
		Currency:    "USD",
		Description: "old draft",
		Status:      appmodels.ExpenseStatusDraft,
	}
	err = b.expenseRepo.Create(ctx, expired)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `UPDATE expenses SET created_at = $2 WHERE id = $1`,
		expired.ID, time.Now().Add(-(DraftExpirationTimeout + time.Minute)))
	require.NoError(t, err)

	fresh := &appmodels.Expense{
		UserID:      123456,
		Amount:      decimal.RequireFromString("11.00"),
		Currency:    "USD",
		Description: "fresh draft",
		Status:      appmodels.ExpenseStatusDraft,
	}
	err = b.expenseRepo.Create(ctx, fresh)
	require.NoError(t, err)

	b.cleanupExpiredDrafts(ctx)

	_, err = b.expenseRepo.GetByID(ctx, expired.ID)
	require.Error(t, err)

	stillThere, err := b.expenseRepo.GetByID(ctx, fresh.ID)
	require.NoError(t, err)
	require.Equal(t, fresh.ID, stillThere.ID)
}

func TestCleanupExpiredDrafts_CanceledContext_NoDelete(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	baseCtx := context.Background()
	user := &appmodels.User{
		ID:        123456,
		Username:  "cleanup-cancel-user",
		FirstName: "Cleanup",
		LastName:  "Cancel",
	}
	err := b.userRepo.UpsertUser(baseCtx, user)
	require.NoError(t, err)

	expired := &appmodels.Expense{
		UserID:      123456,
		Amount:      decimal.RequireFromString("12.00"),
		Currency:    "USD",
		Description: "expired draft",
		Status:      appmodels.ExpenseStatusDraft,
	}
	err = b.expenseRepo.Create(baseCtx, expired)
	require.NoError(t, err)

	_, err = pool.Exec(baseCtx, `UPDATE expenses SET created_at = $2 WHERE id = $1`,
		expired.ID, time.Now().Add(-(DraftExpirationTimeout + time.Minute)))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(baseCtx)
	cancel()
	b.cleanupExpiredDrafts(ctx)

	stillThere, err := b.expenseRepo.GetByID(baseCtx, expired.ID)
	require.NoError(t, err)
	require.Equal(t, expired.ID, stillThere.ID)
}

func TestNew_InvalidTokenReturnsError(t *testing.T) {
	db := TestDB(t)

	cfg := &config.Config{
		TelegramBotToken: "",
		ReminderTimezone: "UTC",
	}

	b, err := New(cfg, db)
	require.Error(t, err)
	require.Nil(t, b)
}

func TestNew_LoadsPersistedSuperadminBindings(t *testing.T) {
	db := TestDB(t)
	ctx := context.Background()

	bindingRepo := repository.NewSuperadminBindingRepository(db)
	err := bindingRepo.Save(ctx, "alice", 42)
	require.NoError(t, err)

	cfg := &config.Config{
		TelegramBotToken:     "",
		ReminderTimezone:     "UTC",
		WhitelistedUsernames: []string{"alice"},
		GeminiAPIKey:         "test-key",
	}

	b, err := New(cfg, db)
	require.Error(t, err)
	require.Nil(t, b)

	require.True(t, cfg.IsSuperAdmin(42, "alice"))
	require.False(t, cfg.IsSuperAdmin(777, "alice"))
}

func TestHandleAdminAndTagWrappers_NilMessage(t *testing.T) {
	t.Parallel()

	b := &Bot{}
	ctx := context.Background()
	update := &tgmodels.Update{}
	var tgBot *tgbot.Bot

	b.handleApprove(ctx, tgBot, update)
	b.handleRevoke(ctx, tgBot, update)
	b.handleUsers(ctx, tgBot, update)
	b.handleRenameCategory(ctx, tgBot, update)
	b.handleDeleteCategory(ctx, tgBot, update)
	b.handleTag(ctx, tgBot, update)
	b.handleUntag(ctx, tgBot, update)
	b.handleTags(ctx, tgBot, update)
}
