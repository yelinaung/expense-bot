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
	b.handleEditCallbackCore(ctx, tgBot, update)
}

// handleEditCallbackCore is the testable implementation of handleEditCallback.
func (b *Bot) handleEditCallbackCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	data := update.CallbackQuery.Data
	userID := update.CallbackQuery.From.ID
	chatID := update.CallbackQuery.Message.Message.Chat.ID
	messageID := update.CallbackQuery.Message.Message.ID

	_, _ = tg.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
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
		b.promptEditAmountCore(ctx, tg, chatID, messageID, expense)

	case "category":
		b.showCategorySelectionCore(ctx, tg, chatID, messageID, expense)
	}
}

// promptEditAmountCore prompts the user to enter a new amount.
func (b *Bot) promptEditAmountCore(
	ctx context.Context,
	tg TelegramAPI,
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

	_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})
}

// handlePendingEdit checks for and processes pending edit operations.
func (b *Bot) handlePendingEdit(ctx context.Context, tgBot *bot.Bot, update *models.Update) bool {
	return b.handlePendingEditCore(ctx, tgBot, update)
}

// handlePendingEditCore is the testable implementation of handlePendingEdit.
func (b *Bot) handlePendingEditCore(ctx context.Context, tg TelegramAPI, update *models.Update) bool {
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
		return b.processAmountEditCore(ctx, tg, chatID, userID, pending, update.Message.Text)
	case "category":
		return b.processCategoryCreateCore(ctx, tg, chatID, userID, pending, update.Message.Text)
	}

	return false
}

// processAmountEditCore processes user input for amount editing.
func (b *Bot) processAmountEditCore(
	ctx context.Context,
	tg TelegramAPI,
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
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
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
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Expense not found.",
		})
		return true
	}

	if expense.UserID != userID {
		logger.Log.Warn().Str("user_hash", logger.HashUserID(userID)).Int("expense_id", pending.ExpenseID).Msg("User mismatch on edit")
		return true
	}

	// Update the expense amount.
	expense.Amount = amount
	if err := b.expenseRepo.Update(ctx, expense); err != nil {
		logger.Log.Error().Err(err).Int("expense_id", expense.ID).Msg("Failed to update amount")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
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
	_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
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
	b.handleCancelEditCallbackCore(ctx, tgBot, update)
}

// handleCancelEditCallbackCore is the testable implementation of handleCancelEditCallback.
func (b *Bot) handleCancelEditCallbackCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	data := update.CallbackQuery.Data
	userID := update.CallbackQuery.From.ID
	chatID := update.CallbackQuery.Message.Message.Chat.ID
	messageID := update.CallbackQuery.Message.Message.ID

	_, _ = tg.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
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
	b.handleEditReceiptCore(ctx, tg, chatID, messageID, expense)
}

// showCategorySelectionCore shows category selection buttons.
func (b *Bot) showCategorySelectionCore(
	ctx context.Context,
	tg TelegramAPI,
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

	_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
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
	b.handleSetCategoryCallbackCore(ctx, tgBot, update)
}

// handleSetCategoryCallbackCore is the testable implementation of handleSetCategoryCallback.
func (b *Bot) handleSetCategoryCallbackCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	data := update.CallbackQuery.Data
	userID := update.CallbackQuery.From.ID
	chatID := update.CallbackQuery.Message.Message.Chat.ID
	messageID := update.CallbackQuery.Message.Message.ID

	_, _ = tg.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
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

	_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})
}

// processCategoryCreateCore processes user input for creating a new category.
func (b *Bot) processCategoryCreateCore(
	ctx context.Context,
	tg TelegramAPI,
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
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Category name cannot be empty.",
		})
		return true
	}

	if len(categoryName) > 50 {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Category name is too long (max 50 characters).",
		})
		return true
	}

	expense, err := b.expenseRepo.GetByID(ctx, pending.ExpenseID)
	if err != nil {
		logger.Log.Error().Err(err).Int("expense_id", pending.ExpenseID).Msg("Expense not found")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Expense not found.",
		})
		return true
	}

	if expense.UserID != userID {
		logger.Log.Warn().Str("user_hash", logger.HashUserID(userID)).Int("expense_id", pending.ExpenseID).Msg("User mismatch")
		return true
	}

	category, err := b.categoryRepo.Create(ctx, categoryName)
	if err != nil {
		logger.Log.Error().Err(err).Str("name", categoryName).Msg("Failed to create category")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
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
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
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

	_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
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
	b.handleCreateCategoryCallbackCore(ctx, tgBot, update)
}

// handleCreateCategoryCallbackCore is the testable implementation of handleCreateCategoryCallback.
func (b *Bot) handleCreateCategoryCallbackCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	data := update.CallbackQuery.Data
	userID := update.CallbackQuery.From.ID
	chatID := update.CallbackQuery.Message.Message.Chat.ID
	messageID := update.CallbackQuery.Message.Message.ID

	_, _ = tg.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
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

	b.promptCreateCategoryCore(ctx, tg, chatID, messageID, expense)
}

// promptCreateCategoryCore prompts the user to enter a new category name.
func (b *Bot) promptCreateCategoryCore(
	ctx context.Context,
	tg TelegramAPI,
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

	_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})
}

// handleExpenseActionCallback handles inline edit/delete buttons on expense confirmations.
func (b *Bot) handleExpenseActionCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleExpenseActionCallbackCore(ctx, tgBot, update)
}

// handleExpenseActionCallbackCore is the testable implementation.
func (b *Bot) handleExpenseActionCallbackCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
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
		Msg("Processing expense action callback")

	_, _ = tg.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})

	parts := strings.Split(data, "_")
	if len(parts) < 3 {
		logger.Log.Error().Str("data", data).Msg("Invalid callback data format")
		return
	}

	action := parts[0] + "_" + parts[1] // "edit_expense" or "delete_expense"
	expenseID, err := strconv.Atoi(parts[2])
	if err != nil {
		logger.Log.Error().Err(err).Str("data", data).Msg("Failed to parse expense ID")
		return
	}

	expense, err := b.expenseRepo.GetByID(ctx, expenseID)
	if err != nil {
		logger.Log.Error().Err(err).Int("expense_id", expenseID).Msg("Expense not found")
		_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Expense not found.",
		})
		return
	}

	if expense.UserID != userID {
		logger.Log.Warn().Str("user_hash", logger.HashUserID(userID)).Int("expense_id", expenseID).Msg("User mismatch")
		_, _ = tg.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "❌ You can only modify your own expenses.",
			ShowAlert:       true,
		})
		return
	}

	switch action {
	case "edit_expense":
		b.handleInlineEditExpenseCore(ctx, tg, chatID, messageID, expense)
	case "delete_expense":
		b.handleInlineDeleteExpenseCore(ctx, tg, chatID, messageID, expense)
	}
}

// handleInlineEditExpenseCore shows the edit options menu.
func (b *Bot) handleInlineEditExpenseCore(
	ctx context.Context,
	tg TelegramAPI,
	chatID int64,
	messageID int,
	expense *appmodels.Expense,
) {
	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = expense.Category.Name
	}

	text := fmt.Sprintf(`✏️ <b>Edit Expense #%d</b>

Current Details:
💰 Amount: $%s SGD
📝 Description: %s
📁 Category: %s

What would you like to edit?`,
		expense.ID,
		expense.Amount.StringFixed(2),
		expense.Description,
		categoryText)

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "💰 Amount", CallbackData: fmt.Sprintf("edit_amount_%d", expense.ID)},
			},
			{
				{Text: "📝 Description", CallbackData: fmt.Sprintf("edit_desc_%d", expense.ID)},
			},
			{
				{Text: "📁 Category", CallbackData: fmt.Sprintf("edit_cat_%d", expense.ID)},
			},
			{
				{Text: "⬅️ Back", CallbackData: fmt.Sprintf("back_to_expense_%d", expense.ID)},
			},
		},
	}

	_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})
}

// handleInlineDeleteExpenseCore shows delete confirmation.
func (b *Bot) handleInlineDeleteExpenseCore(
	ctx context.Context,
	tg TelegramAPI,
	chatID int64,
	messageID int,
	expense *appmodels.Expense,
) {
	text := fmt.Sprintf(`🗑️ <b>Delete Expense?</b>

Are you sure you want to delete this expense?

💰 $%s SGD
📝 %s
🆔 #%d

This action cannot be undone.`,
		expense.Amount.StringFixed(2),
		expense.Description,
		expense.ID)

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Yes, Delete", CallbackData: fmt.Sprintf("confirm_delete_%d", expense.ID)},
			},
			{
				{Text: "❌ No, Keep It", CallbackData: fmt.Sprintf("back_to_expense_%d", expense.ID)},
			},
		},
	}

	_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})
}

// handleConfirmDeleteCallback handles deletion confirmation.
func (b *Bot) handleConfirmDeleteCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleConfirmDeleteCallbackCore(ctx, tgBot, update)
}

// handleConfirmDeleteCallbackCore is the testable implementation.
func (b *Bot) handleConfirmDeleteCallbackCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	data := update.CallbackQuery.Data
	userID := update.CallbackQuery.From.ID
	chatID := update.CallbackQuery.Message.Message.Chat.ID
	messageID := update.CallbackQuery.Message.Message.ID

	_, _ = tg.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
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
		_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Expense not found or unauthorized.",
		})
		return
	}

	if err := b.expenseRepo.Delete(ctx, expenseID); err != nil {
		logger.Log.Error().Err(err).Int("expense_id", expenseID).Msg("Failed to delete expense")
		_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Failed to delete expense. Please try again.",
		})
		return
	}

	logger.Log.Debug().
		Int64("chat_id", chatID).
		Int("expense_id", expenseID).
		Msg("Expense deleted via inline button")

	_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      fmt.Sprintf("✅ Expense #%d deleted.", expenseID),
	})
}

// handleBackToExpenseCallback handles "Back" button to return to original expense view.
func (b *Bot) handleBackToExpenseCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleBackToExpenseCallbackCore(ctx, tgBot, update)
}

// handleBackToExpenseCallbackCore is the testable implementation.
func (b *Bot) handleBackToExpenseCallbackCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	data := update.CallbackQuery.Data
	userID := update.CallbackQuery.From.ID
	chatID := update.CallbackQuery.Message.Message.Chat.ID
	messageID := update.CallbackQuery.Message.Message.ID

	_, _ = tg.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})

	parts := strings.Split(data, "_")
	if len(parts) < 4 {
		return
	}

	expenseID, err := strconv.Atoi(parts[3])
	if err != nil {
		return
	}

	expense, err := b.expenseRepo.GetByID(ctx, expenseID)
	if err != nil || expense.UserID != userID {
		return
	}

	// Load category if needed
	if expense.CategoryID != nil {
		categories, _ := b.getCategoriesWithCache(ctx)
		for i := range categories {
			if categories[i].ID == *expense.CategoryID {
				expense.Category = &categories[i]
				break
			}
		}
	}

	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = expense.Category.Name
	}

	descText := ""
	if expense.Description != "" {
		descText = "\n📝 " + expense.Description
	}

	text := fmt.Sprintf(`✅ <b>Expense Added</b>

💰 $%s SGD%s
📁 %s
🆔 #%d`,
		expense.Amount.StringFixed(2),
		descText,
		categoryText,
		expense.ID)

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✏️ Edit", CallbackData: fmt.Sprintf("edit_expense_%d", expense.ID)},
				{Text: "🗑️ Delete", CallbackData: fmt.Sprintf("delete_expense_%d", expense.ID)},
			},
		},
	}

	_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})
}
