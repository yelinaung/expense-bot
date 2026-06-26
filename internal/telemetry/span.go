package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// StartSpan starts a child span on the bot tracer with the given attributes.
// When OTel is disabled the global no-op tracer is used, so this is safe and
// cheap to call unconditionally. The caller must End the returned span.
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	//nolint:spancheck // span is intentionally returned for the caller to End.
	return tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}
