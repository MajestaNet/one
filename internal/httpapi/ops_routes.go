package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/ops"
)

func (s *Server) registerOpsRoutes() {
	opsScope := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeOps, h))
	}
	opsAdmin := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeOps, s.requireAdmin(h)))
	}
	s.mux.Handle("GET /ops/v1/upgrades/available", opsScope(s.handleOpsAvailable))
	s.mux.Handle("GET /ops/v1/upgrades", opsScope(s.handleOpsListUpgrades))
	s.mux.Handle("GET /ops/v1/upgrades/{id}", opsScope(s.handleOpsGetUpgrade))
	s.mux.Handle("POST /ops/v1/upgrades", opsAdmin(s.handleOpsConfirmUpgrade))
	s.mux.Handle("POST /ops/v1/upgrades/{id}/rollback", opsAdmin(s.handleOpsRollbackUpgrade))
}

func (s *Server) requireOps() bool {
	return s.ops != nil
}

func (s *Server) handleOpsAvailable(w http.ResponseWriter, _ *http.Request) {
	if !s.requireOps() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "ops engine not configured")
		return
	}
	writeJSON(w, http.StatusOK, s.ops.GetAvailable())
}

func (s *Server) handleOpsListUpgrades(w http.ResponseWriter, r *http.Request) {
	if !s.requireOps() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "ops engine not configured")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := s.ops.List(r.Context(), limit)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"upgrades": rows})
}

func (s *Server) handleOpsGetUpgrade(w http.ResponseWriter, r *http.Request) {
	if !s.requireOps() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "ops engine not configured")
		return
	}
	row, err := s.ops.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (s *Server) handleOpsConfirmUpgrade(w http.ResponseWriter, r *http.Request) {
	if !s.requireOps() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "ops engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	var body struct {
		APIImage       string `json:"apiImage"`
		WorkerImage    string `json:"workerImage"`
		ProductVersion string `json:"productVersion"`
		SkipRoll       bool   `json:"skipRoll"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	row, err := s.ops.Confirm(r.Context(), ops.ConfirmInput{
		APIImage:       body.APIImage,
		WorkerImage:    body.WorkerImage,
		ProductVersion: body.ProductVersion,
		Actor:          actor,
		SkipRoll:       body.SkipRoll,
	})
	if row != nil {
		status := http.StatusCreated
		if row.Status == ops.StatusFailed || row.Status == ops.StatusRolledBack {
			status = http.StatusUnprocessableEntity
		}
		writeJSON(w, status, row)
		return
	}
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeErr(w, http.StatusInternalServerError, "INTERNAL", "upgrade produced no result")
}

func (s *Server) handleOpsRollbackUpgrade(w http.ResponseWriter, r *http.Request) {
	if !s.requireOps() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "ops engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	row, err := s.ops.Rollback(r.Context(), r.PathValue("id"), actor)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}
