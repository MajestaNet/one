package authz_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/golang-jwt/jwt/v5"
)

func TestOIDCIgnoresIdPAdminClaims(t *testing.T) {
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
		"sub":              "user-admin-group",
		"email":            "bob@example.com",
		"name":             "Bob",
		"iss":              issuer,
		"aud":              audience,
		"token_use":        "id",
		"exp":              time.Now().Add(time.Hour).Unix(),
		"iat":              time.Now().Unix(),
		"cognito:groups":   []string{"admin", "one-admin"},
		"custom:one_admin": true,
		"one_scopes":       "client+metadata",
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
	if actor.IsAdmin {
		t.Fatalf("IdP admin claims must not elevate IsAdmin: %+v", actor)
	}
	if !actor.HasScope(authz.ScopeClient) || !actor.HasScope(authz.ScopeMetadata) {
		t.Fatalf("scopes should still resolve from claims without DB: %+v", actor.Scopes)
	}
}

func TestOIDCRejectsInvalidRemoteURLsBeforeJWKSFetch(t *testing.T) {
	v := authz.NewOIDCVerifier("not-an-absolute-url", "one-app", "file:///tmp/keys.json", nil)
	if _, err := v.Verify(t.Context(), "not-a-token"); err == nil {
		t.Fatal("invalid OIDC issuer/JWKS configuration must be rejected")
	}
}

func TestOIDCRejectsPlaintextRemoteURLs(t *testing.T) {
	v := authz.NewOIDCVerifier("http://idp.example.test", "one-app", "http://idp.example.test/keys", nil)
	if _, err := v.Verify(t.Context(), "not-a-token"); err == nil {
		t.Fatal("non-loopback HTTP OIDC configuration must be rejected")
	}
}

func TestVerifyIDTokenAcceptsMissingTokenUse(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer := "https://login.microsoftonline.com/tid/v2.0"
	audience := "entra-app"
	v := authz.NewOIDCVerifier(issuer, audience, "", nil)
	v.SetLocalKey(func(token *jwt.Token) (any, error) {
		return &key.PublicKey, nil
	})
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":   "entra-user-1",
		"email": "ada@contoso.test",
		"iss":   issuer,
		"aud":   audience,
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	})
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := v.VerifyIDToken(t.Context(), signed)
	if err != nil {
		t.Fatalf("Entra-shaped ID token without token_use must be accepted: %v", err)
	}
	if claims.Subject != "entra-user-1" {
		t.Fatalf("sub=%q", claims.Subject)
	}
}

func TestVerifyIDTokenRejectsAccessTokenUse(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer := "https://cognito-idp.test.example.com/pool"
	audience := "one-app"
	v := authz.NewOIDCVerifier(issuer, audience, "", nil)
	v.SetLocalKey(func(token *jwt.Token) (any, error) {
		return &key.PublicKey, nil
	})
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":       "user-1",
		"iss":       issuer,
		"aud":       audience,
		"token_use": "access",
		"exp":       time.Now().Add(time.Hour).Unix(),
		"iat":       time.Now().Unix(),
	})
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.VerifyIDToken(t.Context(), signed); err == nil {
		t.Fatal("token_use=access must be rejected")
	}
}

func TestVerifyIDTokenRejectsHMAC(t *testing.T) {
	issuer := "https://idp.example.test"
	audience := "one-app"
	v := authz.NewOIDCVerifier(issuer, audience, "", nil)
	v.SetLocalKey(func(token *jwt.Token) (any, error) {
		return []byte("attacker-controlled-secret"), nil
	})
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-1",
		"iss": issuer,
		"aud": audience,
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, err := tok.SignedString([]byte("attacker-controlled-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.VerifyIDToken(t.Context(), signed); err == nil {
		t.Fatal("HMAC ID token must be rejected")
	}
}

func TestVerifyIDTokenUsesDiscoveryJWKSNotGuessedPath(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	wrong, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		iss := "http://" + r.Host
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 iss,
			"authorization_endpoint": iss + "/authorize",
			"token_endpoint":         iss + "/token",
			"jwks_uri":               iss + "/real-jwks",
		})
	})
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{rsaPublicJWK(t, "wrong", &wrong.PublicKey)}})
	})
	mux.HandleFunc("/real-jwks", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{rsaPublicJWK(t, "real", &key.PublicKey)}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	v := authz.NewOIDCVerifier(srv.URL, "one-app", "", nil)
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "disco-user",
		"iss": srv.URL,
		"aud": "one-app",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})
	tok.Header["kid"] = "real"
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := v.VerifyIDToken(t.Context(), signed)
	if err != nil {
		t.Fatalf("token must verify against discovery jwks_uri, not guessed jwks.json: %v", err)
	}
	if claims.Subject != "disco-user" {
		t.Fatalf("sub=%q", claims.Subject)
	}
	if !strings.HasSuffix(v.JWKSURI, "/real-jwks") {
		t.Fatalf("JWKSURI=%q want discovery /real-jwks", v.JWKSURI)
	}
}

func TestDiscoverOIDCRejectsIssuerMismatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   "https://other.example.test",
			"jwks_uri": "https://other.example.test/keys",
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	if _, err := authz.DiscoverOIDC(t.Context(), srv.Client(), srv.URL); err == nil {
		t.Fatal("discovery issuer mismatch must be rejected")
	}
}

func rsaPublicJWK(t *testing.T, kid string, pub *rsa.PublicKey) map[string]any {
	t.Helper()
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	return map[string]any{
		"kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256", "n": n, "e": e,
	}
}
