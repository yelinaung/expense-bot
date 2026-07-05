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
	"gitlab.com/yelinaung/expense-bot/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const (
	failedGenerateChartMsg = "❌ Failed to generate chart. Please try again."

	periodLabelWeek  = "Week"
	periodLabelMonth = "Month"
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
	now := b.now()
	safeLoc := normalizeLocation(b.displayLocation)
	current := now.In(safeLoc)

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
		startDate, endDate = getWeekDateRangeAt(current)
		period = periodLabelWeek
		title = fmt.Sprintf("Weekly Expenses (%s to %s)",
			startDate.Format("Jan 2"), endDate.AddDate(0, 0, -1).Format("Jan 2, 2006"))
	case periodMonth:
		startDate, endDate = getMonthDateRangeAt(current)
		period = periodLabelMonth
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
			Text:   failedGenerateChartMsg,
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
	_, genSpan := telemetry.StartSpan(
		ctx, "chart.generate",
		attribute.String("chart.period", period),
		attribute.Int("chart.expense_count", len(expenses)),
	)
	chartData, err := GenerateExpenseChart(expenses, period)
	if err != nil {
		genSpan.RecordError(err)
		genSpan.SetStatus(codes.Error, "chart generation failed")
		genSpan.End()
		logger.Log.Error().Err(err).Msg("Failed to generate chart")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   failedGenerateChartMsg,
		})
		return
	}
	genSpan.SetAttributes(attribute.Int("chart.size_bytes", len(chartData)))
	genSpan.End()

	total, err := b.expenseRepo.GetTotalByUserIDAndDateRange(ctx, userID, startDate, endDate)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to calculate total for chart")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   failedGenerateChartMsg,
		})
		return
	}

	// Format period range for caption
	var periodRange string
	if period == periodLabelWeek {
		periodRange = fmt.Sprintf("%s to %s",
			startDate.Format("Jan 2"), endDate.AddDate(0, 0, -1).Format("Jan 2, 2006"))
	} else {
		periodRange = startDate.Format("January 2006")
	}

	// Send chart as document
	filename := generateChartFilename(strings.ToLower(args), b.displayLocation, now)
	caption := fmt.Sprintf("📊 <b>%s</b>\n\nTotal: $%s SGD\nCount: %d expenses\nPeriod: %s",
		title, total.StringFixed(2), len(expenses), periodRange)

	sendCtx, sendSpan := telemetry.StartSpan(
		ctx, "telegram.send_document",
		attribute.Int("document.size_bytes", len(chartData)),
		attribute.String("document.filename", filename),
	)
	_, err = tg.SendDocument(sendCtx, &bot.SendDocumentParams{
		ChatID:    chatID,
		Document:  &models.InputFileUpload{Filename: filename, Data: bytes.NewReader(chartData)},
		Caption:   caption,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		sendSpan.RecordError(err)
		sendSpan.SetStatus(codes.Error, "send document failed")
		sendSpan.End()
		logger.Log.Error().Err(err).Msg("Failed to send chart document")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to send chart. Please try again.",
		})
		return
	}
	sendSpan.End()

	logger.Log.Info().
		Int64("user_id", userID).
		Str("period", period).
		Int("expense_count", len(expenses)).
		Str("total", total.String()).
		Msg("Chart generated successfully")
}
