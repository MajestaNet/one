package authz_test

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/golang-jwt/jwt/v5"
)

func TestParseAPIKeyEntries(t *testing.T) {
	entries, err := authz.ParseAPIKeyEntries("ops:client+metadata+deploy+admin,agent:client")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries", len(entries))
	}
	if !entries[0].IsAdmin {
		t.Fatal("expected admin marker on ops key")
	}
	if entries[1].Key != "agent" || len(entries[1].Scopes) != 1 || entries[1].Scopes[0] != authz.ScopeClient {
		t.Fatalf("unexpected agent entry: %+v", entries[1])
	}
	if entries[1].IsAdmin {
		t.Fatal("agent must not be admin")
	}
}

func TestParseAPIKeyEntriesRejectsDuplicateSecrets(t *testing.T) {
	if _, err := authz.ParseAPIKeyEntries("same-secret:client,same-secret:deploy"); err == nil {
		t.Fatal("duplicate key secrets with conflicting grants must be rejected")
	}
}

func TestResolveAPIKey(t *testing.T) {
	entries, _ := authz.ParseAPIKeyEntries("dev-admin-key+admin,dev-agent-key:client")
	r := &authz.Resolver{Entries: entries, DefaultOwnerID: "00000000-0000-4000-8000-000000000001"}
	actor, err := r.ResolveAPIKey("dev-admin-key")
	if err != nil {
		t.Fatal(err)
	}
	if !actor.IsAdmin || actor.AuthMethod != "api_key" {
		t.Fatalf("unexpected actor: %+v", actor)
	}
	if !actor.HasScope(authz.ScopeDeploy) {
		t.Fatal("admin key should have deploy")
	}
	// Key name containing "admin" without +admin marker is not privileged.
	plain, err := authz.ParseAPIKeyEntries("named-admin-key")
	if err != nil {
		t.Fatal(err)
	}
	r2 := &authz.Resolver{Entries: plain, DefaultOwnerID: "x"}
	a2, err := r2.ResolveAPIKey("named-admin-key")
	if err != nil {
		t.Fatal(err)
	}
	if a2.IsAdmin {
		t.Fatal("substring admin must not grant privilege")
	}
	agent, err := r.ResolveAPIKey("dev-agent-key")
	if err != nil {
		t.Fatal(err)
	}
	if agent.HasScope(authz.ScopeMetadata) {
		t.Fatal("agent should not have metadata")
	}
}

func TestOIDCResolve(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer := "https://cognito-idp.test.example.com/pool"
	audience := "one-app"
	v := authz.NewOIDCVerifier(issuer, audience, "", []authz.Scope{authz.ScopeClient})
	v.SetLocalKey(func(token *jwt.Token) (any, error) {
		return &key.PublicKey, nil
	})

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":        "user-1",
		"email":      "alice@example.com",
		"name":       "Alice",
		"iss":        issuer,
		"aud":        audience,
		"exp":        time.Now().Add(time.Hour).Unix(),
		"iat":        time.Now().Unix(),
		"one_scopes": "client+metadata",
	})
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}

	r := &authz.Resolver{OIDC: v, DefaultOwnerID: "x"}
	actor, err := r.ResolveOIDC(signed)
	if err != nil {
		t.Fatal(err)
	}
	if actor.AuthMethod != "oidc" || actor.Email != "alice@example.com" {
		t.Fatalf("unexpected: %+v", actor)
	}
	if !actor.HasScope(authz.ScopeMetadata) || !actor.HasScope(authz.ScopeClient) {
		t.Fatalf("scopes: %+v", actor.Scopes)
	}
}

func TestLooksLikeJWT(t *testing.T) {
	if !authz.LooksLikeJWT("aaa.bbb.ccc") {
		t.Fatal("expected jwt")
	}
	if authz.LooksLikeJWT("dev-admin-key") {
		t.Fatal("api key should not look like jwt")
	}
}

func TestParseOpsScope(t *testing.T) {
	entries, err := authz.ParseAPIKeyEntries("upgrade:ops+admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d", len(entries))
	}
	if !entries[0].IsAdmin || len(entries[0].Scopes) != 1 || entries[0].Scopes[0] != authz.ScopeOps {
		t.Fatalf("unexpected: %+v", entries[0])
	}
	bare, err := authz.ParseAPIKeyEntries("full-key")
	if err != nil {
		t.Fatal(err)
	}
	r := &authz.Resolver{Entries: bare, DefaultOwnerID: "x"}
	actor, err := r.ResolveAPIKey("full-key")
	if err != nil {
		t.Fatal(err)
	}
	if !actor.HasScope(authz.ScopeOps) {
		t.Fatal("bare key should include ops")
	}
}
