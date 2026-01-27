package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"

	"gitlab.com/yelinaung/expense-bot/internal/logger"
)

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
		b.promptEditAmount(ctx, tgBot, chatID, messageID, expense)

	case "category":
		b.showCategorySelection(ctx, tgBot, chatID, messageID, expense)
	}
}

// promptEditAmount prompts the user to enter a new amount.
func (b *Bot) promptEditAmount(
	ctx context.Context,
	tgBot *bot.Bot,
	chatID int64,
	messageID int,
	expense *appmodels.Expense,
) {
	// Store pending edit state.
	b.pendingEditsMu.Lock()
	b.pendingEdits[chatID] = &pendingEdit{
		ExpenseID: expense.ID,
		EditType:  "amount",
		MessageID: messageID,
	}
	b.pendingEditsMu.Unlock()

	text := fmt.Sprintf(`💰 <b>Edit Amount</b>

Current amount: $%s SGD

Please type the new amount (e.g., <code>25.50</code>):`,
		expense.Amount.StringFixed(2))

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "⬅️ Cancel", CallbackData: fmt.Sprintf("cancel_edit_%d", expense.ID)},
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

// handlePendingEdit checks for and processes pending edit operations.
func (b *Bot) handlePendingEdit(ctx context.Context, tgBot *bot.Bot, update *models.Update) bool {
	if update.Message == nil || update.Message.Text == "" {
		return false
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	b.pendingEditsMu.RLock()
	pending, exists := b.pendingEdits[chatID]
	b.pendingEditsMu.RUnlock()

	if !exists {
		return false
	}

	switch pending.EditType {
	case "amount":
		return b.processAmountEdit(ctx, tgBot, chatID, userID, pending, update.Message.Text)
	case "category":
		return b.processCategoryCreate(ctx, tgBot, chatID, userID, pending, update.Message.Text)
	}

	return false
}

// processAmountEdit processes user input for amount editing.
func (b *Bot) processAmountEdit(
	ctx context.Context,
	tgBot *bot.Bot,
	chatID int64,
	userID int64,
	pending *pendingEdit,
	input string,
) bool {
	// Clear pending edit state.
	b.pendingEditsMu.Lock()
	delete(b.pendingEdits, chatID)
	b.pendingEditsMu.Unlock()

	// Parse the amount.
	input = strings.TrimSpace(input)
	input = strings.TrimPrefix(input, "$")
	input = strings.TrimSpace(input)

	amount, err := parseAmount(input)
	if err != nil {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Invalid amount. Please enter a valid number (e.g., 25.50).",
			ParseMode: models.ParseModeHTML,
		})
		return true
	}

	// Fetch and verify expense ownership.
	expense, err := b.expenseRepo.GetByID(ctx, pending.ExpenseID)
	if err != nil {
		logger.Log.Error().Err(err).Int("expense_id", pending.ExpenseID).Msg("Expense not found for edit")
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Expense not found.",
		})
		return true
	}

	if expense.UserID != userID {
		logger.Log.Warn().Int64("user_id", userID).Int("expense_id", pending.ExpenseID).Msg("User mismatch on edit")
		return true
	}

	// Update the expense amount.
	expense.Amount = amount
	if err := b.expenseRepo.Update(ctx, expense); err != nil {
		logger.Log.Error().Err(err).Int("expense_id", expense.ID).Msg("Failed to update amount")
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to update amount. Please try again.",
		})
		return true
	}

	logger.Log.Info().
		Int("expense_id", expense.ID).
		Str("new_amount", amount.String()).
		Msg("Amount updated via pending edit")

	// Show updated confirmation message.
	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = expense.Category.Name
	} else if expense.CategoryID != nil {
		cat, err := b.categoryRepo.GetByID(ctx, *expense.CategoryID)
		if err == nil {
			categoryText = cat.Name
		}
	}

	keyboard := buildReceiptConfirmationKeyboard(expense.ID)

	text := fmt.Sprintf(`📸 <b>Amount Updated!</b>

💰 Amount: $%s SGD
🏪 Merchant: %s
📁 Category: %s

Amount updated. Confirm to save.`,
		expense.Amount.StringFixed(2),
		expense.Description,
		categoryText)

	// Edit the original message.
	_, _ = tgBot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   pending.MessageID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})

	return true
}

// handleCancelEditCallback handles cancel edit button presses.
func (b *Bot) handleCancelEditCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
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

	// Clear any pending edit state.
	b.pendingEditsMu.Lock()
	delete(b.pendingEdits, chatID)
	b.pendingEditsMu.Unlock()

	parts := strings.Split(data, "_")
	if len(parts) < 3 {
		return
	}

	expenseID, err := strconv.Atoi(parts[2])
	if err != nil {
		return
	}

	expense, err := b.expenseRepo.GetByID(ctx, expenseID)
	if err != nil || expense.UserID != userID {
		return
	}

	// Return to edit menu.
	b.handleEditReceipt(ctx, tgBot, chatID, messageID, expense)
}

// showCategorySelection shows category selection buttons.
func (b *Bot) showCategorySelection(
	ctx context.Context,
	tgBot *bot.Bot,
	chatID int64,
	messageID int,
	expense *appmodels.Expense,
) {
	categories, err := b.getCategoriesWithCache(ctx)
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
		{Text: "➕ Create New", CallbackData: fmt.Sprintf("create_category_%d", expense.ID)},
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

	keyboard := buildReceiptConfirmationKeyboard(expense.ID)

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

// processCategoryCreate processes user input for creating a new category.
func (b *Bot) processCategoryCreate(
	ctx context.Context,
	tgBot *bot.Bot,
	chatID int64,
	userID int64,
	pending *pendingEdit,
	input string,
) bool {
	b.pendingEditsMu.Lock()
	delete(b.pendingEdits, chatID)
	b.pendingEditsMu.Unlock()

	categoryName := strings.TrimSpace(input)
	if categoryName == "" {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Category name cannot be empty.",
		})
		return true
	}

	if len(categoryName) > 50 {
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Category name is too long (max 50 characters).",
		})
		return true
	}

	expense, err := b.expenseRepo.GetByID(ctx, pending.ExpenseID)
	if err != nil {
		logger.Log.Error().Err(err).Int("expense_id", pending.ExpenseID).Msg("Expense not found")
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Expense not found.",
		})
		return true
	}

	if expense.UserID != userID {
		logger.Log.Warn().Int64("user_id", userID).Int("expense_id", pending.ExpenseID).Msg("User mismatch")
		return true
	}

	category, err := b.categoryRepo.Create(ctx, categoryName)
	if err != nil {
		logger.Log.Error().Err(err).Str("name", categoryName).Msg("Failed to create category")
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to create category. It may already exist.",
		})
		return true
	}

	// Invalidate category cache after successful creation.
	b.invalidateCategoryCache()

	expense.CategoryID = &category.ID
	expense.Category = category
	if err := b.expenseRepo.Update(ctx, expense); err != nil {
		logger.Log.Error().Err(err).Int("expense_id", expense.ID).Msg("Failed to update expense category")
		_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Category created but failed to assign it. Please select it from the list.",
		})
		return true
	}

	logger.Log.Info().
		Int("expense_id", expense.ID).
		Int("category_id", category.ID).
		Str("category_name", category.Name).
		Msg("New category created and assigned")

	keyboard := buildReceiptConfirmationKeyboard(expense.ID)

	text := fmt.Sprintf(`📸 <b>Category Created!</b>

💰 Amount: $%s SGD
🏪 Merchant: %s
📁 Category: %s

New category created. Confirm to save.`,
		expense.Amount.StringFixed(2),
		expense.Description,
		category.Name)

	_, _ = tgBot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   pending.MessageID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})

	return true
}

// handleCreateCategoryCallback handles the create new category button press.
func (b *Bot) handleCreateCategoryCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
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

	expenseID, err := strconv.Atoi(parts[2])
	if err != nil {
		return
	}

	expense, err := b.expenseRepo.GetByID(ctx, expenseID)
	if err != nil || expense.UserID != userID {
		return
	}

	b.promptCreateCategory(ctx, tgBot, chatID, messageID, expense)
}

// promptCreateCategory prompts the user to enter a new category name.
func (b *Bot) promptCreateCategory(
	ctx context.Context,
	tgBot *bot.Bot,
	chatID int64,
	messageID int,
	expense *appmodels.Expense,
) {
	b.pendingEditsMu.Lock()
	b.pendingEdits[chatID] = &pendingEdit{
		ExpenseID: expense.ID,
		EditType:  "category",
		MessageID: messageID,
	}
	b.pendingEditsMu.Unlock()

	text := `📁 <b>Create New Category</b>

Please type the name for the new category (e.g., <code>Subscriptions</code>):`

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "⬅️ Cancel", CallbackData: fmt.Sprintf("cancel_edit_%d", expense.ID)},
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
