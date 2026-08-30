package identity

import "testing"

func TestCognitoConfigConfigured(t *testing.T) {
	if (CognitoConfig{}).Configured() {
		t.Fatal("empty config")
	}
	if !(CognitoConfig{UserPoolID: "pool"}).Configured() {
		t.Fatal("expected configured")
	}
	if (CognitoConfig{UserPoolID: "  "}).Configured() {
		t.Fatal("whitespace is not configured")
	}
}

func TestCognitoBackendEnabledAndMode(t *testing.T) {
	var none *CognitoBackend
	if none.Enabled() {
		t.Fatal("nil backend")
	}
	b := &CognitoBackend{Config: CognitoConfig{UserPoolID: "p"}}
	if !b.Enabled() || b.Mode() != "cognito" {
		t.Fatalf("enabled=%v mode=%s", b.Enabled(), b.Mode())
	}
}

func TestDefaultM2MAppClientSpec(t *testing.T) {
	spec := DefaultM2MAppClientSpec("svc", "service")
	if spec.Name != "svc" || !spec.Confidential || !spec.GenerateSecret {
		t.Fatalf("%+v", spec)
	}
	if len(spec.OAuthFlows) != 1 || spec.OAuthFlows[0] != FlowClientCredentials {
		t.Fatalf("flows=%v", spec.OAuthFlows)
	}
}
