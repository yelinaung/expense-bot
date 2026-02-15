package exchange

import (
	"context"
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

var errInvalidNonPositiveRate = errors.New("invalid non-positive rate")

// ConversionResult contains converted amount details.
type ConversionResult struct {
	Amount   decimal.Decimal
	Rate     decimal.Decimal
	RateDate time.Time
}

// Converter converts amounts between currencies.
type Converter interface {
	Convert(ctx context.Context, amount decimal.Decimal, fromCurrency, toCurrency string) (ConversionResult, error)
}

func validateConversionRate(rate decimal.Decimal) error {
	if !rate.IsPositive() {
		return errInvalidNonPositiveRate
	}
	return nil
}
