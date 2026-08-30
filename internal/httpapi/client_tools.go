package httpapi

import (
	"net/http"
)

func (s *Server) handleListClientTools(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	if s.toolAz == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "tool authorization not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	rows, err := pool.Query(r.Context(), `
SELECT c.api_name, c.label, c.description, c.icon, c.sort_order
FROM metadata_canvases c
WHERE c.active = true
  AND (
    c.ownership = 'custom'
    OR c.package_name IN ('customer.default', 'core')
    OR EXISTS (
      SELECT 1 FROM package_installs p
      WHERE p.package_name = c.package_name AND p.enabled = true
    )
  )
ORDER BY c.sort_order ASC, c.label ASC, c.api_name ASC`)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var apiName, label, description, icon string
		var sortOrder int
		if err := rows.Scan(&apiName, &label, &description, &icon, &sortOrder); err != nil {
			writeAPIError(w, err)
			return
		}
		access, err := s.toolAz.ActorToolAccess(r.Context(), actor, apiName)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		if !access.CanOpen {
			continue
		}
		out = append(out, map[string]any{
			"apiName":     apiName,
			"label":       label,
			"description": description,
			"icon":        icon,
			"sortOrder":   sortOrder,
			"permissions": access,
		})
	}
	if err := rows.Err(); err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tools": out})
}

func (s *Server) handleGetClientTool(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	if s.toolAz == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "tool authorization not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	apiName := r.PathValue("apiName")
	m, err := loadCanvasSpec(r.Context(), pool, apiName)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "ToolSpec not found: "+apiName)
		return
	}
	if active, _ := m["active"].(bool); !active {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "ToolSpec not found: "+apiName)
		return
	}
	if !canvasSpecVisible(r.Context(), pool, m) {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "ToolSpec not found: "+apiName)
		return
	}
	access, err := s.toolAz.ActorToolAccess(r.Context(), actor, apiName)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if !access.CanOpen {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "ToolSpec not found: "+apiName)
		return
	}
	m["permissions"] = access
	writeJSON(w, http.StatusOK, m)
}
