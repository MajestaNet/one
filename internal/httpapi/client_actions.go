package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/MajestaNet/ide/internal/actions"
)

func (s *Server) actionsOrErr(w http.ResponseWriter) bool {
	if s.actions == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "platform actions not configured")
		return false
	}
	return true
}

func (s *Server) handleListActions(w http.ResponseWriter, r *http.Request) {
	if !s.actionsOrErr(w) {
		return
	}
	list, err := s.actions.Catalog(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if list == nil {
		list = []actions.CatalogItem{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"actions": list})
}

func (s *Server) handleGetAction(w http.ResponseWriter, r *http.Request) {
	if !s.actionsOrErr(w) {
		return
	}
	apiName := strings.TrimSpace(r.PathValue("apiName"))
	if apiName == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_FAILED", "apiName required")
		return
	}
	desc, err := s.actions.Describe(r.Context(), apiName)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, desc)
}

func (s *Server) handleInvokeAction(w http.ResponseWriter, r *http.Request) {
	if !s.actionsOrErr(w) {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	apiName := strings.TrimSpace(r.PathValue("apiName"))
	if apiName == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_FAILED", "apiName required")
		return
	}
	var body struct {
		Input map[string]any `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, "VALIDATION_FAILED", "Invalid JSON body")
		return
	}
	if body.Input == nil {
		body.Input = map[string]any{}
	}
	result, err := s.actions.Invoke(r.Context(), actor, apiName, body.Input)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
