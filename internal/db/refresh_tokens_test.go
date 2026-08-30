package db_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestRefreshTokenRotateReuseExpiryFreeze(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()
	users := db.NewUserStore(d.Pool)
	suffix := time.Now().UnixNano()
	u, err := users.CreateSocialUser(ctx, fmt.Sprintf("rt-store-%d@example.com", suffix), "RT Store", "StandardUser")
	if err != nil {
		t.Fatal(err)
	}
	store := db.NewRefreshTokenStore(d.Pool)

	issued, err := authz.IssueRefreshToken(ctx, store, authz.IssueRefreshOpts{
		UserID:  u.ID,
		Azp:     authz.ControlIDEAzp,
		IdleTTL: time.Hour,
		AbsTTL:  24 * time.Hour,
		Bytes:   32,
	})
	if err != nil {
		t.Fatal(err)
	}
	if issued.Raw == "" || issued.Token.ID == "" || issued.Token.FamilyID == "" {
		t.Fatalf("incomplete issue: %+v", issued)
	}
	if got, err := store.GetByHash(ctx, authz.HashRefreshToken(issued.Raw)); err != nil || got.ID != issued.Token.ID {
		t.Fatalf("get-by-hash: %+v err=%v", got, err)
	}

	rotated, err := authz.RotateRefreshToken(ctx, store, issued.Raw, authz.ControlIDEAzp, time.Hour, 32)
	if err != nil {
		t.Fatal(err)
	}
	if rotated.Raw == issued.Raw {
		t.Fatal("expected new raw token")
	}
	if rotated.Token.FamilyID != issued.Token.FamilyID {
		t.Fatalf("family changed: %s vs %s", rotated.Token.FamilyID, issued.Token.FamilyID)
	}

	_, err = authz.RotateRefreshToken(ctx, store, issued.Raw, authz.ControlIDEAzp, time.Hour, 32)
	if err != authz.ErrRefreshReuse {
		t.Fatalf("reuse want ErrRefreshReuse got %v", err)
	}
	old, err := store.GetByHash(ctx, authz.HashRefreshToken(rotated.Raw))
	if err != nil {
		t.Fatal(err)
	}
	if old.RevokedAt == nil {
		t.Fatal("family sibling should be revoked after reuse")
	}

	u2, err := users.CreateSocialUser(ctx, fmt.Sprintf("rt-idle-%d@example.com", suffix), "RT Idle", "StandardUser")
	if err != nil {
		t.Fatal(err)
	}
	idle, err := authz.IssueRefreshToken(ctx, store, authz.IssueRefreshOpts{
		UserID:  u2.ID,
		Azp:     authz.ControlIDEAzp,
		IdleTTL: time.Millisecond,
		AbsTTL:  24 * time.Hour,
		Bytes:   32,
	})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(15 * time.Millisecond)
	if _, err := authz.RotateRefreshToken(ctx, store, idle.Raw, "", time.Hour, 32); err != authz.ErrInvalidRefresh {
		t.Fatalf("idle expiry want ErrInvalidRefresh got %v", err)
	}

	u3, err := users.CreateSocialUser(ctx, fmt.Sprintf("rt-freeze-%d@example.com", suffix), "RT Freeze", "StandardUser")
	if err != nil {
		t.Fatal(err)
	}
	live, err := authz.IssueRefreshToken(ctx, store, authz.IssueRefreshOpts{
		UserID:  u3.ID,
		Azp:     authz.ControlIDEAzp,
		IdleTTL: time.Hour,
		AbsTTL:  24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := users.FreezePrincipal(ctx, u3.ID, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := authz.RotateRefreshToken(ctx, store, live.Raw, "", time.Hour, 32); err != authz.ErrInvalidRefresh {
		t.Fatalf("frozen want ErrInvalidRefresh got %v", err)
	}

	u4, err := users.CreateSocialUser(ctx, fmt.Sprintf("rt-revoke-%d@example.com", suffix), "RT Revoke", "StandardUser")
	if err != nil {
		t.Fatal(err)
	}
	a, err := authz.IssueRefreshToken(ctx, store, authz.IssueRefreshOpts{UserID: u4.ID, Azp: authz.ControlIDEAzp})
	if err != nil {
		t.Fatal(err)
	}
	n, err := store.RevokeAllForUser(ctx, u4.ID)
	if err != nil || n < 1 {
		t.Fatalf("revoke all n=%d err=%v", n, err)
	}
	if _, err := authz.RotateRefreshToken(ctx, store, a.Raw, "", time.Hour, 32); err != authz.ErrInvalidRefresh {
		t.Fatalf("revoked want ErrInvalidRefresh got %v", err)
	}

	mismatch, err := authz.IssueRefreshToken(ctx, store, authz.IssueRefreshOpts{
		UserID: u.ID, Azp: authz.ControlIDEAzp, IdleTTL: time.Hour, AbsTTL: 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authz.RotateRefreshToken(ctx, store, mismatch.Raw, "other.client", time.Hour, 32); err != authz.ErrInvalidRefresh {
		t.Fatalf("azp mismatch want ErrInvalidRefresh got %v", err)
	}
}
