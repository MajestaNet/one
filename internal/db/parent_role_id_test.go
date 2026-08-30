package db_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/identity"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestIdentityLinkSlackProviderExact(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()
	users := db.NewUserStore(d.Pool)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	email := "slack-link-" + suffix + "@example.com"
	subject := "U123SLACK_" + suffix
	u, err := users.CreateSocialUser(ctx, email, "Slack Link", "StandardUser")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(ctx, `
DELETE FROM identity_links WHERE user_id = $1::uuid;
DELETE FROM user_roles WHERE user_id = $1::uuid;
DELETE FROM users WHERE id = $1::uuid`, u.ID)
	})
	link, err := db.NewIdentityLinkStore(d.Pool).Upsert(ctx, u.ID, identity.ProviderSlack, "https://slack.com", subject)
	if err != nil {
		t.Fatal(err)
	}
	if link.Provider != "slack" {
		t.Fatalf("provider=%q want slack", link.Provider)
	}
	got, err := db.NewIdentityLinkStore(d.Pool).GetBySubject(ctx, "slack", "https://slack.com", subject)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != u.ID || got.Provider != identity.ProviderSlack {
		t.Fatalf("got %+v", got)
	}
}

func TestListRoleGrantsDoesNotInheritParentRoleID(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()
	users := db.NewUserStore(d.Pool)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	parentName := "ParentRoleBP013_" + suffix
	childName := "ChildRoleBP013_" + suffix

	parent, err := users.CreateRole(ctx, parentName, "Parent Role", []string{"client", "metadata"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(ctx, `
DELETE FROM role_api_scopes WHERE role_id = $1::uuid;
UPDATE roles SET parent_role_id = NULL WHERE parent_role_id = $1::uuid;
DELETE FROM roles WHERE id = $1::uuid`, parent.ID)
	})
	child, err := users.CreateRole(ctx, childName, "Child Role", []string{"client"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(ctx, `
DELETE FROM role_api_scopes WHERE role_id = $1::uuid;
UPDATE roles SET parent_role_id = NULL WHERE id = $1::uuid;
DELETE FROM roles WHERE id = $1::uuid`, child.ID)
	})
	if _, err := d.Pool.Exec(ctx, `UPDATE roles SET parent_role_id = $1::uuid WHERE id = $2::uuid`, parent.ID, child.ID); err != nil {
		t.Fatal(err)
	}
	u, err := users.CreateWithGrants(ctx, db.CreatePrincipalInput{
		Email: "child-role-" + suffix + "@example.com", DisplayName: "Child", PrincipalType: "user",
		RoleAPINames: []string{childName},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(ctx, `
DELETE FROM user_roles WHERE user_id = $1::uuid;
DELETE FROM users WHERE id = $1::uuid`, u.ID)
	})

	scopes, admin, names, err := users.ListRoleGrants(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if admin {
		t.Fatal("child role must not inherit admin from parent_role_id")
	}
	if len(names) != 1 || names[0] != childName {
		t.Fatalf("roles=%v", names)
	}
	if len(scopes) != 1 || scopes[0] != authz.ScopeClient {
		t.Fatalf("scopes=%v (must not inherit metadata from parent_role_id)", scopes)
	}
}

func TestListRoleGrantsSQLOmitsParentRoleID(t *testing.T) {
	src, err := os.ReadFile("users.go")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(src), "func (s *UserStore) ListRoleGrants")
	if start < 0 {
		t.Fatal("ListRoleGrants not found")
	}
	chunk := string(src[start:])
	if i := strings.Index(chunk, "\nfunc "); i > 0 {
		chunk = chunk[:i]
	}
	if strings.Contains(chunk, "parent_role_id") {
		t.Fatal("ListRoleGrants SQL must not walk roles.parent_role_id")
	}
}
