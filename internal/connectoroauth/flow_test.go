package connectoroauth

import "testing"

func TestNormalizeAndValidateFlow(t *testing.T) {
	if got := NormalizeAuthType("bearer"); got != AuthStaticBearer {
		t.Fatalf("got %s", got)
	}
	if err := ValidateAuthType("oauth2_authorization_code"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAuthType("nope"); err == nil {
		t.Fatal("expected error")
	}
	flow := Flow{
		AuthorizationURL: "https://login.example/oauth/authorize",
		TokenURL:         "https://login.example/oauth/token",
		ClientID:         "cid",
		Scopes:           []string{"a", "b"},
		PKCE:             true,
	}
	if err := ValidateFlow(AuthOAuth2AuthorizationCode, flow); err != nil {
		t.Fatal(err)
	}
	bad := flow
	bad.AuthorizationParams = map[string]string{"client_id": "x"}
	if err := ValidateFlow(AuthOAuth2AuthorizationCode, bad); err == nil {
		t.Fatal("expected reserved param error")
	}
	h1 := ConfigHash(AuthOAuth2AuthorizationCode, flow, "sec")
	h2 := ConfigHash(AuthOAuth2AuthorizationCode, flow, "sec")
	if h1 != h2 || h1 == "" {
		t.Fatalf("hash stable expected")
	}
	u, err := BuildAuthorizeURL(flow, "https://app.example/auth/v1/connectors/callback", "st", "chal")
	if err != nil || u == "" {
		t.Fatalf("authorize url: %v %s", err, u)
	}
}
