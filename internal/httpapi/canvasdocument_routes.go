package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/canvas"
)

func (s *Server) registerCanvasDocumentRoutes(prefix string) {
	client := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeClient, h))
	}
	s.mux.Handle("GET "+prefix+"/canvases", client(s.handleListPrincipalCanvases))
	s.mux.Handle("GET "+prefix+"/canvases/{id}", client(s.handleGetPrincipalCanvas))
	s.mux.Handle("PUT "+prefix+"/canvases/{id}", client(s.handlePutPrincipalCanvas))
	s.mux.Handle("DELETE "+prefix+"/canvases/{id}", client(s.handleDeletePrincipalCanvas))
}

func (s *Server) handleListPrincipalCanvases(w http.ResponseWriter, r *http.Request) {
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
SELECT canvas_id, title, document, updated_at
FROM principal_canvas_documents
WHERE principal_id=$1::uuid
ORDER BY updated_at DESC`, actor.ID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		var canvasID, title string
		var document []byte
		var updated time.Time
		if err := rows.Scan(&canvasID, &title, &document, &updated); err != nil {
			writeAPIError(w, err)
			return
		}
		var doc any
		_ = json.Unmarshal(document, &doc)
		list = append(list, map[string]any{
			"id":        canvasID,
			"title":     title,
			"document":  doc,
			"updatedAt": updated,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"canvases": list})
}

func (s *Server) handleGetPrincipalCanvas(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing actor")
		return
	}
	canvasID := r.PathValue("id")
	var title string
	var document []byte
	var updated time.Time
	err := pool.QueryRow(r.Context(), `
SELECT title, document, updated_at
FROM principal_canvas_documents
WHERE principal_id=$1::uuid AND canvas_id=$2`, actor.ID, canvasID).Scan(&title, &document, &updated)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "Canvas not found: "+canvasID)
			return
		}
		writeAPIError(w, err)
		return
	}
	var doc any
	_ = json.Unmarshal(document, &doc)
	writeJSON(w, http.StatusOK, map[string]any{
		"id":        canvasID,
		"title":     title,
		"document":  doc,
		"updatedAt": updated,
	})
}

func (s *Server) handlePutPrincipalCanvas(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing actor")
		return
	}
	canvasID := strings.TrimSpace(r.PathValue("id"))
	if canvasID == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "canvas id is required")
		return
	}
	body, err := readBodyJSON(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	if err := canvas.ValidateDocument(body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	var doc map[string]any
	_ = json.Unmarshal(body, &doc)
	docID, _ := doc["id"].(string)
	if docID != canvasID {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "document.id must match path id")
		return
	}
	title, _ := doc["title"].(string)
	var updated time.Time
	err = pool.QueryRow(r.Context(), `
INSERT INTO principal_canvas_documents (principal_id, canvas_id, title, document)
VALUES ($1::uuid, $2, $3, $4::jsonb)
ON CONFLICT (principal_id, canvas_id) DO UPDATE
SET title=EXCLUDED.title, document=EXCLUDED.document, updated_at=now()
RETURNING updated_at`,
		actor.ID, canvasID, title, string(body),
	).Scan(&updated)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":        canvasID,
		"title":     title,
		"document":  doc,
		"updatedAt": updated,
	})
}

func (s *Server) handleDeletePrincipalCanvas(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing actor")
		return
	}
	canvasID := r.PathValue("id")
	tag, err := pool.Exec(r.Context(), `
DELETE FROM principal_canvas_documents
WHERE principal_id=$1::uuid AND canvas_id=$2`, actor.ID, canvasID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "Canvas not found: "+canvasID)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": canvasID, "deleted": true})
}

func readBodyJSON(r *http.Request) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}
