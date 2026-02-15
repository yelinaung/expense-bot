// Package bot provides the Telegram bot initialization and handlers.
package bot

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"gitlab.com/yelinaung/expense-bot/internal/config"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/exchange"
	"gitlab.com/yelinaung/expense-bot/internal/gemini"
	"gitlab.com/yelinaung/expense-bot/internal/logger"
	"gitlab.com/yelinaung/expense-bot/internal/models"
	"gitlab.com/yelinaung/expense-bot/internal/repository"
)

var downloadClient = &http.Client{
	Timeout: 30 * time.Second,
}

const maxDownloadBytes = 10 << 20

// pendingEdit represents a pending edit operation waiting for user input.
type pendingEdit struct {
	ExpenseID int
	EditType  string // "amount" or "category"
	MessageID int    // Message ID to edit after update.
}

// Bot wraps the Telegram bot with application dependencies.
type Bot struct {
	bot              *bot.Bot
	cfg              *config.Config
	db               database.PGXDB
	userRepo         *repository.UserRepository
	categoryRepo     *repository.CategoryRepository
	expenseRepo      *repository.ExpenseRepository
	tagRepo          *repository.TagRepository
	approvedUserRepo *repository.ApprovedUserRepository
	bindingRepo      *repository.SuperadminBindingRepository
	geminiClient     *gemini.Client

	messageSender   TelegramAPI
	exchangeService exchange.Service
	displayLocation *time.Location

	pendingEdits   map[int64]*pendingEdit // key is chatID
	pendingEditsMu sync.RWMutex

	// Category cache to reduce database queries.
	categoryCache       []models.Category
	categoryCacheExpiry time.Time
	categoryCacheMu     sync.RWMutex
}

// New creates a new Bot instance.
func New(cfg *config.Config, db database.PGXDB) (*Bot, error) {
	bindingRepo := repository.NewSuperadminBindingRepository(db)
	bindings, err := bindingRepo.LoadAll(context.Background())
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to load superadmin bindings from DB")
	} else if len(bindings) > 0 {
		configBindings := make([]config.SuperadminBinding, len(bindings))
		for i, b := range bindings {
			configBindings[i] = config.SuperadminBinding{
				Username: b.Username,
				UserID:   b.UserID,
			}
		}
		cfg.LoadSuperadminBindings(configBindings)
		logger.Log.Info().Int("count", len(bindings)).Msg("Loaded superadmin bindings from DB")
	}

	b := &Bot{
		cfg:              cfg,
		db:               db,
		userRepo:         repository.NewUserRepository(db),
		categoryRepo:     repository.NewCategoryRepository(db),
		expenseRepo:      repository.NewExpenseRepository(db),
		tagRepo:          repository.NewTagRepository(db),
		approvedUserRepo: repository.NewApprovedUserRepository(db),
		bindingRepo:      bindingRepo,
		pendingEdits:     make(map[int64]*pendingEdit),
	}

	if cfg.GeminiAPIKey != "" {
		geminiClient, err := gemini.NewClient(context.Background(), cfg.GeminiAPIKey)
		if err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to create Gemini client, receipt OCR disabled")
		} else {
			b.geminiClient = geminiClient
			logger.Log.Info().Msg("Gemini client initialized for receipt OCR")
		}
	}

	opts := []bot.Option{
		bot.WithMiddlewares(b.whitelistMiddleware),
		bot.WithDefaultHandler(b.defaultHandler),
	}

	telegramBot, err := bot.New(cfg.TelegramBotToken, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	b.bot = telegramBot
	b.messageSender = telegramBot

	loc, err := time.LoadLocation(cfg.ReminderTimezone)
	if err != nil {
		loc = time.UTC
	}
	b.displayLocation = loc

	b.registerHandlers()

	return b, nil
}

const (
	// DraftExpirationTimeout is the duration after which draft expenses are deleted.
	DraftExpirationTimeout = 10 * time.Minute
	// DraftCleanupInterval is how often the cleanup goroutine runs.
	DraftCleanupInterval = 5 * time.Minute
	// CategoryCacheTTL is how long category cache remains valid.
	CategoryCacheTTL = 5 * time.Minute
)

// Start begins polling for updates.
func (b *Bot) Start(ctx context.Context) {
	// Clear any existing webhook/polling sessions to avoid conflicts.
	// This helps when restarting or during rolling deploys.
	_, err := b.bot.DeleteWebhook(ctx, &bot.DeleteWebhookParams{
		DropPendingUpdates: false,
	})
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear webhook (may be expected)")
	}

	b.cleanupExpiredDrafts(ctx)

	go b.startDraftCleanupLoop(ctx)
	go b.startDailyReminderLoop(ctx)

	logger.Log.Info().Msg("Bot started polling")
	b.bot.Start(ctx)
}

// cleanupExpiredDrafts removes draft expenses older than DraftExpirationTimeout.
func (b *Bot) cleanupExpiredDrafts(ctx context.Context) {
	count, err := b.expenseRepo.DeleteExpiredDrafts(ctx, DraftExpirationTimeout)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to cleanup expired drafts")
		return
	}
	if count > 0 {
		logger.Log.Info().Int("count", count).Msg("Cleaned up expired draft expenses")
	}
}

// startDraftCleanupLoop runs periodic cleanup of expired draft expenses.
func (b *Bot) startDraftCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(DraftCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Log.Info().Msg("Draft cleanup loop stopped")
			return
		case <-ticker.C:
			b.cleanupExpiredDrafts(ctx)
		}
	}
}

// registerHandlers sets up command handlers.
func (b *Bot) registerHandlers() {
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypePrefix, b.handleStart)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/help", bot.MatchTypePrefix, b.handleHelp)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/categories", bot.MatchTypePrefix, b.handleCategories)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/add", bot.MatchTypePrefix, b.handleAdd)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/list", bot.MatchTypePrefix, b.handleList)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/today", bot.MatchTypePrefix, b.handleToday)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/week", bot.MatchTypePrefix, b.handleWeek)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/category", bot.MatchTypePrefix, b.handleCategory)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/report", bot.MatchTypePrefix, b.handleReport)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/chart", bot.MatchTypePrefix, b.handleChart)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/addcategory", bot.MatchTypePrefix, b.handleAddCategory)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/renamecategory", bot.MatchTypePrefix, b.handleRenameCategory)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/deletecategory", bot.MatchTypePrefix, b.handleDeleteCategory)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/edit", bot.MatchTypePrefix, b.handleEdit)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/delete", bot.MatchTypePrefix, b.handleDelete)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/setcurrency", bot.MatchTypePrefix, b.handleSetCurrency)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/currency", bot.MatchTypePrefix, b.handleShowCurrency)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/untag", bot.MatchTypePrefix, b.handleUntag)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/tags", bot.MatchTypePrefix, b.handleTags)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/tag", bot.MatchTypePrefix, b.handleTag)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/approve", bot.MatchTypePrefix, b.handleApprove)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/revoke", bot.MatchTypePrefix, b.handleRevoke)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/users", bot.MatchTypePrefix, b.handleUsers)

	// Callback query handlers for receipt confirmation flow.
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "receipt_", bot.MatchTypePrefix, b.handleReceiptCallback)
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "edit_", bot.MatchTypePrefix, b.handleEditCallback)
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "set_category_", bot.MatchTypePrefix, b.handleSetCategoryCallback)
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "cancel_edit_", bot.MatchTypePrefix, b.handleCancelEditCallback)
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "create_category_", bot.MatchTypePrefix, b.handleCreateCategoryCallback)

	// Callback query handlers for inline expense actions.
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "edit_expense_", bot.MatchTypePrefix, b.handleExpenseActionCallback)
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "delete_expense_", bot.MatchTypePrefix, b.handleExpenseActionCallback)
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "confirm_delete_", bot.MatchTypePrefix, b.handleConfirmDeleteCallback)
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "back_to_expense_", bot.MatchTypePrefix, b.handleBackToExpenseCallback)
}

// isAuthorized checks if a user is a superadmin or a DB-approved user.
// Returns false on DB errors (fail closed).
func (b *Bot) isAuthorized(ctx context.Context, userID int64, username string) bool {
	isSuperAdmin, newBinding := b.cfg.CheckSuperAdmin(userID, username)
	if isSuperAdmin {
		if newBinding != nil {
			go func() {
				if err := b.bindingRepo.Save(context.Background(), newBinding.Username, newBinding.UserID); err != nil {
					logger.Log.Error().Err(err).
						Str("username", newBinding.Username).
						Int64("user_id", newBinding.UserID).
						Msg("Failed to persist superadmin binding")
				} else {
					logger.Log.Info().
						Str("username", newBinding.Username).
						Int64("user_id", newBinding.UserID).
						Msg("Persisted superadmin binding; consider adding user_id to WHITELISTED_USER_IDS")
				}
			}()
		}
		return true
	}

	approved, needsBackfill, err := b.approvedUserRepo.IsApproved(ctx, userID, username)
	if err != nil {
		logger.Log.Error().Err(err).
			Int64("user_id", userID).
			Msg("Failed to check approved status, denying access")
		return false
	}
	if needsBackfill {
		// Backfill user_id for username-only approved users (fire-and-forget).
		go func() {
			if err := b.approvedUserRepo.UpdateUserID(context.Background(), username, userID); err != nil {
				logger.Log.Debug().Err(err).Str("username", username).Msg("Failed to backfill user ID")
			}
		}()
	}
	return approved
}

// whitelistMiddleware checks if the user is whitelisted before processing.
func (b *Bot) whitelistMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, tgBot *bot.Bot, update *tgmodels.Update) {
		userID := extractUserID(update)
		if userID == 0 {
			return
		}

		username := extractUsername(update)
		logUserAction(userID, username, update)

		if !b.isAuthorized(ctx, userID, username) {
			logger.Log.Warn().
				Int64("user_id", userID).
				Str("username", username).
				Msg("Blocked non-whitelisted user")
			if update.Message != nil {
				_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text:   "⛔ Sorry, you are not authorized to use this bot.",
				})
			}
			return
		}

		if err := b.ensureUserRegistered(ctx, update); err != nil {
			logger.Log.Error().
				Int64("user_id", userID).
				Err(err).
				Msg("Failed to register user")
		}

		next(ctx, tgBot, update)
	}
}

// logUserAction logs the user's input/action.
// Set log_level=debug for it to show up.
func logUserAction(userID int64, username string, update *tgmodels.Update) {
	switch {
	case update.Message != nil:
		msg := update.Message
		event := logger.Log.Debug().
			Int64("user_id", userID).
			Str("username", username).
			Int64("chat_id", msg.Chat.ID)

		if msg.Text != "" {
			event = event.Str("text", logger.SanitizeText(msg.Text))
		}
		if len(msg.Photo) > 0 {
			event = event.Str("type", "photo")
		}
		if msg.Document != nil {
			event = event.Str("type", "document").Str("filename", msg.Document.FileName)
		}
		if msg.Voice != nil {
			event = event.Str("type", "voice").Int("duration", msg.Voice.Duration)
		}

		event.Msg("User input")

	case update.CallbackQuery != nil:
		logger.Log.Debug().
			Int64("user_id", userID).
			Str("username", username).
			Str("data", update.CallbackQuery.Data).
			Msg("Callback query")

	case update.EditedMessage != nil:
		logger.Log.Debug().
			Int64("user_id", userID).
			Str("username", username).
			Str("text", logger.SanitizeText(update.EditedMessage.Text)).
			Msg("Edited message")
	}
}

// extractUsername gets the username from the update.
func extractUsername(update *tgmodels.Update) string {
	if update.Message != nil && update.Message.From != nil {
		return update.Message.From.Username
	}
	if update.CallbackQuery != nil {
		return update.CallbackQuery.From.Username
	}
	if update.EditedMessage != nil && update.EditedMessage.From != nil {
		return update.EditedMessage.From.Username
	}
	return ""
}

// ensureUserRegistered creates or updates the user record.
func (b *Bot) ensureUserRegistered(ctx context.Context, update *tgmodels.Update) error {
	var user *models.User

	if update.Message != nil && update.Message.From != nil {
		from := update.Message.From
		user = &models.User{
			ID:        from.ID,
			Username:  from.Username,
			FirstName: from.FirstName,
			LastName:  from.LastName,
		}
	} else if update.CallbackQuery != nil {
		from := update.CallbackQuery.From
		user = &models.User{
			ID:        from.ID,
			Username:  from.Username,
			FirstName: from.FirstName,
			LastName:  from.LastName,
		}
	}

	if user == nil {
		return nil
	}

	if err := b.userRepo.UpsertUser(ctx, user); err != nil {
		return fmt.Errorf("failed to upsert user: %w", err)
	}
	return nil
}

// extractUserID gets the user ID from various update types.
func extractUserID(update *tgmodels.Update) int64 {
	if update.Message != nil && update.Message.From != nil {
		return update.Message.From.ID
	}
	if update.CallbackQuery != nil {
		return update.CallbackQuery.From.ID
	}
	if update.EditedMessage != nil && update.EditedMessage.From != nil {
		return update.EditedMessage.From.ID
	}
	return 0
}

// defaultHandler handles unrecognized messages, attempting free-text expense parsing.
func (b *Bot) defaultHandler(ctx context.Context, tgBot *bot.Bot, update *tgmodels.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	logger.Log.Debug().
		Int64("chat_id", chatID).
		Str("text", logger.SanitizeText(update.Message.Text)).
		Msg("Default handler triggered")

	if update.Message.Voice != nil {
		b.handleVoice(ctx, tgBot, update)
		return
	}

	if len(update.Message.Photo) > 0 {
		b.handlePhoto(ctx, tgBot, update)
		return
	}

	// Check for pending edit operations first.
	if b.handlePendingEdit(ctx, tgBot, update) {
		return
	}

	if b.handleFreeTextExpense(ctx, tgBot, update) {
		return
	}

	_, err := tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      "I didn't understand that. Use /help to see available commands, or send an expense like <code>5.50 Coffee</code>",
		ParseMode: tgmodels.ParseModeHTML,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send default response")
	}
}

// downloadFile downloads a file from Telegram servers.
func (b *Bot) downloadFile(ctx context.Context, tg TelegramAPI, fileID string) ([]byte, error) {
	file, err := tg.GetFile(ctx, &bot.GetFileParams{
		FileID: fileID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	downloadURL := tg.FileDownloadLink(file)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := downloadClient.Do(req) // #nosec G704 -- URL comes from Telegram's FileDownloadLink API, not user input.
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownloadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read file data: %w", err)
	}
	if len(data) > maxDownloadBytes {
		return nil, fmt.Errorf("downloaded file exceeds size limit (%d bytes)", maxDownloadBytes)
	}

	return data, nil
}

// getCategoriesWithCache returns categories from cache if valid, otherwise fetches from DB.
func (b *Bot) getCategoriesWithCache(ctx context.Context) ([]models.Category, error) {
	// Try reading from cache first.
	b.categoryCacheMu.RLock()
	if time.Now().Before(b.categoryCacheExpiry) && b.categoryCache != nil {
		categories := b.categoryCache
		b.categoryCacheMu.RUnlock()
		logger.Log.Debug().Msg("Categories served from cache")
		return categories, nil
	}
	b.categoryCacheMu.RUnlock()

	// Cache miss or expired, fetch from database.
	b.categoryCacheMu.Lock()
	defer b.categoryCacheMu.Unlock()

	// Double-check after acquiring write lock (another goroutine might have updated it).
	if time.Now().Before(b.categoryCacheExpiry) && b.categoryCache != nil {
		logger.Log.Debug().Msg("Categories served from cache after lock")
		return b.categoryCache, nil
	}

	categories, err := b.categoryRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch categories: %w", err)
	}

	// Update cache.
	b.categoryCache = categories
	b.categoryCacheExpiry = time.Now().Add(CategoryCacheTTL)
	logger.Log.Debug().Int("count", len(categories)).Msg("Categories cached")

	return categories, nil
}

// invalidateCategoryCache clears the category cache, forcing a refresh on next access.
func (b *Bot) invalidateCategoryCache() {
	b.categoryCacheMu.Lock()
	defer b.categoryCacheMu.Unlock()
	b.categoryCache = nil
	b.categoryCacheExpiry = time.Time{}
	logger.Log.Debug().Msg("Category cache invalidated")
}
