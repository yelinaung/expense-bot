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

const categoryUncategorized = "Uncategorized"

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

	categoryText := categoryUncategorized
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

	categoryText := categoryUncategorized
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

// handlePhoto handles photo messages for receipt OCR.
func (b *Bot) handlePhoto(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil || len(update.Message.Photo) == 0 {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	logger.Log.Info().
		Int64("chat_id", chatID).
		Int64("user_id", userID).
		Int("photo_count", len(update.Message.Photo)).
		Msg("Received photo message")

	if b.geminiClient == nil {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "📷 Receipt OCR is not configured. Please add expenses manually using /add or send text like <code>5.50 Coffee</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	largestPhoto := update.Message.Photo[len(update.Message.Photo)-1]

	logger.Log.Debug().
		Int64("chat_id", chatID).
		Int64("user_id", userID).
		Str("file_id", largestPhoto.FileID).
		Int("width", largestPhoto.Width).
		Int("height", largestPhoto.Height).
		Msg("Downloading photo")

	_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "📷 Processing receipt...",
	})

	imageBytes, err := b.downloadPhoto(ctx, tgBot, largestPhoto.FileID)
	if err != nil {
		logger.Log.Error().Err(err).
			Int64("chat_id", chatID).
			Int64("user_id", userID).
			Msg("Failed to download photo")
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to download photo. Please try again.",
		})
		return
	}

	logger.Log.Info().
		Int64("chat_id", chatID).
		Int64("user_id", userID).
		Int("size_bytes", len(imageBytes)).
		Msg("Photo downloaded successfully")

	receiptData, err := b.geminiClient.ParseReceipt(ctx, imageBytes, "image/jpeg")
	if err != nil {
		logger.Log.Error().Err(err).
			Int64("chat_id", chatID).
			Int64("user_id", userID).
			Msg("Failed to parse receipt")
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to read receipt. Please try again with a clearer image or add manually.",
		})
		return
	}

	logger.Log.Info().
		Int64("chat_id", chatID).
		Int64("user_id", userID).
		Str("amount", receiptData.Amount.String()).
		Str("merchant", receiptData.Merchant).
		Str("category", receiptData.SuggestedCategory).
		Float64("confidence", receiptData.Confidence).
		Msg("Receipt parsed successfully")

	categories, _ := b.categoryRepo.GetAll(ctx)
	var categoryID *int
	var category *appmodels.Category
	for i := range categories {
		if strings.EqualFold(categories[i].Name, receiptData.SuggestedCategory) {
			categoryID = &categories[i].ID
			category = &categories[i]
			break
		}
	}

	expense := &appmodels.Expense{
		UserID:        userID,
		Amount:        receiptData.Amount,
		Currency:      "SGD",
		Description:   receiptData.Merchant,
		CategoryID:    categoryID,
		Category:      category,
		ReceiptFileID: largestPhoto.FileID,
		Status:        appmodels.ExpenseStatusDraft,
	}

	if err := b.expenseRepo.Create(ctx, expense); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to create draft expense")
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to save expense. Please try again.",
		})
		return
	}

	categoryText := categoryUncategorized
	if category != nil {
		categoryText = category.Name
	}

	dateText := "Unknown"
	if !receiptData.Date.IsZero() {
		dateText = receiptData.Date.Format("02 Jan 2006")
	}

	text := fmt.Sprintf(`📸 <b>Receipt Scanned!</b>

💰 Amount: $%s SGD
🏪 Merchant: %s
📅 Date: %s
📁 Category: %s`,
		expense.Amount.StringFixed(2),
		expense.Description,
		dateText,
		categoryText)

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Confirm", CallbackData: fmt.Sprintf("receipt_confirm_%d", expense.ID)},
				{Text: "✏️ Edit", CallbackData: fmt.Sprintf("receipt_edit_%d", expense.ID)},
				{Text: "❌ Cancel", CallbackData: fmt.Sprintf("receipt_cancel_%d", expense.ID)},
			},
		},
	}

	msg, err := tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send receipt confirmation")
		return
	}

	logger.Log.Debug().
		Int64("chat_id", chatID).
		Int("expense_id", expense.ID).
		Int("message_id", msg.ID).
		Msg("Receipt confirmation sent with inline keyboard")
}

// handleReceiptCallback handles receipt confirmation button presses.
func (b *Bot) handleReceiptCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	data := update.CallbackQuery.Data
	userID := update.CallbackQuery.From.ID
	chatID := update.CallbackQuery.Message.Message.Chat.ID
	messageID := update.CallbackQuery.Message.Message.ID

	logger.Log.Debug().
		Str("callback_data", data).
		Int64("user_id", userID).
		Msg("Processing receipt callback")

	_, _ = tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})

	parts := strings.Split(data, "_")
	if len(parts) < 3 {
		logger.Log.Error().Str("data", data).Msg("Invalid callback data format")
		return
	}

	action := parts[1]
	expenseID, err := strconv.Atoi(parts[2])
	if err != nil {
		logger.Log.Error().Err(err).Str("data", data).Msg("Failed to parse expense ID")
		return
	}

	expense, err := b.expenseRepo.GetByID(ctx, expenseID)
	if err != nil {
		logger.Log.Error().Err(err).Int("expense_id", expenseID).Msg("Expense not found")
		_, _ = tgBot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Expense not found.",
		})
		return
	}

	if expense.UserID != userID {
		logger.Log.Warn().Int64("user_id", userID).Int("expense_id", expenseID).Msg("User mismatch")
		return
	}

	switch action {
	case "confirm":
		b.handleConfirmReceipt(ctx, tgBot, chatID, messageID, expense)
	case "cancel":
		b.handleCancelReceipt(ctx, tgBot, chatID, messageID, expense)
	case "edit":
		b.handleEditReceipt(ctx, tgBot, chatID, messageID, expense)
	case "back":
		b.handleBackToReceipt(ctx, tgBot, chatID, messageID, expense)
	}
}

// handleBackToReceipt returns to the main receipt confirmation view.
func (b *Bot) handleBackToReceipt(
	ctx context.Context,
	tgBot *bot.Bot,
	chatID int64,
	messageID int,
	expense *appmodels.Expense,
) {
	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = expense.Category.Name
	} else if expense.CategoryID != nil {
		cat, err := b.categoryRepo.GetByID(ctx, *expense.CategoryID)
		if err == nil {
			categoryText = cat.Name
		}
	}

	text := fmt.Sprintf(`📸 <b>Receipt Scanned!</b>

💰 Amount: $%s SGD
🏪 Merchant: %s
📁 Category: %s`,
		expense.Amount.StringFixed(2),
		expense.Description,
		categoryText)

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Confirm", CallbackData: fmt.Sprintf("receipt_confirm_%d", expense.ID)},
				{Text: "✏️ Edit", CallbackData: fmt.Sprintf("receipt_edit_%d", expense.ID)},
				{Text: "❌ Cancel", CallbackData: fmt.Sprintf("receipt_cancel_%d", expense.ID)},
			},
		},
	}

	_, _ = tgBot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})
}

// handleConfirmReceipt confirms a draft expense.
func (b *Bot) handleConfirmReceipt(
	ctx context.Context,
	tgBot *bot.Bot,
	chatID int64,
	messageID int,
	expense *appmodels.Expense,
) {
	expense.Status = appmodels.ExpenseStatusConfirmed
	if err := b.expenseRepo.Update(ctx, expense); err != nil {
		logger.Log.Error().Err(err).Int("expense_id", expense.ID).Msg("Failed to confirm expense")
		_, _ = tgBot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Failed to confirm expense. Please try again.",
		})
		return
	}

	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = expense.Category.Name
	} else if expense.CategoryID != nil {
		cat, err := b.categoryRepo.GetByID(ctx, *expense.CategoryID)
		if err == nil {
			categoryText = cat.Name
		}
	}

	text := fmt.Sprintf(`✅ <b>Expense Confirmed!</b>

💰 Amount: $%s SGD
🏪 Description: %s
📁 Category: %s

Expense #%d has been saved.`,
		expense.Amount.StringFixed(2),
		expense.Description,
		categoryText,
		expense.ID)

	logger.Log.Info().
		Int("expense_id", expense.ID).
		Str("amount", expense.Amount.String()).
		Msg("Expense confirmed via callback")

	_, _ = tgBot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
}

// handleCancelReceipt cancels and deletes a draft expense.
func (b *Bot) handleCancelReceipt(
	ctx context.Context,
	tgBot *bot.Bot,
	chatID int64,
	messageID int,
	expense *appmodels.Expense,
) {
	if err := b.expenseRepo.Delete(ctx, expense.ID); err != nil {
		logger.Log.Error().Err(err).Int("expense_id", expense.ID).Msg("Failed to delete expense")
		_, _ = tgBot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Failed to cancel expense. Please try again.",
		})
		return
	}

	logger.Log.Info().
		Int("expense_id", expense.ID).
		Msg("Expense cancelled via callback")

	_, _ = tgBot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      "🗑️ Receipt scan cancelled. The expense was not saved.",
	})
}

// handleEditReceipt shows edit options for a draft expense.
func (b *Bot) handleEditReceipt(
	ctx context.Context,
	tgBot *bot.Bot,
	chatID int64,
	messageID int,
	expense *appmodels.Expense,
) {
	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "💰 Edit Amount", CallbackData: fmt.Sprintf("edit_amount_%d", expense.ID)},
				{Text: "📁 Edit Category", CallbackData: fmt.Sprintf("edit_category_%d", expense.ID)},
			},
			{
				{Text: "⬅️ Back", CallbackData: fmt.Sprintf("receipt_back_%d", expense.ID)},
			},
		},
	}

	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = expense.Category.Name
	} else if expense.CategoryID != nil {
		cat, err := b.categoryRepo.GetByID(ctx, *expense.CategoryID)
		if err == nil {
			categoryText = cat.Name
		}
	}

	text := fmt.Sprintf(`✏️ <b>Edit Expense</b>

💰 Amount: $%s SGD
🏪 Description: %s
📁 Category: %s

Select what to edit:`,
		expense.Amount.StringFixed(2),
		expense.Description,
		categoryText)

	_, _ = tgBot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})
}

// handleEditCallback handles edit sub-menu button presses.
func (b *Bot) handleEditCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	data := update.CallbackQuery.Data
	userID := update.CallbackQuery.From.ID
	chatID := update.CallbackQuery.Message.Message.Chat.ID
	messageID := update.CallbackQuery.Message.Message.ID

	_, _ = tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})

	parts := strings.Split(data, "_")
	if len(parts) < 3 {
		return
	}

	action := parts[1]
	expenseID, err := strconv.Atoi(parts[2])
	if err != nil {
		return
	}

	expense, err := b.expenseRepo.GetByID(ctx, expenseID)
	if err != nil || expense.UserID != userID {
		return
	}

	switch action {
	case "amount":
		text := fmt.Sprintf(`💰 <b>Edit Amount</b>

Current amount: $%s SGD

To change the amount, use:
<code>/edit %d amount NEW_AMOUNT</code>

Example: <code>/edit %d amount 15.50</code>`,
			expense.Amount.StringFixed(2),
			expense.ID,
			expense.ID)

		keyboard := &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "⬅️ Back", CallbackData: fmt.Sprintf("receipt_edit_%d", expense.ID)},
				},
			},
		}

		_, _ = tgBot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        text,
			ParseMode:   models.ParseModeHTML,
			ReplyMarkup: keyboard,
		})

	case "category":
		b.showCategorySelection(ctx, tgBot, chatID, messageID, expense)
	}
}

// showCategorySelection shows category selection buttons.
func (b *Bot) showCategorySelection(
	ctx context.Context,
	tgBot *bot.Bot,
	chatID int64,
	messageID int,
	expense *appmodels.Expense,
) {
	categories, err := b.categoryRepo.GetAll(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch categories")
		return
	}

	var rows [][]models.InlineKeyboardButton
	var currentRow []models.InlineKeyboardButton

	for _, cat := range categories {
		btn := models.InlineKeyboardButton{
			Text:         cat.Name,
			CallbackData: fmt.Sprintf("set_category_%d_%d", expense.ID, cat.ID),
		}
		currentRow = append(currentRow, btn)
		if len(currentRow) == 2 {
			rows = append(rows, currentRow)
			currentRow = nil
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, currentRow)
	}

	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "⬅️ Back", CallbackData: fmt.Sprintf("receipt_edit_%d", expense.ID)},
	})

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}

	text := fmt.Sprintf(`📁 <b>Select Category</b>

Current: %s

Choose a new category:`,
		getCategoryName(expense))

	_, _ = tgBot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})
}

// getCategoryName returns the category name for an expense.
func getCategoryName(expense *appmodels.Expense) string {
	if expense.Category != nil {
		return expense.Category.Name
	}
	return categoryUncategorized
}

// handleSetCategoryCallback handles category selection.
func (b *Bot) handleSetCategoryCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	data := update.CallbackQuery.Data
	userID := update.CallbackQuery.From.ID
	chatID := update.CallbackQuery.Message.Message.Chat.ID
	messageID := update.CallbackQuery.Message.Message.ID

	_, _ = tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})

	parts := strings.Split(data, "_")
	if len(parts) < 4 {
		return
	}

	expenseID, err := strconv.Atoi(parts[2])
	if err != nil {
		return
	}

	categoryID, err := strconv.Atoi(parts[3])
	if err != nil {
		return
	}

	expense, err := b.expenseRepo.GetByID(ctx, expenseID)
	if err != nil || expense.UserID != userID {
		return
	}

	category, err := b.categoryRepo.GetByID(ctx, categoryID)
	if err != nil {
		logger.Log.Error().Err(err).Int("category_id", categoryID).Msg("Category not found")
		return
	}

	expense.CategoryID = &categoryID
	expense.Category = category
	if err := b.expenseRepo.Update(ctx, expense); err != nil {
		logger.Log.Error().Err(err).Int("expense_id", expense.ID).Msg("Failed to update category")
		return
	}

	logger.Log.Info().
		Int("expense_id", expense.ID).
		Str("category", category.Name).
		Msg("Category updated via callback")

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Confirm", CallbackData: fmt.Sprintf("receipt_confirm_%d", expense.ID)},
				{Text: "✏️ Edit", CallbackData: fmt.Sprintf("receipt_edit_%d", expense.ID)},
				{Text: "❌ Cancel", CallbackData: fmt.Sprintf("receipt_cancel_%d", expense.ID)},
			},
		},
	}

	text := fmt.Sprintf(`📸 <b>Receipt Updated!</b>

💰 Amount: $%s SGD
🏪 Merchant: %s
📁 Category: %s

Category updated. Confirm to save.`,
		expense.Amount.StringFixed(2),
		expense.Description,
		category.Name)

	_, _ = tgBot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})
}
