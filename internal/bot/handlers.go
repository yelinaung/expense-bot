package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"

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

// handleAdd handles the /add command for structured expense input.
func (b *Bot) handleAdd(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	categories, err := b.categoryRepo.GetAll(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch categories for parsing")
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to process expense. Please try again.",
		})
		return
	}

	categoryNames := make([]string, len(categories))
	for i, cat := range categories {
		categoryNames[i] = cat.Name
	}

	parsed := ParseAddCommandWithCategories(update.Message.Text, categoryNames)
	if parsed == nil {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Invalid format. Use: <code>/add 5.50 Coffee [category]</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	b.saveExpense(ctx, tgBot, chatID, userID, parsed, categories)
}

// handleFreeTextExpense handles free-text expense input like "5.50 Coffee".
func (b *Bot) handleFreeTextExpense(ctx context.Context, tgBot *bot.Bot, update *models.Update) bool {
	if update.Message == nil || update.Message.Text == "" {
		return false
	}

	text := update.Message.Text
	if strings.HasPrefix(text, "/") {
		return false
	}

	categories, err := b.categoryRepo.GetAll(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch categories for free-text parsing")
		return false
	}

	categoryNames := make([]string, len(categories))
	for i, cat := range categories {
		categoryNames[i] = cat.Name
	}

	parsed := ParseExpenseInputWithCategories(text, categoryNames)
	if parsed == nil {
		return false
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	b.saveExpense(ctx, tgBot, chatID, userID, parsed, categories)
	return true
}

// saveExpense creates and saves an expense to the database.
func (b *Bot) saveExpense(
	ctx context.Context,
	tgBot *bot.Bot,
	chatID int64,
	userID int64,
	parsed *ParsedExpense,
	categories []appmodels.Category,
) {
	expense := &appmodels.Expense{
		UserID:      userID,
		Amount:      parsed.Amount,
		Currency:    "SGD",
		Description: parsed.Description,
	}

	if parsed.CategoryName != "" {
		for _, cat := range categories {
			if strings.EqualFold(cat.Name, parsed.CategoryName) {
				expense.CategoryID = &cat.ID
				expense.Category = &cat
				break
			}
		}
	}

	if err := b.expenseRepo.Create(ctx, expense); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to create expense")
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to save expense. Please try again.",
		})
		return
	}

	logger.Log.Debug().
		Int64("chat_id", chatID).
		Int64("user_id", userID).
		Str("amount", expense.Amount.String()).
		Str("description", expense.Description).
		Msg("Expense created")

	categoryText := "Uncategorized"
	if expense.Category != nil {
		categoryText = expense.Category.Name
	}

	descText := ""
	if expense.Description != "" {
		descText = fmt.Sprintf("\n📝 %s", expense.Description)
	}

	text := fmt.Sprintf(`✅ <b>Expense Added</b>

💰 $%s SGD%s
📁 %s
🆔 #%d`,
		expense.Amount.StringFixed(2),
		descText,
		categoryText,
		expense.ID)

	_, err := tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send expense confirmation")
	}
}

// handleList handles the /list command to show recent expenses.
func (b *Bot) handleList(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	expenses, err := b.expenseRepo.GetByUserID(ctx, userID, 10)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch expenses")
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to fetch expenses. Please try again.",
		})
		return
	}

	b.sendExpenseList(ctx, tgBot, chatID, expenses, "📋 <b>Recent Expenses</b>")
}

// handleToday handles the /today command to show today's expenses.
func (b *Bot) handleToday(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	expenses, err := b.expenseRepo.GetByUserIDAndDateRange(ctx, userID, startOfDay, endOfDay)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch today's expenses")
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to fetch expenses. Please try again.",
		})
		return
	}

	total, _ := b.expenseRepo.GetTotalByUserIDAndDateRange(ctx, userID, startOfDay, endOfDay)
	header := fmt.Sprintf("📅 <b>Today's Expenses</b> (Total: $%s)", total.StringFixed(2))
	b.sendExpenseList(ctx, tgBot, chatID, expenses, header)
}

// handleWeek handles the /week command to show this week's expenses.
func (b *Bot) handleWeek(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	startOfWeek := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
	endOfWeek := startOfWeek.Add(7 * 24 * time.Hour)

	expenses, err := b.expenseRepo.GetByUserIDAndDateRange(ctx, userID, startOfWeek, endOfWeek)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch week's expenses")
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to fetch expenses. Please try again.",
		})
		return
	}

	total, _ := b.expenseRepo.GetTotalByUserIDAndDateRange(ctx, userID, startOfWeek, endOfWeek)
	header := fmt.Sprintf("📆 <b>This Week's Expenses</b> (Total: $%s)", total.StringFixed(2))
	b.sendExpenseList(ctx, tgBot, chatID, expenses, header)
}

// sendExpenseList formats and sends a list of expenses.
func (b *Bot) sendExpenseList(
	ctx context.Context,
	tgBot *bot.Bot,
	chatID int64,
	expenses []appmodels.Expense,
	header string,
) {
	if len(expenses) == 0 {
		_, err := tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      header + "\n\nNo expenses found.",
			ParseMode: models.ParseModeHTML,
		})
		if err != nil {
			logger.Log.Error().Err(err).Msg("Failed to send empty expense list")
		}
		return
	}

	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n\n")

	for _, exp := range expenses {
		categoryText := ""
		if exp.Category != nil {
			categoryText = fmt.Sprintf(" [%s]", exp.Category.Name)
		}

		descText := ""
		if exp.Description != "" {
			descText = " - " + exp.Description
		}

		sb.WriteString(fmt.Sprintf(
			"#%d $%s%s%s\n<i>%s</i>\n\n",
			exp.ID,
			exp.Amount.StringFixed(2),
			descText,
			categoryText,
			exp.CreatedAt.Format("Jan 2 15:04"),
		))
	}

	logger.Log.Debug().Int64("chat_id", chatID).Int("count", len(expenses)).Msg("Sending expense list")
	_, err := tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      sb.String(),
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send expense list")
	}
}

// handleEdit handles the /edit command to modify an expense.
func (b *Bot) handleEdit(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	args := strings.TrimPrefix(update.Message.Text, "/edit")
	args = strings.TrimSpace(args)

	if idx := strings.Index(args, "@"); idx == 0 {
		if spaceIdx := strings.Index(args, " "); spaceIdx != -1 {
			args = strings.TrimSpace(args[spaceIdx:])
		} else {
			args = ""
		}
	}

	if args == "" {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Usage: <code>/edit &lt;id&gt; &lt;amount&gt; &lt;description&gt; [category]</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	parts := strings.SplitN(args, " ", 2)
	expenseID, err := strconv.Atoi(parts[0])
	if err != nil {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Invalid expense ID. Use: <code>/edit &lt;id&gt; &lt;amount&gt; &lt;description&gt;</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	expense, err := b.expenseRepo.GetByID(ctx, expenseID)
	if err != nil {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Expense #%d not found.", expenseID),
		})
		return
	}

	if expense.UserID != userID {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ You can only edit your own expenses.",
		})
		return
	}

	if len(parts) < 2 {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Please provide new values: <code>/edit &lt;id&gt; &lt;amount&gt; &lt;description&gt;</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	categories, _ := b.categoryRepo.GetAll(ctx)
	categoryNames := make([]string, len(categories))
	for i, cat := range categories {
		categoryNames[i] = cat.Name
	}

	parsed := ParseExpenseInputWithCategories(parts[1], categoryNames)
	if parsed == nil {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Invalid format. Use: <code>/edit &lt;id&gt; &lt;amount&gt; &lt;description&gt;</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	expense.Amount = parsed.Amount
	expense.Description = parsed.Description

	if parsed.CategoryName != "" {
		for _, cat := range categories {
			if strings.EqualFold(cat.Name, parsed.CategoryName) {
				expense.CategoryID = &cat.ID
				expense.Category = &cat
				break
			}
		}
	}

	if err := b.expenseRepo.Update(ctx, expense); err != nil {
		logger.Log.Error().Err(err).Int("expense_id", expenseID).Msg("Failed to update expense")
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to update expense. Please try again.",
		})
		return
	}

	logger.Log.Debug().
		Int64("chat_id", chatID).
		Int("expense_id", expenseID).
		Msg("Expense updated")

	categoryText := "Uncategorized"
	if expense.Category != nil {
		categoryText = expense.Category.Name
	}

	text := fmt.Sprintf(`✅ <b>Expense Updated</b>

🆔 #%d
💰 $%s SGD
📝 %s
📁 %s`,
		expense.ID,
		expense.Amount.StringFixed(2),
		expense.Description,
		categoryText)

	_, err = tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send edit confirmation")
	}
}

// handleDelete handles the /delete command to remove an expense.
func (b *Bot) handleDelete(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	args := strings.TrimPrefix(update.Message.Text, "/delete")
	args = strings.TrimSpace(args)

	if idx := strings.Index(args, "@"); idx == 0 {
		if spaceIdx := strings.Index(args, " "); spaceIdx != -1 {
			args = strings.TrimSpace(args[spaceIdx:])
		} else {
			args = ""
		}
	}

	if args == "" {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Usage: <code>/delete &lt;id&gt;</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	expenseID, err := strconv.Atoi(args)
	if err != nil {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Invalid expense ID. Use: <code>/delete &lt;id&gt;</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	expense, err := b.expenseRepo.GetByID(ctx, expenseID)
	if err != nil {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Expense #%d not found.", expenseID),
		})
		return
	}

	if expense.UserID != userID {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ You can only delete your own expenses.",
		})
		return
	}

	if err := b.expenseRepo.Delete(ctx, expenseID); err != nil {
		logger.Log.Error().Err(err).Int("expense_id", expenseID).Msg("Failed to delete expense")
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to delete expense. Please try again.",
		})
		return
	}

	logger.Log.Debug().
		Int64("chat_id", chatID).
		Int("expense_id", expenseID).
		Msg("Expense deleted")

	text := fmt.Sprintf("✅ Expense #%d deleted.", expenseID)
	_, err = tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send delete confirmation")
	}
}
