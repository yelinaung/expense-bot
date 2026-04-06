package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"

	"gitlab.com/yelinaung/expense-bot/internal/logger"
)

// handleSetTimezone handles the /settimezone command.
func (b *Bot) handleSetTimezone(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleSetTimezoneCore(ctx, tgBot, update)
}

// handleSetTimezoneCore is the testable implementation of handleSetTimezone.
func (b *Bot) handleSetTimezoneCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	args := extractCommandArgs(update.Message.Text, "/settimezone")

	if args == "" {
		text := `<b>Set Your Timezone</b>

Usage: <code>/settimezone Asia/Singapore</code>

<b>Common timezones:</b>
• Asia/Singapore
• Asia/Tokyo
• Asia/Shanghai
• Asia/Kolkata
• America/New_York
• America/Los_Angeles
• Europe/London
• Europe/Berlin
• Australia/Sydney

Use IANA timezone names.`

		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      text,
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	tz := strings.TrimSpace(args)

	loc, err := time.LoadLocation(tz)
	if err != nil {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      fmt.Sprintf("Unknown timezone: <code>%s</code>\n\nUse /settimezone to see common timezones.", tz),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	if err := b.userRepo.UpdateTimezone(ctx, userID, loc.String()); err != nil {
		logger.Log.Error().Err(err).Int64("user_id", userID).Str("timezone", tz).Msg("Failed to update timezone")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Failed to update timezone. Please try again.",
		})
		return
	}

	localNow := time.Now().In(loc)
	logger.Log.Info().Int64("user_id", userID).Str("timezone", loc.String()).Msg("Timezone updated")

	_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      fmt.Sprintf("Timezone set to <b>%s</b>\n\nYour local time: %s", loc.String(), localNow.Format("Mon, 02 Jan 2006 15:04")),
		ParseMode: models.ParseModeHTML,
	})
}

// handleShowTimezone handles the /timezone command.
func (b *Bot) handleShowTimezone(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleShowTimezoneCore(ctx, tgBot, update)
}

// handleShowTimezoneCore is the testable implementation of handleShowTimezone.
func (b *Bot) handleShowTimezoneCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	tz, err := b.userRepo.GetTimezone(ctx, userID)
	if err != nil {
		logger.Log.Error().Err(err).Int64("user_id", userID).Msg("Failed to get timezone")
		tz = appmodels.DefaultTimezone
	}

	loc := b.userLocation(tz)

	localNow := time.Now().In(loc)

	text := fmt.Sprintf(`<b>Timezone Settings</b>

Your timezone: <b>%s</b>
Local time: %s

To change it, use:
<code>/settimezone Asia/Tokyo</code>`, tz, localNow.Format("Mon, 02 Jan 2006 15:04"))

	_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
}
