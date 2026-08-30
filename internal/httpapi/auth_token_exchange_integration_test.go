package httpapi_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/httpapi"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestTokenExchangeJITHonorsPolicyAndIssuerScopedIdentity(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})

	ctx := t.Context()
	users := db.NewUserStore(d.Pool)
	authStore := db.NewInstallAuthStore(d.Pool)
	original, err := authStore.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	empty := ""
	falseValue := false
	emptyList := []string{}
	emptyProvisioning := db.ProvisioningConfig{}
	if _, err := authStore.Update(ctx, db.InstallAuthUpdate{
		OIDCIssuer:          &empty,
		OIDCAudience:        &empty,
		OIDCJWKSURI:         &empty,
		JITProvisionUsers:   &falseValue,
		AllowedEmailDomains: &emptyList,
		Provisioning:        &emptyProvisioning,
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = authStore.Update(context.Background(), db.InstallAuthUpdate{
			OIDCIssuer:           &original.OIDCIssuer,
			OIDCAudience:         &original.OIDCAudience,
			OIDCJWKSURI:          &original.OIDCJWKSURI,
			OIDCDisplayName:      &original.OIDCDisplayName,
			OIDCClientID:         &original.OIDCClientID,
			OIDCClientSecretEnc:  &original.OIDCClientSecretEnc,
			JITProvisionUsers:    &original.JITProvisionUsers,
			JITDefaultRole:       &original.JITDefaultRole,
			AllowedEmailDomains:  &original.AllowedEmailDomains,
			SocialProviders:      &original.SocialProviders,
			PasswordLoginEnabled: &original.PasswordLoginEnabled,
			Provisioning:         &original.Provisioning,
		})
	})

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	roleName := "OIDCJIT" + suffix
	if _, err := users.CreateRole(ctx, roleName, "OIDC JIT "+suffix, []string{"client"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = d.Pool.Exec(cleanupCtx, `DELETE FROM users WHERE email LIKE $1`, "%+"+suffix+"@review.test")
		_ = users.DeleteRole(cleanupCtx, roleName, true)
	})

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer := "https://idp.review.test/" + suffix
	audience := "review-client-" + suffix
	oidc := authz.NewOIDCVerifier(issuer, audience, "", []authz.Scope{authz.ScopeClient})
	oidc.SetLocalKey(func(*jwt.Token) (any, error) { return &key.PublicKey, nil })

	cfg := testutil.NewTestConfig(t, testutil.ServerOptions{APIKeys: "review-admin-key+admin", EnableJWT: true})
	cfg.OIDCEnabled = true
	cfg.OIDCIssuer = issuer
	cfg.OIDCAudience = audience
	cfg.OIDCAutoProvision = true
	cfg.AuthAutoProvisionUsers = true
	cfg.AuthAutoProvisionRole = roleName
	cfg.AuthLoginAllowedEmailDomains = []string{"review.test"}
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
		Users:       &db.AuthzUsers{Store: users},
	}
	h := httpapi.New(httpapi.Options{Config: cfg, Resolver: resolver, DB: d.Pool, Pool: d.Pool, Metadata: d.Meta}).Handler()

	exchange := func(subject, email string) *httptest.ResponseRecorder {
		t.Helper()
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"sub":            subject,
			"email":          email,
			"email_verified": true,
			"name":           "Review User",
			"iss":            issuer,
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
		return rr
	}

	deniedEmail := "denied+" + suffix + "@outside.test"
	denied := exchange("denied-sub-"+suffix, deniedEmail)
	if denied.Code != http.StatusForbidden || !strings.Contains(denied.Body.String(), "EMAIL_DOMAIN_DENIED") {
		t.Fatalf("domain-denied exchange: status=%d body=%s", denied.Code, denied.Body.String())
	}
	if _, err := users.GetByEmail(ctx, deniedEmail); err == nil {
		t.Fatal("domain-denied JIT must not create a principal")
	}
	hadHumanAdmin, err := users.AnyActiveHumanHasRole(ctx, db.SystemAdminRoleAPIName)
	if err != nil {
		t.Fatal(err)
	}

	allowedEmail := "allowed+" + suffix + "@review.test"
	allowed := exchange("allowed-sub-"+suffix, allowedEmail)
	if allowed.Code != http.StatusOK {
		t.Fatalf("allowed exchange: status=%d body=%s", allowed.Code, allowed.Body.String())
	}
	var tokenBody struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(allowed.Body.Bytes(), &tokenBody); err != nil {
		t.Fatal(err)
	}
	claims, err := resolver.One.Verify(tokenBody.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(claims.Roles, roleName) || slices.Contains(claims.Roles, "StandardUser") {
		t.Fatalf("JIT roles=%v want only configured role %s", claims.Roles, roleName)
	}
	if !hadHumanAdmin && !slices.Contains(claims.Roles, db.SystemAdminRoleAPIName) {
		t.Fatalf("first token-exchange human roles=%v want %s bootstrap", claims.Roles, db.SystemAdminRoleAPIName)
	}

	legacyEmail := "legacy+" + suffix + "@review.test"
	legacy, err := users.CreateWithGrants(ctx, db.CreatePrincipalInput{
		Email: legacyEmail, DisplayName: "Legacy Victim", PrincipalType: "user", RoleAPINames: []string{"StandardUser"},
	})
	if err != nil {
		t.Fatal(err)
	}
	sharedSubject := "shared-sub-" + suffix
	if _, err := d.Pool.Exec(ctx, `UPDATE users SET oidc_sub=$2 WHERE id=$1::uuid`, legacy.ID, sharedSubject); err != nil {
		t.Fatal(err)
	}
	attackerEmail := "attacker+" + suffix + "@review.test"
	collision := exchange(sharedSubject, attackerEmail)
	if collision.Code != http.StatusOK {
		t.Fatalf("collision exchange: status=%d body=%s", collision.Code, collision.Body.String())
	}
	link, err := db.NewIdentityLinkStore(d.Pool).GetByIssuerSubject(ctx, issuer, sharedSubject)
	if err != nil {
		t.Fatal(err)
	}
	if link.UserID == legacy.ID {
		t.Fatal("same sub at a new issuer must not bind to a legacy principal with a different email")
	}
	unchanged, err := users.GetByID(ctx, legacy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Email != legacyEmail {
		t.Fatalf("legacy principal email changed to %q", unchanged.Email)
	}
}
