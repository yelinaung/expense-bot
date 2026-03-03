package bot

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
	"gitlab.com/yelinaung/expense-bot/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func histogramDataPointCount(resourceMetrics metricdata.ResourceMetrics, metricName string) int {
	for _, scopeMetric := range resourceMetrics.ScopeMetrics {
		for _, metric := range scopeMetric.Metrics {
			if metric.Name != metricName {
				continue
			}
			if hist, ok := metric.Data.(metricdata.Histogram[float64]); ok {
				return len(hist.DataPoints)
			}
		}
	}
	return 0
}

func TestSaveExpenseCore_RecordsAmountMetricForInexactFloat(t *testing.T) {
	ctx := context.Background()
	pool := TestDB(ctx, t)
	b := setupTestBot(t, pool)

	userID := int64(310001)
	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "metricsuser",
		FirstName: "Metrics",
	})
	require.NoError(t, err)

	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(meterProvider)
	defer otel.SetMeterProvider(noop.NewMeterProvider())
	defer func() {
		_ = meterProvider.Shutdown(ctx)
	}()

	metrics, err := telemetry.NewBotMetrics()
	require.NoError(t, err)
	b.metrics = metrics

	mockBot := mocks.NewMockBot()
	parsed := &ParsedExpense{
		Amount:      mustParseDecimal("12.34"),
		Currency:    "SGD",
		Description: "inexact float amount",
	}

	b.saveExpenseCore(ctx, mockBot, 12345, userID, parsed, nil)
	require.Equal(t, 1, mockBot.SentMessageCount())

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &rm))
	require.Positive(t, histogramDataPointCount(rm, "expense.amount"))
}
