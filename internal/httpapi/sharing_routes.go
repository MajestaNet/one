package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
)

const maxSharingRulesPerObject = 50

func (s *Server) registerSharingRoutes(prefix string) {
	meta := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeMetadata, s.requireCapability(authz.CapAuthzManage, h)))
	}
	s.mux.Handle("GET "+prefix+"/sharing/settings", meta(s.handleGetSharingSettings))
	s.mux.Handle("POST "+prefix+"/sharing/enable", meta(s.handleEnableSharing))
	s.mux.Handle("GET "+prefix+"/sharing/objects", meta(s.handleListSharingObjects))
	s.mux.Handle("GET "+prefix+"/sharing/objects/{apiName}", meta(s.handleGetSharingObject))
	s.mux.Handle("PATCH "+prefix+"/sharing/objects/{apiName}", meta(s.handlePatchSharingObject))
	s.mux.Handle("GET "+prefix+"/sharing/objects/{apiName}/rules", meta(s.handleListSharingRules))
	s.mux.Handle("POST "+prefix+"/sharing/objects/{apiName}/rules", meta(s.handleCreateSharingRule))
	s.mux.Handle("GET "+prefix+"/sharing/objects/{apiName}/rules/{ruleApiName}", meta(s.handleGetSharingRule))
	s.mux.Handle("PATCH "+prefix+"/sharing/objects/{apiName}/rules/{ruleApiName}", meta(s.handlePatchSharingRule))
	s.mux.Handle("DELETE "+prefix+"/sharing/objects/{apiName}/rules/{ruleApiName}", meta(s.handleDeleteSharingRule))
}

func (s *Server) sharingStore() *db.SharingStore {
	if s.pool == nil {
		return nil
	}
	return db.NewSharingStore(s.pool)
}

func (s *Server) handleGetSharingSettings(w http.ResponseWriter, r *http.Request) {
	store := s.sharingStore()
	if store == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	settings, err := store.GetOrganizationSettings(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	out := map[string]any{"recordSharingEnabled": settings.RecordSharingEnabled}
	if settings.RecordSharingEnabledAt != nil {
		out["recordSharingEnabledAt"] = settings.RecordSharingEnabledAt
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleEnableSharing(w http.ResponseWriter, r *http.Request) {
	store := s.sharingStore()
	if store == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	var body struct {
		Confirm bool `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !body.Confirm {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "confirm: true is required")
		return
	}
	current, err := store.GetOrganizationSettings(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if current.RecordSharingEnabled {
		writeErr(w, http.StatusConflict, "CONFLICT", "Record sharing is already enabled")
		return
	}
	settings, err := store.EnableRecordSharing(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	_ = db.EnqueueSharingRecalc(r.Context(), s.pool, map[string]any{"scope": "full"})
	out := map[string]any{"recordSharingEnabled": settings.RecordSharingEnabled}
	if settings.RecordSharingEnabledAt != nil {
		out["recordSharingEnabledAt"] = settings.RecordSharingEnabledAt
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleListSharingObjects(w http.ResponseWriter, r *http.Request) {
	store := s.sharingStore()
	if store == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	list, err := store.ListObjectSharingSettings(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	objects := make([]map[string]any, 0, len(list))
	for _, o := range list {
		objects = append(objects, objectSharingJSON(o))
	}
	writeJSON(w, http.StatusOK, map[string]any{"objects": objects})
}

func (s *Server) handleGetSharingObject(w http.ResponseWriter, r *http.Request) {
	store := s.sharingStore()
	if store == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	apiName := r.PathValue("apiName")
	o, err := store.GetObjectSharingSettings(r.Context(), apiName)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, objectSharingJSON(*o))
}

func (s *Server) handlePatchSharingObject(w http.ResponseWriter, r *http.Request) {
	store := s.sharingStore()
	if store == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	org, err := store.GetOrganizationSettings(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if !org.RecordSharingEnabled {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Enable record sharing before configuring objects")
		return
	}
	apiName := r.PathValue("apiName")
	var body struct {
		DefaultAccess       string `json:"defaultAccess"`
		SharingRulesEnabled *bool  `json:"sharingRulesEnabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	if body.DefaultAccess != "" {
		if err := authz.ValidateOWDDefaultAccess(body.DefaultAccess); err != nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
	}
	if body.SharingRulesEnabled != nil && !*body.SharingRulesEnabled {
		n, err := store.CountActiveSharingRules(r.Context(), apiName)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		if n > 0 {
			writeErr(w, http.StatusConflict, "CONFLICT", "Delete active sharing rules before disabling sharingRulesEnabled")
			return
		}
	}
	o, err := store.UpdateObjectSharingSettings(r.Context(), apiName, body.DefaultAccess, body.SharingRulesEnabled)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	_ = db.EnqueueSharingRecalc(r.Context(), s.pool, map[string]any{"scope": "object", "objectApiName": apiName})
	writeJSON(w, http.StatusOK, objectSharingJSON(*o))
}

func objectSharingJSON(o db.ObjectSharingSettings) map[string]any {
	return map[string]any{
		"objectApiName":       o.ObjectAPIName,
		"defaultAccess":       o.DefaultAccess,
		"sharingRulesEnabled": o.SharingRulesEnabled,
		"updatedAt":           o.UpdatedAt,
	}
}

func (s *Server) handleListSharingRules(w http.ResponseWriter, r *http.Request) {
	store := s.sharingStore()
	if store == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	apiName := r.PathValue("apiName")
	rules, err := store.ListSharingRules(r.Context(), apiName)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rules))
	for _, rule := range rules {
		out = append(out, sharingRuleJSON(r.Context(), rule, s.pool))
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": out})
}

func (s *Server) handleGetSharingRule(w http.ResponseWriter, r *http.Request) {
	store := s.sharingStore()
	if store == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	rule, err := store.GetSharingRule(r.Context(), r.PathValue("apiName"), r.PathValue("ruleApiName"))
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sharingRuleJSON(r.Context(), *rule, s.pool))
}

func (s *Server) handleCreateSharingRule(w http.ResponseWriter, r *http.Request) {
	store := s.sharingStore()
	if store == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	org, err := store.GetOrganizationSettings(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if !org.RecordSharingEnabled {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Enable record sharing before creating rules")
		return
	}
	objectAPIName := r.PathValue("apiName")
	objSettings, err := store.GetObjectSharingSettings(r.Context(), objectAPIName)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if !objSettings.SharingRulesEnabled {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "sharingRulesEnabled must be true on the object")
		return
	}
	existing, _ := store.ListSharingRules(r.Context(), objectAPIName)
	if len(existing) >= maxSharingRulesPerObject {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Maximum sharing rules reached for object")
		return
	}
	var body struct {
		APIName                 string          `json:"apiName"`
		Label                   string          `json:"label"`
		Active                  bool            `json:"active"`
		AccessLevel             string          `json:"accessLevel"`
		SharedToDataRoleAPIName string          `json:"sharedToDataRoleApiName"`
		Criteria                json.RawMessage `json:"criteria"`
		SortOrder               int             `json:"sortOrder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	body.APIName = strings.TrimSpace(body.APIName)
	if body.APIName == "" || body.Label == "" || body.SharedToDataRoleAPIName == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "apiName, label, and sharedToDataRoleApiName are required")
		return
	}
	if body.AccessLevel == "" {
		body.AccessLevel = "read"
	}
	if err := authz.ValidateSharingAccessLevel(body.AccessLevel); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	if len(body.Criteria) == 0 {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "criteria.filters must contain at least one filter")
		return
	}
	roleStore := db.NewDataRoleStore(s.pool)
	role, err := roleStore.GetDataRoleByAPIName(r.Context(), body.SharedToDataRoleAPIName)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	rule, err := store.CreateSharingRule(r.Context(), db.SharingRule{
		ObjectAPIName:      objectAPIName,
		APIName:            body.APIName,
		Label:              body.Label,
		Active:             body.Active,
		AccessLevel:        body.AccessLevel,
		SharedToDataRoleID: role.ID,
		Criteria:           body.Criteria,
		SortOrder:          body.SortOrder,
	})
	if err != nil {
		writeAPIError(w, err)
		return
	}
	_ = db.EnqueueSharingRecalc(r.Context(), s.pool, map[string]any{"scope": "rule", "ruleId": rule.ID, "objectApiName": objectAPIName})
	writeJSON(w, http.StatusCreated, sharingRuleJSON(r.Context(), *rule, s.pool))
}

func (s *Server) handlePatchSharingRule(w http.ResponseWriter, r *http.Request) {
	store := s.sharingStore()
	if store == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	objectAPIName := r.PathValue("apiName")
	ruleAPIName := r.PathValue("ruleApiName")
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	if v, ok := raw["sharedToDataRoleApiName"].(string); ok && v != "" {
		role, err := db.NewDataRoleStore(s.pool).GetDataRoleByAPIName(r.Context(), v)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		raw["sharedToDataRoleId"] = role.ID
		delete(raw, "sharedToDataRoleApiName")
	}
	rule, err := store.UpdateSharingRule(r.Context(), objectAPIName, ruleAPIName, raw)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	// Fail closed: drop stale grants immediately; worker rebuilds asynchronously.
	_ = store.DeleteRuleGrants(r.Context(), rule.ID)
	_ = db.EnqueueSharingRecalc(r.Context(), s.pool, map[string]any{"scope": "rule", "ruleId": rule.ID, "objectApiName": objectAPIName})
	writeJSON(w, http.StatusOK, sharingRuleJSON(r.Context(), *rule, s.pool))
}

func (s *Server) handleDeleteSharingRule(w http.ResponseWriter, r *http.Request) {
	store := s.sharingStore()
	if store == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return
	}
	objectAPIName := r.PathValue("apiName")
	ruleAPIName := r.PathValue("ruleApiName")
	rule, err := store.GetSharingRule(r.Context(), objectAPIName, ruleAPIName)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if err := store.DeleteSharingRule(r.Context(), objectAPIName, ruleAPIName); err != nil {
		writeAPIError(w, err)
		return
	}
	_ = store.DeleteRuleGrants(r.Context(), rule.ID)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "apiName": ruleAPIName})
}

func sharingRuleJSON(ctx context.Context, rule db.SharingRule, pool *db.Pool) map[string]any {
	out := map[string]any{
		"id":          rule.ID,
		"apiName":     rule.APIName,
		"label":       rule.Label,
		"active":      rule.Active,
		"accessLevel": rule.AccessLevel,
		"sortOrder":   rule.SortOrder,
		"createdAt":   rule.CreatedAt,
		"updatedAt":   rule.UpdatedAt,
	}
	var criteria any
	_ = json.Unmarshal(rule.Criteria, &criteria)
	out["criteria"] = criteria
	if pool != nil {
		if role, err := db.NewDataRoleStore(pool).GetDataRoleByID(ctx, rule.SharedToDataRoleID); err == nil {
			out["sharedToDataRoleApiName"] = role.APIName
		}
	}
	return out
}
