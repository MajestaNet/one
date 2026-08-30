package db_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
)

func TestObjectAuthzWithDB(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_ = pool.EnsureKernel(ctx)

	users := db.NewUserStore(pool)
	adminID := "00000000-0000-4000-8000-000000000002"
	_, err = users.EnsureBootstrapAdmin(ctx, adminID, "reader@one.local", "Reader")
	if err != nil {
		// may conflict if email taken with different id — use unique email
		adminID = "00000000-0000-4000-8000-000000000003"
		_, err = users.EnsureBootstrapAdmin(ctx, adminID, "goreader@one.local", "Reader")
		if err != nil {
			t.Fatal(err)
		}
	}
	// Force non-admin for object perm test
	_, _ = pool.Exec(ctx, `UPDATE users SET is_admin = false WHERE id = $1::uuid`, adminID)

	store := &db.ObjectPermStore{Pool: pool}
	_, _ = pool.Exec(ctx, `DELETE FROM permission_sets WHERE api_name = 'GoAccountRead'`)
	psID, err := store.CreatePermissionSet(ctx, "GoAccountRead", "Go Account Read", []authz.ObjectPermission{{
		ObjectAPIName: "Account",
		CanRead:       true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AssignPermissionSet(ctx, adminID, psID); err != nil {
		t.Fatal(err)
	}

	az := &authz.ObjectAuthz{Store: store}
	ids, _ := users.ListPermissionSetIDs(ctx, adminID)
	actor := &authz.Actor{ID: adminID, PermissionSetIDs: ids, IsAdmin: false}
	if err := az.AssertObjectAccess(ctx, actor, "Account", authz.ActionRead); err != nil {
		t.Fatal(err)
	}
	if err := az.AssertObjectAccess(ctx, actor, "Account", authz.ActionCreate); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("expected forbidden got %v", err)
	}
}
