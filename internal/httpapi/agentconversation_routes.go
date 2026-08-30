package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
)

func (s *Server) registerAgentConversationRoutes(prefix string) {
	client := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeClient, h))
	}
	s.mux.Handle("GET "+prefix+"/agents/conversations", client(s.handleListAgentConversations))
	s.mux.Handle("POST "+prefix+"/agents/conversations", client(s.handleCreateAgentConversation))
	s.mux.Handle("GET "+prefix+"/agents/conversations/{id}", client(s.handleGetAgentConversation))
	s.mux.Handle("PATCH "+prefix+"/agents/conversations/{id}", client(s.handlePatchAgentConversation))
	s.mux.Handle("POST "+prefix+"/agents/conversations/{id}/messages", client(s.handleAppendAgentConversationMessages))
	s.mux.Handle("GET "+prefix+"/preferences", client(s.handleListPrincipalPreferences))
	s.mux.Handle("GET "+prefix+"/preferences/{kind}", client(s.handleGetPrincipalPreference))
	s.mux.Handle("PUT "+prefix+"/preferences/{kind}", client(s.handlePutPrincipalPreference))
	s.mux.Handle("DELETE "+prefix+"/preferences/{kind}", client(s.handleDeletePrincipalPreference))
}

func (s *Server) handleListAgentConversations(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing actor")
		return
	}
	rows, err := pool.Query(r.Context(), `
SELECT id::text, playbook_api_name, mode, title, created_at, updated_at
FROM agent_conversations
WHERE principal_id=$1::uuid
ORDER BY updated_at DESC
LIMIT 200`, actor.ID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		var id, mode, title string
		var playbook *string
		var created, updated time.Time
		if err := rows.Scan(&id, &playbook, &mode, &title, &created, &updated); err != nil {
			writeAPIError(w, err)
			return
		}
		row := map[string]any{
			"id":        id,
			"mode":      mode,
			"title":     title,
			"createdAt": created,
			"updatedAt": updated,
		}
		if playbook != nil {
			row["playbookApiName"] = *playbook
		}
		list = append(list, row)
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": list})
}

func (s *Server) handleCreateAgentConversation(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing actor")
		return
	}
	var body struct {
		PlaybookAPIName *string `json:"playbookApiName"`
		Mode            string  `json:"mode"`
		Title           string  `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	mode := strings.TrimSpace(body.Mode)
	if mode == "" {
		mode = "operate"
	}
	title := strings.TrimSpace(body.Title)
	if title == "" {
		title = "Agent chat"
	}
	var id string
	var created, updated time.Time
	err := pool.QueryRow(r.Context(), `
INSERT INTO agent_conversations (principal_id, playbook_api_name, mode, title)
VALUES ($1::uuid, $2, $3, $4)
RETURNING id::text, created_at, updated_at`,
		actor.ID, body.PlaybookAPIName, mode, title,
	).Scan(&id, &created, &updated)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":              id,
		"playbookApiName": body.PlaybookAPIName,
		"mode":            mode,
		"title":           title,
		"createdAt":       created,
		"updatedAt":       updated,
	})
}

func (s *Server) handleGetAgentConversation(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing actor")
		return
	}
	convID := r.PathValue("id")
	conv, err := s.loadAgentConversation(r.Context(), pool, actor.ID, convID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "Conversation not found")
			return
		}
		writeAPIError(w, err)
		return
	}
	messages, err := s.loadAgentConversationMessages(r.Context(), pool, convID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	conv["messages"] = messages
	writeJSON(w, http.StatusOK, conv)
}

func (s *Server) handlePatchAgentConversation(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing actor")
		return
	}
	convID := r.PathValue("id")
	var body struct {
		Title *string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	if body.Title == nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "title is required")
		return
	}
	tag, err := pool.Exec(r.Context(), `
UPDATE agent_conversations SET title=$3, updated_at=now()
WHERE id=$1::uuid AND principal_id=$2::uuid`, convID, actor.ID, strings.TrimSpace(*body.Title))
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "Conversation not found")
		return
	}
	conv, err := s.loadAgentConversation(r.Context(), pool, actor.ID, convID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, conv)
}

func (s *Server) handleAppendAgentConversationMessages(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing actor")
		return
	}
	convID := r.PathValue("id")
	var body struct {
		Messages []map[string]any `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	if len(body.Messages) == 0 {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "messages is required")
		return
	}
	var owner string
	err := pool.QueryRow(r.Context(), `
SELECT principal_id::text FROM agent_conversations WHERE id=$1::uuid`, convID).Scan(&owner)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "Conversation not found")
			return
		}
		writeAPIError(w, err)
		return
	}
	if owner != actor.ID {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "Conversation not found")
		return
	}
	inserted := []map[string]any{}
	for _, msg := range body.Messages {
		role, _ := msg["role"].(string)
		if role == "" {
			continue
		}
		bodyText, _ := msg["body"].(string)
		partsJSON, _ := json.Marshal(msg["parts"])
		if partsJSON == nil {
			partsJSON = []byte("[]")
		}
		var runID *string
		if v, ok := msg["runId"].(string); ok && v != "" {
			runID = &v
		}
		var id string
		var created time.Time
		err := pool.QueryRow(r.Context(), `
INSERT INTO agent_conversation_messages (conversation_id, role, body, parts, run_id)
VALUES ($1::uuid, $2, $3, $4::jsonb, $5::uuid)
RETURNING id::text, created_at`,
			convID, role, bodyText, string(partsJSON), runID,
		).Scan(&id, &created)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		var parts any
		_ = json.Unmarshal(partsJSON, &parts)
		row := map[string]any{
			"id":        id,
			"role":      role,
			"body":      bodyText,
			"parts":     parts,
			"createdAt": created,
		}
		if runID != nil {
			row["runId"] = *runID
		}
		inserted = append(inserted, row)
	}
	_, _ = pool.Exec(r.Context(), `UPDATE agent_conversations SET updated_at=now() WHERE id=$1::uuid`, convID)
	writeJSON(w, http.StatusOK, map[string]any{"messages": inserted})
}

func (s *Server) loadAgentConversation(ctx context.Context, pool *db.Pool, principalID, convID string) (map[string]any, error) {
	var id, mode, title string
	var playbook *string
	var created, updated time.Time
	err := pool.QueryRow(ctx, `
SELECT id::text, playbook_api_name, mode, title, created_at, updated_at
FROM agent_conversations
WHERE id=$1::uuid AND principal_id=$2::uuid`, convID, principalID).Scan(&id, &playbook, &mode, &title, &created, &updated)
	if err != nil {
		return nil, err
	}
	row := map[string]any{
		"id":        id,
		"mode":      mode,
		"title":     title,
		"createdAt": created,
		"updatedAt": updated,
	}
	if playbook != nil {
		row["playbookApiName"] = *playbook
	}
	return row, nil
}

func (s *Server) loadAgentConversationMessages(ctx context.Context, pool *db.Pool, convID string) ([]map[string]any, error) {
	rows, err := pool.Query(ctx, `
SELECT id::text, role, body, parts, run_id::text, created_at
FROM agent_conversation_messages
WHERE conversation_id=$1::uuid
ORDER BY created_at ASC`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		var id, role, bodyText string
		var parts []byte
		var runID *string
		var created time.Time
		if err := rows.Scan(&id, &role, &bodyText, &parts, &runID, &created); err != nil {
			return nil, err
		}
		var partsAny any
		_ = json.Unmarshal(parts, &partsAny)
		row := map[string]any{
			"id":        id,
			"role":      role,
			"body":      bodyText,
			"parts":     partsAny,
			"createdAt": created,
		}
		if runID != nil {
			row["runId"] = *runID
		}
		list = append(list, row)
	}
	return list, nil
}

func (s *Server) handleListPrincipalPreferences(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing actor")
		return
	}
	rows, err := pool.Query(r.Context(), `
SELECT kind, document, updated_at
FROM principal_preferences
WHERE principal_id=$1::uuid
ORDER BY kind`, actor.ID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		var kind string
		var document []byte
		var updated time.Time
		if err := rows.Scan(&kind, &document, &updated); err != nil {
			writeAPIError(w, err)
			return
		}
		var doc any
		_ = json.Unmarshal(document, &doc)
		list = append(list, map[string]any{
			"kind":      kind,
			"document":  doc,
			"updatedAt": updated,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"preferences": list})
}

func (s *Server) handleGetPrincipalPreference(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing actor")
		return
	}
	kind := strings.TrimSpace(r.PathValue("kind"))
	var document []byte
	var updated time.Time
	err := pool.QueryRow(r.Context(), `
SELECT document, updated_at
FROM principal_preferences
WHERE principal_id=$1::uuid AND kind=$2`, actor.ID, kind).Scan(&document, &updated)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "Preference not found: "+kind)
			return
		}
		writeAPIError(w, err)
		return
	}
	var doc any
	_ = json.Unmarshal(document, &doc)
	writeJSON(w, http.StatusOK, map[string]any{
		"kind":      kind,
		"document":  doc,
		"updatedAt": updated,
	})
}

func (s *Server) handlePutPrincipalPreference(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing actor")
		return
	}
	kind := strings.TrimSpace(r.PathValue("kind"))
	if kind == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "kind is required")
		return
	}
	body, err := readBodyJSON(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	var updated time.Time
	err = pool.QueryRow(r.Context(), `
INSERT INTO principal_preferences (principal_id, kind, document)
VALUES ($1::uuid, $2, $3::jsonb)
ON CONFLICT (principal_id, kind) DO UPDATE
SET document=EXCLUDED.document, updated_at=now()
RETURNING updated_at`,
		actor.ID, kind, string(body),
	).Scan(&updated)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	var doc any
	_ = json.Unmarshal(body, &doc)
	writeJSON(w, http.StatusOK, map[string]any{
		"kind":      kind,
		"document":  doc,
		"updatedAt": updated,
	})
}

func (s *Server) handleDeletePrincipalPreference(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing actor")
		return
	}
	kind := r.PathValue("kind")
	tag, err := pool.Exec(r.Context(), `
DELETE FROM principal_preferences
WHERE principal_id=$1::uuid AND kind=$2`, actor.ID, kind)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "Preference not found: "+kind)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"kind": kind, "deleted": true})
}
