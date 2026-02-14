package exchange

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var errRateMissing = errors.New("conversion rate missing in response")

// FrankfurterClient is a client for frankfurter.app exchange rates API.
type FrankfurterClient struct {
	baseURL    string
	httpClient *http.Client
}

type frankfurterResponse struct {
	Base  string                 `json:"base"`
	Date  string                 `json:"date"`
	Rates map[string]json.Number `json:"rates"`
}

// NewFrankfurterClient creates a Frankfurter API client.
func NewFrankfurterClient(baseURL string, timeout time.Duration) *FrankfurterClient {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		trimmed = "https://api.frankfurter.app"
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &FrankfurterClient{
		baseURL: trimmed,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Convert converts amount from one currency to another using latest rates.
func (c *FrankfurterClient) Convert(
	ctx context.Context,
	amount decimal.Decimal,
	fromCurrency, toCurrency string,
) (ConversionResult, error) {
	from := strings.ToUpper(strings.TrimSpace(fromCurrency))
	to := strings.ToUpper(strings.TrimSpace(toCurrency))
	if from == "" || to == "" {
		return ConversionResult{}, errors.New("from and to currencies are required")
	}
	if amount.IsNegative() || amount.IsZero() {
		return ConversionResult{}, errors.New("amount must be positive")
	}
	if from == to {
		return ConversionResult{
			Amount:   amount,
			Rate:     decimal.NewFromInt(1),
			RateDate: time.Now().UTC(),
		}, nil
	}

	endpoint := fmt.Sprintf("%s/latest?from=%s&to=%s",
		c.baseURL,
		url.QueryEscape(from),
		url.QueryEscape(to),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ConversionResult{}, fmt.Errorf("failed to create conversion request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ConversionResult{}, fmt.Errorf("failed to request conversion rate: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return ConversionResult{}, fmt.Errorf("exchange API returned status %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()

	var payload frankfurterResponse
	if err := decoder.Decode(&payload); err != nil {
		return ConversionResult{}, fmt.Errorf("failed to decode conversion response: %w", err)
	}

	rateStr, ok := payload.Rates[to]
	if !ok {
		return ConversionResult{}, errRateMissing
	}

	rate, err := decimal.NewFromString(rateStr.String())
	if err != nil {
		return ConversionResult{}, fmt.Errorf("failed to parse conversion rate: %w", err)
	}
	if !rate.IsPositive() {
		return ConversionResult{}, errors.New("conversion rate must be positive")
	}

	rateDate, err := time.Parse("2006-01-02", payload.Date)
	if err != nil {
		return ConversionResult{}, fmt.Errorf("failed to parse conversion date: %w", err)
	}

	convertedAmount := amount.Mul(rate).Round(2)

	return ConversionResult{
		Amount:   convertedAmount,
		Rate:     rate,
		RateDate: rateDate,
	}, nil
}
