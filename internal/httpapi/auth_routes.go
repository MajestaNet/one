package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/MajestaNet/ide/internal/authlogin"
	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/identity"
)

func (s *Server) registerAuthRoutes() {
	s.mux.HandleFunc("POST /auth/v1/token", s.handleAuthToken)
	s.mux.HandleFunc("POST /auth/v1/token/exchange", s.handleAuthTokenExchange)
	s.mux.HandleFunc("POST /auth/v1/revoke", s.handleAuthRevoke)
	s.mux.HandleFunc("GET /auth/v1/.well-known/openid-configuration", s.handleAuthOIDCDiscovery)
	s.registerSocialAuthRoutes()
	s.registerInstallClaimRoutes()
	s.registerConnectorOAuthRoutes()
}

type tokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
	CodeVerifier string `json:"code_verifier"`
	Scope        string `json:"scope"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	RefreshToken string `json:"refresh_token"`
	DeviceID     string `json:"device_id"`
}

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int64  `json:"expires_in"`
	Scope            string `json:"scope,omitempty"`
	RefreshToken     string `json:"refresh_token,omitempty"`
	RefreshExpiresIn int64  `json:"refresh_expires_in,omitempty"`
}

func (s *Server) handleAuthToken(w http.ResponseWriter, r *http.Request) {
	if s.resolver == nil || s.resolver.One == nil || !s.resolver.One.Enabled() {
		writeErr(w, http.StatusServiceUnavailable, "AUTH_JWT_UNAVAILABLE", "Majesta One JWT signer not configured (set AUTH_JWT_SIGNING_KEY)")
		return
	}

	req, err := parseTokenRequest(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	grant := strings.TrimSpace(req.GrantType)
	if grant == "" {
		grant = "client_credentials"
	}

	authKey := "ip:" + clientKey(r)
	if grant == authz.GrantRefreshToken {
		if rt := strings.TrimSpace(req.RefreshToken); rt != "" {
			authKey = authz.RefreshRateLimitKey(rt)
		}
	} else if req.ClientID != "" {
		authKey = "client:" + req.ClientID
	}
	if !s.authTokenLimiter.allow(authKey) {
		writeErr(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many token requests")
		return
	}

	switch grant {
	case "authorization_code":
		s.handleAuthAuthorizationCode(w, r, req)
		return
	case "password":
		s.handleAuthPasswordGrant(w, r, req)
		return
	case authz.GrantRefreshToken:
		s.handleAuthRefreshGrant(w, r, req)
		return
	case "client_credentials":
		// continue below
	default:
		writeErr(w, http.StatusBadRequest, "UNSUPPORTED_GRANT", "supported grant_type: client_credentials, authorization_code, password, refresh_token")
		return
	}
	if req.ClientSecret == "" {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "client_secret is required")
		return
	}

	actor, err := s.resolveClientCredentials(r.Context(), req.ClientID, req.ClientSecret)
	if err != nil {
		if errors.Is(err, authz.ErrPrincipalNoRole) {
			writeErr(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		writeErr(w, http.StatusUnauthorized, "INVALID_CLIENT", "invalid client credentials")
		return
	}

	// Resolve azp: prefer Connected App apiName for the principal; else client_id.
	azp := strings.TrimSpace(req.ClientID)
	isBreakglass := false
	if actor.APIKeyName != "" || actor.AuthMethod == "api_key" {
		isBreakglass = true
		azp = authz.BootstrapAzp
	} else if s.pool != nil && actor.ID != "" {
		if name, err := db.IntegrationAPINameForPrincipal(r.Context(), s.pool, actor.ID); err == nil && name != "" {
			azp = name
		}
	}
	actor.Azp = azp

	policy, err := s.loadExposurePolicy(r.Context())
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "POLICY_UNAVAILABLE", "access policy unavailable")
		return
	}
	if err := authz.AllowClientAccess(policy.EffectiveClientAccessMode(), actor.Azp, "client_credentials", isBreakglass); err != nil {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}

	s.writeMintedToken(w, r, actor, "client_credentials", req.Scope, refreshDeviceID(r, req), nil, nil)
}

func (s *Server) handleAuthAuthorizationCode(w http.ResponseWriter, r *http.Request, req tokenRequest) {
	if s.pool == nil {
		writeErr(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database required")
		return
	}
	if req.ClientID == "" || req.Code == "" || req.RedirectURI == "" || req.CodeVerifier == "" {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "client_id, code, redirect_uri, and code_verifier are required")
		return
	}
	store := db.NewAuthLoginStore(s.pool)
	code, err := store.ConsumeAuthorizationCode(r.Context(), req.Code, req.ClientID, req.RedirectURI, req.CodeVerifier)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "INVALID_GRANT", "authorization code rejected")
		return
	}
	users := db.NewUserStore(s.pool)
	u, err := users.GetByID(r.Context(), code.UserID)
	if err != nil || !u.CanAuthenticate() {
		writeErr(w, http.StatusUnauthorized, "INVALID_GRANT", "user unavailable")
		return
	}
	actor, err := s.actorFromUser(r.Context(), users, u, code.IdentitySubject)
	if err != nil {
		if errors.Is(err, authz.ErrPrincipalNoRole) {
			writeErr(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		writeErr(w, http.StatusUnauthorized, "INVALID_GRANT", "user unavailable")
		return
	}
	actor.AuthMethod = "authorization_code"
	actor.Azp = code.Azp
	if actor.Azp == "" {
		actor.Azp = code.ClientID
	}

	policy, err := s.loadExposurePolicy(r.Context())
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "POLICY_UNAVAILABLE", "access policy unavailable")
		return
	}
	if err := authz.AllowClientAccess(policy.EffectiveClientAccessMode(), actor.Azp, "authorization_code", false); err != nil {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}
	if err := authz.AllowBearerAzp(policy.EffectiveClientAccessMode(), actor.Azp, "authorization_code", false); err != nil {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}

	s.writeMintedToken(w, r, actor, "authorization_code", req.Scope, refreshDeviceID(r, req), nil, nil)
}

func (s *Server) handleAuthTokenExchange(w http.ResponseWriter, r *http.Request) {
	if s.resolver == nil || s.resolver.One == nil || !s.resolver.One.Enabled() {
		writeErr(w, http.StatusServiceUnavailable, "AUTH_JWT_UNAVAILABLE", "Majesta One JWT signer not configured (set AUTH_JWT_SIGNING_KEY)")
		return
	}
	installAuth, err := s.loadInstallAuth(r)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "AUTH_POLICY_UNAVAILABLE", "authentication policy unavailable")
		return
	}
	oidc := s.effectiveOIDCVerifier(installAuth)
	slack := s.effectiveSlackVerifier(r)
	if (oidc == nil || !oidc.Enabled()) && (slack == nil || !slack.Enabled()) {
		writeErr(w, http.StatusServiceUnavailable, "OIDC_UNAVAILABLE", "OIDC verifier not configured (set OIDC_ISSUER or configure Metadata install auth SSO)")
		return
	}

	var req struct {
		GrantType        string `json:"grant_type"`
		SubjectToken     string `json:"subject_token"`
		SubjectTokenType string `json:"subject_token_type"`
	}
	if err := parseJSONOrForm(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	grant := strings.TrimSpace(req.GrantType)
	if grant == "" {
		grant = "urn:ietf:params:oauth:grant-type:token-exchange"
	}
	if grant != "urn:ietf:params:oauth:grant-type:token-exchange" {
		writeErr(w, http.StatusBadRequest, "UNSUPPORTED_GRANT", "only token-exchange is supported")
		return
	}
	if req.SubjectToken == "" {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "subject_token is required")
		return
	}
	if !s.authTokenLimiter.allow("exchange:" + clientKey(r)) {
		writeErr(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many token exchange requests")
		return
	}

	if looksLikeSlackBotToken(req.SubjectToken) {
		writeErr(w, http.StatusUnauthorized, "INVALID_TOKEN", "subject token rejected")
		return
	}

	issHint := oidcIssuerHint(req.SubjectToken)
	var verifier *authz.OIDCVerifier
	if authlogin.IsSlackIssuer(issHint) {
		if slack == nil || !slack.Enabled() {
			if s.slackCredentialsConfigured() {
				writeErr(w, http.StatusForbidden, "PROVIDER_DISABLED", "login provider is not enabled")
				return
			}
			writeErr(w, http.StatusUnauthorized, "INVALID_TOKEN", "subject token rejected")
			return
		}
		if !s.loginProviderAllowed(installAuth, identity.ProviderSlack) {
			writeErr(w, http.StatusForbidden, "PROVIDER_DISABLED", "login provider is not enabled")
			return
		}
		verifier = slack
	} else {
		if oidc == nil || !oidc.Enabled() {
			writeErr(w, http.StatusUnauthorized, "INVALID_TOKEN", "subject token rejected")
			return
		}
		verifier = oidc
	}

	autoProvision := false
	jitRole := "StandardUser"
	var allowedDomains []string
	if installAuth != nil {
		envAuto := false
		envRole := "StandardUser"
		var envDomains []string
		if s.cfg != nil {
			envAuto = s.cfg.OIDCAutoProvision || s.cfg.AuthAutoProvisionUsers
			envRole = s.cfg.AuthAutoProvisionRole
			envDomains = s.cfg.AuthLoginAllowedEmailDomains
		}
		autoProvision, jitRole, allowedDomains = installAuth.EffectiveJIT(envAuto, envRole, envDomains)
	} else if s.cfg != nil {
		autoProvision = s.cfg.OIDCAutoProvision || s.cfg.AuthAutoProvisionUsers
		if strings.TrimSpace(s.cfg.AuthAutoProvisionRole) != "" {
			jitRole = strings.TrimSpace(s.cfg.AuthAutoProvisionRole)
		}
		allowedDomains = append([]string(nil), s.cfg.AuthLoginAllowedEmailDomains...)
	}
	requestResolver := *s.resolver
	requestResolver.OIDC = verifier
	requestResolver.AutoProvision = autoProvision
	tt := strings.TrimSpace(req.SubjectTokenType)
	if tt == "" {
		tt = "urn:ietf:params:oauth:token-type:id_token"
	}
	if tt != "urn:ietf:params:oauth:token-type:id_token" {
		writeErr(w, http.StatusBadRequest, "UNSUPPORTED_TOKEN_TYPE", "subject_token_type must be id_token")
		return
	}

	provisioning := db.ProvisioningConfig{}
	if installAuth != nil {
		provisioning = installAuth.Provisioning
	}
	actor, created, err := s.exchangeOIDCSubject(r.Context(), req.SubjectToken, &requestResolver, jitRole, allowedDomains, provisioning)
	if err != nil {
		if errors.Is(err, authz.ErrPrincipalNoRole) {
			writeErr(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		if errors.Is(err, errIdentityEmailUnverified) {
			writeErr(w, http.StatusForbidden, "LOGIN_EMAIL_UNVERIFIED", "email is not verified by the identity provider")
			return
		}
		if errors.Is(err, errIdentityEmailDomainDenied) {
			writeErr(w, http.StatusForbidden, "EMAIL_DOMAIN_DENIED", "email domain is not allowed")
			return
		}
		if errors.Is(err, db.ErrValidation) {
			writeErr(w, http.StatusForbidden, "LOGIN_EMAIL_REQUIRED", "email is required from the identity provider")
			return
		}
		writeErr(w, http.StatusUnauthorized, "INVALID_TOKEN", "subject token rejected")
		return
	}
	if created {
		_ = db.EnqueueOutbox(r.Context(), s.pool, db.EventPrincipalCreated, actor.ID, map[string]any{
			"userId": actor.ID, "email": actor.Email, "principalType": actor.PrincipalType, "source": "jit",
		})
		s.writeAudit(r, "auth.provision", "", &actor.ID, map[string]any{
			"provider": authlogin.InferProviderFromIssuer(issHint),
			"issuer":   issHint,
			"source":   "jit",
		})
	}
	s.writeAudit(r, "auth.login", "", &actor.ID, map[string]any{
		"provider": authlogin.InferProviderFromIssuer(issHint),
		"issuer":   issHint,
	})

	// Human exchange: default generic install azp; remap when OIDC aud maps to a Connected App.
	actor.Azp = authz.InstallAzp
	if s.pool != nil {
		if claimsAud := oidcAudienceHint(req.SubjectToken); claimsAud != "" {
			if name, err := db.IntegrationAPINameForCognitoClient(r.Context(), s.pool, claimsAud); err == nil && name != "" {
				actor.Azp = name
			}
		}
	}

	policy, err := s.loadExposurePolicy(r.Context())
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "POLICY_UNAVAILABLE", "access policy unavailable")
		return
	}
	if err := authz.AllowClientAccess(policy.EffectiveClientAccessMode(), actor.Azp, "token-exchange", false); err != nil {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}
	if err := authz.AllowBearerAzp(policy.EffectiveClientAccessMode(), actor.Azp, "token_exchange", false); err != nil {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}

	s.writeMintedToken(w, r, actor, authz.GrantTokenExchange, "", strings.TrimSpace(r.Header.Get("X-One-Device-Id")), nil, nil)
}

func (s *Server) exchangeOIDCSubject(ctx context.Context, subjectToken string, resolver *authz.Resolver, jitRole string, allowedDomains []string, provisioning db.ProvisioningConfig) (*authz.Actor, bool, error) {
	if resolver == nil || resolver.OIDC == nil {
		return nil, false, fmt.Errorf("OIDC verifier unavailable")
	}
	claims, err := resolver.OIDC.VerifyIDToken(ctx, subjectToken)
	if err != nil {
		return nil, false, err
	}
	issuer := claims.Issuer
	if issuer == "" {
		issuer = resolver.OIDC.Issuer
	}
	provider := authlogin.InferProviderFromIssuer(issuer)
	if s.pool == nil {
		actor, err := resolver.ResolveOIDC(subjectToken)
		return actor, false, err
	}

	users := db.NewUserStore(s.pool)
	actorForUser := func(userID string) (*authz.Actor, error) {
		u, err := users.GetByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		if !u.CanAuthenticate() {
			return nil, authz.ErrUserNotFound
		}
		// Token exchange is an interactive human grant. Keep its greenfield
		// bootstrap behavior aligned with browser/local sign-in: the first active
		// human becomes System Admin, while later humans retain their JIT role.
		if err := users.EnsureInitialHumanSystemAdmin(ctx, u.ID); err != nil {
			return nil, err
		}
		u, err = users.GetByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		actor, err := s.actorFromUser(ctx, users, u, claims.Subject)
		if actor != nil {
			actor.AuthMethod = "token_exchange"
		}
		return actor, err
	}

	if s.pool != nil {
		links := db.NewIdentityLinkStore(s.pool)
		if link, err := links.GetByIssuerSubject(ctx, issuer, claims.Subject); err == nil {
			actor, err := actorForUser(link.UserID)
			return actor, false, err
		} else if !errors.Is(err, db.ErrNotFound) {
			return nil, false, err
		}
		// Backward-compatible cognito provider lookup when issuer empty on link rows.
		if link, err := links.GetBySubject(ctx, "cognito", issuer, claims.Subject); err == nil {
			actor, err := actorForUser(link.UserID)
			return actor, false, err
		} else if !errors.Is(err, db.ErrNotFound) {
			return nil, false, err
		}

		// Legacy users.oidc_sub rows predate issuer-scoped identity_links. Migrate
		// only when the token email matches, so a subject collision at a different
		// issuer cannot silently bind to an existing principal. Social providers
		// must also attest that email before it can be used for this one-time link.
		if legacy, err := users.GetByOIDCSub(ctx, claims.Subject); err == nil {
			emailMatches := strings.TrimSpace(claims.Email) != "" && strings.EqualFold(strings.TrimSpace(legacy.Email), strings.TrimSpace(claims.Email))
			emailAllowed := authlogin.EmailDomainAllowed(claims.Email, allowedDomains, false)
			emailTrusted := !requiresVerifiedJITEmail(provider) || claims.EmailIsVerified()
			if emailMatches && emailAllowed && emailTrusted {
				if _, err := links.Upsert(ctx, legacy.ID, provider, issuer, claims.Subject); err != nil {
					return nil, false, err
				}
				actor, err := actorForUser(legacy.ID)
				return actor, false, err
			}
		} else if !errors.Is(err, db.ErrNotFound) {
			return nil, false, err
		}
	}
	if !resolver.AutoProvision {
		return nil, false, authz.ErrUserNotFound
	}
	if strings.TrimSpace(claims.Email) == "" {
		return nil, false, fmt.Errorf("%w: email is required", db.ErrValidation)
	}
	if !authlogin.EmailDomainAllowed(claims.Email, allowedDomains, false) {
		return nil, false, errIdentityEmailDomainDenied
	}
	if requiresVerifiedJITEmail(provider) && !claims.EmailIsVerified() {
		return nil, false, errIdentityEmailUnverified
	}
	displayName := strings.TrimSpace(claims.Name)
	if displayName == "" {
		displayName = strings.TrimSpace(claims.PreferredUsername)
	}
	u, err := s.applyJITCreateProvisioning(ctx, users, claims.Email, displayName, jitRole,
		oidcClaimsMap(subjectToken, claims.Email, claims.Name, claims.PreferredUsername, claims.Subject), provisioning)
	if err != nil {
		return nil, false, err
	}
	if _, err := db.NewIdentityLinkStore(s.pool).Upsert(ctx, u.ID, provider, issuer, claims.Subject); err != nil {
		return nil, false, err
	}
	actor, err := actorForUser(u.ID)
	return actor, true, err
}

func (s *Server) actorFromUser(ctx context.Context, store *db.UserStore, u *db.User, oidcSub string) (*authz.Actor, error) {
	if !u.CanAuthenticate() {
		return nil, authz.ErrUserNotFound
	}
	psIDs, err := store.ListPermissionSetIDs(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	roleScopes, roleAdmin, roleNames, err := store.ListRoleGrants(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	if len(roleNames) == 0 {
		return nil, authz.ErrPrincipalNoRole
	}
	return &authz.Actor{
		ID:               u.ID,
		Email:            u.Email,
		DisplayName:      u.DisplayName,
		Scopes:           roleScopes,
		IsAdmin:          u.IsAdmin || roleAdmin,
		AuthMethod:       "token_exchange",
		PrincipalType:    u.PrincipalType,
		Roles:            roleNames,
		PermissionSetIDs: psIDs,
		OIDCSub:          oidcSub,
	}, nil
}

func parseJSONOrForm(r *http.Request, dst any) error {
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return err
		}
		m := map[string]any{
			"grant_type":         r.FormValue("grant_type"),
			"subject_token":      r.FormValue("subject_token"),
			"subject_token_type": r.FormValue("subject_token_type"),
		}
		b, err := json.Marshal(m)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, dst)
	}
	defer func() { _ = r.Body.Close() }()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func (s *Server) handleAuthOIDCDiscovery(w http.ResponseWriter, _ *http.Request) {
	issuer := ""
	if s.cfg != nil {
		issuer = s.cfg.AuthJWTIssuer
	}
	if issuer == "" && s.resolver != nil && s.resolver.One != nil {
		issuer = s.resolver.One.Issuer
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                  issuer,
		"authorization_endpoint":  issuer + "/authorize",
		"token_endpoint":          issuer + "/token",
		"token_exchange_endpoint": issuer + "/token/exchange",
		"grant_types_supported": []string{
			"client_credentials",
			"authorization_code",
			"password",
			"refresh_token",
			"urn:ietf:params:oauth:grant-type:token-exchange",
		},
		"revocation_endpoint":                   issuer + "/revoke",
		"token_endpoint_auth_methods_supported": []string{"client_secret_post", "none"},
		"code_challenge_methods_supported":      []string{"S256"},
		"response_types_supported":              []string{"code", "token"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"HS256"},
		"one_note":                              "HS256 Majesta One access JWT; social login via /authorize; external OIDC ID tokens via /token/exchange",
	})
}

func (s *Server) resolveClientCredentials(ctx context.Context, clientID, clientSecret string) (*authz.Actor, error) {
	// 1) Transitional: client_secret (or client_id) matches an env API key.
	secret := clientSecret
	if entry := s.resolver.MatchAPIKeyEntry(secret); entry != nil {
		return s.actorFromAPIKeyEntry(ctx, entry)
	}
	if clientID != "" {
		if entry := s.resolver.MatchAPIKeyEntry(clientID); entry != nil && clientSecret == clientID {
			return s.actorFromAPIKeyEntry(ctx, entry)
		}
	}

	// 2) DB principal_credentials: client_id = user UUID, client_secret = plaintext secret.
	if clientID == "" || s.resolver.Users == nil || s.resolver.Credentials == nil {
		return nil, authz.ErrCredentialNotFound
	}
	u, err := s.resolver.Users.GetByID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if !u.CanAuthenticate() {
		return nil, authz.ErrUserNotFound
	}
	creds, err := s.resolver.Credentials.ListActiveByUserID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	matched := false
	for _, c := range creds {
		if c.CredentialKind != "" && c.CredentialKind != "client_secret" && c.CredentialKind != "bootstrap_api_key" {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(c.SecretHash), []byte(clientSecret)) == nil {
			matched = true
			break
		}
	}
	if !matched {
		return nil, authz.ErrCredentialNotFound
	}

	scopes := authz.AllScopes
	isAdmin := u.IsAdmin
	roleScopes, roleAdmin, roleNames, err := s.resolver.Users.ListRoleGrants(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	if len(roleNames) == 0 {
		return nil, authz.ErrPrincipalNoRole
	}
	if len(roleScopes) > 0 {
		scopes = roleScopes
	}
	isAdmin = isAdmin || roleAdmin
	psIDs, err := s.resolver.Users.ListPermissionSetIDs(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	return &authz.Actor{
		ID:               u.ID,
		Email:            u.Email,
		DisplayName:      u.DisplayName,
		PrincipalType:    u.PrincipalType,
		Scopes:           scopes,
		Roles:            roleNames,
		PermissionSetIDs: psIDs,
		IsAdmin:          isAdmin,
		AuthMethod:       authz.AuthMethodOneJWT,
	}, nil
}

func (s *Server) actorFromAPIKeyEntry(ctx context.Context, entry *authz.APIKeyEntry) (*authz.Actor, error) {
	actor := &authz.Actor{
		ID:            s.cfg.DefaultOwnerID,
		Email:         "admin@one.local",
		DisplayName:   "Admin",
		APIKeyName:    authz.APIKeyIdentifier(entry.Key),
		PrincipalType: "service",
		Scopes:        append([]authz.Scope(nil), entry.Scopes...),
		IsAdmin:       entry.IsAdmin,
		AuthMethod:    authz.AuthMethodOneJWT,
	}
	if s.resolver.Users != nil {
		u, err := s.resolver.Users.EnsureAPIKeyServicePrincipal(ctx, entry.Key, entry.IsAdmin, entry.Scopes)
		if err != nil {
			return nil, err
		}
		actor.ID = u.ID
		actor.Email = u.Email
		actor.DisplayName = u.DisplayName
		actor.IsAdmin = entry.IsAdmin || u.IsAdmin
		if u.PrincipalType != "" {
			actor.PrincipalType = u.PrincipalType
		}
		ids, err := s.resolver.Users.ListPermissionSetIDs(ctx, u.ID)
		if err != nil {
			return nil, err
		}
		actor.PermissionSetIDs = ids
		roleScopes, roleAdmin, roleNames, err := s.resolver.Users.ListRoleGrants(ctx, u.ID)
		if err != nil {
			return nil, err
		}
		if len(roleNames) == 0 {
			return nil, authz.ErrPrincipalNoRole
		}
		if len(roleScopes) > 0 {
			actor.Scopes = roleScopes
		}
		actor.IsAdmin = actor.IsAdmin || roleAdmin
		actor.Roles = roleNames
	}
	return actor, nil
}

func parseTokenRequest(r *http.Request) (tokenRequest, error) {
	var req tokenRequest
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		defer func() { _ = r.Body.Close() }()
		dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		if err := dec.Decode(&req); err != nil {
			return req, err
		}
		return req, nil
	}
	if err := r.ParseForm(); err != nil {
		return req, err
	}
	req.GrantType = r.FormValue("grant_type")
	req.ClientID = r.FormValue("client_id")
	req.ClientSecret = r.FormValue("client_secret")
	req.Code = r.FormValue("code")
	req.RedirectURI = r.FormValue("redirect_uri")
	req.CodeVerifier = r.FormValue("code_verifier")
	req.Scope = r.FormValue("scope")
	req.Username = r.FormValue("username")
	if req.Username == "" {
		req.Username = r.FormValue("email")
	}
	req.Password = r.FormValue("password")
	req.RefreshToken = r.FormValue("refresh_token")
	req.DeviceID = r.FormValue("device_id")
	if req.DeviceID == "" {
		req.DeviceID = r.Header.Get("X-One-Device-Id")
	}
	// Also accept Authorization: Basic for client_id:client_secret (optional).
	if req.ClientSecret == "" {
		if u, p, ok := r.BasicAuth(); ok {
			if req.ClientID == "" {
				req.ClientID = u
			}
			req.ClientSecret = p
		}
	}
	return req, nil
}

// oidcAudienceHint extracts aud from an unverified JWT payload (hint only; VerifyIDToken already ran).
func oidcAudienceHint(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	var claims struct {
		Aud any `json:"aud"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	switch v := claims.Aud.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func oidcIssuerHint(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	var claims struct {
		Iss string `json:"iss"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return strings.TrimSpace(claims.Iss)
}

func looksLikeSlackBotToken(raw string) bool {
	s := strings.TrimSpace(raw)
	return strings.HasPrefix(s, "xoxb-") ||
		strings.HasPrefix(s, "xoxp-") ||
		strings.HasPrefix(s, "xoxa-") ||
		strings.HasPrefix(s, "xoxs-")
}
