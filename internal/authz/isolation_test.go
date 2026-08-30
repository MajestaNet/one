package authz_test

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/golang-jwt/jwt/v5"
)

// Community managed-channel horizontal isolation notes (sdk/aws/docs/managed-channel-security.md).
// Install A credentials must never verify on install B's AuthN material.

func TestCrossInstallOneJWTRejected(t *testing.T) {
	signerA := &authz.OneSigner{
		SigningKey: []byte("install-a-signing-key-32bytes!!!!"),
		Issuer:     "https://a.example/auth/v1",
		TTL:        time.Hour,
	}
	signerB := &authz.OneSigner{
		SigningKey: []byte("install-b-signing-key-32bytes!!!!"),
		Issuer:     "https://b.example/auth/v1",
		TTL:        time.Hour,
	}
	token, _, err := signerA.MintAccessToken(&authz.Actor{
		ID:            "00000000-0000-4000-8000-0000000000aa",
		PrincipalType: "user",
		Scopes:        []authz.Scope{authz.ScopeClient, authz.ScopeMetadata, authz.ScopeDeploy, authz.ScopeOps},
		IsAdmin:       true,
		Roles:         []string{"Admin"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := signerA.Verify(token); err != nil {
		t.Fatalf("A must accept its own JWT: %v", err)
	}
	if _, err := signerB.Verify(token); err == nil {
		t.Fatal("install B must reject Majesta One JWT minted by install A (different signing key / issuer)")
	}
}

func TestCrossInstallOIDCIssuerRejected(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuerA := "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_PoolA"
	issuerB := "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_PoolB"
	audience := "ui-client-shared-name-ok" // same client id string must still fail on issuer

	verifierB := authz.NewOIDCVerifier(issuerB, audience, "", []authz.Scope{authz.ScopeClient})
	verifierB.SetLocalKey(func(token *jwt.Token) (any, error) {
		return &key.PublicKey, nil
	})

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":       "cognito-user-a",
		"email":     "alice@customer-a.example",
		"iss":       issuerA,
		"aud":       audience,
		"token_use": "id",
		"exp":       time.Now().Add(time.Hour).Unix(),
		"iat":       time.Now().Unix(),
	})
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifierB.VerifyIDToken(t.Context(), signed); err == nil {
		t.Fatal("install B must reject Cognito ID token whose iss is Pool A")
	}
}

func TestCrossInstallOIDCAudienceRejected(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer := "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_PoolB"
	verifierB := authz.NewOIDCVerifier(issuer, "ui-client-b", "", []authz.Scope{authz.ScopeClient})
	verifierB.SetLocalKey(func(token *jwt.Token) (any, error) {
		return &key.PublicKey, nil
	})

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":       "cognito-user-a",
		"iss":       issuer,
		"aud":       "ui-client-a",
		"token_use": "id",
		"exp":       time.Now().Add(time.Hour).Unix(),
		"iat":       time.Now().Unix(),
	})
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifierB.VerifyIDToken(t.Context(), signed); err == nil {
		t.Fatal("install B must reject ID token with foreign app-client aud")
	}
}
