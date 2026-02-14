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
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/config"
)

func TestBotDefaultHandler_NilMessage(t *testing.T) {
	t.Parallel()

	b := &Bot{pendingEdits: make(map[int64]*pendingEdit)}
	b.defaultHandler(context.Background(), nil, &tgmodels.Update{})
}

func TestBotDownloadFile(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("ok-bytes"))
		}))
		defer server.Close()

		mockBot := mocks.NewMockBot()
		mockBot.FileDownloadLinkToReturn = server.URL

		b := &Bot{}
		data, err := b.downloadFile(context.Background(), mockBot, "file-1")
		require.NoError(t, err)
		require.Equal(t, []byte("ok-bytes"), data)
	})

	t.Run("get file error", func(t *testing.T) {
		t.Parallel()

		mockBot := mocks.NewMockBot()
		mockBot.GetFileError = errors.New("boom")

		b := &Bot{}
		data, err := b.downloadFile(context.Background(), mockBot, "file-1")
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
		data, err := b.downloadFile(context.Background(), mockBot, "file-1")
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
		data, err := b.downloadFile(context.Background(), mockBot, "file-1")
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
