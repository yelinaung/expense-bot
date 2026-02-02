# Fuzz Testing Plan

**Date**: 2026-02-02
**Status**: In Progress

---

## Overview

This document outlines the fuzz testing strategy for the expense-bot codebase. Fuzz testing helps discover edge cases, crashes, and security vulnerabilities by feeding random/malformed inputs to parsing functions.

---

## Priority 1: High-Risk Parsing Functions

| Function | File | Risk | Rationale |
|----------|------|------|-----------|
| `parseAmount` | `internal/bot/parser.go:52` | **HIGH** | Handles decimal parsing with comma/dot conversion; financial data |
| `ParseExpenseInput` | `internal/bot/parser.go:70` | **HIGH** | Complex regex + currency detection + user input |
| `extractJSON` | `internal/gemini/category_suggester.go:215` | **HIGH** | Extracts JSON from untrusted LLM output |
| `sanitizeDescription` | `internal/gemini/category_suggester.go:238` | **HIGH** | Security-critical prompt injection defense |

---

## Priority 2: Medium-Risk Functions

| Function | File | Risk | Rationale |
|----------|------|------|-----------|
| `parseReceiptResponse` | `internal/gemini/receipt_parser.go:160` | **MEDIUM** | JSON parsing + markdown stripping from LLM |
| `sanitizeReasoning` | `internal/gemini/category_suggester.go:261` | **MEDIUM** | Output sanitization |
| `ParseAddCommandWithCategories` | `internal/bot/parser.go:165` | **MEDIUM** | String suffix matching with categories |

---

## Priority 3: Low-Risk Functions

| Function | File | Risk | Rationale |
|----------|------|------|-----------|
| `SanitizeDescription` | `internal/logger/privacy.go:40` | **LOW** | Privacy redaction for logging |
| `SanitizeText` | `internal/logger/privacy.go:54` | **LOW** | Privacy redaction for logging |
| `HashUserID` | `internal/logger/privacy.go:24` | **LOW** | Hash generation |

---

## Fuzz Test Specifications

### 1. FuzzParseAmount

**Target**: `internal/bot/parser.go:parseAmount`

**Seed Corpus**:
- Valid: `"5.50"`, `"5,50"`, `"100"`, `"0.01"`, `"999999999.99"`
- Invalid: `"0"`, `"-10"`, `""`, `"abc"`, `"5.5.5"`, `"NaN"`, `"Inf"`

**Invariants**:
- If no error, amount must be positive (> 0)
- Must not panic on any input

### 2. FuzzParseExpenseInput

**Target**: `internal/bot/parser.go:ParseExpenseInput`

**Seed Corpus**:
- `"5.50 Coffee"`, `"$10 Lunch"`, `"50 USD Coffee"`
- `"S$5.50 Taxi"`, `"€100"`, `"¥1000 Ramen"`
- Edge cases: `""`, `"Coffee"`, `"$"`, `"-5 Invalid"`

**Invariants**:
- If result is non-nil, amount must be positive
- Currency (if set) must be in `models.SupportedCurrencies`
- Must not panic on any input

### 3. FuzzExtractJSON

**Target**: `internal/gemini/category_suggester.go:extractJSON`

**Seed Corpus**:
- Valid: `{"key": "value"}`, `Here is JSON: {"a": 1}`
- Nested: `{"nested": {"a": 1}}`, `{"arr": [1,2,3]}`
- Invalid: `{incomplete`, `no json`, `}backwards{`
- Tricky: `{"a": "}{"}`, `{ } { }`, `{{nested}}`

**Invariants**:
- If non-empty result, must start with `{` and end with `}`
- Must not panic on any input

### 4. FuzzSanitizeDescription

**Target**: `internal/gemini/category_suggester.go:sanitizeDescription`

**Seed Corpus**:
- Normal: `"Coffee Shop"`, `"Lunch at restaurant"`
- Injection: `"Coffee\" injection"`, `"Test\nNew line"`
- Control chars: `"Test\x00null"`, `"Tab\there"`
- Unicode: `"コーヒー"`, `"\u200B\u200C\u200D"` (zero-width)
- Long: 300+ character strings

**Invariants**:
- Result must not contain `"` (double quote)
- Result must not contain `\n`, `\r`, `\x00`
- Result length must be ≤ `MaxDescriptionLength` (200)
- Must not panic on any input

### 5. FuzzParseReceiptResponse

**Target**: `internal/gemini/receipt_parser.go:parseReceiptResponse`

**Seed Corpus**:
- Valid: `{"amount": "5.50", "merchant": "Shop"}`
- Markdown: `` ```json\n{"amount": "10"}\n``` ``
- Invalid: `{"amount": "abc"}`, `{}`, `not json`

**Invariants**:
- If no error, amount must be non-negative
- Must not panic on any input

### 6. FuzzSanitizeReasoning

**Target**: `internal/gemini/category_suggester.go:sanitizeReasoning`

**Seed Corpus**:
- Normal: `"Category matched well"`
- Whitespace: `"Test\ttab"`, `"Multi\n\nline"`
- Long: 600+ character strings

**Invariants**:
- Result must not contain `\n`, `\r`, `\t`
- Result length must be ≤ 500
- Must not panic on any input

---

## Running Fuzz Tests

### Local Development

```bash
# Run specific fuzz test for 30 seconds
go test -fuzz=FuzzParseAmount -fuzztime=30s ./internal/bot/...

# Run all fuzz tests in a package
go test -fuzz=Fuzz -fuzztime=1m ./internal/gemini/...

# Run with specific seed
go test -run=FuzzParseAmount/seed_corpus_entry ./internal/bot/...
```

### CI Integration

Add to `.gitlab-ci.yml`:

```yaml
fuzz-tests:
  stage: test
  script:
    - go test -fuzz=Fuzz -fuzztime=1m ./internal/bot/...
    - go test -fuzz=Fuzz -fuzztime=1m ./internal/gemini/...
  only:
    - schedules  # Run on nightly schedule
```

---

## Crash Handling

Fuzz test crashes are stored in `testdata/fuzz/<FuzzTestName>/` directories. When a crash is found:

1. The failing input is saved as a file in the corpus
2. Running `go test` will replay all corpus entries
3. Fix the bug and verify the corpus entry passes

---

## Expected Discoveries

- Decimal parsing edge cases (overflow, precision loss)
- Unicode handling issues in regex patterns
- Malformed JSON extraction edge cases
- Buffer/length boundary issues
- Panic conditions in error paths

---

## Implementation Checklist

- [x] Document fuzz testing plan
- [x] Implement Priority 1 fuzz tests
  - [x] `FuzzParseAmount` - `internal/bot/parser_fuzz_test.go`
  - [x] `FuzzParseExpenseInput` - `internal/bot/parser_fuzz_test.go`
  - [x] `FuzzExtractJSON` - `internal/gemini/category_suggester_fuzz_test.go`
  - [x] `FuzzSanitizeDescription` - `internal/gemini/category_suggester_fuzz_test.go`
- [x] Implement Priority 2 fuzz tests
  - [x] `FuzzSanitizeReasoning` - `internal/gemini/category_suggester_fuzz_test.go`
  - [x] `FuzzParseAddCommandWithCategories` - `internal/bot/parser_fuzz_test.go` (as `FuzzParseExpenseInputWithCategories`)
  - [ ] `FuzzParseReceiptResponse` - requires export or separate test file
- [x] Additional fuzz tests
  - [x] `FuzzParseAddCommand` - `internal/bot/parser_fuzz_test.go`
  - [x] `FuzzHashDescription` - `internal/gemini/category_suggester_fuzz_test.go`
- [ ] Add CI integration
- [x] Document bugs found (see below)

## Bugs Found and Fixed

### 1. `extractJSON` - Incomplete JSON Handling
**Issue**: When input started with `{` but had no closing `}`, function returned the incomplete string.
**Fix**: Changed condition from `end < start` to `end <= start` and removed early return for `{` prefix.

### 2. `sanitizeReasoning` / `sanitizeDescription` - Trailing Whitespace After Truncation
**Issue**: Truncating at max length could leave trailing whitespace if cut happened mid-word.
**Fix**: Added `strings.TrimSpace()` after truncation.

---

## References

- [Go Fuzzing Documentation](https://go.dev/doc/security/fuzz/)
- [Go Fuzzing Tutorial](https://go.dev/doc/tutorial/fuzz)
