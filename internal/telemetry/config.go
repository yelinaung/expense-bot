// Package telemetry provides OpenTelemetry tracing and metrics integration.
package telemetry

// Exporter type constants.
const (
	ExporterOTLPGRPC = "otlp-grpc"
	ExporterOTLPHTTP = "otlp-http"
	ExporterStdout   = "stdout"
)

// Config holds OpenTelemetry configuration.
type Config struct {
	Enabled         bool
	ServiceName     string
	ServiceVersion  string
	Environment     string
	ExporterType    string // "otlp-grpc", "otlp-http", "stdout"
	Endpoint        string // host:port for gRPC, full URL for HTTP
	Insecure        bool
	TraceSampleRate float64 // 0.0 to 1.0
}
