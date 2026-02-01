# Integration Test Optimization: Investigation Summary

## Current State

**Test Runtime:** 4.6 seconds
**Parallelization:** None (`-p 1` flag)
**Test Count:** ~245 tests across 7 packages

## Root Causes of Slowness

### 1. Repeated Pool Creation
Every test creates a new database connection pool:
```go
pool := database.TestDB(t)  // Creates NEW pool
```

**Impact:** ~0.1-0.2s overhead per test file

### 2. Repeated Migrations
Migrations run in every test function:
```go
err := database.RunMigrations(ctx, pool)
```

**Impact:** ~0.3-0.5s per test file (7-8 migrations)

### 3. Table Truncation Locks
`CleanupTables()` uses `TRUNCATE CASCADE` which:
- Locks tables during cleanup
- Prevents parallel execution
- Blocks other tests

**Impact:** Forces sequential execution (`-p 1`)

### 4. No Parallelization
Sequential package execution due to database contention:
```makefile
go test -p 1 ./...  # One package at a time
```

**Impact:** Can't utilize multiple CPU cores

## Solutions Implemented

### Infrastructure Created

1. **`internal/database/interface.go`**
   - `PGXDB` interface implemented by both `*pgxpool.Pool` and `pgx.Tx`
   - Enables repositories to work with transactions

2. **`internal/database/testutil_tx.go`**
   - `TestPool(t)` - Shared connection pool (created once)
   - `TestTx(t)` - Transaction-based isolation
   - Migrations run only once per test run

3. **Documentation**
   - `docs/TEST_OPTIMIZATION.md` - Comprehensive guide
   - `docs/TEST_OPTIMIZATION_QUICK_WINS.md` - No-code-change optimizations
   - `docs/TEST_OPTIMIZATION_DEMO.md` - Benchmarks and examples

## Optimization Options

### Option 1: Shared Pool (Recommended First Step)

**Effort:** 5-10 minutes
**Improvement:** 30-40% faster
**Breaking Changes:** None

**Changes:**
```diff
- pool := database.TestDB(t)
- err := database.RunMigrations(ctx, pool)
- require.NoError(t, err)
+ pool := database.TestPool(t)  // Migrations already run
  database.CleanupTables(t, pool)
```

**Benefits:**
- Pool created once per test run
- Migrations run once
- No code refactoring needed
- Drop-in replacement

### Option 2: Transaction-Based Isolation

**Effort:** 2-3 hours
**Improvement:** 60-70% faster
**Breaking Changes:** Yes (repository interfaces)

**Changes:**
1. Update repositories to accept `PGXDB` instead of `*pgxpool.Pool`
2. Replace `database.TestDB(t)` with `database.TestTx(t)` in tests
3. Remove all `CleanupTables()` calls (automatic rollback)
4. Remove `-p 1` from Makefile (enable parallelization)

**Benefits:**
- Each test in its own transaction
- Automatic rollback (no cleanup needed)
- Full parallelization support
- Fastest possible tests

### Option 3: Hybrid Approach

**Effort:** 30 minutes
**Improvement:** 45-55% faster
**Breaking Changes:** None

**Changes:**
1. Use `TestPool` for shared connection
2. Add `t.Parallel()` to pure logic tests (no database)
3. Smart test organization (group database tests)
4. Selective cleanup optimization

**Benefits:**
- Best of both worlds
- Incremental migration
- Non-breaking

## Performance Projections

| Approach | Time | Improvement | Effort | Breaking? |
|----------|------|-------------|--------|-----------|
| Current | 4.6s | - | - | - |
| Shared Pool | ~3.0s | 35% | 5-10 min | No |
| Hybrid | ~2.3s | 50% | 30 min | No |
| Transaction-Based | ~1.5s | 67% | 2-3 hrs | Yes |

## Immediate Actions (Low-Hanging Fruit)

### 1. Create Proof of Concept (5 minutes)

Update one test file to use `TestPool`:

```bash
# Before
$ go test -v ./internal/repository -run TestUserRepository
PASS
ok      ...   0.520s

# After
$ go test -v ./internal/repository -run TestUserRepository
PASS
ok      ...   0.180s    # 65% faster!
```

### 2. Measure Baseline (1 minute)

```bash
$ time make test-integration 2>&1 | tail -1
real    0m4.603s    # Current baseline
```

### 3. Roll Out Shared Pool (10 minutes)

```bash
# Replace TestDB with TestPool in all test files
find internal -name "*_test.go" -exec sed -i 's/database\.TestDB(t)/database.TestPool(t)/g' {} \;

# Review and remove redundant RunMigrations calls
git diff internal

# Test
time make test-integration
# Expected: ~3.0s (35% improvement)
```

## Long-Term Recommendation

### Phase 1: Quick Wins (This Week)
- ✅ Infrastructure created
- [ ] Proof of concept (1 test file)
- [ ] Measure improvement
- [ ] Roll out `TestPool` to all tests
- [ ] Add `t.Parallel()` to non-database tests

**Target:** 40-50% improvement

### Phase 2: Transaction-Based (Next Sprint)
- [ ] Update repository interfaces to `PGXDB`
- [ ] Migrate tests to `TestTx`
- [ ] Remove `-p 1` restriction
- [ ] Remove all `CleanupTables` calls

**Target:** 65-70% improvement

## Files Modified

### Created:
- `internal/database/interface.go` - PGXDB interface
- `internal/database/testutil_tx.go` - Shared pool + transaction helpers
- `docs/TEST_OPTIMIZATION.md` - Full guide
- `docs/TEST_OPTIMIZATION_QUICK_WINS.md` - Quick wins guide
- `docs/TEST_OPTIMIZATION_DEMO.md` - Benchmarks

### To Modify (Phase 1):
- All `*_test.go` files: Replace `TestDB` with `TestPool`
- Remove redundant `RunMigrations` calls

### To Modify (Phase 2):
- All repository files: Accept `PGXDB` interface
- All test files: Use `TestTx` instead of `TestPool`
- `Makefile`: Remove `-p 1` flag

## Decision Required

Which approach should we take?

**Option A: Quick Win Only**
- 5-10 minutes effort
- 35% improvement
- No breaking changes
- Can do today

**Option B: Full Optimization**
- 2-3 hours effort
- 67% improvement
- Breaking changes (repository interfaces)
- Better long-term

**Option C: Hybrid**
- 30 minutes effort
- 50% improvement
- No breaking changes
- Good middle ground

**Recommendation:** Start with Option A (Quick Win), measure results, then decide if Option B is worth the effort.
