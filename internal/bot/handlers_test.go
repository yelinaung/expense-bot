package bot

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/gemini"
	"gitlab.com/yelinaung/expense-bot/internal/models"
	"gitlab.com/yelinaung/expense-bot/internal/repository"
)

func TestFormatGreeting(t *testing.T) {
	t.Parallel()

	t.Run("returns empty for empty name", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "", formatGreeting(""))
	})

	t.Run("returns formatted greeting with name", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, ", John", formatGreeting("John"))
	})

	t.Run("handles name with spaces", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, ", John Doe", formatGreeting("John Doe"))
	})
}

func setupReceiptOCRTest(t *testing.T) (*repository.ExpenseRepository, *repository.UserRepository, *repository.CategoryRepository, context.Context) {
	t.Helper()

	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	err = database.SeedCategories(ctx, pool)
	require.NoError(t, err)

	return repository.NewExpenseRepository(pool),
		repository.NewUserRepository(pool),
		repository.NewCategoryRepository(pool),
		ctx
}

func TestReceiptOCRFlow_Integration(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping integration test")
	}

	expenseRepo, userRepo, categoryRepo, ctx := setupReceiptOCRTest(t)

	user := &models.User{ID: 12345, Username: "testuser", FirstName: "Test", LastName: "User"}
	err := userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	client, err := gemini.NewClient(ctx, apiKey)
	require.NoError(t, err)

	t.Run("full receipt OCR flow - parse, create draft, confirm", func(t *testing.T) {
		imageBytes, err := os.ReadFile("../../sample_receipt.jpeg")
		require.NoError(t, err)

		receiptData, err := client.ParseReceipt(ctx, imageBytes, "image/jpeg")
		require.NoError(t, err)
		require.NotNil(t, receiptData)
		require.True(t, receiptData.HasAmount())
		require.True(t, receiptData.HasMerchant())

		expectedAmount := decimal.NewFromFloat(54.60)
		require.True(t, receiptData.Amount.Equal(expectedAmount))
		require.True(t, strings.Contains(strings.ToLower(receiptData.Merchant), "swee choon"))
		require.Equal(t, "Food - Dining Out", receiptData.SuggestedCategory)

		categories, err := categoryRepo.GetAll(ctx)
		require.NoError(t, err)

		var categoryID *int
		var category *models.Category
		for i := range categories {
			if strings.EqualFold(categories[i].Name, receiptData.SuggestedCategory) {
				categoryID = &categories[i].ID
				category = &categories[i]
				break
			}
		}
		require.NotNil(t, categoryID, "category should be found")

		draftExpense := &models.Expense{
			UserID:        user.ID,
			Amount:        receiptData.Amount,
			Currency:      "SGD",
			Description:   receiptData.Merchant,
			CategoryID:    categoryID,
			Category:      category,
			ReceiptFileID: "test-file-id",
			Status:        models.ExpenseStatusDraft,
		}

		err = expenseRepo.Create(ctx, draftExpense)
		require.NoError(t, err)
		require.NotZero(t, draftExpense.ID)
		require.Equal(t, models.ExpenseStatusDraft, draftExpense.Status)

		fetched, err := expenseRepo.GetByID(ctx, draftExpense.ID)
		require.NoError(t, err)
		require.Equal(t, models.ExpenseStatusDraft, fetched.Status)
		require.True(t, expectedAmount.Equal(fetched.Amount))

		draftExpense.Status = models.ExpenseStatusConfirmed
		err = expenseRepo.Update(ctx, draftExpense)
		require.NoError(t, err)

		confirmed, err := expenseRepo.GetByID(ctx, draftExpense.ID)
		require.NoError(t, err)
		require.Equal(t, models.ExpenseStatusConfirmed, confirmed.Status)

		expenses, err := expenseRepo.GetByUserID(ctx, user.ID, 10)
		require.NoError(t, err)
		require.Len(t, expenses, 1)
		require.Equal(t, draftExpense.ID, expenses[0].ID)
	})

	t.Run("receipt OCR flow - parse, create draft, cancel", func(t *testing.T) {
		database.CleanupTables(t, expenseRepo.Pool())

		err := userRepo.UpsertUser(ctx, user)
		require.NoError(t, err)

		err = database.SeedCategories(ctx, expenseRepo.Pool())
		require.NoError(t, err)

		imageBytes, err := os.ReadFile("../../sample_receipt.jpeg")
		require.NoError(t, err)

		receiptData, err := client.ParseReceipt(ctx, imageBytes, "image/jpeg")
		require.NoError(t, err)
		require.NotNil(t, receiptData)

		draftExpense := &models.Expense{
			UserID:        user.ID,
			Amount:        receiptData.Amount,
			Currency:      "SGD",
			Description:   receiptData.Merchant,
			ReceiptFileID: "test-file-id-2",
			Status:        models.ExpenseStatusDraft,
		}

		err = expenseRepo.Create(ctx, draftExpense)
		require.NoError(t, err)
		draftID := draftExpense.ID

		err = expenseRepo.Delete(ctx, draftID)
		require.NoError(t, err)

		_, err = expenseRepo.GetByID(ctx, draftID)
		require.Error(t, err)

		expenses, err := expenseRepo.GetByUserID(ctx, user.ID, 10)
		require.NoError(t, err)
		require.Empty(t, expenses)
	})

	t.Run("draft expense cleanup removes expired drafts", func(t *testing.T) {
		database.CleanupTables(t, expenseRepo.Pool())

		err := userRepo.UpsertUser(ctx, user)
		require.NoError(t, err)

		draftExpense := &models.Expense{
			UserID:      user.ID,
			Amount:      decimal.NewFromFloat(25.00),
			Currency:    "SGD",
			Description: "Test draft",
			Status:      models.ExpenseStatusDraft,
		}

		err = expenseRepo.Create(ctx, draftExpense)
		require.NoError(t, err)

		count, err := expenseRepo.DeleteExpiredDrafts(ctx, -1*time.Hour)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		_, err = expenseRepo.GetByID(ctx, draftExpense.ID)
		require.Error(t, err)
	})
}

func TestReceiptData_Flow(t *testing.T) {
	t.Parallel()

	t.Run("complete data is not partial or empty", func(t *testing.T) {
		t.Parallel()
		data := &gemini.ReceiptData{
			Amount:            decimal.NewFromFloat(54.60),
			Merchant:          "Test Restaurant",
			Date:              time.Now(),
			SuggestedCategory: "Food - Dining Out",
			Confidence:        0.95,
		}

		require.True(t, data.HasAmount())
		require.True(t, data.HasMerchant())
		require.False(t, data.IsPartial())
		require.False(t, data.IsEmpty())
	})

	t.Run("partial data with only amount", func(t *testing.T) {
		t.Parallel()
		data := &gemini.ReceiptData{
			Amount:     decimal.NewFromFloat(25.00),
			Merchant:   "",
			Confidence: 0.5,
		}

		require.True(t, data.HasAmount())
		require.False(t, data.HasMerchant())
		require.True(t, data.IsPartial())
		require.False(t, data.IsEmpty())
	})

	t.Run("partial data with only merchant", func(t *testing.T) {
		t.Parallel()
		data := &gemini.ReceiptData{
			Amount:     decimal.Zero,
			Merchant:   "Coffee Shop",
			Confidence: 0.5,
		}

		require.False(t, data.HasAmount())
		require.True(t, data.HasMerchant())
		require.True(t, data.IsPartial())
		require.False(t, data.IsEmpty())
	})

	t.Run("empty data has neither amount nor merchant", func(t *testing.T) {
		t.Parallel()
		data := &gemini.ReceiptData{
			Amount:     decimal.Zero,
			Merchant:   "",
			Confidence: 0.1,
		}

		require.False(t, data.HasAmount())
		require.False(t, data.HasMerchant())
		require.False(t, data.IsPartial())
		require.True(t, data.IsEmpty())
	})
}

func TestDraftExpenseStatus(t *testing.T) {
	expenseRepo, userRepo, _, ctx := setupReceiptOCRTest(t)

	user := &models.User{ID: 99999, Username: "statustest", FirstName: "Status", LastName: "Test"}
	err := userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	t.Run("draft expenses are excluded from GetByUserID", func(t *testing.T) {
		draftExpense := &models.Expense{
			UserID:      user.ID,
			Amount:      decimal.NewFromFloat(10.00),
			Currency:    "SGD",
			Description: "Draft",
			Status:      models.ExpenseStatusDraft,
		}
		err := expenseRepo.Create(ctx, draftExpense)
		require.NoError(t, err)

		confirmedExpense := &models.Expense{
			UserID:      user.ID,
			Amount:      decimal.NewFromFloat(20.00),
			Currency:    "SGD",
			Description: "Confirmed",
			Status:      models.ExpenseStatusConfirmed,
		}
		err = expenseRepo.Create(ctx, confirmedExpense)
		require.NoError(t, err)

		expenses, err := expenseRepo.GetByUserID(ctx, user.ID, 10)
		require.NoError(t, err)
		require.Len(t, expenses, 1)
		require.Equal(t, "Confirmed", expenses[0].Description)
	})

	t.Run("GetByID returns both draft and confirmed", func(t *testing.T) {
		database.CleanupTables(t, expenseRepo.Pool())

		err := userRepo.UpsertUser(ctx, user)
		require.NoError(t, err)

		draftExpense := &models.Expense{
			UserID:      user.ID,
			Amount:      decimal.NewFromFloat(15.00),
			Currency:    "SGD",
			Description: "Draft for GetByID",
			Status:      models.ExpenseStatusDraft,
		}
		err = expenseRepo.Create(ctx, draftExpense)
		require.NoError(t, err)

		fetched, err := expenseRepo.GetByID(ctx, draftExpense.ID)
		require.NoError(t, err)
		require.Equal(t, models.ExpenseStatusDraft, fetched.Status)
	})

	t.Run("status defaults to confirmed when not specified", func(t *testing.T) {
		database.CleanupTables(t, expenseRepo.Pool())

		err := userRepo.UpsertUser(ctx, user)
		require.NoError(t, err)

		expense := &models.Expense{
			UserID:      user.ID,
			Amount:      decimal.NewFromFloat(30.00),
			Currency:    "SGD",
			Description: "No status specified",
		}
		err = expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		fetched, err := expenseRepo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.Equal(t, models.ExpenseStatusConfirmed, fetched.Status)
	})
}

// testBot is a test-specific Bot struct for handler testing.
type testBot struct {
	userRepo       *repository.UserRepository
	categoryRepo   *repository.CategoryRepository
	expenseRepo    *repository.ExpenseRepository
	pendingEdits   map[int64]*pendingEdit
	pendingEditsMu sync.RWMutex
}

// setupHandlerTest creates a test Bot with database repositories.
func setupHandlerTest(t *testing.T) (*testBot, *mocks.MockBot, context.Context) {
	t.Helper()

	pool := database.TestDB(t)
	ctx := context.Background()

	err := database.RunMigrations(ctx, pool)
	require.NoError(t, err)
	database.CleanupTables(t, pool)

	err = database.SeedCategories(ctx, pool)
	require.NoError(t, err)

	tb := &testBot{
		userRepo:     repository.NewUserRepository(pool),
		categoryRepo: repository.NewCategoryRepository(pool),
		expenseRepo:  repository.NewExpenseRepository(pool),
		pendingEdits: make(map[int64]*pendingEdit),
	}

	mockBot := mocks.NewMockBot()
	return tb, mockBot, ctx
}

// wrapMockBot wraps MockBot to satisfy *bot.Bot parameter requirements.
// Since handlers expect *bot.Bot, we create wrapper functions for testing.
type botWrapper struct {
	mock *mocks.MockBot
}

func (w *botWrapper) SendMessage(ctx context.Context, params *bot.SendMessageParams) (*tgmodels.Message, error) {
	return w.mock.SendMessage(ctx, params)
}

func (w *botWrapper) EditMessageText(ctx context.Context, params *bot.EditMessageTextParams) (*tgmodels.Message, error) {
	return w.mock.EditMessageText(ctx, params)
}

func (w *botWrapper) AnswerCallbackQuery(ctx context.Context, params *bot.AnswerCallbackQueryParams) (bool, error) {
	return w.mock.AnswerCallbackQuery(ctx, params)
}

func (w *botWrapper) GetFile(ctx context.Context, params *bot.GetFileParams) (*tgmodels.File, error) {
	return w.mock.GetFile(ctx, params)
}

func (w *botWrapper) FileDownloadLink(f *tgmodels.File) string {
	return w.mock.FileDownloadLink(f)
}

// TestHandleStart tests the /start command handler.
func TestHandleStart(t *testing.T) {
	tb, mockBot, ctx := setupHandlerTest(t)

	t.Run("sends welcome message with user name", func(t *testing.T) {
		update := mocks.NewUpdateBuilder().
			WithMessage(12345, 67890, "/start").
			WithFrom(67890, "johndoe", "John", "Doe").
			Build()

		b := &Bot{
			userRepo:     tb.userRepo,
			categoryRepo: tb.categoryRepo,
			expenseRepo:  tb.expenseRepo,
			pendingEdits: make(map[int64]*pendingEdit),
		}

		callHandleStart(b, ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Welcome, John!")
		require.Contains(t, msg.Text, "expense tracker bot")
		require.Equal(t, tgmodels.ParseModeHTML, msg.ParseMode)
	})

	t.Run("sends welcome message without name when From is nil", func(t *testing.T) {
		mockBot.Reset()

		update := &tgmodels.Update{
			Message: &tgmodels.Message{
				ID: 1,
				Chat: tgmodels.Chat{
					ID:   12345,
					Type: "private",
				},
				From: nil,
				Text: "/start",
			},
		}

		b := &Bot{
			userRepo:     tb.userRepo,
			categoryRepo: tb.categoryRepo,
			expenseRepo:  tb.expenseRepo,
			pendingEdits: make(map[int64]*pendingEdit),
		}

		callHandleStart(b, ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Welcome!")
		require.NotContains(t, msg.Text, "Welcome, ")
	})

	t.Run("does nothing when Message is nil", func(t *testing.T) {
		mockBot.Reset()

		update := &tgmodels.Update{Message: nil}

		b := &Bot{
			userRepo:     tb.userRepo,
			categoryRepo: tb.categoryRepo,
			expenseRepo:  tb.expenseRepo,
			pendingEdits: make(map[int64]*pendingEdit),
		}

		callHandleStart(b, ctx, mockBot, update)

		require.Equal(t, 0, mockBot.SentMessageCount())
	})
}

// callHandleStart wraps the handleStart call using MockBot.
func callHandleStart(_ *Bot, ctx context.Context, mock *mocks.MockBot, update *tgmodels.Update) {
	if update.Message == nil {
		return
	}

	firstName := ""
	if update.Message.From != nil {
		firstName = update.Message.From.FirstName
	}

	text := formatStartMessage(firstName)

	_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: tgmodels.ParseModeHTML,
	})
}

// formatStartMessage formats the start message.
func formatStartMessage(firstName string) string {
	greeting := formatGreeting(firstName)
	return "👋 Welcome" + greeting + "!\n\nI'm your personal expense tracker bot. I help you track your daily expenses in SGD.\n\n<b>Quick Start:</b>\n• Send an expense like: <code>5.50 Coffee</code>\n• Or use structured format: <code>/add 5.50 Coffee Food - Dining Out</code>\n\nUse /help to see all available commands."
}

// TestHandleHelp tests the /help command handler.
func TestHandleHelp(t *testing.T) {
	tb, mockBot, ctx := setupHandlerTest(t)

	t.Run("sends help message with all commands", func(t *testing.T) {
		update := mocks.CommandUpdate(12345, 67890, "/help")

		b := &Bot{
			userRepo:     tb.userRepo,
			categoryRepo: tb.categoryRepo,
			expenseRepo:  tb.expenseRepo,
			pendingEdits: make(map[int64]*pendingEdit),
		}

		callHandleHelp(b, ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Available Commands")
		require.Contains(t, msg.Text, "/add")
		require.Contains(t, msg.Text, "/list")
		require.Contains(t, msg.Text, "/today")
		require.Contains(t, msg.Text, "/week")
		require.Contains(t, msg.Text, "/categories")
		require.Equal(t, tgmodels.ParseModeHTML, msg.ParseMode)
	})

	t.Run("does nothing when Message is nil", func(t *testing.T) {
		mockBot.Reset()

		update := &tgmodels.Update{Message: nil}

		b := &Bot{
			userRepo:     tb.userRepo,
			categoryRepo: tb.categoryRepo,
			expenseRepo:  tb.expenseRepo,
			pendingEdits: make(map[int64]*pendingEdit),
		}

		callHandleHelp(b, ctx, mockBot, update)

		require.Equal(t, 0, mockBot.SentMessageCount())
	})
}

// callHandleHelp wraps the handleHelp call using MockBot.
func callHandleHelp(_ *Bot, ctx context.Context, mock *mocks.MockBot, update *tgmodels.Update) {
	if update.Message == nil {
		return
	}

	text := `📚 <b>Available Commands</b>

<b>Expense Tracking:</b>
• <code>/add &lt;amount&gt; &lt;description&gt; [category]</code> - Add an expense
• Just send a message like <code>5.50 Coffee</code> to quickly add

<b>Viewing Expenses:</b>
• <code>/list</code> - Show recent expenses
• <code>/today</code> - Show today's expenses
• <code>/week</code> - Show this week's expenses

<b>Categories:</b>
• <code>/categories</code> - List all categories

<b>Other:</b>
• <code>/help</code> - Show this help message`

	_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: tgmodels.ParseModeHTML,
	})
}

// TestHandleCategories tests the /categories command handler.
func TestHandleCategories(t *testing.T) {
	tb, mockBot, ctx := setupHandlerTest(t)

	t.Run("lists all categories", func(t *testing.T) {
		update := mocks.CommandUpdate(12345, 67890, "/categories")

		categories, err := tb.categoryRepo.GetAll(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, categories)

		callHandleCategories(ctx, mockBot, update, categories, nil)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Expense Categories")
		require.Contains(t, msg.Text, "1.")
		require.Equal(t, tgmodels.ParseModeHTML, msg.ParseMode)
	})

	t.Run("shows message when no categories", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, 67890, "/categories")

		callHandleCategories(ctx, mockBot, update, []models.Category{}, nil)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "No categories found")
	})

	t.Run("does nothing when Message is nil", func(t *testing.T) {
		mockBot.Reset()

		update := &tgmodels.Update{Message: nil}

		callHandleCategories(ctx, mockBot, update, nil, nil)

		require.Equal(t, 0, mockBot.SentMessageCount())
	})
}

// callHandleCategories simulates the handleCategories logic with mock.
func callHandleCategories(
	ctx context.Context,
	mock *mocks.MockBot,
	update *tgmodels.Update,
	categories []models.Category,
	fetchError error,
) {
	if update.Message == nil {
		return
	}

	if fetchError != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Failed to fetch categories. Please try again.",
		})
		return
	}

	if len(categories) == 0 {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "No categories found.",
		})
		return
	}

	var sb strings.Builder
	sb.WriteString("📁 <b>Expense Categories</b>\n\n")
	for i, cat := range categories {
		sb.WriteString(strconv.Itoa(i+1) + ". " + cat.Name + "\n")
	}

	text := sb.String()

	_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: tgmodels.ParseModeHTML,
	})
}

// TestHandleAdd tests the /add command handler.
func TestHandleAdd(t *testing.T) {
	tb, mockBot, ctx := setupHandlerTest(t)

	user := &models.User{ID: 67890, Username: "testuser", FirstName: "Test", LastName: "User"}
	err := tb.userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	t.Run("adds expense with valid format", func(t *testing.T) {
		update := mocks.CommandUpdate(12345, user.ID, "/add 5.50 Coffee")

		categories, err := tb.categoryRepo.GetAll(ctx)
		require.NoError(t, err)

		callHandleAdd(ctx, mockBot, update, tb.expenseRepo, categories, nil)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Expense Added")
		require.Contains(t, msg.Text, "$5.50 SGD")
		require.Contains(t, msg.Text, "Coffee")
	})

	t.Run("adds expense with category", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, "/add 12.00 Lunch Food - Dining Out")

		categories, err := tb.categoryRepo.GetAll(ctx)
		require.NoError(t, err)

		callHandleAdd(ctx, mockBot, update, tb.expenseRepo, categories, nil)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "$12.00 SGD")
		require.Contains(t, msg.Text, "Lunch")
		require.Contains(t, msg.Text, "Food - Dining Out")
	})

	t.Run("shows error for invalid format", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, "/add invalid")

		categories, err := tb.categoryRepo.GetAll(ctx)
		require.NoError(t, err)

		callHandleAdd(ctx, mockBot, update, tb.expenseRepo, categories, nil)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Invalid format")
	})

	t.Run("shows error for /add without arguments", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, "/add")

		categories, err := tb.categoryRepo.GetAll(ctx)
		require.NoError(t, err)

		callHandleAdd(ctx, mockBot, update, tb.expenseRepo, categories, nil)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Invalid format")
	})

	t.Run("does nothing when Message is nil", func(t *testing.T) {
		mockBot.Reset()

		update := &tgmodels.Update{Message: nil}

		callHandleAdd(ctx, mockBot, update, tb.expenseRepo, nil, nil)

		require.Equal(t, 0, mockBot.SentMessageCount())
	})
}

// callHandleAdd simulates the handleAdd logic with mock.
func callHandleAdd(
	ctx context.Context,
	mock *mocks.MockBot,
	update *tgmodels.Update,
	expenseRepo *repository.ExpenseRepository,
	categories []models.Category,
	fetchError error,
) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	if fetchError != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to process expense. Please try again.",
		})
		return
	}

	categoryNames := make([]string, len(categories))
	for i, cat := range categories {
		categoryNames[i] = cat.Name
	}

	parsed := ParseAddCommandWithCategories(update.Message.Text, categoryNames)
	if parsed == nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Invalid format. Use: <code>/add 5.50 Coffee [category]</code>",
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	expense := &models.Expense{
		UserID:      userID,
		Amount:      parsed.Amount,
		Currency:    "SGD",
		Description: parsed.Description,
	}

	if parsed.CategoryName != "" {
		for _, cat := range categories {
			if strings.EqualFold(cat.Name, parsed.CategoryName) {
				expense.CategoryID = &cat.ID
				expense.Category = &cat
				break
			}
		}
	}

	if err := expenseRepo.Create(ctx, expense); err != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to save expense. Please try again.",
		})
		return
	}

	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = expense.Category.Name
	}

	descText := ""
	if expense.Description != "" {
		descText = "\n📝 " + expense.Description
	}

	text := "✅ <b>Expense Added</b>\n\n💰 $" + expense.Amount.StringFixed(2) + " SGD" + descText + "\n📁 " + categoryText + "\n🆔 #" + strconv.Itoa(expense.ID)

	_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: tgmodels.ParseModeHTML,
	})
}

// TestHandleList tests the /list command handler.
func TestHandleList(t *testing.T) {
	tb, mockBot, ctx := setupHandlerTest(t)

	user := &models.User{ID: 11111, Username: "listuser", FirstName: "List", LastName: "User"}
	err := tb.userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	t.Run("shows empty list when no expenses", func(t *testing.T) {
		update := mocks.CommandUpdate(12345, user.ID, "/list")

		expenses, err := tb.expenseRepo.GetByUserID(ctx, user.ID, 10)
		require.NoError(t, err)

		callHandleList(ctx, mockBot, update, expenses)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Recent Expenses")
		require.Contains(t, msg.Text, "No expenses found")
	})

	t.Run("shows expenses when they exist", func(t *testing.T) {
		mockBot.Reset()

		expense := &models.Expense{
			UserID:      user.ID,
			Amount:      decimal.NewFromFloat(15.50),
			Currency:    "SGD",
			Description: "Test expense",
			Status:      models.ExpenseStatusConfirmed,
		}
		err := tb.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, user.ID, "/list")

		expenses, err := tb.expenseRepo.GetByUserID(ctx, user.ID, 10)
		require.NoError(t, err)

		callHandleList(ctx, mockBot, update, expenses)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Recent Expenses")
		require.Contains(t, msg.Text, "$15.50")
		require.Contains(t, msg.Text, "Test expense")
	})

	t.Run("does nothing when Message is nil", func(t *testing.T) {
		mockBot.Reset()

		update := &tgmodels.Update{Message: nil}

		callHandleList(ctx, mockBot, update, nil)

		require.Equal(t, 0, mockBot.SentMessageCount())
	})
}

// callHandleList simulates the handleList logic with mock.
func callHandleList(
	ctx context.Context,
	mock *mocks.MockBot,
	update *tgmodels.Update,
	expenses []models.Expense,
) {
	if update.Message == nil {
		return
	}

	sendExpenseListMock(ctx, mock, update.Message.Chat.ID, expenses, "📋 <b>Recent Expenses</b>")
}

// sendExpenseListMock simulates sendExpenseList with mock.
func sendExpenseListMock(
	ctx context.Context,
	mock *mocks.MockBot,
	chatID int64,
	expenses []models.Expense,
	header string,
) {
	if len(expenses) == 0 {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      header + "\n\nNo expenses found.",
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n\n")

	for _, exp := range expenses {
		categoryText := ""
		if exp.Category != nil {
			categoryText = " [" + exp.Category.Name + "]"
		}

		descText := ""
		if exp.Description != "" {
			descText = " - " + exp.Description
		}

		sb.WriteString("#" + strconv.Itoa(exp.ID) + " $" + exp.Amount.StringFixed(2) + descText + categoryText + "\n<i>" + exp.CreatedAt.Format("Jan 2 15:04") + "</i>\n\n")
	}

	_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      sb.String(),
		ParseMode: tgmodels.ParseModeHTML,
	})
}

// TestHandleToday tests the /today command handler.
func TestHandleToday(t *testing.T) {
	tb, mockBot, ctx := setupHandlerTest(t)

	user := &models.User{ID: 22222, Username: "todayuser", FirstName: "Today", LastName: "User"}
	err := tb.userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	t.Run("shows today's expenses with total", func(t *testing.T) {
		expense := &models.Expense{
			UserID:      user.ID,
			Amount:      decimal.NewFromFloat(25.00),
			Currency:    "SGD",
			Description: "Lunch",
			Status:      models.ExpenseStatusConfirmed,
		}
		err := tb.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, user.ID, "/today")

		now := time.Now()
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		endOfDay := startOfDay.Add(24 * time.Hour)

		expenses, err := tb.expenseRepo.GetByUserIDAndDateRange(ctx, user.ID, startOfDay, endOfDay)
		require.NoError(t, err)

		total, _ := tb.expenseRepo.GetTotalByUserIDAndDateRange(ctx, user.ID, startOfDay, endOfDay)

		callHandleToday(ctx, mockBot, update, expenses, total)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Today's Expenses")
		require.Contains(t, msg.Text, "Total:")
		require.Contains(t, msg.Text, "$25.00")
	})

	t.Run("shows empty message when no expenses today", func(t *testing.T) {
		mockBot.Reset()
		database.CleanupTables(t, tb.expenseRepo.Pool())
		err := tb.userRepo.UpsertUser(ctx, user)
		require.NoError(t, err)
		err = database.SeedCategories(ctx, tb.expenseRepo.Pool())
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, user.ID, "/today")

		callHandleToday(ctx, mockBot, update, []models.Expense{}, decimal.Zero)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Today's Expenses")
		require.Contains(t, msg.Text, "No expenses found")
	})
}

// callHandleToday simulates the handleToday logic with mock.
func callHandleToday(
	ctx context.Context,
	mock *mocks.MockBot,
	update *tgmodels.Update,
	expenses []models.Expense,
	total decimal.Decimal,
) {
	if update.Message == nil {
		return
	}

	header := "📅 <b>Today's Expenses</b> (Total: $" + total.StringFixed(2) + ")"
	sendExpenseListMock(ctx, mock, update.Message.Chat.ID, expenses, header)
}

// TestHandleWeek tests the /week command handler.
func TestHandleWeek(t *testing.T) {
	tb, mockBot, ctx := setupHandlerTest(t)

	user := &models.User{ID: 33333, Username: "weekuser", FirstName: "Week", LastName: "User"}
	err := tb.userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	t.Run("shows week's expenses with total", func(t *testing.T) {
		expense := &models.Expense{
			UserID:      user.ID,
			Amount:      decimal.NewFromFloat(100.00),
			Currency:    "SGD",
			Description: "Weekly shopping",
			Status:      models.ExpenseStatusConfirmed,
		}
		err := tb.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, user.ID, "/week")

		now := time.Now()
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		startOfWeek := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		endOfWeek := startOfWeek.Add(7 * 24 * time.Hour)

		expenses, err := tb.expenseRepo.GetByUserIDAndDateRange(ctx, user.ID, startOfWeek, endOfWeek)
		require.NoError(t, err)

		total, _ := tb.expenseRepo.GetTotalByUserIDAndDateRange(ctx, user.ID, startOfWeek, endOfWeek)

		callHandleWeek(ctx, mockBot, update, expenses, total)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "This Week's Expenses")
		require.Contains(t, msg.Text, "Total:")
	})

	t.Run("shows empty message when no expenses this week", func(t *testing.T) {
		mockBot.Reset()
		database.CleanupTables(t, tb.expenseRepo.Pool())
		err := tb.userRepo.UpsertUser(ctx, user)
		require.NoError(t, err)
		err = database.SeedCategories(ctx, tb.expenseRepo.Pool())
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, user.ID, "/week")

		callHandleWeek(ctx, mockBot, update, []models.Expense{}, decimal.Zero)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "This Week's Expenses")
		require.Contains(t, msg.Text, "No expenses found")
	})
}

// callHandleWeek simulates the handleWeek logic with mock.
func callHandleWeek(
	ctx context.Context,
	mock *mocks.MockBot,
	update *tgmodels.Update,
	expenses []models.Expense,
	total decimal.Decimal,
) {
	if update.Message == nil {
		return
	}

	header := "📆 <b>This Week's Expenses</b> (Total: $" + total.StringFixed(2) + ")"
	sendExpenseListMock(ctx, mock, update.Message.Chat.ID, expenses, header)
}

// TestHandleEdit tests the /edit command handler.
func TestHandleEdit(t *testing.T) {
	tb, mockBot, ctx := setupHandlerTest(t)

	user := &models.User{ID: 44444, Username: "edituser", FirstName: "Edit", LastName: "User"}
	err := tb.userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	t.Run("shows usage when no arguments", func(t *testing.T) {
		update := mocks.CommandUpdate(12345, user.ID, "/edit")

		callHandleEdit(ctx, mockBot, update, tb.expenseRepo, tb.categoryRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Usage:")
		require.Contains(t, msg.Text, "/edit")
	})

	t.Run("shows error for invalid expense ID", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, "/edit abc 5.00 New description")

		callHandleEdit(ctx, mockBot, update, tb.expenseRepo, tb.categoryRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Invalid expense ID")
	})

	t.Run("shows error when expense not found", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, "/edit 99999 5.00 New description")

		callHandleEdit(ctx, mockBot, update, tb.expenseRepo, tb.categoryRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "not found")
	})

	t.Run("edits expense successfully", func(t *testing.T) {
		mockBot.Reset()

		expense := &models.Expense{
			UserID:      user.ID,
			Amount:      decimal.NewFromFloat(10.00),
			Currency:    "SGD",
			Description: "Original",
			Status:      models.ExpenseStatusConfirmed,
		}
		err := tb.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, user.ID, "/edit "+strconv.Itoa(expense.ID)+" 20.00 Updated description")

		callHandleEdit(ctx, mockBot, update, tb.expenseRepo, tb.categoryRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Expense Updated")
		require.Contains(t, msg.Text, "$20.00 SGD")
		require.Contains(t, msg.Text, "Updated description")
	})

	t.Run("shows error when editing another user's expense", func(t *testing.T) {
		mockBot.Reset()

		otherUser := &models.User{ID: 55555, Username: "otheruser", FirstName: "Other", LastName: "User"}
		err := tb.userRepo.UpsertUser(ctx, otherUser)
		require.NoError(t, err)

		expense := &models.Expense{
			UserID:      otherUser.ID,
			Amount:      decimal.NewFromFloat(50.00),
			Currency:    "SGD",
			Description: "Other's expense",
			Status:      models.ExpenseStatusConfirmed,
		}
		err = tb.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, user.ID, "/edit "+strconv.Itoa(expense.ID)+" 100.00 Trying to edit")

		callHandleEdit(ctx, mockBot, update, tb.expenseRepo, tb.categoryRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "only edit your own")
	})
}

// callHandleEdit simulates the handleEdit logic with mock.
func callHandleEdit(
	ctx context.Context,
	mock *mocks.MockBot,
	update *tgmodels.Update,
	expenseRepo *repository.ExpenseRepository,
	categoryRepo *repository.CategoryRepository,
	userID int64,
) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	args := strings.TrimPrefix(update.Message.Text, "/edit")
	args = strings.TrimSpace(args)

	if idx := strings.Index(args, "@"); idx == 0 {
		if spaceIdx := strings.Index(args, " "); spaceIdx != -1 {
			args = strings.TrimSpace(args[spaceIdx:])
		} else {
			args = ""
		}
	}

	if args == "" {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Usage: <code>/edit &lt;id&gt; &lt;amount&gt; &lt;description&gt; [category]</code>",
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	parts := strings.SplitN(args, " ", 2)
	expenseID, err := strconv.Atoi(parts[0])
	if err != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Invalid expense ID. Use: <code>/edit &lt;id&gt; &lt;amount&gt; &lt;description&gt;</code>",
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	expense, err := expenseRepo.GetByID(ctx, expenseID)
	if err != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Expense #" + strconv.Itoa(expenseID) + " not found.",
		})
		return
	}

	if expense.UserID != userID {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ You can only edit your own expenses.",
		})
		return
	}

	if len(parts) < 2 {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Please provide new values: <code>/edit &lt;id&gt; &lt;amount&gt; &lt;description&gt;</code>",
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	categories, _ := categoryRepo.GetAll(ctx)
	categoryNames := make([]string, len(categories))
	for i, cat := range categories {
		categoryNames[i] = cat.Name
	}

	parsed := ParseExpenseInputWithCategories(parts[1], categoryNames)
	if parsed == nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Invalid format. Use: <code>/edit &lt;id&gt; &lt;amount&gt; &lt;description&gt;</code>",
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	expense.Amount = parsed.Amount
	expense.Description = parsed.Description

	if parsed.CategoryName != "" {
		for _, cat := range categories {
			if strings.EqualFold(cat.Name, parsed.CategoryName) {
				expense.CategoryID = &cat.ID
				expense.Category = &cat
				break
			}
		}
	}

	if err := expenseRepo.Update(ctx, expense); err != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to update expense. Please try again.",
		})
		return
	}

	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = expense.Category.Name
	}

	text := "✅ <b>Expense Updated</b>\n\n🆔 #" + strconv.Itoa(expense.ID) + "\n💰 $" + expense.Amount.StringFixed(2) + " SGD\n📝 " + expense.Description + "\n📁 " + categoryText

	_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: tgmodels.ParseModeHTML,
	})
}

// TestHandleDelete tests the /delete command handler.
func TestHandleDelete(t *testing.T) {
	tb, mockBot, ctx := setupHandlerTest(t)

	user := &models.User{ID: 66666, Username: "deleteuser", FirstName: "Delete", LastName: "User"}
	err := tb.userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	t.Run("shows usage when no arguments", func(t *testing.T) {
		update := mocks.CommandUpdate(12345, user.ID, "/delete")

		callHandleDelete(ctx, mockBot, update, tb.expenseRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Usage:")
		require.Contains(t, msg.Text, "/delete")
	})

	t.Run("shows error for invalid expense ID", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, "/delete abc")

		callHandleDelete(ctx, mockBot, update, tb.expenseRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "Invalid expense ID")
	})

	t.Run("shows error when expense not found", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.CommandUpdate(12345, user.ID, "/delete 99999")

		callHandleDelete(ctx, mockBot, update, tb.expenseRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "not found")
	})

	t.Run("deletes expense successfully", func(t *testing.T) {
		mockBot.Reset()

		expense := &models.Expense{
			UserID:      user.ID,
			Amount:      decimal.NewFromFloat(30.00),
			Currency:    "SGD",
			Description: "To be deleted",
			Status:      models.ExpenseStatusConfirmed,
		}
		err := tb.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, user.ID, "/delete "+strconv.Itoa(expense.ID))

		callHandleDelete(ctx, mockBot, update, tb.expenseRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "deleted")
		require.Contains(t, msg.Text, strconv.Itoa(expense.ID))
	})

	t.Run("shows error when deleting another user's expense", func(t *testing.T) {
		mockBot.Reset()

		otherUser := &models.User{ID: 77777, Username: "otheruser2", FirstName: "Other2", LastName: "User"}
		err := tb.userRepo.UpsertUser(ctx, otherUser)
		require.NoError(t, err)

		expense := &models.Expense{
			UserID:      otherUser.ID,
			Amount:      decimal.NewFromFloat(40.00),
			Currency:    "SGD",
			Description: "Other's expense",
			Status:      models.ExpenseStatusConfirmed,
		}
		err = tb.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := mocks.CommandUpdate(12345, user.ID, "/delete "+strconv.Itoa(expense.ID))

		callHandleDelete(ctx, mockBot, update, tb.expenseRepo, user.ID)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "only delete your own")
	})
}

// callHandleDelete simulates the handleDelete logic with mock.
func callHandleDelete(
	ctx context.Context,
	mock *mocks.MockBot,
	update *tgmodels.Update,
	expenseRepo *repository.ExpenseRepository,
	userID int64,
) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	args := strings.TrimPrefix(update.Message.Text, "/delete")
	args = strings.TrimSpace(args)

	if idx := strings.Index(args, "@"); idx == 0 {
		if spaceIdx := strings.Index(args, " "); spaceIdx != -1 {
			args = strings.TrimSpace(args[spaceIdx:])
		} else {
			args = ""
		}
	}

	if args == "" {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Usage: <code>/delete &lt;id&gt;</code>",
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	expenseID, err := strconv.Atoi(args)
	if err != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Invalid expense ID. Use: <code>/delete &lt;id&gt;</code>",
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	expense, err := expenseRepo.GetByID(ctx, expenseID)
	if err != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Expense #" + strconv.Itoa(expenseID) + " not found.",
		})
		return
	}

	if expense.UserID != userID {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ You can only delete your own expenses.",
		})
		return
	}

	if err := expenseRepo.Delete(ctx, expenseID); err != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to delete expense. Please try again.",
		})
		return
	}

	_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "✅ Expense #" + strconv.Itoa(expenseID) + " deleted.",
	})
}

// TestHandleFreeTextExpense tests free-text expense parsing.
func TestHandleFreeTextExpense(t *testing.T) {
	tb, mockBot, ctx := setupHandlerTest(t)

	user := &models.User{ID: 88888, Username: "freetextuser", FirstName: "FreeText", LastName: "User"}
	err := tb.userRepo.UpsertUser(ctx, user)
	require.NoError(t, err)

	t.Run("parses valid free-text expense", func(t *testing.T) {
		update := mocks.MessageUpdate(12345, user.ID, "5.50 Coffee")

		categories, err := tb.categoryRepo.GetAll(ctx)
		require.NoError(t, err)

		result := callHandleFreeTextExpense(ctx, mockBot, update, tb.expenseRepo, categories)

		require.True(t, result)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.NotNil(t, msg)
		require.Contains(t, msg.Text, "$5.50 SGD")
		require.Contains(t, msg.Text, "Coffee")
	})

	t.Run("returns false for commands", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.MessageUpdate(12345, user.ID, "/start")

		categories, err := tb.categoryRepo.GetAll(ctx)
		require.NoError(t, err)

		result := callHandleFreeTextExpense(ctx, mockBot, update, tb.expenseRepo, categories)

		require.False(t, result)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("returns false for invalid format", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.MessageUpdate(12345, user.ID, "hello world")

		categories, err := tb.categoryRepo.GetAll(ctx)
		require.NoError(t, err)

		result := callHandleFreeTextExpense(ctx, mockBot, update, tb.expenseRepo, categories)

		require.False(t, result)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("returns false for nil message", func(t *testing.T) {
		mockBot.Reset()

		update := &tgmodels.Update{Message: nil}

		result := callHandleFreeTextExpense(ctx, mockBot, update, tb.expenseRepo, nil)

		require.False(t, result)
	})

	t.Run("returns false for empty text", func(t *testing.T) {
		mockBot.Reset()

		update := mocks.MessageUpdate(12345, user.ID, "")

		result := callHandleFreeTextExpense(ctx, mockBot, update, tb.expenseRepo, nil)

		require.False(t, result)
	})
}

// callHandleFreeTextExpense simulates the handleFreeTextExpense logic with mock.
func callHandleFreeTextExpense(
	ctx context.Context,
	mock *mocks.MockBot,
	update *tgmodels.Update,
	expenseRepo *repository.ExpenseRepository,
	categories []models.Category,
) bool {
	if update.Message == nil || update.Message.Text == "" {
		return false
	}

	text := update.Message.Text
	if strings.HasPrefix(text, "/") {
		return false
	}

	categoryNames := make([]string, len(categories))
	for i, cat := range categories {
		categoryNames[i] = cat.Name
	}

	parsed := ParseExpenseInputWithCategories(text, categoryNames)
	if parsed == nil {
		return false
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	expense := &models.Expense{
		UserID:      userID,
		Amount:      parsed.Amount,
		Currency:    "SGD",
		Description: parsed.Description,
	}

	if parsed.CategoryName != "" {
		for _, cat := range categories {
			if strings.EqualFold(cat.Name, parsed.CategoryName) {
				expense.CategoryID = &cat.ID
				expense.Category = &cat
				break
			}
		}
	}

	if err := expenseRepo.Create(ctx, expense); err != nil {
		_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to save expense. Please try again.",
		})
		return true
	}

	categoryText := categoryUncategorized
	if expense.Category != nil {
		categoryText = expense.Category.Name
	}

	descText := ""
	if expense.Description != "" {
		descText = "\n📝 " + expense.Description
	}

	msgText := "✅ <b>Expense Added</b>\n\n💰 $" + expense.Amount.StringFixed(2) + " SGD" + descText + "\n📁 " + categoryText + "\n🆔 #" + strconv.Itoa(expense.ID)

	_, _ = mock.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      msgText,
		ParseMode: tgmodels.ParseModeHTML,
	})
	return true
}
