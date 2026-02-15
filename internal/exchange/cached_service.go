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

type inFlightCall struct {
	done   chan struct{}
	result ConversionResult
	err    error
}

const maxCleanupInterval = 5 * time.Minute

// CachedService wraps an exchange Converter with in-memory TTL caching.
// Cache entries are keyed by normalized "FROM->TO" currency pair.
type CachedService struct {
	inner Converter
	ttl   time.Duration

	mu          sync.RWMutex
	rates       map[string]cachedRateEntry
	inFlight    map[string]*inFlightCall
	lastCleanup time.Time
}

// NewCachedService returns a converter that caches exchange rates in memory.
func NewCachedService(inner Converter, ttl time.Duration) *CachedService {
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	return &CachedService{
		inner:    inner,
		ttl:      ttl,
		rates:    make(map[string]cachedRateEntry),
		inFlight: make(map[string]*inFlightCall),
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
		return applyCachedRate(amount, entry), nil
	}

	s.mu.Lock()
	// Re-check under write lock in case another goroutine refreshed it.
	entry, ok = s.rates[key]
	if ok && now.Before(entry.ExpiresAt) {
		s.mu.Unlock()
		return applyCachedRate(amount, entry), nil
	}
	if ok && !now.Before(entry.ExpiresAt) {
		delete(s.rates, key)
	}

	if call, waiting := s.inFlight[key]; waiting {
		s.mu.Unlock()
		return waitForInFlight(ctx, amount, call)
	}

	call := &inFlightCall{done: make(chan struct{})}
	s.inFlight[key] = call
	s.mu.Unlock()

	// Run refresh with cancellation detached from a single caller so one
	// short/deadline-bound caller cannot fail all concurrent waiters.
	go s.fetchAndBroadcast(context.WithoutCancel(ctx), key, amount, fromCurrency, toCurrency, call)
	return waitForInFlight(ctx, amount, call)
}

func (s *CachedService) fetchAndBroadcast(
	ctx context.Context,
	key string,
	amount decimal.Decimal,
	fromCurrency, toCurrency string,
	call *inFlightCall,
) {
	result, err := s.inner.Convert(ctx, amount, fromCurrency, toCurrency)
	if err == nil {
		err = validateConversionRate(result.Rate)
	}

	fetchedAt := time.Now()
	s.mu.Lock()
	if err == nil {
		s.rates[key] = cachedRateEntry{
			Rate:      result.Rate,
			RateDate:  result.RateDate,
			ExpiresAt: fetchedAt.Add(s.ttl),
		}
		s.cleanupExpiredLocked(fetchedAt)
	}
	call.result = result
	call.err = err
	delete(s.inFlight, key)
	close(call.done)
	s.mu.Unlock()
}

func waitForInFlight(ctx context.Context, amount decimal.Decimal, call *inFlightCall) (ConversionResult, error) {
	select {
	case <-ctx.Done():
		return ConversionResult{}, ctx.Err()
	case <-call.done:
		if call.err != nil {
			return ConversionResult{}, call.err
		}
		return ConversionResult{
			Amount:   amount.Mul(call.result.Rate).Round(2),
			Rate:     call.result.Rate,
			RateDate: call.result.RateDate,
		}, nil
	}
}

func (s *CachedService) cleanupExpiredLocked(now time.Time) {
	interval := min(s.ttl, maxCleanupInterval)
	if !s.lastCleanup.IsZero() && now.Sub(s.lastCleanup) < interval {
		return
	}
	for pair, entry := range s.rates {
		if !now.Before(entry.ExpiresAt) {
			delete(s.rates, pair)
		}
	}
	s.lastCleanup = now
}

func applyCachedRate(amount decimal.Decimal, entry cachedRateEntry) ConversionResult {
	return ConversionResult{
		Amount:   amount.Mul(entry.Rate).Round(2),
		Rate:     entry.Rate,
		RateDate: entry.RateDate,
	}
}
