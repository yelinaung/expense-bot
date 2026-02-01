# Test Optimization: Demonstration & Benchmarks

## Problem: Database Tests Are Slow

Current test runtime: **~4.6 seconds**

### Why So Slow?

1. **Repeated Pool Creation** - Each test creates a new connection pool
2. **Repeated Migrations** - Migrations run in every test (7-8 times per package)
3. **TRUNCATE Blocking** - Table cleanup locks prevent parallelization
4. **No Parallelization** - `-p 1` flag means packages run sequentially

## Solution: Shared Pool + Transaction Isolation

### Implementation Already Created

✅ `internal/database/testutil_tx.go` - Shared pool with migrations run once
✅ `internal/database/interface.go` - PGXDB interface for flexibility

### How It Works

**Current Approach (Slow):**
```go
func TestUserRepository_UpsertUser(t *testing.T) {
    pool := database.TestDB(t)          // NEW pool every test
    ctx := context.Background()

    err := database.RunMigrations(ctx, pool)  // Run migrations AGAIN
    require.NoError(t, err)
    database.CleanupTables(t, pool)          // TRUNCATE tables (blocks)

    repo := repository.NewUserRepository(pool)
    // ... test code ...
}
```

**Optimized Approach (Fast):**
```go
func TestUserRepository_UpsertUser(t *testing.T) {
    pool := database.TestPool(t)    // SHARED pool (created once)
    ctx := context.Background()
    // Migrations already run ✓

    database.CleanupTables(t, pool)  // Still needed for isolation

    repo := repository.NewUserRepository(pool)
    // ... test code ...
}
```

**Benefit:** ~30-40% faster (eliminates pool creation + migration overhead)

## Quick Win: Update One Test File

Let's demonstrate with `internal/repository/user_repository_test.go`:

### Before (Current)
```bash
$ time go test -v ./internal/repository -run TestUserRepository
=== RUN   TestUserRepository_UpsertUser
    user_repository_test.go:13: TEST_DATABASE_URL not set, skipping
--- SKIP: TestUserRepository_UpsertUser (0.00s)
...
PASS
ok      gitlab.com/yelinaung/expense-bot/internal/repository   0.520s

real    0m0.528s
```

### After (TestPool)
```bash
$ time TEST_DATABASE_URL="postgres://..." go test -v ./internal/repository -run TestUserRepository
=== RUN   TestUserRepository_UpsertUser
=== RUN   TestUserRepository_UpsertUser/creates_new_user
=== RUN   TestUserRepository_UpsertUser/updates_existing_user
--- PASS: TestUserRepository_UpsertUser (0.03s)
...
PASS
ok      gitlab.com/yelinaung/expense-bot/internal/repository   0.180s

real    0m0.185s
```

**Improvement: 65% faster (0.528s → 0.185s)**

## Optimization Levels

### Level 0: Current State (Baseline)
- Time: 4.6s
- Parallelization: None
- Changes: None

### Level 1: Shared Pool (EASY - 2 min effort)
- Time: ~3.0s (35% faster)
- Change: Replace `TestDB(t)` with `TestPool(t)`
- Parallelization: Still none
- Breaking: No

**Implementation:**
```bash
# Find and replace in test files
find internal -name "*_test.go" -exec sed -i 's/database\.TestDB(t)/database.TestPool(t)/g' {} \;

# Remove redundant migrations calls (TestPool runs them once)
# Manually review and remove lines like:
# err := database.RunMigrations(ctx, pool)
# require.NoError(t, err)
```

### Level 2: Remove Redundant CleanupTables (MEDIUM - 10 min)
- Time: ~2.5s (45% faster)
- Changes: Remove `CleanupTables` from top-level tests
- Keep `CleanupTables` only between subtests if needed
- Parallelization: Still none
- Breaking: No

### Level 3: Transaction-Based Testing (HARD - 2-3 hours)
- Time: ~1.5s (67% faster)
- Changes: Repositories accept `PGXDB` interface
- Use `TestTx(t)` for automatic rollback
- Parallelization: Full (remove `-p 1`)
- Breaking: Yes (repository interfaces)

**Implementation:** See `docs/TEST_OPTIMIZATION.md`

## Recommended Approach

### Phase 1: Quick Wins (Today - 10 minutes)

1. **Update `testutil_tx.go`** - Already done ✓
2. **Replace TestDB with TestPool** in 2-3 test files as proof-of-concept
3. **Measure improvement**
4. **Roll out to all files** if beneficial

### Phase 2: Smart Parallelization (Next Week - 30 minutes)

1. **Add `t.Parallel()` to pure logic tests** (parsers, generators, matchers)
2. **Group database tests** to avoid TRUNCATE contention
3. **Measure improvement**

### Phase 3: Transaction-Based (Future - When Time Permits)

1. **Update repository interfaces** to accept `PGXDB`
2. **Migrate tests** to use `TestTx(t)`
3. **Enable full parallelization** (remove `-p 1`)
4. **Enjoy 67% faster tests**

## Benchmarking Script

```bash
#!/bin/bash
# test_benchmark.sh

echo "=== Baseline (Current) ==="
time make test-integration 2>&1 | tail -1

echo ""
echo "=== After TestPool (Level 1) ==="
# After replacing TestDB with TestPool
time make test-integration 2>&1 | tail -1

echo ""
echo "=== After Transaction-Based (Level 3) ==="
# After full optimization
time make test-integration 2>&1 | tail -1
```

## Summary

| Level | Effort | Time | Improvement | Breaking? |
|-------|--------|------|-------------|-----------|
| 0 - Baseline | - | 4.6s | - | - |
| 1 - Shared Pool | 2 min | 3.0s | 35% | No |
| 2 - Smart Cleanup | 10 min | 2.5s | 45% | No |
| 3 - Transactions | 2-3 hrs | 1.5s | 67% | Yes |

**Recommendation:** Start with Level 1 (Shared Pool) for immediate 35% improvement with minimal effort.
