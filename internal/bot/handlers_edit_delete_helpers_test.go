package bot

import (
	"context"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/models"
	"gitlab.com/yelinaung/expense-bot/internal/repository"
)

func findEditableExpense(
	ctx context.Context,
	mock *mocks.MockBot,
	chatID int64,
	expenseRepo *repository.ExpenseRepository,
	userID int64,
	args string,
) (*models.Expense, string, bool) {
	parts := strings.SplitN(args, " ", 2)
	expenseNum, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		sendMockHTMLMessage(ctx, mock, chatID, editInvalidIDHTML)
		return nil, "", false
	}

	expense, err := expenseRepo.GetByUserAndNumber(ctx, userID, expenseNum)
	if err != nil {
		sendMockTextMessage(ctx, mock, chatID, "❌ Expense #"+strconv.FormatInt(expenseNum, 10)+" "+testNotFoundText+".")
		return nil, "", false
	}

	if expense.UserID != userID {
		sendMockTextMessage(ctx, mock, chatID, "❌ You can only edit your own expenses.")
		return nil, "", false
	}

	if len(parts) < 2 {
		sendMockHTMLMessage(ctx, mock, chatID, editProvideValsHTML)
		return nil, "", false
	}

	values := strings.TrimSpace(parts[1])
	if values == "" {
		sendMockHTMLMessage(ctx, mock, chatID, editProvideValsHTML)
		return nil, "", false
	}

	return expense, values, true
}

func loadEditCategories(
	ctx context.Context,
	mock *mocks.MockBot,
	chatID int64,
	categoryRepo *repository.CategoryRepository,
) ([]models.Category, bool) {
	categories, err := categoryRepo.GetAll(ctx)
	if err != nil {
		sendMockTextMessage(ctx, mock, chatID, failedFetchCategoriesMsg)
		return nil, false
	}

	return categories, true
}

func attachExpenseCategory(expense *models.Expense, categories []models.Category) {
	if expense.CategoryID == nil {
		return
	}

	for i := range categories {
		if categories[i].ID == *expense.CategoryID {
			expense.Category = &categories[i]
			return
		}
	}
}

func parseEditValues(
	ctx context.Context,
	mock *mocks.MockBot,
	chatID int64,
	values string,
	categories []models.Category,
) *ParsedExpense {
	parsed := ParseExpenseInputWithCategories(values, categoryNames(categories))
	if parsed == nil {
		sendMockHTMLMessage(ctx, mock, chatID, editInvalidFmtHTML)
		return nil
	}

	return parsed
}

func categoryNames(categories []models.Category) []string {
	names := make([]string, len(categories))
	for i := range categories {
		names[i] = categories[i].Name
	}

	return names
}

func applyParsedExpenseEdit(
	expense *models.Expense,
	parsed *ParsedExpense,
	categories []models.Category,
) {
	expense.Amount = parsed.Amount

	if parsed.Currency != "" {
		expense.Currency = parsed.Currency
	}

	if parsed.Description != "" {
		expense.Description = parsed.Description
	}

	if parsed.CategoryName == "" {
		return
	}

	for i := range categories {
		if strings.EqualFold(categories[i].Name, parsed.CategoryName) {
			expense.CategoryID = &categories[i].ID
			expense.Category = &categories[i]
			return
		}
	}
}

func updateEditedExpense(
	ctx context.Context,
	mock *mocks.MockBot,
	chatID int64,
	expenseRepo *repository.ExpenseRepository,
	expense *models.Expense,
) bool {
	if expenseRepo.Update(ctx, expense) != nil {
		sendMockTextMessage(ctx, mock, chatID, "❌ Failed to update expense. Please try again.")
		return false
	}

	return true
}

func sendEditedExpenseMessage(
	ctx context.Context,
	mock *mocks.MockBot,
	chatID int64,
	expense *models.Expense,
) {
	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = expense.Category.Name
	}

	currency := expense.Currency
	if currency == "" {
		currency = testCurrencySGD
	}

	currencySymbol := models.SupportedCurrencies[currency]
	if currencySymbol == "" {
		currencySymbol = currency
	}

	text := "✅ <b>Expense Updated</b>\n\n🆔 #" + strconv.FormatInt(expense.UserExpenseNumber, 10) +
		"\n💰 " + currencySymbol + expense.Amount.StringFixed(2) + " " + currency +
		"\n📝 " + expense.Description + "\n📁 " + categoryText

	sendMockHTMLMessage(ctx, mock, chatID, text)
}

func sendMockHTMLMessage(ctx context.Context, mock *mocks.MockBot, chatID int64, text string) {
	_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: tgmodels.ParseModeHTML,
	})
}

func sendMockTextMessage(ctx context.Context, mock *mocks.MockBot, chatID int64, text string) {
	_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
}
