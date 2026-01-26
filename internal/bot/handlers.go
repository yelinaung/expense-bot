package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
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

*Quick Start:*
• Send an expense like: `+"`5.50 Coffee`"+`
• Or use structured format: `+"`/add 5.50 Coffee Food - Dining Out`"+`

Use /help to see all available commands.`,
		formatGreeting(firstName))

	_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: models.ParseModeMarkdown,
	})
}

// handleHelp handles the /help command.
func (b *Bot) handleHelp(_ context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	text := `📚 *Available Commands*

*Expense Tracking:*
• ` + "`/add <amount> <description> [category]`" + ` - Add an expense
• Just send a message like ` + "`5.50 Coffee`" + ` to quickly add

*Viewing Expenses:*
• ` + "`/list`" + ` - Show recent expenses
• ` + "`/today`" + ` - Show today's expenses
• ` + "`/week`" + ` - Show this week's expenses

*Categories:*
• ` + "`/categories`" + ` - List all categories

*Other:*
• ` + "`/help`" + ` - Show this help message`

	_, _ = tgBot.SendMessage(context.Background(), &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: models.ParseModeMarkdown,
	})
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
	sb.WriteString("📁 *Expense Categories*\n\n")
	for i, cat := range categories {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, cat.Name))
	}

	_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      sb.String(),
		ParseMode: models.ParseModeMarkdown,
	})
}

// formatGreeting returns a greeting suffix with the user's name.
func formatGreeting(firstName string) string {
	if firstName == "" {
		return ""
	}
	return ", " + firstName
}
