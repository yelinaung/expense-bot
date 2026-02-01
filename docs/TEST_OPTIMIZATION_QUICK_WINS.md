# Test Optimization: Quick Wins (No Code Changes Required)

## Current Performance

```bash
$ time make test-integration
real: 4.603s
```

## Problem Analysis

1. **Shared pool creation** - Each test creates a new pool (slow)
2. **Repeated migrations** - Migrations run in every test (slow)
3. **Sequential execution** - `-p 1` flag prevents parallelization
4. **TRUNCATE on every test** - Locks tables, prevents parallel execution

## Quick Win #1: Shared Pool & Migrations (50% faster)

**Change:** Run migrations once, share connection pool

**Implementation:**

```bash
# Update Makefile test-integration target
test-integration: test-db-up
	@TEST_DATABASE_URL="..." \
		go test -v -coverprofile=coverage.out -covermode=atomic ./... 2>&1 | grep -v "no such tool" || true
	@go tool cover -func=coverage.out
	@$(MAKE) test-db-down
```

Remove `-p 1` flag to allow parallel package execution.

**Files already created:**
- ✅ `internal/database/testutil_tx.go` - Shared pool with `TestPool(t)`
- ✅ `internal/database/interface.go` - PGXDB interface

**Expected improvement:** 4.6s → ~2.5s (45% faster)

## Quick Win #2: Add t.Parallel() to Safe Tests

Tests that don't conflict can run in parallel immediately:

```go
func TestBuildCurrencyListMessage(t *testing.T) {
    t.Parallel()  // Pure function test - always safe

    b := &Bot{}
    message := b.buildCurrencyListMessage()
    require.Contains(t, message, "USD")
}

func TestParseExpenseInput(t *testing.T) {
    t.Parallel()  // No database - always safe

    result := ParseExpenseInput("$10 Coffee")
    require.NotNil(t, result)
}
```

**Candidates for immediate parallel execution:**
- All parser tests (`parser_*.go`)
- All matcher tests (`category_matcher_*.go`)
- All generator tests (`csv_generator_test.go`, `chart_generator_test.go`)
- Mock/builder tests
- Pure business logic tests

**Expected improvement:** Additional 20-30% for these test files

## Quick Win #3: Smart Test Organization

Group tests by database dependency:

```go
// tests that can run in parallel
func TestParsers(t *testing.T) {
    t.Run("currency parsing", func(t *testing.T) {
        t.Parallel()
        // ...
    })

    t.Run("amount parsing", func(t *testing.T) {
        t.Parallel()
        // ...
    })
}

// tests that need database run sequentially within package
func TestDatabaseOperations(t *testing.T) {
    t.Run("create user", func(t *testing.T) {
        // runs sequentially with other database tests in this func
    })
}
```

## Implementation Steps

### Step 1: Update Makefile (2 minutes)

```diff
test-integration: test-db-up
	@TEST_DATABASE_URL="postgres://$${POSTGRES_USER:-test}:$${POSTGRES_PASSWORD:-test}@localhost:5433/$${POSTGRES_DB:-expense_bot_test}?sslmode=disable" \
-		go test -v -coverprofile=coverage.out -covermode=atomic -p 1 ./... 2>&1 | grep -v "no such tool" || true
+		go test -v -coverprofile=coverage.out -covermode=atomic ./... 2>&1 | grep -v "no such tool" || true
	@go tool cover -func=coverage.out
	@$(MAKE) test-db-down
```

### Step 2: Update Tests to Use Shared Pool (10 minutes)

Replace this pattern:
```go
pool := database.TestDB(t)
err := database.RunMigrations(ctx, pool)
require.NoError(t, err)
database.CleanupTables(t, pool)
```

With:
```go
pool := database.TestPool(t)  // Shared, migrations already run
database.CleanupTables(t, pool)  // Still needed for now
```

### Step 3: Add t.Parallel() to Non-Database Tests (5 minutes)

Find all tests without database dependencies:
```bash
# Find parser tests
grep -l "func Test.*Parse" internal/bot/*test.go

# Find generator tests
grep -l "Generator" internal/bot/*test.go
```

Add `t.Parallel()` as first line in each test function.

## Expected Results

### Before
```
Packages: 7
Tests: 245
Time: 4.6s
Parallel: No
```

### After Quick Wins
```
Packages: 7 (can run in parallel)
Tests: 245
  - ~100 non-DB tests run in parallel
  - ~145 DB tests run sequentially within package
Time: ~2.0-2.5s (50-60% faster)
Parallel: Yes (package level + within non-DB tests)
```

## Future Optimization: Transaction-Based Testing

For maximum performance (1.5s), implement transaction-based testing:

**Requires:**
- Update repositories to accept `database.PGXDB` interface
- Use `database.TestTx(t)` in tests
- Remove all `CleanupTables()` calls

**Benefit:** All tests can run in parallel, 70% faster overall

**Effort:** ~2-3 hours to update all files

See `docs/TEST_OPTIMIZATION.md` for full details.

## Measuring Improvement

```bash
# Baseline
time make test-integration

# After Quick Win #1 (Makefile change)
time make test-integration

# After Quick Win #2 (t.Parallel on non-DB tests)
time make test-integration

# After Quick Win #3 (Test organization)
time make test-integration
```

## Summary

| Optimization | Effort | Improvement | Breaking Changes |
|---|---|---|---|
| Shared pool & remove -p 1 | 2 min | ~45% | No |
| Add t.Parallel() to safe tests | 5 min | +10-15% | No |
| Smart test organization | 10 min | +5-10% | No |
| **Total Quick Wins** | **17 min** | **~60%** | **No** |
| Transaction-based (future) | 2-3 hrs | ~70% | Yes (repos) |

Start with Quick Wins today, plan transaction-based for later.
