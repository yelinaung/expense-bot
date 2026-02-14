package bot

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/logger"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
	"gitlab.com/yelinaung/expense-bot/internal/repository"
)

// extractCommandArgs strips the /command prefix (and optional @botname suffix)
// from a message and returns the remaining trimmed arguments.
func extractCommandArgs(text, command string) string {
	args := strings.TrimSpace(strings.TrimPrefix(text, command))
	if strings.HasPrefix(args, "@") {
		if spaceIdx := strings.Index(args, " "); spaceIdx != -1 {
			args = strings.TrimSpace(args[spaceIdx:])
		} else {
			args = ""
		}
	}
	return args
}

// formatGreeting returns a greeting suffix with the user's name.
func formatGreeting(firstName string) string {
	if firstName == "" {
		return ""
	}
	return ", " + escapeHTML(firstName)
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

I'm your personal expense tracker bot. I help you track your daily expenses.

<b>Quick Start:</b>
• Send an expense like: <code>5.50 Coffee</code>
• Or use structured format: <code>/add 5.50 Coffee Food - Dining Out</code>
• Upload a receipt photo to extract expenses automatically
• Send a voice message describing your expense

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
• Send a receipt photo to extract expenses automatically
• Send a voice message like <code>spent five fifty on coffee</code>

<b>Managing Expenses:</b>
• <code>/edit &lt;id&gt; &lt;amount&gt; &lt;description&gt; [category]</code> - Edit an expense
• <code>/delete &lt;id&gt;</code> - Delete an expense

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
• <code>/renamecategory Old -&gt; New</code> - Rename a category
• <code>/deletecategory &lt;name&gt;</code> - Delete a category

<b>Currency:</b>
• <code>/currency</code> - Show your default currency
• <code>/setcurrency &lt;code&gt;</code> - Set default currency (e.g., USD, EUR, THB)

<b>Tags:</b>
• Add tags inline: <code>5.50 Coffee #work #meeting</code>
• <code>/tag &lt;id&gt; #tag1 [#tag2] ...</code> - Add tags to expense
• <code>/untag &lt;id&gt; #tag</code> - Remove tag from expense
• <code>/tags</code> - List all tags
• <code>/tags #name</code> - Filter expenses by tag

<b>Admin:</b>
• <code>/approve &lt;user_id&gt;</code> or <code>/approve @username</code> - Approve a user
• <code>/revoke &lt;user_id&gt;</code> or <code>/revoke @username</code> - Revoke a user
• <code>/users</code> - List all authorized users

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
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, escapeHTML(cat.Name)))
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

	args := extractCommandArgs(update.Message.Text, "/addcategory")

	if args == "" {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Please provide a category name.\n\nUsage: <code>/addcategory Food - Dining Out</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Reject category names containing control characters.
	for _, r := range args {
		if unicode.IsControl(r) {
			_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "❌ Category name cannot contain control characters (newlines, tabs, etc.).",
			})
			return
		}
	}

	// Reject category names that are too long.
	if len(args) > appmodels.MaxCategoryNameLength {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Category name is too long (max %d characters).", appmodels.MaxCategoryNameLength),
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
		Text:      fmt.Sprintf("✅ Category '<b>%s</b>' created.", escapeHTML(cat.Name)),
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send /addcategory response")
	}
}

// handleRenameCategory handles the /renamecategory command.
func (b *Bot) handleRenameCategory(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleRenameCategoryCore(ctx, tgBot, update)
}

// handleRenameCategoryCore is the testable implementation of handleRenameCategory.
func (b *Bot) handleRenameCategoryCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	args := extractCommandArgs(update.Message.Text, "/renamecategory")

	// Parse "Old Name -> New Name" syntax.
	parts := strings.SplitN(args, "->", 2)
	if len(parts) != 2 {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Please use the format:\n<code>/renamecategory Old Name -&gt; New Name</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	oldName := strings.TrimSpace(parts[0])
	newName := strings.TrimSpace(parts[1])

	if oldName == "" || newName == "" {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Both old and new category names are required.\n\nUsage: <code>/renamecategory Old Name -&gt; New Name</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Validate new name: reject control characters.
	for _, r := range newName {
		if unicode.IsControl(r) {
			_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "❌ Category name cannot contain control characters (newlines, tabs, etc.).",
			})
			return
		}
	}

	// Validate new name: reject too long.
	if len(newName) > appmodels.MaxCategoryNameLength {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ New category name is too long (max %d characters).", appmodels.MaxCategoryNameLength),
		})
		return
	}

	// Find existing category by old name.
	cat, err := b.categoryRepo.GetByName(ctx, oldName)
	if err != nil {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Category '%s' not found.\n\nUse /categories to see all categories.", oldName),
		})
		return
	}

	// Check if new name already exists.
	existing, err := b.categoryRepo.GetByName(ctx, newName)
	if err == nil && existing.ID != cat.ID {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Category '%s' already exists.", existing.Name),
		})
		return
	}

	if err := b.categoryRepo.Update(ctx, cat.ID, newName); err != nil {
		logger.Log.Error().Err(err).Str("old_name", oldName).Str("new_name", newName).Msg("Failed to rename category")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to rename category. Please try again.",
		})
		return
	}

	b.invalidateCategoryCache()

	logger.Log.Info().Int("category_id", cat.ID).Str("old_name", oldName).Str("new_name", newName).Msg("Category renamed")

	_, err = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      fmt.Sprintf("✅ Category '<b>%s</b>' renamed to '<b>%s</b>'.", escapeHTML(oldName), escapeHTML(newName)),
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send /renamecategory response")
	}
}

// deleteCategoryWithExpenses nullifies expenses and deletes the category atomically.
// When the underlying db supports transactions it wraps both operations in a tx;
// otherwise it falls back to sequential calls (e.g. inside test transactions).
// Returns the number of expenses that were uncategorized.
func (b *Bot) deleteCategoryWithExpenses(ctx context.Context, categoryID int) (int64, error) {
	beginner, ok := b.db.(database.TxBeginner)
	if !ok {
		return b.deleteCategorySequential(ctx, categoryID)
	}

	tx, err := beginner.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txExpRepo := repository.NewExpenseRepository(tx)
	txCatRepo := repository.NewCategoryRepository(tx)

	affected, err := txExpRepo.NullifyCategoryOnExpenses(ctx, categoryID)
	if err != nil {
		return 0, fmt.Errorf("nullify expenses: %w", err)
	}
	if err := txCatRepo.Delete(ctx, categoryID); err != nil {
		return 0, fmt.Errorf("delete category: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	return affected, nil
}

// deleteCategorySequential performs nullify+delete without a transaction.
func (b *Bot) deleteCategorySequential(ctx context.Context, categoryID int) (int64, error) {
	affected, err := b.expenseRepo.NullifyCategoryOnExpenses(ctx, categoryID)
	if err != nil {
		return 0, fmt.Errorf("nullify expenses: %w", err)
	}
	if err := b.categoryRepo.Delete(ctx, categoryID); err != nil {
		return 0, fmt.Errorf("delete category: %w", err)
	}
	return affected, nil
}

// handleDeleteCategory handles the /deletecategory command.
func (b *Bot) handleDeleteCategory(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleDeleteCategoryCore(ctx, tgBot, update)
}

// handleDeleteCategoryCore is the testable implementation of handleDeleteCategory.
func (b *Bot) handleDeleteCategoryCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	args := extractCommandArgs(update.Message.Text, "/deletecategory")

	if args == "" {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Please provide a category name.\n\nUsage: <code>/deletecategory Food - Dining Out</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Find the category.
	cat, err := b.categoryRepo.GetByName(ctx, args)
	if err != nil {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Category '%s' not found.\n\nUse /categories to see all categories.", args),
		})
		return
	}

	// Nullify category on affected expenses and delete inside a transaction
	// so both succeed or both roll back.
	affected, err := b.deleteCategoryWithExpenses(ctx, cat.ID)
	if err != nil {
		logger.Log.Error().Err(err).Int("category_id", cat.ID).Msg("Failed to delete category")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to delete category. Please try again.",
		})
		return
	}

	b.invalidateCategoryCache()

	logger.Log.Info().Int("category_id", cat.ID).Str("name", cat.Name).Int64("affected_expenses", affected).Msg("Category deleted")

	text := fmt.Sprintf("✅ Category '<b>%s</b>' deleted.", escapeHTML(cat.Name))
	if affected > 0 {
		text += fmt.Sprintf("\n\n%d expense(s) have been uncategorized.", affected)
	}

	_, err = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send /deletecategory response")
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

	// Save tags if any were parsed inline.
	if len(parsed.Tags) > 0 {
		var tagIDs []int
		for _, name := range parsed.Tags {
			tag, err := b.tagRepo.GetOrCreate(ctx, name)
			if err != nil {
				logger.Log.Warn().Err(err).Str("tag", name).Msg("Failed to create tag")
				continue
			}
			tagIDs = append(tagIDs, tag.ID)
		}
		if len(tagIDs) > 0 {
			if err := b.tagRepo.SetExpenseTags(ctx, expense.ID, tagIDs); err != nil {
				logger.Log.Warn().Err(err).Int("expense_id", expense.ID).Msg("Failed to set expense tags")
			}
		}
	}

	logger.Log.Debug().
		Int64("chat_id", chatID).
		Int64("user_id", userID).
		Str("amount", expense.Amount.String()).
		Str("description", expense.Description).
		Msg("Expense created")

	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = escapeHTML(expense.Category.Name)
	}

	descText := ""
	if expense.Description != "" {
		descText = "\n📝 " + escapeHTML(expense.Description)
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

	if len(parsed.Tags) > 0 {
		escapedTags := make([]string, len(parsed.Tags))
		for i, t := range parsed.Tags {
			escapedTags[i] = escapeHTML(t)
		}
		text += "\n🏷️ " + strings.Join(escapedTags, ", ")
	}

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

	total, err := b.expenseRepo.GetTotalByUserIDAndDateRange(ctx, userID, startOfDay, endOfDay)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to calculate today's total")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to fetch expenses. Please try again.",
		})
		return
	}
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

	total, err := b.expenseRepo.GetTotalByUserIDAndDateRange(ctx, userID, startOfWeek, endOfWeek)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to calculate week's total")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to fetch expenses. Please try again.",
		})
		return
	}
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
			Text:      fmt.Sprintf("❌ Category '%s' not found.\n\nUse /categories to see all available categories.", escapeHTML(args)),
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

	total, err := b.expenseRepo.GetTotalByUserIDAndCategory(ctx, userID, matchedCategory.ID)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to calculate category total")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to fetch expenses. Please try again.",
		})
		return
	}
	header := fmt.Sprintf("📁 <b>%s Expenses</b> (Total: $%s)", escapeHTML(matchedCategory.Name), total.StringFixed(2))
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

	// Batch-load tags for all expenses.
	expenseIDs := make([]int, len(expenses))
	for i := range expenses {
		expenseIDs[i] = expenses[i].ID
	}
	tagsByExpense, err := b.tagRepo.GetByExpenseIDs(ctx, expenseIDs)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to batch-load tags for expense list")
	}

	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n\n")

	for i := range expenses {
		exp := &expenses[i]
		categoryText := ""
		if exp.Category != nil {
			categoryText = fmt.Sprintf(" [%s]", escapeHTML(exp.Category.Name))
		}

		tagText := ""
		if tags, ok := tagsByExpense[exp.ID]; ok && len(tags) > 0 {
			names := make([]string, len(tags))
			for j, t := range tags {
				names[j] = "#" + escapeHTML(t.Name)
			}
			tagText = " " + strings.Join(names, " ")
		}

		descText := ""
		if exp.Merchant != "" {
			descText = " - " + escapeHTML(exp.Merchant)
		} else if exp.Description != "" {
			descText = " - " + escapeHTML(exp.Description)
		}

		currencySymbol := appmodels.SupportedCurrencies[exp.Currency]
		if currencySymbol == "" {
			currencySymbol = exp.Currency
		}

		sb.WriteString(fmt.Sprintf(
			"#%d %s%s %s%s%s%s\n<i>%s</i>\n\n",
			exp.UserExpenseNumber,
			currencySymbol,
			exp.Amount.StringFixed(2),
			exp.Currency,
			descText,
			categoryText,
			tagText,
			exp.CreatedAt.In(b.displayLocation).Format("Jan 2 15:04"),
		))
	}

	logger.Log.Debug().Int64("chat_id", chatID).Int("count", len(expenses)).Msg("Sending expense list")
	_, err = tg.SendMessage(ctx, &bot.SendMessageParams{
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

	total, err := b.expenseRepo.GetTotalByUserIDAndDateRange(ctx, userID, startDate, endDate)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to calculate report total")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to generate report. Please try again.",
		})
		return
	}

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

	args := extractCommandArgs(update.Message.Text, "/edit")

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

	categories, err := b.getCategoriesWithCache(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch categories for edit")
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to fetch categories. Please try again.",
		})
		return
	}

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

	expense.Amount = parsed.Amount

	if parsed.Currency != "" {
		expense.Currency = parsed.Currency
	}

	if parsed.Description != "" {
		expense.Description = parsed.Description
		expense.Merchant = parsed.Description
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
		categoryText = escapeHTML(expense.Category.Name)
	}

	currencySymbol := appmodels.SupportedCurrencies[expense.Currency]
	if currencySymbol == "" {
		currencySymbol = expense.Currency
	}

	text := fmt.Sprintf(`✅ <b>Expense Updated</b>

🆔 #%d
💰 %s%s %s
📝 %s
📁 %s`,
		expense.UserExpenseNumber,
		currencySymbol,
		expense.Amount.StringFixed(2),
		expense.Currency,
		escapeHTML(expense.Description),
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

	args := extractCommandArgs(update.Message.Text, "/delete")

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
