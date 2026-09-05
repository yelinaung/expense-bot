package bot

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/config"
	"gitlab.com/yelinaung/expense-bot/internal/repository"
	"gitlab.com/yelinaung/expense-bot/internal/testutil/dbtest"
)

// TestRecycledUsernameBypassPersistedAcrossRestart is the DB-backed
// end-to-end regression test for the recycled-username protection bypass
// when the same admin is listed in BOTH WHITELISTED_USER_IDS and
// WHITELISTED_USERNAMES.
//
// It exercises the production superadmin binding upsert via Bot.isAuthorized
// (which launches the persistence goroutine that calls
// SuperadminBindingRepository.Save) and the production reload path
// (loadSuperadminBindings, run by Bot.New at startup) against a real
// Postgres, and asserts the FIXED behavior across a simulated restart:
//
//  1. The legit dual-listed admin's first authorization via isAuthorized
//     surfaces a persistable binding (admin → 100) and persists it through
//     the production goroutine — previously the env-ID short-circuit returned
//     a nil binding and persisted NOTHING, leaving the username unbound.
//  2. A recycler using the same whitelisted username with a different
//     user_id is REJECTED in-process and is NOT persisted (previously the
//     recycler was authorized and its user_id was upserted over the
//     admin's, granting permanent superadmin status).
//  3. After a simulated restart (fresh Config + real loadSuperadminBindings
//     reload), the recycler is still rejected and the legit admin is still
//     authorized (previously the recycler remained a superadmin across
//     restarts via the persisted attacker binding).
//
// Requires TEST_DATABASE_URL; skips otherwise (see dbtest.TestPool).
func TestRecycledUsernameBypassPersistedAcrossRestart(t *testing.T) {
	ctx := context.Background()
	// Use a pool (not a transaction) because Bot.isAuthorized launches an async
	// goroutine that calls bindingRepo.Save concurrently with the test's
	// LoadAll polling. A single pgx.Tx allows only one operation at a time,
	// causing "conn busy" errors. The pool hands each concurrent caller its
	// own connection, avoiding the contention.
	pool := dbtest.TestPool(t)
	// Truncate superadmin_bindings before and after to isolate from other tests.
	_, _ = pool.Exec(ctx, `TRUNCATE TABLE superadmin_bindings`)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.WithoutCancel(ctx), `TRUNCATE TABLE superadmin_bindings`)
	})

	const adminUser = "admin"
	const legitID int64 = 100
	const attackerID int64 = 999

	// Dual-listed legit admin: env-listed user_id AND whitelisted username.
	cfg := &config.Config{
		WhitelistedUserIDs:   []int64{legitID},
		WhitelistedUsernames: []string{adminUser},
	}
	bindingRepo := repository.NewSuperadminBindingRepository(pool)

	// Construct a Bot with the real binding repository so that Bot.isAuthorized
	// exercises the production persistence goroutine.
	b := &Bot{
		cfg:              cfg,
		db:               pool,
		bindingRepo:      bindingRepo,
		approvedUserRepo: repository.NewApprovedUserRepository(pool),
		pendingEdits:     make(map[int64]*pendingEdit),
	}

	// 1. Legit admin's first authorization through the production path.
	//    isAuthorized calls cfg.CheckSuperAdmin, receives a non-nil binding
	//    (admin → 100), and launches the persistence goroutine that calls
	//    bindingRepo.Save. We poll the DB until the row appears (or the
	//    test deadline fires) to wait deterministically for the goroutine.
	require.True(t, b.isAuthorized(ctx, legitID, adminUser),
		"legit dual-listed admin must be authorized")

	// Poll until the persistence goroutine commits the row.
	require.Eventually(t, func() bool {
		bindings, err := bindingRepo.LoadAll(ctx)
		if err != nil {
			return false
		}
		m := bindingsToMap(bindings)
		id, ok := m[adminUser]
		return ok && id == legitID
	}, 5*time.Second, 10*time.Millisecond,
		"persistence goroutine must have written admin→legitID within 5s")

	// The persisted row is admin → legitID (NOT empty, NOT the attacker).
	bindings, err := bindingRepo.LoadAll(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]int64{adminUser: legitID}, bindingsToMap(bindings),
		"superadmin_bindings must contain only the legit admin's binding")

	// 2. Recycler with a different user_id is REJECTED in-process (the
	//    username is now bound to the legit admin) and, because
	//    isAuthorized only persists when authorized, nothing is upserted
	//    for the attacker. Assert rejection and that the DB row is
	//    unchanged.
	require.False(t, b.isAuthorized(ctx, attackerID, adminUser),
		"recycler with a different user_id must be rejected")

	bindings, err = bindingRepo.LoadAll(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]int64{adminUser: legitID}, bindingsToMap(bindings),
		"attacker must not be persisted; legit admin's binding must remain intact")

	// 3. Simulated restart: a fresh Config and the production reload path
	//    (loadSuperadminBindings, the function Bot.New runs at startup).
	restarted := &config.Config{
		WhitelistedUserIDs:   []int64{legitID},
		WhitelistedUsernames: []string{adminUser},
	}
	loadSuperadminBindings(ctx, restarted, pool)

	require.True(t, restarted.IsSuperAdmin(legitID, adminUser),
		"legit admin must remain authorized after restart+reload")
	require.True(t, restarted.IsSuperAdmin(legitID, ""),
		"legit admin must remain authorized by user_id alone after restart")
	require.False(t, restarted.IsSuperAdmin(attackerID, adminUser),
		"recycler must be rejected after restart+reload (was authorized in the buggy version)")
	require.False(t, restarted.IsSuperAdmin(attackerID, ""),
		"recycler must not be a superadmin by user_id alone after restart")
}

// TestRecycledUsernameBypassPersistedAcrossRestart_NonEnvListed covers the
// non-env-listed (username-only bootstrap) logging branch in isAuthorized:
// a user listed only in WhitelistedUsernames (not WhitelistedUserIDs) must
// be authorized on first login, have their binding persisted through the
// production goroutine, and have the non-env-ID log message emitted instead
// of the env-ID locking message.
func TestRecycledUsernameBypassPersistedAcrossRestart_NonEnvListed(t *testing.T) {
	ctx := context.Background()
	// Use a pool (not a transaction) for the same reason as the sibling test:
	// the async persistence goroutine and the polling LoadAll call run
	// concurrently and a single pgx.Tx cannot be used from two goroutines
	// simultaneously.
	pool := dbtest.TestPool(t)
	_, _ = pool.Exec(ctx, `TRUNCATE TABLE superadmin_bindings`)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.WithoutCancel(ctx), `TRUNCATE TABLE superadmin_bindings`)
	})

	const onlyUsernameUser = "onlylisted"
	const onlyUsernameID int64 = 200

	cfg := &config.Config{
		WhitelistedUserIDs:   []int64{},
		WhitelistedUsernames: []string{onlyUsernameUser},
	}
	bindingRepo := repository.NewSuperadminBindingRepository(pool)

	b := &Bot{
		cfg:              cfg,
		db:               pool,
		bindingRepo:      bindingRepo,
		approvedUserRepo: repository.NewApprovedUserRepository(pool),
		pendingEdits:     make(map[int64]*pendingEdit),
	}

	// Username-only listed user logs in: exercises the non-envListed branch
	// (the "Persisted superadmin binding; consider adding user_id" log message).
	require.True(t, b.isAuthorized(ctx, onlyUsernameID, onlyUsernameUser),
		"username-only whitelisted user must be authorized")

	require.Eventually(t, func() bool {
		bindings, err := bindingRepo.LoadAll(ctx)
		if err != nil {
			return false
		}
		m := bindingsToMap(bindings)
		id, ok := m[onlyUsernameUser]
		return ok && id == onlyUsernameID
	}, 5*time.Second, 10*time.Millisecond,
		"persistence goroutine must have written the username-only binding within 5s")
}

// bindingsToMap converts a repository binding slice to a username → user_id
// map for assertion, mirroring the on-disk shape (one row per username).
func bindingsToMap(bs []repository.SuperadminBinding) map[string]int64 {
	out := make(map[string]int64, len(bs))
	for _, b := range bs {
		out[b.Username] = b.UserID
	}
	return out
}
