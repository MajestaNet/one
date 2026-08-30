package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/MajestaNet/ide/internal/authlogin"
	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/secretcrypt"
)

func (s *Server) registerInstallAuthRoutes(prefix string) {
	capMeta := func(cap string, h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeMetadata, s.requireCapability(cap, h)))
	}
	// Prefer identity.manage; CapIdentityManage satisfies CapIdentityUsers via CapabilitySatisfied.
	s.mux.Handle("GET "+prefix+"/install/auth", capMeta(authz.CapIdentityManage, s.handleGetInstallAuth))
	s.mux.Handle("PUT "+prefix+"/install/auth", capMeta(authz.CapIdentityManage, s.handlePutInstallAuth))
}

func installAuthResponse(st *db.InstallAuthSettings) map[string]any {
	return map[string]any{
		"claimed":              st.ClaimedAt != nil,
		"claimedAt":            st.ClaimedAt,
		"ssoConfigured":        strings.TrimSpace(st.OIDCIssuer) != "" && strings.TrimSpace(st.OIDCAudience) != "",
		"oidcIssuer":           st.OIDCIssuer,
		"oidcAudience":         st.OIDCAudience,
		"oidcJwksUri":          st.OIDCJWKSURI,
		"oidcDisplayName":      st.OIDCDisplayName,
		"oidcClientId":         st.OIDCClientID,
		"oidcClientSecretSet":  strings.TrimSpace(st.OIDCClientSecretEnc) != "",
		"jitProvisionUsers":    st.JITProvisionUsers,
		"jitDefaultRole":       st.JITDefaultRole,
		"allowedEmailDomains":  st.AllowedEmailDomains,
		"socialProviders":      st.SocialProviders,
		"passwordLoginEnabled": st.PasswordLoginEnabled,
		"provisioning":         st.Provisioning,
	}
}

func (s *Server) handleGetInstallAuth(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	st, err := db.NewInstallAuthStore(pool).Get(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, installAuthResponse(st))
}

type installAuthPutBody struct {
	OIDCIssuer            *string                `json:"oidcIssuer"`
	OIDCAudience          *string                `json:"oidcAudience"`
	OIDCJWKSURI           *string                `json:"oidcJwksUri"`
	OIDCDisplayName       *string                `json:"oidcDisplayName"`
	OIDCClientID          *string                `json:"oidcClientId"`
	OIDCClientSecret      *string                `json:"oidcClientSecret"`
	ClearOIDCClientSecret *bool                  `json:"clearOidcClientSecret"`
	JITProvisionUsers     *bool                  `json:"jitProvisionUsers"`
	JITDefaultRole        *string                `json:"jitDefaultRole"`
	AllowedEmailDomains   *[]string              `json:"allowedEmailDomains"`
	SocialProviders       *[]string              `json:"socialProviders"`
	PasswordLoginEnabled  *bool                  `json:"passwordLoginEnabled"`
	Provisioning          *db.ProvisioningConfig `json:"provisioning"`
}

func (s *Server) handlePutInstallAuth(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body installAuthPutBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	upd := db.InstallAuthUpdate{
		OIDCIssuer:           body.OIDCIssuer,
		OIDCAudience:         body.OIDCAudience,
		OIDCJWKSURI:          body.OIDCJWKSURI,
		OIDCDisplayName:      body.OIDCDisplayName,
		OIDCClientID:         body.OIDCClientID,
		JITProvisionUsers:    body.JITProvisionUsers,
		JITDefaultRole:       body.JITDefaultRole,
		AllowedEmailDomains:  body.AllowedEmailDomains,
		SocialProviders:      body.SocialProviders,
		PasswordLoginEnabled: body.PasswordLoginEnabled,
		Provisioning:         body.Provisioning,
	}
	if body.Provisioning != nil {
		if err := s.validateProvisioning(r.Context(), pool, *body.Provisioning); err != nil {
			writeAPIError(w, err)
			return
		}
	}
	if body.ClearOIDCClientSecret != nil && *body.ClearOIDCClientSecret {
		upd.ClearOIDCClientSecret = true
	} else if body.OIDCClientSecret != nil {
		sec := strings.TrimSpace(*body.OIDCClientSecret)
		if sec != "" {
			enc, err := protectInstallAuthClientSecret(sec, s.encKey())
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "SECRET_ERROR", "failed to protect OIDC client secret")
				return
			}
			upd.OIDCClientSecretEnc = &enc
		}
	}
	st, err := db.NewInstallAuthStore(pool).Update(r.Context(), upd)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "install.auth.put", "", nil, map[string]any{
		"ssoConfigured":     strings.TrimSpace(st.OIDCIssuer) != "",
		"jitProvisionUsers": st.JITProvisionUsers,
		"socialProviders":   st.SocialProviders,
	})
	writeJSON(w, http.StatusOK, installAuthResponse(st))
}

func protectInstallAuthClientSecret(secret, key string) (string, error) {
	return secretcrypt.Encrypt(strings.TrimSpace(secret), key)
}

func revealInstallAuthClientSecret(stored, key string) (string, error) {
	stored = strings.TrimSpace(stored)
	if strings.HasPrefix(stored, "plain:") {
		// Compatibility for installs configured before encrypted storage shipped.
		// A subsequent PUT rotates the value into the enc:v1 envelope.
		return strings.TrimPrefix(stored, "plain:"), nil
	}
	return secretcrypt.Decrypt(stored, key)
}

// loadInstallAuth returns one request-scoped settings snapshot. A configured
// database error is surfaced so authentication policy never falls back open.
func (s *Server) loadInstallAuth(r *http.Request) (*db.InstallAuthSettings, error) {
	if s.pool == nil {
		return nil, nil
	}
	st, err := db.NewInstallAuthStore(s.pool).Get(r.Context())
	if err != nil {
		return nil, err
	}
	return st, nil
}

// effectiveOIDCVerifier returns a verifier from a DB settings snapshot when configured, else env resolver.OIDC.
func (s *Server) effectiveOIDCVerifier(st *db.InstallAuthSettings) *authz.OIDCVerifier {
	if st != nil {
		issuer := strings.TrimSpace(st.OIDCIssuer)
		aud := strings.TrimSpace(st.OIDCAudience)
		if issuer != "" && aud != "" {
			return authz.NewOIDCVerifier(issuer, aud, strings.TrimSpace(st.OIDCJWKSURI), nil)
		}
	}
	if s.resolver != nil {
		return s.resolver.OIDC
	}
	return nil
}

func (s *Server) slackCredentialsConfigured() bool {
	return s != nil && s.cfg != nil &&
		strings.TrimSpace(s.cfg.AuthSlackClientID) != "" &&
		strings.TrimSpace(s.cfg.AuthSlackClientSecret) != ""
}

// effectiveSlackVerifier returns a Slack OpenID ID-token verifier when lab secrets are set.
func (s *Server) effectiveSlackVerifier(_ *http.Request) *authz.OIDCVerifier {
	if !s.slackCredentialsConfigured() {
		return nil
	}
	if s.resolver != nil && s.resolver.OIDC != nil && s.resolver.OIDC.Enabled() &&
		authlogin.IsSlackIssuer(s.resolver.OIDC.Issuer) {
		return s.resolver.OIDC
	}
	return authz.NewOIDCVerifier(authlogin.IssuerSlack, strings.TrimSpace(s.cfg.AuthSlackClientID), "https://slack.com/openid/connect/keys", nil)
}
