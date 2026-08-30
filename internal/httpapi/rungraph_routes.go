package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/rungraph"
)

func (s *Server) registerRunGraphRoutes(prefix string) {
	client := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeClient, h))
	}
	s.mux.Handle("GET "+prefix+"/run-graphs/home", client(s.handleGetHomeRunGraph))
	s.mux.Handle("GET "+prefix+"/run-graphs/{graphKey}", client(s.handleGetRunGraph))
	s.mux.Handle("PUT "+prefix+"/run-graphs/{graphKey}", client(s.handlePutRunGraph))
	s.mux.Handle("PATCH "+prefix+"/run-graphs/{graphKey}", client(s.handlePatchRunGraph))
	s.mux.Handle("POST "+prefix+"/run-graphs/resolve", client(s.handleResolveRunGraph))
}

func (s *Server) handleGetHomeRunGraph(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil || strings.TrimSpace(actor.ID) == "" {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing actor")
		return
	}
	graph, err := rungraph.NewStore(pool).GetOrCreateHome(r.Context(), actor.ID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeRunGraph(w, http.StatusOK, graph)
}

func (s *Server) handleGetRunGraph(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil || strings.TrimSpace(actor.ID) == "" {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing actor")
		return
	}
	graphKey := strings.TrimSpace(r.PathValue("graphKey"))
	if err := rungraph.ValidateGraphKey(graphKey); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	graph, err := rungraph.NewStore(pool).Get(r.Context(), actor.ID, graphKey)
	if err != nil {
		if rungraph.IsNotFound(err) {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "Run graph not found: "+graphKey)
			return
		}
		writeAPIError(w, err)
		return
	}
	writeRunGraph(w, http.StatusOK, graph)
}

func (s *Server) handlePutRunGraph(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil || strings.TrimSpace(actor.ID) == "" {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing actor")
		return
	}
	graphKey := strings.TrimSpace(r.PathValue("graphKey"))
	if err := rungraph.ValidateGraphKey(graphKey); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	body, err := readBodyJSON(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	prepared, title, err := prepareRunGraphWrite(body, graphKey)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	expected, err := parseRunGraphIfMatch(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	store := rungraph.NewStore(pool)
	current, currentErr := store.Get(r.Context(), actor.ID, graphKey)
	if currentErr != nil && !rungraph.IsNotFound(currentErr) {
		writeAPIError(w, currentErr)
		return
	}
	graph, err := store.UpsertIfRevision(r.Context(), actor.ID, graphKey, title, prepared, expected)
	if err != nil {
		if rungraph.IsRevisionConflict(err) {
			writeErr(w, http.StatusConflict, "REVISION_CONFLICT", "Run graph changed; reload and retry")
			return
		}
		writeAPIError(w, err)
		return
	}
	s.auditRunGraphWrite(r, "run_graph.put", graphKey, current, graph)
	writeRunGraph(w, http.StatusOK, graph)
}

func (s *Server) handlePatchRunGraph(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil || strings.TrimSpace(actor.ID) == "" {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing actor")
		return
	}
	graphKey := strings.TrimSpace(r.PathValue("graphKey"))
	if err := rungraph.ValidateGraphKey(graphKey); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	store := rungraph.NewStore(pool)
	current, err := store.Get(r.Context(), actor.ID, graphKey)
	if err != nil {
		if rungraph.IsNotFound(err) {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "Run graph not found: "+graphKey)
			return
		}
		writeAPIError(w, err)
		return
	}
	expected, err := parseRunGraphIfMatch(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	if expected != nil && *expected != current.Revision {
		writeErr(w, http.StatusConflict, "REVISION_CONFLICT", "Run graph changed; reload and retry")
		return
	}
	patch, err := readBodyJSON(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	merged, err := rungraph.MergePatch(current.Document, patch)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	prepared, title, err := prepareRunGraphWrite(merged, graphKey)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	graph, err := store.UpsertIfRevision(r.Context(), actor.ID, graphKey, title, prepared, &current.Revision)
	if err != nil {
		if rungraph.IsRevisionConflict(err) {
			writeErr(w, http.StatusConflict, "REVISION_CONFLICT", "Run graph changed; reload and retry")
			return
		}
		writeAPIError(w, err)
		return
	}
	s.auditRunGraphWrite(r, "run_graph.patch", graphKey, current, graph)
	writeRunGraph(w, http.StatusOK, graph)
}

func writeRunGraph(w http.ResponseWriter, status int, graph *rungraph.Graph) {
	w.Header().Set("ETag", `"`+strconv.FormatInt(graph.Revision, 10)+`"`)
	writeJSON(w, status, graph)
}

func parseRunGraphIfMatch(r *http.Request) (*int64, error) {
	raw := strings.TrimSpace(r.Header.Get("If-Match"))
	if raw == "" {
		return nil, nil
	}
	// If-Match uses strong comparison; weak entity tags must never authorize a
	// write against a newer graph representation.
	if strings.HasPrefix(raw, "W/") {
		return nil, errors.New("If-Match must use a strong quoted graph revision")
	}
	if len(raw) < 3 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return nil, errors.New("If-Match must be a quoted graph revision")
	}
	revision, err := strconv.ParseInt(raw[1:len(raw)-1], 10, 64)
	if err != nil || revision < 1 {
		return nil, errors.New("If-Match must contain a positive graph revision")
	}
	return &revision, nil
}

func (s *Server) auditRunGraphWrite(r *http.Request, action, graphKey string, before, after *rungraph.Graph) {
	beforeNodes, beforeEdges := runGraphShape(before)
	afterNodes, afterEdges := runGraphShape(after)
	s.writeAudit(r, action, "", nil, map[string]any{
		"graphKey": graphKey, "revision": after.Revision, "documentBytes": len(after.Document),
		"nodeDelta": afterNodes - beforeNodes, "edgeDelta": afterEdges - beforeEdges,
	})
}

func runGraphShape(graph *rungraph.Graph) (int, int) {
	if graph == nil {
		return 0, 0
	}
	var shape struct {
		Nodes []json.RawMessage `json:"nodes"`
		Edges []json.RawMessage `json:"edges"`
	}
	if json.Unmarshal(graph.Document, &shape) != nil {
		return 0, 0
	}
	return len(shape.Nodes), len(shape.Edges)
}

func prepareRunGraphWrite(raw json.RawMessage, graphKey string) (json.RawMessage, string, error) {
	prepared, err := rungraph.PrepareDocument(raw)
	if err != nil {
		return nil, "", err
	}
	var identity struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(prepared, &identity); err != nil {
		return nil, "", err
	}
	if identity.ID != graphKey {
		return nil, "", &runGraphValidationError{message: "document.id must match path graph key"}
	}
	return prepared, identity.Title, nil
}

type runGraphValidationError struct {
	message string
}

func (e *runGraphValidationError) Error() string { return e.message }

type runGraphResolveNode struct {
	NodeID        string `json:"nodeId"`
	ObjectAPIName string `json:"objectApiName"`
	RecordID      string `json:"recordId"`
}

type runGraphResolveResult struct {
	NodeID string                   `json:"nodeId"`
	OK     bool                     `json:"ok"`
	Record dataengine.SObjectRecord `json:"record,omitempty"`
	Code   string                   `json:"code,omitempty"`
}

func (s *Server) handleResolveRunGraph(w http.ResponseWriter, r *http.Request) {
	if s.data == nil || s.objectAz == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil || strings.TrimSpace(actor.ID) == "" {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing actor")
		return
	}
	var body struct {
		Nodes      []runGraphResolveNode `json:"nodes"`
		Projection string                `json:"projection"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	if body.Projection == "" {
		body.Projection = "card"
	}
	if body.Projection != "card" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", `projection must be "card"`)
		return
	}
	if len(body.Nodes) == 0 {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "nodes must be a non-empty array")
		return
	}
	if len(body.Nodes) > rungraph.MaxResolveNodes {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "nodes exceeds max "+strconv.Itoa(rungraph.MaxResolveNodes))
		return
	}
	seen := make(map[string]struct{}, len(body.Nodes))
	for i, node := range body.Nodes {
		node.NodeID = strings.TrimSpace(node.NodeID)
		node.ObjectAPIName = strings.TrimSpace(node.ObjectAPIName)
		node.RecordID = strings.TrimSpace(node.RecordID)
		if node.NodeID == "" || node.ObjectAPIName == "" || node.RecordID == "" {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "nodes["+jsonIndex(i)+"] requires nodeId, objectApiName, and recordId")
			return
		}
		if _, duplicate := seen[node.NodeID]; duplicate {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "duplicate nodeId: "+node.NodeID)
			return
		}
		seen[node.NodeID] = struct{}{}
		if _, err := uuid.Parse(node.RecordID); err != nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "nodes["+jsonIndex(i)+"].recordId must be a UUID")
			return
		}
		body.Nodes[i] = node
	}

	viewAll, err := s.objectAz.GetViewAllObjects(r.Context(), actor)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	flsCtx := authz.ContextWithFLSCache(r.Context())
	objectAccess := make(map[string]error)
	results := make([]runGraphResolveResult, 0, len(body.Nodes))
	for _, node := range body.Nodes {
		accessErr, checked := objectAccess[node.ObjectAPIName]
		if !checked {
			accessErr = s.objectAz.AssertObjectAccess(flsCtx, actor, node.ObjectAPIName, authz.ActionRead)
			objectAccess[node.ObjectAPIName] = accessErr
		}
		result, err := s.resolveRunGraphNode(flsCtx, actor, viewAll, node, accessErr)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		results = append(results, result)
	}
	writeJSON(w, http.StatusOK, map[string]any{"nodes": results})
}

func (s *Server) resolveRunGraphNode(
	ctx context.Context,
	actor *authz.Actor,
	viewAll map[string]struct{},
	node runGraphResolveNode,
	objectAccessErr error,
) (runGraphResolveResult, error) {
	result := runGraphResolveResult{NodeID: node.NodeID}
	if objectAccessErr != nil {
		if errors.Is(objectAccessErr, authz.ErrForbidden) {
			result.Code = "FORBIDDEN"
			return result, nil
		}
		return result, objectAccessErr
	}
	record, err := s.data.Get(ctx, node.ObjectAPIName, node.RecordID)
	if err != nil {
		if errors.Is(err, dataengine.ErrNotFound) {
			result.Code = "NOT_FOUND"
			return result, nil
		}
		return result, err
	}
	ownerID, _ := record["OwnerId"].(string)
	createdByID, _ := record["CreatedById"].(string)
	ok, err := s.canViewRecord(ctx, actor, node.RecordID, ownerID, createdByID, node.ObjectAPIName, viewAll)
	if err != nil {
		return result, err
	}
	if !ok {
		result.Code = "FORBIDDEN"
		return result, nil
	}
	if s.fieldAz != nil {
		record, err = s.fieldAz.StripUnreadableFields(ctx, actor, node.ObjectAPIName, record)
		if err != nil {
			return result, err
		}
	}
	result.OK = true
	result.Record = projectRunGraphCard(record)
	return result, nil
}

var runGraphCardFields = []string{
	"Id", "attributes", "Name", "FirstName", "LastName", "Subject", "Title",
	"Status", "StageName", "Priority", "Email", "Phone", "OwnerId", "UpdatedAt",
}

func projectRunGraphCard(record dataengine.SObjectRecord) dataengine.SObjectRecord {
	card := make(dataengine.SObjectRecord, len(runGraphCardFields))
	for _, key := range runGraphCardFields {
		if value, ok := record[key]; ok {
			card[key] = value
		}
	}
	return card
}

func jsonIndex(i int) string {
	return strconv.Itoa(i)
}
