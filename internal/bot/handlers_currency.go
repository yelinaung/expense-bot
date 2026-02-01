package bot

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"

	"gitlab.com/yelinaung/expense-bot/internal/logger"
)

// handleSetCurrency handles the /setcurrency command.
func (b *Bot) handleSetCurrency(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleSetCurrencyCore(ctx, tgBot, update)
}

// handleSetCurrencyCore is the testable implementation of handleSetCurrency.
func (b *Bot) handleSetCurrencyCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	args := strings.TrimPrefix(update.Message.Text, "/setcurrency")
	args = strings.TrimSpace(args)

	// Handle @botname suffix
	if idx := strings.Index(args, "@"); idx == 0 {
		if spaceIdx := strings.Index(args, " "); spaceIdx != -1 {
			args = strings.TrimSpace(args[spaceIdx:])
		} else {
			args = ""
		}
	}

	if args == "" {
		// Show usage and list of supported currencies
		text := b.buildCurrencyListMessage()
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      text,
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	currency := strings.ToUpper(strings.TrimSpace(args))

	// Validate currency
	if _, ok := appmodels.SupportedCurrencies[currency]; !ok {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      fmt.Sprintf("❌ Unknown currency: <code>%s</code>\n\nUse /setcurrency to see supported currencies.", currency),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Update user's default currency
	if err := b.userRepo.UpdateDefaultCurrency(ctx, userID, currency); err != nil {
		logger.Log.Error().Err(err).Int64("user_id", userID).Str("currency", currency).Msg("Failed to update default currency")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to update currency. Please try again.",
		})
		return
	}

	symbol := appmodels.SupportedCurrencies[currency]
	logger.Log.Info().Int64("user_id", userID).Str("currency", currency).Msg("Default currency updated")

	_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      fmt.Sprintf("✅ Default currency set to <b>%s</b> (%s)\n\nNew expenses will use this currency unless you specify otherwise.", currency, symbol),
		ParseMode: models.ParseModeHTML,
	})
}

// handleShowCurrency handles the /currency command to show current default currency.
func (b *Bot) handleShowCurrency(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleShowCurrencyCore(ctx, tgBot, update)
}

// handleShowCurrencyCore is the testable implementation of handleShowCurrency.
func (b *Bot) handleShowCurrencyCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	currency, err := b.userRepo.GetDefaultCurrency(ctx, userID)
	if err != nil {
		logger.Log.Error().Err(err).Int64("user_id", userID).Msg("Failed to get default currency")
		currency = appmodels.DefaultCurrency
	}

	symbol := appmodels.SupportedCurrencies[currency]

	text := fmt.Sprintf(`💱 <b>Currency Settings</b>

Your default currency: <b>%s</b> (%s)

To change it, use:
<code>/setcurrency USD</code>

You can also specify currency per expense:
• <code>$10 Coffee</code> (USD)
• <code>€5 Lunch</code> (EUR)
• <code>50 THB Taxi</code>`, currency, symbol)

	_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
}

// buildCurrencyListMessage builds a message listing all supported currencies.
func (b *Bot) buildCurrencyListMessage() string {
	var sb strings.Builder
	sb.WriteString("💱 <b>Set Default Currency</b>\n\n")
	sb.WriteString("Usage: <code>/setcurrency USD</code>\n\n")
	sb.WriteString("<b>Supported currencies:</b>\n")

	// Sort currencies alphabetically
	codes := make([]string, 0, len(appmodels.SupportedCurrencies))
	for code := range appmodels.SupportedCurrencies {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	for _, code := range codes {
		symbol := appmodels.SupportedCurrencies[code]
		sb.WriteString(fmt.Sprintf("• %s (%s)\n", code, symbol))
	}

	sb.WriteString("\n<b>Tip:</b> You can also use currency symbols:\n")
	sb.WriteString("• <code>$10 Coffee</code> → USD\n")
	sb.WriteString("• <code>€5.50 Lunch</code> → EUR\n")
	sb.WriteString("• <code>50 THB Taxi</code> → THB\n")

	return sb.String()
}
