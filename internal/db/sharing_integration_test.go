package db_test

import (
	"context"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestSharingHierarchyVisibility(t *testing.T) {
	td := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, td, testutil.BootstrapOptions{})
	ctx := context.Background()
	pool := td.Pool

	const (
		mgrRole = "SalesManager_ShareTest"
		repRole = "SalesRep_ShareTest"
		mgrMail = "mgr-share@test.local"
		repMail = "rep-share@test.local"
	)

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM record_access_grants WHERE object_api_name = 'Account'`)
		_, _ = pool.Exec(ctx, `DELETE FROM records WHERE object_api_name = 'Account' AND data->>'Name' = 'AcmeShareTest'`)
		_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE job_type = 'sharing.recalc'`)
		_, _ = pool.Exec(ctx, `
DELETE FROM identity_links WHERE user_id IN (SELECT id FROM users WHERE email = ANY($1::text[]));
DELETE FROM user_permission_sets WHERE user_id IN (SELECT id FROM users WHERE email = ANY($1::text[]));
DELETE FROM user_roles WHERE user_id IN (SELECT id FROM users WHERE email = ANY($1::text[]));
UPDATE users SET data_role_id = NULL WHERE email = ANY($1::text[]);
DELETE FROM users WHERE email = ANY($1::text[])`, []string{mgrMail, repMail})
		_, _ = pool.Exec(ctx, `DELETE FROM data_roles WHERE api_name = ANY($1::text[])`, []string{mgrRole, repRole})
		_, _ = pool.Exec(ctx, `UPDATE organization_settings SET record_sharing_enabled = false, record_sharing_enabled_at = NULL WHERE id = true`)
		_, _ = pool.Exec(ctx, `UPDATE object_sharing_settings SET default_access = 'private', sharing_rules_enabled = false WHERE object_api_name = 'Account'`)
	}
	cleanup()
	t.Cleanup(cleanup)

	roleStore := db.NewDataRoleStore(pool)
	mgr, err := roleStore.CreateDataRole(ctx, mgrRole, "Sales Manager", nil)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := roleStore.CreateDataRole(ctx, repRole, "Sales Rep", &mgr.ID)
	if err != nil {
		t.Fatal(err)
	}

	userStore := db.NewUserStore(pool)
	mgrUser, err := userStore.CreateWithGrants(ctx, db.CreatePrincipalInput{
		Email: mgrMail, DisplayName: "Mgr", PrincipalType: "user",
		RoleAPINames: []string{"StandardUser"},
	})
	if err != nil {
		t.Fatal(err)
	}
	repUser, err := userStore.CreateWithGrants(ctx, db.CreatePrincipalInput{
		Email: repMail, DisplayName: "Rep", PrincipalType: "user",
		RoleAPINames: []string{"StandardUser"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = roleStore.SetUserDataRole(ctx, mgrUser.ID, &mgr.ID)
	_ = roleStore.SetUserDataRole(ctx, repUser.ID, &rep.ID)

	sharing := db.NewSharingStore(pool)
	_, err = sharing.EnableRecordSharing(ctx)
	if err != nil {
		t.Fatal(err)
	}
	re := true
	_, err = sharing.UpdateObjectSharingSettings(ctx, "Account", "private", &re)
	if err != nil {
		t.Fatal(err)
	}

	data := dataengine.NewService(pool, metadata.NewService(pool))
	rec, err := data.Create(ctx, "Account", map[string]any{"Name": "AcmeShareTest", "OwnerId": repUser.ID}, &authz.Actor{ID: repUser.ID})
	if err != nil {
		t.Fatal(err)
	}
	recID, _ := rec["Id"].(string)

	eval := db.NewRecordAccessEvaluator(pool)
	ok, err := eval.CanViewRecordFull(ctx, &authz.Actor{ID: mgrUser.ID, DataRoleID: mgr.ID}, recID, repUser.ID, repUser.ID, "Account", map[string]struct{}{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("manager should view rep-owned account via data-role hierarchy")
	}
}
