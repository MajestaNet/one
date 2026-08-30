package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/inference"
	"github.com/MajestaNet/ide/internal/secretcrypt"
)

func (s *Server) registerInferenceRoutes(prefix string) {
	meta := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeMetadata, h))
	}
	capMeta := func(cap string, h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeMetadata, s.requireCapability(cap, h)))
	}
	s.mux.Handle("GET "+prefix+"/inference/config", meta(s.handleGetInferenceConfig))
	s.mux.Handle("PATCH "+prefix+"/inference/config", capMeta(authz.CapMetadataBuild, s.handlePatchInferenceConfig))
	s.mux.Handle("GET "+prefix+"/inference/providers", meta(s.handleListInferenceProviders))
	s.mux.Handle("POST "+prefix+"/inference/providers", capMeta(authz.CapMetadataBuild, s.handleCreateInferenceProvider))
	s.mux.Handle("PATCH "+prefix+"/inference/providers/{apiName}", capMeta(authz.CapMetadataBuild, s.handlePatchInferenceProvider))
	s.mux.Handle("DELETE "+prefix+"/inference/providers/{apiName}", capMeta(authz.CapMetadataBuild, s.handleDeleteInferenceProvider))
}

func (s *Server) doTokenConfigured() bool {
	return s.cfg != nil && strings.TrimSpace(s.cfg.DigitalOceanAPIToken) != ""
}

func (s *Server) handleGetInferenceConfig(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	cfg, err := inference.GetConfig(r.Context(), pool)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	providers, err := inference.ListProviders(r.Context(), pool)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inference.StatusJSON(cfg, providers, s.doTokenConfigured()))
}

func (s *Server) handlePatchInferenceConfig(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		ActiveSource           *string `json:"activeSource"`
		DefaultProviderAPIName *string `json:"defaultProviderApiName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	active := false
	if body.ActiveSource != nil {
		switch strings.ToLower(strings.TrimSpace(*body.ActiveSource)) {
		case "byo":
			active = true
		case "none", "":
			active = false
		case "digitalocean":
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Use Deploy /deploy/v1/cloud/inference to enable Native DigitalOcean Inference")
			return
		default:
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "activeSource must be none or byo")
			return
		}
	} else if body.DefaultProviderAPIName != nil {
		active = true
	}
	cfg, err := inference.PatchBYOConfig(r.Context(), pool, active, body.DefaultProviderAPIName)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	providers, _ := inference.ListProviders(r.Context(), pool)
	writeJSON(w, http.StatusOK, inference.StatusJSON(cfg, providers, s.doTokenConfigured()))
}

func (s *Server) handleListInferenceProviders(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	list, err := inference.ListProviders(r.Context(), pool)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, p := range list {
		m := map[string]any{
			"apiName": p.APIName, "label": p.Label, "baseUrl": p.BaseURL,
			"defaultModel": p.DefaultModel, "active": p.Active,
			"hasSecret": p.SecretRef != nil && *p.SecretRef != "",
			"createdAt": p.CreatedAt, "updatedAt": p.UpdatedAt,
		}
		if p.SecretRef != nil {
			m["secretRef"] = *p.SecretRef
		}
		out = append(out, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": out})
}

func (s *Server) handleCreateInferenceProvider(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		APIName      string `json:"apiName"`
		Label        string `json:"label"`
		BaseURL      string `json:"baseUrl"`
		SecretRef    string `json:"secretRef"`
		APIKey       string `json:"apiKey"`
		DefaultModel string `json:"defaultModel"`
		Active       *bool  `json:"active"`
		SetDefault   bool   `json:"setDefault"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.APIName == "" || body.BaseURL == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "apiName and baseUrl required")
		return
	}
	if err := inference.ValidateProviderBaseURL(body.BaseURL, s.allowDevLocalInference()); err != nil {
		msg := "baseUrl must be https and not target private/metadata hosts"
		if s.allowDevLocalInference() {
			msg = "baseUrl must be https (or http://localhost / host.docker.internal in development) and not target private/metadata hosts"
		}
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", msg)
		return
	}
	label := body.Label
	if label == "" {
		label = body.APIName
	}
	secretRef := strings.TrimSpace(body.SecretRef)
	if body.APIKey != "" {
		if secretRef == "" {
			secretRef = "inference." + body.APIName
		}
		enc, err := secretcrypt.Encrypt(body.APIKey, s.encKey())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "SECRET_ERROR", "failed to protect secret")
			return
		}
		if err := db.UpsertInstallSecret(r.Context(), pool, secretRef, label+" API key", enc); err != nil {
			writeAPIError(w, err)
			return
		}
	}
	active := true
	if body.Active != nil {
		active = *body.Active
	}
	var ref *string
	if secretRef != "" {
		ref = &secretRef
	}
	p := inference.Provider{
		APIName: body.APIName, Label: label, BaseURL: body.BaseURL,
		SecretRef: ref, DefaultModel: body.DefaultModel, Active: active,
	}
	if err := inference.UpsertProvider(r.Context(), pool, p); err != nil {
		writeAPIError(w, err)
		return
	}
	if body.SetDefault {
		_, _ = inference.PatchBYOConfig(r.Context(), pool, true, &body.APIName)
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"apiName": p.APIName, "label": p.Label, "baseUrl": p.BaseURL,
		"defaultModel": p.DefaultModel, "active": p.Active, "hasSecret": ref != nil,
		"secretRef": secretRef,
	})
}

func (s *Server) handlePatchInferenceProvider(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := r.PathValue("apiName")
	existing, err := inference.GetProvider(r.Context(), pool, apiName)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "provider not found")
		return
	}
	var body struct {
		Label        *string `json:"label"`
		BaseURL      *string `json:"baseUrl"`
		SecretRef    *string `json:"secretRef"`
		APIKey       *string `json:"apiKey"`
		DefaultModel *string `json:"defaultModel"`
		Active       *bool   `json:"active"`
		SetDefault   bool    `json:"setDefault"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	if body.Label != nil {
		existing.Label = *body.Label
	}
	if body.BaseURL != nil {
		if err := inference.ValidateProviderBaseURL(*body.BaseURL, s.allowDevLocalInference()); err != nil {
			msg := "baseUrl must be https and not target private/metadata hosts"
			if s.allowDevLocalInference() {
				msg = "baseUrl must be https (or http://localhost / host.docker.internal in development) and not target private/metadata hosts"
			}
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", msg)
			return
		}
		existing.BaseURL = *body.BaseURL
	}
	if body.DefaultModel != nil {
		existing.DefaultModel = *body.DefaultModel
	}
	if body.Active != nil {
		existing.Active = *body.Active
	}
	if body.SecretRef != nil {
		existing.SecretRef = body.SecretRef
	}
	if body.APIKey != nil && *body.APIKey != "" {
		ref := "inference." + apiName
		if existing.SecretRef != nil && *existing.SecretRef != "" {
			ref = *existing.SecretRef
		}
		enc, err := secretcrypt.Encrypt(*body.APIKey, s.encKey())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "SECRET_ERROR", "failed to protect secret")
			return
		}
		if err := db.UpsertInstallSecret(r.Context(), pool, ref, existing.Label+" API key", enc); err != nil {
			writeAPIError(w, err)
			return
		}
		existing.SecretRef = &ref
	}
	if err := inference.UpsertProvider(r.Context(), pool, *existing); err != nil {
		writeAPIError(w, err)
		return
	}
	if body.SetDefault {
		_, _ = inference.PatchBYOConfig(r.Context(), pool, true, &apiName)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"apiName": existing.APIName, "label": existing.Label, "baseUrl": existing.BaseURL,
		"defaultModel": existing.DefaultModel, "active": existing.Active,
		"hasSecret": existing.SecretRef != nil && *existing.SecretRef != "",
	})
}

func (s *Server) handleDeleteInferenceProvider(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := r.PathValue("apiName")
	if err := inference.DeleteProvider(r.Context(), pool, apiName); err != nil {
		writeAPIError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
