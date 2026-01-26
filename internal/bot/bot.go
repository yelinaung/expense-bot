// Package bot provides the Telegram bot initialization and handlers.
package bot

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.com/yelinaung/expense-bot/internal/config"
	"gitlab.com/yelinaung/expense-bot/internal/logger"
	"gitlab.com/yelinaung/expense-bot/internal/models"
	"gitlab.com/yelinaung/expense-bot/internal/repository"
)

// Bot wraps the Telegram bot with application dependencies.
type Bot struct {
	bot          *bot.Bot
	cfg          *config.Config
	userRepo     *repository.UserRepository
	categoryRepo *repository.CategoryRepository
	expenseRepo  *repository.ExpenseRepository
}

// New creates a new Bot instance.
func New(cfg *config.Config, pool *pgxpool.Pool) (*Bot, error) {
	b := &Bot{
		cfg:          cfg,
		userRepo:     repository.NewUserRepository(pool),
		categoryRepo: repository.NewCategoryRepository(pool),
		expenseRepo:  repository.NewExpenseRepository(pool),
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
	b.registerHandlers()

	return b, nil
}

// Start begins polling for updates.
func (b *Bot) Start(ctx context.Context) {
	logger.Log.Info().Msg("Bot started polling")
	b.bot.Start(ctx)
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
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/edit", bot.MatchTypePrefix, b.handleEdit)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/delete", bot.MatchTypePrefix, b.handleDelete)
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

		if !b.cfg.IsUserWhitelisted(userID) {
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
func logUserAction(userID int64, username string, update *tgmodels.Update) {
	switch {
	case update.Message != nil:
		msg := update.Message
		event := logger.Log.Info().
			Int64("user_id", userID).
			Str("username", username).
			Int64("chat_id", msg.Chat.ID)

		if msg.Text != "" {
			event = event.Str("text", msg.Text)
		}
		if len(msg.Photo) > 0 {
			event = event.Str("type", "photo")
		}
		if msg.Document != nil {
			event = event.Str("type", "document").Str("filename", msg.Document.FileName)
		}

		event.Msg("User input")

	case update.CallbackQuery != nil:
		logger.Log.Info().
			Int64("user_id", userID).
			Str("username", username).
			Str("data", update.CallbackQuery.Data).
			Msg("Callback query")

	case update.EditedMessage != nil:
		logger.Log.Info().
			Int64("user_id", userID).
			Str("username", username).
			Str("text", update.EditedMessage.Text).
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

	logger.Log.Debug().
		Int64("chat_id", update.Message.Chat.ID).
		Str("text", update.Message.Text).
		Msg("Default handler triggered")

	if b.handleFreeTextExpense(ctx, tgBot, update) {
		return
	}

	_, err := tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      "I didn't understand that. Use /help to see available commands, or send an expense like <code>5.50 Coffee</code>",
		ParseMode: tgmodels.ParseModeHTML,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send default response")
	}
}
