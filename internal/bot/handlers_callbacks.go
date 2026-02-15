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

const (
	editCancelText                 = "⬅️ Cancel"
	cancelEditCallback             = "cancel_edit_%d"
	expenseNotFoundMsgCB           = "❌ Expense not found."
	backButtonTextCB               = "⬅️ Back"
	logFieldExpenseIDCB            = "expense_id"
	logFieldUserHashCB             = "user_hash"
	logFieldCategoryIDCB           = "category_id"
	logFieldCategoryCB             = "category"
	logFieldDataCB                 = "data"
	actionEditExpenseCB            = "edit_expense"
	actionDeleteExpenseCB          = "delete_expense"
	backToExpenseCallbackFmtCB     = "back_to_expense_%d"
	editTypeAmountCB               = "amount"
	editTypeMerchantCB             = "merchant"
	userMismatchOnEditMsgCB        = "User mismatch on edit"
	userMismatchMsgCB              = "User mismatch"
	expenseNotFoundForEditLogMsgCB = "Expense not found for edit"
	expenseNotFoundLogMsgCB        = "Expense not found"
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
	// Defensive dispatch: inline action callbacks like edit_expense_123
	// may be routed here because of the generic "edit_" prefix handler.
	// Delegate so edit/delete buttons always work.
	if strings.HasPrefix(data, "edit_expense_") || strings.HasPrefix(data, "delete_expense_") {
		b.handleExpenseActionCallbackCore(ctx, tg, update)
		return
	}

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
	case editTypeAmountCB:
		b.promptEditAmountCore(ctx, tg, chatID, messageID, expense)

	case editTypeMerchantCB:
		b.promptEditMerchantCore(ctx, tg, chatID, messageID, expense)

	case logFieldCategoryCB:
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
		EditType:  editTypeAmountCB,
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
				{Text: editCancelText, CallbackData: fmt.Sprintf(cancelEditCallback, expense.ID)},
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

// promptEditMerchantCore prompts the user to enter a new merchant name.
func (b *Bot) promptEditMerchantCore(
	ctx context.Context,
	tg TelegramAPI,
	chatID int64,
	messageID int,
	expense *appmodels.Expense,
) {
	b.pendingEditsMu.Lock()
	b.pendingEdits[chatID] = &pendingEdit{
		ExpenseID: expense.ID,
		EditType:  editTypeMerchantCB,
		MessageID: messageID,
	}
	b.pendingEditsMu.Unlock()

	text := fmt.Sprintf(`🏪 <b>Edit Merchant</b>

Current merchant: %s

Please type the new merchant name:`,
		expense.Merchant)

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: editCancelText, CallbackData: fmt.Sprintf(cancelEditCallback, expense.ID)},
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
	case editTypeAmountCB:
		return b.processAmountEditCore(ctx, tg, chatID, userID, pending, update.Message.Text)
	case editTypeMerchantCB:
		return b.processMerchantEditCore(ctx, tg, chatID, userID, pending, update.Message.Text)
	case logFieldCategoryCB:
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
		logger.Log.Error().Err(err).Int(logFieldExpenseIDCB, pending.ExpenseID).Msg(expenseNotFoundForEditLogMsgCB)
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   expenseNotFoundMsgCB,
		})
		return true
	}

	if expense.UserID != userID {
		logger.Log.Warn().Str(logFieldUserHashCB, logger.HashUserID(userID)).Int(logFieldExpenseIDCB, pending.ExpenseID).Msg(userMismatchOnEditMsgCB)
		return true
	}

	// Update the expense amount.
	expense.Amount = amount
	if err := b.expenseRepo.Update(ctx, expense); err != nil {
		logger.Log.Error().Err(err).Int(logFieldExpenseIDCB, expense.ID).Msg("Failed to update amount")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to update amount. Please try again.",
		})
		return true
	}

	logger.Log.Info().
		Int(logFieldExpenseIDCB, expense.ID).
		Str("new_amount", amount.String()).
		Msg("Amount updated via pending edit")

	// Show updated confirmation message.
	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = escapeHTML(expense.Category.Name)
	} else if expense.CategoryID != nil {
		cat, err := b.categoryRepo.GetByID(ctx, *expense.CategoryID)
		if err == nil {
			categoryText = escapeHTML(cat.Name)
		}
	}

	keyboard := buildReceiptConfirmationKeyboard(expense.ID)

	text := fmt.Sprintf(`📸 <b>Amount Updated!</b>

💰 Amount: $%s SGD
🏪 Merchant: %s
📁 Category: %s

Amount updated. Confirm to save.`,
		expense.Amount.StringFixed(2),
		escapeHTML(expense.Merchant),
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

// processMerchantEditCore processes user input for merchant editing.
func (b *Bot) processMerchantEditCore(
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

	merchant := strings.TrimSpace(input)
	if merchant == "" {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Merchant name cannot be empty.",
		})
		return true
	}

	expense, err := b.expenseRepo.GetByID(ctx, pending.ExpenseID)
	if err != nil {
		logger.Log.Error().Err(err).Int(logFieldExpenseIDCB, pending.ExpenseID).Msg(expenseNotFoundForEditLogMsgCB)
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   expenseNotFoundMsgCB,
		})
		return true
	}

	if expense.UserID != userID {
		logger.Log.Warn().Str(logFieldUserHashCB, logger.HashUserID(userID)).Int(logFieldExpenseIDCB, pending.ExpenseID).Msg(userMismatchOnEditMsgCB)
		return true
	}

	expense.Merchant = merchant
	expense.Description = merchant
	if err := b.expenseRepo.Update(ctx, expense); err != nil {
		logger.Log.Error().Err(err).Int(logFieldExpenseIDCB, expense.ID).Msg("Failed to update merchant")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to update merchant. Please try again.",
		})
		return true
	}

	logger.Log.Info().
		Int(logFieldExpenseIDCB, expense.ID).
		Str("new_merchant", merchant).
		Msg("Merchant updated via pending edit")

	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = escapeHTML(expense.Category.Name)
	} else if expense.CategoryID != nil {
		cat, err := b.categoryRepo.GetByID(ctx, *expense.CategoryID)
		if err == nil {
			categoryText = escapeHTML(cat.Name)
		}
	}

	keyboard := buildReceiptConfirmationKeyboard(expense.ID)

	text := fmt.Sprintf(`📸 <b>Merchant Updated!</b>

💰 Amount: $%s SGD
🏪 Merchant: %s
📁 Category: %s

Merchant updated. Confirm to save.`,
		expense.Amount.StringFixed(2),
		escapeHTML(expense.Merchant),
		categoryText)

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
	currentRow := make([]models.InlineKeyboardButton, 0, 2)

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
		{Text: backButtonTextCB, CallbackData: fmt.Sprintf("receipt_edit_%d", expense.ID)},
	})

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}

	text := fmt.Sprintf(`📁 <b>Select Category</b>

Current: %s

		Choose a new category:`,
		escapeHTML(getCategoryName(expense)))

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
		logger.Log.Error().Err(err).Int(logFieldCategoryIDCB, categoryID).Msg("Category not found")
		return
	}

	expense.CategoryID = &categoryID
	expense.Category = category
	if err := b.expenseRepo.Update(ctx, expense); err != nil {
		logger.Log.Error().Err(err).Int(logFieldExpenseIDCB, expense.ID).Msg("Failed to update category")
		return
	}

	logger.Log.Info().
		Int(logFieldExpenseIDCB, expense.ID).
		Str(logFieldCategoryCB, category.Name).
		Msg("Category updated via callback")

	keyboard := buildReceiptConfirmationKeyboard(expense.ID)

	text := fmt.Sprintf(`📸 <b>Receipt Updated!</b>

💰 Amount: $%s SGD
🏪 Merchant: %s
📁 Category: %s

Category updated. Confirm to save.`,
		expense.Amount.StringFixed(2),
		escapeHTML(expense.Merchant),
		escapeHTML(category.Name))

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
		logger.Log.Error().Err(err).Int(logFieldExpenseIDCB, pending.ExpenseID).Msg(expenseNotFoundLogMsgCB)
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   expenseNotFoundMsgCB,
		})
		return true
	}

	if expense.UserID != userID {
		logger.Log.Warn().Str(logFieldUserHashCB, logger.HashUserID(userID)).Int(logFieldExpenseIDCB, pending.ExpenseID).Msg(userMismatchMsgCB)
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
		logger.Log.Error().Err(err).Int(logFieldExpenseIDCB, expense.ID).Msg("Failed to update expense category")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Category created but failed to assign it. Please select it from the list.",
		})
		return true
	}

	logger.Log.Info().
		Int(logFieldExpenseIDCB, expense.ID).
		Int(logFieldCategoryIDCB, category.ID).
		Str("category_name", category.Name).
		Msg("New category created and assigned")

	keyboard := buildReceiptConfirmationKeyboard(expense.ID)

	text := fmt.Sprintf(`📸 <b>Category Created!</b>

💰 Amount: $%s SGD
🏪 Merchant: %s
📁 Category: %s

New category created. Confirm to save.`,
		expense.Amount.StringFixed(2),
		expense.Merchant,
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
		EditType:  logFieldCategoryCB,
		MessageID: messageID,
	}
	b.pendingEditsMu.Unlock()

	text := `📁 <b>Create New Category</b>

Please type the name for the new category (e.g., <code>Subscriptions</code>):`

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: editCancelText, CallbackData: fmt.Sprintf(cancelEditCallback, expense.ID)},
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
		logger.Log.Error().Str(logFieldDataCB, data).Msg("Invalid callback data format")
		return
	}

	action := parts[0] + "_" + parts[1] // actionEditExpenseCB or actionDeleteExpenseCB
	expenseID, err := strconv.Atoi(parts[2])
	if err != nil {
		logger.Log.Error().Err(err).Str(logFieldDataCB, data).Msg("Failed to parse expense ID")
		return
	}

	expense, err := b.expenseRepo.GetByID(ctx, expenseID)
	if err != nil {
		logger.Log.Error().Err(err).Int(logFieldExpenseIDCB, expenseID).Msg(expenseNotFoundLogMsgCB)
		_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      expenseNotFoundMsgCB,
		})
		return
	}

	if expense.UserID != userID {
		logger.Log.Warn().Str(logFieldUserHashCB, logger.HashUserID(userID)).Int(logFieldExpenseIDCB, expenseID).Msg(userMismatchMsgCB)
		_, _ = tg.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "❌ You can only modify your own expenses.",
			ShowAlert:       true,
		})
		return
	}

	switch action {
	case actionEditExpenseCB:
		b.handleInlineEditExpenseCore(ctx, tg, chatID, messageID, expense)
	case actionDeleteExpenseCB:
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
		categoryText = escapeHTML(expense.Category.Name)
	}

	text := fmt.Sprintf(`✏️ <b>Edit Expense #%d</b>

Current Details:
💰 Amount: $%s SGD
📝 Description: %s
📁 Category: %s

What would you like to edit?`,
		expense.UserExpenseNumber,
		expense.Amount.StringFixed(2),
		escapeHTML(expense.Description),
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
				{Text: backButtonTextCB, CallbackData: fmt.Sprintf(backToExpenseCallbackFmtCB, expense.ID)},
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
		escapeHTML(expense.Description),
		expense.UserExpenseNumber)

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Yes, Delete", CallbackData: fmt.Sprintf("confirm_delete_%d", expense.ID)},
			},
			{
				{Text: "❌ No, Keep It", CallbackData: fmt.Sprintf(backToExpenseCallbackFmtCB, expense.ID)},
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
		logger.Log.Error().Err(err).Int(logFieldExpenseIDCB, expenseID).Msg("Failed to delete expense")
		_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Failed to delete expense. Please try again.",
		})
		return
	}

	logger.Log.Debug().
		Int64("chat_id", chatID).
		Int(logFieldExpenseIDCB, expenseID).
		Msg("Expense deleted via inline button")

	_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      fmt.Sprintf("✅ Expense #%d deleted.", expense.UserExpenseNumber),
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
		categories, err := b.getCategoriesWithCache(ctx)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Failed to fetch categories for expense display")
			return
		}
		for i := range categories {
			if categories[i].ID == *expense.CategoryID {
				expense.Category = &categories[i]
				break
			}
		}
	}

	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = escapeHTML(expense.Category.Name)
	}

	descText := ""
	if expense.Description != "" {
		descText = "\n📝 " + escapeHTML(expense.Description)
	}

	text := fmt.Sprintf(`✅ <b>Expense Added</b>

💰 $%s SGD%s
📁 %s
🆔 #%d`,
		expense.Amount.StringFixed(2),
		descText,
		categoryText,
		expense.UserExpenseNumber)

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
