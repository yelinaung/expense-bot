package bot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"gitlab.com/yelinaung/expense-bot/internal/gemini"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"

	"gitlab.com/yelinaung/expense-bot/internal/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

// buildReceiptConfirmationKeyboard creates the inline keyboard for receipt confirmation.
func buildReceiptConfirmationKeyboard(expenseID int) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Confirm", CallbackData: fmt.Sprintf("receipt_confirm_%d", expenseID)},
				{Text: "✏️ Edit", CallbackData: fmt.Sprintf("receipt_edit_%d", expenseID)},
				{Text: "❌ Cancel", CallbackData: fmt.Sprintf("receipt_cancel_%d", expenseID)},
			},
		},
	}
}

// handlePhoto handles photo messages for receipt OCR.
func (b *Bot) handlePhoto(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handlePhotoCore(ctx, tgBot, update)
}

// handlePhotoCore is the testable implementation of handlePhoto.
func (b *Bot) handlePhotoCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
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
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
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

	_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "📷 Processing receipt...",
	})

	dlCtx, dlSpan := otel.Tracer("expense-bot/telegram").Start(ctx, "telegram.download_file")
	imageBytes, err := b.downloadFile(dlCtx, tg, largestPhoto.FileID)
	if err != nil {
		dlSpan.RecordError(err)
		dlSpan.SetStatus(codes.Error, err.Error())
		dlSpan.End()
		logger.Log.Error().Err(err).
			Int64("chat_id", chatID).
			Int64("user_id", userID).
			Msg("Failed to download photo")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to download photo. Please try again.",
		})
		return
	}
	dlSpan.End()

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
		sendReceiptParseError(ctx, tg, chatID, err)
		return
	}

	isPartial := receiptData.IsPartial()

	logger.Log.Info().
		Int64("chat_id", chatID).
		Int64("user_id", userID).
		Str("amount", receiptData.Amount.String()).
		Str("merchant", receiptData.Merchant).
		Str("category", receiptData.SuggestedCategory).
		Float64("confidence", receiptData.Confidence).
		Bool("partial", isPartial).
		Msg("Receipt parsed")

	categories, err := b.getCategoriesWithCache(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch categories for receipt")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to fetch categories. Please try again.",
		})
		return
	}
	categoryID, category := findCategoryByName(categories, receiptData.SuggestedCategory)

	// Use sensible defaults for partial data.
	merchant := receiptData.Merchant
	if merchant == "" {
		merchant = "Unknown merchant"
	}
	amount, currency, description := b.convertExpenseCurrency(
		ctx,
		userID,
		receiptData.Amount,
		receiptData.Currency,
		merchant,
	)

	expense := &appmodels.Expense{
		UserID:        userID,
		Amount:        amount,
		Currency:      currency,
		Description:   description,
		Merchant:      merchant,
		CategoryID:    categoryID,
		Category:      category,
		ReceiptFileID: largestPhoto.FileID,
		Status:        appmodels.ExpenseStatusDraft,
	}

	if err := b.expenseRepo.Create(ctx, expense); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to create draft expense")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to save expense. Please try again.",
		})
		return
	}

	text := buildReceiptConfirmationText(expense, receiptData.Date, isPartial)

	keyboard := buildReceiptConfirmationKeyboard(expense.ID)

	msg, err := tg.SendMessage(ctx, &bot.SendMessageParams{
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
		Bool("partial", isPartial).
		Msg("Receipt confirmation sent with inline keyboard")
}

func sendReceiptParseError(ctx context.Context, tg TelegramAPI, chatID int64, err error) {
	text := "❌ Could not read this receipt. Please add manually: <code>/add &lt;amount&gt; &lt;description&gt;</code>"
	if errors.Is(err, gemini.ErrParseTimeout) {
		text = "⏱️ Receipt processing timed out. Please try again or add manually: <code>/add &lt;amount&gt; &lt;description&gt;</code>"
	}
	_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
}

func findCategoryByName(
	categories []appmodels.Category,
	name string,
) (*int, *appmodels.Category) {
	for i := range categories {
		if strings.EqualFold(categories[i].Name, name) {
			return &categories[i].ID, &categories[i]
		}
	}
	return nil, nil
}

func buildReceiptConfirmationText(
	expense *appmodels.Expense,
	receiptDate time.Time,
	isPartial bool,
) string {
	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = escapeHTML(expense.Category.Name)
	}
	dateText := "Unknown"
	if !receiptDate.IsZero() {
		dateText = receiptDate.Format("02 Jan 2006")
	}
	currencySymbol := getCurrencyOrCodeSymbol(expense.Currency)
	if isPartial {
		return fmt.Sprintf(`⚠️ <b>Partial Extraction - Please Verify</b>

💰 Amount: %s%s %s
🏪 Merchant: %s
📅 Date: %s
📁 Category: %s

<i>Some data could not be extracted. Please edit or confirm.</i>`,
			currencySymbol,
			expense.Amount.StringFixed(2),
			expense.Currency,
			escapeHTML(expense.Merchant),
			dateText,
			categoryText)
	}

	return fmt.Sprintf(`📸 <b>Receipt Scanned!</b>

💰 Amount: %s%s %s
🏪 Merchant: %s
📅 Date: %s
📁 Category: %s`,
		currencySymbol,
		expense.Amount.StringFixed(2),
		expense.Currency,
		escapeHTML(expense.Merchant),
		dateText,
		categoryText)
}

// handleReceiptCallback handles receipt confirmation button presses.
func (b *Bot) handleReceiptCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleReceiptCallbackCore(ctx, tgBot, update)
}

// handleReceiptCallbackCore is the testable implementation of handleReceiptCallback.
func (b *Bot) handleReceiptCallbackCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
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

	_, _ = tg.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
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
		_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
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
		b.handleConfirmReceiptCore(ctx, tg, chatID, messageID, expense)
	case "cancel":
		b.handleCancelReceiptCore(ctx, tg, chatID, messageID, expense)
	case "edit":
		b.handleEditReceiptCore(ctx, tg, chatID, messageID, expense)
	case "back":
		b.handleBackToReceiptCore(ctx, tg, chatID, messageID, expense)
	}
}

// handleBackToReceiptCore returns to the main receipt confirmation view.
func (b *Bot) handleBackToReceiptCore(
	ctx context.Context,
	tg TelegramAPI,
	chatID int64,
	messageID int,
	expense *appmodels.Expense,
) {
	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = escapeHTML(expense.Category.Name)
	} else if expense.CategoryID != nil {
		cat, err := b.categoryRepo.GetByID(ctx, *expense.CategoryID)
		if err == nil {
			categoryText = escapeHTML(cat.Name)
		}
	}

	text := fmt.Sprintf(`📸 <b>Receipt Scanned!</b>

💰 Amount: %s%s %s
🏪 Merchant: %s
📁 Category: %s`,
		getCurrencyOrCodeSymbol(expense.Currency),
		expense.Amount.StringFixed(2),
		expense.Currency,
		escapeHTML(expense.Merchant),
		categoryText)

	keyboard := buildReceiptConfirmationKeyboard(expense.ID)

	_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})
}

// handleConfirmReceiptCore confirms a draft expense.
func (b *Bot) handleConfirmReceiptCore(
	ctx context.Context,
	tg TelegramAPI,
	chatID int64,
	messageID int,
	expense *appmodels.Expense,
) {
	expense.Status = appmodels.ExpenseStatusConfirmed
	if err := b.expenseRepo.Update(ctx, expense); err != nil {
		logger.Log.Error().Err(err).Int("expense_id", expense.ID).Msg("Failed to confirm expense")
		_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Failed to confirm expense. Please try again.",
		})
		return
	}

	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = escapeHTML(expense.Category.Name)
	} else if expense.CategoryID != nil {
		cat, err := b.categoryRepo.GetByID(ctx, *expense.CategoryID)
		if err == nil {
			categoryText = escapeHTML(cat.Name)
		}
	}

	currencyCode := expense.Currency
	if currencyCode == "" {
		currencyCode = appmodels.DefaultCurrency
	}
	currencySymbol := getCurrencyOrCodeSymbol(currencyCode)

	text := fmt.Sprintf(`✅ <b>Expense Confirmed!</b>

💰 Amount: %s%s %s
🏪 Merchant: %s
📁 Category: %s
🗓️ Date: %s

Expense #%d has been saved.`,
		currencySymbol,
		expense.Amount.StringFixed(2),
		currencyCode,
		escapeHTML(expense.Merchant),
		categoryText,
		expense.CreatedAt.In(b.displayLocation).Format("02 Jan 2006"),
		expense.UserExpenseNumber)

	logger.Log.Info().
		Int("expense_id", expense.ID).
		Str("amount", expense.Amount.String()).
		Msg("Expense confirmed via callback")

	_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
}

// handleCancelReceiptCore cancels and deletes a draft expense.
func (b *Bot) handleCancelReceiptCore(
	ctx context.Context,
	tg TelegramAPI,
	chatID int64,
	messageID int,
	expense *appmodels.Expense,
) {
	if err := b.expenseRepo.Delete(ctx, expense.ID); err != nil {
		logger.Log.Error().Err(err).Int("expense_id", expense.ID).Msg("Failed to delete expense")
		_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Failed to cancel expense. Please try again.",
		})
		return
	}

	logger.Log.Info().
		Int("expense_id", expense.ID).
		Msg("Expense cancelled via callback")

	_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      "🗑️ Receipt scan cancelled. The expense was not saved.",
	})
}

// handleEditReceiptCore shows edit options for a draft expense.
func (b *Bot) handleEditReceiptCore(
	ctx context.Context,
	tg TelegramAPI,
	chatID int64,
	messageID int,
	expense *appmodels.Expense,
) {
	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "💰 Edit Amount", CallbackData: fmt.Sprintf("edit_amount_%d", expense.ID)},
				{Text: "🏪 Edit Merchant", CallbackData: fmt.Sprintf("edit_merchant_%d", expense.ID)},
			},
			{
				{Text: "📁 Edit Category", CallbackData: fmt.Sprintf("edit_category_%d", expense.ID)},
			},
			{
				{Text: "⬅️ Back", CallbackData: fmt.Sprintf("receipt_back_%d", expense.ID)},
			},
		},
	}

	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = escapeHTML(expense.Category.Name)
	} else if expense.CategoryID != nil {
		cat, err := b.categoryRepo.GetByID(ctx, *expense.CategoryID)
		if err == nil {
			categoryText = escapeHTML(cat.Name)
		}
	}

	text := fmt.Sprintf(`✏️ <b>Edit Expense</b>

💰 Amount: %s%s %s
🏪 Merchant: %s
📁 Category: %s

Select what to edit:`,
		getCurrencyOrCodeSymbol(expense.Currency),
		expense.Amount.StringFixed(2),
		expense.Currency,
		escapeHTML(expense.Merchant),
		categoryText)

	_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})
}
