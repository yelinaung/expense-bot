package bot

import (
	"bytes"
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

// formatGreeting returns a greeting suffix with the user's name.
func formatGreeting(firstName string) string {
	if firstName == "" {
		return ""
	}
	return ", " + firstName
}

// handleStart handles the /start command.
func (b *Bot) handleStart(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleStartCore(ctx, tgBot, update)
}

// handleStartCore is the testable implementation of handleStart.
func (b *Bot) handleStartCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
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
	_, err := tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send /start response")
	}
}

// handleHelp handles the /help command.
func (b *Bot) handleHelp(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleHelpCore(ctx, tgBot, update)
}

// handleHelpCore is the testable implementation of handleHelp.
func (b *Bot) handleHelpCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	text := `📚 <b>Available Commands</b>

<b>Expense Tracking:</b>
• <code>/add &lt;amount&gt; &lt;description&gt; [category]</code> - Add an expense
• Just send a message like <code>5.50 Coffee</code> to quickly add
• Use currency: <code>$10 Lunch</code>, <code>€5 Coffee</code>, <code>50 THB Taxi</code>

<b>Viewing Expenses:</b>
• <code>/list</code> - Show recent expenses
• <code>/today</code> - Show today's expenses
• <code>/week</code> - Show this week's expenses
• <code>/category &lt;name&gt;</code> - Filter expenses by category

<b>Reports:</b>
• <code>/report week</code> - Generate weekly CSV report
• <code>/report month</code> - Generate monthly CSV report
• <code>/chart week</code> - Generate weekly expense chart
• <code>/chart month</code> - Generate monthly expense chart

<b>Categories:</b>
• <code>/categories</code> - List all categories
• <code>/addcategory &lt;name&gt;</code> - Create a new category

<b>Currency:</b>
• <code>/currency</code> - Show your default currency
• <code>/setcurrency USD</code> - Set default currency

<b>Other:</b>
• <code>/help</code> - Show this help message`

	logger.Log.Debug().Int64("chat_id", update.Message.Chat.ID).Msg("Sending /help response")
	_, err := tg.SendMessage(ctx, &bot.SendMessageParams{
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
	b.handleCategoriesCore(ctx, tgBot, update)
}

// handleCategoriesCore is the testable implementation of handleCategories.
func (b *Bot) handleCategoriesCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	categories, err := b.getCategoriesWithCache(ctx)
	if err != nil {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Failed to fetch categories. Please try again.",
		})
		return
	}

	if len(categories) == 0 {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
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
	_, err = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      sb.String(),
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send /categories response")
	}
}

// handleAddCategory handles the /addcategory command to create a new category.
func (b *Bot) handleAddCategory(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleAddCategoryCore(ctx, tgBot, update)
}

// handleAddCategoryCore is the testable implementation of handleAddCategory.
func (b *Bot) handleAddCategoryCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	args := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/addcategory"))
	if idx := strings.Index(args, "@"); idx == 0 {
		if spaceIdx := strings.Index(args, " "); spaceIdx != -1 {
			args = strings.TrimSpace(args[spaceIdx:])
		} else {
			args = ""
		}
	}

	if args == "" {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Please provide a category name.\n\nUsage: <code>/addcategory Food - Dining Out</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	cat, err := b.categoryRepo.Create(ctx, args)
	if err != nil {
		logger.Log.Error().Err(err).Str("name", args).Msg("Failed to create category")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Failed to create category '%s'. It may already exist.", args),
		})
		return
	}

	b.invalidateCategoryCache()

	logger.Log.Info().Int("category_id", cat.ID).Str("name", cat.Name).Msg("Category created")

	_, err = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      fmt.Sprintf("✅ Category '<b>%s</b>' created.", cat.Name),
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send /addcategory response")
	}
}

// handleAdd handles the /add command for structured expense input.
func (b *Bot) handleAdd(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleAddCore(ctx, tgBot, update)
}

// handleAddCore is the testable implementation of handleAdd.
func (b *Bot) handleAddCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	categories, err := b.getCategoriesWithCache(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch categories for parsing")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
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
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Invalid format. Use: <code>/add 5.50 Coffee [category]</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	b.saveExpenseCore(ctx, tg, chatID, userID, parsed, categories)
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

	categories, err := b.getCategoriesWithCache(ctx)
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
	b.saveExpenseCore(ctx, tgBot, chatID, userID, parsed, categories)
}

// saveExpenseCore is the testable implementation of saveExpense.
func (b *Bot) saveExpenseCore(
	ctx context.Context,
	tg TelegramAPI,
	chatID int64,
	userID int64,
	parsed *ParsedExpense,
	categories []appmodels.Category,
) {
	// Determine currency: use parsed currency, fall back to user default
	currency := parsed.Currency
	if currency == "" {
		var err error
		currency, err = b.userRepo.GetDefaultCurrency(ctx, userID)
		if err != nil {
			logger.Log.Debug().Err(err).Str("user_hash", logger.HashUserID(userID)).Msg("Failed to get default currency, using SGD")
			currency = appmodels.DefaultCurrency
		}
	}

	expense := &appmodels.Expense{
		UserID:      userID,
		Amount:      parsed.Amount,
		Currency:    currency,
		Description: parsed.Description,
		Merchant:    parsed.Description,
	}

	// Try to match category from parsed input first
	categoryMatched := false
	if parsed.CategoryName != "" {
		for _, cat := range categories {
			if strings.EqualFold(cat.Name, parsed.CategoryName) {
				expense.CategoryID = &cat.ID
				expense.Category = &cat
				categoryMatched = true
				break
			}
		}
	}

	// If no category matched and Gemini is available, use AI to suggest category
	if !categoryMatched && b.geminiClient != nil && parsed.Description != "" {
		categoryNames := make([]string, len(categories))
		for i, cat := range categories {
			categoryNames[i] = cat.Name
		}

		suggestion, err := b.geminiClient.SuggestCategory(ctx, parsed.Description, categoryNames)
		if err != nil {
			logger.Log.Debug().Err(err).
				Str("description", logger.SanitizeDescription(parsed.Description)).
				Msg("Failed to get AI category suggestion")
		} else if suggestion != nil && suggestion.Confidence > 0.5 {
			// Use AI suggestion if confidence is above 50%
			for _, cat := range categories {
				if strings.EqualFold(cat.Name, suggestion.Category) {
					expense.CategoryID = &cat.ID
					expense.Category = &cat
					logger.Log.Info().
						Str("description", logger.SanitizeDescription(parsed.Description)).
						Str("suggested_category", suggestion.Category).
						Float64("confidence", suggestion.Confidence).
						Str("reasoning", suggestion.Reasoning).
						Msg("AI category suggestion applied")
					break
				}
			}
		}
	}

	if err := b.expenseRepo.Create(ctx, expense); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to create expense")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
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

	currencySymbol := appmodels.SupportedCurrencies[expense.Currency]
	if currencySymbol == "" {
		currencySymbol = expense.Currency
	}

	text := fmt.Sprintf(`✅ <b>Expense Added</b>

💰 %s%s %s%s
📁 %s
🆔 #%d`,
		currencySymbol,
		expense.Amount.StringFixed(2),
		expense.Currency,
		descText,
		categoryText,
		expense.UserExpenseNumber)

	// Add inline edit/delete buttons
	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✏️ Edit", CallbackData: fmt.Sprintf("edit_expense_%d", expense.ID)},
				{Text: "🗑️ Delete", CallbackData: fmt.Sprintf("delete_expense_%d", expense.ID)},
			},
		},
	}

	_, err := tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send expense confirmation")
	}
}

// handleList handles the /list command to show recent expenses.
func (b *Bot) handleList(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleListCore(ctx, tgBot, update)
}

// handleListCore is the testable implementation of handleList.
func (b *Bot) handleListCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	expenses, err := b.expenseRepo.GetByUserID(ctx, userID, 10)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch expenses")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to fetch expenses. Please try again.",
		})
		return
	}

	b.sendExpenseListCore(ctx, tg, chatID, expenses, "📋 <b>Recent Expenses</b>")
}

// handleToday handles the /today command to show today's expenses.
func (b *Bot) handleToday(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleTodayCore(ctx, tgBot, update)
}

// handleTodayCore is the testable implementation of handleToday.
func (b *Bot) handleTodayCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
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
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to fetch expenses. Please try again.",
		})
		return
	}

	total, _ := b.expenseRepo.GetTotalByUserIDAndDateRange(ctx, userID, startOfDay, endOfDay)
	header := fmt.Sprintf("📅 <b>Today's Expenses</b> (Total: $%s)", total.StringFixed(2))
	b.sendExpenseListCore(ctx, tg, chatID, expenses, header)
}

// handleWeek handles the /week command to show this week's expenses.
func (b *Bot) handleWeek(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleWeekCore(ctx, tgBot, update)
}

// handleWeekCore is the testable implementation of handleWeek.
func (b *Bot) handleWeekCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
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
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to fetch expenses. Please try again.",
		})
		return
	}

	total, _ := b.expenseRepo.GetTotalByUserIDAndDateRange(ctx, userID, startOfWeek, endOfWeek)
	header := fmt.Sprintf("📆 <b>This Week's Expenses</b> (Total: $%s)", total.StringFixed(2))
	b.sendExpenseListCore(ctx, tg, chatID, expenses, header)
}

// handleCategory handles the /category command to filter expenses by category.
func (b *Bot) handleCategory(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleCategoryCore(ctx, tgBot, update)
}

// handleCategoryCore is the testable implementation of handleCategory.
func (b *Bot) handleCategoryCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	// Extract category name from command
	args := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/category"))
	if args == "" {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Please provide a category name.\n\nUsage: <code>/category Food - Dining Out</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Find matching category
	categories, err := b.getCategoriesWithCache(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch categories")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to fetch categories. Please try again.",
		})
		return
	}

	var matchedCategory *appmodels.Category
	for i := range categories {
		if strings.EqualFold(categories[i].Name, args) {
			matchedCategory = &categories[i]
			break
		}
	}

	if matchedCategory == nil {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      fmt.Sprintf("❌ Category '%s' not found.\n\nUse /categories to see all available categories.", args),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Fetch expenses for this category
	expenses, err := b.expenseRepo.GetByUserIDAndCategory(ctx, userID, matchedCategory.ID, 20)
	if err != nil {
		logger.Log.Error().Err(err).Int("category_id", matchedCategory.ID).Msg("Failed to fetch expenses by category")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to fetch expenses. Please try again.",
		})
		return
	}

	// Get total for this category
	total, _ := b.expenseRepo.GetTotalByUserIDAndCategory(ctx, userID, matchedCategory.ID)
	header := fmt.Sprintf("📁 <b>%s Expenses</b> (Total: $%s)", matchedCategory.Name, total.StringFixed(2))
	b.sendExpenseListCore(ctx, tg, chatID, expenses, header)

	logger.Log.Info().
		Int64("user_id", userID).
		Int("category_id", matchedCategory.ID).
		Str("category_name", matchedCategory.Name).
		Int("count", len(expenses)).
		Msg("Category filter applied")
}

// sendExpenseListCore formats and sends a list of expenses.
func (b *Bot) sendExpenseListCore(
	ctx context.Context,
	tg TelegramAPI,
	chatID int64,
	expenses []appmodels.Expense,
	header string,
) {
	if len(expenses) == 0 {
		_, err := tg.SendMessage(ctx, &bot.SendMessageParams{
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
		if exp.Merchant != "" {
			descText = " - " + exp.Merchant
		} else if exp.Description != "" {
			descText = " - " + exp.Description
		}

		currencySymbol := appmodels.SupportedCurrencies[exp.Currency]
		if currencySymbol == "" {
			currencySymbol = exp.Currency
		}

		sb.WriteString(fmt.Sprintf(
			"#%d %s%s %s%s%s\n<i>%s</i>\n\n",
			exp.UserExpenseNumber,
			currencySymbol,
			exp.Amount.StringFixed(2),
			exp.Currency,
			descText,
			categoryText,
			exp.CreatedAt.Format("Jan 2 15:04"),
		))
	}

	logger.Log.Debug().Int64("chat_id", chatID).Int("count", len(expenses)).Msg("Sending expense list")
	_, err := tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      sb.String(),
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send expense list")
	}
}

// handleReport handles the /report command to generate CSV reports.
func (b *Bot) handleReport(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleReportCore(ctx, tgBot, update)
}

// handleReportCore is the testable implementation of handleReport.
func (b *Bot) handleReportCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	args := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/report"))
	if args == "" {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Please specify report type.\n\nUsage: <code>/report week</code> or <code>/report month</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	var startDate, endDate time.Time
	var period, title string

	switch strings.ToLower(args) {
	case periodWeek:
		startDate, endDate = getWeekDateRange()
		period = periodWeek
		title = fmt.Sprintf("Weekly Expenses (%s to %s)",
			startDate.Format("Jan 2"), endDate.Add(-24*time.Hour).Format("Jan 2, 2006"))
	case periodMonth:
		startDate, endDate = getMonthDateRange()
		period = periodMonth
		title = fmt.Sprintf("Monthly Expenses (%s)", startDate.Format("January 2006"))
	default:
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Invalid report type. Use <code>week</code> or <code>month</code>.",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	logger.Log.Info().
		Int64("user_id", userID).
		Str("period", period).
		Time("start", startDate).
		Time("end", endDate).
		Msg("Generating expense report")

	// Fetch expenses
	expenses, err := b.expenseRepo.GetByUserIDAndDateRange(ctx, userID, startDate, endDate)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch expenses for report")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to generate report. Please try again.",
		})
		return
	}

	if len(expenses) == 0 {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      fmt.Sprintf("📊 No expenses found for %s.", period),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Generate CSV
	csvData, err := GenerateExpensesCSV(expenses)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to generate CSV")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to generate CSV report. Please try again.",
		})
		return
	}

	// Calculate total
	total, _ := b.expenseRepo.GetTotalByUserIDAndDateRange(ctx, userID, startDate, endDate)

	// Send CSV file
	filename := generateReportFilename(period)
	caption := fmt.Sprintf("📊 <b>%s</b>\n\nTotal Expenses: $%s SGD\nCount: %d",
		title, total.StringFixed(2), len(expenses))

	_, err = tg.SendDocument(ctx, &bot.SendDocumentParams{
		ChatID:    chatID,
		Document:  &models.InputFileUpload{Filename: filename, Data: bytes.NewReader(csvData)},
		Caption:   caption,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send CSV document")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to send report. Please try again.",
		})
		return
	}

	logger.Log.Info().
		Int64("user_id", userID).
		Str("period", period).
		Int("expense_count", len(expenses)).
		Str("total", total.String()).
		Msg("Report generated successfully")
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
	expenseNum, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Invalid expense ID. Use: <code>/edit &lt;id&gt; &lt;amount&gt; &lt;description&gt;</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	expense, err := b.expenseRepo.GetByUserAndNumber(ctx, userID, expenseNum)
	if err != nil {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Expense #%d not found.", expenseNum),
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

	categories, _ := b.getCategoriesWithCache(ctx)

	// Load the existing category if one is set
	if expense.CategoryID != nil {
		for i := range categories {
			if categories[i].ID == *expense.CategoryID {
				expense.Category = &categories[i]
				break
			}
		}
	}
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

	// Update amount (always required)
	expense.Amount = parsed.Amount

	// Only update description and merchant if provided
	if parsed.Description != "" {
		expense.Description = parsed.Description
		expense.Merchant = parsed.Description
	}

	// Only update category if provided
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
		logger.Log.Error().Err(err).Int64("expense_num", expenseNum).Msg("Failed to update expense")
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to update expense. Please try again.",
		})
		return
	}

	logger.Log.Debug().
		Int64("chat_id", chatID).
		Int64("expense_num", expenseNum).
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
		expense.UserExpenseNumber,
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

	expenseNum, err := strconv.ParseInt(args, 10, 64)
	if err != nil {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Invalid expense ID. Use: <code>/delete &lt;id&gt;</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	expense, err := b.expenseRepo.GetByUserAndNumber(ctx, userID, expenseNum)
	if err != nil {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Expense #%d not found.", expenseNum),
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

	if err := b.expenseRepo.Delete(ctx, expense.ID); err != nil {
		logger.Log.Error().Err(err).Int64("expense_num", expenseNum).Msg("Failed to delete expense")
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to delete expense. Please try again.",
		})
		return
	}

	logger.Log.Debug().
		Int64("chat_id", chatID).
		Int64("expense_num", expenseNum).
		Msg("Expense deleted")

	text := fmt.Sprintf("✅ Expense #%d deleted.", expenseNum)
	_, err = tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send delete confirmation")
	}
}
