package bot

import (
	"context"
	"testing"

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
// It exercises the production superadmin binding upsert
// (SuperadminBindingRepository.Save, the ON CONFLICT SQL that Bot.isAuthorized
// runs in its persistence goroutine) and the production reload path
// (loadSuperadminBindings, run by Bot.New at startup) against a real
// Postgres, and asserts the FIXED behavior across a simulated restart:
//
//  1. The legit dual-listed admin's first authorization surfaces a
//     persistable binding (admin → 100) which, once upserted, populates
//     superadmin_bindings (previously the env-ID short-circuit returned a
//     nil binding and persisted NOTHING, leaving the username unbound).
//  2. A recycler using the same whitelisted username with a different
//     user_id is REJECTED in-process and is NOT persisted (previously the
//     recycler was authorized and its user_id was upserted over the
//     admin's, granting permanent superadmin status).
//  3. After a simulated restart (fresh Config + real loadSuperadminBindings
//     reload), the recycler is still rejected and the legit admin is still
//     authorized (previously the recycler remained a superadmin across
//     restarts via the persisted attacker binding).
//
// Requires TEST_DATABASE_URL; skips otherwise (see dbtest.TestTx).
func TestRecycledUsernameBypassPersistedAcrossRestart(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.TestTx(ctx, t) // per-test transaction, auto-rolled-back

	const adminUser = "admin"
	const legitID int64 = 100
	const attackerID int64 = 999

	// Dual-listed legit admin: env-listed user_id AND whitelisted username.
	cfg := &config.Config{
		WhitelistedUserIDs:   []int64{legitID},
		WhitelistedUsernames: []string{adminUser},
	}
	bindingRepo := repository.NewSuperadminBindingRepository(tx)

	// 1. Legit admin's first authorization via the env-ID path. With the
	//    fix, CheckSuperAdmin surfaces a non-nil binding (admin → 100) so
	//    isAuthorized persists it — exactly what the persistence goroutine
	//    upserts. Simulate that goroutine by calling Save with the surfaced
	//    binding (the production goroutine calls Save with the same args).
	ok, binding := cfg.CheckSuperAdmin(legitID, adminUser)
	require.True(t, ok, "legit dual-listed admin must be authorized")
	require.NotNil(t, binding, "first dual-listed login must surface a binding to persist")
	require.Equal(t, adminUser, binding.Username)
	require.Equal(t, legitID, binding.UserID)

	require.NoError(t, bindingRepo.Save(ctx, binding.Username, binding.UserID))

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
	recycledOk, recycledBinding := cfg.CheckSuperAdmin(attackerID, adminUser)
	require.False(t, recycledOk, "recycler with a different user_id must be rejected")
	require.Nil(t, recycledBinding, "rejected recycler must not surface a binding to persist")

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
	loadSuperadminBindings(ctx, restarted, tx)

	require.True(t, restarted.IsSuperAdmin(legitID, adminUser),
		"legit admin must remain authorized after restart+reload")
	require.True(t, restarted.IsSuperAdmin(legitID, ""),
		"legit admin must remain authorized by user_id alone after restart")
	require.False(t, restarted.IsSuperAdmin(attackerID, adminUser),
		"recycler must be rejected after restart+reload (was authorized in the buggy version)")
	require.False(t, restarted.IsSuperAdmin(attackerID, ""),
		"recycler must not be a superadmin by user_id alone after restart")
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
