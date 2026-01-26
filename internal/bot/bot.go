// Package bot provides the Telegram bot initialization and handlers.
package bot

import (
	"context"
	"fmt"
	"log"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.com/yelinaung/expense-bot/internal/config"
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
	log.Println("Bot started polling...")
	b.bot.Start(ctx)
}

// registerHandlers sets up command handlers.
func (b *Bot) registerHandlers() {
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, b.handleStart)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/help", bot.MatchTypeExact, b.handleHelp)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/categories", bot.MatchTypeExact, b.handleCategories)
}

// whitelistMiddleware checks if the user is whitelisted before processing.
func (b *Bot) whitelistMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, tgBot *bot.Bot, update *tgmodels.Update) {
		userID := extractUserID(update)
		if userID == 0 {
			return
		}

		if !b.cfg.IsUserWhitelisted(userID) {
			if update.Message != nil {
				_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text:   "⛔ Sorry, you are not authorized to use this bot.",
				})
			}
			return
		}

		if err := b.ensureUserRegistered(ctx, update); err != nil {
			log.Printf("Failed to register user: %v", err)
		}

		next(ctx, tgBot, update)
	}
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

// defaultHandler handles unrecognized messages.
func (b *Bot) defaultHandler(ctx context.Context, tgBot *bot.Bot, update *tgmodels.Update) {
	if update.Message == nil {
		return
	}

	_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "I didn't understand that. Use /help to see available commands.",
	})
}
