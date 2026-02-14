package exchange

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// ConversionResult contains converted amount details.
type ConversionResult struct {
	Amount   decimal.Decimal
	Rate     decimal.Decimal
	RateDate time.Time
}

// Service converts amounts between currencies.
type Service interface {
	Convert(ctx context.Context, amount decimal.Decimal, fromCurrency, toCurrency string) (ConversionResult, error)
}
