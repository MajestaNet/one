package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
)

func (s *Server) handleAuthRefreshGrant(w http.ResponseWriter, r *http.Request, req tokenRequest) {
	if s.pool == nil {
		writeErr(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database required")
		return
	}
	raw := strings.TrimSpace(req.RefreshToken)
	if raw == "" {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "refresh_token is required")
		return
	}

	store := db.NewRefreshTokenStore(s.pool)
	presentedHash := authz.HashRefreshToken(raw)
	presented, err := store.GetByHash(r.Context(), presentedHash)
	if err != nil || presented == nil || !authz.EqualRefreshHash(presented.TokenHash, presentedHash) {
		writeErr(w, http.StatusUnauthorized, "INVALID_GRANT", "invalid grant")
		return
	}
	if clientID := strings.TrimSpace(req.ClientID); clientID != "" && clientID != presented.Azp {
		writeErr(w, http.StatusUnauthorized, "INVALID_GRANT", "invalid grant")
		return
	}

	users := db.NewUserStore(s.pool)
	u, err := users.GetByID(r.Context(), presented.UserID)
	if err != nil || !u.CanAuthenticate() {
		_ = store.RevokeFamily(r.Context(), presented.FamilyID)
		writeErr(w, http.StatusUnauthorized, "INVALID_GRANT", "invalid grant")
		return
	}
	actor, err := s.actorFromUser(r.Context(), users, u, "")
	if err != nil {
		if errors.Is(err, authz.ErrPrincipalNoRole) {
			writeErr(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		writeErr(w, http.StatusUnauthorized, "INVALID_GRANT", "invalid grant")
		return
	}
	actor.AuthMethod = authz.AuthMethodOneJWT
	actor.Azp = presented.Azp

	policy, err := s.loadExposurePolicy(r.Context())
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "POLICY_UNAVAILABLE", "access policy unavailable")
		return
	}
	if err := authz.AllowClientAccess(policy.EffectiveClientAccessMode(), actor.Azp, authz.GrantRefreshToken, false); err != nil {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}
	if err := authz.AllowBearerAzp(policy.EffectiveClientAccessMode(), actor.Azp, authz.GrantRefreshToken, false); err != nil {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		return
	}

	// Rotate only after every fallible principal/policy check. Otherwise a
	// transient policy-store error consumes the old token while the replacement
	// is never delivered, irrecoverably ending an otherwise valid session.
	idle, bytes := s.refreshIdleTTL(), s.refreshBytes()
	issued, err := authz.RotateRefreshToken(r.Context(), store, raw, strings.TrimSpace(req.ClientID), idle, bytes)
	if err != nil {
		if errors.Is(err, authz.ErrRefreshReuse) {
			s.writeAudit(r, "token.refresh.reuse", "", nil, map[string]any{"azp": strings.TrimSpace(req.ClientID)})
		}
		writeErr(w, http.StatusUnauthorized, "INVALID_GRANT", "invalid grant")
		return
	}

	s.writeAudit(r, "token.refresh", "", nil, map[string]any{
		"userId": issued.Token.UserID, "azp": issued.Token.Azp, "familyId": issued.Token.FamilyID,
	})
	s.writeMintedToken(w, r, actor, authz.GrantRefreshToken, "", issued.Token.DeviceID, nil, issued)
}

func (s *Server) handleAuthRevoke(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token         string `json:"token"`
		TokenTypeHint string `json:"token_type_hint"`
	}
	if err := parseRevokeRequest(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	token := strings.TrimSpace(req.Token)
	key := "ip:" + clientKey(r)
	if token != "" {
		key = authz.RefreshRateLimitKey(token)
	}
	if !s.authTokenLimiter.allow(key) {
		writeErr(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many token requests")
		return
	}
	hint := strings.TrimSpace(strings.ToLower(req.TokenTypeHint))
	if hint != "access_token" && token != "" && s.pool != nil {
		store := db.NewRefreshTokenStore(s.pool)
		if row, err := store.GetByHash(r.Context(), authz.HashRefreshToken(token)); err == nil && row != nil {
			_ = store.RevokeFamily(r.Context(), row.FamilyID)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func parseRevokeRequest(r *http.Request, dst *struct {
	Token         string `json:"token"`
	TokenTypeHint string `json:"token_type_hint"`
}) error {
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return err
		}
		dst.Token = r.FormValue("token")
		dst.TokenTypeHint = r.FormValue("token_type_hint")
		return nil
	}
	defer func() { _ = r.Body.Close() }()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func (s *Server) writeMintedToken(w http.ResponseWriter, r *http.Request, actor *authz.Actor, grant, requestedScope, deviceID string, extra map[string]any, rotated *authz.IssuedRefresh) {
	if s.resolver == nil || s.resolver.One == nil {
		writeErr(w, http.StatusInternalServerError, "TOKEN_ERROR", "failed to mint access token")
		return
	}
	token, expiresIn, err := s.resolver.One.MintAccessToken(actor)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "TOKEN_ERROR", "failed to mint access token")
		return
	}
	scopeParts := make([]string, 0, len(actor.Scopes))
	for _, sc := range actor.Scopes {
		scopeParts = append(scopeParts, string(sc))
	}
	resp := tokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
		Scope:       strings.Join(scopeParts, " "),
	}
	if rotated != nil {
		resp.RefreshToken = rotated.Raw
		resp.RefreshExpiresIn = rotated.ExpiresIn
	} else {
		issued, err := s.maybeIssueRefresh(r.Context(), actor, grant, requestedScope, deviceID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "TOKEN_ERROR", "failed to mint refresh token")
			return
		}
		if issued != nil {
			resp.RefreshToken = issued.Raw
			resp.RefreshExpiresIn = issued.ExpiresIn
		}
	}
	if extra == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "TOKEN_ERROR", "failed to mint access token")
		return
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		writeErr(w, http.StatusInternalServerError, "TOKEN_ERROR", "failed to mint access token")
		return
	}
	for k, v := range extra {
		out[k] = v
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) maybeIssueRefresh(ctx context.Context, actor *authz.Actor, grant, requestedScope, deviceID string) (*authz.IssuedRefresh, error) {
	if s.pool == nil || actor == nil || actor.ID == "" {
		return nil, nil
	}
	kind := s.connectedAppKind(ctx, actor.Azp)
	if !authz.ShouldIssueRefresh(actor.Azp, grant, strings.Fields(requestedScope), kind) {
		return nil, nil
	}
	return authz.IssueRefreshToken(ctx, db.NewRefreshTokenStore(s.pool), s.refreshIssueOpts(actor.ID, actor.Azp, deviceID))
}

func (s *Server) connectedAppKind(ctx context.Context, azp string) string {
	if s.pool == nil || strings.TrimSpace(azp) == "" {
		return ""
	}
	c, err := db.NewIntegrationStore(s.pool).GetByAPIName(ctx, azp)
	if err != nil {
		return ""
	}
	return c.ClientKind
}

func (s *Server) refreshIssueOpts(userID, azp, deviceID string) authz.IssueRefreshOpts {
	return authz.IssueRefreshOpts{
		UserID:   userID,
		Azp:      azp,
		DeviceID: deviceID,
		IdleTTL:  s.refreshIdleTTL(),
		AbsTTL:   s.refreshAbsTTL(),
		Bytes:    s.refreshBytes(),
	}
}

func (s *Server) refreshIdleTTL() time.Duration {
	sec := authz.DefaultRefreshIdleSeconds
	if s.cfg != nil && s.cfg.AuthRefreshIdleSeconds > 0 {
		sec = s.cfg.AuthRefreshIdleSeconds
	}
	return time.Duration(sec) * time.Second
}

func (s *Server) refreshAbsTTL() time.Duration {
	sec := authz.DefaultRefreshAbsSeconds
	if s.cfg != nil && s.cfg.AuthRefreshAbsSeconds > 0 {
		sec = s.cfg.AuthRefreshAbsSeconds
	}
	return time.Duration(sec) * time.Second
}

func (s *Server) refreshBytes() int {
	n := authz.DefaultRefreshBytes
	if s.cfg != nil && s.cfg.AuthRefreshBytes > 0 {
		n = s.cfg.AuthRefreshBytes
	}
	return n
}

func (s *Server) revokeRefreshForUser(ctx context.Context, userID string) error {
	if s.pool == nil || strings.TrimSpace(userID) == "" {
		return nil
	}
	_, err := db.NewRefreshTokenStore(s.pool).RevokeAllForUser(ctx, userID)
	return err
}

func refreshDeviceID(r *http.Request, req tokenRequest) string {
	if d := strings.TrimSpace(req.DeviceID); d != "" {
		return d
	}
	return strings.TrimSpace(r.Header.Get("X-One-Device-Id"))
}
