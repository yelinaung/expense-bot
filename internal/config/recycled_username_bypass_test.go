package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRecycledUsernameBypassWhenEnvIDListed is the regression test for the
// recycled-username protection bypass that occurred when the same admin was
// listed in BOTH WhitelistedUserIDs and WhitelistedUsernames.
//
// Previously, checkWhitelist short-circuited on an env-listed user_id and
// returned (nil, true) BEFORE the username-binding block ran, so the
// whitelisted username was never bound to the admin's user_id. A later
// attacker who recycled that username (with a different user_id) then
// reached the binding block, was bootstrapped to the attacker's user_id,
// authorized as a superadmin, and persisted.
//
// With the fix, the env-ID path also binds a co-carried whitelisted username
// to the env-listed user_id (when not yet bound) and surfaces the binding so
// the caller persists it. The legit admin's first login now binds the
// username, and a recycler with a different user_id is rejected.
func TestRecycledUsernameBypassWhenEnvIDListed(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		WhitelistedUserIDs:   []int64{100},
		WhitelistedUsernames: []string{adminUsernameConfigTest}, // "admin"
	}

	// 1. Legit admin logs in via the env-ID path while also carrying the
	//    whitelisted username. Must be authorized.
	require.True(t, cfg.IsSuperAdmin(100, adminUsernameConfigTest))

	// 2. FIXED: the username is now bound to the admin's user_id after the
	//    admin's first login (previously it stayed unbound, which was the
	//    bug). This binding is what blocks a recycler on the next step.
	boundID, bound := cfg.SuperadminBound(adminUsernameConfigTest)
	require.True(t, bound, "username should be bound to the admin's user_id after login")
	require.Equal(t, int64(100), boundID)

	// 3. FIXED: a recycler using the same whitelisted username but a
	//    different user_id is REJECTED (previously it was authorized).
	require.False(t, cfg.IsSuperAdmin(999, adminUsernameConfigTest),
		"recycled username with a different user_id must be rejected")

	// 4. The recycler must not have overwritten the admin's binding: the
	//    username is still bound to the legit admin's user_id.
	boundID, bound = cfg.SuperadminBound(adminUsernameConfigTest)
	require.True(t, bound)
	require.Equal(t, int64(100), boundID,
		"recycler must not steal or overwrite the legit admin's username binding")

	// 5. The legit admin is still authorized, with and without the username.
	require.True(t, cfg.IsSuperAdmin(100, adminUsernameConfigTest))
	require.True(t, cfg.IsSuperAdmin(100, ""), "bound user_id should work without username")
}

// TestRecycledUsernameBypass_DualListedSurfacesPersistableBinding asserts that
// the env-ID path surfaces a non-nil *SuperadminBinding on the legit admin's
// first login so the bot persists it to superadmin_bindings. Without
// persistence, a restart with a fresh in-memory state would lose the binding
// and re-open the bypass window. Subsequent legit logins must NOT surface a
// new binding (idempotent; no repeated DB writes).
func TestRecycledUsernameBypass_DualListedSurfacesPersistableBinding(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		WhitelistedUserIDs:   []int64{100},
		WhitelistedUsernames: []string{adminUsernameConfigTest},
	}

	// First legit login surfaces a binding so isAuthorized persists it.
	ok, binding := cfg.CheckSuperAdmin(100, adminUsernameConfigTest)
	require.True(t, ok)
	require.NotNil(t, binding, "first dual-listed login must surface a binding for persistence")
	require.Equal(t, adminUsernameConfigTest, binding.Username)
	require.Equal(t, int64(100), binding.UserID)

	// Subsequent legit logins are authorized but surface no new binding
	// (no repeated DB upserts needed once the username is locked).
	for range 3 {
		ok, binding = cfg.CheckSuperAdmin(100, adminUsernameConfigTest)
		require.True(t, ok)
		require.Nil(t, binding, "already-bound username must not surface a new binding")
	}

	// A recycler is rejected and also surfaces no binding.
	ok, binding = cfg.CheckSuperAdmin(999, adminUsernameConfigTest)
	require.False(t, ok)
	require.Nil(t, binding, "rejected recycler must not surface a binding")
}

// TestRecycledUsernameBypass_NonWhitelistedUsernameNotBoundOnEnvIDPath
// asserts the env-ID-path binding only locks WHITELISTED usernames, not
// arbitrary usernames the caller happens to carry. An env-listed admin
// logging in with a non-whitelisted (or empty) username must not create a
// binding, and must not surface one (no spurious persistence). This
// preserves the existing "no binding on plain user-ID match" behavior.
func TestRecycledUsernameBypass_NonWhitelistedUsernameNotBoundOnEnvIDPath(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		WhitelistedUserIDs:   []int64{100},
		WhitelistedUsernames: []string{adminUsernameConfigTest},
	}

	// Env-ID match with no username: authorized, no binding.
	ok, binding := cfg.CheckSuperAdmin(100, "")
	require.True(t, ok)
	require.Nil(t, binding, "env-ID match without username must not create a binding")

	// Env-ID match with a non-whitelisted username: authorized, no binding,
	// and that username is NOT recorded as a superadmin binding.
	ok, binding = cfg.CheckSuperAdmin(100, "someone-else")
	require.True(t, ok)
	require.Nil(t, binding, "env-ID match with a non-whitelisted username must not bind it")

	_, bound := cfg.SuperadminBound("someone-else")
	require.False(t, bound, "non-whitelisted username must not be recorded as a binding")

	// The whitelisted username (not yet seen on the env-ID path with this
	// identity because the calls above carried other/no usernames) is still
	// unbound: the env-ID path must not speculatively bind whitelisted
	// usernames it never actually observed on the wire.
	_, bound = cfg.SuperadminBound(adminUsernameConfigTest)
	require.False(t, bound, "whitelisted username must only bind when it is actually carried")
}

// TestRecycledUsernameBypass_PreBoundUsernameNotStolenByOtherEnvIDListedAdmin
// covers the cross-list conflict edge case: the username is already bound to
// user_id 100, and a DIFFERENT env-listed user_id (200) logs in carrying that
// same whitelisted username. The env-ID path must still authorize user 200
// (they are explicitly env-listed) but must NOT rebind the username away from
// 100, so the original identity's recycled-username protection is preserved
// for non-env-listed recyclers.
func TestRecycledUsernameBypass_PreBoundUsernameNotStolenByOtherEnvIDListedAdmin(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		WhitelistedUserIDs:   []int64{100, 200},
		WhitelistedUsernames: []string{adminUsernameConfigTest},
	}
	// Pre-bind "admin" to the legit admin 100 (simulates a prior binding).
	cfg.LoadSuperadminBindings([]SuperadminBinding{
		{Username: adminUsernameConfigTest, UserID: 100},
	})

	// A different env-listed admin (200) carrying "admin" is authorized
	// (they are explicitly env-listed) but does NOT steal the binding.
	ok, binding := cfg.CheckSuperAdmin(200, adminUsernameConfigTest)
	require.True(t, ok, "env-listed user_id is authorized regardless of username")
	require.Nil(t, binding, "must not rebind a username already bound to another user_id")

	boundID, bound := cfg.SuperadminBound(adminUsernameConfigTest)
	require.True(t, bound)
	require.Equal(t, int64(100), boundID, "binding must remain with the original legit admin")

	// A non-env-listed recycler of "admin" is still rejected because the
	// binding to 100 is intact.
	require.False(t, cfg.IsSuperAdmin(999, adminUsernameConfigTest))
}
