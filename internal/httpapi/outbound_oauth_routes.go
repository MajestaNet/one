package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/connectoroauth"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/egress"
	"github.com/MajestaNet/ide/internal/secretcrypt"
)

func (s *Server) registerConnectorOAuthRoutes() {
	authzMeta := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeMetadata, s.requireCapability(authz.CapMetadataBuild, h)))
	}
	s.mux.Handle("POST /auth/v1/connectors/{apiName}/authorize", authzMeta(s.handleConnectorOAuthAuthorize))
	s.mux.HandleFunc("GET /auth/v1/connectors/callback", s.handleConnectorOAuthCallback)
}

func (s *Server) connectorOAuthCallbackURL() string {
	base := "http://localhost:8080"
	if s.cfg != nil && s.cfg.PlatformPublicURL != "" {
		base = strings.TrimRight(s.cfg.PlatformPublicURL, "/")
	}
	return base + "/auth/v1/connectors/callback"
}

func (s *Server) handleConnectorOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := strings.TrimSpace(r.PathValue("apiName"))
	conn, err := db.GetInstallConnector(r.Context(), pool, apiName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "connector not found")
			return
		}
		writeAPIError(w, err)
		return
	}
	authType := connectoroauth.NormalizeAuthType(conn.AuthType)
	if authType != connectoroauth.AuthOAuth2AuthorizationCode {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "connector authType must be oauth2_authorization_code")
		return
	}
	if !conn.Active {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "connector is inactive")
		return
	}
	if err := connectoroauth.ValidateFlow(authType, conn.OAuthFlow); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	if err := egress.ValidateURL(conn.OAuthFlow.AuthorizationURL); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid authorizationUrl: "+err.Error())
		return
	}
	if err := egress.ValidateURL(conn.OAuthFlow.TokenURL); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid tokenUrl: "+err.Error())
		return
	}
	allow, err := db.ListEgressHostPatterns(r.Context(), pool)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if err := requireHostAllowlisted(conn.OAuthFlow.TokenURL, allow); err != nil {
		writeErr(w, http.StatusBadRequest, "EGRESS_DENIED", err.Error())
		return
	}
	if err := requireHostAllowlisted(conn.OAuthFlow.AuthorizationURL, allow); err != nil {
		writeErr(w, http.StatusBadRequest, "EGRESS_DENIED", err.Error())
		return
	}

	state, err := connectoroauth.NewState()
	if err != nil {
		writeAPIError(w, err)
		return
	}
	verifier, challenge := "", ""
	if conn.OAuthFlow.PKCE {
		verifier, challenge, err = connectoroauth.NewPKCE()
		if err != nil {
			writeAPIError(w, err)
			return
		}
	}
	redirectURI := s.connectorOAuthCallbackURL()
	secretRef := ""
	if conn.SecretRef != nil {
		secretRef = *conn.SecretRef
	}
	actor := ActorFromContext(r.Context())
	var actorID *string
	if actor != nil {
		actorID = &actor.ID
	}
	st := db.InstallConnectorOAuthState{
		StateHash:        connectoroauth.HashState(state),
		ConnectorAPIName: apiName,
		ActorID:          actorID,
		CodeVerifier:     verifier,
		RedirectURI:      redirectURI,
		ConfigHash:       connectoroauth.ConfigHash(authType, conn.OAuthFlow, secretRef),
		ExpiresAt:        time.Now().UTC().Add(10 * time.Minute),
	}
	if err := db.PutInstallConnectorOAuthState(r.Context(), pool, st); err != nil {
		writeAPIError(w, err)
		return
	}
	authURL, err := connectoroauth.BuildAuthorizeURL(conn.OAuthFlow, redirectURI, state, challenge)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authorizationUrl": authURL,
		"redirectUri":      redirectURI,
		"expiresIn":        600,
	})
}

func (s *Server) handleConnectorOAuthCallback(w http.ResponseWriter, r *http.Request) {
	pool := s.pool
	if pool == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}
	q := r.URL.Query()
	if errMsg := q.Get("error"); errMsg != "" {
		http.Error(w, "oauth error: "+errMsg+" "+q.Get("error_description"), http.StatusBadRequest)
		return
	}
	code := q.Get("code")
	state := q.Get("state")
	if code == "" || state == "" {
		http.Error(w, "code and state required", http.StatusBadRequest)
		return
	}
	st, err := db.TakeInstallConnectorOAuthState(r.Context(), pool, connectoroauth.HashState(state))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "invalid or reused state", http.StatusBadRequest)
			return
		}
		http.Error(w, "state lookup failed", http.StatusInternalServerError)
		return
	}
	if time.Now().UTC().After(st.ExpiresAt) {
		http.Error(w, "state expired", http.StatusBadRequest)
		return
	}
	conn, err := db.GetInstallConnector(r.Context(), pool, st.ConnectorAPIName)
	if err != nil {
		http.Error(w, "connector not found", http.StatusBadRequest)
		return
	}
	secretRef := ""
	if conn.SecretRef != nil {
		secretRef = *conn.SecretRef
	}
	cfgHash := connectoroauth.ConfigHash(connectoroauth.NormalizeAuthType(conn.AuthType), conn.OAuthFlow, secretRef)
	if cfgHash != st.ConfigHash {
		http.Error(w, "connector oauth config changed; restart authorize", http.StatusBadRequest)
		return
	}
	clientSecret := ""
	if secretRef != "" {
		ct, err := db.GetInstallSecretCiphertext(r.Context(), pool, secretRef)
		if err != nil {
			http.Error(w, "client secret unavailable", http.StatusBadRequest)
			return
		}
		clientSecret, err = secretcrypt.Decrypt(ct, s.encKey())
		if err != nil {
			http.Error(w, "decrypt client secret failed", http.StatusInternalServerError)
			return
		}
	}
	allow, _ := db.ListEgressHostPatterns(r.Context(), pool)
	if err := requireHostAllowlisted(conn.OAuthFlow.TokenURL, allow); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	httpClient := egress.Client
	tokClient := &connectoroauth.TokenHTTPClient{HTTPClient: httpClient}
	tok, err := tokClient.ExchangeAuthorizationCode(r.Context(), conn.OAuthFlow, st.RedirectURI, code, st.CodeVerifier, clientSecret)
	if err != nil {
		slog.Warn("connector oauth token exchange failed", "connector", conn.APIName, "err", err)
		http.Error(w, "token exchange failed", http.StatusBadGateway)
		return
	}
	raw, _ := json.Marshal(tok)
	ct, err := secretcrypt.Encrypt(string(raw), s.encKey())
	if err != nil {
		http.Error(w, "encrypt token failed", http.StatusInternalServerError)
		return
	}
	var exp *time.Time
	if !tok.Expiry.IsZero() {
		e := tok.Expiry.UTC()
		exp = &e
	}
	if err := db.UpsertInstallConnectorOAuthToken(r.Context(), pool, conn.APIName, ct, exp, tok.RefreshToken != ""); err != nil {
		http.Error(w, "persist token failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html><html><body><h1>Connected</h1><p>Connector <code>%s</code> is connected. You can close this window.</p></body></html>`,
		htmlEscape(conn.APIName))
}

func requireHostAllowlisted(rawURL string, allow []string) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return fmt.Errorf("invalid url")
	}
	if !egress.HostAllowed(u.Hostname(), allow) {
		return fmt.Errorf("host %q not on install egress allowlist", u.Hostname())
	}
	return nil
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}
