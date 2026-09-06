package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const testExportDrainTimeout = 300 * time.Millisecond

// newRecordingServer starts an httptest server that records the path of each
// received request into got without blocking the handler. The caller owns the
// channel and is responsible for tearing the server down.
func newRecordingServer(t *testing.T, got chan<- string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case got <- r.URL.Path:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	return srv
}

// waitForPaths drains buffered request paths until no new request arrives
// within the drain timeout, returning every path captured in arrival order.
func waitForPaths(ch <-chan string) []string {
	var paths []string
	for {
		select {
		case p := <-ch:
			paths = append(paths, p)
		case <-time.After(testExportDrainTimeout):
			return paths
		}
	}
}

// waitForPath returns the first recorded request path, failing the test if no
// export request was ever received.
func waitForPath(t *testing.T, got <-chan string) string {
	t.Helper()
	paths := waitForPaths(got)
	require.NotEmpty(t, paths, "no otlp-http export request was received")
	return paths[0]
}

// TestNormalizeOTLPHTTPEndpoint pins the helper contract after the base-path
// fix: it returns the parsed host, the operator-supplied base path (no longer
// dropped), the raw un-cleaned URL path, and the transport-security flag
// derived from the scheme and the insecure override. It guards against a
// regression where the helper silently drops u.Path again.
func TestNormalizeOTLPHTTPEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		endpoint         string
		insecure         bool
		wantHost         string
		wantBasePath     string
		wantHTTPInsecure bool
		wantErr          bool
	}{
		{"http_host_port", "http://localhost:4318", false, "localhost:4318", "", true, false},
		{"https_host_port", "https://collector:4318", false, "collector:4318", "", false, false},
		{"http_with_base_path", "http://collector:4318/otlp", false, "collector:4318", "/otlp", true, false},
		{"https_with_base_path", "https://collector:4318/otlp/v1", false, "collector:4318", "/otlp/v1", false, false},
		{"http_trailing_slash", "http://collector:4318/otlp/", false, "collector:4318", "/otlp/", true, false},
		{"insecure_override_forces_https_insecure", "https://collector:4318", true, "collector:4318", "", true, false},
		{"missing_host", "http:///otlp", false, "", "", false, true},
		{"parse_error", "http://localhost:4318/%zz", false, "", "", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			host, basePath, httpInsecure, err := normalizeOTLPHTTPEndpoint(tt.endpoint, tt.insecure)
			if tt.wantErr {
				require.Error(t, err)
				require.Empty(t, host)
				require.Empty(t, basePath)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantHost, host)
			require.Equal(t, tt.wantBasePath, basePath)
			require.Equal(t, tt.wantHTTPInsecure, httpInsecure)
		})
	}
}

// traceExportPath exercises the real newTraceExporter wiring (the production
// code path under test) for an endpoint appended with urlPath, returning the
// absolute path the OTLP/HTTP trace exporter POSTs the span export to.
func traceExportPath(t *testing.T, urlPath string) string {
	t.Helper()
	got := make(chan string, 4)
	srv := newRecordingServer(t, got)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exp, err := newTraceExporter(ctx, ExporterOTLPHTTP, srv.URL+urlPath, true)
	require.NoError(t, err)

	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp))
	defer func() { _ = tp.Shutdown(ctx) }()

	_, span := tp.Tracer("test").Start(ctx, "span")
	span.End()
	require.NoError(t, tp.ForceFlush(ctx))

	return waitForPath(t, got)
}

// TestTraceExporterHonorsBasePath verifies the OTLP/HTTP trace exporter
// forwards the operator-supplied base path to WithURLPath: a pathless endpoint
// still targets /v1/traces (no regression) and a base-path endpoint targets
// /otlp/v1/traces (the bug, fixed). This guards against a regression where the
// WithURLPath call is removed at the call site.
func TestTraceExporterHonorsBasePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		urlPath string
		want    string
	}{
		{"no_path", "", "/v1/traces"},
		{"base_path", "/otlp", "/otlp/v1/traces"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, traceExportPath(t, tt.urlPath))
		})
	}
}

// metricExportPath exercises the real newMetricExporter wiring for an endpoint
// appended with urlPath, returning the absolute path the OTLP/HTTP metric
// exporter POSTs the metric export to.
func metricExportPath(t *testing.T, urlPath string) string {
	t.Helper()
	got := make(chan string, 4)
	srv := newRecordingServer(t, got)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exp, err := newMetricExporter(ctx, ExporterOTLPHTTP, srv.URL+urlPath, true)
	require.NoError(t, err)

	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)))
	defer func() { _ = mp.Shutdown(ctx) }()

	counter, err := mp.Meter("test").Int64Counter("c")
	require.NoError(t, err)
	counter.Add(ctx, 1)
	require.NoError(t, mp.ForceFlush(ctx))

	return waitForPath(t, got)
}

// TestMetricExporterHonorsBasePath is the metric-exporter counterpart of
// TestTraceExporterHonorsBasePath: it verifies a pathless endpoint targets
// /v1/metrics and a base-path endpoint targets /otlp/v1/metrics.
func TestMetricExporterHonorsBasePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		urlPath string
		want    string
	}{
		{"no_path", "", "/v1/metrics"},
		{"base_path", "/otlp", "/otlp/v1/metrics"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, metricExportPath(t, tt.urlPath))
		})
	}
}
