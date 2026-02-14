package bot

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/exchange"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

type mockExchangeService struct {
	result exchange.ConversionResult
	err    error
	calls  int
}

func (m *mockExchangeService) Convert(
	_ context.Context,
	_ decimal.Decimal,
	_, _ string,
) (exchange.ConversionResult, error) {
	m.calls++
	if m.err != nil {
		return exchange.ConversionResult{}, m.err
	}
	return m.result, nil
}

func TestConvertExpenseCurrency(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(910001)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:              userID,
		Username:        "convertuser",
		FirstName:       "Convert",
		DefaultCurrency: "SGD",
	})
	require.NoError(t, err)

	t.Run("same currency skips conversion", func(t *testing.T) {
		mockSvc := &mockExchangeService{}
		b.exchangeService = mockSvc

		amount, currency, description := b.convertExpenseCurrency(
			ctx,
			userID,
			decimal.RequireFromString("18"),
			"SGD",
			"Lunch",
		)
		require.Equal(t, decimal.RequireFromString("18"), amount)
		require.Equal(t, "SGD", currency)
		require.Equal(t, "Lunch", description)
		require.Equal(t, 0, mockSvc.calls)
	})

	t.Run("different currency converts and appends metadata", func(t *testing.T) {
		mockSvc := &mockExchangeService{
			result: exchange.ConversionResult{
				Amount:   decimal.RequireFromString("24.30"),
				Rate:     decimal.RequireFromString("1.35"),
				RateDate: time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
			},
		}
		b.exchangeService = mockSvc

		amount, currency, description := b.convertExpenseCurrency(
			ctx,
			userID,
			decimal.RequireFromString("18"),
			"USD",
			"Valentine roses",
		)
		require.Equal(t, decimal.RequireFromString("24.30"), amount)
		require.Equal(t, "SGD", currency)
		require.Contains(t, description, "Valentine roses")
		require.Contains(t, description, "[orig: 18.00 USD -> 24.30 SGD @ 1.3500 (2026-02-14)]")
		require.Equal(t, 1, mockSvc.calls)
	})

	t.Run("conversion failure falls back to original currency", func(t *testing.T) {
		mockSvc := &mockExchangeService{err: errors.New("rate unavailable")}
		b.exchangeService = mockSvc

		amount, currency, description := b.convertExpenseCurrency(
			ctx,
			userID,
			decimal.RequireFromString("18"),
			"USD",
			"Valentine roses",
		)
		require.Equal(t, decimal.RequireFromString("18"), amount)
		require.Equal(t, "USD", currency)
		require.Contains(t, description, "[fx_unavailable: kept USD, target SGD]")
		require.Equal(t, 1, mockSvc.calls)
	})
}

func TestSaveExpenseCore_ConvertsCurrencyToDefault(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(910002)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:              userID,
		Username:        "convertsaveuser",
		FirstName:       "Convert Save",
		DefaultCurrency: "SGD",
	})
	require.NoError(t, err)

	b.exchangeService = &mockExchangeService{
		result: exchange.ConversionResult{
			Amount:   decimal.RequireFromString("24.30"),
			Rate:     decimal.RequireFromString("1.35"),
			RateDate: time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
		},
	}

	parsed := &ParsedExpense{
		Amount:      decimal.RequireFromString("18"),
		Currency:    "USD",
		Description: "Valentine roses",
	}

	mockBot := mocks.NewMockBot()
	b.saveExpenseCore(ctx, mockBot, 12345, userID, parsed, nil)

	require.Equal(t, 1, mockBot.SentMessageCount())
	msg := mockBot.LastSentMessage()
	require.Contains(t, msg.Text, "S$24.30 SGD")
	require.Contains(t, msg.Text, "[orig: 18.00 USD -&gt; 24.30 SGD @ 1.3500 (2026-02-14)]")

	expenses, err := b.expenseRepo.GetByUserID(ctx, userID, 1)
	require.NoError(t, err)
	require.Len(t, expenses, 1)
	require.Equal(t, "SGD", expenses[0].Currency)
	require.True(t, decimal.RequireFromString("24.30").Equal(expenses[0].Amount))
	require.Contains(t, expenses[0].Description, "[orig: 18.00 USD -> 24.30 SGD @ 1.3500 (2026-02-14)]")
	require.Equal(t, "Valentine roses", expenses[0].Merchant)
}

func TestSaveExpenseCore_ExchangeOutageDoesNotBlockSave(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(910003)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:              userID,
		Username:        "fxfallbackuser",
		FirstName:       "FX Fallback",
		DefaultCurrency: "SGD",
	})
	require.NoError(t, err)

	b.exchangeService = &mockExchangeService{err: errors.New("service down")}

	parsed := &ParsedExpense{
		Amount:      decimal.RequireFromString("18"),
		Currency:    "USD",
		Description: "Valentine roses",
	}

	mockBot := mocks.NewMockBot()
	b.saveExpenseCore(ctx, mockBot, 12345, userID, parsed, nil)

	require.Equal(t, 1, mockBot.SentMessageCount())
	msg := mockBot.LastSentMessage()
	require.Contains(t, msg.Text, "$18.00 USD")
	require.Contains(t, msg.Text, "[fx_unavailable: kept USD, target SGD]")

	expenses, err := b.expenseRepo.GetByUserID(ctx, userID, 1)
	require.NoError(t, err)
	require.Len(t, expenses, 1)
	require.Equal(t, "USD", expenses[0].Currency)
	require.True(t, decimal.RequireFromString("18").Equal(expenses[0].Amount))
	require.Contains(t, expenses[0].Description, "[fx_unavailable: kept USD, target SGD]")
	require.Equal(t, "Valentine roses", expenses[0].Merchant)
}
