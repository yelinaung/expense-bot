package exchange

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrankfurterClient_Convert(t *testing.T) {
	t.Parallel()

	t.Run("converts successfully", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/latest", r.URL.Path)
			assert.Equal(t, "USD", r.URL.Query().Get("from"))
			assert.Equal(t, "SGD", r.URL.Query().Get("to"))
			_, _ = w.Write([]byte(`{"amount":1,"base":"USD","date":"2026-02-14","rates":{"SGD":1.35}}`))
		}))

		client := NewFrankfurterClient(server.URL, time.Second, server.Client().Transport)
		got, err := client.Convert(context.Background(), decimal.RequireFromString("10"), "usd", "sgd")
		require.NoError(t, err)
		require.Equal(t, decimal.RequireFromString("13.50"), got.Amount)
		require.Equal(t, decimal.RequireFromString("1.35"), got.Rate)
		require.Equal(t, "2026-02-14", got.RateDate.Format("2006-01-02"))
	})

	t.Run("returns error on non 200 response", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))

		client := NewFrankfurterClient(server.URL, time.Second, server.Client().Transport)
		_, err := client.Convert(context.Background(), decimal.RequireFromString("10"), "USD", "SGD")
		require.Error(t, err)
		require.Contains(t, err.Error(), "status 502")
	})

	t.Run("returns error when target rate is missing", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"amount":1,"base":"USD","date":"2026-02-14","rates":{"EUR":0.93}}`))
		}))

		client := NewFrankfurterClient(server.URL, time.Second, server.Client().Transport)
		_, err := client.Convert(context.Background(), decimal.RequireFromString("10"), "USD", "SGD")
		require.ErrorIs(t, err, errRateMissing)
	})

	t.Run("returns error when target rate is non-positive", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"amount":1,"base":"USD","date":"2026-02-14","rates":{"SGD":0}}`))
		}))

		client := NewFrankfurterClient(server.URL, time.Second, server.Client().Transport)
		_, err := client.Convert(context.Background(), decimal.RequireFromString("10"), "USD", "SGD")
		require.ErrorIs(t, err, errInvalidNonPositiveRate)
	})

	t.Run("returns same amount for same currency", func(t *testing.T) {
		t.Parallel()

		client := NewFrankfurterClient("https://api.frankfurter.app", time.Second, nil)
		got, err := client.Convert(context.Background(), decimal.RequireFromString("12.34"), "SGD", "SGD")
		require.NoError(t, err)
		require.Equal(t, decimal.RequireFromString("12.34"), got.Amount)
		require.Equal(t, decimal.NewFromInt(1), got.Rate)
	})

	t.Run("returns validation error for non-positive amount", func(t *testing.T) {
		t.Parallel()

		client := NewFrankfurterClient("https://api.frankfurter.app", time.Second, nil)
		_, err := client.Convert(context.Background(), decimal.Zero, "USD", "SGD")
		require.Error(t, err)
		require.Contains(t, err.Error(), "positive")
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(100 * time.Millisecond)
				_, _ = fmt.Fprint(w, `{"amount":13.5,"base":"USD","date":"2026-02-14","rates":{"SGD":1.35}}`)
			}))

			client := NewFrankfurterClient(server.URL, time.Second, server.Client().Transport)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()

			_, err := client.Convert(ctx, decimal.RequireFromString("10"), "USD", "SGD")
			require.Error(t, err)
		})
	})
}
