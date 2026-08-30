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

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/canvas"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
)

const canvasSpecSelectCols = `id::text, api_name, label, description, icon, sort_order, layout, nodes, data_bindings,
       active, ownership, package_name, created_at, updated_at`

func (s *Server) registerCanvasSpecRoutes(prefix string) {
	meta := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeMetadata, h))
	}
	capMeta := func(cap string, h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeMetadata, s.requireCapability(cap, h)))
	}
	// Legacy CanvasSpec path (kept as alias).
	s.mux.Handle("GET "+prefix+"/canvases", meta(s.handleListCanvasSpecsLegacy))
	s.mux.Handle("GET "+prefix+"/canvases/{apiName}", meta(s.handleGetCanvasSpec))
	s.mux.Handle("POST "+prefix+"/canvases", capMeta(authz.CapMetadataBuild, s.handleCreateCanvasSpec))
	s.mux.Handle("PATCH "+prefix+"/canvases/{apiName}", capMeta(authz.CapMetadataBuild, s.handlePatchCanvasSpec))
	s.mux.Handle("DELETE "+prefix+"/canvases/{apiName}", capMeta(authz.CapMetadataBuild, s.handleDeleteCanvasSpec))
	// ToolSpec product path (ADR-021 / BP-050 Phase 2).
	s.mux.Handle("GET "+prefix+"/tools", meta(s.handleListToolSpecs))
	s.mux.Handle("GET "+prefix+"/tools/{apiName}", meta(s.handleGetCanvasSpec))
	s.mux.Handle("POST "+prefix+"/tools", capMeta(authz.CapMetadataBuild, s.handleCreateCanvasSpec))
	s.mux.Handle("PATCH "+prefix+"/tools/{apiName}", capMeta(authz.CapMetadataBuild, s.handlePatchCanvasSpec))
	s.mux.Handle("DELETE "+prefix+"/tools/{apiName}", capMeta(authz.CapMetadataBuild, s.handleDeleteCanvasSpec))
}

func (s *Server) handleListCanvasSpecsLegacy(w http.ResponseWriter, r *http.Request) {
	s.listCanvasSpecs(w, r, "canvases")
}

func (s *Server) handleListToolSpecs(w http.ResponseWriter, r *http.Request) {
	s.listCanvasSpecs(w, r, "tools")
}

func (s *Server) listCanvasSpecs(w http.ResponseWriter, r *http.Request, listKey string) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	// Package-gate managed ToolSpecs: customer.* always visible;
	// managed package specs only when that package is installed + enabled.
	rows, err := pool.Query(r.Context(), `
SELECT c.id::text, c.api_name, c.label, c.description, c.icon, c.sort_order, c.layout, c.nodes, c.data_bindings,
       c.active, c.ownership, c.package_name, c.created_at, c.updated_at
FROM metadata_canvases c
WHERE c.ownership = 'custom'
   OR c.package_name IN ('customer.default', 'core')
   OR EXISTS (
        SELECT 1 FROM package_installs p
        WHERE p.package_name = c.package_name AND p.enabled = true
   )
ORDER BY c.sort_order ASC, c.label ASC, c.api_name ASC`)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		m, err := scanCanvasSpecRow(rows)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		list = append(list, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{listKey: list})
}

func (s *Server) handleGetCanvasSpec(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := r.PathValue("apiName")
	m, err := loadCanvasSpec(r.Context(), pool, apiName)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "ToolSpec not found: "+apiName)
		return
	}
	if !canvasSpecVisible(r.Context(), pool, m) {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "ToolSpec not found: "+apiName)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

type canvasSpecWriteBody struct {
	APIName      string          `json:"apiName"`
	Label        string          `json:"label"`
	Description  string          `json:"description"`
	Icon         string          `json:"icon"`
	SortOrder    *int            `json:"sortOrder"`
	Layout       json.RawMessage `json:"layout"`
	Nodes        json.RawMessage `json:"nodes"`
	DataBindings json.RawMessage `json:"dataBindings"`
	Document     json.RawMessage `json:"document"`
	Active       *bool           `json:"active"`
	PackageName  *string         `json:"packageName"`
}

func unwrapCanvasDocument(body *canvasSpecWriteBody) error {
	if len(body.Document) == 0 {
		return nil
	}
	var doc map[string]any
	if err := json.Unmarshal(body.Document, &doc); err != nil {
		return fmt.Errorf("document: invalid JSON")
	}
	if layout, ok := doc["layout"]; ok && len(body.Layout) == 0 {
		b, err := json.Marshal(layout)
		if err != nil {
			return err
		}
		body.Layout = b
	}
	if nodes, ok := doc["nodes"]; ok && len(body.Nodes) == 0 {
		b, err := json.Marshal(nodes)
		if err != nil {
			return err
		}
		body.Nodes = b
	}
	if bindings, ok := doc["dataBindings"]; ok && len(body.DataBindings) == 0 {
		b, err := json.Marshal(bindings)
		if err != nil {
			return err
		}
		body.DataBindings = b
	}
	return nil
}

func (s *Server) handleCreateCanvasSpec(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body canvasSpecWriteBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.APIName == "" || body.Label == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "apiName and label required")
		return
	}
	if err := unwrapCanvasDocument(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	sanitizedNodes, err := canvas.SanitizeNodesJSON(body.Nodes)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	body.Nodes = sanitizedNodes
	if err := canvas.ValidateSpecBody(body.Layout, body.Nodes, body.DataBindings); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	active := true
	if body.Active != nil {
		active = *body.Active
	}
	pkg := metadata.DefaultCustomerPackage
	if body.PackageName != nil && *body.PackageName != "" {
		pkg = *body.PackageName
	}
	sortOrder := 0
	if body.SortOrder != nil {
		sortOrder = *body.SortOrder
	}
	layout := body.Layout
	nodes := body.Nodes
	bindings := body.DataBindings
	if bindings == nil {
		bindings = json.RawMessage(`[]`)
	}
	var id string
	var created, updated time.Time
	err = pool.QueryRow(r.Context(), `
INSERT INTO metadata_canvases (api_name, label, description, icon, sort_order, layout, nodes, data_bindings, active, ownership, package_name)
VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,$8::jsonb,$9,'custom',$10)
RETURNING id::text, created_at, updated_at`,
		body.APIName, body.Label, body.Description, body.Icon, sortOrder,
		string(layout), string(nodes), string(bindings), active, pkg,
	).Scan(&id, &created, &updated)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if err := db.EnsureToolInAccessCatalog(r.Context(), pool, body.APIName); err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, canvasSpecJSON(id, body.APIName, body.Label, body.Description, body.Icon, sortOrder, layout, nodes, bindings, active, "custom", pkg, created, updated))
}

func (s *Server) handlePatchCanvasSpec(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := r.PathValue("apiName")
	var ownership string
	err := pool.QueryRow(r.Context(), `SELECT ownership FROM metadata_canvases WHERE api_name=$1`, apiName).Scan(&ownership)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "ToolSpec not found: "+apiName)
		return
	}
	if err := metadata.AssertCustomerMutable(ownership, apiName, "toolSpec"); err != nil {
		writeAPIError(w, err)
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	if docRaw, ok := body["document"]; ok {
		b, _ := json.Marshal(docRaw)
		tmp := canvasSpecWriteBody{Document: b}
		if err := unwrapCanvasDocument(&tmp); err != nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		if len(tmp.Layout) > 0 {
			body["layout"] = json.RawMessage(tmp.Layout)
		}
		if len(tmp.Nodes) > 0 {
			body["nodes"] = json.RawMessage(tmp.Nodes)
		}
		if len(tmp.DataBindings) > 0 {
			body["dataBindings"] = json.RawMessage(tmp.DataBindings)
		}
	}
	existing, err := loadCanvasSpec(r.Context(), pool, apiName)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	layout := json.RawMessage(mustJSONBytes(existing["layout"]))
	nodes := json.RawMessage(mustJSONBytes(existing["nodes"]))
	bindings := json.RawMessage(mustJSONBytes(existing["dataBindings"]))
	if v, ok := body["layout"]; ok {
		layout = json.RawMessage(mustJSONBytes(v))
	}
	if v, ok := body["nodes"]; ok {
		nodes = json.RawMessage(mustJSONBytes(v))
	}
	if v, ok := body["dataBindings"]; ok {
		bindings = json.RawMessage(mustJSONBytes(v))
	}
	sanitizedNodes, err := canvas.SanitizeNodesJSON(nodes)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	nodes = sanitizedNodes
	if err := canvas.ValidateSpecBody(layout, nodes, bindings); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	sets := []string{"updated_at=now()", "layout=$2::jsonb", "nodes=$3::jsonb", "data_bindings=$4::jsonb"}
	args := []any{apiName, string(layout), string(nodes), string(bindings)}
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, col+"=$"+strconv.Itoa(len(args)))
	}
	if v, ok := body["label"].(string); ok {
		add("label", v)
	}
	if v, ok := body["description"].(string); ok {
		add("description", v)
	}
	if v, ok := body["icon"].(string); ok {
		add("icon", v)
	}
	if v, ok := body["sortOrder"].(float64); ok {
		add("sort_order", int(v))
	}
	if v, ok := body["active"].(bool); ok {
		add("active", v)
	}
	_, err = pool.Exec(r.Context(), `UPDATE metadata_canvases SET `+strings.Join(sets, ",")+` WHERE api_name=$1`, args...)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	m, err := loadCanvasSpec(r.Context(), pool, apiName)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleDeleteCanvasSpec(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := r.PathValue("apiName")
	var ownership string
	err := pool.QueryRow(r.Context(), `SELECT ownership FROM metadata_canvases WHERE api_name=$1`, apiName).Scan(&ownership)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "ToolSpec not found: "+apiName)
		return
	}
	if err := metadata.AssertCustomerMutable(ownership, apiName, "toolSpec"); err != nil {
		writeAPIError(w, err)
		return
	}
	_, err = pool.Exec(r.Context(), `DELETE FROM metadata_canvases WHERE api_name=$1 AND ownership='custom'`, apiName)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if err := db.RemoveToolFromAccessCatalog(r.Context(), pool, apiName); err != nil {
		writeAPIError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type canvasSpecScanner interface {
	Scan(dest ...any) error
}

func scanCanvasSpecRow(row canvasSpecScanner) (map[string]any, error) {
	var id, apiName, label, description, icon, ownership, pkg string
	var sortOrder int
	var layout, nodes, bindings []byte
	var active bool
	var created, updated time.Time
	if err := row.Scan(&id, &apiName, &label, &description, &icon, &sortOrder, &layout, &nodes, &bindings, &active, &ownership, &pkg, &created, &updated); err != nil {
		return nil, err
	}
	return canvasSpecJSON(id, apiName, label, description, icon, sortOrder, json.RawMessage(layout), json.RawMessage(nodes), json.RawMessage(bindings), active, ownership, pkg, created, updated), nil
}

func loadCanvasSpec(ctx context.Context, pool *db.Pool, apiName string) (map[string]any, error) {
	row := pool.QueryRow(ctx, `
SELECT `+canvasSpecSelectCols+`
FROM metadata_canvases WHERE api_name=$1`, apiName)
	return scanCanvasSpecRow(row)
}

func canvasSpecJSON(
	id, apiName, label, description, icon string,
	sortOrder int,
	layout, nodes, bindings json.RawMessage,
	active bool, ownership, pkg string,
	created, updated time.Time,
) map[string]any {
	var layoutVal, nodesVal, bindingsVal any
	_ = json.Unmarshal(layout, &layoutVal)
	_ = json.Unmarshal(nodes, &nodesVal)
	_ = json.Unmarshal(bindings, &bindingsVal)
	if bindingsVal == nil {
		bindingsVal = []any{}
	}
	title := label
	doc := map[string]any{
		"apiVersion":   canvas.DocumentAPIVersion,
		"id":           apiName,
		"title":        title,
		"layout":       layoutVal,
		"nodes":        nodesVal,
		"dataBindings": bindingsVal,
	}
	return map[string]any{
		"id": id, "apiName": apiName, "label": label, "description": description,
		"icon": icon, "sortOrder": sortOrder,
		"layout": layoutVal, "nodes": nodesVal, "dataBindings": bindingsVal,
		"document": doc,
		"active":   active, "ownership": ownership, "packageName": pkg,
		"createdAt": created, "updatedAt": updated,
	}
}

func mustJSONBytes(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("null")
	}
	return b
}

func validatePlaybookCanvasSpecs(ctx context.Context, pool *db.Pool, specs []string) error {
	for _, name := range specs {
		name = strings.TrimSpace(name)
		if name == "" {
			return errors.New("allowedToolSpecs entries must be non-empty ToolSpec apiNames")
		}
		m, err := loadCanvasSpec(ctx, pool, name)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("allowedToolSpecs %q is not a known ToolSpec", name)
			}
			return err
		}
		if !canvasSpecVisible(ctx, pool, m) {
			return fmt.Errorf("allowedToolSpecs %q requires an enabled package", name)
		}
	}
	return nil
}

// mergeAllowedToolSpecs prefers allowedToolSpecs, falls back to allowedCanvasSpecs.
func mergeAllowedToolSpecs(toolSpecs, canvasSpecs []string) []string {
	if len(toolSpecs) > 0 {
		return toolSpecs
	}
	return canvasSpecs
}

// canvasSpecVisible reports whether a ToolSpec is package-gated on.
func canvasSpecVisible(ctx context.Context, pool *db.Pool, m map[string]any) bool {
	ownership, _ := m["ownership"].(string)
	pkg, _ := m["packageName"].(string)
	if ownership == "custom" || pkg == "" || pkg == "customer.default" || pkg == "core" {
		return true
	}
	var enabled bool
	err := pool.QueryRow(ctx, `
SELECT enabled FROM package_installs WHERE package_name=$1`, pkg).Scan(&enabled)
	if err != nil {
		return false
	}
	return enabled
}
