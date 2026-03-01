package telemetry

import (
	"go.opentelemetry.io/otel"
	otelmetric "go.opentelemetry.io/otel/metric"
)

// BotMetrics holds all OTel metric instruments for the bot.
type BotMetrics struct {
	// Handler metrics (recorded by middleware)
	HandlerCount     otelmetric.Int64Counter
	HandlerDuration  otelmetric.Float64Histogram
	HandlersInFlight otelmetric.Int64UpDownCounter

	// Expense operation metrics
	ExpenseOps    otelmetric.Int64Counter
	ExpenseAmount otelmetric.Float64Histogram

	// Background job metrics
	BackgroundJobRuns     otelmetric.Int64Counter
	BackgroundJobDuration otelmetric.Float64Histogram
	DraftsCleaned         otelmetric.Int64Counter

	// Cache metrics
	CacheHits   otelmetric.Int64Counter
	CacheMisses otelmetric.Int64Counter
}

// NewBotMetrics creates and registers all metric instruments.
func NewBotMetrics() (*BotMetrics, error) {
	meter := otel.Meter("expense-bot")

	handlerCount, err := meter.Int64Counter("telegram.handler.count",
		otelmetric.WithDescription("Number of handled Telegram updates"))
	if err != nil {
		return nil, err
	}

	handlerDuration, err := meter.Float64Histogram("telegram.handler.duration",
		otelmetric.WithDescription("Duration of Telegram update handling in seconds"),
		otelmetric.WithUnit("s"))
	if err != nil {
		return nil, err
	}

	handlersInFlight, err := meter.Int64UpDownCounter("telegram.handler.in_flight",
		otelmetric.WithDescription("Number of Telegram updates being processed"))
	if err != nil {
		return nil, err
	}

	expenseOps, err := meter.Int64Counter("expense.operations",
		otelmetric.WithDescription("Number of expense operations"))
	if err != nil {
		return nil, err
	}

	expenseAmount, err := meter.Float64Histogram("expense.amount",
		otelmetric.WithDescription("Expense amounts recorded"))
	if err != nil {
		return nil, err
	}

	backgroundJobRuns, err := meter.Int64Counter("background.job.runs",
		otelmetric.WithDescription("Number of background job runs"))
	if err != nil {
		return nil, err
	}

	backgroundJobDuration, err := meter.Float64Histogram("background.job.duration",
		otelmetric.WithDescription("Duration of background job runs in seconds"),
		otelmetric.WithUnit("s"))
	if err != nil {
		return nil, err
	}

	draftsCleaned, err := meter.Int64Counter("background.drafts_cleaned",
		otelmetric.WithDescription("Number of expired drafts cleaned up"))
	if err != nil {
		return nil, err
	}

	cacheHits, err := meter.Int64Counter("cache.hits",
		otelmetric.WithDescription("Number of cache hits"))
	if err != nil {
		return nil, err
	}

	cacheMisses, err := meter.Int64Counter("cache.misses",
		otelmetric.WithDescription("Number of cache misses"))
	if err != nil {
		return nil, err
	}

	return &BotMetrics{
		HandlerCount:          handlerCount,
		HandlerDuration:       handlerDuration,
		HandlersInFlight:      handlersInFlight,
		ExpenseOps:            expenseOps,
		ExpenseAmount:         expenseAmount,
		BackgroundJobRuns:     backgroundJobRuns,
		BackgroundJobDuration: backgroundJobDuration,
		DraftsCleaned:         draftsCleaned,
		CacheHits:             cacheHits,
		CacheMisses:           cacheMisses,
	}, nil
}
