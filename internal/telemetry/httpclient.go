package telemetry

import (
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// InstrumentedTransport wraps the given http.RoundTripper (or
// http.DefaultTransport if nil) with OTel HTTP tracing.
func InstrumentedTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return otelhttp.NewTransport(base)
}

// TelegramHTTPClient returns an *http.Client for the go-telegram bot library
// that emits a client span per Telegram API call, named after the method
// (e.g. "telegram.api sendMessage"). pollTimeout must match the bot's poll
// timeout so the long-poll getUpdates request is not cut short.
//
// It deliberately does NOT use otelhttp: that instrumentation records the full
// request URL, and the Telegram endpoint embeds the bot token in the path
// (/bot<token>/<method>), which would leak the secret into span attributes.
// This transport records only the method name and response status.
func TelegramHTTPClient(pollTimeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   pollTimeout,
		Transport: telegramTransport{base: http.DefaultTransport},
	}
}

type telegramTransport struct {
	base http.RoundTripper
}

func (t telegramTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	method := telegramMethod(req.URL.Path)

	// Skip the long-poll getUpdates request: it is held open for the poll
	// timeout and runs outside any handler span, so instrumenting it would
	// emit a continuous stream of parentless, ~pollTimeout-long spans.
	if method == "getUpdates" {
		return t.base.RoundTrip(req)
	}

	_, span := tracer.Start(req.Context(), "telegram.api "+method,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("rpc.system", "telegram"),
			attribute.String("telegram.method", method),
		),
	)
	defer span.End()

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return resp, err
	}
	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	if resp.StatusCode >= http.StatusBadRequest {
		span.SetStatus(codes.Error, resp.Status)
	}
	return resp, nil
}

// telegramMethod extracts the Telegram API method (the trailing path segment)
// from a request URL path of the form /bot<token>/<method>. The token is in a
// separate segment and is never returned.
func telegramMethod(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		path = path[idx+1:]
	}
	if path == "" {
		return unknownTelegramMethod
	}
	return path
}

const unknownTelegramMethod = "unknown"
