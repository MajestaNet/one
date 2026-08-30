package db_test

import (
	"errors"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestDeriveDirectoryTagAPIName(t *testing.T) {
	cases := map[string]string{
		"Sales":       "Sales",
		"Okta Sales":  "OktaSales",
		"sales-team":  "SalesTeam",
		"  ":          "Tag",
		"123":         "T123",
		"okta-grp-01": "OktaGrp01",
	}
	for in, want := range cases {
		if got := db.DeriveDirectoryTagAPIName(in); got != want {
			t.Errorf("DeriveDirectoryTagAPIName(%q)=%q want %q", in, got, want)
		}
	}
	if err := db.ValidateDirectoryTagAPIName("OktaSales"); err != nil {
		t.Fatal(err)
	}
	if err := db.ValidateDirectoryTagAPIName("1Bad"); err == nil {
		t.Fatal("expected invalid apiName")
	}
}

func TestDirectoryTagStoreCRUD(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()
	store := db.NewDirectoryTagStore(d.Pool)
	stamp := time.Now().Format("150405.000000")

	tag, err := store.Create(ctx, db.CreateDirectoryTagInput{
		DisplayName: "Sales " + stamp,
		ExternalID:  "ext-" + stamp,
		AutoAPIName: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if tag.APIName == "" || tag.MemberCount != 0 {
		t.Fatalf("unexpected tag %+v", tag)
	}

	_, err = store.Create(ctx, db.CreateDirectoryTagInput{
		DisplayName: "Sales " + stamp,
		AutoAPIName: true,
	})
	if !errors.Is(err, db.ErrConflict) {
		t.Fatalf("expected duplicate displayName conflict, got %v", err)
	}

	users := db.NewUserStore(d.Pool)
	u, err := users.CreateWithGrants(ctx, db.CreatePrincipalInput{
		Email:         "tag-member-" + stamp + "@example.com",
		DisplayName:   "Tagged",
		PrincipalType: "user",
		RoleAPINames:  []string{"StandardUser"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Assign(ctx, u.ID, tag.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.Assign(ctx, u.ID, tag.ID); err != nil {
		t.Fatal(err)
	}
	names, err := store.ListAPINamesForUser(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != tag.APIName {
		t.Fatalf("names=%v", names)
	}
	if err := store.Unassign(ctx, u.ID, tag.ID); err != nil {
		t.Fatal(err)
	}
	names, err = store.ListAPINamesForUser(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Fatalf("expected empty after unassign, got %v", names)
	}
	if err := store.Delete(ctx, tag.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := users.GetByID(ctx, u.ID); err != nil {
		t.Fatalf("user should remain after tag delete: %v", err)
	}
}
