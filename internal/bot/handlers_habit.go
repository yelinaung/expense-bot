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
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"gitlab.com/yelinaung/expense-bot/internal/logger"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

const (
	reviewWorthPrefix    = "review_worth_"
	reviewNotWorthPrefix = "review_not_worth_"
	reviewLaterPrefix    = "review_later_"
	reviewSkipPrefix     = "review_skip_"
	reviewDriverPrefix   = "review_driver_"

	noExpensesToReviewMsg   = "No expenses to review."
	noMoreExpensesReviewMsg = "No more expenses to review."
	driverPromptHTML        = "<b>What drove this spend?</b>"
	failedSaveReflectionMsg = "Failed to save reflection. Please try again."
	invalidHabitPeriodMsg   = "Invalid habit period. Use <code>/habit week</code>, <code>/habit month</code>, or <code>/habit 90d</code>."
)

func (b *Bot) handleReview(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleReviewCore(ctx, tgBot, update)
}

func (b *Bot) handleReviewCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	expenses, err := b.expenseRepo.GetUnreviewedByUserID(ctx, userID, 1)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch unreviewed expenses")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: failedFetchExpensesMsg})
		return
	}
	if len(expenses) == 0 {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: noExpensesToReviewMsg})
		return
	}

	loc := b.locationForUser(ctx, userID)
	_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        formatReviewPrompt(&expenses[0], loc),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: buildReviewKeyboard(expenses[0].ID),
	})
}

func (b *Bot) handleHabit(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleHabitCore(ctx, tgBot, update)
}

func (b *Bot) handleHabitCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	period := strings.ToLower(extractCommandArgs(update.Message.Text, "/habit"))
	if period == "" {
		period = periodMonth
	}

	loc := b.locationForUser(ctx, userID)
	current := b.now().In(loc)
	startDate, endDate, label, ok := habitPeriodRange(period, current)
	if !ok {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      invalidHabitPeriodMsg,
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	expenses, err := b.expenseRepo.GetByUserIDAndDateRange(ctx, userID, startDate, endDate)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch expenses for habit summary")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: failedFetchExpensesMsg})
		return
	}

	reviewed, err := b.expenseRepo.GetReviewedByUserIDAndDateRange(ctx, userID, startDate, endDate)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch reviewed expenses for habit summary")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: failedFetchExpensesMsg})
		return
	}

	summary := analyzeExpenseHabit(len(expenses), reviewed, loc, label)
	_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      formatHabitSummary(&summary),
		ParseMode: models.ParseModeHTML,
	})
}

func (b *Bot) handleReviewCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleReviewCallbackCore(ctx, tgBot, update)
}

func (b *Bot) handleReviewCallbackCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.CallbackQuery == nil || update.CallbackQuery.Message.Message == nil {
		return
	}

	data := update.CallbackQuery.Data
	userID := update.CallbackQuery.From.ID
	chatID := update.CallbackQuery.Message.Message.Chat.ID
	messageID := update.CallbackQuery.Message.Message.ID

	_, _ = tg.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: update.CallbackQuery.ID})

	switch {
	case strings.HasPrefix(data, reviewWorthPrefix):
		expenseID, ok := parseReviewID(data, reviewWorthPrefix)
		if ok {
			b.showDriverPrompt(ctx, tg, chatID, messageID, userID, expenseID, true)
		}
	case strings.HasPrefix(data, reviewNotWorthPrefix):
		expenseID, ok := parseReviewID(data, reviewNotWorthPrefix)
		if ok {
			b.showDriverPrompt(ctx, tg, chatID, messageID, userID, expenseID, false)
		}
	case strings.HasPrefix(data, reviewLaterPrefix):
		expenseID, ok := parseReviewID(data, reviewLaterPrefix)
		if ok {
			b.dismissReflectionButtons(ctx, tg, chatID, messageID, userID, expenseID, update.CallbackQuery.Message.Message.Text)
		}
	case strings.HasPrefix(data, reviewSkipPrefix):
		expenseID, ok := parseReviewID(data, reviewSkipPrefix)
		if ok {
			b.editToNextReviewOrDone(ctx, tg, chatID, messageID, userID, expenseID)
		}
	case strings.HasPrefix(data, reviewDriverPrefix):
		b.handleReviewDriverCallback(ctx, tg, chatID, messageID, userID, data)
	}
}

func (b *Bot) showDriverPrompt(
	ctx context.Context,
	tg TelegramAPI,
	chatID int64,
	messageID int,
	userID int64,
	expenseID int,
	worthIt bool,
) {
	expense, ok := b.getOwnedExpense(ctx, tg, updateTarget{chatID: chatID, messageID: messageID}, userID, expenseID)
	if !ok {
		return
	}

	loc := b.locationForUser(ctx, userID)
	text := formatReviewPrompt(expense, loc) + "\n\n" + driverPromptHTML
	_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: buildDriverKeyboard(expenseID, worthIt),
	})
}

func (b *Bot) handleReviewDriverCallback(
	ctx context.Context,
	tg TelegramAPI,
	chatID int64,
	messageID int,
	userID int64,
	data string,
) {
	callback := parseDriverCallback(data)
	if !callback.ok || callback.driverIndex < 0 || callback.driverIndex >= len(spendingDrivers) {
		return
	}
	if _, ok := b.getOwnedExpense(ctx, tg, updateTarget{chatID: chatID, messageID: messageID}, userID, callback.expenseID); !ok {
		return
	}

	driver := string(spendingDrivers[callback.driverIndex])
	if err := b.expenseRepo.UpdateReflection(ctx, callback.expenseID, userID, &callback.worthIt, driver); err != nil {
		logger.Log.Error().Err(err).Int("expense_id", callback.expenseID).Msg("Failed to update reflection")
		_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      failedSaveReflectionMsg,
		})
		return
	}

	b.editToNextReviewOrDone(ctx, tg, chatID, messageID, userID, callback.expenseID)
}

type updateTarget struct {
	chatID    int64
	messageID int
}

func (b *Bot) getOwnedExpense(
	ctx context.Context,
	tg TelegramAPI,
	target updateTarget,
	userID int64,
	expenseID int,
) (*appmodels.Expense, bool) {
	expense, err := b.expenseRepo.GetByID(ctx, expenseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID:    target.chatID,
				MessageID: target.messageID,
				Text:      expenseNotFoundMsgCB,
			})
			return nil, false
		}

		logger.Log.Error().
			Err(err).
			Int(logFieldExpenseIDCB, expenseID).
			Str(logFieldUserHashCB, logger.HashUserID(userID)).
			Msg("Failed to get expense for reflection")
		_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    target.chatID,
			MessageID: target.messageID,
			Text:      expenseUnexpectedErrorMsgCB,
		})
		return nil, false
	}

	if expense.UserID != userID {
		_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    target.chatID,
			MessageID: target.messageID,
			Text:      expenseNotFoundMsgCB,
		})
		return nil, false
	}
	return expense, true
}

func (b *Bot) dismissReflectionButtons(
	ctx context.Context,
	tg TelegramAPI,
	chatID int64,
	messageID int,
	userID int64,
	expenseID int,
	text string,
) {
	expense, ok := b.getOwnedExpense(ctx, tg, updateTarget{chatID: chatID, messageID: messageID}, userID, expenseID)
	if !ok {
		return
	}
	if text == "" {
		text = buildExpenseAddedMessage(expense, nil)
	}
	_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: buildExpenseActionKeyboard(expense.ID),
	})
}

func (b *Bot) editToNextReviewOrDone(
	ctx context.Context,
	tg TelegramAPI,
	chatID int64,
	messageID int,
	userID int64,
	currentExpenseID int,
) {
	next, err := b.expenseRepo.GetNextUnreviewedByUserID(ctx, userID, currentExpenseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID:    chatID,
				MessageID: messageID,
				Text:      noMoreExpensesReviewMsg,
			})
			return
		}
		logger.Log.Error().Err(err).Msg("Failed to fetch next unreviewed expense")
		_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      failedFetchExpensesMsg,
		})
		return
	}

	loc := b.locationForUser(ctx, userID)
	_, _ = tg.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        formatReviewPrompt(next, loc),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: buildReviewKeyboard(next.ID),
	})
}

func buildExpenseActionKeyboard(expenseID int) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: editExpenseButtonTextCB, CallbackData: fmt.Sprintf(editExpenseCallbackFmtCB, expenseID)},
				{Text: deleteExpenseButtonTextCB, CallbackData: fmt.Sprintf(deleteExpenseCallbackFmtCB, expenseID)},
			},
		},
	}
}

func buildExpenseReflectionKeyboard(expenseID int) *models.InlineKeyboardMarkup {
	keyboard := buildExpenseActionKeyboard(expenseID)
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []models.InlineKeyboardButton{
		{Text: "Worth it", CallbackData: fmt.Sprintf("%s%d", reviewWorthPrefix, expenseID)},
		{Text: "Not worth it", CallbackData: fmt.Sprintf("%s%d", reviewNotWorthPrefix, expenseID)},
		{Text: "Later", CallbackData: fmt.Sprintf("%s%d", reviewLaterPrefix, expenseID)},
	})
	return keyboard
}

func buildReviewKeyboard(expenseID int) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Worth it", CallbackData: fmt.Sprintf("%s%d", reviewWorthPrefix, expenseID)},
				{Text: "Not worth it", CallbackData: fmt.Sprintf("%s%d", reviewNotWorthPrefix, expenseID)},
			},
			{
				{Text: "Skip", CallbackData: fmt.Sprintf("%s%d", reviewSkipPrefix, expenseID)},
			},
		},
	}
}

func buildDriverKeyboard(expenseID int, worthIt bool) *models.InlineKeyboardMarkup {
	worthBit := 0
	if worthIt {
		worthBit = 1
	}

	rows := make([][]models.InlineKeyboardButton, 0, len(spendingDrivers)/2+1)
	for i := 0; i < len(spendingDrivers); i += 2 {
		row := []models.InlineKeyboardButton{{
			Text:         string(spendingDrivers[i]),
			CallbackData: fmt.Sprintf("%s%d_%d_%d", reviewDriverPrefix, expenseID, worthBit, i),
		}}
		if i+1 < len(spendingDrivers) {
			row = append(row, models.InlineKeyboardButton{
				Text:         string(spendingDrivers[i+1]),
				CallbackData: fmt.Sprintf("%s%d_%d_%d", reviewDriverPrefix, expenseID, worthBit, i+1),
			})
		}
		rows = append(rows, row)
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func parseReviewID(data, prefix string) (int, bool) {
	expenseID, err := strconv.Atoi(strings.TrimPrefix(data, prefix))
	return expenseID, err == nil
}

type reviewDriverCallback struct {
	expenseID   int
	worthIt     bool
	driverIndex int
	ok          bool
}

func parseDriverCallback(data string) reviewDriverCallback {
	payload := strings.TrimPrefix(data, reviewDriverPrefix)
	parts := strings.Split(payload, "_")
	if len(parts) != 3 {
		return reviewDriverCallback{}
	}
	expenseID, err := strconv.Atoi(parts[0])
	if err != nil {
		return reviewDriverCallback{}
	}
	worthBit, err := strconv.Atoi(parts[1])
	if err != nil || (worthBit != 0 && worthBit != 1) {
		return reviewDriverCallback{}
	}
	driverIndex, err := strconv.Atoi(parts[2])
	if err != nil {
		return reviewDriverCallback{}
	}
	return reviewDriverCallback{
		expenseID:   expenseID,
		worthIt:     worthBit == 1,
		driverIndex: driverIndex,
		ok:          true,
	}
}

func formatReviewPrompt(expense *appmodels.Expense, loc *time.Location) string {
	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = escapeHTML(expense.Category.Name)
	}
	description := expense.Description
	if description == "" {
		description = expense.Merchant
	}

	return fmt.Sprintf(
		`Spending reflection

<b>Was this worth it?</b>

%s%s %s
%s
%s
%s`,
		escapeHTML(getCurrencyOrCodeSymbol(expense.Currency)),
		escapeHTML(expense.Amount.StringFixed(2)),
		escapeHTML(expense.Currency),
		escapeHTML(description),
		categoryText,
		escapeHTML(expense.CreatedAt.In(normalizeLocation(loc)).Format("02 Jan 2006 15:04")),
	)
}

func habitPeriodRange(period string, current time.Time) (time.Time, time.Time, string, bool) {
	switch period {
	case periodWeek:
		start, end := getWeekDateRangeAt(current)
		return start, end, "This week", true
	case periodMonth:
		start, end := getMonthDateRangeAt(current)
		return start, end, start.Format("January 2006"), true
	case "90d":
		start, end := getRollingDayRangeAt(current, 90)
		return start, end, "Last 90 days", true
	default:
		return time.Time{}, time.Time{}, "", false
	}
}

func formatHabitSummary(summary *habitSummary) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "<b>Spending Reflection</b>\n%s\n\n", escapeHTML(summary.PeriodLabel))
	fmt.Fprintf(&sb, "Reviewed: %d/%d\n", summary.ReviewedCount, summary.TotalCount)
	fmt.Fprintf(&sb, "Worth it: %d\n", summary.WorthItCount)
	fmt.Fprintf(&sb, "Not worth it: %d\n\n", summary.NotWorthItCount)

	sb.WriteString("Worth-it spend:\n")
	appendCurrencyTotals(&sb, summary.WorthItByCurrency)
	sb.WriteString("\nNot-worth-it spend:\n")
	appendCurrencyTotals(&sb, summary.NotWorthItByCurrency)

	fmt.Fprintf(&sb, "\nBest-value category: %s\n", habitCategoryOrFallback(summary.BestValueCategory))
	fmt.Fprintf(&sb, "Most-regretted category: %s\n", habitCategoryOrFallback(summary.MostRegrettedCategory))
	if summary.TopDriver != "" {
		fmt.Fprintf(&sb, "Top driver: %s\n", escapeHTML(string(summary.TopDriver)))
	} else {
		sb.WriteString("Top driver: Not enough data\n")
	}
	if summary.HasBusiestNotWorthItWeekday {
		fmt.Fprintf(&sb, "Not-worth-it weekday: %s\n", summary.BusiestNotWorthItWeekday.String())
	} else {
		sb.WriteString("Not-worth-it weekday: Not enough data\n")
	}

	fmt.Fprintf(&sb, "\n%s", escapeHTML(formatHabitInsight(summary)))
	return sb.String()
}

func formatHabitInsight(summary *habitSummary) string {
	switch {
	case summary.ReviewedCount == 0:
		return "Review a few expenses with /review to build your spending reflection."
	case summary.MostRegrettedCategory != "":
		return "Your not-worth-it spending showed up most often in " + summary.MostRegrettedCategory + "."
	case summary.TopDriver != "":
		return "Your most common spending driver was " + string(summary.TopDriver) + "."
	default:
		return "Keep reviewing expenses to reveal stronger spending patterns."
	}
}

func appendCurrencyTotals(sb *strings.Builder, totals map[string]decimal.Decimal) {
	if len(totals) == 0 {
		sb.WriteString("  None\n")
		return
	}
	for _, currency := range sortedCurrencyKeys(totals) {
		fmt.Fprintf(
			sb, "  %s: %s%s\n",
			escapeHTML(currency),
			escapeHTML(getCurrencyOrCodeSymbol(currency)),
			totals[currency].StringFixed(2),
		)
	}
}

func habitCategoryOrFallback(category string) string {
	if category == "" {
		return "Not enough data"
	}
	return escapeHTML(category)
}

func (b *Bot) locationForUser(ctx context.Context, userID int64) *time.Location {
	user, err := b.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to fetch user timezone, using fallback location")
		return b.userLocation("")
	}
	return b.userLocation(user.Timezone)
}
