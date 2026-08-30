package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
)

func (s *Server) registerInstallClaimRoutes() {
	s.mux.HandleFunc("GET /auth/v1/install/status", s.handleInstallStatus)
	s.mux.HandleFunc("POST /auth/v1/install/claim", s.handleInstallClaim)
}

func (s *Server) handleInstallStatus(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeJSON(w, http.StatusOK, db.InstallAuthStatus{
			Claimed:              false,
			PasswordLoginEnabled: true,
			SocialProviders:      []string{},
		})
		return
	}
	st, err := db.NewInstallAuthStore(s.pool).Get(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	status := st.PublicStatus()
	// Merge env social for lab when DB empty.
	if len(status.SocialProviders) == 0 && s.cfg != nil {
		status.SocialProviders = st.EffectiveSocialProviders(s.cfg.AuthLoginProviders)
	}
	writeJSON(w, http.StatusOK, status)
}

type installClaimRequest struct {
	Token       string `json:"token"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

func (s *Server) handleInstallClaim(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeErr(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database required")
		return
	}
	if s.resolver == nil || s.resolver.One == nil || !s.resolver.One.Enabled() {
		writeErr(w, http.StatusServiceUnavailable, "AUTH_JWT_UNAVAILABLE", "Majesta One JWT signer not configured")
		return
	}
	if !s.authTokenLimiter.allow("claim:" + clientKey(r)) {
		writeErr(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many claim requests")
		return
	}

	var req installClaimRequest
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		defer func() { _ = r.Body.Close() }()
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
			return
		}
	} else {
		_ = r.ParseForm()
		req.Token = r.FormValue("token")
		req.Email = r.FormValue("email")
		req.Password = r.FormValue("password")
		req.DisplayName = r.FormValue("displayName")
	}
	req.Token = strings.TrimSpace(req.Token)
	req.Email = strings.TrimSpace(req.Email)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.Token == "" || req.Email == "" || req.Password == "" {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "token, email, and password are required")
		return
	}
	if len(req.Password) < 10 {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "password must be at least 10 characters")
		return
	}

	authStore := db.NewInstallAuthStore(s.pool)
	st, err := authStore.Get(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if st.ClaimedAt != nil {
		writeErr(w, http.StatusConflict, "ALREADY_CLAIMED", "install has already been claimed")
		return
	}
	if st.ClaimTokenHash == "" {
		writeErr(w, http.StatusServiceUnavailable, "CLAIM_TOKEN_UNSET", "INSTALL_CLAIM_TOKEN is not configured on this install")
		return
	}
	ok, err := authStore.VerifyClaimToken(r.Context(), req.Token)
	if err != nil {
		if errors.Is(err, db.ErrConflict) {
			writeErr(w, http.StatusConflict, "ALREADY_CLAIMED", "install has already been claimed")
			return
		}
		writeAPIError(w, err)
		return
	}
	if !ok {
		writeErr(w, http.StatusUnauthorized, "INVALID_CLAIM_TOKEN", "claim token rejected")
		return
	}

	users := db.NewUserStore(s.pool)
	u, err := users.CreateSocialUser(r.Context(), req.Email, req.DisplayName, db.SystemAdminRoleAPIName)
	if err != nil {
		if errors.Is(err, db.ErrConflict) {
			writeErr(w, http.StatusConflict, "EMAIL_IN_USE", "email already exists on this install")
			return
		}
		if errors.Is(err, db.ErrValidation) {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
			return
		}
		writeAPIError(w, err)
		return
	}
	if err := users.GrantSystemAdminPack(r.Context(), u.ID); err != nil {
		writeAPIError(w, err)
		return
	}
	creds := db.NewCredentialStore(s.pool)
	if _, err := creds.CreatePassword(r.Context(), u.ID, req.Password); err != nil {
		if errors.Is(err, db.ErrValidation) {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
			return
		}
		writeAPIError(w, err)
		return
	}
	if err := authStore.MarkClaimed(r.Context()); err != nil {
		if errors.Is(err, db.ErrConflict) {
			writeErr(w, http.StatusConflict, "ALREADY_CLAIMED", "install has already been claimed")
			return
		}
		writeAPIError(w, err)
		return
	}

	actor, err := s.actorFromUser(r.Context(), users, u, "")
	if err != nil {
		writeAPIError(w, err)
		return
	}
	actor.AuthMethod = "password"
	actor.Azp = authz.InstallAzp
	s.writeAudit(r, "install.claim", u.ID, nil, map[string]any{"claimed": true, "email": u.Email})
	_ = db.EnqueueOutbox(r.Context(), s.pool, db.EventInstallClaimed, u.ID, map[string]any{
		"userId": u.ID, "email": u.Email, "claimed": true,
	})
	s.writeMintedToken(w, r, actor, authz.GrantPassword, "", strings.TrimSpace(r.Header.Get("X-One-Device-Id")), map[string]any{
		"user": map[string]any{
			"id":          u.ID,
			"email":       u.Email,
			"displayName": u.DisplayName,
		},
		"claimed": true,
	}, nil)
}

func (s *Server) handleAuthPasswordGrant(w http.ResponseWriter, r *http.Request, req tokenRequest) {
	if s.pool == nil {
		writeErr(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database required")
		return
	}
	email := strings.TrimSpace(req.Username)
	if email == "" {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "username (email) and password are required")
		return
	}
	if req.Password == "" {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "username (email) and password are required")
		return
	}

	authStore := db.NewInstallAuthStore(s.pool)
	st, err := authStore.Get(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if !st.PasswordLoginEnabled {
		writeErr(w, http.StatusForbidden, "PASSWORD_LOGIN_DISABLED", "password login is disabled on this install")
		return
	}

	users := db.NewUserStore(s.pool)
	u, err := users.GetByEmail(r.Context(), email)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "INVALID_GRANT", "invalid email or password")
		return
	}
	if !u.CanAuthenticate() || u.PrincipalType != "user" {
		writeErr(w, http.StatusUnauthorized, "INVALID_GRANT", "invalid email or password")
		return
	}
	ok, err := db.NewCredentialStore(s.pool).VerifyPassword(r.Context(), u.ID, req.Password)
	if err != nil || !ok {
		writeErr(w, http.StatusUnauthorized, "INVALID_GRANT", "invalid email or password")
		return
	}

	actor, err := s.actorFromUser(r.Context(), users, u, "")
	if err != nil {
		if errors.Is(err, authz.ErrPrincipalNoRole) {
			writeErr(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		writeErr(w, http.StatusUnauthorized, "INVALID_GRANT", "invalid email or password")
		return
	}
	actor.AuthMethod = "password"
	azp := strings.TrimSpace(req.ClientID)
	if azp == "" {
		azp = authz.InstallAzp
	}
	actor.Azp = azp

	policy, err := s.loadExposurePolicy(r.Context())
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "POLICY_UNAVAILABLE", "access policy unavailable")
		return
	}
	if err := authz.AllowClientAccess(policy.EffectiveClientAccessMode(), actor.Azp, "password", false); err != nil {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}

	s.writeMintedToken(w, r, actor, "password", req.Scope, refreshDeviceID(r, req), nil, nil)
}
