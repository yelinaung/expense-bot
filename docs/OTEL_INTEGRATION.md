# OpenTelemetry Integration Plan

## Context

The expense-bot is a polling-based Telegram bot with **no existing metrics or tracing**. It uses PostgreSQL (pgx), calls external APIs (Gemini, Frankfurter, Telegram), and runs background jobs. Standard HTTP middleware patterns don't apply here since there's no HTTP server — instrumentation must target the Telegram bot middleware layer, database hooks, and HTTP clients directly.

**Goal**: Add full observability (traces, metrics, log correlation) via OpenTelemetry, disabled by default (`OTEL_ENABLED=true` to activate), exporting via OTLP to any compatible backend.

---

## Phase 1: Foundation — OTel package + config + initialization

### 1a. Add dependencies

```
go.opentelemetry.io/otel
go.opentelemetry.io/otel/sdk
go.opentelemetry.io/otel/sdk/metric
go.opentelemetry.io/otel/trace
go.opentelemetry.io/otel/metric
go.opentelemetry.io/otel/attribute
go.opentelemetry.io/otel/codes
go.opentelemetry.io/otel/semconv/v1.26.0
go.opentelemetry.io/otel/propagation
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc
go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp
go.opentelemetry.io/otel/exporters/stdout/stdouttrace
go.opentelemetry.io/otel/exporters/stdout/stdoutmetric
go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp
github.com/exaring/otelpgx
```

### 1b. Create `internal/telemetry/` package (4 new files)

**`internal/telemetry/config.go`** — OTel config struct:
- `Enabled bool`, `ServiceName string`, `ServiceVersion string`, `Environment string`
- `ExporterType string` ("otlp-grpc" | "otlp-http" | "stdout")
- `Endpoint string`, `Insecure bool`, `TraceSampleRate float64`

**`internal/telemetry/telemetry.go`** — Provider init/shutdown:
- `Init(ctx, Config) (*Providers, error)` — creates TracerProvider + MeterProvider
- Creates resource with `service.name`, `service.version`, `deployment.environment`
- Selects exporter based on `ExporterType`
- Configures sampler: `AlwaysSample` / `TraceIDRatioBased` / `NeverSample`
- Registers global providers via `otel.SetTracerProvider()`, `otel.SetMeterProvider()`
- When `Enabled=false`, returns a no-op `*Providers` (non-nil) whose `Shutdown()` is a harmless no-op. This avoids nil-guard requirements in callers — `defer providers.Shutdown(ctx)` is always safe.
- `Shutdown(ctx)` — flushes and shuts down both providers (no-op when disabled)

**`internal/telemetry/middleware.go`** — Telegram bot tracing middleware:
- `TracingMiddleware(instruments) bot.Middleware` — creates root span per Telegram update
- `classifyUpdate(update)` — derives span name: `telegram.command /add`, `telegram.photo`, `telegram.callback receipt_confirm`, etc.
- `updateAttributes(update)` — extracts `telegram.chat_id`, `telegram.user_id`, `messaging.system=telegram` (see **Privacy** section below for ID handling)
- Panic recovery that records error on span before re-panicking
- Records handler duration (Histogram), handler count (Counter), and in-flight handlers (Int64UpDownCounter — not a synchronous Gauge, which has limited support in OTel Go)

**`internal/telemetry/httpclient.go`** — Instrumented HTTP transport:
- `InstrumentedTransport(base) http.RoundTripper` — wraps with `otelhttp.NewTransport`

### 1c. Modify `internal/config/config.go`

Add OTel fields to `Config` struct + env var parsing in `Load()`:

| Env var | Default | Field |
|---------|---------|-------|
| `OTEL_ENABLED` | `false` | `OTelEnabled` |
| `OTEL_SERVICE_NAME` | `expense-bot` | `OTelServiceName` |
| `OTEL_ENVIRONMENT` | `production` | `OTelEnvironment` |
| `OTEL_EXPORTER_TYPE` | `otlp-grpc` | `OTelExporterType` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | per-exporter (see below) | `OTelEndpoint` |
| `OTEL_EXPORTER_OTLP_INSECURE` | `false` | `OTelInsecure` |
| `OTEL_TRACE_SAMPLE_RATE` | `1.0` | `OTelTraceSampleRate` |

**Endpoint defaults by exporter type:**
- `otlp-grpc`: `localhost:4317` (host:port, no scheme)
- `otlp-http`: `http://localhost:4318` (full URL with scheme)
- `stdout`: endpoint is ignored

When `OTEL_EXPORTER_OTLP_ENDPOINT` is unset, `Init()` applies the correct default for the selected exporter type. When a user-provided endpoint is set, `Init()` validates it at startup:
- `otlp-grpc`: must be `host:port` (no scheme). Fail with a clear error if a `http://` or `https://` prefix is detected.
- `otlp-http`: must include a scheme (`http://` or `https://`). Fail with a clear error if no scheme is present.
- `stdout`: endpoint is ignored regardless of value.

Validation errors are returned from `Init()` before any exporter is created, giving a fast-fail on misconfiguration.

### 1d. Modify `main.go`

After config load + logger setup, before DB connect:
- Build `telemetry.Config` from `cfg` fields (pass `version` from ldflags as `ServiceVersion`)
- Call `telemetry.Init(ctx, otelCfg)`
- Defer `providers.Shutdown()` with a 10s timeout context

---

## Phase 2: Database Tracing

### Modify `internal/database/database.go`

Change `Connect()` to:
1. `pgxpool.ParseConfig(databaseURL)` instead of `pgxpool.New`
2. Attach `cfg.ConnConfig.Tracer = otelpgx.NewTracer()`
3. `pgxpool.NewWithConfig(ctx, cfg)`

This gives **automatic spans for every DB query** across all repositories with zero other code changes. The `otelpgx` tracer reads span context from `ctx` (already passed by all repository methods) and creates child spans with `db.system`, `db.statement`, `db.operation` attributes.

---

## Phase 3: Handler Tracing Middleware

### Modify `internal/bot/bot.go`

- Add `metrics *telemetry.BotMetrics` and `httpClient *http.Client` fields to `Bot` struct
- In `New()`: if `cfg.OTelEnabled`, create `telemetry.NewInstruments()` and prepend `telemetry.TracingMiddleware(instruments)` before `whitelistMiddleware` in the middleware chain
- Initialize `httpClient` with `telemetry.InstrumentedTransport` when OTel enabled, plain `http.DefaultTransport` otherwise. **Must preserve the existing 30s `Timeout`** from the current `downloadClient` (`&http.Client{Timeout: 30 * time.Second, Transport: transport}`) to prevent goroutine leaks or hanging connections.
- Replace package-level `downloadClient` usage in `downloadFile` with `b.httpClient`

The tracing middleware runs **before** whitelist — so even rejected requests get a span (useful for monitoring unauthorized access attempts).

---

## Phase 4: External API Instrumentation

### Modify `internal/exchange/frankfurter_client.go`
- `NewFrankfurterClient` accepts optional `http.RoundTripper` parameter
- Pass `telemetry.InstrumentedTransport(nil)` from bot initialization when OTel enabled

### Modify `internal/gemini/receipt_parser.go`, `voice_parser.go`, `category_suggester.go`
- Add manual spans around `GenerateContent` calls (genai manages its own HTTP transport)
- Attributes: `gemini.model`, `gemini.operation`, `gemini.input_size_bytes`
- Record errors on spans

### Modify `internal/bot/handlers_receipt.go`, `handlers_voice.go`
- Add child spans for file download + OCR/voice parse sub-operations

---

## Phase 5: Metrics

### Create `internal/telemetry/metrics.go`

Define `BotMetrics` struct with these instruments:

| Metric | Type | Attributes | Where recorded |
|--------|------|------------|----------------|
| `expense.operations` | Counter | `operation`, `status` | `handlers_commands.go` (add/edit/delete) |
| `expense.amount` | Histogram | `currency` | `handlers_commands.go` (after create) |
| `external.api.duration` | Histogram | `service`, `operation` | gemini/, exchange/, handlers_receipt.go |
| `external.api.errors` | Counter | `service`, `error_type` | gemini/, exchange/ |
| `background.job.runs` | Counter | `job`, `status` | bot.go (cleanup), reminder.go |
| `background.job.duration` | Histogram | `job` | bot.go (cleanup), reminder.go |
| `background.drafts_cleaned` | Counter | — | bot.go (cleanup) |
| `cache.hits` / `cache.misses` | Counter | `cache` | bot.go (categories), cached_service.go |

All metric recording is guarded by `if b.metrics != nil` — zero overhead when OTel is disabled.

### Modify handler files to record metrics
- `handlers_commands.go`: expense CRUD counters + amount histogram
- `bot.go`: background job metrics in `cleanupExpiredDrafts` and `startDraftCleanupLoop`
- `reminder.go`: reminder job metrics in `checkAndSendReminders`
- `exchange/cached_service.go`: cache hit/miss counters

---

## Phase 6: Log Correlation

### Modify `internal/logger/logger.go`

Add `WithTraceContext(ctx) zerolog.Logger` helper:
- Extracts `trace_id` and `span_id` from the active span in context
- Returns enriched zerolog logger with those fields
- Returns base `Log` if no active span

### Gradual adoption in handlers
- Replace key `logger.Log.Error()` calls in handler error paths with `logger.WithTraceContext(ctx).Error()`
- Start with error-level logs, expand to info-level over time
- Log backends (Loki, Datadog) can then correlate logs to traces via `trace_id`

---

## Privacy

The codebase already hashes Telegram identifiers in logs via `logger.HashUserID()` and `logger.HashChatID()` (see `internal/logger/privacy.go`). **Telemetry attributes must follow the same convention**:

- `telegram.user_id` and `telegram.chat_id` span attributes use the hashed values from `logger.HashUserID()`/`logger.HashChatID()`, never raw numeric IDs.
- No user-provided text (expense descriptions, receipt content, voice transcriptions) is recorded in span attributes or metric labels.
- The `db.statement` attribute from `otelpgx` may include query parameters. By default `otelpgx` does **not** include query parameters in spans. If this default ever changes in a future version, explicitly pass `otelpgx.WithIncludeQueryParameters(false)` when constructing the tracer. Review `otelpgx` release notes on upgrade.

---

## Files Modified Summary

| File | Change |
|------|--------|
| `go.mod` | Add 17 OTel packages (9 distinct Go modules + transitive deps) |
| `main.go` | +10 lines: init OTel, defer shutdown |
| `internal/config/config.go` | Add 7 OTel config fields + env parsing |
| `internal/telemetry/telemetry.go` | **New** — provider init/shutdown (~120 lines) |
| `internal/telemetry/config.go` | **New** — config struct (~20 lines) |
| `internal/telemetry/middleware.go` | **New** — bot tracing middleware (~120 lines) |
| `internal/telemetry/metrics.go` | **New** — metric instruments (~80 lines) |
| `internal/telemetry/httpclient.go` | **New** — HTTP transport wrapper (~15 lines) |
| `internal/database/database.go` | ~5 line change: add otelpgx tracer |
| `internal/bot/bot.go` | Add fields, register middleware, instrument background jobs |
| `internal/bot/handlers_commands.go` | Add metric recording on expense CRUD (~20 lines) |
| `internal/bot/handlers_receipt.go` | Add child spans for download/OCR |
| `internal/bot/handlers_voice.go` | Add child spans for voice parse |
| `internal/bot/reminder.go` | Add spans + metrics for reminder job |
| `internal/exchange/frankfurter_client.go` | Accept transport parameter |
| `internal/exchange/cached_service.go` | Add cache hit/miss counters |
| `internal/gemini/receipt_parser.go` | Add spans around GenerateContent |
| `internal/gemini/voice_parser.go` | Add spans around GenerateContent |
| `internal/gemini/category_suggester.go` | Add spans around GenerateContent |
| `internal/logger/logger.go` | Add WithTraceContext helper |

---

## Verification

1. **No regression**: Run `make test` with `OTEL_ENABLED` unset — all existing tests pass unchanged (noop providers)
2. **Stdout smoke test**: Set `OTEL_ENABLED=true OTEL_EXPORTER_TYPE=stdout`, send `/start` to bot, verify spans + metrics print to stdout
3. **Trace chain**: With stdout exporter, send `/add 5.50 Coffee` — verify root span (`telegram.command /add`) has child spans for DB queries
4. **External API traces**: Send a receipt photo — verify `gemini.generate_content` and `telegram.download_file` child spans
5. **Metrics**: Check stdout for `expense.operations`, `background.job.runs` counters
6. **Log correlation**: Check log output for `trace_id` and `span_id` fields on error logs
7. **OTLP export**: Point at a local collector/Jaeger, verify traces and metrics arrive
