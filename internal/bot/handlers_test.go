package bot

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
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

	err = database.SeedCategories(ctx, pool)
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
