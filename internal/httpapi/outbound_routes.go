package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/connectoroauth"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/egress"
	"github.com/MajestaNet/ide/internal/secretcrypt"
)

func (s *Server) registerOutboundRoutes(prefix string) {
	meta := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeMetadata, h))
	}
	capMeta := func(cap string, h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeMetadata, s.requireCapability(cap, h)))
	}
	s.mux.Handle("GET "+prefix+"/secrets", meta(s.handleListInstallSecrets))
	s.mux.Handle("POST "+prefix+"/secrets", capMeta(authz.CapMetadataBuild, s.handleCreateInstallSecret))
	s.mux.Handle("POST "+prefix+"/secrets/{apiName}/rotate", capMeta(authz.CapMetadataBuild, s.handleRotateInstallSecret))
	s.mux.Handle("DELETE "+prefix+"/secrets/{apiName}", capMeta(authz.CapMetadataBuild, s.handleDeleteInstallSecret))

	s.mux.Handle("GET "+prefix+"/connectors", meta(s.handleListConnectors))
	s.mux.Handle("GET "+prefix+"/connectors/{apiName}", meta(s.handleGetConnector))
	s.mux.Handle("POST "+prefix+"/connectors", capMeta(authz.CapMetadataBuild, s.handleCreateConnector))
	s.mux.Handle("PATCH "+prefix+"/connectors/{apiName}", capMeta(authz.CapMetadataBuild, s.handlePatchConnector))
	s.mux.Handle("DELETE "+prefix+"/connectors/{apiName}", capMeta(authz.CapMetadataBuild, s.handleDeleteConnector))
	s.mux.Handle("GET "+prefix+"/connectors/{apiName}/oauth/status", meta(s.handleConnectorOAuthStatus))
	s.mux.Handle("DELETE "+prefix+"/connectors/{apiName}/oauth/connection", capMeta(authz.CapMetadataBuild, s.handleDisconnectConnectorOAuth))

	s.mux.Handle("GET "+prefix+"/install/egress", meta(s.handleListEgress))
	s.mux.Handle("POST "+prefix+"/install/egress", capMeta(authz.CapGovernNetwork, s.handleAddEgress))
	s.mux.Handle("DELETE "+prefix+"/install/egress/{hostPattern}", capMeta(authz.CapGovernNetwork, s.handleDeleteEgress))
}

func (s *Server) encKey() string {
	if s.cfg == nil {
		return ""
	}
	return s.cfg.WebhookEncryptionKey
}

func (s *Server) handleListInstallSecrets(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	list, err := db.ListInstallSecrets(r.Context(), pool)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, sec := range list {
		out = append(out, map[string]any{
			"apiName": sec.APIName, "label": sec.Label, "hasSecret": sec.HasSecret,
			"createdAt": sec.CreatedAt, "updatedAt": sec.UpdatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"secrets": out})
}

func (s *Server) handleCreateInstallSecret(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		APIName string `json:"apiName"`
		Label   string `json:"label"`
		Secret  string `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.APIName == "" || body.Secret == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "apiName and secret required")
		return
	}
	enc, err := secretcrypt.Encrypt(body.Secret, s.encKey())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "SECRET_ERROR", "failed to protect secret")
		return
	}
	if err := db.UpsertInstallSecret(r.Context(), pool, body.APIName, body.Label, enc); err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"apiName": body.APIName, "label": firstNonEmptyLabel(body.Label, body.APIName), "hasSecret": true,
	})
}

func (s *Server) handleRotateInstallSecret(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := r.PathValue("apiName")
	var body struct {
		Secret string `json:"secret"`
		Label  string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Secret == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "secret required")
		return
	}
	enc, err := secretcrypt.Encrypt(body.Secret, s.encKey())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "SECRET_ERROR", "failed to protect secret")
		return
	}
	label := body.Label
	if label == "" {
		label = apiName
	}
	if err := db.UpsertInstallSecret(r.Context(), pool, apiName, label, enc); err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"apiName": apiName, "hasSecret": true})
}

func (s *Server) handleDeleteInstallSecret(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	if err := db.DeleteInstallSecret(r.Context(), pool, r.PathValue("apiName")); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "secret not found")
			return
		}
		writeAPIError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func connectorJSON(c db.InstallConnector) map[string]any {
	m := map[string]any{
		"apiName": c.APIName, "label": c.Label, "baseUrl": c.BaseURL,
		"allowedMethods": c.AllowedMethods, "pathPrefix": c.PathPrefix,
		"active": c.Active, "authType": connectoroauth.NormalizeAuthType(c.AuthType),
		"oauthFlow": c.OAuthFlow,
		"createdAt": c.CreatedAt, "updatedAt": c.UpdatedAt,
	}
	if c.SecretRef != nil {
		m["secretRef"] = *c.SecretRef
	} else {
		m["secretRef"] = nil
	}
	return m
}

func parseOAuthFlowBody(raw any) (connectoroauth.Flow, error) {
	var flow connectoroauth.Flow
	if raw == nil {
		return flow, nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return flow, err
	}
	return connectoroauth.ParseFlowJSON(b)
}

func (s *Server) handleListConnectors(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	list, err := db.ListInstallConnectors(r.Context(), pool)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, c := range list {
		out = append(out, connectorJSON(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"connectors": out})
}

func (s *Server) handleGetConnector(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	c, err := db.GetInstallConnector(r.Context(), pool, r.PathValue("apiName"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "connector not found")
			return
		}
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, connectorJSON(*c))
}

func (s *Server) handleCreateConnector(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		APIName        string          `json:"apiName"`
		Label          string          `json:"label"`
		BaseURL        string          `json:"baseUrl"`
		SecretRef      *string         `json:"secretRef"`
		AllowedMethods []string        `json:"allowedMethods"`
		PathPrefix     string          `json:"pathPrefix"`
		Active         *bool           `json:"active"`
		AuthType       string          `json:"authType"`
		OAuthFlow      json.RawMessage `json:"oauthFlow"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.APIName == "" || body.BaseURL == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "apiName and baseUrl required")
		return
	}
	if err := egress.ValidateURL(body.BaseURL); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid baseUrl: "+err.Error())
		return
	}
	active := true
	if body.Active != nil {
		active = *body.Active
	}
	authType := connectoroauth.NormalizeAuthType(body.AuthType)
	if err := connectoroauth.ValidateAuthType(authType); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	flow, err := connectoroauth.ParseFlowJSON(body.OAuthFlow)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid oauthFlow")
		return
	}
	if err := connectoroauth.ValidateFlow(authType, flow); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	if authType != connectoroauth.AuthStaticBearer {
		if err := egress.ValidateURL(flow.TokenURL); err != nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid oauthFlow.tokenUrl: "+err.Error())
			return
		}
		if flow.AuthorizationURL != "" {
			if err := egress.ValidateURL(flow.AuthorizationURL); err != nil {
				writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid oauthFlow.authorizationUrl: "+err.Error())
				return
			}
		}
	}
	c := db.InstallConnector{
		APIName: body.APIName, Label: body.Label, BaseURL: body.BaseURL,
		SecretRef: body.SecretRef, AllowedMethods: body.AllowedMethods,
		PathPrefix: body.PathPrefix, Active: active,
		AuthType: authType, OAuthFlow: flow,
	}
	if err := db.UpsertInstallConnector(r.Context(), pool, c); err != nil {
		writeAPIError(w, err)
		return
	}
	got, err := db.GetInstallConnector(r.Context(), pool, body.APIName)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, connectorJSON(*got))
}

func (s *Server) handlePatchConnector(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := r.PathValue("apiName")
	cur, err := db.GetInstallConnector(r.Context(), pool, apiName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "connector not found")
			return
		}
		writeAPIError(w, err)
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON")
		return
	}
	if v, ok := body["label"].(string); ok {
		cur.Label = v
	}
	if v, ok := body["baseUrl"].(string); ok {
		if err := egress.ValidateURL(v); err != nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid baseUrl: "+err.Error())
			return
		}
		cur.BaseURL = v
	}
	if v, ok := body["pathPrefix"].(string); ok {
		cur.PathPrefix = v
	}
	if v, ok := body["active"].(bool); ok {
		cur.Active = v
	}
	if v, ok := body["secretRef"]; ok {
		if v == nil {
			cur.SecretRef = nil
		} else if s, ok := v.(string); ok {
			cur.SecretRef = &s
		}
	}
	if v, ok := body["allowedMethods"].([]any); ok {
		methods := make([]string, 0, len(v))
		for _, m := range v {
			if ms, ok := m.(string); ok {
				methods = append(methods, ms)
			}
		}
		cur.AllowedMethods = methods
	}
	prevAuth := connectoroauth.NormalizeAuthType(cur.AuthType)
	prevHash := connectoroauth.ConfigHash(prevAuth, cur.OAuthFlow, secretRefString(cur.SecretRef))
	if v, ok := body["authType"].(string); ok {
		authType := connectoroauth.NormalizeAuthType(v)
		if err := connectoroauth.ValidateAuthType(authType); err != nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		cur.AuthType = authType
	}
	if v, ok := body["oauthFlow"]; ok {
		flow, err := parseOAuthFlowBody(v)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid oauthFlow")
			return
		}
		cur.OAuthFlow = flow
	}
	authType := connectoroauth.NormalizeAuthType(cur.AuthType)
	if err := connectoroauth.ValidateFlow(authType, cur.OAuthFlow); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	if authType != connectoroauth.AuthStaticBearer {
		if err := egress.ValidateURL(cur.OAuthFlow.TokenURL); err != nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid oauthFlow.tokenUrl: "+err.Error())
			return
		}
		if cur.OAuthFlow.AuthorizationURL != "" {
			if err := egress.ValidateURL(cur.OAuthFlow.AuthorizationURL); err != nil {
				writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid oauthFlow.authorizationUrl: "+err.Error())
				return
			}
		}
	}
	newHash := connectoroauth.ConfigHash(authType, cur.OAuthFlow, secretRefString(cur.SecretRef))
	if err := db.UpsertInstallConnector(r.Context(), pool, *cur); err != nil {
		writeAPIError(w, err)
		return
	}
	if prevHash != newHash && authType != connectoroauth.AuthStaticBearer {
		_ = db.DeleteInstallConnectorOAuthToken(r.Context(), pool, apiName)
	}
	got, err := db.GetInstallConnector(r.Context(), pool, apiName)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, connectorJSON(*got))
}

func secretRefString(ref *string) string {
	if ref == nil {
		return ""
	}
	return *ref
}

func (s *Server) handleConnectorOAuthStatus(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := r.PathValue("apiName")
	c, err := db.GetInstallConnector(r.Context(), pool, apiName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "connector not found")
			return
		}
		writeAPIError(w, err)
		return
	}
	authType := connectoroauth.NormalizeAuthType(c.AuthType)
	out := map[string]any{
		"apiName": apiName, "authType": authType, "connected": false, "refreshable": false,
	}
	if authType == connectoroauth.AuthStaticBearer {
		out["connected"] = c.SecretRef != nil && *c.SecretRef != ""
		writeJSON(w, http.StatusOK, out)
		return
	}
	tok, err := db.GetInstallConnectorOAuthToken(r.Context(), pool, apiName)
	if err == nil && tok != nil && tok.TokenCiphertext != "" {
		out["connected"] = true
		out["refreshable"] = tok.Refreshable
		if tok.ExpiresAt != nil {
			out["expiresAt"] = *tok.ExpiresAt
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDisconnectConnectorOAuth(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := r.PathValue("apiName")
	if _, err := db.GetInstallConnector(r.Context(), pool, apiName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "connector not found")
			return
		}
		writeAPIError(w, err)
		return
	}
	if err := db.DeleteInstallConnectorOAuthToken(r.Context(), pool, apiName); err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"apiName": apiName, "connected": false})
}

func (s *Server) handleDeleteConnector(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := r.PathValue("apiName")
	if err := db.DeleteInstallConnector(r.Context(), pool, apiName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "connector not found")
			return
		}
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "apiName": apiName})
}

func (s *Server) handleListEgress(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	list, err := db.ListEgressAllowlist(r.Context(), pool)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, e := range list {
		out = append(out, map[string]any{
			"id": e.ID, "hostPattern": e.HostPattern, "label": e.Label, "createdAt": e.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"allowlist": out})
}

func (s *Server) handleAddEgress(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		HostPattern string `json:"hostPattern"`
		Label       string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.HostPattern) == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "hostPattern required")
		return
	}
	e, err := db.AddEgressAllowEntry(r.Context(), pool, body.HostPattern, body.Label)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": e.ID, "hostPattern": e.HostPattern, "label": e.Label, "createdAt": e.CreatedAt,
	})
}

func (s *Server) handleDeleteEgress(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	pattern, err := url.PathUnescape(r.PathValue("hostPattern"))
	if err != nil {
		pattern = r.PathValue("hostPattern")
	}
	if err := db.DeleteEgressAllowEntry(r.Context(), pool, pattern); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "egress entry not found")
			return
		}
		writeAPIError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func firstNonEmptyLabel(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
