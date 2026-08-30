package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/edge"
)

func (s *Server) registerExposureRoutes(prefix string) {
	capMeta := func(cap string, h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeMetadata, s.requireCapability(cap, h)))
	}
	s.mux.Handle("GET "+prefix+"/install/exposure", capMeta(authz.CapGovernNetwork, s.handleGetExposure))
	s.mux.Handle("PUT "+prefix+"/install/exposure", capMeta(authz.CapGovernNetwork, s.handlePutExposure))
	s.mux.Handle("POST "+prefix+"/install/exposure/apply", capMeta(authz.CapGovernNetwork, s.handleApplyExposure))
}

func exposureResponse(row *db.ExposurePolicyRow, rollerMode string) map[string]any {
	mode := row.Policy.EffectiveClientAccessMode()
	return map[string]any{
		"clientAccessMode":  mode,
		"requireDeviceCert": row.Policy.RequireDeviceCert,
		"client":            row.Policy.Client,
		"auth":              row.Policy.Auth,
		"metadata":          row.Policy.Metadata,
		"deploy":            row.Policy.Deploy,
		"ops":               row.Policy.Ops,
		"status":            row.Status,
		"lastError":         row.LastError,
		"updatedAt":         row.UpdatedAt,
		"appliedAt":         row.AppliedAt,
		"rollerMode":        rollerMode,
	}
}

func (s *Server) edgeRollerMode() string {
	if s.edgeRoller == nil {
		return "local"
	}
	return s.edgeRoller.Mode()
}

func (s *Server) handleGetExposure(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	store := db.NewExposureStore(pool)
	row, err := store.Get(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, exposureResponse(row, s.edgeRollerMode()))
}

func (s *Server) handlePutExposure(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body edge.Policy
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	if err := edge.Validate(body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	store := db.NewExposureStore(pool)
	if err := store.Put(r.Context(), body, edge.StatusPending, nil); err != nil {
		writeAPIError(w, err)
		return
	}
	row, err := s.reconcileExposure(r, store)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "install.exposure.put", "", nil, map[string]any{"status": row.Status})
	writeJSON(w, http.StatusOK, exposureResponse(row, s.edgeRollerMode()))
}

func (s *Server) handleApplyExposure(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	store := db.NewExposureStore(pool)
	row, err := store.Get(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if row.Policy.ClientAccessMode == edge.ClientAccessIDEUsers {
		row.Policy.ClientAccessMode = edge.ClientAccessOpen
	}
	if err := edge.Validate(row.Policy); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	if err := store.Put(r.Context(), row.Policy, edge.StatusPending, nil); err != nil {
		writeAPIError(w, err)
		return
	}
	row, err = s.reconcileExposure(r, store)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "install.exposure.apply", "", nil, map[string]any{"status": row.Status})
	writeJSON(w, http.StatusOK, exposureResponse(row, s.edgeRollerMode()))
}

func (s *Server) reconcileExposure(r *http.Request, store *db.ExposureStore) (*db.ExposurePolicyRow, error) {
	row, err := store.Get(r.Context())
	if err != nil {
		return nil, err
	}
	policy := row.Policy
	// Merge Connected App allowedCidrs into Client/Auth allowlists for edge apply only (desired-state JSON unchanged).
	if s.pool != nil && (policy.Client.Mode == edge.ModeAllowlist || policy.Auth.Mode == edge.ModeAllowlist) {
		extra, _ := db.ListIntegrationAllowedCIDRs(r.Context(), s.pool)
		if len(extra) > 0 {
			if policy.Client.Mode == edge.ModeAllowlist {
				policy.Client.CIDRs = edge.MergeCIDRs(policy.Client.CIDRs, extra)
			}
			if policy.Auth.Mode == edge.ModeAllowlist {
				policy.Auth.CIDRs = edge.MergeCIDRs(policy.Auth.CIDRs, extra)
			}
		}
	}
	roller := s.edgeRoller
	if roller == nil {
		roller = &edge.MemoryRoller{}
	}
	if err := roller.Apply(r.Context(), policy); err != nil {
		msg := err.Error()
		_ = store.MarkStatus(r.Context(), edge.StatusError, &msg)
		return store.Get(r.Context())
	}
	if err := store.MarkStatus(r.Context(), edge.StatusApplied, nil); err != nil {
		return nil, err
	}
	return store.Get(r.Context())
}
