# Security Assessment: Prompt Injection and LLM Output Safety

**Date**: 2026-02-02
**Last Updated**: 2026-02-13
**Classification**: Internal Security Review
**Status**: Active, mitigations implemented, residual risk tracked

---

## Executive Summary

This document reviews prompt-injection and output-safety controls for Gemini-backed features in Expense Bot.

Current codebase status:
- Direct prompt-injection risk in category suggestion is **mitigated**.
- Category-list injection risk in receipt and voice parsing is **mitigated**.
- Output parsing/validation controls are **partially hardened** and functioning.
- Residual risk remains for multimodal adversarial content and operational monitoring gaps.

---

## Scope

Reviewed components:
- `internal/gemini/category_suggester.go`
- `internal/gemini/receipt_parser.go`
- `internal/gemini/voice_parser.go`
- `internal/gemini/category_suggester_test.go`
- `internal/gemini/category_suggester_fuzz_test.go`
- `internal/gemini/receipt_parser_test.go`
- `internal/gemini/voice_parser_test.go`

---

## Findings and Current State

### 1) Category suggestion prompt injection

**Original risk**: User-supplied description could alter model behavior when embedded in prompt text.

**Current controls (implemented):**
- Input is sanitized before prompt construction via `sanitizeDescription()` / `SanitizeForPrompt()`.
- Sanitization removes/normalizes injection-relevant formatting:
  - double quotes and backticks replaced,
  - null bytes removed,
  - whitespace collapsed (`strings.Fields`),
  - length capped (`MaxDescriptionLength = 200`).
- Model output is constrained and validated:
  - JSON response schema configured with required fields,
  - category must match provided whitelist (case-insensitive check with canonical rewrite),
  - confidence range enforced (`0.0 <= confidence <= 1.0`),
  - reasoning sanitized via `sanitizeReasoning()` (normalized and max 500 chars).
- Logging uses `description_hash` rather than raw description content.

**Assessment**: **Mitigated (Low residual risk)**.

### 2) Receipt and voice prompt category injection

**Risk**: Category names might carry injected instructions that influence model behavior.

**Current controls (implemented):**
- Category names are sanitized with `SanitizeCategoryName()` before being inserted in prompts.
- Prompts explicitly instruct the model that the category list is system data, not instructions.
- Parsed outputs sanitize merchant/description/currency/category fields before returning.

**Assessment**: **Mitigated (Low residual risk)**.

### 3) Output handling and parser robustness

**Risk**: Non-JSON wrappers, malformed output, or suspicious response text could bypass assumptions.

**Current controls (implemented):**
- Category suggestion extracts JSON envelope (`extractJSON`) before unmarshalling.
- Receipt/voice parsers strip markdown fences and unmarshal typed JSON structures.
- Invalid category/confidence values are rejected in category suggestion path.

**Limitations / residual concerns:**
- `extractJSON` is intentionally simple and may still be brittle for edge-case nested brace text.
- Receipt/voice paths rely on schema-like prompt instructions but do not enforce a strict enum check against runtime categories post-parse.

**Assessment**: **Partially mitigated (Medium residual risk)**.

### 4) Multimodal prompt injection (receipt images / audio)

**Risk**: Adversarial visual/audio content can still steer model behavior even with text-side sanitization.

**Current controls:**
- Prompt hardening language and output sanitation.
- Timeout and no-data handling for parser paths.

**Remaining gap:**
- No dedicated adversarial-content detection/pre-filtering layer.

**Assessment**: **Accepted residual risk (Medium)**.

---

## Test Evidence in Repository

Relevant automated coverage includes:
- `TestSuggestCategory_PromptInjection`
- `TestSanitizeDescription`
- `TestSanitizeReasoning`
- `TestSanitizeForPrompt`
- `TestSanitizeCategoryName`
- `TestSuggestCategory_ConfidenceValidation`
- `FuzzSanitizeDescription`
- `FuzzSanitizeReasoning`
- `FuzzSanitizeCategoryName`
- `FuzzExtractJSON`
- `TestBuildReceiptPrompt_SanitizesCategories`
- `TestBuildVoiceExpensePrompt_SanitizesCategories`

These tests validate primary injection vectors (quote/newline/delimiter/control-char patterns), boundary conditions, and sanitizer invariants.

---

## Risk Matrix (Current)

| Component | Risk Level | Status |
|---|---|---|
| Category Suggestion (text prompt injection) | Low | Mitigated |
| Receipt Parser (category-list injection) | Low | Mitigated |
| Voice Parser (category-list injection) | Low | Mitigated |
| LLM Output Parsing Robustness | Medium | Partially Mitigated |
| Multimodal Adversarial Content | Medium | Residual |
| Debug Log Data Exposure | Low | Mitigated |

---

## Recommended Next Steps

1. Enforce post-parse category whitelist checks in receipt/voice paths, similar to category suggestion.
2. Add structured metrics/alerting for malformed model output and repeated parser failures.
3. Add targeted red-team tests for adversarial receipt/audio samples.
4. Consider stronger JSON extraction/parsing hardening in category suggestion for complex malformed responses.

---

## Review Cadence

- Re-review after any prompt/LLM model change or parser refactor.
- Otherwise, review quarterly.
- Next scheduled review target: 2026-05-13.

---

**Document Owner**: Security Team
**Reviewers**: Engineering, AI/ML
**Distribution**: Internal Only
