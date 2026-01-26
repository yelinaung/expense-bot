package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"gitlab.com/yelinaung/expense-bot/internal/logger"
)

// handleStart handles the /start command.
func (b *Bot) handleStart(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	firstName := ""
	if update.Message.From != nil {
		firstName = update.Message.From.FirstName
	}

	text := fmt.Sprintf(`👋 Welcome%s!

I'm your personal expense tracker bot. I help you track your daily expenses in SGD.

<b>Quick Start:</b>
• Send an expense like: <code>5.50 Coffee</code>
• Or use structured format: <code>/add 5.50 Coffee Food - Dining Out</code>

Use /help to see all available commands.`,
		formatGreeting(firstName))

	logger.Log.Debug().Int64("chat_id", update.Message.Chat.ID).Msg("Sending /start response")
	_, err := tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send /start response")
	}
}

// handleHelp handles the /help command.
func (b *Bot) handleHelp(_ context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	text := `📚 <b>Available Commands</b>

<b>Expense Tracking:</b>
• <code>/add &lt;amount&gt; &lt;description&gt; [category]</code> - Add an expense
• Just send a message like <code>5.50 Coffee</code> to quickly add

<b>Viewing Expenses:</b>
• <code>/list</code> - Show recent expenses
• <code>/today</code> - Show today's expenses
• <code>/week</code> - Show this week's expenses

<b>Categories:</b>
• <code>/categories</code> - List all categories

<b>Other:</b>
• <code>/help</code> - Show this help message`

	logger.Log.Debug().Int64("chat_id", update.Message.Chat.ID).Msg("Sending /help response")
	_, err := tgBot.SendMessage(context.Background(), &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send /help response")
	}
}

// handleCategories handles the /categories command.
func (b *Bot) handleCategories(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	categories, err := b.categoryRepo.GetAll(ctx)
	if err != nil {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Failed to fetch categories. Please try again.",
		})
		return
	}

	if len(categories) == 0 {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "No categories found.",
		})
		return
	}

	var sb strings.Builder
	sb.WriteString("📁 <b>Expense Categories</b>\n\n")
	for i, cat := range categories {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, cat.Name))
	}

	logger.Log.Debug().Int64("chat_id", update.Message.Chat.ID).Msg("Sending /categories response")
	_, err = tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      sb.String(),
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send /categories response")
	}
}

// formatGreeting returns a greeting suffix with the user's name.
func formatGreeting(firstName string) string {
	if firstName == "" {
		return ""
	}
	return ", " + firstName
}
