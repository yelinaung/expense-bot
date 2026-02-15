package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
	"gitlab.com/yelinaung/expense-bot/internal/logger"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

func normalizeCurrencyCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func getCurrencyOrCodeSymbol(code string) string {
	symbol := appmodels.SupportedCurrencies[code]
	if symbol == "" {
		return code
	}
	return symbol
}

func appendOriginalAmountDescription(
	description string,
	originalAmount decimal.Decimal,
	originalCurrency string,
	convertedAmount decimal.Decimal,
	convertedCurrency string,
	rate decimal.Decimal,
	rateDate string,
) string {
	metadata := fmt.Sprintf(
		"[orig: %s %s -> %s %s @ %s (%s)]",
		originalAmount.StringFixed(2),
		originalCurrency,
		convertedAmount.StringFixed(2),
		convertedCurrency,
		rate.StringFixed(4),
		rateDate,
	)
	if strings.TrimSpace(description) == "" {
		return metadata
	}
	return description + " " + metadata
}

func appendConversionUnavailableDescription(
	description, originalCurrency, targetCurrency string,
) string {
	metadata := fmt.Sprintf("[fx_unavailable: kept %s, target %s]", originalCurrency, targetCurrency)
	if strings.TrimSpace(description) == "" {
		return metadata
	}
	return description + " " + metadata
}

func (b *Bot) getUserDefaultCurrency(ctx context.Context, userID int64) string {
	currency, err := b.userRepo.GetDefaultCurrency(ctx, userID)
	if err != nil {
		logger.Log.Debug().
			Err(err).
			Str("user_hash", logger.HashUserID(userID)).
			Msg("Failed to get default currency, using SGD")
		return appmodels.DefaultCurrency
	}

	currency = normalizeCurrencyCode(currency)
	if _, ok := appmodels.SupportedCurrencies[currency]; !ok {
		return appmodels.DefaultCurrency
	}
	return currency
}

func (b *Bot) convertExpenseCurrency(
	ctx context.Context,
	userID int64,
	amount decimal.Decimal,
	sourceCurrency string,
	description string,
) (convertedAmount decimal.Decimal, finalCurrency string, finalDescription string) {
	defaultCurrency := b.getUserDefaultCurrency(ctx, userID)
	source := normalizeCurrencyCode(sourceCurrency)
	if source == "" {
		source = defaultCurrency
	}
	if _, ok := appmodels.SupportedCurrencies[source]; !ok {
		logger.Log.Warn().
			Str("source_currency", source).
			Str("user_hash", logger.HashUserID(userID)).
			Msg("Unsupported currency from input/LLM; using default currency")
		source = defaultCurrency
	}
	if source == defaultCurrency {
		return amount, defaultCurrency, description
	}
	if b.exchangeService == nil {
		logger.Log.Warn().
			Str("source_currency", source).
			Str("target_currency", defaultCurrency).
			Str("user_hash", logger.HashUserID(userID)).
			Msg("Exchange service unavailable; saving original currency")
		return amount, source, appendConversionUnavailableDescription(description, source, defaultCurrency)
	}

	result, err := b.exchangeService.Convert(ctx, amount, source, defaultCurrency)
	if err != nil {
		logger.Log.Warn().
			Err(err).
			Str("source_currency", source).
			Str("target_currency", defaultCurrency).
			Str("user_hash", logger.HashUserID(userID)).
			Msg("Exchange lookup failed; saving original currency")
		return amount, source, appendConversionUnavailableDescription(description, source, defaultCurrency)
	}

	finalDescription = appendOriginalAmountDescription(
		description,
		amount,
		source,
		result.Amount,
		defaultCurrency,
		result.Rate,
		result.RateDate.Format("2006-01-02"),
	)
	return result.Amount, defaultCurrency, finalDescription
}
