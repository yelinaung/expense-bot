# LLM Provider Abstraction Plan

## Objective

Enable swapping Gemini with other LLM providers (OpenAI, Anthropic/Claude,
etc.) for these bot capabilities without changing handler behavior unless
explicitly called out as an intentional improvement:

- Receipt OCR extraction from photos.
- Voice expense extraction from voice messages.
- Category suggestion for text expense entries.

Provider selection must be configuration-driven.

User decisions captured (2026-03-06):

- Receipt parsing should return a specific no-data error message for
  `ErrNoData`.
- Providers should be treated as all-or-nothing for the full feature set.
- Two-step voice flow is acceptable if transparent to end users.
- Do not rely on API key pattern detection for provider routing.
- Official SDKs are preferred unless blocked by feature gaps.

## Current State Summary

Gemini is currently a concrete dependency in runtime and handlers.

- `internal/bot/bot.go`: `Bot` stores `*gemini.Client`, initialized via
  `initGeminiClient`.
- `internal/bot/handlers_receipt.go`: calls `ParseReceipt`, branches on
  `gemini.ErrParseTimeout` only.
- `internal/bot/handlers_voice.go`: calls `ParseVoiceExpense`, branches on
  `gemini.ErrVoiceParseTimeout` and `gemini.ErrNoVoiceData`.
- `internal/bot/handlers_commands.go`: calls `SuggestCategory`.
- `internal/config/config.go`: has `GeminiAPIKey` only.
- `internal/gemini/*`: owns parsing types/errors, prompt sanitization,
  category defaults, tracing/logging helpers, and test seams.

## Scope

In scope:

- Introduce provider-agnostic LLM contract and shared domain types.
- Refactor bot wiring to depend on interfaces, not Gemini concrete type.
- Add factory + config-driven provider/model/parameter selection.
- Keep Gemini as first adapter, then add OpenAI and Anthropic.
- Preserve security controls (sanitization, redaction, bounded parsing).
- Preserve existing behavior first, then document intentional deltas.

Out of scope:

- Intelligent multi-provider routing or cost-based selection.
- Runtime admin commands to switch providers dynamically.

## Architectural Decisions

### 1) Shared package ownership

Create `internal/llm` as provider-agnostic home for:

- Core types:
  - `ReceiptData`
  - `VoiceExpenseData`
  - `CategorySuggestion`
- Shared errors:
  - `ErrParseTimeout`
  - `ErrNoData`
  - `ErrVoiceParseTimeout`
  - `ErrNoVoiceData`
  - `ErrProviderMisconfigured`
- Shared sanitization helpers moved from `internal/gemini`:
  - `SanitizeForPrompt`
  - `SanitizeCategoryName`
- Shared defaults moved from Gemini:
  - `DefaultCategories`
- Shared privacy-safe logging helper moved from Gemini pattern:
  - description hashing helper (equivalent to `hashDescription` behavior)

This removes residual `gemini` imports from handlers after migration.

### 2) Client shape and lifecycle

Use a single all-features interface plus lifecycle:

```go
type Client interface {
    ParseReceipt(ctx context.Context, imageBytes []byte, mimeType string) (*ReceiptData, error)
    ParseVoiceExpense(ctx context.Context, audioBytes []byte, mimeType string, categories []string) (*VoiceExpenseData, error)
    SuggestCategory(ctx context.Context, description string, categories []string) (*CategorySuggestion, error)
    Close() error
}
```

Rationale:

- Enforces all-or-nothing provider requirement.
- Supports underlying SDK cleanup via `Close()`.

### 3) Provider capability policy

All-or-nothing policy:

- Selected provider must satisfy receipt + voice + category methods.
- If provider/model choice cannot satisfy all three, bot should not initialize
  LLM features for that provider selection.

### 4) Timeout and generation-parameter ownership

Introduce shared operation settings in config with provider mapping:

- `LLM_TIMEOUT_RECEIPT` (default 30s)
- `LLM_TIMEOUT_VOICE` (default 15s)
- `LLM_TIMEOUT_CATEGORY` (default 10s)
- `LLM_TEMP_RECEIPT` (optional)
- `LLM_TEMP_VOICE` (optional)
- `LLM_TEMP_CATEGORY` (default 0.2)

Adapters own provider-specific mapping of these settings.

### 5) Response strategy by provider (explicit)

Adapters should prefer provider-native structured output features where
available, with raw JSON extraction fallback only if needed:

- Gemini: current JSON-in-text + extraction/sanitization flow.
- OpenAI: structured output/JSON mode for categorization and receipt metadata;
  voice uses transparent two-step flow (transcribe then extract).
- Anthropic: tool/JSON schema style for structure; if voice is unsupported
  directly, use transparent two-step transcription + extraction flow.

All adapters must normalize outputs to identical domain behavior.

### 6) Prompt portability

Prompts are provider-specific assets.

- Keep current Gemini prompts unchanged in Gemini adapter.
- Create provider-specific prompt templates for OpenAI/Anthropic.
- Keep common intent and output schema aligned in shared tests.

### 7) Logging/privacy parity

Replicate privacy-safe logging behavior across adapters:

- hash user descriptions before logging (Gemini-equivalent pattern).
- never log raw receipt/voice payloads.
- never log secrets.

## Configuration

Add fields:

- `LLM_PROVIDER`: `gemini` (default), `openai`, `anthropic`.
- `LLM_API_KEY`: optional generic override.
- Provider keys:
  - `GEMINI_API_KEY`
  - `OPENAI_API_KEY`
  - `ANTHROPIC_API_KEY`
- Models:
  - `LLM_MODEL_RECEIPT`
  - `LLM_MODEL_VOICE`
  - `LLM_MODEL_CATEGORY`
- Timeouts and temperatures (section above).

### API key precedence (explicit)

1. `LLM_API_KEY` (highest precedence)
2. Provider-specific key for selected provider
3. If missing, LLM initialization fails for the selected provider

If both generic and provider-specific keys exist and differ, use `LLM_API_KEY`
and log a startup warning without printing values.

Provider routing policy:

- Provider is selected by explicit `LLM_PROVIDER`.
- Key pattern detection is not used for routing decisions.

## Migration Plan (Phased)

### Phase 0: Baseline and acceptance criteria

- Freeze existing behavior with focused tests for receipt/voice/category flows.
- Document existing receipt error UX behavior and apply intentional delta:
  - add explicit `ErrNoData` branch with improved user message.

### Phase 1: Extract shared llm domain and utilities

- Create `internal/llm` package for types/errors/sanitizers/default categories.
- Move privacy hash helper to shared location.
- Keep behavior unchanged except intentional `ErrNoData` UX update.

### Phase 2: Introduce interfaces and bot decoupling

- Replace `*gemini.Client` dependency with `llm.Client`.
- Add `Close()` lifecycle handling in shutdown path.
- Refactor handlers to use shared errors.
- Update receipt handler to branch on shared `ErrNoData` with specific message.

### Phase 3: Factory + config

- Replace `initGeminiClient` with `initLLMClient`.
- Implement provider selection, key/model/time/temp resolution.
- Add startup telemetry/log metadata for provider/model settings.

### Phase 4: Gemini adapter stabilization

- Wrap existing Gemini behavior under new interface.
- Preserve test seam equivalent to `ContentGenerator` injection.

Test seam decision:

- Keep adapter-local generator interfaces for provider unit tests.
- Bot tests stop constructing provider clients directly and instead mock
  `internal/llm` interface.

### Phase 5: OpenAI adapter

- Receipt parsing via vision-capable flow.
- Voice parsing via transparent two-step flow:
  - transcribe audio
  - extract structured expense fields
- Category suggestion via structured output.

### Phase 6: Anthropic adapter

- Receipt parsing via image-capable flow.
- Voice parsing via transparent two-step transcription + extraction flow.
- Category suggestion via structured output/tooling.

Gate:

- Anthropic adapter is enabled only when all three operations can be satisfied
  by the selected Anthropic strategy.

### Phase 7: Hardening and rollout

- Add retries/rate-limit handling where safe.
- Validate observability dimensions and privacy logging.
- Staged rollout with provider-specific smoke tests.

## Testing Plan

### 1) Bot test migration impact

Current bot tests rely on `gemini.NewClientWithGenerator(...)` in places.
Those tests will be updated to use `llm.Client` mocks/fakes instead.

Acceptance condition:

- No bot tests instantiate provider SDK clients directly.

### 2) Adapter contract tests

Build shared contract tests each adapter must pass:

- timeout mapping
- empty/no-data mapping
- malformed response handling
- sanitization invariants
- confidence bounds and category mapping behavior

### 3) Provider-specific tests

- Gemini: preserve current test depth.
- OpenAI/Anthropic: unit tests around request construction, response parsing,
  structured output handling, and fallback paths.
- Integration tests behind env gates/build tags.

### 4) Regression/security tests

- Prompt-injection regression tests for all adapters.
- Control-character and oversized input tests.
- Raw payload and key redaction verification in logs.

### 5) Required repository test commands

Per project instructions, run all:

- `make test`
- `make test-coverage` (coverage must stay >= 50%)
- `make test-race`
- `make test-integration`

## Security Requirements

- Preserve and centralize prompt/input sanitization in shared `internal/llm`.
- Enforce schema/structured-output validation with strict bounds checks.
- Keep provider endpoint TLS and bounded timeout/payload limits.
- Ensure redacted logging and hash-based description logging parity.
- Validate adapter output before persistence/display.

## Observability Requirements

Add/standardize these attrs/metrics:

- attrs:
  - `llm.provider`
  - `llm.model`
  - `llm.operation`
  - `llm.input_size_bytes`
- metrics:
  - call count by provider/model/operation
  - latency histogram by provider/operation
  - error count by provider/error type
  - timeout count

## Dependency Plan

Expected SDK additions:

- OpenAI official SDK preferred: `github.com/openai/openai-go`
- Anthropic official SDK preferred: `github.com/anthropics/anthropic-sdk-go`

Community alternatives (only if official SDK blocks required features):

- OpenAI community SDK: `github.com/sashabaranov/go-openai`
- Anthropic community SDK: `github.com/liushuangls/go-anthropic`

Also verify:

- HTTP middleware/telemetry compatibility for new SDK clients.
- module size and transitive dependency impact.

## Effort Estimate

Assuming one engineer:

- Phase 0-2: 1.5-2.0 days.
- Phase 3-4: 1.0-1.5 days.
- Phase 5 (OpenAI): 2.0-2.5 days.
- Phase 6 (Anthropic): 2.0-2.5 days.
- Phase 7 hardening and rollout: 1.0 day.

Total: 7.5-9.5 engineer-days.

## Risks and Mitigations

- Provider output variability.
  - Mitigation: adapter contract tests + native structured outputs.
- Voice capability mismatch across providers.
  - Mitigation: transparent two-step transcription path and all-features gate
    at provider init.
- Behavior regression in handler UX.
  - Mitigation: freeze tests before refactor and preserve messages except
    intentional `ErrNoData` improvement.
- Privacy regression in logs.
  - Mitigation: shared hashed logging utilities and redaction tests.

## Provider Support Snapshot (To Verify During Implementation)

- Gemini (`gemini-2.5-flash`): expected to support receipt, voice, category.
- OpenAI: expected to support full set with two-step voice flow.
- Anthropic: text+image are straightforward; voice likely requires a
  transcription step and must still satisfy all-features requirement.

This matrix must be validated against current provider docs and tested with
selected model IDs before enabling each provider in production.

## Open Questions

1. Anthropic voice policy: if Anthropic needs non-Anthropic transcription to
   satisfy voice, do we still classify this as `LLM_PROVIDER=anthropic`?
2. Receipt no-data wording: final user-facing copy for `ErrNoData` message.
3. SDK fallback policy: what evidence threshold allows switching from official
   to community SDK for a provider.

## Acceptance Criteria

- Provider can be switched by config only.
- Bot has no direct imports of provider packages in handlers.
- Selected provider supports all receipt/voice/category operations or bot
  disables LLM features consistently.
- All required test commands pass and coverage >= 50%.
- Security controls and logging redaction parity are maintained.
- Docs updated (`README`, `.env.example`, `docs/PRIVACY.md`).
