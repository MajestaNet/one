package db_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestEnsureBootstrapAdminRepairsLegacyHumanPrincipal(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()
	users := db.NewUserStore(d.Pool)

	if _, err := d.Pool.Exec(ctx, `
UPDATE users SET principal_type = 'user' WHERE id = $1::uuid`, testutil.DefaultOwnerID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(context.Background(), `
UPDATE users SET principal_type = 'service' WHERE id = $1::uuid`, testutil.DefaultOwnerID)
	})

	repaired, err := users.EnsureBootstrapAdmin(
		ctx,
		testutil.DefaultOwnerID,
		"admin@one.local",
		"Majesta One Admin",
	)
	if err != nil {
		t.Fatal(err)
	}
	if repaired.PrincipalType != "service" {
		t.Fatalf("principal_type=%q want service", repaired.PrincipalType)
	}
}

func TestEnsureInitialHumanSystemAdminPromotesWhenNoneExist(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()
	users := db.NewUserStore(d.Pool)
	cleanupHumanUsers(t, d)

	first, err := users.CreateSocialUser(ctx, "oldest@example.com", "Oldest", "StandardUser")
	if err != nil {
		t.Fatal(err)
	}
	second, err := users.CreateSocialUser(ctx, "newer@example.com", "Newer", "StandardUser")
	if err != nil {
		t.Fatal(err)
	}

	// First authenticating human without a human SystemAdmin gets the pack.
	if err := users.EnsureInitialHumanSystemAdmin(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	scopes, isAdmin, roles, err := users.ListRoleGrants(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !isAdmin {
		t.Fatal("first human should become System Admin")
	}
	found := false
	for _, r := range roles {
		if r == db.SystemAdminRoleAPIName {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %s, got %v", db.SystemAdminRoleAPIName, roles)
	}
	want := map[string]bool{"client": true, "metadata": true, "deploy": true, "ops": true}
	for _, sc := range scopes {
		delete(want, string(sc))
	}
	if len(want) > 0 {
		t.Fatalf("missing scopes %v from %v", want, scopes)
	}
	names, err := users.ListPermissionSetAPINames(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	hasPS := false
	for _, n := range names {
		if n == db.SystemAdminPermissionSetAPIName {
			hasPS = true
		}
	}
	if !hasPS {
		t.Fatalf("expected %s PS, got %v", db.SystemAdminPermissionSetAPIName, names)
	}

	// Later humans are not promoted once a human SystemAdmin exists.
	if err := users.EnsureInitialHumanSystemAdmin(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	_, isAdmin2, roles2, err := users.ListRoleGrants(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if isAdmin2 {
		t.Fatal("second human must stay non-admin after first promotion")
	}
	for _, r := range roles2 {
		if r == db.SystemAdminRoleAPIName {
			t.Fatalf("second human must not receive SystemAdmin, roles=%v", roles2)
		}
	}
}

func TestEnsureInitialHumanSystemAdminSerializesConcurrentFirstLogin(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()
	users := db.NewUserStore(d.Pool)
	cleanupHumanUsers(t, d)

	first, err := users.CreateSocialUser(ctx, "race-first@example.com", "Race First", "StandardUser")
	if err != nil {
		t.Fatal(err)
	}
	second, err := users.CreateSocialUser(ctx, "race-second@example.com", "Race Second", "StandardUser")
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, id := range []string{first.ID, second.ID} {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			<-start
			errs <- users.EnsureInitialHumanSystemAdmin(ctx, userID)
		}(id)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	var admins int
	if err := d.Pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM user_roles ur
JOIN users u ON u.id=ur.user_id
JOIN roles r ON r.id=ur.role_id
WHERE r.api_name=$1 AND u.id = ANY($2::uuid[])`,
		db.SystemAdminRoleAPIName, []string{first.ID, second.ID}).Scan(&admins); err != nil {
		t.Fatal(err)
	}
	if admins != 1 {
		t.Fatalf("concurrent first sign-ins promoted %d humans, want exactly 1", admins)
	}
}

func TestEnsureSystemAdminPermissionSetLabel(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	id, err := db.EnsureSystemAdminPermissionSet(t.Context(), d.Pool)
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("empty id")
	}
	var label string
	var caps []byte
	if err := d.Pool.QueryRow(t.Context(), `
SELECT label, system_permissions FROM permission_sets WHERE api_name = $1`,
		db.SystemAdminPermissionSetAPIName).Scan(&label, &caps); err != nil {
		t.Fatal(err)
	}
	if label != db.SystemAdminPermissionSetLabel {
		t.Fatalf("label=%q", label)
	}
	if !containsAllCaps(string(caps)) {
		t.Fatalf("expected full system_permissions, got %s", caps)
	}
}

func TestEnsureAdminFullDataAccessRepairsDenyStubs(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()

	psID, err := db.EnsureSystemAdminPermissionSet(ctx, d.Pool)
	if err != nil {
		t.Fatal(err)
	}
	_, err = d.Pool.Exec(ctx, `
UPDATE object_permissions
SET can_read = false, view_all = false, modify_all = false
WHERE permission_set_id = $1::uuid AND object_api_name = 'Account'`, psID)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureAdminFullDataAccess(ctx, d.Pool); err != nil {
		t.Fatal(err)
	}
	var viewAll, modifyAll bool
	err = d.Pool.QueryRow(ctx, `
SELECT view_all, modify_all FROM object_permissions
WHERE permission_set_id = $1::uuid AND object_api_name = 'Account'`, psID).Scan(&viewAll, &modifyAll)
	if err != nil {
		t.Fatal(err)
	}
	if !viewAll || !modifyAll {
		t.Fatalf("expected full Account access on Admin PS, view_all=%v modify_all=%v", viewAll, modifyAll)
	}
}

func cleanupHumanUsers(t *testing.T, d *testutil.Database) {
	t.Helper()
	_, err := d.Pool.Exec(t.Context(), `
DELETE FROM record_access_grants WHERE user_id IN (SELECT id FROM users WHERE principal_type = 'user');
DELETE FROM records WHERE owner_id IN (SELECT id FROM users WHERE principal_type = 'user')
   OR created_by_id IN (SELECT id FROM users WHERE principal_type = 'user')
   OR last_modified_by_id IN (SELECT id FROM users WHERE principal_type = 'user');
DELETE FROM identity_links WHERE user_id IN (SELECT id FROM users WHERE principal_type = 'user');
DELETE FROM user_permission_sets WHERE user_id IN (SELECT id FROM users WHERE principal_type = 'user');
DELETE FROM user_roles WHERE user_id IN (SELECT id FROM users WHERE principal_type = 'user');
DELETE FROM users WHERE principal_type = 'user'`)
	if err != nil {
		t.Fatalf("cleanup humans: %v", err)
	}
}

func containsAllCaps(raw string) bool {
	for _, c := range []string{
		"identity.users", "identity.integrations", "authz.manage", "metadata.build",
		"deploy.promote", "govern.network", "govern.agents", "govern.audit",
	} {
		if !strings.Contains(raw, c) {
			return false
		}
	}
	return true
}
