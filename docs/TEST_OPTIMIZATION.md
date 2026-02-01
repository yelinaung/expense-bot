# Integration Test Optimization Guide

## Current State

**Problems:**
- Tests run sequentially (`-p 1` in Makefile) due to database contention
- Each test creates its own connection pool
- Each test runs migrations independently
- Tests use `TRUNCATE` for cleanup, which locks tables
- Total runtime: ~4.6 seconds (could be much faster)

**Current Pattern:**
```go
func TestSomething(t *testing.T) {
    pool := database.TestDB(t)          // New pool per test
    ctx := context.Background()

    err := database.RunMigrations(ctx, pool)  // Migrations per test
    require.NoError(t, err)
    database.CleanupTables(t, pool)    // TRUNCATE - blocks parallel tests

    // ... test code ...
}
```

## Optimization Strategy

### 1. Shared Connection Pool
- Create pool once, reuse across all tests
- Run migrations once at startup
- Seed reference data once

### 2. Transaction-Based Isolation
- Each test runs in its own transaction
- Transaction is rolled back after test completes
- No need for `TRUNCATE` or `CleanupTables()`
- Tests can run in parallel safely

### 3. Proper Parallelization
- Use `t.Parallel()` in tests
- Remove `-p 1` restriction in Makefile
- Subtests can run concurrently

## Implementation

### New Test Infrastructure

**`internal/database/interface.go`:**
```go
type PGXDB interface {
    Exec(ctx context.Context, sql string, arguments ...any) (pgx.CommandTag, error)
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
```

Both `*pgxpool.Pool` and `pgx.Tx` implement this interface.

**`internal/database/testutil_tx.go`:**
- `TestPool(t)` - Returns shared pool (created once)
- `TestTx(t)` - Returns transaction that auto-rollbacks

### Migration Path

#### Option A: Minimal Changes (Recommended for Quick Wins)

Keep existing `TestDB()` for non-parallel tests, add `TestTx()` for parallel tests:

```go
func TestSomethingParallel(t *testing.T) {
    t.Parallel()  // Now safe!

    tx := database.TestTx(t)  // Auto-rollback transaction
    userRepo := repository.NewUserRepository(tx)

    // No migrations needed - already run
    // No cleanup needed - auto rollback

    // ... test code ...
}
```

**Benefits:**
- Opt-in migration (test by test)
- No breaking changes
- Immediate parallelization for new tests

#### Option B: Full Migration (Maximum Performance)

Update all repositories to accept `PGXDB` interface:

```go
type UserRepository struct {
    db database.PGXDB  // was: pool *pgxpool.Pool
}

func NewUserRepository(db database.PGXDB) *UserRepository {
    return &UserRepository{db: db}
}
```

Update all usages:
```go
// Production code
repo := repository.NewUserRepository(pool)  // Still works

// Test code
repo := repository.NewUserRepository(database.TestTx(t))  // Uses transaction
```

**Benefits:**
- All tests can run in parallel
- Cleaner architecture (interface-based)
- 2-3x faster test suite

**Migration effort:**
- ~11 repository files to update
- ~50 test files to update
- Estimated: 1-2 hours

## Performance Expectations

### Before Optimization
```
Total time: ~4.6 seconds
Parallelization: None (-p 1)
Migrations: Run in every test
Cleanup: TRUNCATE (slow, blocking)
```

### After Optimization (Option A - Partial)
```
Total time: ~2-3 seconds (40-50% faster)
Parallelization: New tests only
Migrations: Run once
Cleanup: Rollback (fast, non-blocking)
```

### After Optimization (Option B - Full)
```
Total time: ~1.5-2 seconds (60-70% faster)
Parallelization: All tests
Migrations: Run once
Cleanup: Rollback (fast, non-blocking)
```

## Makefile Changes

Update `test-integration` target:

```makefile
# Before
test-integration: test-db-up
	@TEST_DATABASE_URL="..." \
		go test -v -coverprofile=coverage.out -covermode=atomic -p 1 ./...
	@$(MAKE) test-db-down

# After (allows parallelization)
test-integration: test-db-up
	@TEST_DATABASE_URL="..." \
		go test -v -coverprofile=coverage.out -covermode=atomic ./...
	@$(MAKE) test-db-down
```

Remove `-p 1` to allow parallel execution.

## Example: Before and After

### Before (Current)
```go
func TestHandleSetCurrencyCore(t *testing.T) {
    pool := database.TestDB(t)
    ctx := context.Background()

    err := database.RunMigrations(ctx, pool)  // Slow
    require.NoError(t, err)
    database.CleanupTables(t, pool)  // Blocking

    userRepo := repository.NewUserRepository(pool)
    // ... test ...
}
```

### After (Optimized)
```go
func TestHandleSetCurrencyCore(t *testing.T) {
    t.Parallel()  // Safe now!

    tx := database.TestTx(t)  // Fast, isolated
    ctx := context.Background()

    userRepo := repository.NewUserRepository(tx)
    // ... test ...
    // Auto-rollback on completion
}
```

## Proof of Concept

See `internal/repository/user_repository_optimized_test.go` for a working example showing:
- Transaction-based isolation
- Parallel test execution
- No manual cleanup needed

## Recommendations

### Short Term (Quick Win)
1. Implement `TestPool()` and `TestTx()` helpers
2. Update 2-3 test files as proof of concept
3. Measure performance improvement
4. Add `t.Parallel()` to new tests going forward

### Long Term (Full Optimization)
1. Update repositories to accept `PGXDB` interface
2. Migrate all tests to use `TestTx()`
3. Remove `TestDB()` and `CleanupTables()`
4. Remove `-p 1` from Makefile
5. Enjoy 2-3x faster test suite

## Additional Optimizations

### Connection Pool Tuning
```go
// In testutil_tx.go
config, _ := pgxpool.ParseConfig(dbURL)
config.MaxConns = 20  // Increase for parallel tests
config.MinConns = 5
pool, _ := pgxpool.NewWithConfig(ctx, config)
```

### Test-Specific Schemas (Advanced)
For complete isolation, create schema per test package:
```sql
CREATE SCHEMA test_bot;
CREATE SCHEMA test_repository;
```

Each package uses its own schema, true parallel execution.

## References

- [pgx documentation](https://pkg.go.dev/github.com/jackc/pgx/v5)
- [Go testing best practices](https://go.dev/doc/tutorial/add-a-test)
- [Database testing patterns](https://www.alexedwards.net/blog/organising-database-access)
