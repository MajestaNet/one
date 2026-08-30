package authz_test

import (
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/golang-jwt/jwt/v5"
)

func TestOneMintAndVerify(t *testing.T) {
	signer := &authz.OneSigner{
		SigningKey: []byte("test-signing-key-32bytes-minimum!!"),
		Issuer:     "http://localhost:8080/auth/v1",
		TTL:        time.Hour,
	}
	actor := &authz.Actor{
		ID:            "00000000-0000-4000-8000-000000000001",
		PrincipalType: "service",
		Scopes:        []authz.Scope{authz.ScopeClient, authz.ScopeMetadata},
		IsAdmin:       true,
		Roles:         []string{"Admin"},
	}
	token, expiresIn, err := signer.MintAccessToken(actor)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || expiresIn != 3600 {
		t.Fatalf("token=%q expiresIn=%d", token, expiresIn)
	}
	if !authz.LooksLikeJWT(token) {
		t.Fatal("minted token should look like jwt")
	}

	claims, err := signer.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != actor.ID {
		t.Fatalf("sub=%s", claims.Subject)
	}
	if !claims.Admin {
		t.Fatal("expected admin")
	}
	scopes, admin := authz.ScopesFromClaims(claims.Scopes)
	if admin {
		t.Fatal("admin should not be in scopes list")
	}
	if len(scopes) != 2 {
		t.Fatalf("scopes=%v", scopes)
	}
}

func TestOneJWTRejectsMissingRequiredIdentityClaims(t *testing.T) {
	signer := &authz.OneSigner{
		SigningKey: []byte("test-signing-key-32bytes-minimum!!"),
		Issuer:     "http://localhost:8080/auth/v1",
		TTL:        time.Hour,
	}
	now := time.Now()
	base := authz.OneClaims{
		PrincipalType: "service",
		Scopes:        []string{"client"},
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    signer.Issuer,
			Subject:   "00000000-0000-4000-8000-000000000001",
			Audience:  jwt.ClaimStrings{authz.OneAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, base).SignedString(signer.SigningKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := signer.Verify(raw); err == nil {
		t.Fatal("token without iat must be rejected")
	}

	base.IssuedAt = jwt.NewNumericDate(now)
	base.PrincipalType = ""
	raw, err = jwt.NewWithClaims(jwt.SigningMethodHS256, base).SignedString(signer.SigningKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := signer.Verify(raw); err == nil {
		t.Fatal("token without principal_type must be rejected")
	}
}

func TestOneMintRejectsActorWithoutScope(t *testing.T) {
	signer := &authz.OneSigner{
		SigningKey: []byte("test-signing-key-32bytes-minimum!!"),
		Issuer:     "http://localhost:8080/auth/v1",
	}
	if _, _, err := signer.MintAccessToken(&authz.Actor{
		ID:            "00000000-0000-4000-8000-000000000001",
		PrincipalType: "service",
	}); err == nil {
		t.Fatal("actor without family scope must not receive an access token")
	}
}

func TestResolveBearerOneJWT(t *testing.T) {
	signer := &authz.OneSigner{
		SigningKey: []byte("test-signing-key-32bytes-minimum!!"),
		Issuer:     "http://localhost:8080/auth/v1",
		TTL:        time.Hour,
	}
	r := &authz.Resolver{
		Entries:        mustParseKeys(t, "dev-admin-key+admin"),
		DefaultOwnerID: "00000000-0000-4000-8000-000000000001",
		One:            signer,
	}
	token, _, err := signer.MintAccessToken(&authz.Actor{
		ID:            r.DefaultOwnerID,
		PrincipalType: "service",
		Scopes:        []authz.Scope{authz.ScopeClient},
	})
	if err != nil {
		t.Fatal(err)
	}
	actor, err := r.ResolveBearer(token)
	if err != nil {
		t.Fatal(err)
	}
	if actor.AuthMethod != authz.AuthMethodOneJWT {
		t.Fatalf("method=%s", actor.AuthMethod)
	}
	if !actor.HasScope(authz.ScopeClient) {
		t.Fatalf("scopes=%v", actor.Scopes)
	}
}

func TestResolveBearerStillAcceptsAPIKey(t *testing.T) {
	r := &authz.Resolver{
		Entries:        mustParseKeys(t, "dev-admin-key+admin"),
		DefaultOwnerID: "00000000-0000-4000-8000-000000000001",
		One: &authz.OneSigner{
			SigningKey: []byte("test-signing-key-32bytes-minimum!!"),
			Issuer:     "http://localhost:8080/auth/v1",
		},
	}
	actor, err := r.ResolveBearer("dev-admin-key")
	if err != nil {
		t.Fatal(err)
	}
	if actor.AuthMethod != "api_key" {
		t.Fatalf("method=%s", actor.AuthMethod)
	}
}

func mustParseKeys(t *testing.T, raw string) []authz.APIKeyEntry {
	t.Helper()
	e, err := authz.ParseAPIKeyEntries(raw)
	if err != nil {
		t.Fatal(err)
	}
	return e
}
