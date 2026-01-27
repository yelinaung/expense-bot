# Test Coverage Improvement Plan

**Current Status**: 39.5% (with integration tests) → **Target**: 80%
**Gap**: 40.5 percentage points to close

## Executive Summary

### Current Coverage by Package

| Package | Unit Tests Only | With Integration | Target | Gap |
|---------|----------------|------------------|--------|-----|
| **internal/bot** | 15.4% | ~40-45% | 80% | 35-40% |
| **internal/config** | 100% | 100% | 100% | ✅ |
| **internal/database** | 29.4% | 85.7% | 90% | 4.3% |
| **internal/gemini** | 92.1% | 92.1% | 95% | 2.9% |
| **internal/logger** | 100% | 100% | 100% | ✅ |
| **internal/models** | N/A | N/A | N/A | N/A |
| **internal/repository** | 0% | 85.7% | 90% | 4.3% |
| **TOTAL** | 28.3% | 39.5% | 80% | 40.5% |

### Root Cause Analysis

**Why Coverage is Low:**
1. ❌ **Integration tests are skipped in unit runs** (no TEST_DATABASE_URL)
   - All handler tests: 0% coverage in unit mode
   - All repository tests: 0% coverage in unit mode
   - Impact: ~60% of codebase untested in CI

2. ❌ **Bot initialization and lifecycle not tested**
   - `New()`, `Start()`, `registerHandlers()`: 0%
   - Cleanup routines: 0%
   - Middleware: 0%

3. ❌ **Handler functions need more test scenarios**
   - Error paths not tested
   - Edge cases missing
   - Callback flow coverage incomplete

---

## Phase 1: Quick Wins (5-10 hours) → Target: 55%

### Priority 1A: Add Unit Tests for Bot Core (Effort: S)
**Impact**: +5-8% coverage

**Files to test:**
- `internal/bot/bot.go` (currently 15%)

**Tests to add:**

```go
// TestNew - Test bot initialization
func TestNew(t *testing.T) {
    t.Run("with valid config", func(t *testing.T) {
        // Test successful bot creation
    })

    t.Run("without Gemini key", func(t *testing.T) {
        // Test bot works without OCR
    })

    t.Run("with invalid token", func(t *testing.T) {
        // Test error handling
    })
}

// TestRegisterHandlers - Verify all handlers registered
func TestRegisterHandlers(t *testing.T) {
    // Mock bot, call registerHandlers, verify all routes
}

// TestGetCategoriesWithCache
func TestGetCategoriesWithCache(t *testing.T) {
    t.Run("cache miss", func(t *testing.T) {
        // First call fetches from DB
    })

    t.Run("cache hit", func(t *testing.T) {
        // Second call uses cache
    })

    t.Run("cache expiry", func(t *testing.T) {
        // After TTL, refetches from DB
    })
}

// TestInvalidateCategoryCache
func TestInvalidateCategoryCache(t *testing.T) {
    // Verify cache clears
}
```

**Estimated effort**: 2-3 hours
**Lines of test code**: ~200-300

---

### Priority 1B: Increase Handler Test Coverage (Effort: M)
**Impact**: +15-20% coverage

**Current gaps in `handlers_test.go`:**
- Error paths not tested (DB failures, invalid input)
- Edge cases (nil checks, empty responses)
- Callback flows incomplete

**Tests to add:**

```go
// Error scenarios for each handler
func TestHandleAdd_Errors(t *testing.T) {
    t.Run("invalid format", func(t *testing.T) {...})
    t.Run("database failure", func(t *testing.T) {...})
    t.Run("category not found", func(t *testing.T) {...})
}

// Receipt flow edge cases
func TestHandlePhoto_EdgeCases(t *testing.T) {
    t.Run("nil photo array", func(t *testing.T) {...})
    t.Run("download failure", func(t *testing.T) {...})
    t.Run("Gemini timeout", func(t *testing.T) {...})
    t.Run("invalid OCR response", func(t *testing.T) {...})
}

// Callback query error handling
func TestHandleEditCallback_Errors(t *testing.T) {
    t.Run("expense not found", func(t *testing.T) {...})
    t.Run("permission denied", func(t *testing.T) {...})
    t.Run("invalid callback data", func(t *testing.T) {...})
}

// Category management
func TestHandleSetCategoryCallback_Complete(t *testing.T) {
    t.Run("set existing category", func(t *testing.T) {...})
    t.Run("create new category", func(t *testing.T) {...})
    t.Run("duplicate category", func(t *testing.T) {...})
    t.Run("cache invalidation", func(t *testing.T) {...})
}
```

**Estimated effort**: 4-5 hours
**Lines of test code**: ~600-800

---

### Priority 1C: Repository Edge Cases (Effort: S)
**Impact**: +3-5% coverage

**Current**: 85.7% → **Target**: 90%

**Missing tests:**
```go
func TestExpenseRepository_Errors(t *testing.T) {
    t.Run("Create with nil expense", func(t *testing.T) {...})
    t.Run("Update non-existent", func(t *testing.T) {...})
    t.Run("Delete non-existent", func(t *testing.T) {...})
    t.Run("GetByID with invalid ID", func(t *testing.T) {...})
}

func TestCategoryRepository_Errors(t *testing.T) {
    t.Run("Create duplicate", func(t *testing.T) {...})
    t.Run("Update non-existent", func(t *testing.T) {...})
    t.Run("GetByName not found", func(t *testing.T) {...})
}

func TestUserRepository_Upsert_EdgeCases(t *testing.T) {
    t.Run("very long username", func(t *testing.T) {...})
    t.Run("special characters", func(t *testing.T) {...})
    t.Run("update all fields to empty", func(t *testing.T) {...})
}
```

**Estimated effort**: 2 hours
**Lines of test code**: ~200-300

---

### Priority 1D: Database & Gemini Edge Cases (Effort: S)
**Impact**: +2-3% coverage

**Database** (29.4% → 90%):
```go
func TestRunMigrations_Idempotent(t *testing.T) {
    // Run twice, should succeed both times
}

func TestSeedCategories_CustomCategories(t *testing.T) {
    // Test with existing categories
}

func TestConnect_InvalidURL(t *testing.T) {
    // Test error handling
}
```

**Gemini** (92.1% → 95%):
```go
func TestGenerateContent_Errors(t *testing.T) {
    t.Run("API error", func(t *testing.T) {...})
    t.Run("nil response", func(t *testing.T) {...})
    t.Run("empty response", func(t *testing.T) {...})
}

func TestNewClient_InvalidKey(t *testing.T) {
    // Test with bad API key
}
```

**Estimated effort**: 1-2 hours
**Lines of test code**: ~150-200

---

## Phase 1 Summary
- **Total effort**: 9-12 hours
- **Coverage gain**: +25-36%
- **New coverage**: 64-75%
- **Tests to add**: ~1,150-1,600 lines

---

## Phase 2: Medium Effort (8-12 hours) → Target: 70%

### Priority 2A: Bot Lifecycle & Middleware (Effort: M)
**Impact**: +5-8% coverage

**Tests to add:**
```go
func TestStart(t *testing.T) {
    // Test bot start with context cancellation
}

func TestCleanupExpiredDrafts(t *testing.T) {
    // Create draft expenses, wait, verify cleanup
}

func TestStartDraftCleanupLoop(t *testing.T) {
    // Test cleanup loop with context cancellation
}

func TestWhitelistMiddleware(t *testing.T) {
    t.Run("whitelisted user allowed", func(t *testing.T) {...})
    t.Run("non-whitelisted blocked", func(t *testing.T) {...})
    t.Run("user registration", func(t *testing.T) {...})
}

func TestDefaultHandler(t *testing.T) {
    t.Run("photo message", func(t *testing.T) {...})
    t.Run("pending edit", func(t *testing.T) {...})
    t.Run("free text expense", func(t *testing.T) {...})
    t.Run("unknown input", func(t *testing.T) {...})
}

func TestDownloadPhoto(t *testing.T) {
    t.Run("successful download", func(t *testing.T) {...})
    t.Run("invalid file ID", func(t *testing.T) {...})
    t.Run("download timeout", func(t *testing.T) {...})
    t.Run("large file", func(t *testing.T) {...})
}
```

**Estimated effort**: 5-6 hours
**Lines of test code**: ~400-500

---

### Priority 2B: Handler Callback Flows (Effort: M)
**Impact**: +5-7% coverage

**Current gaps:**
- `handlers_callbacks.go`: Only partial coverage

**Tests to add:**
```go
func TestHandlePendingEdit_CompleteFlow(t *testing.T) {
    t.Run("amount edit", func(t *testing.T) {...})
    t.Run("category change", func(t *testing.T) {...})
    t.Run("concurrent edits", func(t *testing.T) {...})
    t.Run("expired edit", func(t *testing.T) {...})
}

func TestProcessAmountEdit_Validation(t *testing.T) {
    t.Run("valid amount", func(t *testing.T) {...})
    t.Run("negative amount", func(t *testing.T) {...})
    t.Run("zero amount", func(t *testing.T) {...})
    t.Run("very large amount", func(t *testing.T) {...})
    t.Run("non-numeric input", func(t *testing.T) {...})
}

func TestPromptCreateCategory(t *testing.T) {
    t.Run("valid category name", func(t *testing.T) {...})
    t.Run("empty name", func(t *testing.T) {...})
    t.Run("duplicate name", func(t *testing.T) {...})
    t.Run("very long name", func(t *testing.T) {...})
}
```

**Estimated effort**: 3-4 hours
**Lines of test code**: ~300-400

---

## Phase 2 Summary
- **Total effort**: 8-10 hours
- **Coverage gain**: +10-15%
- **New coverage**: 74-90%
- **Tests to add**: ~700-900 lines

---

## Phase 3: Complete Coverage (10-15 hours) → Target: 80%+

### Priority 3A: Integration Test Improvements (Effort: L)
**Impact**: +5-10% coverage

**Current issues:**
- Integration tests good but could cover more scenarios
- Transaction rollback testing missing
- Concurrent access scenarios untested

**Tests to add:**
```go
func TestConcurrentExpenseCreation(t *testing.T) {
    // Multiple goroutines creating expenses
}

func TestTransactionIsolation(t *testing.T) {
    // Test database transaction behavior
}

func TestDatabaseConnectionPooling(t *testing.T) {
    // Test pgxpool behavior under load
}

func TestMigrationRollback(t *testing.T) {
    // Test what happens if migration fails mid-way
}
```

**Estimated effort**: 4-6 hours
**Lines of test code**: ~400-600

---

### Priority 3B: Performance & Stress Tests (Effort: M)
**Impact**: +2-3% coverage (mostly validation)

```go
func TestCategoryCaching_Performance(t *testing.T) {
    // Benchmark cache vs DB access
}

func TestBulkExpenseInsertion(t *testing.T) {
    // Insert 1000+ expenses, verify performance
}

func TestLargeReceiptProcessing(t *testing.T) {
    // Test with large image files
}
```

**Estimated effort**: 3-4 hours
**Lines of test code**: ~200-300

---

### Priority 3C: Edge Case Cleanup (Effort: M)
**Impact**: +3-5% coverage

**Remaining gaps:**
- Unicode handling
- Timezone edge cases
- Large dataset queries

```go
func TestUnicodeHandling(t *testing.T) {
    t.Run("emoji in description", func(t *testing.T) {...})
    t.Run("RTL text", func(t *testing.T) {...})
    t.Run("special characters", func(t *testing.T) {...})
}

func TestTimezoneHandling(t *testing.T) {
    t.Run("daylight saving transition", func(t *testing.T) {...})
    t.Run("different timezones", func(t *testing.T) {...})
}

func TestLargeDatasetQueries(t *testing.T) {
    t.Run("10000+ expenses", func(t *testing.T) {...})
    t.Run("pagination", func(t *testing.T) {...})
}
```

**Estimated effort**: 3-5 hours
**Lines of test code**: ~250-400

---

## Phase 3 Summary
- **Total effort**: 10-15 hours
- **Coverage gain**: +10-18%
- **Target coverage**: 80-95%
- **Tests to add**: ~850-1,300 lines

---

## Implementation Roadmap

### Week 1: Quick Wins (Phase 1)
**Goal**: 55-65% coverage

**Day 1-2**: Bot core unit tests
- [ ] TestNew with variations
- [ ] TestGetCategoriesWithCache
- [ ] TestInvalidateCategoryCache

**Day 3-4**: Handler error scenarios
- [ ] Add error path tests to all handlers
- [ ] Test edge cases in receipt flow
- [ ] Test callback error handling

**Day 5**: Repository & Database
- [ ] Repository error cases
- [ ] Database migration tests
- [ ] Gemini edge cases

**Milestone**: CI should show 55-65% coverage

---

### Week 2: Core Functionality (Phase 2)
**Goal**: 70-75% coverage

**Day 1-3**: Bot lifecycle & middleware
- [ ] Test Start(), cleanup routines
- [ ] Middleware tests (whitelist, registration)
- [ ] Default handler complete coverage

**Day 4-5**: Callback flows
- [ ] Complete pending edit tests
- [ ] Amount validation tests
- [ ] Category creation flow

**Milestone**: CI should show 70-75% coverage

---

### Week 3: Complete Coverage (Phase 3)
**Goal**: 80%+ coverage

**Day 1-2**: Integration improvements
- [ ] Concurrent access tests
- [ ] Transaction isolation
- [ ] Connection pooling

**Day 3-4**: Edge cases
- [ ] Unicode handling
- [ ] Timezone tests
- [ ] Large datasets

**Day 5**: Polish & validation
- [ ] Review coverage report
- [ ] Fill remaining gaps
- [ ] Document test patterns

**Milestone**: CI should show 80%+ coverage, meet AGENTS.md target

---

## Critical Success Factors

### 1. Test Infrastructure
- ✅ Already have: Good test helpers (`TestDB`, `CleanupTables`)
- ✅ Already have: Mock builders for updates
- ⚠️ Need: More mock scenarios for error cases
- ⚠️ Need: Test fixtures for common data

### 2. Test Patterns to Follow
```go
// Good: Table-driven tests
func TestParser(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *ParsedExpense
        wantErr bool
    }{
        // cases...
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic
        })
    }
}

// Good: Parallel execution for unit tests
func TestSomething(t *testing.T) {
    t.Parallel()  // Only for non-DB tests
    // test logic
}

// Good: Proper cleanup
func TestWithDB(t *testing.T) {
    pool := database.TestDB(t)
    t.Cleanup(func() {
        database.CleanupTables(t, pool)
    })
    // test logic
}
```

### 3. Coverage Monitoring
- Update CI threshold from 40% to 50% after Phase 1
- Increment to 60% after Phase 2
- Target 80% after Phase 3
- Generate HTML reports for review

### 4. Avoid Common Pitfalls
- ❌ Don't test implementation details
- ❌ Don't create flaky tests
- ❌ Don't use `t.Parallel()` for DB tests
- ❌ Don't skip cleanup
- ✅ Do test behavior, not internals
- ✅ Do use descriptive test names
- ✅ Do clean up after each test

---

## Estimated Total Effort

| Phase | Effort | Coverage Gain | New Coverage |
|-------|--------|---------------|--------------|
| Phase 1 | 9-12 hours | +25-36% | 64-75% |
| Phase 2 | 8-10 hours | +10-15% | 74-90% |
| Phase 3 | 10-15 hours | +10-18% | 80-95% |
| **TOTAL** | **27-37 hours** | **+40-60%** | **80-95%** |

Estimated **5-7 working days** of focused effort to reach 80% target.

---

## Quick Start: Next Steps

### Immediate Action Items (Today):

1. **Create test file for bot core:**
   ```bash
   touch internal/bot/bot_lifecycle_test.go
   ```

2. **Add first test:**
   ```go
   func TestGetCategoriesWithCache(t *testing.T) {
       // This is a quick win - no DB needed
   }
   ```

3. **Run coverage:**
   ```bash
   make test-coverage
   ```

4. **Track progress:**
   - Update CI threshold in `.gitlab-ci.yml` after each phase
   - Generate HTML reports: `make coverage-html`
   - Review uncovered lines

### Commands to Use:

```bash
# Check current coverage
make test-coverage

# Run integration tests
make test-integration

# View HTML report
make coverage-html
open coverage.html  # or xdg-open on Linux

# Find uncovered functions
go tool cover -func=coverage.out | grep -E "0.0%$"

# Test specific package
go test -v -cover ./internal/bot/...
```

---

## Success Metrics

- [ ] Overall coverage: 80%+ (from 39.5%)
- [ ] No package below 70% coverage
- [ ] All critical paths tested
- [ ] Error scenarios covered
- [ ] Edge cases validated
- [ ] CI enforces minimum threshold
- [ ] HTML coverage report green

---

## Maintenance Plan

**After reaching 80%:**
1. Set CI threshold to 75% (allow some flexibility)
2. Require coverage check on PRs
3. Review coverage monthly
4. Add tests for new features
5. Keep test-to-code ratio high

**Long-term target**: 85-90% coverage with focus on critical business logic.

---

## Resources & References

- **AGENTS.md**: "ENSURE that the test coverage stays at or above 40% (CI enforced). Target is 80%."
- **Current tests**: Study `parser_test.go` for excellent table-driven test examples
- **Mock patterns**: See `internal/bot/mocks/` for mock builder patterns
- **Test utilities**: `database.TestDB()`, `CleanupTables()` in `testutil.go`

---

## Notes

- Most unit tests can run in parallel (`t.Parallel()`)
- Database tests MUST use `-p 1` (already configured in CI)
- Use `t.Helper()` for test utilities
- Follow existing patterns in `handlers_test.go`
- Keep tests focused and readable
- Each test should test one thing
