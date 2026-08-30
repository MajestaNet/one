package authlogin

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestVerifyCustomerOIDCIDTokenStrictValidation(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer := "https://idp.example.test"
	audience := "control-ide"
	keyfunc := func(*jwt.Token) (any, error) { return &key.PublicKey, nil }
	sign := func(claims jwt.MapClaims, method jwt.SigningMethod) string {
		t.Helper()
		raw, err := jwt.NewWithClaims(method, claims).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	validClaims := func() jwt.MapClaims {
		return jwt.MapClaims{
			"iss":   issuer,
			"aud":   audience,
			"sub":   "subject-1",
			"exp":   time.Now().Add(time.Hour).Unix(),
			"nonce": "nonce-1",
		}
	}

	valid := sign(validClaims(), jwt.SigningMethodRS256)
	if _, err := verifyCustomerOIDCIDToken(valid, keyfunc, issuer, audience, "", "nonce-1"); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}

	withoutExpiry := validClaims()
	delete(withoutExpiry, "exp")
	if _, err := verifyCustomerOIDCIDToken(sign(withoutExpiry, jwt.SigningMethodRS256), keyfunc, issuer, audience, "", "nonce-1"); err == nil {
		t.Fatal("token without exp must be rejected")
	}

	withoutSubject := validClaims()
	delete(withoutSubject, "sub")
	if _, err := verifyCustomerOIDCIDToken(sign(withoutSubject, jwt.SigningMethodRS256), keyfunc, issuer, audience, "", "nonce-1"); err == nil {
		t.Fatal("token without sub must be rejected")
	}

	if _, err := verifyCustomerOIDCIDToken(valid, keyfunc, issuer, audience, "", "wrong-nonce"); err == nil {
		t.Fatal("nonce mismatch must be rejected")
	}
}

func TestVerifyCustomerOIDCIDTokenRejectsHMAC(t *testing.T) {
	claims := jwt.MapClaims{
		"iss": "https://idp.example.test",
		"aud": "control-ide",
		"sub": "subject-1",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("attacker-controlled-secret"))
	if err != nil {
		t.Fatal(err)
	}
	keyfunc := func(*jwt.Token) (any, error) { return []byte("attacker-controlled-secret"), nil }
	if _, err := verifyCustomerOIDCIDToken(raw, keyfunc, "https://idp.example.test", "control-ide", "", ""); err == nil {
		t.Fatal("HMAC token must be rejected for external OIDC")
	}
}

func TestValidateOIDCEndpoint(t *testing.T) {
	for _, valid := range []string{"https://idp.example.test/oauth/token", "http://localhost:8080/keys"} {
		if err := validateOIDCEndpoint("test", valid); err != nil {
			t.Fatalf("valid endpoint %q rejected: %v", valid, err)
		}
	}
	for _, invalid := range []string{"", "/relative", "javascript:alert(1)", "http://idp.example.test/token", "https://user:pass@idp.example.test/token"} {
		if err := validateOIDCEndpoint("test", invalid); err == nil {
			t.Fatalf("invalid endpoint %q accepted", invalid)
		}
	}
}
