package exchange

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

type cachedRateEntry struct {
	Rate      decimal.Decimal
	RateDate  time.Time
	ExpiresAt time.Time
}

// CachedService wraps an exchange Converter with in-memory TTL caching.
// Cache entries are keyed by normalized "FROM->TO" currency pair.
type CachedService struct {
	inner Converter
	ttl   time.Duration

	mu    sync.RWMutex
	rates map[string]cachedRateEntry
}

// NewCachedService returns a converter that caches exchange rates in memory.
func NewCachedService(inner Converter, ttl time.Duration) *CachedService {
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	return &CachedService{
		inner: inner,
		ttl:   ttl,
		rates: make(map[string]cachedRateEntry),
	}
}

func normalizePair(fromCurrency, toCurrency string) string {
	from := strings.ToUpper(strings.TrimSpace(fromCurrency))
	to := strings.ToUpper(strings.TrimSpace(toCurrency))
	return from + "->" + to
}

// Convert returns converted amount using cached rate when available.
func (s *CachedService) Convert(
	ctx context.Context,
	amount decimal.Decimal,
	fromCurrency, toCurrency string,
) (ConversionResult, error) {
	if s.inner == nil {
		return ConversionResult{}, errors.New("inner exchange service is required")
	}

	key := normalizePair(fromCurrency, toCurrency)
	now := time.Now()

	s.mu.RLock()
	entry, ok := s.rates[key]
	s.mu.RUnlock()
	if ok && now.Before(entry.ExpiresAt) {
		return ConversionResult{
			Amount:   amount.Mul(entry.Rate).Round(2),
			Rate:     entry.Rate,
			RateDate: entry.RateDate,
		}, nil
	}

	result, err := s.inner.Convert(ctx, amount, fromCurrency, toCurrency)
	if err != nil {
		return ConversionResult{}, err
	}
	if !result.Rate.IsPositive() {
		return ConversionResult{}, errors.New("invalid non-positive rate")
	}

	fetchedAt := time.Now()
	s.mu.Lock()
	s.rates[key] = cachedRateEntry{
		Rate:      result.Rate,
		RateDate:  result.RateDate,
		ExpiresAt: fetchedAt.Add(s.ttl),
	}
	s.mu.Unlock()

	return result, nil
}
