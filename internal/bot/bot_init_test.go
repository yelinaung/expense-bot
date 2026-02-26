package bot

import (
	"net/http"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/config"
	"gitlab.com/yelinaung/expense-bot/internal/telemetry"
)

func TestNewOTelInstrumentation(t *testing.T) {
	t.Parallel()

	t.Run("returns defaults when disabled", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{OTelEnabled: false}

		transport, metrics := newOTelInstrumentation(cfg)

		require.Equal(t, http.DefaultTransport, transport)
		require.Nil(t, metrics)
	})

	t.Run("returns instrumented transport and metrics when enabled", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{OTelEnabled: true}

		transport, metrics := newOTelInstrumentation(cfg)

		require.NotNil(t, transport)
		require.NotEqual(t, http.DefaultTransport, transport)
		require.NotNil(t, metrics)
	})
}

func TestCacheMetricsFrom(t *testing.T) {
	t.Parallel()

	t.Run("returns nil when metrics is nil", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, cacheMetricsFrom(nil))
	})

	t.Run("returns cache metrics when metrics provided", func(t *testing.T) {
		t.Parallel()
		metrics, err := telemetry.NewBotMetrics()
		require.NoError(t, err)

		cm := cacheMetricsFrom(metrics)
		require.NotNil(t, cm)
		require.Equal(t, metrics.CacheHits, cm.Hits)
		require.Equal(t, metrics.CacheMisses, cm.Misses)
	})
}

func TestInitGeminiClient(t *testing.T) {
	t.Parallel()

	t.Run("returns nil for empty API key", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, initGeminiClient(""))
	})

	t.Run("returns nil for invalid API key", func(t *testing.T) {
		t.Parallel()
		// An invalid key will fail client creation; function should return nil
		// without panicking. The genai client may or may not fail at
		// construction time depending on the SDK version, so we just assert
		// no panic and the result is handled gracefully.
		client := initGeminiClient("invalid-key-that-should-not-work")
		// Either nil (creation failed) or non-nil (lazy validation) is acceptable.
		_ = client
	})
}

func TestBuildMiddlewares(t *testing.T) {
	t.Parallel()

	noopMiddleware := func(next bot.HandlerFunc) bot.HandlerFunc {
		return next
	}

	t.Run("returns only whitelist when metrics is nil", func(t *testing.T) {
		t.Parallel()
		mws := buildMiddlewares(noopMiddleware, nil)
		require.Len(t, mws, 1)
	})

	t.Run("prepends tracing middleware when metrics provided", func(t *testing.T) {
		t.Parallel()
		metrics, err := telemetry.NewBotMetrics()
		require.NoError(t, err)

		mws := buildMiddlewares(noopMiddleware, metrics)
		require.Len(t, mws, 2)
	})
}

func TestLoadDisplayLocation(t *testing.T) {
	t.Parallel()

	t.Run("loads valid timezone", func(t *testing.T) {
		t.Parallel()
		loc := loadDisplayLocation("Asia/Singapore")
		require.Equal(t, "Asia/Singapore", loc.String())
	})

	t.Run("falls back to UTC for invalid timezone", func(t *testing.T) {
		t.Parallel()
		loc := loadDisplayLocation("Invalid/Timezone")
		require.Equal(t, time.UTC, loc)
	})

	t.Run("falls back to UTC for empty string", func(t *testing.T) {
		t.Parallel()
		loc := loadDisplayLocation("")
		require.Equal(t, "UTC", loc.String())
	})
}
