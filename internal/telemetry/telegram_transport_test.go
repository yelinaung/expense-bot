package telemetry

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestTelegramMethod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{"send message", "/bot12345:secret/sendMessage", "sendMessage"},
		{"get updates", "/bot12345:secret/getUpdates", "getUpdates"},
		{"get file", "/bot12345:secret/getFile", "getFile"},
		{"test environment", "/bot12345:secret/test/sendMessage", "sendMessage"},
		{"empty trailing", "/bot12345:secret/", "unknown"},
		{"root bot path leaks no token", "/bot12345:secret", "unknown"},
		{"no bot prefix", "/foo/sendMessage", "unknown"},
		{"no slash", "", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, telegramMethod(tt.path))
		})
	}
}

func okResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(http.NoBody),
		Header:     make(http.Header),
	}
}

// telegramTestURL is a fake Telegram endpoint whose token segment is used to
// assert the secret never leaks into span attributes.
const (
	telegramTestToken = "12345:secret"
	telegramTestURL   = "https://api.telegram.org/bot" + telegramTestToken + "/"
)

// roundTrip runs a request for the given Telegram method through tr and closes
// the response body.
func roundTrip(t *testing.T, tr telegramTransport, method string) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, telegramTestURL+method, nil)
	require.NoError(t, err)
	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
}

// TestTelegramTransport_SpanBehavior installs an in-memory tracer provider and
// asserts the transport's span behavior. It is intentionally not parallel: it
// mutates the global tracer provider that the package-level tracer delegates to.
func TestTelegramTransport_SpanBehavior(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	base := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		return okResponse(), nil
	})
	tr := telegramTransport{base: base}

	t.Run("sendMessage creates a client span without leaking the token", func(t *testing.T) {
		sr.Reset()
		roundTrip(t, tr, "sendMessage")

		spans := sr.Ended()
		require.Len(t, spans, 1)
		span := spans[0]
		require.Equal(t, "telegram.api sendMessage", span.Name())
		require.Equal(t, trace.SpanKindClient, span.SpanKind())

		attrs := make(map[string]string)
		for _, kv := range span.Attributes() {
			attrs[string(kv.Key)] = kv.Value.String()
			// The bot token must never appear in any span attribute value.
			require.NotContains(t, kv.Value.String(), "secret")
			require.NotContains(t, kv.Value.String(), "12345")
		}
		require.Equal(t, "telegram", attrs["rpc.system"])
		require.Equal(t, "sendMessage", attrs["telegram.method"])
		require.Equal(t, "200", attrs["http.response.status_code"])
	})

	t.Run("getUpdates creates no span", func(t *testing.T) {
		sr.Reset()
		roundTrip(t, tr, "getUpdates")
		require.Empty(t, sr.Ended())
	})

	t.Run("non-2xx status marks the span as error", func(t *testing.T) {
		sr.Reset()
		errTr := telegramTransport{base: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Status:     "400 Bad Request",
				Body:       io.NopCloser(http.NoBody),
				Header:     make(http.Header),
			}, nil
		})}
		roundTrip(t, errTr, "sendMessage")

		spans := sr.Ended()
		require.Len(t, spans, 1)
		require.Equal(t, codes.Error, spans[0].Status().Code)
	})

	t.Run("transport error is recorded on the span", func(t *testing.T) {
		sr.Reset()
		wantErr := errors.New("dial failed")
		errTr := telegramTransport{base: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			return nil, wantErr
		})}

		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, telegramTestURL+"sendMessage", nil)
		require.NoError(t, err)
		resp, err := errTr.RoundTrip(req)
		require.ErrorIs(t, err, wantErr)
		require.Nil(t, resp)

		spans := sr.Ended()
		require.Len(t, spans, 1)
		require.Equal(t, codes.Error, spans[0].Status().Code)
	})
}

func TestTelegramHTTPClient(t *testing.T) {
	t.Parallel()

	c := TelegramHTTPClient(time.Minute)
	require.NotNil(t, c)
	require.Equal(t, time.Minute, c.Timeout)
	require.IsType(t, telegramTransport{}, c.Transport)
}
