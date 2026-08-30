package httpapi_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authlogin"
	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/config"
	"github.com/MajestaNet/ide/internal/httpapi"
	"github.com/MajestaNet/ide/internal/identity"
	"github.com/golang-jwt/jwt/v5"
)

func TestAuthV1Token(t *testing.T) {
	cfg := &config.Config{
		Port:               8080,
		ProductVersion:     "0.1.0",
		CustomerID:         "t1",
		InstallID:          "i1",
		InstallRole:        "dev",
		APIKeyEntries:      mustKeys(t, "dev-admin-key+admin,dev-agent-key:client"),
		DefaultOwnerID:     "00000000-0000-4000-8000-000000000001",
		AuthJWTSigningKey:  "test-signing-key-32bytes-minimum!!",
		AuthJWTIssuer:      "http://localhost:8080/auth/v1",
		AuthJWTTTLSeconds:  3600,
		AuthJWTEnabled:     true,
		RequestBodyLimit:   1 << 20,
		RateLimitPerMinute: 0,
	}
	one := &authz.OneSigner{
		SigningKey: []byte(cfg.AuthJWTSigningKey),
		Issuer:     cfg.AuthJWTIssuer,
		TTL:        time.Hour,
	}
	resolver := &authz.Resolver{
		Entries:        cfg.APIKeyEntries,
		DefaultOwnerID: cfg.DefaultOwnerID,
		One:            one,
	}
	s := httpapi.New(httpapi.Options{Config: cfg, Resolver: resolver})
	h := s.Handler()

	t.Run("mint with api key as client_secret", func(t *testing.T) {
		body := `{"grant_type":"client_credentials","client_secret":"dev-admin-key"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/v1/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != 200 {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
		var tok map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &tok); err != nil {
			t.Fatal(err)
		}
		access, _ := tok["access_token"].(string)
		if access == "" || tok["token_type"] != "Bearer" {
			t.Fatalf("body=%v", tok)
		}
		if tok["refresh_token"] != nil && tok["refresh_token"] != "" {
			t.Fatalf("client_credentials must not issue refresh_token: %v", tok)
		}

		me := httptest.NewRequest(http.MethodGet, "/client/v1/me", nil)
		me.Header.Set("Authorization", "Bearer "+access)
		rr2 := httptest.NewRecorder()
		h.ServeHTTP(rr2, me)
		if rr2.Code != 200 {
			t.Fatalf("me status %d body=%s", rr2.Code, rr2.Body.String())
		}
		var meBody map[string]any
		_ = json.Unmarshal(rr2.Body.Bytes(), &meBody)
		if meBody["authMethod"] != authz.AuthMethodOneJWT {
			t.Fatalf("me=%v", meBody)
		}
	})

	t.Run("agent key token lacks metadata scope", func(t *testing.T) {
		body := `{"grant_type":"client_credentials","client_secret":"dev-agent-key"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/v1/token", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != 200 {
			t.Fatalf("status %d", rr.Code)
		}
		var tok map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &tok)
		access, _ := tok["access_token"].(string)

		req2 := httptest.NewRequest(http.MethodGet, "/metadata/v1/objects", nil)
		req2.Header.Set("Authorization", "Bearer "+access)
		rr2 := httptest.NewRecorder()
		h.ServeHTTP(rr2, req2)
		if rr2.Code != 403 {
			t.Fatalf("expected 403, got %d", rr2.Code)
		}
	})

	t.Run("exchange without oidc", func(t *testing.T) {
		body := `{"grant_type":"urn:ietf:params:oauth:grant-type:token-exchange","subject_token":"x"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/v1/token/exchange", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != 503 {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("discovery", func(t *testing.T) {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/auth/v1/.well-known/openid-configuration", nil))
		if rr.Code != 200 {
			t.Fatalf("status %d", rr.Code)
		}
		var disc map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &disc); err != nil {
			t.Fatal(err)
		}
		grants, _ := disc["grant_types_supported"].([]any)
		joined := fmt.Sprint(grants)
		if !strings.Contains(joined, "refresh_token") || !strings.Contains(joined, "password") {
			t.Fatalf("grant_types_supported=%v", grants)
		}
		if disc["revocation_endpoint"] != "http://localhost:8080/auth/v1/revoke" {
			t.Fatalf("revocation_endpoint=%v", disc["revocation_endpoint"])
		}
	})

	t.Run("invalid secret", func(t *testing.T) {
		body := `{"grant_type":"client_credentials","client_secret":"nope"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/v1/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != 401 {
			t.Fatalf("status %d", rr.Code)
		}
	})
}

func TestAuthV1UnavailableWithoutSigningKey(t *testing.T) {
	cfg := &config.Config{
		APIKeyEntries:      mustKeys(t, "dev-admin-key+admin"),
		DefaultOwnerID:     "00000000-0000-4000-8000-000000000001",
		RequestBodyLimit:   1 << 20,
		RateLimitPerMinute: 0,
	}
	resolver := &authz.Resolver{Entries: cfg.APIKeyEntries, DefaultOwnerID: cfg.DefaultOwnerID}
	s := httpapi.New(httpapi.Options{Config: cfg, Resolver: resolver})
	body := `{"grant_type":"client_credentials","client_secret":"dev-admin-key"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/v1/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != 503 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestAuthV1TokenExchange(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer := "https://cognito-idp.test.example.com/pool"
	audience := "one-ui"
	oidc := authz.NewOIDCVerifier(issuer, audience, "", []authz.Scope{authz.ScopeClient})
	oidc.SetLocalKey(func(token *jwt.Token) (any, error) {
		return &key.PublicKey, nil
	})
	cfg := &config.Config{
		DefaultOwnerID:     "00000000-0000-4000-8000-000000000001",
		APIKeyEntries:      mustKeys(t, "dev-admin-key+admin"),
		AuthJWTSigningKey:  "test-signing-key-32bytes-minimum!!",
		AuthJWTIssuer:      "http://localhost:8080/auth/v1",
		AuthJWTTTLSeconds:  3600,
		AuthJWTEnabled:     true,
		OIDCIssuer:         issuer,
		OIDCAudience:       audience,
		OIDCEnabled:        true,
		RequestBodyLimit:   1 << 20,
		RateLimitPerMinute: 0,
	}
	one := &authz.OneSigner{
		SigningKey: []byte(cfg.AuthJWTSigningKey),
		Issuer:     cfg.AuthJWTIssuer,
		TTL:        time.Hour,
	}
	resolver := &authz.Resolver{
		Entries:        cfg.APIKeyEntries,
		DefaultOwnerID: cfg.DefaultOwnerID,
		One:            one,
		OIDC:           oidc,
		OIDCDefault:    []authz.Scope{authz.ScopeClient},
	}
	s := httpapi.New(httpapi.Options{Config: cfg, Resolver: resolver})
	h := s.Handler()

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":       "cognito-user-1",
		"email":     "alice@example.com",
		"name":      "Alice",
		"iss":       issuer,
		"aud":       audience,
		"token_use": "id",
		"exp":       time.Now().Add(time.Hour).Unix(),
		"iat":       time.Now().Unix(),
	})
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{
		"grant_type":         "urn:ietf:params:oauth:grant-type:token-exchange",
		"subject_token":      signed,
		"subject_token_type": "urn:ietf:params:oauth:token-type:id_token",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/v1/token/exchange", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	access, _ := out["access_token"].(string)
	if access == "" {
		t.Fatalf("missing access_token: %v", out)
	}
	claims, err := one.Verify(access)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject == "" {
		t.Fatal("empty sub")
	}
	if claims.Azp != authz.InstallAzp {
		t.Fatalf("exchange default azp=%q want %s", claims.Azp, authz.InstallAzp)
	}

	t.Run("rejects access_token token_use", func(t *testing.T) {
		bad := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"sub":       "cognito-user-1",
			"iss":       issuer,
			"aud":       audience,
			"token_use": "access",
			"exp":       time.Now().Add(time.Hour).Unix(),
			"iat":       time.Now().Unix(),
		})
		signedBad, err := bad.SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		bodyBad, _ := json.Marshal(map[string]string{
			"grant_type":         "urn:ietf:params:oauth:grant-type:token-exchange",
			"subject_token":      signedBad,
			"subject_token_type": "urn:ietf:params:oauth:token-type:id_token",
		})
		reqBad := httptest.NewRequest(http.MethodPost, "/auth/v1/token/exchange", bytes.NewReader(bodyBad))
		reqBad.Header.Set("Content-Type", "application/json")
		rrBad := httptest.NewRecorder()
		h.ServeHTTP(rrBad, reqBad)
		if rrBad.Code != 401 {
			t.Fatalf("status %d body=%s", rrBad.Code, rrBad.Body.String())
		}
	})

	t.Run("accepts missing token_use", func(t *testing.T) {
		entra := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"sub":   "cognito-user-1",
			"email": "alice@example.com",
			"iss":   issuer,
			"aud":   audience,
			"exp":   time.Now().Add(time.Hour).Unix(),
			"iat":   time.Now().Unix(),
		})
		signedEntra, err := entra.SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		bodyEntra, _ := json.Marshal(map[string]string{
			"grant_type":         "urn:ietf:params:oauth:grant-type:token-exchange",
			"subject_token":      signedEntra,
			"subject_token_type": "urn:ietf:params:oauth:token-type:id_token",
		})
		reqEntra := httptest.NewRequest(http.MethodPost, "/auth/v1/token/exchange", bytes.NewReader(bodyEntra))
		reqEntra.Header.Set("Content-Type", "application/json")
		rrEntra := httptest.NewRecorder()
		h.ServeHTTP(rrEntra, reqEntra)
		if rrEntra.Code != 200 {
			t.Fatalf("status %d body=%s", rrEntra.Code, rrEntra.Body.String())
		}
	})
}

// TestAuthV1TokenExchangeRejectsForeignIssuer proves managed-channel horizontal isolation:
// a Cognito ID token from Pool A must not exchange on install B (different OIDC issuer).
func TestAuthV1TokenExchangeRejectsForeignIssuer(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuerA := "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_PoolA"
	issuerB := "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_PoolB"
	audienceB := "ui-client-b"

	oidcB := authz.NewOIDCVerifier(issuerB, audienceB, "", []authz.Scope{authz.ScopeClient})
	oidcB.SetLocalKey(func(token *jwt.Token) (any, error) {
		return &key.PublicKey, nil
	})
	cfg := &config.Config{
		DefaultOwnerID:     "00000000-0000-4000-8000-000000000001",
		APIKeyEntries:      mustKeys(t, "dev-admin-key+admin"),
		AuthJWTSigningKey:  "install-b-signing-key-32bytes!!!!",
		AuthJWTIssuer:      "https://b.example/auth/v1",
		AuthJWTTTLSeconds:  3600,
		AuthJWTEnabled:     true,
		OIDCIssuer:         issuerB,
		OIDCAudience:       audienceB,
		OIDCEnabled:        true,
		RequestBodyLimit:   1 << 20,
		RateLimitPerMinute: 0,
	}
	one := &authz.OneSigner{
		SigningKey: []byte(cfg.AuthJWTSigningKey),
		Issuer:     cfg.AuthJWTIssuer,
		TTL:        time.Hour,
	}
	resolver := &authz.Resolver{
		Entries:        cfg.APIKeyEntries,
		DefaultOwnerID: cfg.DefaultOwnerID,
		One:            one,
		OIDC:           oidcB,
		OIDCDefault:    []authz.Scope{authz.ScopeClient},
	}
	h := httpapi.New(httpapi.Options{Config: cfg, Resolver: resolver}).Handler()

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":       "cognito-user-a",
		"email":     "alice@customer-a.example",
		"iss":       issuerA,
		"aud":       audienceB,
		"token_use": "id",
		"exp":       time.Now().Add(time.Hour).Unix(),
		"iat":       time.Now().Unix(),
	})
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{
		"grant_type":         "urn:ietf:params:oauth:grant-type:token-exchange",
		"subject_token":      signed,
		"subject_token_type": "urn:ietf:params:oauth:token-type:id_token",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/v1/token/exchange", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Fatalf("expected 401 exchanging foreign Cognito issuer; got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCrossInstallOneJWTRejectedOnFamilyRoute(t *testing.T) {
	signerA := &authz.OneSigner{
		SigningKey: []byte("install-a-signing-key-32bytes!!!!"),
		Issuer:     "https://a.example/auth/v1",
		TTL:        time.Hour,
	}
	token, _, err := signerA.MintAccessToken(&authz.Actor{
		ID:            "00000000-0000-4000-8000-0000000000aa",
		PrincipalType: "user",
		Scopes:        []authz.Scope{authz.ScopeClient},
		Roles:         []string{"StandardUser"},
	})
	if err != nil {
		t.Fatal(err)
	}

	cfgB := &config.Config{
		DefaultOwnerID:     "00000000-0000-4000-8000-000000000001",
		APIKeyEntries:      mustKeys(t, "dev-admin-key+admin"),
		AuthJWTSigningKey:  "install-b-signing-key-32bytes!!!!",
		AuthJWTIssuer:      "https://b.example/auth/v1",
		AuthJWTTTLSeconds:  3600,
		AuthJWTEnabled:     true,
		RequestBodyLimit:   1 << 20,
		RateLimitPerMinute: 0,
	}
	oneB := &authz.OneSigner{
		SigningKey: []byte(cfgB.AuthJWTSigningKey),
		Issuer:     cfgB.AuthJWTIssuer,
		TTL:        time.Hour,
	}
	resolverB := &authz.Resolver{
		Entries:        cfgB.APIKeyEntries,
		DefaultOwnerID: cfgB.DefaultOwnerID,
		One:            oneB,
	}
	h := httpapi.New(httpapi.Options{Config: cfgB, Resolver: resolverB}).Handler()

	req := httptest.NewRequest(http.MethodGet, "/client/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Fatalf("install B must reject install A's Majesta One JWT; got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthV1TokenExchangeSlack(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	audience := "slack-client-id"
	oidc := authz.NewOIDCVerifier(authlogin.IssuerSlack, audience, "", []authz.Scope{authz.ScopeClient})
	oidc.SetLocalKey(func(token *jwt.Token) (any, error) {
		return &key.PublicKey, nil
	})
	cfg := &config.Config{
		DefaultOwnerID:        "00000000-0000-4000-8000-000000000001",
		APIKeyEntries:         mustKeys(t, "dev-admin-key+admin"),
		AuthJWTSigningKey:     "test-signing-key-32bytes-minimum!!",
		AuthJWTIssuer:         "http://localhost:8080/auth/v1",
		AuthJWTTTLSeconds:     3600,
		AuthJWTEnabled:        true,
		AuthLoginProviders:    []string{"slack"},
		AuthSlackClientID:     audience,
		AuthSlackClientSecret: "slack-secret",
		RequestBodyLimit:      1 << 20,
		RateLimitPerMinute:    0,
	}
	one := &authz.OneSigner{
		SigningKey: []byte(cfg.AuthJWTSigningKey),
		Issuer:     cfg.AuthJWTIssuer,
		TTL:        time.Hour,
	}
	resolver := &authz.Resolver{
		Entries:        cfg.APIKeyEntries,
		DefaultOwnerID: cfg.DefaultOwnerID,
		One:            one,
		OIDC:           oidc,
		OIDCDefault:    []authz.Scope{authz.ScopeClient},
	}
	h := httpapi.New(httpapi.Options{Config: cfg, Resolver: resolver}).Handler()

	sign := func(claims jwt.MapClaims) string {
		t.Helper()
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		raw, err := tok.SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	exchange := func(subject string) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(map[string]string{
			"grant_type":         "urn:ietf:params:oauth:grant-type:token-exchange",
			"subject_token":      subject,
			"subject_token_type": "urn:ietf:params:oauth:token-type:id_token",
		})
		req := httptest.NewRequest(http.MethodPost, "/auth/v1/token/exchange", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	t.Run("accepts slack id token without token_use", func(t *testing.T) {
		rr := exchange(sign(jwt.MapClaims{
			"sub":            "U123SLACK",
			"email":          "ada@workspace.test",
			"email_verified": true,
			"iss":            authlogin.IssuerSlack,
			"aud":            audience,
			"exp":            time.Now().Add(time.Hour).Unix(),
			"iat":            time.Now().Unix(),
		}))
		if rr.Code != 200 {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("rejects slack bot token", func(t *testing.T) {
		rr := exchange("xoxb-123-not-an-id-token")
		if rr.Code != 401 {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "INVALID_TOKEN") {
			t.Fatalf("body=%s", rr.Body.String())
		}
	})

	t.Run("unknown workspace issuer is 401", func(t *testing.T) {
		rr := exchange(sign(jwt.MapClaims{
			"sub":   "U999",
			"email": "ada@workspace.test",
			"iss":   "https://unknown-workspace.slack.com",
			"aud":   audience,
			"exp":   time.Now().Add(time.Hour).Unix(),
		}))
		if rr.Code != 401 {
			t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
		}
	})
}

func TestAuthV1TokenExchangeSlackDisabledProvider(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	audience := "slack-client-id"
	oidc := authz.NewOIDCVerifier(authlogin.IssuerSlack, audience, "", []authz.Scope{authz.ScopeClient})
	oidc.SetLocalKey(func(token *jwt.Token) (any, error) {
		return &key.PublicKey, nil
	})
	cfg := &config.Config{
		DefaultOwnerID:        "00000000-0000-4000-8000-000000000001",
		APIKeyEntries:         mustKeys(t, "dev-admin-key+admin"),
		AuthJWTSigningKey:     "test-signing-key-32bytes-minimum!!",
		AuthJWTIssuer:         "http://localhost:8080/auth/v1",
		AuthJWTTTLSeconds:     3600,
		AuthJWTEnabled:        true,
		AuthLoginProviders:    []string{"dev"},
		AuthSlackClientID:     audience,
		AuthSlackClientSecret: "slack-secret",
		RequestBodyLimit:      1 << 20,
		RateLimitPerMinute:    0,
	}
	resolver := &authz.Resolver{
		Entries:        cfg.APIKeyEntries,
		DefaultOwnerID: cfg.DefaultOwnerID,
		One: &authz.OneSigner{
			SigningKey: []byte(cfg.AuthJWTSigningKey),
			Issuer:     cfg.AuthJWTIssuer,
			TTL:        time.Hour,
		},
		OIDC:        oidc,
		OIDCDefault: []authz.Scope{authz.ScopeClient},
	}
	h := httpapi.New(httpapi.Options{Config: cfg, Resolver: resolver}).Handler()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":            "U123SLACK",
		"email":          "ada@workspace.test",
		"email_verified": true,
		"iss":            authlogin.IssuerSlack,
		"aud":            audience,
		"exp":            time.Now().Add(time.Hour).Unix(),
		"iat":            time.Now().Unix(),
	})
	raw, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{
		"grant_type":         "urn:ietf:params:oauth:grant-type:token-exchange",
		"subject_token":      raw,
		"subject_token_type": "urn:ietf:params:oauth:token-type:id_token",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/v1/token/exchange", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("disabled Slack want 403 got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "PROVIDER_DISABLED") {
		t.Fatalf("body=%s", rr.Body.String())
	}
}

func TestAuthV1LoginProvidersListsSlackWhenEnabled(t *testing.T) {
	cfg := &config.Config{
		APIKeyEntries:         mustKeys(t, "dev-admin-key+admin"),
		AuthJWTSigningKey:     "test-signing-key-32bytes-minimum!!",
		AuthJWTIssuer:         "http://localhost:8080/auth/v1",
		AuthJWTTTLSeconds:     3600,
		AuthJWTEnabled:        true,
		AuthLoginProviders:    []string{"slack"},
		AuthSlackClientID:     "slack-client",
		AuthSlackClientSecret: "slack-secret",
		RequestBodyLimit:      1 << 20,
	}
	h := httpapi.New(httpapi.Options{Config: cfg, Resolver: &authz.Resolver{
		Entries: cfg.APIKeyEntries, One: &authz.OneSigner{SigningKey: []byte(cfg.AuthJWTSigningKey), Issuer: cfg.AuthJWTIssuer, TTL: time.Hour},
	}}).Handler()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/auth/v1/login/providers", nil))
	if rr.Code != 200 {
		t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), identity.ProviderSlack) {
		t.Fatalf("expected slack in providers: %s", rr.Body.String())
	}
}

func TestAuthV1LoginProvidersOmitsSlackWhenDisabled(t *testing.T) {
	cfg := &config.Config{
		APIKeyEntries:         mustKeys(t, "dev-admin-key+admin"),
		AuthJWTSigningKey:     "test-signing-key-32bytes-minimum!!",
		AuthJWTIssuer:         "http://localhost:8080/auth/v1",
		AuthJWTTTLSeconds:     3600,
		AuthJWTEnabled:        true,
		AuthLoginProviders:    []string{"dev"},
		AuthSlackClientID:     "slack-client",
		AuthSlackClientSecret: "slack-secret",
		RequestBodyLimit:      1 << 20,
	}
	h := httpapi.New(httpapi.Options{Config: cfg, Resolver: &authz.Resolver{
		Entries: cfg.APIKeyEntries, One: &authz.OneSigner{SigningKey: []byte(cfg.AuthJWTSigningKey), Issuer: cfg.AuthJWTIssuer, TTL: time.Hour},
	}}).Handler()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/auth/v1/login/providers", nil))
	if rr.Code != 200 {
		t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), `"slack"`) {
		t.Fatalf("slack must not be listed when disabled: %s", rr.Body.String())
	}
}
