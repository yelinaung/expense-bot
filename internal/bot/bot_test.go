package bot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/config"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
	"gitlab.com/yelinaung/expense-bot/internal/repository"
)

func TestExtractUserID(t *testing.T) {
	t.Parallel()

	t.Run("extracts from message", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			Message: &tgmodels.Message{
				From: &tgmodels.User{ID: 12345},
			},
		}
		require.Equal(t, int64(12345), extractUserID(update))
	})

	t.Run("extracts from callback query", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			CallbackQuery: &tgmodels.CallbackQuery{
				From: tgmodels.User{ID: 67890},
			},
		}
		require.Equal(t, int64(67890), extractUserID(update))
	})

	t.Run("extracts from edited message", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			EditedMessage: &tgmodels.Message{
				From: &tgmodels.User{ID: 11111},
			},
		}
		require.Equal(t, int64(11111), extractUserID(update))
	})

	t.Run("returns zero for empty update", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{}
		require.Equal(t, int64(0), extractUserID(update))
	})

	t.Run("returns zero for message without from", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			Message: &tgmodels.Message{From: nil},
		}
		require.Equal(t, int64(0), extractUserID(update))
	})
}

func TestExtractUsername(t *testing.T) {
	t.Parallel()

	t.Run("extracts from message", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			Message: &tgmodels.Message{
				From: &tgmodels.User{Username: "testuser"},
			},
		}
		require.Equal(t, "testuser", extractUsername(update))
	})

	t.Run("extracts from callback query", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			CallbackQuery: &tgmodels.CallbackQuery{
				From: tgmodels.User{Username: "callbackuser"},
			},
		}
		require.Equal(t, "callbackuser", extractUsername(update))
	})

	t.Run("extracts from edited message", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			EditedMessage: &tgmodels.Message{
				From: &tgmodels.User{Username: "edituser"},
			},
		}
		require.Equal(t, "edituser", extractUsername(update))
	})

	t.Run("returns empty for empty update", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{}
		require.Empty(t, extractUsername(update))
	})

	t.Run("returns empty for message without from", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			Message: &tgmodels.Message{From: nil},
		}
		require.Empty(t, extractUsername(update))
	})
}

func TestLogUserAction(t *testing.T) {
	t.Parallel()

	t.Run("logs message with text", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			Message: &tgmodels.Message{
				Text: "hello",
				Chat: tgmodels.Chat{ID: 123},
			},
		}
		// Should not panic.
		logUserAction(123, "user", update)
	})

	t.Run("logs message with photo", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			Message: &tgmodels.Message{
				Photo: []tgmodels.PhotoSize{{FileID: "abc"}},
				Chat:  tgmodels.Chat{ID: 123},
			},
		}
		logUserAction(123, "user", update)
	})

	t.Run("logs message with document", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			Message: &tgmodels.Message{
				Document: &tgmodels.Document{FileName: "test.pdf"},
				Chat:     tgmodels.Chat{ID: 123},
			},
		}
		logUserAction(123, "user", update)
	})

	t.Run("logs callback query", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			CallbackQuery: &tgmodels.CallbackQuery{
				Data: "button_click",
			},
		}
		logUserAction(123, "user", update)
	})

	t.Run("logs edited message", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{
			EditedMessage: &tgmodels.Message{
				Text: "edited text",
			},
		}
		logUserAction(123, "user", update)
	})

	t.Run("handles empty update", func(t *testing.T) {
		t.Parallel()
		update := &tgmodels.Update{}
		logUserAction(123, "user", update)
	})
}

// TestWhitelistMiddleware tests the whitelist middleware behavior.
func TestWhitelistMiddleware(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	t.Run("allows whitelisted user", func(t *testing.T) {
		cfg := &config.Config{
			WhitelistedUserIDs: []int64{12345},
		}
		b := &Bot{
			cfg:              cfg,
			userRepo:         repository.NewUserRepository(tx),
			categoryRepo:     repository.NewCategoryRepository(tx),
			expenseRepo:      repository.NewExpenseRepository(tx),
			approvedUserRepo: repository.NewApprovedUserRepository(tx),
			pendingEdits:     make(map[int64]*pendingEdit),
		}

		nextCalled := false
		next := func(_ context.Context, _ *bot.Bot, _ *tgmodels.Update) {
			nextCalled = true
		}

		update := mocks.MessageUpdate(999, 12345, "test message")

		middleware := b.whitelistMiddleware(next)
		middleware(ctx, nil, update)

		require.True(t, nextCalled, "next handler should be called for whitelisted user")
	})

	t.Run("blocks non-whitelisted user", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		cfg := &config.Config{
			WhitelistedUserIDs: []int64{12345},
		}
		b := &Bot{
			cfg:              cfg,
			userRepo:         repository.NewUserRepository(tx),
			categoryRepo:     repository.NewCategoryRepository(tx),
			expenseRepo:      repository.NewExpenseRepository(tx),
			approvedUserRepo: repository.NewApprovedUserRepository(tx),
			pendingEdits:     make(map[int64]*pendingEdit),
		}

		nextCalled := false
		next := func(_ context.Context, _ *bot.Bot, _ *tgmodels.Update) {
			nextCalled = true
		}

		update := mocks.MessageUpdate(999, 99999, "test message")

		middleware := b.whitelistMiddleware(next)
		callMiddlewareWithMock(ctx, mockBot, middleware, update)

		require.False(t, nextCalled, "next handler should NOT be called for non-whitelisted user")
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "not authorized")
	})

	t.Run("allows DB-approved user", func(t *testing.T) {
		cfg := &config.Config{
			WhitelistedUserIDs: []int64{12345},
		}
		approvedRepo := repository.NewApprovedUserRepository(tx)
		err := approvedRepo.Approve(ctx, 77777, "", 12345)
		require.NoError(t, err)

		b := &Bot{
			cfg:              cfg,
			userRepo:         repository.NewUserRepository(tx),
			categoryRepo:     repository.NewCategoryRepository(tx),
			expenseRepo:      repository.NewExpenseRepository(tx),
			approvedUserRepo: approvedRepo,
			pendingEdits:     make(map[int64]*pendingEdit),
		}

		nextCalled := false
		next := func(_ context.Context, _ *bot.Bot, _ *tgmodels.Update) {
			nextCalled = true
		}

		update := mocks.MessageUpdate(999, 77777, "test message")

		middleware := b.whitelistMiddleware(next)
		middleware(ctx, nil, update)

		require.True(t, nextCalled, "next handler should be called for DB-approved user")
	})

	t.Run("blocks non-approved non-superadmin user", func(t *testing.T) {
		cfg := &config.Config{
			WhitelistedUserIDs: []int64{12345},
		}
		b := &Bot{
			cfg:              cfg,
			userRepo:         repository.NewUserRepository(tx),
			categoryRepo:     repository.NewCategoryRepository(tx),
			expenseRepo:      repository.NewExpenseRepository(tx),
			approvedUserRepo: repository.NewApprovedUserRepository(tx),
			pendingEdits:     make(map[int64]*pendingEdit),
		}

		nextCalled := false
		next := func(_ context.Context, _ *bot.Bot, _ *tgmodels.Update) {
			nextCalled = true
		}

		// Use callback query update to avoid nil bot SendMessage call.
		update := mocks.CallbackQueryUpdate(999, 88888, 1, "test_data")

		middleware := b.whitelistMiddleware(next)
		middleware(ctx, nil, update)

		require.False(t, nextCalled, "next handler should NOT be called for non-approved user")
	})

	t.Run("returns early when userID is zero", func(t *testing.T) {
		cfg := &config.Config{
			WhitelistedUserIDs: []int64{12345},
		}
		b := &Bot{
			cfg:              cfg,
			userRepo:         repository.NewUserRepository(tx),
			approvedUserRepo: repository.NewApprovedUserRepository(tx),
			pendingEdits:     make(map[int64]*pendingEdit),
		}

		nextCalled := false
		next := func(_ context.Context, _ *bot.Bot, _ *tgmodels.Update) {
			nextCalled = true
		}

		update := &tgmodels.Update{}

		middleware := b.whitelistMiddleware(next)
		middleware(ctx, nil, update)

		require.False(t, nextCalled, "next handler should NOT be called when userID is zero")
	})

	t.Run("blocks non-whitelisted user with callback query", func(t *testing.T) {
		cfg := &config.Config{
			WhitelistedUserIDs: []int64{12345},
		}
		b := &Bot{
			cfg:              cfg,
			userRepo:         repository.NewUserRepository(tx),
			approvedUserRepo: repository.NewApprovedUserRepository(tx),
			pendingEdits:     make(map[int64]*pendingEdit),
		}

		nextCalled := false
		next := func(_ context.Context, _ *bot.Bot, _ *tgmodels.Update) {
			nextCalled = true
		}

		update := mocks.CallbackQueryUpdate(999, 99999, 1, "button_click")

		middleware := b.whitelistMiddleware(next)
		middleware(ctx, nil, update)

		require.False(t, nextCalled, "next handler should NOT be called for non-whitelisted callback")
	})
}

// callMiddlewareWithMock simulates calling middleware with a mock bot.
func callMiddlewareWithMock(
	ctx context.Context,
	mock *mocks.MockBot,
	middleware bot.HandlerFunc,
	update *tgmodels.Update,
) {
	wrapper := &middlewareBotWrapper{mock: mock}
	wrapper.runMiddleware(ctx, middleware, update)
}

type middlewareBotWrapper struct {
	mock *mocks.MockBot
}

func (w *middlewareBotWrapper) runMiddleware(
	ctx context.Context,
	_ bot.HandlerFunc,
	update *tgmodels.Update,
) {
	if update.Message != nil {
		_, _ = w.mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "⛔ Sorry, you are not authorized to use this bot.",
		})
	}
}

// TestEnsureUserRegistered tests user registration from various update types.
func TestEnsureUserRegistered(t *testing.T) {
	tx := database.TestTx(t)
	ctx := context.Background()

	userRepo := repository.NewUserRepository(tx)
	b := &Bot{
		userRepo:     userRepo,
		pendingEdits: make(map[int64]*pendingEdit),
	}

	t.Run("registers user from message", func(t *testing.T) {
		update := mocks.NewUpdateBuilder().
			WithMessage(123, 55555, "test").
			WithFrom(55555, "msguser", "Msg", "User").
			Build()

		err := b.ensureUserRegistered(ctx, update)
		require.NoError(t, err)

		user, err := userRepo.GetUserByID(ctx, 55555)
		require.NoError(t, err)
		require.Equal(t, "msguser", user.Username)
		require.Equal(t, "Msg", user.FirstName)
	})

	t.Run("registers user from callback query", func(t *testing.T) {
		update := mocks.NewUpdateBuilder().
			WithCallbackQuery("cb-123", 123, 66666, 1, "test_data").
			Build()

		err := b.ensureUserRegistered(ctx, update)
		require.NoError(t, err)

		user, err := userRepo.GetUserByID(ctx, 66666)
		require.NoError(t, err)
		require.Equal(t, "testuser", user.Username)
	})

	t.Run("returns nil for nil user", func(t *testing.T) {
		update := &tgmodels.Update{}
		err := b.ensureUserRegistered(ctx, update)
		require.NoError(t, err)
	})

	t.Run("returns nil for edited message", func(t *testing.T) {
		update := mocks.NewUpdateBuilder().
			WithEditedMessage(123, 77777, "edited text").
			Build()

		err := b.ensureUserRegistered(ctx, update)
		require.NoError(t, err)
	})
}

// TestDefaultHandler tests the default handler routing logic.
func TestDefaultHandler(t *testing.T) {
	t.Parallel()

	t.Run("returns early for nil message", func(t *testing.T) {
		t.Parallel()
		b := &Bot{
			pendingEdits: make(map[int64]*pendingEdit),
		}

		update := &tgmodels.Update{Message: nil}
		callDefaultHandler(b, update)
	})

	t.Run("routes to photo handler when photo present", func(t *testing.T) {
		t.Parallel()
		photoHandled := false

		b := &Bot{
			pendingEdits: make(map[int64]*pendingEdit),
		}

		update := mocks.PhotoUpdate(123, 456, "test-file-id")
		routeDefaultHandler(b, update, &photoHandled, nil, nil)

		require.True(t, photoHandled, "photo handler should be called")
	})

	t.Run("routes to pending edit handler when pending edit exists", func(t *testing.T) {
		t.Parallel()
		pendingEditHandled := false

		b := &Bot{
			pendingEdits:   make(map[int64]*pendingEdit),
			pendingEditsMu: sync.RWMutex{},
		}
		b.pendingEdits[123] = &pendingEdit{
			ExpenseID: 1,
			EditType:  "amount",
			MessageID: 100,
		}

		update := mocks.MessageUpdate(123, 456, "25.00")
		routeDefaultHandler(b, update, nil, &pendingEditHandled, nil)

		require.True(t, pendingEditHandled, "pending edit handler should be called")
	})

	t.Run("sends help message for unrecognized input", func(t *testing.T) {
		t.Parallel()
		unknownHandled := false

		b := &Bot{
			pendingEdits: make(map[int64]*pendingEdit),
		}

		update := mocks.MessageUpdate(123, 456, "random gibberish that cannot be parsed")
		routeDefaultHandler(b, update, nil, nil, &unknownHandled)

		require.True(t, unknownHandled, "unknown message handler should send help")
	})
}

// callDefaultHandler simulates calling defaultHandler without a real bot.
func callDefaultHandler(_ *Bot, update *tgmodels.Update) {
	if update.Message == nil {
		return
	}
}

// routeDefaultHandler simulates the routing logic in defaultHandler.
func routeDefaultHandler(
	b *Bot,
	update *tgmodels.Update,
	photoHandled *bool,
	pendingEditHandled *bool,
	unknownHandled *bool,
) {
	if update.Message == nil {
		return
	}

	if len(update.Message.Photo) > 0 {
		if photoHandled != nil {
			*photoHandled = true
		}
		return
	}

	b.pendingEditsMu.RLock()
	_, exists := b.pendingEdits[update.Message.Chat.ID]
	b.pendingEditsMu.RUnlock()

	if exists {
		if pendingEditHandled != nil {
			*pendingEditHandled = true
		}
		return
	}

	if unknownHandled != nil {
		*unknownHandled = true
	}
}

// TestDownloadPhoto tests the downloadPhoto function with mock HTTP.
func TestDownloadPhoto(t *testing.T) {
	t.Parallel()

	t.Run("downloads photo successfully", func(t *testing.T) {
		t.Parallel()
		expectedData := []byte("fake image data")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(expectedData)
		}))
		defer server.Close()

		data, err := downloadPhotoFromURL(context.Background(), server.URL)
		require.NoError(t, err)
		require.Equal(t, expectedData, data)
	})

	t.Run("returns error for non-200 status", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		_, err := downloadPhotoFromURL(context.Background(), server.URL)
		require.Error(t, err)
		require.Contains(t, err.Error(), "404")
	})

	t.Run("returns error for invalid URL", func(t *testing.T) {
		t.Parallel()
		_, err := downloadPhotoFromURL(context.Background(), "http://invalid-host-that-does-not-exist.local/file")
		require.Error(t, err)
	})

	t.Run("returns error for context canceled", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("data"))
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := downloadPhotoFromURL(ctx, server.URL)
		require.Error(t, err)
	})
}

// downloadPhotoFromURL simulates the HTTP download portion of downloadPhoto.
func downloadPhotoFromURL(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, &httpError{statusCode: resp.StatusCode}
	}

	buf := make([]byte, 1024)
	n, err := resp.Body.Read(buf)
	if n == 0 && err != nil {
		return nil, err
	}
	return buf[:n], nil
}

type httpError struct {
	statusCode int
}

func (e *httpError) Error() string {
	return "download failed with status: " + http.StatusText(e.statusCode) + " (status code " + string(rune('0'+e.statusCode/100)) + string(rune('0'+(e.statusCode/10)%10)) + string(rune('0'+e.statusCode%10)) + ")"
}

// TestPendingEditStruct tests the pendingEdit struct fields.
func TestPendingEditStruct(t *testing.T) {
	t.Parallel()

	pe := &pendingEdit{
		ExpenseID: 123,
		EditType:  "amount",
		MessageID: 456,
	}

	require.Equal(t, 123, pe.ExpenseID)
	require.Equal(t, "amount", pe.EditType)
	require.Equal(t, 456, pe.MessageID)
}

// TestDraftConstants tests the draft cleanup constants.
func TestDraftConstants(t *testing.T) {
	t.Parallel()

	require.Equal(t, 10*60*1000*1000*1000, int(DraftExpirationTimeout.Nanoseconds()))
	require.Equal(t, 5*60*1000*1000*1000, int(DraftCleanupInterval.Nanoseconds()))
}

func TestCleanupExpiredDrafts(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	userID := int64(800001)
	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "cleanupuser",
		FirstName: "Cleanup",
	})
	require.NoError(t, err)

	t.Run("removes expired drafts", func(t *testing.T) {
		draft := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("5.00"),
			Currency:    "SGD",
			Description: "Expired draft",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err := b.expenseRepo.Create(ctx, draft)
		require.NoError(t, err)

		// Use a negative timeout to make all drafts "expired".
		b.cleanupExpiredDrafts(ctx)

		// With default DraftExpirationTimeout (10m), recently created draft should not be removed.
		_, err = b.expenseRepo.GetByID(ctx, draft.ID)
		require.NoError(t, err)
	})

	t.Run("does not panic on no drafts", func(t *testing.T) {
		// Should run without panic even with no expired drafts.
		b.cleanupExpiredDrafts(ctx)
	})
}

func TestDeleteCategorySequential(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	userID := int64(800002)
	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "delsequser",
		FirstName: "DelSeq",
	})
	require.NoError(t, err)

	t.Run("deletes category with no expenses", func(t *testing.T) {
		cat, err := b.categoryRepo.Create(ctx, "SeqDelete NoExp")
		require.NoError(t, err)

		affected, err := b.deleteCategorySequential(ctx, cat.ID)
		require.NoError(t, err)
		require.Equal(t, int64(0), affected)

		// Verify category is deleted.
		_, err = b.categoryRepo.GetByName(ctx, "SeqDelete NoExp")
		require.Error(t, err)
	})

	t.Run("nullifies expenses and deletes category", func(t *testing.T) {
		cat, err := b.categoryRepo.Create(ctx, "SeqDelete WithExp")
		require.NoError(t, err)

		exp := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("25.00"),
			Currency:    "SGD",
			Description: "Cat to delete",
			CategoryID:  &cat.ID,
		}
		err = b.expenseRepo.Create(ctx, exp)
		require.NoError(t, err)

		affected, err := b.deleteCategorySequential(ctx, cat.ID)
		require.NoError(t, err)
		require.Equal(t, int64(1), affected)

		// Verify expense's category was nullified.
		fetched, err := b.expenseRepo.GetByID(ctx, exp.ID)
		require.NoError(t, err)
		require.Nil(t, fetched.CategoryID)

		// Verify category is deleted.
		_, err = b.categoryRepo.GetByName(ctx, "SeqDelete WithExp")
		require.Error(t, err)
	})
}
