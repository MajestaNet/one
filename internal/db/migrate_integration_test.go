package db_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
)

func TestMigrateAndUsers(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Idempotent second run
	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatalf("remigrate: %v", err)
	}

	store := db.NewUserStore(pool)
	adminID := "00000000-0000-4000-8000-000000000001"
	admin, err := store.EnsureBootstrapAdmin(ctx, adminID, "admin@one.local", "Admin")
	if err != nil {
		t.Fatal(err)
	}
	if !admin.IsAdmin || admin.Email != "admin@one.local" {
		t.Fatalf("admin=%+v", admin)
	}

	u, err := store.EnsureOIDCUser(ctx,
		"11111111-1111-4111-a111-111111111111",
		"cognito-sub-phase2",
		"alice@example.com",
		"Alice",
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if u.OIDCSub == nil || *u.OIDCSub != "cognito-sub-phase2" {
		t.Fatalf("oidc user=%+v", u)
	}
	if u.PrincipalType != "user" {
		t.Fatalf("principal_type=%q want user", u.PrincipalType)
	}
	hasRole, err := store.HasAnyRole(ctx, u.ID)
	if err != nil || !hasRole {
		t.Fatalf("OIDC user should have StandardUser role, hasRole=%v err=%v", hasRole, err)
	}

	again, err := store.EnsureOIDCUser(ctx, u.ID, "cognito-sub-phase2", "alice@example.com", "Alice Updated", true)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != u.ID || again.DisplayName != "Alice Updated" {
		t.Fatalf("reuse failed: %+v", again)
	}
	if _, err := store.EnsureOIDCUser(ctx,
		"22222222-2222-4222-a222-222222222222",
		"attacker-controlled-subject",
		"alice@example.com",
		"Not Alice",
		true,
	); err == nil {
		t.Fatal("a new OIDC subject must not attach to an existing principal by matching email")
	}

	links := db.NewIdentityLinkStore(pool)
	subject := "rebind-test-" + time.Now().UTC().Format("20060102150405.000000000")
	link, err := links.Upsert(ctx, u.ID, "oidc", "https://issuer.example.test", subject)
	if err != nil {
		t.Fatal(err)
	}
	if link.UserID != u.ID {
		t.Fatalf("link user=%s want %s", link.UserID, u.ID)
	}
	if _, err := links.Upsert(ctx, admin.ID, "oidc", "https://issuer.example.test", subject); !errors.Is(err, db.ErrConflict) {
		t.Fatalf("identity rebind error=%v want conflict", err)
	}
	linked, err := links.GetBySubject(ctx, "oidc", "https://issuer.example.test", subject)
	if err != nil || linked.UserID != u.ID {
		t.Fatalf("identity link was rebound: %+v err=%v", linked, err)
	}

	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM one_schema_migrations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n < 8 {
		t.Fatalf("expected >=8 migrations recorded, got %d", n)
	}

	// EnsureAPIKeyServicePrincipal must work with unique lower(email) after ADR-015 email migrations.
	keyUser, keyErr := store.EnsureAPIKeyServicePrincipal(ctx, "ci-agent-key", false, []authz.Scope{authz.ScopeClient})
	if keyErr != nil {
		t.Fatalf("ensure ci-agent-key principal: %v", keyErr)
	}
	if keyUser.APIKeyName == nil || *keyUser.APIKeyName != authz.APIKeyIdentifier("ci-agent-key") {
		t.Fatalf("api_key_name=%v", keyUser.APIKeyName)
	}
	var (
		keyScopes []authz.Scope
		keyAdmin  bool
		keyRoles  []string
	)
	keyScopes, keyAdmin, keyRoles, keyErr = store.ListRoleGrants(ctx, keyUser.ID)
	if keyErr != nil {
		t.Fatal(keyErr)
	}
	if keyAdmin {
		t.Fatal("ci-agent-key must not be admin")
	}
	hasClient, hasMetadata := false, false
	for _, sc := range keyScopes {
		if sc == authz.ScopeClient {
			hasClient = true
		}
		if sc == authz.ScopeMetadata {
			hasMetadata = true
		}
	}
	if !hasClient || hasMetadata {
		t.Fatalf("ci-agent-key scopes=%v roles=%v want client only", keyScopes, keyRoles)
	}
	// Reusing a key with narrower/different config must replace, not accumulate,
	// its Role scopes.
	againKey, keyErr := store.EnsureAPIKeyServicePrincipal(ctx, "ci-agent-key", false, []authz.Scope{authz.ScopeOps})
	if keyErr != nil || againKey.ID != keyUser.ID {
		t.Fatalf("reuse ci-agent-key: %+v err=%v", againKey, keyErr)
	}
	keyScopes, keyAdmin, _, keyErr = store.ListRoleGrants(ctx, againKey.ID)
	if keyErr != nil {
		t.Fatal(keyErr)
	}
	if keyAdmin || len(keyScopes) != 1 || keyScopes[0] != authz.ScopeOps {
		t.Fatalf("downgraded ci-agent-key scopes=%v admin=%v want ops only", keyScopes, keyAdmin)
	}
}

func TestScrubBootstrapAPIKeyMigrationRemovesLegacySecretMetadata(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatal(err)
	}

	secret := "legacy-bootstrap-secret-" + time.Now().UTC().Format("20060102150405.000000000")
	var userID string
	if err := pool.QueryRow(ctx, `
INSERT INTO users (email, display_name, principal_type, api_key_name)
VALUES ($1, $2, 'service', $3)
RETURNING id::text`, "apikey+"+secret+"@one.local", "API Key "+secret, secret).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1::uuid`, userID) }()

	migrationsDir, err := db.ResolveMigrationsPath()
	if err != nil {
		t.Fatal(err)
	}
	sql, err := os.ReadFile(filepath.Join(migrationsDir, "0049_scrub_bootstrap_api_key_secrets.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		t.Fatal(err)
	}

	var identifier, email, display string
	if err := pool.QueryRow(ctx, `
SELECT api_key_name, email, display_name FROM users WHERE id=$1::uuid`, userID,
	).Scan(&identifier, &email, &display); err != nil {
		t.Fatal(err)
	}
	if identifier != authz.APIKeyIdentifier(secret) {
		t.Fatalf("identifier=%q want %q", identifier, authz.APIKeyIdentifier(secret))
	}
	if strings.Contains(email, secret) || strings.Contains(display, secret) {
		t.Fatalf("legacy secret remains in metadata: email=%q display=%q", email, display)
	}
}
