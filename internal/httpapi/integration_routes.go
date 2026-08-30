package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/integration"
)

func (s *Server) registerIntegrationRoutes(prefix string) {
	capClient := func(cap string, h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeClient, s.requireCapability(cap, h)))
	}
	s.mux.Handle("GET "+prefix+"/integrations", capClient(authz.CapIdentityIntegrations, s.handleListIntegrations))
	s.mux.Handle("POST "+prefix+"/integrations", capClient(authz.CapIdentityIntegrations, s.handleCreateIntegration))
	s.mux.Handle("GET "+prefix+"/integrations/{apiName}", capClient(authz.CapIdentityIntegrations, s.handleGetIntegration))
	s.mux.Handle("PATCH "+prefix+"/integrations/{apiName}", capClient(authz.CapIdentityIntegrations, s.handlePatchIntegration))
	s.mux.Handle("DELETE "+prefix+"/integrations/{apiName}", capClient(authz.CapIdentityIntegrations, s.handleDeleteIntegration))
	s.mux.Handle("POST "+prefix+"/integrations/{apiName}/secrets/rotate", capClient(authz.CapIdentityIntegrations, s.handleRotateIntegrationSecrets))
	s.mux.Handle("POST "+prefix+"/integrations/{apiName}/secrets/reveal", capClient(authz.CapIdentityIntegrations, s.handleRevealIntegrationSecrets))
}

func (s *Server) integrationService() (*integration.Service, bool) {
	pool := s.pool
	if pool == nil {
		return nil, false
	}
	encKey := ""
	issuer := ""
	if s.cfg != nil {
		encKey = s.cfg.WebhookEncryptionKey
		if encKey == "" {
			encKey = s.cfg.AuthJWTSigningKey
		}
		issuer = s.identityLinkIssuer()
	}
	return &integration.Service{
		Pool:           pool,
		Identity:       s.identity,
		EncryptionKey:  encKey,
		IdentityIssuer: issuer,
	}, true
}

func writeIntegrationErr(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, integration.ErrValidation):
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	case errors.Is(err, integration.ErrConflict):
		writeErr(w, http.StatusConflict, "CONFLICT", err.Error())
	case errors.Is(err, integration.ErrNotFound):
		writeErr(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, integration.ErrForbidden):
		writeErr(w, http.StatusForbidden, "FORBIDDEN", err.Error())
	default:
		writeAPIError(w, err)
	}
}

func (s *Server) handleListIntegrations(w http.ResponseWriter, r *http.Request) {
	svc, ok := s.integrationService()
	if !ok {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	list, err := svc.List(r.Context())
	if err != nil {
		writeIntegrationErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (s *Server) handleCreateIntegration(w http.ResponseWriter, r *http.Request) {
	svc, ok := s.integrationService()
	if !ok {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	var body struct {
		APIName           string   `json:"apiName"`
		Label             string   `json:"label"`
		Description       string   `json:"description"`
		ClientKind        string   `json:"clientKind"`
		OAuthFlows        []string `json:"oauthFlows"`
		CallbackURLs      []string `json:"callbackUrls"`
		LogoutURLs        []string `json:"logoutUrls"`
		AllowedScopesHint []string `json:"allowedScopesHint"`
		PKCERequired      *bool    `json:"pkceRequired"`
		RoleAPINames      []string `json:"roleApiNames"`
		PrincipalEmail    string   `json:"principalEmail"`
		PrincipalName     string   `json:"principalName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	res, err := svc.Create(r.Context(), integration.CreateInput{
		APIName:           body.APIName,
		Label:             body.Label,
		Description:       body.Description,
		ClientKind:        body.ClientKind,
		OAuthFlows:        body.OAuthFlows,
		CallbackURLs:      body.CallbackURLs,
		LogoutURLs:        body.LogoutURLs,
		AllowedScopesHint: body.AllowedScopesHint,
		PKCERequired:      body.PKCERequired,
		RoleAPINames:      body.RoleAPINames,
		PrincipalEmail:    body.PrincipalEmail,
		PrincipalName:     body.PrincipalName,
	})
	if err != nil {
		writeIntegrationErr(w, err)
		return
	}
	s.writeAudit(r, "integration.create", "", nil, map[string]any{"apiName": res.View.APIName, "principalId": res.View.PrincipalID})
	out := map[string]any{
		"id": res.View.ID, "apiName": res.View.APIName, "label": res.View.Label,
		"description": res.View.Description, "principalId": res.View.PrincipalID,
		"clientKind": res.View.ClientKind, "oauthFlows": res.View.OAuthFlows,
		"callbackUrls": res.View.CallbackURLs, "logoutUrls": res.View.LogoutURLs,
		"allowedScopesHint": res.View.AllowedScopesHint, "pkceRequired": res.View.PKCERequired,
		"ownership": res.View.Ownership, "packageName": res.View.PackageName,
		"isActive": res.View.IsActive, "hasOneSecret": res.View.HasOneSecret,
		"createdAt": res.View.CreatedAt, "updatedAt": res.View.UpdatedAt,
	}
	if res.OneClientSecret != "" {
		out["oneClientSecret"] = res.OneClientSecret
		out["clientId"] = res.View.PrincipalID
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleGetIntegration(w http.ResponseWriter, r *http.Request) {
	svc, ok := s.integrationService()
	if !ok {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	v, err := svc.Get(r.Context(), r.PathValue("apiName"))
	if err != nil {
		writeIntegrationErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handlePatchIntegration(w http.ResponseWriter, r *http.Request) {
	svc, ok := s.integrationService()
	if !ok {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	var body struct {
		Label             *string   `json:"label"`
		Description       *string   `json:"description"`
		OAuthFlows        *[]string `json:"oauthFlows"`
		CallbackURLs      *[]string `json:"callbackUrls"`
		LogoutURLs        *[]string `json:"logoutUrls"`
		AllowedScopesHint *[]string `json:"allowedScopesHint"`
		AllowedCIDRs      *[]string `json:"allowedCidrs"`
		PKCERequired      *bool     `json:"pkceRequired"`
		IsActive          *bool     `json:"isActive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	apiName := r.PathValue("apiName")
	v, err := svc.Patch(r.Context(), apiName, integration.PatchInput{
		Label: body.Label, Description: body.Description, OAuthFlows: body.OAuthFlows,
		CallbackURLs: body.CallbackURLs, LogoutURLs: body.LogoutURLs,
		AllowedScopesHint: body.AllowedScopesHint, AllowedCIDRs: body.AllowedCIDRs,
		PKCERequired: body.PKCERequired, IsActive: body.IsActive,
	})
	if err != nil {
		writeIntegrationErr(w, err)
		return
	}
	s.writeAudit(r, "integration.update", "", nil, map[string]any{"apiName": apiName})
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleDeleteIntegration(w http.ResponseWriter, r *http.Request) {
	svc, ok := s.integrationService()
	if !ok {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	apiName := r.PathValue("apiName")
	if err := svc.Delete(r.Context(), apiName); err != nil {
		writeIntegrationErr(w, err)
		return
	}
	s.writeAudit(r, "integration.delete", "", nil, map[string]any{"apiName": apiName})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "apiName": apiName})
}

func (s *Server) handleRotateIntegrationSecrets(w http.ResponseWriter, r *http.Request) {
	svc, ok := s.integrationService()
	if !ok {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	apiName := r.PathValue("apiName")
	res, err := svc.Rotate(r.Context(), apiName)
	if err != nil {
		writeIntegrationErr(w, err)
		return
	}
	s.writeAudit(r, "integration.secrets.rotate", "", nil, map[string]any{"apiName": apiName})
	out := map[string]any{
		"apiName": res.View.APIName, "hasOneSecret": res.View.HasOneSecret,
		"clientId": res.View.PrincipalID,
	}
	if res.OneClientSecret != "" {
		out["oneClientSecret"] = res.OneClientSecret
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleRevealIntegrationSecrets(w http.ResponseWriter, r *http.Request) {
	svc, ok := s.integrationService()
	if !ok {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	apiName := r.PathValue("apiName")
	res, err := svc.Reveal(r.Context(), apiName)
	if err != nil {
		writeIntegrationErr(w, err)
		return
	}
	s.writeAudit(r, "integration.secrets.reveal", "", nil, map[string]any{"apiName": apiName})
	writeJSON(w, http.StatusOK, res)
}
