package telemetry

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const unsupportedExporterTypeFmt = "unsupported exporter type: %q"

// Providers holds the initialized OTel providers. Its Shutdown method is
// always safe to call, even when OTel is disabled (no-op).
type Providers struct {
	tp *trace.TracerProvider
	mp *metric.MeterProvider
}

// Shutdown flushes and shuts down both providers. Safe to call on a no-op
// instance returned when OTel is disabled.
func (p *Providers) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}
	var firstErr error
	if p.tp != nil {
		if err := p.tp.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if p.mp != nil {
		if err := p.mp.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Init initializes OpenTelemetry tracing and metrics. When cfg.Enabled is
// false it returns a non-nil no-op Providers so callers can always safely
// defer providers.Shutdown(ctx).
func Init(ctx context.Context, cfg *Config) (*Providers, error) {
	if !cfg.Enabled {
		return &Providers{}, nil
	}

	endpoint := cfg.Endpoint
	if endpoint == "" {
		switch cfg.ExporterType {
		case ExporterOTLPHTTP:
			endpoint = "http://localhost:4318"
		default: // otlp-grpc
			endpoint = "localhost:4317"
		}
	}

	if err := validateEndpoint(cfg.ExporterType, endpoint); err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTel resource: %w", err)
	}

	traceExp, err := newTraceExporter(ctx, cfg.ExporterType, endpoint, cfg.Insecure)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	metricExp, err := newMetricExporter(ctx, cfg.ExporterType, endpoint, cfg.Insecure)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	sampler := buildSampler(cfg.TraceSampleRate)

	tp := trace.NewTracerProvider(
		trace.WithBatcher(traceExp),
		trace.WithResource(res),
		trace.WithSampler(sampler),
	)

	mp := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExp)),
		metric.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Providers{tp: tp, mp: mp}, nil
}

func validateEndpoint(exporterType, endpoint string) error {
	switch exporterType {
	case ExporterOTLPGRPC, "":
		if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
			return fmt.Errorf("otlp-grpc endpoint must be host:port without scheme, got %q", endpoint)
		}
	case ExporterOTLPHTTP:
		if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
			return fmt.Errorf("otlp-http endpoint must include scheme (http:// or https://), got %q", endpoint)
		}
	case ExporterStdout:
		// no validation needed
	default:
		return fmt.Errorf(unsupportedExporterTypeFmt, exporterType)
	}
	return nil
}

func newTraceExporter(ctx context.Context, exporterType, endpoint string, insecure bool) (trace.SpanExporter, error) {
	switch exporterType {
	case ExporterOTLPGRPC, "":
		opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(endpoint)}
		if insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		return otlptracegrpc.New(ctx, opts...)
	case ExporterOTLPHTTP:
		opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(endpoint)}
		if insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		return otlptracehttp.New(ctx, opts...)
	case ExporterStdout:
		return stdouttrace.New(stdouttrace.WithPrettyPrint())
	default:
		return nil, fmt.Errorf(unsupportedExporterTypeFmt, exporterType)
	}
}

func newMetricExporter(ctx context.Context, exporterType, endpoint string, insecure bool) (metric.Exporter, error) {
	switch exporterType {
	case ExporterOTLPGRPC, "":
		opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(endpoint)}
		if insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
		return otlpmetricgrpc.New(ctx, opts...)
	case ExporterOTLPHTTP:
		opts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(endpoint)}
		if insecure {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}
		return otlpmetrichttp.New(ctx, opts...)
	case ExporterStdout:
		return stdoutmetric.New(stdoutmetric.WithPrettyPrint())
	default:
		return nil, fmt.Errorf(unsupportedExporterTypeFmt, exporterType)
	}
}

func buildSampler(rate float64) trace.Sampler {
	switch {
	case rate <= 0:
		return trace.NeverSample()
	case rate >= 1:
		return trace.AlwaysSample()
	default:
		return trace.TraceIDRatioBased(rate)
	}
}
