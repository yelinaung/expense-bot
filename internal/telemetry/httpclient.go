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

	ctx, span := tracer.Start(req.Context(), "telegram.api "+method,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("rpc.system", "telegram"),
			attribute.String("telegram.method", method),
		),
	)
	defer span.End()

	// Attach the span context so the base transport (and any further
	// instrumentation) observes the in-flight span.
	req = req.WithContext(ctx)

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

const (
	unknownTelegramMethod = "unknown"
	telegramBotPrefix     = "/bot"
)

// telegramMethod extracts the Telegram API method from a request URL path of
// the form /bot<token>/<method>. It returns the method only when a non-empty
// segment follows the bot token; for a root or malformed path such as
// "/bot<token>" it returns unknownTelegramMethod so the token (which is part of
// the path) is never exposed in span names or attributes.
func telegramMethod(path string) string {
	if !strings.HasPrefix(path, telegramBotPrefix) {
		return unknownTelegramMethod
	}
	// rest is "<token>[/...]/<method>"; a '/' must separate the token from a
	// method segment, otherwise the trailing segment is the token itself.
	rest := path[len(telegramBotPrefix):]
	if !strings.Contains(rest, "/") {
		return unknownTelegramMethod
	}
	method := rest[strings.LastIndexByte(rest, '/')+1:]
	if method == "" {
		return unknownTelegramMethod
	}
	return method
}
