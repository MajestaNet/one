package authlogin_test

import (
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/authlogin"
	"github.com/MajestaNet/ide/internal/db"
)

func TestPKCEChallengeRoundTrip(t *testing.T) {
	verifier, err := authlogin.RandomURLToken(32)
	if err != nil {
		t.Fatal(err)
	}
	challenge := authlogin.PKCEChallengeS256(verifier)
	if !db.VerifyPKCES256(verifier, challenge) {
		t.Fatalf("S256 verify failed")
	}
	if db.VerifyPKCES256(verifier+"x", challenge) {
		t.Fatalf("expected mismatch")
	}
}

func TestEmailDomainAllowed(t *testing.T) {
	if !authlogin.EmailDomainAllowed("a@ex.com", nil, false) {
		t.Fatal("empty allowlist should allow")
	}
	if !authlogin.EmailDomainAllowed("a@ex.com", []string{"ex.com"}, false) {
		t.Fatal("domain match")
	}
	if authlogin.EmailDomainAllowed("a@other.com", []string{"ex.com"}, false) {
		t.Fatal("domain miss")
	}
	if !authlogin.EmailDomainAllowed("", nil, true) {
		t.Fatal("emailless ok")
	}
	if authlogin.EmailDomainAllowed("", []string{"ex.com"}, false) {
		t.Fatal("emailless denied when allowlist set and okEmailless false")
	}
}

func TestInferProviderFromIssuer(t *testing.T) {
	if got := authlogin.InferProviderFromIssuer("https://accounts.google.com"); got != "google" {
		t.Fatalf("got %s", got)
	}
	if got := authlogin.InferProviderFromIssuer("https://appleid.apple.com"); got != "apple" {
		t.Fatalf("got %s", got)
	}
	if got := authlogin.InferProviderFromIssuer("https://cognito-idp.us-east-1.amazonaws.com/pool"); got != "cognito" {
		t.Fatalf("got %s", got)
	}
	if got := authlogin.InferProviderFromIssuer("https://okta.example.com"); got != "oidc" {
		t.Fatalf("got %s", got)
	}
	if got := authlogin.InferProviderFromIssuer("https://slack.com"); got != "slack" {
		t.Fatalf("got %s", got)
	}
	if !authlogin.IsSlackIssuer("https://slack.com/") {
		t.Fatal("expected slack issuer with trailing slash")
	}
	if authlogin.IsSlackIssuer("https://evil.example/slack.com") {
		t.Fatal("substring slack.com must not match")
	}
}

func TestSlackProviderConfiguredAndName(t *testing.T) {
	if p := authlogin.NewSlackProvider(authlogin.SlackConfig{}); p.Configured() {
		t.Fatal("empty slack config must not be configured")
	}
	p := authlogin.NewSlackProvider(authlogin.SlackConfig{ClientID: "cid", ClientSecret: "sec"})
	if !p.Configured() {
		t.Fatal("expected configured")
	}
	if p.Name() != "slack" {
		t.Fatalf("name=%q", p.Name())
	}
}

func TestFakeProviderExchange(t *testing.T) {
	f := &authlogin.FakeProvider{
		ProviderName: "google",
		Claims: authlogin.SubjectClaims{
			Issuer:  authlogin.IssuerGoogle,
			Subject: "sub-1",
			Email:   "",
			Name:    "Ada",
		},
	}
	c, err := f.Exchange(t.Context(), "code", "ver", "https://cb", "nonce")
	if err != nil {
		t.Fatal(err)
	}
	if c.Subject != "sub-1" || c.Email != "" || c.Nonce != "nonce" {
		t.Fatalf("%+v", c)
	}
}

func TestDevProviderRoundTrip(t *testing.T) {
	d := authlogin.NewDevProvider()
	if !d.Configured() {
		t.Fatal("dev should be configured")
	}
	cb := "http://localhost:8080/auth/v1/callback/dev"
	u, err := d.AuthCodeURL("state-1", "nonce-1", "challenge", cb)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u, "code=one-dev-login") || !strings.Contains(u, "state=state-1") {
		t.Fatalf("auth url %s", u)
	}
	claims, err := d.Exchange(t.Context(), "one-dev-login", "", cb, "nonce-1")
	if err != nil {
		t.Fatal(err)
	}
	if claims.Email != authlogin.DevEmail || claims.Subject != authlogin.DevSubject || claims.Provider != "dev" {
		t.Fatalf("%+v", claims)
	}
}
