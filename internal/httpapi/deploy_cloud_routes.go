package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/deploy"
	"github.com/MajestaNet/ide/internal/inference"
)

func (s *Server) registerDeployCloudRoutes() {
	wrap := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeDeploy, h))
	}
	// Mutating cloud routes: deploy scope + admin (BP-030 / agnostic uplift).
	cloudAdmin := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeDeploy, s.requireAdmin(h)))
	}

	// Primary host-free surface.
	s.mux.Handle("GET /deploy/v1/cloud/status", wrap(s.handleCloudStatus))
	s.mux.Handle("PUT /deploy/v1/cloud/binding", cloudAdmin(s.handleCloudBinding))
	s.mux.Handle("GET /deploy/v1/cloud/app", wrap(s.handleCloudApp))
	s.mux.Handle("PATCH /deploy/v1/cloud/app/scale", cloudAdmin(s.handleCloudScale))
	s.mux.Handle("PATCH /deploy/v1/cloud/database/resize", cloudAdmin(s.handleCloudResizeDB))
	s.mux.Handle("POST /deploy/v1/cloud/app/redeploy", cloudAdmin(s.handleCloudRedeploy))
	s.mux.Handle("POST /deploy/v1/cloud/environments", cloudAdmin(s.handleCloudProvision))
	s.mux.Handle("GET /deploy/v1/cloud/environments", wrap(s.handleCloudListEnvironments))
	s.mux.Handle("GET /deploy/v1/cloud/inference", wrap(s.handleCloudInferenceGet))
	s.mux.Handle("PUT /deploy/v1/cloud/inference", cloudAdmin(s.handleCloudInferencePut))

	// DigitalOcean compatibility aliases → same handlers.
	s.mux.Handle("GET /deploy/v1/cloud/digitalocean/status", wrap(s.handleCloudStatus))
	s.mux.Handle("PUT /deploy/v1/cloud/digitalocean/binding", cloudAdmin(s.handleCloudBinding))
	s.mux.Handle("GET /deploy/v1/cloud/digitalocean/app", wrap(s.handleCloudApp))
	s.mux.Handle("PATCH /deploy/v1/cloud/digitalocean/app/scale", cloudAdmin(s.handleCloudScale))
	s.mux.Handle("PATCH /deploy/v1/cloud/digitalocean/database/resize", cloudAdmin(s.handleCloudResizeDB))
	s.mux.Handle("POST /deploy/v1/cloud/digitalocean/app/redeploy", cloudAdmin(s.handleCloudRedeploy))
	s.mux.Handle("POST /deploy/v1/cloud/digitalocean/environments", cloudAdmin(s.handleCloudProvision))
	s.mux.Handle("GET /deploy/v1/cloud/digitalocean/environments", wrap(s.handleCloudListEnvironments))
	s.mux.Handle("GET /deploy/v1/cloud/digitalocean/inference", wrap(s.handleCloudInferenceGet))
	s.mux.Handle("PUT /deploy/v1/cloud/digitalocean/inference", cloudAdmin(s.handleCloudInferencePut))
}

func (s *Server) handleCloudStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	st, err := s.deploy.GetCloudStatus(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleCloudBinding(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	var body deploy.BindInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	b, err := s.deploy.PutCloudBinding(r.Context(), body)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (s *Server) handleCloudApp(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	app, err := s.deploy.GetCloudApp(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (s *Server) handleCloudScale(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	var body deploy.ScaleInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	app, err := s.deploy.ScaleCloudApp(r.Context(), body)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (s *Server) handleCloudResizeDB(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	var body deploy.ResizeDatabaseInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	db, err := s.deploy.ResizeCloudDatabase(r.Context(), body)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, db)
}

func (s *Server) handleCloudRedeploy(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	var body deploy.RedeployInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	app, err := s.deploy.RedeployCloudApp(r.Context(), body)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, app)
}

func (s *Server) handleCloudProvision(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	var body deploy.ProvisionPeerInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	actor := ActorFromContext(r.Context())
	if actor != nil && actor.ID != "" {
		body.CreatedBy = &actor.ID
	}
	result, err := s.deploy.ProvisionCloudEnvironment(r.Context(), body)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleCloudListEnvironments(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	out, err := s.deploy.ListCloudEnvironments(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCloudInferenceGet(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	cfg, err := inference.GetConfig(r.Context(), pool)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	providers, _ := inference.ListProviders(r.Context(), pool)
	writeJSON(w, http.StatusOK, inference.StatusJSON(cfg, providers, s.doTokenConfigured()))
}

func (s *Server) handleCloudInferencePut(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		Enabled bool   `json:"enabled"`
		Mode    string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	mode := inference.ModeDev
	if body.Mode != "" {
		mode = inference.Mode(strings.ToLower(strings.TrimSpace(body.Mode)))
	}
	if body.Enabled {
		if !s.doTokenConfigured() {
			writeErr(w, http.StatusBadRequest, "DO_TOKEN_MISSING", "DIGITALOCEAN_API_TOKEN must be set on this install before enabling Native DigitalOcean Inference")
			return
		}
		if err := inference.ValidateMode(mode); err != nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", `mode must be "dev", "standard", or "pro"`)
			return
		}
	}
	cfg, err := inference.PutDOConfig(r.Context(), pool, body.Enabled, mode)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	providers, _ := inference.ListProviders(r.Context(), pool)
	writeJSON(w, http.StatusOK, inference.StatusJSON(cfg, providers, s.doTokenConfigured()))
}
