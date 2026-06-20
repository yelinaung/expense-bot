# Hegel Property-Based Testing Findings

**Date**: 2026-06-18
**Status**: Fixed — 3 bugs identified and patched; regression tests (Hegel PBTs + deterministic unit tests) added and passing

---

## Overview

A pass of [Hegel](https://hegel.dev) property-based tests (PBTs) over
previously unit-tested-only modules surfaced **3 real bugs**. Each bug was
demonstrated by a Hegel test encoding the *correct* contract; the test
initially failed on the buggy code and Hegel shrunk the input to a minimal
counterexample. All three bugs are now fixed; the Hegel tests plus
deterministic regression unit tests pass and serve as permanent regression
markers.

The tests were added alongside the existing tests for the code under test
(see `AGENTS.md` / Hegel workflow — "Property-based tests belong with the
code they're testing"):

| File | Tests added |
|------|-------------|
| `internal/logger/privacy_rapid_test.go` | 3 |
| `internal/bot/chart_generator_hegel_test.go` | 2 (new file) |
| `internal/bot/csv_generator_rapid_test.go` | 2 (appended) |

All 7 tests now pass after the fixes below; `go vet` and `gofumpt` are clean.
Run them with:

```sh
GOCACHE=$PWD/.cache/go-build go test ./internal/logger/ ./internal/bot/ \
  -run 'TestHegelSanitizeText|TestHegelAggregateByCategory|TestHegelGenerateExpensesCSVEmptyNameIsUncategorized|TestHegelGenerateExpensesCSVCategoryColumnConsistentWithHabitCategoryName'
```

---

## Bug 1 — `SanitizeText` emits invalid UTF-8

**Severity**: Medium (corrupts logs, can break log parsers / aggregators that
require valid UTF-8).

**File**: `internal/logger/privacy.go:78`

```go
// For longer text, show prefix and length
return fmt.Sprintf("%s...<%d chars>", text[:3], len(text))
```

### Root cause

The "long text" branch slices `text[:3]` by **byte index**, not by rune.
When the first 3 bytes fall inside a multi-byte UTF-8 sequence (any non-ASCII
character that encodes to 4 bytes, e.g. `𐀀`, `😀`, or a 3-byte CJK char
preceded by 1 ASCII char), the slice cuts a rune in half and the resulting
string is no longer valid UTF-8.

A secondary issue: the label says `<N chars>` but `len(text)` reports the
**byte** count, not the rune count — so the docstring ("show prefix and
length") and the label disagree for non-ASCII input.

### Why existing tests missed it

`internal/logger/privacy_test.go` only exercises ASCII inputs
(`"short"`, `"this is a long text"`). ASCII never spans a multi-byte
boundary, so byte-index slicing happens to be rune-aligned.

### Hegel counterexamples

Hegel shrinks each failure to a minimal input:

| Test | Minimal input | Output |
|------|---------------|--------|
| `TestHegelSanitizeTextOutputIsValidUTF8` | `"0𐀀\u0080𐀀"` | `"0\xf0\x9..."` — invalid UTF-8 |
| `TestHegelSanitizeTextLongInputValidUTF8` | `"00\u008000000000"` (11 bytes) | `"00..."` prefix splits the rune |
| `TestHegelSanitizeTextPrefixIsRuneAligned` | same as above | prefix `"00\..."` is not rune-aligned |

### Fix

Applied in `internal/logger/privacy.go`: slice by rune and report the rune
count so the output stays valid UTF-8 and the `<N chars>` label matches its
content.

```go
if len(text) <= 10 {
    return fmt.Sprintf("<%d chars>", utf8.RuneCountInString(text))
}
runes := []rune(text)
return fmt.Sprintf("%s...<%d chars>", string(runes[:3]), len(runes))
```

### Tests

`internal/logger/privacy_rapid_test.go`:

- `TestHegelSanitizeTextOutputIsValidUTF8` — `utf8.ValidString(SanitizeText(in))`
  for any `hegel.Text()` input.
- `TestHegelSanitizeTextLongInputValidUTF8` — focused on inputs ≥ 11 bytes
  (the branch where the bug lives).
- `TestHegelSanitizeTextPrefixIsRuneAligned` — the sharper property: the
  exposed prefix must be a whole number of runes.

---

## Bug 2 — `aggregateByCategory` keys empty-named categories as `""`

**Severity**: Low–Medium (incorrect chart grouping; empty category appears as
a blank legend entry / unlabeled slice).

**File**: `internal/bot/chart_generator.go:75`

```go
categoryName := categoryUncategorized
if expenses[i].Category != nil {
    categoryName = expenses[i].Category.Name
}
```

### Root cause

The function treats "no category" as *only* a nil `Category` pointer. A
non-nil `Category` whose `Name` is the empty string falls through and becomes
the map key `""` instead of `"Uncategorized"`.

This is **inconsistent** with the rest of the bot: `habitCategoryName`
(`internal/bot/habit_analyzer.go:117`) explicitly falls back to
`categoryUncategorized` when `Category.Name == ""`, and
`GenerateExpensesCSV` (Bug 3 below) shares the same defect. Two code paths
that classify an expense's category therefore disagree on what "no category"
means.

### Why existing tests missed it

`internal/bot/chart_generator_test.go` only constructs categories with
non-empty names (`testCategoryFoodGroceries`, `testCategoryFoodDiningOut`)
or `nil`. The `Category{Name: ""}` case is never exercised.

### Hegel counterexample

Hegel shrinks to the empty-name case directly:

```
TestHegelAggregateByCategoryEmptyNameIsUncategorized
  expected: map[string]decimal.Decimal{"Uncategorized": ...}
  actual  : map[string]decimal.Decimal{"": ...}
```

i.e. a nil `Category` aggregates under `"Uncategorized"` but a
`&Category{Name: ""}` aggregates under `""`.

### Fix

Applied in `internal/bot/chart_generator.go`: also guard against an empty
`Name` so empty-named categories fall back to `categoryUncategorized`,
matching `habitCategoryName`.

```go
categoryName := categoryUncategorized
if expenses[i].Category != nil && expenses[i].Category.Name != "" {
    categoryName = expenses[i].Category.Name
}
```

### Tests

`internal/bot/chart_generator_hegel_test.go`:

- `TestHegelAggregateByCategoryConsistentWithHabitCategoryName` — for any
  `*Category` (nil / empty-name / non-empty-name), the map key used by
  `aggregateByCategory` must equal `habitCategoryName(&expense)`.
- `TestHegelAggregateByCategoryEmptyNameIsUncategorized` — the focused
  counterexample: nil and empty-name must aggregate identically, and the key
  must be `categoryUncategorized`.

A shared generator `hegelCategoryOrNilGen` draws all three cases (nil,
empty-name, non-empty-name) so the same coverage feeds Bug 3's tests too.

---

## Bug 3 — `GenerateExpensesCSV` writes an empty Category cell

**Severity**: Low–Medium (CSV export shows a blank category column for
empty-named categories, inconsistent with `nil` which shows
`"Uncategorized"`; downstream consumers may treat the empty cell
differently from the sentinel).

**File**: `internal/bot/csv_generator.go:67`

```go
categoryName := categoryUncategorized
if expenses[i].Category != nil {
    categoryName = expenses[i].Category.Name
}
```

### Root cause

Identical to Bug 2: the `Category != nil` check does not account for an
empty `Name`. The CSV's Category column therefore diverges from
`habitCategoryName`'s notion of an expense's category.

### Why existing tests missed it

`internal/bot/csv_generator_rapid_test.go` had Hegel tests for the CSV
generator (`TestHegelGenerateExpensesCSVStructure`,
`TestHegelGenerateExpensesCSVNeutralizesFormulas`, …) but every generated
expense left `Category` unset (i.e. `nil`). The empty-name branch was never
hit.

### Hegel counterexample

```text
TestHegelGenerateExpensesCSVEmptyNameIsUncategorized
  nil Category     -> Category cell = "Uncategorized"
  &Category{Name:""} -> Category cell = ""          ← diverges
```

### Fix

Applied in `internal/bot/csv_generator.go`: the same one-line guard as Bug 2
(check `Name != ""`), so all three code paths (chart / CSV / habit analyzer)
now agree on "no category" → `"Uncategorized"`.

### Tests

Appended to `internal/bot/csv_generator_rapid_test.go`:

- `TestHegelGenerateExpensesCSVEmptyNameIsUncategorized` — focused
  counterexample: nil and empty-name must produce the same Category cell,
  and that cell must be `categoryUncategorized`.
- `TestHegelGenerateExpensesCSVCategoryColumnConsistentWithHabitCategoryName`
  — for any `*Category`, the CSV Category column equals
  `habitCategoryName(&expense)`.

A small helper `csvCategoryColumn(t, expense)` renders a single-expense CSV
and returns the 7th column so both tests share the rendering logic.

---

## Summary

| # | Bug | Location | Root cause | Tests |
|---|-----|----------|------------|-------|
| 1 | `SanitizeText` returns invalid UTF-8 | `internal/logger/privacy.go:78` | byte slice `text[:3]` splits a multi-byte rune; `<N chars>` label reports byte count | 3 |
| 2 | `aggregateByCategory` keys empty-named categories as `""` | `internal/bot/chart_generator.go:75` | `Category != nil` check misses `Name == ""`; diverges from `habitCategoryName` | 2 |
| 3 | `GenerateExpensesCSV` writes empty Category cell | `internal/bot/csv_generator.go:67` | same `Category != nil`-only check as Bug 2 | 2 |

Bugs 2 and 3 share a root cause and a one-line fix. Bug 1 is independent.
All three are now patched and covered by passing regression tests.

### Notes on the testing approach

- **Generators stayed broad.** Per Hegel's "Generator Discipline", no bounds
  were added to avoid the bugs — `hegel.Text()` is full Unicode, and
  `hegelCategoryOrNilGen` deliberately includes the empty-name case. The
  edge cases are the *point*, not something to filter out.
- **Properties are evidence-based.** Each property is grounded in the code's
  own contract: `SanitizeText`'s docstring ("show prefix and length"),
  `habitCategoryName`'s explicit fallback, and the existing
  `"Uncategorized"` sentinel.
- **No new test files where one existed.** The logger PBTs join a new file
  (no rapid test existed there); the CSV PBTs were appended to the existing
  `csv_generator_rapid_test.go`; the chart PBTs live in a new file because
  no Hegel/rapid test file for `chart_generator.go` existed.
- **The tests are kept as regression markers.** They initially failed on the
  buggy code; after the fixes they pass and remain in the suite to prevent
  regressions. Each bug also has a focused deterministic unit test
  (`TestSanitizeText/"long multi-byte text produces valid UTF-8 prefix"`,
  `TestAggregateByCategory/"treats empty category name as Uncategorized"`,
  `TestGenerateExpensesCSV/"treats empty category name as Uncategorized"`)
  so the fix is locked in without relying solely on the property harness.
