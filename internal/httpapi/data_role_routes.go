package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
)

func (s *Server) registerDataRoleRoutes(prefix string) {
	capClient := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeClient, s.requireCapability(authz.CapAuthzManage, h)))
	}
	s.mux.Handle("GET "+prefix+"/data-roles", capClient(s.handleListDataRoles))
	s.mux.Handle("POST "+prefix+"/data-roles", capClient(s.handleCreateDataRole))
	s.mux.Handle("GET "+prefix+"/data-roles/{apiName}", capClient(s.handleGetDataRole))
	s.mux.Handle("PATCH "+prefix+"/data-roles/{apiName}", capClient(s.handlePatchDataRole))
	s.mux.Handle("DELETE "+prefix+"/data-roles/{apiName}", capClient(s.handleDeleteDataRole))
}

func (s *Server) dataRoleStore() *db.DataRoleStore {
	if s.pool == nil {
		return nil
	}
	return db.NewDataRoleStore(s.pool)
}

func dataRoleJSON(r db.DataRole) map[string]any {
	out := map[string]any{
		"id":       r.ID,
		"apiName":  r.APIName,
		"label":    r.Label,
		"isSystem": r.IsSystem,
	}
	if r.ParentDataRoleID != nil {
		out["parentDataRoleId"] = *r.ParentDataRoleID
	}
	return out
}

func (s *Server) handleListDataRoles(w http.ResponseWriter, r *http.Request) {
	store := s.dataRoleStore()
	if store == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	roles, err := store.ListDataRoles(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(roles))
	for _, role := range roles {
		j := dataRoleJSON(role)
		if role.ParentDataRoleID != nil {
			if parent, err := store.GetDataRoleByID(r.Context(), *role.ParentDataRoleID); err == nil {
				j["parentDataRoleApiName"] = parent.APIName
			}
		}
		out = append(out, j)
	}
	writeJSON(w, http.StatusOK, map[string]any{"dataRoles": out})
}

func (s *Server) handleGetDataRole(w http.ResponseWriter, r *http.Request) {
	store := s.dataRoleStore()
	if store == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	role, err := store.GetDataRoleByAPIName(r.Context(), r.PathValue("apiName"))
	if err != nil {
		writeAPIError(w, err)
		return
	}
	j := dataRoleJSON(*role)
	if role.ParentDataRoleID != nil {
		if parent, err := store.GetDataRoleByID(r.Context(), *role.ParentDataRoleID); err == nil {
			j["parentDataRoleApiName"] = parent.APIName
		}
	}
	writeJSON(w, http.StatusOK, j)
}

func (s *Server) handleCreateDataRole(w http.ResponseWriter, r *http.Request) {
	store := s.dataRoleStore()
	if store == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	var body struct {
		APIName               string `json:"apiName"`
		Label                 string `json:"label"`
		ParentDataRoleAPIName string `json:"parentDataRoleApiName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	var parentID *string
	if body.ParentDataRoleAPIName != "" {
		parent, err := store.GetDataRoleByAPIName(r.Context(), body.ParentDataRoleAPIName)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		parentID = &parent.ID
	}
	role, err := store.CreateDataRole(r.Context(), body.APIName, body.Label, parentID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dataRoleJSON(*role))
}

func (s *Server) handlePatchDataRole(w http.ResponseWriter, r *http.Request) {
	store := s.dataRoleStore()
	if store == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	var body struct {
		Label                 string  `json:"label"`
		ParentDataRoleAPIName *string `json:"parentDataRoleApiName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	apiName := r.PathValue("apiName")
	var parentID *string
	clearParent := false
	if body.ParentDataRoleAPIName != nil {
		if strings.TrimSpace(*body.ParentDataRoleAPIName) == "" {
			clearParent = true
		} else {
			parent, err := store.GetDataRoleByAPIName(r.Context(), *body.ParentDataRoleAPIName)
			if err != nil {
				writeAPIError(w, err)
				return
			}
			parentID = &parent.ID
		}
	}
	role, err := store.UpdateDataRole(r.Context(), apiName, body.Label, parentID, clearParent)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	_ = db.EnqueueSharingRecalc(r.Context(), s.pool, map[string]any{"scope": "hierarchy"})
	writeJSON(w, http.StatusOK, dataRoleJSON(*role))
}

func (s *Server) handleDeleteDataRole(w http.ResponseWriter, r *http.Request) {
	store := s.dataRoleStore()
	if store == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	if err := store.DeleteDataRole(r.Context(), r.PathValue("apiName")); err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}
