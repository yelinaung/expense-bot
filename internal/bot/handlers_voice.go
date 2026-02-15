package bot

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"gitlab.com/yelinaung/expense-bot/internal/gemini"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"

	"gitlab.com/yelinaung/expense-bot/internal/logger"
)

// handleVoice handles voice messages for expense input.
func (b *Bot) handleVoice(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleVoiceCore(ctx, tgBot, update)
}

// handleVoiceCore is the testable implementation of handleVoice.
func (b *Bot) handleVoiceCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil || update.Message.Voice == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	logger.Log.Info().
		Int64("chat_id", chatID).
		Int64("user_id", userID).
		Int("duration", update.Message.Voice.Duration).
		Msg("Received voice message")

	if b.geminiClient == nil {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "🎙️ Voice expense input is not configured. Please add expenses manually using /add or send text like <code>5.50 Coffee</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "🎙️ Processing voice message...",
	})

	audioBytes, err := b.downloadFile(ctx, tg, update.Message.Voice.FileID)
	if err != nil {
		logger.Log.Error().Err(err).
			Int64("chat_id", chatID).
			Int64("user_id", userID).
			Msg("Failed to download voice file")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to download voice message. Please try again.",
		})
		return
	}

	logger.Log.Info().
		Int64("chat_id", chatID).
		Int64("user_id", userID).
		Int("size_bytes", len(audioBytes)).
		Msg("Voice file downloaded successfully")

	mimeType := update.Message.Voice.MimeType
	if mimeType == "" {
		mimeType = "audio/ogg"
	}

	categories, err := b.getCategoriesWithCache(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch categories for voice expense")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to fetch categories. Please try again.",
		})
		return
	}
	categoryNames := make([]string, len(categories))
	for i, cat := range categories {
		categoryNames[i] = cat.Name
	}
	if len(categoryNames) == 0 {
		categoryNames = gemini.DefaultCategories
	}

	voiceData, err := b.geminiClient.ParseVoiceExpense(ctx, audioBytes, mimeType, categoryNames)
	if err != nil {
		logger.Log.Error().Err(err).
			Int64("chat_id", chatID).
			Int64("user_id", userID).
			Msg("Failed to parse voice expense")
		sendVoiceParseError(ctx, tg, chatID, err)
		return
	}

	logger.Log.Info().
		Int64("chat_id", chatID).
		Int64("user_id", userID).
		Str("amount", voiceData.Amount.String()).
		Str("description", voiceData.Description).
		Str("currency", voiceData.Currency).
		Str("category", voiceData.SuggestedCategory).
		Float64("confidence", voiceData.Confidence).
		Msg("Voice expense parsed")

	categoryID, category := findCategoryByName(categories, voiceData.SuggestedCategory)

	description := voiceData.Description
	if description == "" {
		description = "Voice expense"
	}
	merchant := description
	amount, currency, description := b.convertExpenseCurrency(
		ctx,
		userID,
		voiceData.Amount,
		voiceData.Currency,
		description,
	)

	expense := &appmodels.Expense{
		UserID:      userID,
		Amount:      amount,
		Currency:    currency,
		Description: description,
		Merchant:    merchant,
		CategoryID:  categoryID,
		Category:    category,
		Status:      appmodels.ExpenseStatusDraft,
	}

	if err := b.expenseRepo.Create(ctx, expense); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to create draft expense from voice")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to save expense. Please try again.",
		})
		return
	}

	text := buildVoiceConfirmationText(expense)

	keyboard := buildReceiptConfirmationKeyboard(expense.ID)

	msg, err := tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send voice expense confirmation")
		return
	}

	logger.Log.Debug().
		Int64("chat_id", chatID).
		Int("expense_id", expense.ID).
		Int("message_id", msg.ID).
		Msg("Voice expense confirmation sent with inline keyboard")
}

func sendVoiceParseError(ctx context.Context, tg TelegramAPI, chatID int64, err error) {
	text := "❌ Failed to process voice message. Please try again or add manually: <code>/add &lt;amount&gt; &lt;description&gt;</code>"
	if errors.Is(err, gemini.ErrVoiceParseTimeout) {
		text = "⏱️ Voice processing timed out. Please try again or add manually: <code>/add &lt;amount&gt; &lt;description&gt;</code>"
	}
	if errors.Is(err, gemini.ErrNoVoiceData) {
		text = "❌ Could not extract expense information from your voice message. Please try again or add manually: <code>/add &lt;amount&gt; &lt;description&gt;</code>"
	}
	_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
}

func buildVoiceConfirmationText(expense *appmodels.Expense) string {
	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = escapeHTML(expense.Category.Name)
	}
	currencySymbol := getCurrencyOrCodeSymbol(expense.Currency)
	return fmt.Sprintf(`🎙️ <b>Voice Expense Detected!</b>

💰 Amount: %s%s %s
📝 Description: %s
📁 Category: %s

Please confirm, edit, or cancel:`,
		currencySymbol,
		expense.Amount.StringFixed(2),
		expense.Currency,
		escapeHTML(expense.Description),
		categoryText)
}
