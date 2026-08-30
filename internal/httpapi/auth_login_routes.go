package httpapi

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/MajestaNet/ide/internal/authlogin"
	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/identity"
	"github.com/MajestaNet/ide/internal/integration"
)

var (
	errIdentityEmailUnverified   = errors.New("identity provider email is not verified")
	errIdentityEmailDomainDenied = errors.New("identity provider email domain is not allowed")
)

func (s *Server) registerSocialAuthRoutes() {
	s.mux.HandleFunc("GET /auth/v1/login", s.handleAuthLoginPage)
	s.mux.HandleFunc("GET /auth/v1/authorize", s.handleAuthAuthorize)
	s.mux.HandleFunc("GET /auth/v1/callback/{provider}", s.handleAuthCallback)
	s.mux.HandleFunc("GET /auth/v1/login/providers", s.handleAuthLoginProviders)
}

func (s *Server) ensureCustomerOIDCOnBroker(st *db.InstallAuthSettings) {
	if st == nil || s.loginBroker == nil {
		return
	}
	issuer := strings.TrimSpace(st.OIDCIssuer)
	clientID := strings.TrimSpace(st.OIDCClientID)
	if clientID == "" {
		clientID = strings.TrimSpace(st.OIDCAudience)
	}
	secret, err := revealInstallAuthClientSecret(st.OIDCClientSecretEnc, s.encKey())
	if err != nil {
		slog.Warn("customer OIDC client secret unavailable", "err", err)
		return
	}
	if issuer == "" || clientID == "" || secret == "" {
		return
	}
	aud := strings.TrimSpace(st.OIDCAudience)
	if aud == "" {
		aud = clientID
	}
	s.loginBroker.RegisterOrReplace(authlogin.NewOIDCProvider(authlogin.OIDCConfig{
		Issuer:       issuer,
		Audience:     aud,
		ClientID:     clientID,
		ClientSecret: secret,
		JWKSURI:      strings.TrimSpace(st.OIDCJWKSURI),
		DisplayName:  strings.TrimSpace(st.OIDCDisplayName),
	}))
}

func (s *Server) loginProviderAllowed(st *db.InstallAuthSettings, providerName string) bool {
	providerName = strings.ToLower(strings.TrimSpace(providerName))
	if providerName == identity.ProviderOIDC {
		if st == nil {
			return false
		}
		hasIssuer := strings.TrimSpace(st.OIDCIssuer) != ""
		hasClient := strings.TrimSpace(st.OIDCClientID) != "" || strings.TrimSpace(st.OIDCAudience) != ""
		hasSecret := strings.TrimSpace(st.OIDCClientSecretEnc) != ""
		return hasIssuer && hasClient && hasSecret
	}
	envProviders := []string{}
	if s.cfg != nil {
		envProviders = s.cfg.AuthLoginProviders
	}
	if st != nil {
		for _, p := range st.EffectiveSocialProviders(envProviders) {
			if p == providerName {
				return true
			}
		}
		return false
	}
	if s.cfg != nil && len(s.cfg.AuthLoginProviders) > 0 {
		return s.cfg.LoginProviderEnabled(providerName)
	}
	return false
}

func (s *Server) handleAuthLoginProviders(w http.ResponseWriter, r *http.Request) {
	st, err := s.loadInstallAuth(r)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "AUTH_POLICY_UNAVAILABLE", "authentication policy unavailable")
		return
	}
	s.ensureCustomerOIDCOnBroker(st)
	names := []string{}
	if s.loginBroker != nil {
		for _, n := range s.loginBroker.EnabledNames() {
			if s.loginProviderAllowed(st, n) {
				names = append(names, n)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"providers": names,
		"authorize": "/auth/v1/authorize",
	})
}

func (s *Server) handleAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeErr(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database required for social login")
		return
	}
	if s.resolver == nil || s.resolver.One == nil || !s.resolver.One.Enabled() {
		writeErr(w, http.StatusServiceUnavailable, "AUTH_JWT_UNAVAILABLE", "Majesta One JWT signer not configured")
		return
	}
	if !s.authTokenLimiter.allow("authorize:" + clientKey(r)) {
		writeErr(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many authorize requests")
		return
	}

	providerName := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("provider")))
	clientID := strings.TrimSpace(r.URL.Query().Get("client_id"))
	redirectURI := strings.TrimSpace(r.URL.Query().Get("redirect_uri"))
	codeChallenge := strings.TrimSpace(r.URL.Query().Get("code_challenge"))
	method := strings.TrimSpace(r.URL.Query().Get("code_challenge_method"))
	clientState := r.URL.Query().Get("state")

	if providerName == "" || clientID == "" || redirectURI == "" || codeChallenge == "" {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "provider, client_id, redirect_uri, and code_challenge are required")
		return
	}
	if method == "" {
		method = "S256"
	}
	if method != "S256" {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "code_challenge_method must be S256")
		return
	}
	installAuth, err := s.loadInstallAuth(r)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "AUTH_POLICY_UNAVAILABLE", "authentication policy unavailable")
		return
	}
	s.ensureCustomerOIDCOnBroker(installAuth)
	if !s.loginProviderAllowed(installAuth, providerName) {
		writeErr(w, http.StatusBadRequest, "PROVIDER_DISABLED", "login provider is not enabled")
		return
	}
	if s.loginBroker == nil {
		writeErr(w, http.StatusServiceUnavailable, "PROVIDER_UNAVAILABLE", "social login broker not configured")
		return
	}
	prov, ok := s.loginBroker.Get(providerName)
	if !ok {
		writeErr(w, http.StatusBadRequest, "PROVIDER_UNAVAILABLE", "login provider is not configured")
		return
	}

	integ, err := db.NewIntegrationStore(s.pool).GetByAPIName(r.Context(), clientID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_CLIENT", "unknown client_id")
		return
	}
	if !integ.IsActive {
		writeErr(w, http.StatusForbidden, "CLIENT_INACTIVE", "client is inactive")
		return
	}
	if !callbackAllowed(integ.CallbackURLs, redirectURI) {
		writeErr(w, http.StatusBadRequest, "INVALID_REDIRECT_URI", "redirect_uri is not registered for this client")
		return
	}
	hasAuthCode := false
	for _, f := range integ.OAuthFlows {
		if f == identity.FlowAuthorizationCode {
			hasAuthCode = true
			break
		}
	}
	if !hasAuthCode {
		writeErr(w, http.StatusBadRequest, "UNSUPPORTED_FLOW", "client does not allow authorization_code")
		return
	}

	serverState, err := authlogin.RandomURLToken(32)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "STATE_ERROR", "failed to create state")
		return
	}
	nonce, err := authlogin.RandomURLToken(24)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "NONCE_ERROR", "failed to create nonce")
		return
	}
	idpVerifier, err := authlogin.RandomURLToken(32)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "PKCE_ERROR", "failed to create PKCE verifier")
		return
	}
	idpChallenge := authlogin.PKCEChallengeS256(idpVerifier)

	publicURL := ""
	if s.cfg != nil {
		publicURL = s.cfg.PlatformPublicURL
	}
	callback := authlogin.CallbackURL(publicURL, providerName)

	store := db.NewAuthLoginStore(s.pool)
	if err := store.PutLoginState(r.Context(), serverState, db.AuthLoginState{
		Provider:            providerName,
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		ClientState:         clientState,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: method,
		Nonce:               nonce,
		IDPCodeVerifier:     idpVerifier,
	}, 0); err != nil {
		writeErr(w, http.StatusInternalServerError, "STATE_ERROR", "failed to persist login state")
		return
	}

	authURL, err := prov.AuthCodeURL(serverState, nonce, idpChallenge, callback)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "IDP_ERROR", "failed to start identity provider login")
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeErr(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database required for social login")
		return
	}
	if !s.authTokenLimiter.allow("callback:" + clientKey(r)) {
		writeErr(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many callback requests")
		return
	}

	providerName := strings.ToLower(strings.TrimSpace(r.PathValue("provider")))
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		writeErr(w, http.StatusBadRequest, "IDP_ERROR", errParam+": "+r.URL.Query().Get("error_description"))
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if code == "" || state == "" {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "code and state are required")
		return
	}

	store := db.NewAuthLoginStore(s.pool)
	st, err := store.TakeLoginState(r.Context(), state)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_STATE", "login state missing or expired")
		return
	}
	if st.Provider != providerName {
		writeErr(w, http.StatusBadRequest, "INVALID_STATE", "provider mismatch")
		return
	}

	installAuth, err := s.loadInstallAuth(r)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "AUTH_POLICY_UNAVAILABLE", "authentication policy unavailable")
		return
	}
	s.ensureCustomerOIDCOnBroker(installAuth)
	if s.loginBroker == nil {
		writeErr(w, http.StatusServiceUnavailable, "PROVIDER_UNAVAILABLE", "social login broker not configured")
		return
	}
	prov, ok := s.loginBroker.Get(providerName)
	if !ok {
		writeErr(w, http.StatusBadRequest, "PROVIDER_UNAVAILABLE", "login provider is not configured")
		return
	}

	publicURL := ""
	if s.cfg != nil {
		publicURL = s.cfg.PlatformPublicURL
	}
	callback := authlogin.CallbackURL(publicURL, providerName)

	claims, err := prov.Exchange(r.Context(), code, st.IDPCodeVerifier, callback, st.Nonce)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "IDP_TOKEN_REJECTED", "identity provider token rejected")
		return
	}

	autoProvision := false
	role := "StandardUser"
	var domains []string
	if installAuth != nil {
		envAuto := false
		envRole := "StandardUser"
		var envDomains []string
		if s.cfg != nil {
			envAuto = s.cfg.AuthAutoProvisionUsers
			envRole = s.cfg.AuthAutoProvisionRole
			envDomains = s.cfg.AuthLoginAllowedEmailDomains
		}
		autoProvision, role, domains = installAuth.EffectiveJIT(envAuto, envRole, envDomains)
	} else if s.cfg != nil {
		autoProvision = s.cfg.AuthAutoProvisionUsers
		if s.cfg.AuthAutoProvisionRole != "" {
			role = s.cfg.AuthAutoProvisionRole
		}
		domains = s.cfg.AuthLoginAllowedEmailDomains
	}
	// Email is required profile data. Empty claims.Email is only OK for returning users
	// who already have email stored (Apple often omits email after first consent).
	if !authlogin.EmailDomainAllowed(claims.Email, domains, claims.Email == "") {
		writeErr(w, http.StatusForbidden, "EMAIL_DOMAIN_DENIED", "email domain is not allowed")
		return
	}

	provisioning := db.ProvisioningConfig{}
	if installAuth != nil {
		provisioning = installAuth.Provisioning
	}
	user, err := s.resolveSocialUser(r, claims, autoProvision, role, provisioning)
	if err != nil {
		if errors.Is(err, errIdentityEmailUnverified) {
			writeErr(w, http.StatusForbidden, "LOGIN_EMAIL_UNVERIFIED", "email is not verified by the identity provider")
			return
		}
		if errors.Is(err, db.ErrNotFound) || errors.Is(err, authz.ErrUserNotFound) {
			writeErr(w, http.StatusForbidden, "LOGIN_NOT_PROVISIONED", "user is not provisioned for this install")
			return
		}
		if errors.Is(err, db.ErrValidation) {
			writeErr(w, http.StatusForbidden, "LOGIN_EMAIL_REQUIRED", "email is required from the identity provider")
			return
		}
		if errors.Is(err, authz.ErrPrincipalNoRole) {
			writeErr(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, "PROVISION_ERROR", "failed to resolve user")
		return
	}

	s.writeAudit(r, "auth.login", "", &user.ID, map[string]any{
		"provider": claims.Provider,
		"issuer":   claims.Issuer,
		"subject":  claims.Subject,
	})

	oneCode, err := authlogin.RandomURLToken(32)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "CODE_ERROR", "failed to mint authorization code")
		return
	}
	if err := store.PutAuthorizationCode(r.Context(), oneCode, db.AuthAuthorizationCode{
		UserID:              user.ID,
		ClientID:            st.ClientID,
		RedirectURI:         st.RedirectURI,
		CodeChallenge:       st.CodeChallenge,
		CodeChallengeMethod: st.CodeChallengeMethod,
		Azp:                 st.ClientID,
		IdentityProvider:    claims.Provider,
		IdentitySubject:     claims.Subject,
	}, 0); err != nil {
		writeErr(w, http.StatusInternalServerError, "CODE_ERROR", "failed to persist authorization code")
		return
	}

	redir, err := url.Parse(st.RedirectURI)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "REDIRECT_ERROR", "invalid stored redirect_uri")
		return
	}
	q := redir.Query()
	q.Set("code", oneCode)
	if st.ClientState != "" {
		q.Set("state", st.ClientState)
	}
	redir.RawQuery = q.Encode()
	http.Redirect(w, r, redir.String(), http.StatusFound)
}

func (s *Server) resolveSocialUser(r *http.Request, claims *authlogin.SubjectClaims, autoProvision bool, role string, prov db.ProvisioningConfig) (*db.User, error) {
	links := db.NewIdentityLinkStore(s.pool)
	users := db.NewUserStore(s.pool)

	if link, err := links.GetBySubject(r.Context(), claims.Provider, claims.Issuer, claims.Subject); err == nil {
		u, err := users.GetByID(r.Context(), link.UserID)
		if err != nil {
			return nil, err
		}
		if !u.CanAuthenticate() {
			return nil, authz.ErrUserNotFound
		}
		if u.Email == "" {
			if claims.Email == "" {
				return nil, fmt.Errorf("%w: email is required", db.ErrValidation)
			}
			if err := users.UpdateEmailIfEmpty(r.Context(), u.ID, claims.Email); err != nil {
				return nil, err
			}
			u.Email = claims.Email
		}
		// Promote greenfield first human (e.g. earlier StandardUser auto-provision) so IDE sees all tiles.
		if err := users.EnsureInitialHumanSystemAdmin(r.Context(), u.ID); err != nil {
			return nil, err
		}
		return users.GetByID(r.Context(), u.ID)
	} else if !errors.Is(err, db.ErrNotFound) {
		return nil, err
	}

	if !autoProvision {
		return nil, db.ErrNotFound
	}
	if claims.Email == "" {
		return nil, fmt.Errorf("%w: email is required", db.ErrValidation)
	}
	if requiresVerifiedJITEmail(claims.Provider) && !claims.EmailVerified {
		return nil, errIdentityEmailUnverified
	}

	u, err := s.applyJITCreateProvisioning(r.Context(), users, claims.Email, claims.Name, role, claims.ClaimMap(), prov)
	if err != nil {
		return nil, err
	}
	if _, err := links.Upsert(r.Context(), u.ID, claims.Provider, claims.Issuer, claims.Subject); err != nil {
		return nil, err
	}
	// First human on the install gets SystemAdmin role + managed System Admin permission set.
	if err := users.EnsureInitialHumanSystemAdmin(r.Context(), u.ID); err != nil {
		return nil, err
	}
	_ = db.EnqueueOutbox(r.Context(), s.pool, db.EventPrincipalCreated, u.ID, map[string]any{
		"userId": u.ID, "email": u.Email, "principalType": u.PrincipalType, "source": "jit",
	})
	s.writeAudit(r, "auth.provision", "", &u.ID, map[string]any{
		"provider": claims.Provider,
		"issuer":   claims.Issuer,
		"subject":  claims.Subject,
		"source":   "jit",
	})
	return users.GetByID(r.Context(), u.ID)
}

func requiresVerifiedJITEmail(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case identity.ProviderGoogle, identity.ProviderApple, identity.ProviderSlack:
		return true
	default:
		return false
	}
}

func callbackAllowed(allowed []string, redirectURI string) bool {
	for _, a := range allowed {
		if a == redirectURI {
			return true
		}
	}
	if redirectURI == integration.DefaultControlIDERedirectURI {
		for _, a := range allowed {
			if strings.HasPrefix(a, "one-control://") {
				return true
			}
		}
	}
	return false
}
