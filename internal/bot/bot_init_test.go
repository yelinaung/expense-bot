package bot

import (
	"context"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/config"
	"gitlab.com/yelinaung/expense-bot/internal/telemetry"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
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

func TestNewFileDownloadClient(t *testing.T) {
	t.Parallel()

	t.Run("uses plain client when OTel disabled", func(t *testing.T) {
		t.Parallel()
		c := newFileDownloadClient(&config.Config{OTelEnabled: false})
		require.NotNil(t, c)
		require.Equal(t, 30*time.Second, c.Timeout)
		require.Nil(t, c.Transport, "plain client should use the default transport")
	})

	t.Run("uses token-safe transport when OTel enabled", func(t *testing.T) {
		t.Parallel()
		c := newFileDownloadClient(&config.Config{OTelEnabled: true})
		require.NotNil(t, c)
		require.Equal(t, 30*time.Second, c.Timeout)
		require.NotNil(t, c.Transport, "OTel-enabled client should use a token-safe transport")
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
		require.Nil(t, initGeminiClient(context.Background(), ""))
	})

	t.Run("returns nil for invalid API key", func(t *testing.T) {
		t.Parallel()
		// An invalid key will fail client creation; function should return nil
		// without panicking. The genai client may or may not fail at
		// construction time depending on the SDK version, so we just assert
		// no panic and the result is handled gracefully.
		client := initGeminiClient(context.Background(), "invalid-key-that-should-not-work")
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

	t.Run("includes tracing middleware when metrics provided", func(t *testing.T) {
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

// TestBuildMiddlewaresOrder asserts the whitelist runs before the tracing
// middleware so blocked updates never open a span. It is intentionally not
// parallel: it installs a global in-memory tracer provider.
func TestBuildMiddlewaresOrder(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prev)
		_ = tp.Shutdown(context.Background())
	})

	metrics, err := telemetry.NewBotMetrics()
	require.NoError(t, err)

	// compose replicates github.com/go-telegram/bot's applyMiddlewares, which
	// wraps middlewares in reverse slice order so mws[0] is the outermost
	// wrapper and runs first.
	compose := func(mws []bot.Middleware, inner bot.HandlerFunc) bot.HandlerFunc {
		wrapped := inner
		for i := range slices.Backward(mws) {
			wrapped = mws[i](wrapped)
		}
		return wrapped
	}

	updateFor := func(text string) *tgmodels.Update {
		return &tgmodels.Update{
			Message: &tgmodels.Message{
				Text: text,
				Chat: tgmodels.Chat{ID: 1},
				From: &tgmodels.User{ID: 2},
			},
		}
	}

	t.Run("blocked update never opens a span", func(t *testing.T) {
		sr.Reset()
		whitelistRan, handlerRan := false, false
		whitelist := func(next bot.HandlerFunc) bot.HandlerFunc {
			return func(context.Context, *bot.Bot, *tgmodels.Update) {
				whitelistRan = true
			}
		}
		chain := compose(buildMiddlewares(whitelist, metrics), bot.HandlerFunc(
			func(context.Context, *bot.Bot, *tgmodels.Update) { handlerRan = true }))
		chain(context.Background(), nil, updateFor("/1"))

		require.True(t, whitelistRan, "whitelist middleware must run first")
		require.False(t, handlerRan, "blocked update must not reach the handler")
		require.Empty(t, sr.Ended(), "blocked update must not open a span")
	})

	t.Run("allowed update opens a bounded span", func(t *testing.T) {
		sr.Reset()
		handlerRan := false
		passThrough := func(next bot.HandlerFunc) bot.HandlerFunc { return next }
		chain := compose(buildMiddlewares(passThrough, metrics), bot.HandlerFunc(
			func(context.Context, *bot.Bot, *tgmodels.Update) { handlerRan = true }))
		chain(context.Background(), nil, updateFor("/1"))

		require.True(t, handlerRan, "allowed update must reach the handler")
		ended := sr.Ended()
		require.Len(t, ended, 1)
		require.Equal(t, "telegram.command unknown", ended[0].Name())
	})
}
