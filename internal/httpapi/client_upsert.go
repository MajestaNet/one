package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
)

func (s *Server) upsertAuthz() *dataengine.UpsertAuthz {
	az := &dataengine.UpsertAuthz{
		AssertObjectAccess:  s.objectAz.AssertObjectAccess,
		CanModifyRecord:     s.canModifyRecord,
		GetModifyAllObjects: s.objectAz.GetModifyAllObjects,
	}
	if s.fieldAz != nil {
		az.AssertEditableFields = s.fieldAz.AssertEditableFields
		az.StripUnreadableFields = s.fieldAz.StripUnreadableFields
	}
	return az
}

func (s *Server) handleGetSObjectByExternalID(w http.ResponseWriter, r *http.Request) {
	if s.data == nil || s.objectAz == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	object := r.PathValue("object")
	field, _ := url.PathUnescape(r.PathValue("externalIdField"))
	value, _ := url.PathUnescape(r.PathValue("externalId"))
	if err := s.objectAz.AssertObjectAccess(r.Context(), actor, object, authz.ActionRead); err != nil {
		writeAPIError(w, err)
		return
	}
	rec, err := s.data.GetByExternalID(r.Context(), object, field, value)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	viewAll, err := s.objectAz.GetViewAllObjects(r.Context(), actor)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	ownerID, _ := rec["OwnerId"].(string)
	createdByID, _ := rec["CreatedById"].(string)
	idVal, _ := rec["Id"].(string)
	ok, err := s.canViewRecord(r.Context(), actor, idVal, ownerID, createdByID, object, viewAll)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if !ok {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "Not allowed")
		return
	}
	if s.fieldAz != nil {
		rec, err = s.fieldAz.StripUnreadableFields(r.Context(), actor, object, rec)
		if err != nil {
			writeAPIError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleUpsertSObjectByExternalID(w http.ResponseWriter, r *http.Request) {
	if s.data == nil || s.objectAz == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	object := r.PathValue("object")
	field, _ := url.PathUnescape(r.PathValue("externalIdField"))
	value, _ := url.PathUnescape(r.PathValue("externalId"))
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	if body == nil {
		body = map[string]any{}
	}
	if err := s.assertOwnerIDWrite(r.Context(), actor, object, body); err != nil {
		writeAPIError(w, err)
		return
	}
	result, err := s.data.Upsert(r.Context(), object, field, value, body, actor, s.upsertAuthz())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	out := map[string]any{}
	for k, v := range result.Record {
		out[k] = v
	}
	out["created"] = result.Created
	if result.Created {
		writeJSON(w, http.StatusCreated, out)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteSObjectByExternalID(w http.ResponseWriter, r *http.Request) {
	if s.data == nil || s.objectAz == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	object := r.PathValue("object")
	field, _ := url.PathUnescape(r.PathValue("externalIdField"))
	value, _ := url.PathUnescape(r.PathValue("externalId"))
	if err := s.objectAz.AssertObjectAccess(r.Context(), actor, object, authz.ActionDelete); err != nil {
		writeAPIError(w, err)
		return
	}
	rec, err := s.data.GetByExternalID(r.Context(), object, field, value)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	modifyAll, err := s.objectAz.GetModifyAllObjects(r.Context(), actor)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	ownerID, _ := rec["OwnerId"].(string)
	createdByID, _ := rec["CreatedById"].(string)
	idVal, _ := rec["Id"].(string)
	ok, err := s.canModifyRecord(r.Context(), actor, idVal, ownerID, createdByID, object, modifyAll)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if !ok {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "Not allowed")
		return
	}
	if err := s.data.Delete(r.Context(), object, idVal, actor); err != nil {
		writeAPIError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUpsertSObject(w http.ResponseWriter, r *http.Request) {
	if s.data == nil || s.objectAz == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	object := r.PathValue("object")
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	if body == nil {
		body = map[string]any{}
	}
	field, _ := body["externalIdField"].(string)
	field = strings.TrimSpace(field)
	if field == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "externalIdField is required")
		return
	}
	value, ok := body["externalId"]
	if !ok {
		value = body[field]
	}
	delete(body, "externalIdField")
	delete(body, "externalId")
	if err := s.assertOwnerIDWrite(r.Context(), actor, object, body); err != nil {
		writeAPIError(w, err)
		return
	}
	result, err := s.data.Upsert(r.Context(), object, field, value, body, actor, s.upsertAuthz())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	out := map[string]any{}
	for k, v := range result.Record {
		out[k] = v
	}
	out["created"] = result.Created
	if result.Created {
		writeJSON(w, http.StatusCreated, out)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
