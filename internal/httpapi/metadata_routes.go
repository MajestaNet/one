package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MajestaNet/ide/internal/agentharness"
	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/automation"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/seed"
	"github.com/MajestaNet/ide/internal/webhook"
)

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := ActorFromContext(r.Context())
		if actor == nil || !authz.HasAdminPrivilege(actor) {
			writeErr(w, http.StatusForbidden, "FORBIDDEN", "Admin required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireCapability(cap string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := ActorFromContext(r.Context())
		if actor == nil {
			writeErr(w, http.StatusForbidden, "CAPABILITY_REQUIRED", "capability "+cap+" required")
			return
		}
		if authz.HasAdminPrivilege(actor) {
			next.ServeHTTP(w, r)
			return
		}
		if s.systemAz == nil {
			writeErr(w, http.StatusForbidden, "CAPABILITY_REQUIRED", "capability "+cap+" required")
			return
		}
		if err := s.systemAz.AssertCapability(r.Context(), actor, cap); err != nil {
			writeErr(w, http.StatusForbidden, "CAPABILITY_REQUIRED", "capability "+cap+" required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) registerMetadataWrites(prefix string) {
	meta := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeMetadata, h))
	}
	capMeta := func(cap string, h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeMetadata, s.requireCapability(cap, h)))
	}
	s.mux.Handle("GET "+prefix+"/objects/{apiName}", meta(s.handleGetObject))
	s.mux.Handle("POST "+prefix+"/objects", capMeta(authz.CapMetadataBuild, s.handleCreateObject))
	s.mux.Handle("PATCH "+prefix+"/objects/{apiName}", capMeta(authz.CapMetadataBuild, s.handlePatchObject))
	s.mux.Handle("DELETE "+prefix+"/objects/{apiName}", capMeta(authz.CapMetadataBuild, s.handleDeleteObject))
	s.mux.Handle("POST "+prefix+"/fields", capMeta(authz.CapMetadataBuild, s.handleCreateField))
	s.mux.Handle("GET "+prefix+"/field-types", meta(s.handleListFieldTypes))
	s.mux.Handle("PATCH "+prefix+"/fields/{object}/{apiName}", capMeta(authz.CapMetadataBuild, s.handlePatchField))
	s.mux.Handle("DELETE "+prefix+"/fields/{object}/{apiName}", capMeta(authz.CapMetadataBuild, s.handleDeleteField))
	s.mux.Handle("POST "+prefix+"/validation-rules", capMeta(authz.CapMetadataBuild, s.handleCreateValidationRule))
	s.mux.Handle("GET "+prefix+"/snapshot", meta(s.handleSnapshot))
	s.mux.Handle("GET "+prefix+"/automations", meta(s.handleListAutomations))
	s.mux.Handle("POST "+prefix+"/automations", capMeta(authz.CapMetadataBuild, s.handleCreateAutomation))
	s.mux.Handle("PATCH "+prefix+"/automations/{apiName}", capMeta(authz.CapMetadataBuild, s.handlePatchAutomation))
	s.mux.Handle("GET "+prefix+"/permissions/sets", meta(s.handleListPermissionSets))
	s.mux.Handle("GET "+prefix+"/permissions/sets/{apiName}", meta(s.handleGetPermissionSet))
	s.mux.Handle("POST "+prefix+"/permissions/sets", capMeta(authz.CapAuthzManage, s.handleCreatePermissionSet))
	s.mux.Handle("PATCH "+prefix+"/permissions/sets/{apiName}", capMeta(authz.CapAuthzManage, s.handlePatchPermissionSet))
	s.registerExposureRoutes(prefix)
	s.registerInstallAuthRoutes(prefix)
	s.mux.Handle("GET "+prefix+"/packages", meta(s.handleListPackages))
	s.mux.Handle("GET "+prefix+"/packages/{name}", meta(s.handleGetPackage))
	s.mux.Handle("POST "+prefix+"/packages/{name}/enable", capMeta(authz.CapMetadataBuild, s.handleEnablePackage))
	s.mux.Handle("POST "+prefix+"/packages/{name}/disable", capMeta(authz.CapMetadataBuild, s.handleDisablePackage))
	s.mux.Handle("GET "+prefix+"/webhooks", meta(s.handleListWebhooks))
	s.mux.Handle("POST "+prefix+"/webhooks", capMeta(authz.CapMetadataBuild, s.handleCreateWebhook))
	s.registerOutboundRoutes(prefix)
	s.registerInferenceRoutes(prefix)
	s.mux.Handle("GET "+prefix+"/agents/harnesses", meta(s.handleListAgentHarnesses))
	s.mux.Handle("GET "+prefix+"/agents/playbooks", meta(s.handleListPlaybooks))
	s.mux.Handle("GET "+prefix+"/agents/playbooks/{apiName}", meta(s.handleGetPlaybook))
	s.mux.Handle("POST "+prefix+"/agents/playbooks", capMeta(authz.CapMetadataBuild, s.handleCreatePlaybook))
	s.mux.Handle("PATCH "+prefix+"/agents/playbooks/{apiName}", capMeta(authz.CapMetadataBuild, s.handlePatchPlaybook))
	s.mux.Handle("DELETE "+prefix+"/agents/playbooks/{apiName}", capMeta(authz.CapMetadataBuild, s.handleDeletePlaybook))
	s.registerCanvasSpecRoutes(prefix)
	s.registerExperienceRoutes(prefix)
	s.mux.Handle("POST "+prefix+"/projections/{object}/build", capMeta(authz.CapMetadataBuild, s.handleBuildProjections))
	s.mux.Handle("GET "+prefix+"/projections/{object}", meta(s.handleListProjections))
}

// registerMetadataCoreAliases registers Node-compatible nested /metadata/* aliases for core resources.
func (s *Server) registerMetadataCoreAliases(prefix string) {
	meta := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeMetadata, h))
	}
	capMeta := func(cap string, h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeMetadata, s.requireCapability(cap, h)))
	}
	s.mux.Handle("GET "+prefix+"/objects", meta(s.handleListObjects))
	s.mux.Handle("GET "+prefix+"/objects/{apiName}", meta(s.handleGetObject))
	s.mux.Handle("POST "+prefix+"/objects", capMeta(authz.CapMetadataBuild, s.handleCreateObject))
	s.mux.Handle("PATCH "+prefix+"/objects/{apiName}", capMeta(authz.CapMetadataBuild, s.handlePatchObject))
	s.mux.Handle("DELETE "+prefix+"/objects/{apiName}", capMeta(authz.CapMetadataBuild, s.handleDeleteObject))
	s.mux.Handle("POST "+prefix+"/fields", capMeta(authz.CapMetadataBuild, s.handleCreateField))
	s.mux.Handle("GET "+prefix+"/field-types", meta(s.handleListFieldTypes))
	s.mux.Handle("PATCH "+prefix+"/fields/{object}/{apiName}", capMeta(authz.CapMetadataBuild, s.handlePatchField))
	s.mux.Handle("DELETE "+prefix+"/fields/{object}/{apiName}", capMeta(authz.CapMetadataBuild, s.handleDeleteField))
	s.mux.Handle("POST "+prefix+"/validation-rules", capMeta(authz.CapMetadataBuild, s.handleCreateValidationRule))
	s.mux.Handle("GET "+prefix+"/snapshot", meta(s.handleSnapshot))
}

func (s *Server) registerClientExtras(prefix string) {
	client := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeClient, h))
	}
	adminClient := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeClient, s.requireAdmin(h)))
	}
	capClient := func(cap string, h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeClient, s.requireCapability(cap, h)))
	}
	// Principal working sets are Client-family only — do not alias onto flat /v1
	// (metadata CanvasSpec already owns GET /v1/canvases on the compatibility prefix).
	if prefix == "/client/v1" {
		s.registerCanvasDocumentRoutes(prefix)
		s.registerAgentConversationRoutes(prefix)
		s.registerRunGraphRoutes(prefix)
	}
	s.mux.Handle("GET "+prefix+"/events", client(s.handleListEvents))
	s.mux.Handle("GET "+prefix+"/events/unpublished", client(s.handleListUnpublishedEvents))
	s.mux.Handle("PATCH "+prefix+"/events/{id}/ack", client(s.handleAckEvent))
	s.mux.Handle("GET "+prefix+"/activity-feed", client(s.handleActivityFeed))
	s.mux.Handle("POST "+prefix+"/agents/runs", client(s.handleCreateAgentRun))
	s.mux.Handle("GET "+prefix+"/agents/runs/{id}", client(s.handleGetAgentRun))
	s.mux.Handle("GET "+prefix+"/agents/runs/{id}/stream", client(s.handleStreamAgentRun))
	s.mux.Handle("POST "+prefix+"/agents/runs/{id}/approve", capClient(authz.CapGovernAgents, s.handleApproveAgentRun))
	// Client-only prefix: flat /v1/automations is Metadata definitions (ADR-004).
	if prefix == "/client/v1" {
		s.mux.Handle("GET "+prefix+"/agents/playbooks", client(s.handleListClientPlaybooks))
		s.mux.Handle("GET "+prefix+"/automations", client(s.handleListCallableAutomations))
		s.mux.Handle("GET "+prefix+"/automations/runs/{id}", client(s.handleGetAutomationRun))
		s.mux.Handle("POST "+prefix+"/automations/{apiName}/runs", client(s.handleCreateAutomationRun))
		s.mux.Handle("GET "+prefix+"/tools", client(s.handleListClientTools))
		s.mux.Handle("GET "+prefix+"/tools/{apiName}", client(s.handleGetClientTool))
		s.mux.Handle("GET "+prefix+"/actions", client(s.handleListActions))
		s.mux.Handle("GET "+prefix+"/actions/{apiName}", client(s.handleGetAction))
		s.mux.Handle("POST "+prefix+"/actions/{apiName}", client(s.handleInvokeAction))
	}
	s.mux.Handle("GET "+prefix+"/audit", adminClient(s.handleListAudit))
	s.registerPrincipalAdminRoutes(prefix)
	s.registerIntegrationRoutes(prefix)
}

func (s *Server) poolOrErr(w http.ResponseWriter) *db.Pool {
	if s.pool == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return nil
	}
	return s.pool
}

func (s *Server) handleGetObject(w http.ResponseWriter, r *http.Request) {
	if s.meta == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "metadata service not configured")
		return
	}
	apiName := r.PathValue("apiName")
	desc, err := s.meta.Describe(r.Context(), apiName)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, desc)
}

func (s *Server) handleCreateObject(w http.ResponseWriter, r *http.Request) {
	if s.meta == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "metadata service not configured")
		return
	}
	var body metadata.ObjectDefinition
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	if body.PluralLabel == "" {
		body.PluralLabel = body.Label + "s"
	}
	obj, err := s.meta.InsertObject(r.Context(), body, metadata.CreateOptions{})
	if err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "metadata.object.create", obj.APIName, nil, map[string]any{"apiName": obj.APIName})
	writeJSON(w, http.StatusCreated, obj)
}

func (s *Server) handlePatchObject(w http.ResponseWriter, r *http.Request) {
	if s.meta == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "metadata service not configured")
		return
	}
	apiName := r.PathValue("apiName")
	var body metadata.ObjectDefinition
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	obj, err := s.meta.UpdateObject(r.Context(), apiName, body)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "metadata.object.update", obj.APIName, nil, map[string]any{"apiName": obj.APIName})
	writeJSON(w, http.StatusOK, obj)
}

func (s *Server) handleDeleteObject(w http.ResponseWriter, r *http.Request) {
	if s.meta == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "metadata service not configured")
		return
	}
	apiName := r.PathValue("apiName")
	if err := s.meta.DeleteObject(r.Context(), apiName); err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "metadata.object.delete", apiName, nil, map[string]any{"apiName": apiName})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "apiName": apiName})
}

func (s *Server) handleListFieldTypes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"fieldTypes": metadata.ListFieldTypes(),
	})
}

func (s *Server) handleCreateField(w http.ResponseWriter, r *http.Request) {
	if s.meta == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "metadata service not configured")
		return
	}
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	body := metadata.FieldDefinition{}
	b, _ := json.Marshal(raw)
	_ = json.Unmarshal(b, &body)
	_, filterableSet := raw["filterable"]
	_, sortableSet := raw["sortable"]
	_, indexedSet := raw["indexed"]
	metadata.ApplyFieldTypeDefaults(&body, filterableSet, sortableSet, indexedSet)
	field, err := s.meta.InsertField(r.Context(), body, metadata.CreateOptions{})
	if err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "metadata.field.create", field.ObjectAPIName, nil, map[string]any{"apiName": field.APIName})
	if field.Indexed {
		s.enqueueProjectionBuild(r.Context(), field.ObjectAPIName)
	}
	writeJSON(w, http.StatusCreated, field)
}

func (s *Server) handlePatchField(w http.ResponseWriter, r *http.Request) {
	if s.meta == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "metadata service not configured")
		return
	}
	object := r.PathValue("object")
	apiName := r.PathValue("apiName")
	var patch metadata.FieldPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	field, err := s.meta.UpdateField(r.Context(), object, apiName, patch)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "metadata.field.update", field.ObjectAPIName, nil, map[string]any{"apiName": field.APIName})
	writeJSON(w, http.StatusOK, field)
}

func (s *Server) handleDeleteField(w http.ResponseWriter, r *http.Request) {
	if s.meta == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "metadata service not configured")
		return
	}
	object := r.PathValue("object")
	apiName := r.PathValue("apiName")
	if err := s.meta.DeleteField(r.Context(), object, apiName); err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "metadata.field.delete", object, nil, map[string]any{"apiName": apiName})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "objectApiName": object, "apiName": apiName})
}

func (s *Server) handleCreateValidationRule(w http.ResponseWriter, r *http.Request) {
	if s.meta == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "metadata service not configured")
		return
	}
	var body struct {
		ObjectAPIName string          `json:"objectApiName"`
		APIName       string          `json:"apiName"`
		Label         string          `json:"label"`
		Active        *bool           `json:"active"`
		ErrorMessage  string          `json:"errorMessage"`
		Expression    json.RawMessage `json:"expression"`
		PackageName   *string         `json:"packageName"`
		Ownership     string          `json:"ownership"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	active := true
	if body.Active != nil {
		active = *body.Active
	}
	rule, err := s.meta.InsertValidationRule(r.Context(), body.ObjectAPIName, metadata.ValidationRuleDefinition{
		APIName: body.APIName, Label: body.Label, Active: active,
		ErrorMessage: body.ErrorMessage, Expression: body.Expression,
		PackageName: body.PackageName, Ownership: body.Ownership,
	}, metadata.CreateOptions{})
	if err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "metadata.validation.create", body.ObjectAPIName, nil, map[string]any{"apiName": body.APIName})
	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if s.meta == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "metadata service not configured")
		return
	}
	snap, err := s.meta.ExportCustomerSnapshot(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleListAutomations(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	rows, err := pool.Query(r.Context(), `
SELECT id::text, api_name, label, object_api_name, trigger_event, active, condition, actions,
       package_name, ownership, created_at, updated_at,
       runtime, execution, entry_file, source, run_as_principal_id::text
FROM metadata_automations ORDER BY api_name`)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		var id, apiName, label, obj, trigger, ownership string
		var active bool
		var condition, actions []byte
		var pkg *string
		var created, updated time.Time
		var runtime, execution string
		var entryFile, source, runAs *string
		if err := rows.Scan(
			&id, &apiName, &label, &obj, &trigger, &active, &condition, &actions,
			&pkg, &ownership, &created, &updated,
			&runtime, &execution, &entryFile, &source, &runAs,
		); err != nil {
			writeAPIError(w, err)
			return
		}
		m := map[string]any{
			"id": id, "apiName": apiName, "label": label, "objectApiName": obj,
			"triggerEvent": trigger, "active": active, "ownership": ownership,
			"runtime": runtime, "execution": execution,
			"createdAt": created, "updatedAt": updated,
		}
		if pkg != nil {
			m["packageName"] = *pkg
		}
		if entryFile != nil {
			m["entryFile"] = *entryFile
		}
		if source != nil {
			m["source"] = *source
		}
		if runAs != nil {
			m["runAsPrincipalId"] = *runAs
		}
		var cond any
		_ = json.Unmarshal(condition, &cond)
		m["condition"] = cond
		var acts any
		_ = json.Unmarshal(actions, &acts)
		m["actions"] = acts
		list = append(list, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"automations": list})
}

func (s *Server) handleCreateAutomation(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	apiName, _ := body["apiName"].(string)
	label, _ := body["label"].(string)
	obj, _ := body["objectApiName"].(string)
	if apiName == "" || label == "" || obj == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "apiName, label, objectApiName required")
		return
	}
	if own, _ := body["ownership"].(string); own == "managed" {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "Cannot create managed automation")
		return
	}
	trigger, _ := body["triggerEvent"].(string)
	if trigger == "" {
		trigger = "create"
	}
	active := true
	if v, ok := body["active"].(bool); ok {
		active = v
	}
	pkg := metadata.DefaultCustomerPackage
	if p, ok := body["packageName"].(string); ok && p != "" {
		pkg = p
	}
	runtime, _ := body["runtime"].(string)
	execution, _ := body["execution"].(string)
	var entryFile, source, runAs *string
	if v, ok := body["entryFile"].(string); ok && v != "" {
		entryFile = &v
	}
	if v, ok := body["source"].(string); ok {
		source = &v
	}
	if v, ok := body["runAsPrincipalId"].(string); ok && v != "" {
		runAs = &v
	}
	var actionsList []any
	if raw, ok := body["actions"].([]any); ok {
		actionsList = raw
	}
	if err := automation.ValidateDefinition(apiName, runtime, execution, trigger, entryFile, source, runAs, actionsList); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	runtime = automation.NormalizeRuntime(runtime)
	execution = automation.NormalizeExecution(execution)
	cond, _ := json.Marshal(body["condition"])
	acts, _ := json.Marshal(body["actions"])
	if string(acts) == "null" {
		acts = []byte("[]")
	}
	var entryVal, sourceVal, runAsVal any
	if entryFile != nil {
		entryVal = *entryFile
	}
	if source != nil {
		sourceVal = *source
	}
	if runAs != nil {
		runAsVal = *runAs
	}
	var id string
	var created, updated time.Time
	err := pool.QueryRow(r.Context(), `
INSERT INTO metadata_automations (
  api_name, label, object_api_name, trigger_event, active, condition, actions, package_name, ownership,
  runtime, execution, entry_file, source, run_as_principal_id
)
VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,$8,'custom',$9,$10,$11,$12,$13)
RETURNING id::text, created_at, updated_at`,
		apiName, label, obj, trigger, active, string(cond), string(acts), pkg,
		runtime, execution, entryVal, sourceVal, runAsVal,
	).Scan(&id, &created, &updated)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if err := db.EnsureAutomationInAccessCatalog(r.Context(), pool, apiName); err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, httptestGet(pool, r, apiName))
}

func (s *Server) handlePatchAutomation(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := r.PathValue("apiName")
	var ownership string
	err := pool.QueryRow(r.Context(), `SELECT ownership FROM metadata_automations WHERE api_name=$1`, apiName).Scan(&ownership)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "Automation not found: "+apiName)
		return
	}
	if err := metadata.AssertCustomerMutable(ownership, apiName, "automation"); err != nil {
		writeAPIError(w, err)
		return
	}
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	sets := []string{"updated_at=now()"}
	args := []any{apiName}
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, col+"=$"+strconv.Itoa(len(args)))
	}
	if v, ok := body["label"].(string); ok {
		add("label", v)
	}
	if v, ok := body["active"].(bool); ok {
		add("active", v)
	}
	if v, ok := body["condition"]; ok {
		b, _ := json.Marshal(v)
		add("condition", string(b))
		sets[len(sets)-1] = "condition=$" + strconv.Itoa(len(args)) + "::jsonb"
	}
	if v, ok := body["actions"]; ok {
		b, _ := json.Marshal(v)
		add("actions", string(b))
		sets[len(sets)-1] = "actions=$" + strconv.Itoa(len(args)) + "::jsonb"
	}
	if v, ok := body["runtime"].(string); ok {
		add("runtime", automation.NormalizeRuntime(v))
	}
	if v, ok := body["execution"].(string); ok {
		add("execution", automation.NormalizeExecution(v))
	}
	if v, ok := body["entryFile"].(string); ok {
		add("entry_file", v)
	}
	if v, ok := body["source"].(string); ok {
		add("source", v)
	}
	if v, ok := body["runAsPrincipalId"].(string); ok {
		if v == "" {
			add("run_as_principal_id", nil)
		} else {
			add("run_as_principal_id", v)
		}
	}
	// Load merged definition for validation before update.
	cur := httptestGet(pool, r, apiName)
	mergeStr := func(key string) *string {
		if v, ok := body[key].(string); ok {
			if v == "" {
				return nil
			}
			return &v
		}
		if v, ok := cur[key].(string); ok && v != "" {
			return &v
		}
		return nil
	}
	runtime, _ := cur["runtime"].(string)
	if v, ok := body["runtime"].(string); ok {
		runtime = v
	}
	execution, _ := cur["execution"].(string)
	if v, ok := body["execution"].(string); ok {
		execution = v
	}
	trigger, _ := cur["triggerEvent"].(string)
	var actionsList []any
	if v, ok := body["actions"].([]any); ok {
		actionsList = v
	} else if v, ok := cur["actions"].([]any); ok {
		actionsList = v
	}
	if err := automation.ValidateDefinition(
		apiName, runtime, execution, trigger,
		mergeStr("entryFile"), mergeStr("source"), mergeStr("runAsPrincipalId"), actionsList,
	); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	_, err = pool.Exec(r.Context(), `UPDATE metadata_automations SET `+strings.Join(sets, ",")+` WHERE api_name=$1`, args...)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, httptestGet(pool, r, apiName))
}

func httptestGet(pool *db.Pool, r *http.Request, apiName string) map[string]any {
	var id, label, obj, trigger, ownership string
	var active bool
	var condition, actions []byte
	var pkg *string
	var created, updated time.Time
	var runtime, execution string
	var entryFile, source, runAs *string
	_ = pool.QueryRow(r.Context(), `
SELECT id::text, label, object_api_name, trigger_event, active, condition, actions, package_name, ownership, created_at, updated_at,
       runtime, execution, entry_file, source, run_as_principal_id::text
FROM metadata_automations WHERE api_name=$1`, apiName).Scan(
		&id, &label, &obj, &trigger, &active, &condition, &actions, &pkg, &ownership, &created, &updated,
		&runtime, &execution, &entryFile, &source, &runAs)
	m := map[string]any{
		"id": id, "apiName": apiName, "label": label, "objectApiName": obj,
		"triggerEvent": trigger, "active": active, "ownership": ownership,
		"runtime": runtime, "execution": execution,
		"createdAt": created, "updatedAt": updated,
	}
	if pkg != nil {
		m["packageName"] = *pkg
	}
	if entryFile != nil {
		m["entryFile"] = *entryFile
	}
	if source != nil {
		m["source"] = *source
	}
	if runAs != nil {
		m["runAsPrincipalId"] = *runAs
	}
	var cond, acts any
	_ = json.Unmarshal(condition, &cond)
	_ = json.Unmarshal(actions, &acts)
	m["condition"] = cond
	m["actions"] = acts
	return m
}

func (s *Server) handleListPermissionSets(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	includeDataAccess := strings.Contains(strings.ToLower(r.URL.Query().Get("include")), "dataaccess")
	includeAutomationAccess := includeDataAccess ||
		strings.Contains(strings.ToLower(r.URL.Query().Get("include")), "automationaccess")
	includeToolAccess := includeDataAccess ||
		strings.Contains(strings.ToLower(r.URL.Query().Get("include")), "toolaccess")
	rows, err := pool.Query(r.Context(), `
SELECT id::text, api_name, label, description, is_system, COALESCE(system_permissions, '[]'::jsonb), created_at
FROM permission_sets ORDER BY api_name`)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		var id, apiName, label string
		var desc *string
		var isSystem bool
		var sysPermsRaw []byte
		var created time.Time
		if err := rows.Scan(&id, &apiName, &label, &desc, &isSystem, &sysPermsRaw, &created); err != nil {
			writeAPIError(w, err)
			return
		}
		var sysPerms []string
		_ = json.Unmarshal(sysPermsRaw, &sysPerms)
		if sysPerms == nil {
			sysPerms = []string{}
		}
		m := map[string]any{
			"id": id, "apiName": apiName, "label": label, "isSystem": isSystem,
			"systemPermissions": sysPerms, "createdAt": created,
		}
		if desc != nil {
			m["description"] = *desc
		}
		if includeDataAccess {
			section, err := db.LoadDataAccessSection(r.Context(), pool, id)
			if err != nil {
				writeAPIError(w, err)
				return
			}
			attachDataAccess(m, section)
		}
		if includeAutomationAccess {
			autoSection, err := db.LoadAutomationAccessSection(r.Context(), pool, id)
			if err != nil {
				writeAPIError(w, err)
				return
			}
			attachAutomationAccess(m, autoSection)
		}
		if includeToolAccess {
			toolSection, err := db.LoadToolAccessSection(r.Context(), pool, id)
			if err != nil {
				writeAPIError(w, err)
				return
			}
			attachToolAccess(m, toolSection)
		}
		list = append(list, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"permissionSets": list})
}

func (s *Server) handleGetPermissionSet(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := r.PathValue("apiName")
	m, err := loadPermissionSetJSON(r.Context(), pool, apiName, true)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if m == nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "permission set not found")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

type permissionSetObjectPermBody struct {
	ObjectAPIName string `json:"objectApiName"`
	CanCreate     bool   `json:"canCreate"`
	CanRead       bool   `json:"canRead"`
	CanUpdate     bool   `json:"canUpdate"`
	CanDelete     bool   `json:"canDelete"`
	ViewAll       bool   `json:"viewAll"`
	ModifyAll     bool   `json:"modifyAll"`
}

type permissionSetFieldPermBody struct {
	ObjectAPIName string `json:"objectApiName"`
	FieldAPIName  string `json:"fieldApiName"`
	CanRead       bool   `json:"canRead"`
	CanEdit       bool   `json:"canEdit"`
}

type permissionSetDataAccessBody struct {
	ObjectPermissions []permissionSetObjectPermBody `json:"objectPermissions"`
	FieldPermissions  []permissionSetFieldPermBody  `json:"fieldPermissions"`
}

type permissionSetAutomationEntryBody struct {
	APIName string `json:"apiName"`
	CanRun  bool   `json:"canRun"`
}

type permissionSetAutomationAccessBody struct {
	AllAutomations *bool                              `json:"allAutomations"`
	Automations    []permissionSetAutomationEntryBody `json:"automations"`
}

type permissionSetToolEntryBody struct {
	APIName     string `json:"apiName"`
	CanOpen     bool   `json:"canOpen"`
	CanInteract bool   `json:"canInteract"`
	CanModify   bool   `json:"canModify"`
	CanPublish  bool   `json:"canPublish"`
}

type permissionSetToolAccessBody struct {
	AllTools *bool                        `json:"allTools"`
	Tools    []permissionSetToolEntryBody `json:"tools"`
}

func (s *Server) handleCreatePermissionSet(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		APIName           string                             `json:"apiName"`
		Label             string                             `json:"label"`
		Description       *string                            `json:"description"`
		SystemPermissions []string                           `json:"systemPermissions"`
		ObjectPermissions []permissionSetObjectPermBody      `json:"objectPermissions"`
		FieldPermissions  []permissionSetFieldPermBody       `json:"fieldPermissions"`
		DataAccess        *permissionSetDataAccessBody       `json:"dataAccess"`
		AutomationAccess  *permissionSetAutomationAccessBody `json:"automationAccess"`
		ToolAccess        *permissionSetToolAccessBody       `json:"toolAccess"`
		AllAutomations    *bool                              `json:"allAutomations"`
		AllTools          *bool                              `json:"allTools"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.APIName == "" || body.Label == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "apiName and label required")
		return
	}
	if body.SystemPermissions == nil {
		body.SystemPermissions = []string{}
	}
	if err := authz.ValidateSystemPermissions(body.SystemPermissions); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	body.SystemPermissions = authz.NormalizeCapabilitySet(body.SystemPermissions)
	objPerms := body.ObjectPermissions
	fieldPerms := body.FieldPermissions
	if body.DataAccess != nil {
		if len(body.DataAccess.ObjectPermissions) > 0 {
			objPerms = body.DataAccess.ObjectPermissions
		}
		if len(body.DataAccess.FieldPermissions) > 0 {
			fieldPerms = body.DataAccess.FieldPermissions
		}
	}
	sysJSON, _ := json.Marshal(body.SystemPermissions)
	var id string
	var created time.Time
	err := pool.QueryRow(r.Context(), `
INSERT INTO permission_sets (api_name, label, description, system_permissions) VALUES ($1,$2,$3,$4::jsonb)
RETURNING id::text, created_at`, body.APIName, body.Label, body.Description, string(sysJSON)).Scan(&id, &created)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if err := db.BackfillPermissionSetDataAccess(r.Context(), pool, id); err != nil {
		writeAPIError(w, err)
		return
	}
	if err := db.BackfillPermissionSetAutomationAccess(r.Context(), pool, id); err != nil {
		writeAPIError(w, err)
		return
	}
	if err := db.BackfillPermissionSetToolAccess(r.Context(), pool, id); err != nil {
		writeAPIError(w, err)
		return
	}
	if err := upsertPermissionSetDataAccess(r.Context(), pool, id, objPerms, fieldPerms); err != nil {
		writeAPIError(w, err)
		return
	}
	allAutos := false
	if body.AllAutomations != nil {
		allAutos = *body.AllAutomations
	}
	var autoEntries []permissionSetAutomationEntryBody
	if body.AutomationAccess != nil {
		if body.AutomationAccess.AllAutomations != nil {
			allAutos = *body.AutomationAccess.AllAutomations
		}
		autoEntries = body.AutomationAccess.Automations
	}
	if err := db.SetPermissionSetAllAutomations(r.Context(), pool, id, allAutos); err != nil {
		writeAPIError(w, err)
		return
	}
	if err := upsertPermissionSetAutomationAccess(r.Context(), pool, id, autoEntries); err != nil {
		writeAPIError(w, err)
		return
	}
	allTools := false
	if body.AllTools != nil {
		allTools = *body.AllTools
	}
	var toolEntries []permissionSetToolEntryBody
	if body.ToolAccess != nil {
		if body.ToolAccess.AllTools != nil {
			allTools = *body.ToolAccess.AllTools
		}
		toolEntries = body.ToolAccess.Tools
	}
	if err := db.SetPermissionSetAllTools(r.Context(), pool, id, allTools); err != nil {
		writeAPIError(w, err)
		return
	}
	if err := upsertPermissionSetToolAccess(r.Context(), pool, id, toolEntries); err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "permission_set.create", "", nil, map[string]any{"apiName": body.APIName})
	m, err := loadPermissionSetJSON(r.Context(), pool, body.APIName, true)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) handlePatchPermissionSet(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := r.PathValue("apiName")
	var id string
	var isSystem bool
	err := pool.QueryRow(r.Context(), `
SELECT id::text, is_system FROM permission_sets WHERE api_name=$1`, apiName).Scan(&id, &isSystem)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "permission set not found")
		return
	}
	if isSystem {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "cannot mutate system permission set")
		return
	}
	var body struct {
		Label                   *string                            `json:"label"`
		Description             *string                            `json:"description"`
		SystemPermissions       *[]string                          `json:"systemPermissions"`
		SystemPermissionsAdd    *[]string                          `json:"systemPermissionsAdd"`
		SystemPermissionsRemove *[]string                          `json:"systemPermissionsRemove"`
		ObjectPermissions       *[]permissionSetObjectPermBody     `json:"objectPermissions"`
		FieldPermissions        *[]permissionSetFieldPermBody      `json:"fieldPermissions"`
		DataAccess              *permissionSetDataAccessBody       `json:"dataAccess"`
		AutomationAccess        *permissionSetAutomationAccessBody `json:"automationAccess"`
		ToolAccess              *permissionSetToolAccessBody       `json:"toolAccess"`
		AllAutomations          *bool                              `json:"allAutomations"`
		AllTools                *bool                              `json:"allTools"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	if body.Label != nil && strings.TrimSpace(*body.Label) != "" {
		if _, err := pool.Exec(r.Context(), `UPDATE permission_sets SET label=$2 WHERE id=$1::uuid`, id, *body.Label); err != nil {
			writeAPIError(w, err)
			return
		}
	}
	if body.Description != nil {
		if _, err := pool.Exec(r.Context(), `UPDATE permission_sets SET description=$2 WHERE id=$1::uuid`, id, body.Description); err != nil {
			writeAPIError(w, err)
			return
		}
	}
	if body.SystemPermissions != nil || body.SystemPermissionsAdd != nil || body.SystemPermissionsRemove != nil {
		var current []string
		if body.SystemPermissions != nil {
			current = append([]string(nil), *body.SystemPermissions...)
		} else {
			var raw []byte
			if err := pool.QueryRow(r.Context(), `
SELECT COALESCE(system_permissions, '[]'::jsonb) FROM permission_sets WHERE id=$1::uuid`, id).Scan(&raw); err != nil {
				writeAPIError(w, err)
				return
			}
			_ = json.Unmarshal(raw, &current)
		}
		if body.SystemPermissionsAdd != nil {
			current = append(current, (*body.SystemPermissionsAdd)...)
		}
		if body.SystemPermissionsRemove != nil {
			rm := map[string]struct{}{}
			for _, c := range *body.SystemPermissionsRemove {
				rm[c] = struct{}{}
				// Also remove legacy aliases when removing canonical (and vice versa) via normalize pass.
			}
			filtered := current[:0]
			for _, c := range current {
				if _, drop := rm[c]; !drop {
					filtered = append(filtered, c)
				}
			}
			current = filtered
		}
		current = authz.NormalizeCapabilitySet(current)
		if err := authz.ValidateSystemPermissions(current); err != nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		sysJSON, _ := json.Marshal(current)
		if _, err := pool.Exec(r.Context(), `
UPDATE permission_sets SET system_permissions=$2::jsonb WHERE id=$1::uuid`, id, string(sysJSON)); err != nil {
			writeAPIError(w, err)
			return
		}
	}
	objPerms := body.ObjectPermissions
	fieldPerms := body.FieldPermissions
	if body.DataAccess != nil {
		if body.DataAccess.ObjectPermissions != nil {
			ops := body.DataAccess.ObjectPermissions
			objPerms = &ops
		}
		if body.DataAccess.FieldPermissions != nil {
			fps := body.DataAccess.FieldPermissions
			fieldPerms = &fps
		}
	}
	var ops []permissionSetObjectPermBody
	var fps []permissionSetFieldPermBody
	if objPerms != nil {
		ops = *objPerms
	}
	if fieldPerms != nil {
		fps = *fieldPerms
	}
	if objPerms != nil || fieldPerms != nil {
		// Upsert-merge: provided rows update grants; catalog stubs for other objects remain.
		if err := upsertPermissionSetDataAccess(r.Context(), pool, id, ops, fps); err != nil {
			writeAPIError(w, err)
			return
		}
	}
	if body.AllAutomations != nil || body.AutomationAccess != nil {
		allAutos := false
		var autoEntries []permissionSetAutomationEntryBody
		var setAll bool
		if body.AllAutomations != nil {
			allAutos = *body.AllAutomations
			setAll = true
		}
		if body.AutomationAccess != nil {
			if body.AutomationAccess.AllAutomations != nil {
				allAutos = *body.AutomationAccess.AllAutomations
				setAll = true
			}
			autoEntries = body.AutomationAccess.Automations
		}
		if setAll {
			if err := db.SetPermissionSetAllAutomations(r.Context(), pool, id, allAutos); err != nil {
				writeAPIError(w, err)
				return
			}
		}
		if body.AutomationAccess != nil && autoEntries != nil {
			if err := upsertPermissionSetAutomationAccess(r.Context(), pool, id, autoEntries); err != nil {
				writeAPIError(w, err)
				return
			}
		}
	}
	if body.AllTools != nil || body.ToolAccess != nil {
		allTools := false
		var toolEntries []permissionSetToolEntryBody
		var setAll bool
		if body.AllTools != nil {
			allTools = *body.AllTools
			setAll = true
		}
		if body.ToolAccess != nil {
			if body.ToolAccess.AllTools != nil {
				allTools = *body.ToolAccess.AllTools
				setAll = true
			}
			toolEntries = body.ToolAccess.Tools
		}
		if setAll {
			if err := db.SetPermissionSetAllTools(r.Context(), pool, id, allTools); err != nil {
				writeAPIError(w, err)
				return
			}
		}
		if body.ToolAccess != nil && toolEntries != nil {
			if err := upsertPermissionSetToolAccess(r.Context(), pool, id, toolEntries); err != nil {
				writeAPIError(w, err)
				return
			}
		}
	}
	s.writeAudit(r, "permission_set.update", "", nil, map[string]any{"apiName": apiName})
	m, err := loadPermissionSetJSON(r.Context(), pool, apiName, true)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func attachDataAccess(m map[string]any, section db.DataAccessSection) {
	m["dataAccess"] = section
	m["objectPermissions"] = section.ObjectPermissions
	m["fieldPermissions"] = section.FieldPermissions
}

func attachAutomationAccess(m map[string]any, section db.AutomationAccessSection) {
	m["automationAccess"] = section
}

func attachToolAccess(m map[string]any, section db.ToolAccessSection) {
	m["toolAccess"] = section
}

func loadPermissionSetJSON(ctx context.Context, pool *db.Pool, apiName string, withDataAccess bool) (map[string]any, error) {
	var id, label string
	var desc *string
	var isSystem bool
	var sysPermsRaw []byte
	var created time.Time
	err := pool.QueryRow(ctx, `
SELECT id::text, api_name, label, description, is_system, COALESCE(system_permissions, '[]'::jsonb), created_at
FROM permission_sets WHERE api_name=$1`, apiName).Scan(&id, &apiName, &label, &desc, &isSystem, &sysPermsRaw, &created)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	var sysPerms []string
	_ = json.Unmarshal(sysPermsRaw, &sysPerms)
	if sysPerms == nil {
		sysPerms = []string{}
	}
	m := map[string]any{
		"id": id, "apiName": apiName, "label": label, "isSystem": isSystem,
		"systemPermissions": sysPerms, "createdAt": created,
	}
	if desc != nil {
		m["description"] = *desc
	}
	if withDataAccess {
		section, err := db.LoadDataAccessSection(ctx, pool, id)
		if err != nil {
			return nil, err
		}
		attachDataAccess(m, section)
		autoSection, err := db.LoadAutomationAccessSection(ctx, pool, id)
		if err != nil {
			return nil, err
		}
		attachAutomationAccess(m, autoSection)
		toolSection, err := db.LoadToolAccessSection(ctx, pool, id)
		if err != nil {
			return nil, err
		}
		attachToolAccess(m, toolSection)
	}
	return m, nil
}

func upsertPermissionSetDataAccess(
	ctx context.Context,
	pool *db.Pool,
	permissionSetID string,
	objPerms []permissionSetObjectPermBody,
	fieldPerms []permissionSetFieldPermBody,
) error {
	for _, op := range objPerms {
		if op.ObjectAPIName == "" {
			continue
		}
		_, err := pool.Exec(ctx, `
INSERT INTO object_permissions (permission_set_id, object_api_name, can_create, can_read, can_update, can_delete, view_all, modify_all)
VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (permission_set_id, object_api_name) DO UPDATE SET
  can_create = EXCLUDED.can_create,
  can_read = EXCLUDED.can_read,
  can_update = EXCLUDED.can_update,
  can_delete = EXCLUDED.can_delete,
  view_all = EXCLUDED.view_all,
  modify_all = EXCLUDED.modify_all`,
			permissionSetID, op.ObjectAPIName, op.CanCreate, op.CanRead, op.CanUpdate, op.CanDelete, op.ViewAll, op.ModifyAll)
		if err != nil {
			return err
		}
	}
	for _, fp := range fieldPerms {
		if fp.ObjectAPIName == "" || fp.FieldAPIName == "" {
			continue
		}
		_, err := pool.Exec(ctx, `
INSERT INTO field_permissions (permission_set_id, object_api_name, field_api_name, can_read, can_edit)
VALUES ($1::uuid,$2,$3,$4,$5)
ON CONFLICT (permission_set_id, object_api_name, field_api_name)
DO UPDATE SET can_read = EXCLUDED.can_read, can_edit = EXCLUDED.can_edit`,
			permissionSetID, fp.ObjectAPIName, fp.FieldAPIName, fp.CanRead, fp.CanEdit)
		if err != nil {
			return err
		}
	}
	return nil
}

func upsertPermissionSetAutomationAccess(
	ctx context.Context,
	pool *db.Pool,
	permissionSetID string,
	entries []permissionSetAutomationEntryBody,
) error {
	dbEntries := make([]db.AutomationAccessEntry, 0, len(entries))
	for _, e := range entries {
		if e.APIName == "" {
			continue
		}
		dbEntries = append(dbEntries, db.AutomationAccessEntry{APIName: e.APIName, CanRun: e.CanRun})
	}
	return db.UpsertAutomationAccessEntries(ctx, pool, permissionSetID, dbEntries)
}

func upsertPermissionSetToolAccess(
	ctx context.Context,
	pool *db.Pool,
	permissionSetID string,
	entries []permissionSetToolEntryBody,
) error {
	dbEntries := make([]db.ToolAccessEntry, 0, len(entries))
	for _, e := range entries {
		if e.APIName == "" {
			continue
		}
		dbEntries = append(dbEntries, db.ToolAccessEntry{
			APIName: e.APIName, CanOpen: e.CanOpen, CanInteract: e.CanInteract,
			CanModify: e.CanModify, CanPublish: e.CanPublish,
		})
	}
	return db.UpsertToolAccessEntries(ctx, pool, permissionSetID, dbEntries)
}

func (s *Server) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	// Never SELECT/return cleartext secrets on list — forging deliveries requires the HMAC secret.
	rows, err := pool.Query(r.Context(), `
SELECT id::text, api_name, url, (secret IS NOT NULL AND secret <> '') AS has_secret, event_types, active, created_at
FROM webhooks ORDER BY api_name`)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		var id, apiName, url string
		var hasSecret bool
		var eventTypes []byte
		var active bool
		var created time.Time
		if err := rows.Scan(&id, &apiName, &url, &hasSecret, &eventTypes, &active, &created); err != nil {
			writeAPIError(w, err)
			return
		}
		var types any
		_ = json.Unmarshal(eventTypes, &types)
		list = append(list, map[string]any{
			"id": id, "apiName": apiName, "url": url, "hasSecret": hasSecret,
			"eventTypes": types, "active": active, "createdAt": created,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"webhooks": list})
}

func (s *Server) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		APIName    string   `json:"apiName"`
		URL        string   `json:"url"`
		Secret     *string  `json:"secret"`
		EventTypes []string `json:"eventTypes"`
		Active     *bool    `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.APIName == "" || body.URL == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "apiName and url required")
		return
	}
	if err := webhook.ValidateDeliveryURL(body.URL); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid webhook url: "+err.Error())
		return
	}
	active := true
	if body.Active != nil {
		active = *body.Active
	}
	types := body.EventTypes
	if len(types) == 0 {
		types = []string{"*"}
	}
	typesJSON, _ := json.Marshal(types)
	var storedSecret *string
	if body.Secret != nil && *body.Secret != "" {
		encKey := ""
		if s.cfg != nil {
			encKey = s.cfg.WebhookEncryptionKey
		}
		enc, err := webhook.EncryptSecret(*body.Secret, encKey)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "SECRET_ERROR", "failed to protect webhook secret")
			return
		}
		storedSecret = &enc
	}
	var id string
	var created time.Time
	err := pool.QueryRow(r.Context(), `
INSERT INTO webhooks (api_name, url, secret, event_types, active) VALUES ($1,$2,$3,$4::jsonb,$5)
RETURNING id::text, created_at`, body.APIName, body.URL, storedSecret, string(typesJSON), active).Scan(&id, &created)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": id, "apiName": body.APIName, "url": body.URL, "secret": body.Secret,
		"eventTypes": types, "active": active, "createdAt": created,
	})
}

func (s *Server) handleListAgentHarnesses(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"harnesses":  agentharness.Catalog(),
		"jobClasses": agentharness.JobCatalog(),
	})
}

func (s *Server) handleListPlaybooks(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	rows, err := pool.Query(r.Context(), `
SELECT id::text, api_name, label, goal_template, COALESCE(instructions, ''), allowed_tools, object_scopes,
       COALESCE(allowed_skills, '[]'::jsonb), COALESCE(allowed_canvas_specs, '[]'::jsonb),
       require_approval, active, ownership, package_name, created_at, updated_at,
       COALESCE(primary_section, ''), COALESCE(harness_id, ''), COALESCE(harness_version, ''),
       COALESCE(job_class, '')
FROM agent_playbooks ORDER BY api_name`)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		m, err := scanPlaybookRow(rows)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		list = append(list, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"playbooks": list})
}

func (s *Server) handleGetPlaybook(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := r.PathValue("apiName")
	m, err := loadPlaybook(r.Context(), pool, apiName)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "Playbook not found: "+apiName)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleCreatePlaybook(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		APIName            string   `json:"apiName"`
		Label              string   `json:"label"`
		GoalTemplate       string   `json:"goalTemplate"`
		Instructions       string   `json:"instructions"`
		PrimarySection     string   `json:"primarySection"`
		JobClass           string   `json:"jobClass"`
		AllowedTools       []string `json:"allowedTools"`
		ObjectScopes       []string `json:"objectScopes"`
		AllowedSkills      []string `json:"allowedSkills"`
		AllowedCanvasSpecs []string `json:"allowedCanvasSpecs"`
		AllowedToolSpecs   []string `json:"allowedToolSpecs"`
		RequireApproval    bool     `json:"requireApproval"`
		PackageName        *string  `json:"packageName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.APIName == "" || body.Label == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "apiName and label required")
		return
	}
	binding, err := agentharness.BindSpec(body.JobClass, body.PrimarySection)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	tools := agentharness.EnsureToolFloor(binding.ToolFloor, body.AllowedTools)
	requireApproval := agentharness.EffectiveRequireApproval(binding.RequireApprovalDefault, body.RequireApproval)
	scopes := body.ObjectScopes
	if scopes == nil {
		scopes = []string{}
	}
	skills := body.AllowedSkills
	if skills == nil {
		skills = []string{}
	}
	canvasSpecs := mergeAllowedToolSpecs(body.AllowedToolSpecs, body.AllowedCanvasSpecs)
	if canvasSpecs == nil {
		canvasSpecs = []string{}
	}
	if err := validatePlaybookSkills(r.Context(), pool, skills); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	if err := validatePlaybookCanvasSpecs(r.Context(), pool, canvasSpecs); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	pkg := metadata.DefaultCustomerPackage
	if body.PackageName != nil && *body.PackageName != "" {
		pkg = *body.PackageName
	}
	toolsJSON, _ := json.Marshal(tools)
	scopesJSON, _ := json.Marshal(scopes)
	skillsJSON, _ := json.Marshal(skills)
	canvasSpecsJSON, _ := json.Marshal(canvasSpecs)
	var id string
	var created, updated time.Time
	err = pool.QueryRow(r.Context(), `
INSERT INTO agent_playbooks (
  api_name, label, goal_template, instructions, allowed_tools, object_scopes, allowed_skills,
  allowed_canvas_specs, require_approval, active, ownership, package_name,
  primary_section, harness_id, harness_version, job_class
)
VALUES ($1,$2,$3,$4,$5::jsonb,$6::jsonb,$7::jsonb,$8::jsonb,$9,true,'custom',$10,$11,$12,$13,$14)
RETURNING id::text, created_at, updated_at`,
		body.APIName, body.Label, body.GoalTemplate, body.Instructions,
		string(toolsJSON), string(scopesJSON), string(skillsJSON), string(canvasSpecsJSON), requireApproval, pkg,
		nullablePlaybookText(binding.PrimarySection), binding.HarnessID, binding.HarnessVersion,
		nullablePlaybookText(binding.JobClass),
	).Scan(&id, &created, &updated)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": id, "apiName": body.APIName, "label": body.Label, "goalTemplate": body.GoalTemplate,
		"instructions": body.Instructions, "allowedTools": tools, "objectScopes": scopes,
		"allowedSkills":      skills,
		"allowedToolSpecs":   canvasSpecs,
		"allowedCanvasSpecs": canvasSpecs,
		"requireApproval":    requireApproval, "active": true, "ownership": "custom",
		"packageName": pkg, "primarySection": binding.PrimarySection, "jobClass": binding.JobClass,
		"harnessId": binding.HarnessID, "harnessVersion": binding.HarnessVersion,
		"createdAt": created, "updatedAt": updated,
	})
}

func (s *Server) handlePatchPlaybook(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := r.PathValue("apiName")
	var ownership, curSection, curHarnessID, curJobClass string
	var curTools []byte
	err := pool.QueryRow(r.Context(), `
SELECT ownership, COALESCE(primary_section, ''), COALESCE(harness_id, ''),
       COALESCE(job_class, ''), allowed_tools
FROM agent_playbooks WHERE api_name=$1`, apiName).Scan(
		&ownership, &curSection, &curHarnessID, &curJobClass, &curTools)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "Playbook not found: "+apiName)
		return
	}
	if err := metadata.AssertCustomerMutable(ownership, apiName, "agentSpec"); err != nil {
		writeAPIError(w, err)
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	sets := []string{"updated_at=now()"}
	args := []any{apiName}
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, col+"=$"+strconv.Itoa(len(args)))
	}

	bindingSection := curSection
	bindingJobClass := curJobClass
	var floorBinding *agentharness.Binding
	_, jobClassSet := body["jobClass"]
	_, sectionSet := body["primarySection"]
	if jobClassSet || sectionSet {
		newJC := curJobClass
		newSec := curSection
		if v, ok := body["jobClass"].(string); ok {
			newJC = strings.TrimSpace(v)
		} else if jobClassSet {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "jobClass must be a string")
			return
		}
		if v, ok := body["primarySection"].(string); ok {
			newSec = strings.TrimSpace(v)
		} else if sectionSet {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "primarySection must be a string")
			return
		}
		if jobClassSet && !sectionSet {
			if parsed, err := agentharness.ParseJobClass(newJC); err == nil {
				newSec = agentharness.SectionForJobClass(parsed)
			}
		}
		if sectionSet && !jobClassSet {
			newJC = string(agentharness.JobClassForSection(newSec))
		}
		binding, bindErr := agentharness.BindSpec(newJC, newSec)
		if bindErr != nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", bindErr.Error())
			return
		}
		floorBinding = &binding
		bindingSection = binding.PrimarySection
		bindingJobClass = binding.JobClass
		add("primary_section", nullablePlaybookText(binding.PrimarySection))
		add("job_class", nullablePlaybookText(binding.JobClass))
		add("harness_id", binding.HarnessID)
		add("harness_version", binding.HarnessVersion)
		curHarnessID = binding.HarnessID
		if _, toolsSet := body["allowedTools"]; !toolsSet {
			var existing []string
			_ = json.Unmarshal(curTools, &existing)
			tools := agentharness.EnsureToolFloor(binding.ToolFloor, existing)
			b, _ := json.Marshal(tools)
			add("allowed_tools", string(b))
			sets[len(sets)-1] = "allowed_tools=$" + strconv.Itoa(len(args)) + "::jsonb"
		}
		if _, apprSet := body["requireApproval"]; !apprSet && binding.RequireApprovalDefault {
			add("require_approval", true)
		}
	} else if curJobClass != "" {
		if b, bindErr := agentharness.BindSpec(curJobClass, curSection); bindErr == nil {
			floorBinding = &b
		}
	} else if curSection != "" {
		if b, bindErr := agentharness.Bind(curSection); bindErr == nil {
			floorBinding = &b
		}
	}
	// Clients cannot override harness ids directly — SoR is jobClass / primarySection bind.
	if _, ok := body["harnessId"]; ok {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "harnessId is server-managed; set jobClass or primarySection instead")
		return
	}
	if _, ok := body["harnessVersion"]; ok {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "harnessVersion is server-managed; set jobClass or primarySection instead")
		return
	}

	if v, ok := body["label"].(string); ok {
		add("label", v)
	}
	if v, ok := body["goalTemplate"].(string); ok {
		add("goal_template", v)
	}
	if v, ok := body["instructions"].(string); ok {
		add("instructions", v)
	}
	if v, ok := body["requireApproval"].(bool); ok {
		floorDefault := false
		if floorBinding != nil {
			floorDefault = floorBinding.RequireApprovalDefault
		} else if bindingSection != "" {
			if b, bindErr := agentharness.Bind(bindingSection); bindErr == nil {
				floorDefault = b.RequireApprovalDefault
			}
		}
		add("require_approval", agentharness.EffectiveRequireApproval(floorDefault, v))
	}
	if v, ok := body["active"].(bool); ok {
		add("active", v)
	}
	if v, ok := body["allowedTools"]; ok {
		b, _ := json.Marshal(v)
		var tools []string
		_ = json.Unmarshal(b, &tools)
		if floorBinding != nil {
			tools = agentharness.EnsureToolFloor(floorBinding.ToolFloor, tools)
		} else if bindingJobClass != "" {
			if bind, bindErr := agentharness.BindSpec(bindingJobClass, bindingSection); bindErr == nil {
				tools = agentharness.EnsureToolFloor(bind.ToolFloor, tools)
			}
		} else if bindingSection != "" {
			if bind, bindErr := agentharness.Bind(bindingSection); bindErr == nil {
				tools = agentharness.EnsureToolFloor(bind.ToolFloor, tools)
			}
		} else if curHarnessID != "" {
			if def, ok := agentharness.ForID(curHarnessID); ok {
				tools = agentharness.EnsureToolFloor(def.ToolFloor, tools)
			}
		}
		tb, _ := json.Marshal(tools)
		add("allowed_tools", string(tb))
		sets[len(sets)-1] = "allowed_tools=$" + strconv.Itoa(len(args)) + "::jsonb"
	}
	if v, ok := body["objectScopes"]; ok {
		b, _ := json.Marshal(v)
		add("object_scopes", string(b))
		sets[len(sets)-1] = "object_scopes=$" + strconv.Itoa(len(args)) + "::jsonb"
	}
	if v, ok := body["allowedSkills"]; ok {
		b, _ := json.Marshal(v)
		var skills []string
		_ = json.Unmarshal(b, &skills)
		if err := validatePlaybookSkills(r.Context(), pool, skills); err != nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		add("allowed_skills", string(b))
		sets[len(sets)-1] = "allowed_skills=$" + strconv.Itoa(len(args)) + "::jsonb"
	}
	if v, ok := body["allowedToolSpecs"]; ok {
		b, _ := json.Marshal(v)
		var specs []string
		_ = json.Unmarshal(b, &specs)
		if err := validatePlaybookCanvasSpecs(r.Context(), pool, specs); err != nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		add("allowed_canvas_specs", string(b))
		sets[len(sets)-1] = "allowed_canvas_specs=$" + strconv.Itoa(len(args)) + "::jsonb"
	} else if v, ok := body["allowedCanvasSpecs"]; ok {
		b, _ := json.Marshal(v)
		var specs []string
		_ = json.Unmarshal(b, &specs)
		if err := validatePlaybookCanvasSpecs(r.Context(), pool, specs); err != nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		add("allowed_canvas_specs", string(b))
		sets[len(sets)-1] = "allowed_canvas_specs=$" + strconv.Itoa(len(args)) + "::jsonb"
	}
	_, err = pool.Exec(r.Context(), `UPDATE agent_playbooks SET `+strings.Join(sets, ",")+` WHERE api_name=$1`, args...)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	m, err := loadPlaybook(r.Context(), pool, apiName)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleDeletePlaybook(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := r.PathValue("apiName")
	var ownership string
	err := pool.QueryRow(r.Context(), `SELECT ownership FROM agent_playbooks WHERE api_name=$1`, apiName).Scan(&ownership)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "Playbook not found: "+apiName)
		return
	}
	if err := metadata.AssertCustomerMutable(ownership, apiName, "agentSpec"); err != nil {
		writeAPIError(w, err)
		return
	}
	tag, err := pool.Exec(r.Context(), `DELETE FROM agent_playbooks WHERE api_name=$1`, apiName)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "Playbook not found: "+apiName)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type playbookScanner interface {
	Scan(dest ...any) error
}

func scanPlaybookRow(row playbookScanner) (map[string]any, error) {
	var id, apiName, label, goal, instructions, ownership, pkg string
	var primarySection, harnessID, harnessVersion, jobClass string
	var tools, scopes, skills, canvasSpecs []byte
	var requireApproval, active bool
	var created, updated time.Time
	if err := row.Scan(&id, &apiName, &label, &goal, &instructions, &tools, &scopes, &skills, &canvasSpecs,
		&requireApproval, &active, &ownership, &pkg, &created, &updated,
		&primarySection, &harnessID, &harnessVersion, &jobClass); err != nil {
		return nil, err
	}
	var t, sc, sk, cs any
	_ = json.Unmarshal(tools, &t)
	_ = json.Unmarshal(scopes, &sc)
	_ = json.Unmarshal(skills, &sk)
	_ = json.Unmarshal(canvasSpecs, &cs)
	if sk == nil {
		sk = []any{}
	}
	if cs == nil {
		cs = []any{}
	}
	return map[string]any{
		"id": id, "apiName": apiName, "label": label, "goalTemplate": goal,
		"instructions": instructions, "allowedTools": t, "objectScopes": sc,
		"allowedSkills": sk, "allowedCanvasSpecs": cs, "allowedToolSpecs": cs,
		"requireApproval": requireApproval, "active": active, "ownership": ownership,
		"packageName": pkg, "primarySection": primarySection, "jobClass": jobClass,
		"harnessId": harnessID, "harnessVersion": harnessVersion, "createdAt": created, "updatedAt": updated,
	}, nil
}

func loadPlaybook(ctx context.Context, pool *db.Pool, apiName string) (map[string]any, error) {
	row := pool.QueryRow(ctx, `
SELECT id::text, api_name, label, goal_template, COALESCE(instructions, ''), allowed_tools, object_scopes,
       COALESCE(allowed_skills, '[]'::jsonb), COALESCE(allowed_canvas_specs, '[]'::jsonb),
       require_approval, active, ownership, package_name, created_at, updated_at,
       COALESCE(primary_section, ''), COALESCE(harness_id, ''), COALESCE(harness_version, ''),
       COALESCE(job_class, '')
FROM agent_playbooks WHERE api_name=$1`, apiName)
	return scanPlaybookRow(row)
}

func nullablePlaybookText(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func validatePlaybookSkills(ctx context.Context, pool *db.Pool, skills []string) error {
	for _, s := range skills {
		s = strings.TrimSpace(s)
		if s == "" {
			return errors.New("allowedSkills entries must be non-empty automation apiNames")
		}
		var one int
		err := pool.QueryRow(ctx, `SELECT 1 FROM metadata_automations WHERE api_name=$1`, s).Scan(&one)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("allowedSkills %q is not a known automation", s)
			}
			return err
		}
	}
	return nil
}

func (s *Server) handleBuildProjections(w http.ResponseWriter, r *http.Request) {
	if s.data == nil || s.meta == nil || s.pool == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	object := r.PathValue("object")
	result, err := s.data.RebuildProjections(r.Context(), object)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	payload, _ := json.Marshal(map[string]any{"objectApiName": object})
	var jobID string
	var status string
	var created time.Time
	_ = s.pool.QueryRow(r.Context(), `
INSERT INTO jobs (job_type, payload) VALUES ('projection.build', $1::jsonb)
RETURNING id::text, status, created_at`, string(payload)).Scan(&jobID, &status, &created)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"result": result,
		"job":    map[string]any{"id": jobID, "jobType": "projection.build", "payload": map[string]any{"objectApiName": object}, "status": status, "createdAt": created},
	})
}

func (s *Server) handleListProjections(w http.ResponseWriter, r *http.Request) {
	if s.data == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	list, err := s.data.ListProjections(r.Context(), r.PathValue("object"))
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projections": list})
}

func (s *Server) enqueueProjectionBuild(ctx context.Context, objectAPIName string) {
	if s.pool == nil || objectAPIName == "" {
		return
	}
	if s.meta != nil {
		obj, err := s.meta.GetObject(ctx, objectAPIName)
		if err == nil && db.IsKernelStorage(obj.StorageMode) {
			return
		}
	}
	payload, _ := json.Marshal(map[string]any{"objectApiName": objectAPIName})
	_, _ = s.pool.Exec(ctx, `
INSERT INTO jobs (job_type, payload) VALUES ('projection.build', $1::jsonb)`, string(payload))
}

func (s *Server) handleListPackages(w http.ResponseWriter, r *http.Request) {
	if s.meta == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "metadata service not configured")
		return
	}
	list, err := seed.ListPackageStatuses(r.Context(), s.meta)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"packages": list})
}

func (s *Server) handleGetPackage(w http.ResponseWriter, r *http.Request) {
	if s.meta == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "metadata service not configured")
		return
	}
	st, err := seed.GetPackageStatus(r.Context(), s.meta, r.PathValue("name"))
	if err != nil {
		if strings.HasPrefix(err.Error(), "unknown package") {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleEnablePackage(w http.ResponseWriter, r *http.Request) {
	if s.meta == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "metadata service not configured")
		return
	}
	name := r.PathValue("name")
	st, err := seed.EnablePackage(r.Context(), s.meta, name)
	if err != nil {
		msg := err.Error()
		if strings.HasPrefix(msg, "unknown package") || strings.Contains(msg, "not optionally enableable") {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", msg)
			return
		}
		if strings.HasPrefix(msg, "dependency not installed") {
			writeErr(w, http.StatusConflict, "CONFLICT", msg)
			return
		}
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "metadata.package.enable", "", nil, map[string]any{"packageName": name, "version": st.Version})
	for _, obj := range st.ObjectAPINames {
		s.enqueueProjectionBuild(r.Context(), obj)
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleDisablePackage(w http.ResponseWriter, r *http.Request) {
	if s.meta == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "metadata service not configured")
		return
	}
	name := r.PathValue("name")
	st, err := seed.DisablePackage(r.Context(), s.meta, name)
	if err != nil {
		msg := err.Error()
		if strings.HasPrefix(msg, "unknown package") || strings.Contains(msg, "not optionally enableable") {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", msg)
			return
		}
		if strings.HasPrefix(msg, "package not installed") {
			writeErr(w, http.StatusConflict, "CONFLICT", msg)
			return
		}
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "metadata.package.disable", "", nil, map[string]any{"packageName": name})
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) writeAudit(r *http.Request, action, objectAPIName string, recordID *string, details map[string]any) {
	if s.pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	var actorID *string
	if actor != nil {
		actorID = &actor.ID
	}
	detailsJSON, _ := json.Marshal(details)
	_, _ = s.pool.Exec(r.Context(), `
INSERT INTO audit_log (actor_id, action, object_api_name, record_id, details)
VALUES ($1::uuid, $2, $3, $4::uuid, $5::jsonb)`,
		actorID, action, nullStr(objectAPIName), recordID, string(detailsJSON))
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
