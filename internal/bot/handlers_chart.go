package bot

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"gitlab.com/yelinaung/expense-bot/internal/logger"
)

// handleChart handles the /chart command to generate visual expense breakdown charts.
func (b *Bot) handleChart(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleChartCore(ctx, tgBot, update)
}

// handleChartCore is the testable implementation of handleChart.
func (b *Bot) handleChartCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	args := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/chart"))
	if args == "" {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Please specify chart type.\n\nUsage: <code>/chart week</code> or <code>/chart month</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	var startDate, endDate time.Time
	var period, title string

	switch strings.ToLower(args) {
	case periodWeek:
		startDate, endDate = getWeekDateRange()
		period = "Week"
		title = fmt.Sprintf("Weekly Expenses (%s to %s)",
			startDate.Format("Jan 2"), endDate.Add(-24*time.Hour).Format("Jan 2, 2006"))
	case periodMonth:
		startDate, endDate = getMonthDateRange()
		period = "Month"
		title = fmt.Sprintf("Monthly Expenses (%s)", startDate.Format("January 2006"))
	default:
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Invalid chart type. Use <code>week</code> or <code>month</code>.",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	logger.Log.Info().
		Int64("user_id", userID).
		Str("period", period).
		Time("start", startDate).
		Time("end", endDate).
		Msg("Generating expense chart")

	// Fetch expenses
	expenses, err := b.expenseRepo.GetByUserIDAndDateRange(ctx, userID, startDate, endDate)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch expenses for chart")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to generate chart. Please try again.",
		})
		return
	}

	if len(expenses) == 0 {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      fmt.Sprintf("📊 No expenses found for %s.", strings.ToLower(period)),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Generate chart
	chartData, err := GenerateExpenseChart(expenses, period)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to generate chart")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to generate chart. Please try again.",
		})
		return
	}

	total, err := b.expenseRepo.GetTotalByUserIDAndDateRange(ctx, userID, startDate, endDate)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to calculate total for chart")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to generate chart. Please try again.",
		})
		return
	}

	// Format period range for caption
	var periodRange string
	if period == "Week" {
		periodRange = fmt.Sprintf("%s to %s",
			startDate.Format("Jan 2"), endDate.Add(-24*time.Hour).Format("Jan 2, 2006"))
	} else {
		periodRange = startDate.Format("January 2006")
	}

	// Send chart as document
	filename := generateChartFilename(strings.ToLower(args))
	caption := fmt.Sprintf("📊 <b>%s</b>\n\nTotal: $%s SGD\nCount: %d expenses\nPeriod: %s",
		title, total.StringFixed(2), len(expenses), periodRange)

	_, err = tg.SendDocument(ctx, &bot.SendDocumentParams{
		ChatID:    chatID,
		Document:  &models.InputFileUpload{Filename: filename, Data: bytes.NewReader(chartData)},
		Caption:   caption,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send chart document")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to send chart. Please try again.",
		})
		return
	}

	logger.Log.Info().
		Int64("user_id", userID).
		Str("period", period).
		Int("expense_count", len(expenses)).
		Str("total", total.String()).
		Msg("Chart generated successfully")
}
