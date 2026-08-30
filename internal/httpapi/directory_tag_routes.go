package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
)

func (s *Server) registerDirectoryTagRoutes(prefix string, capClient func(string, http.HandlerFunc) http.Handler) {
	s.mux.Handle("POST "+prefix+"/directory-tags", capClient(authz.CapIdentityUsers, s.handleCreateDirectoryTag))
	s.mux.Handle("GET "+prefix+"/directory-tags", capClient(authz.CapIdentityUsers, s.handleListDirectoryTags))
	s.mux.Handle("GET "+prefix+"/directory-tags/{apiName}", capClient(authz.CapIdentityUsers, s.handleGetDirectoryTag))
	s.mux.Handle("PATCH "+prefix+"/directory-tags/{apiName}", capClient(authz.CapIdentityUsers, s.handlePatchDirectoryTag))
	s.mux.Handle("DELETE "+prefix+"/directory-tags/{apiName}", capClient(authz.CapIdentityUsers, s.handleDeleteDirectoryTag))
	s.mux.Handle("GET "+prefix+"/directory-tags/{apiName}/members", capClient(authz.CapIdentityUsers, s.handleListDirectoryTagMembers))
	s.mux.Handle("POST "+prefix+"/directory-tags/assign", capClient(authz.CapIdentityUsers, s.handleAssignDirectoryTag))
	s.mux.Handle("POST "+prefix+"/directory-tags/unassign", capClient(authz.CapIdentityUsers, s.handleUnassignDirectoryTag))
}

func directoryTagJSON(t *db.DirectoryTag) map[string]any {
	out := map[string]any{
		"id":          t.ID,
		"apiName":     t.APIName,
		"label":       t.DisplayName,
		"memberCount": t.MemberCount,
		"createdAt":   t.CreatedAt,
		"updatedAt":   t.UpdatedAt,
	}
	putStringPtr(out, "externalId", t.ExternalID)
	putStringPtr(out, "description", t.Description)
	return out
}

func writeDirectoryTagNotFound(w http.ResponseWriter, err error) {
	if errors.Is(err, db.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "DIRECTORY_TAG_NOT_FOUND", "directory tag not found")
		return
	}
	writeAPIError(w, err)
}

func (s *Server) handleCreateDirectoryTag(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		APIName     string `json:"apiName"`
		Label       string `json:"label"`
		ExternalID  string `json:"externalId"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	store := db.NewDirectoryTagStore(pool)
	in := db.CreateDirectoryTagInput{
		APIName:     strings.TrimSpace(body.APIName),
		DisplayName: strings.TrimSpace(body.Label),
		ExternalID:  strings.TrimSpace(body.ExternalID),
		Description: strings.TrimSpace(body.Description),
		AutoAPIName: strings.TrimSpace(body.APIName) == "",
	}
	tag, err := store.Create(r.Context(), in)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "identity.tag.create", "", nil, map[string]any{
		"id": tag.ID, "apiName": tag.APIName,
	})
	writeJSON(w, http.StatusCreated, directoryTagJSON(tag))
}

func (s *Server) handleListDirectoryTags(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	store := db.NewDirectoryTagStore(pool)
	list, err := store.List(r.Context(), db.ListDirectoryTagsFilter{})
	if err != nil {
		writeAPIError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for i := range list {
		out = append(out, directoryTagJSON(&list[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"tags": out})
}

func (s *Server) handleGetDirectoryTag(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	store := db.NewDirectoryTagStore(pool)
	tag, err := store.GetByAPIName(r.Context(), r.PathValue("apiName"))
	if err != nil {
		writeDirectoryTagNotFound(w, err)
		return
	}
	writeJSON(w, http.StatusOK, directoryTagJSON(tag))
}

func (s *Server) handlePatchDirectoryTag(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		Label       *string `json:"label"`
		ExternalID  *string `json:"externalId"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	store := db.NewDirectoryTagStore(pool)
	tag, err := store.GetByAPIName(r.Context(), r.PathValue("apiName"))
	if err != nil {
		writeDirectoryTagNotFound(w, err)
		return
	}
	in := db.UpdateDirectoryTagInput{
		ExternalID:  body.ExternalID,
		Description: body.Description,
	}
	if body.Label != nil {
		in.DisplayName = body.Label
	}
	tag, err = store.Update(r.Context(), tag.ID, in)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "identity.tag.update", "", nil, map[string]any{
		"id": tag.ID, "apiName": tag.APIName,
	})
	writeJSON(w, http.StatusOK, directoryTagJSON(tag))
}

func (s *Server) handleDeleteDirectoryTag(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	store := db.NewDirectoryTagStore(pool)
	tag, err := store.GetByAPIName(r.Context(), r.PathValue("apiName"))
	if err != nil {
		writeDirectoryTagNotFound(w, err)
		return
	}
	if err := store.Delete(r.Context(), tag.ID); err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "identity.tag.delete", "", nil, map[string]any{
		"id": tag.ID, "apiName": tag.APIName,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListDirectoryTagMembers(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	store := db.NewDirectoryTagStore(pool)
	tag, err := store.GetByAPIName(r.Context(), r.PathValue("apiName"))
	if err != nil {
		writeDirectoryTagNotFound(w, err)
		return
	}
	limit := 200
	offset := 0
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := strings.TrimSpace(r.URL.Query().Get("offset")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	members, total, err := store.ListMembers(r.Context(), tag.ID, limit, offset)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	users := db.NewUserStore(pool)
	actor := ActorFromContext(r.Context())
	out := make([]map[string]any, 0, len(members))
	for _, m := range members {
		if !s.canReadPrincipalType(r, m.PrincipalType) {
			continue
		}
		u, err := users.GetByID(r.Context(), m.UserID)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		roles, psNames, err := principalGrantNames(r, users, u.ID)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		item, err := s.principalJSONForActor(r.Context(), actor, u, roles, psNames, false)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": out, "totalSize": total})
}

func (s *Server) handleAssignDirectoryTag(w http.ResponseWriter, r *http.Request) {
	s.handleDirectoryTagMembership(w, r, true)
}

func (s *Server) handleUnassignDirectoryTag(w http.ResponseWriter, r *http.Request) {
	s.handleDirectoryTagMembership(w, r, false)
}

func (s *Server) handleDirectoryTagMembership(w http.ResponseWriter, r *http.Request, assign bool) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		PrincipalID string `json:"principalId"`
		TagAPIName  string `json:"tagApiName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	principalID := strings.TrimSpace(body.PrincipalID)
	tagAPIName := strings.TrimSpace(body.TagAPIName)
	if principalID == "" || tagAPIName == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "principalId and tagApiName are required")
		return
	}
	users := db.NewUserStore(pool)
	u, err := users.GetByID(r.Context(), principalID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if !s.assertClientMemberCapability(w, r, u.PrincipalType) {
		return
	}
	tags := db.NewDirectoryTagStore(pool)
	tag, err := tags.GetByAPIName(r.Context(), tagAPIName)
	if err != nil {
		writeDirectoryTagNotFound(w, err)
		return
	}
	if assign {
		if err := tags.Assign(r.Context(), u.ID, tag.ID); err != nil {
			writeAPIError(w, err)
			return
		}
		s.writeAudit(r, "identity.tag.assign", "", nil, map[string]any{
			"id": tag.ID, "apiName": tag.APIName, "principalId": u.ID,
		})
	} else {
		if err := tags.Unassign(r.Context(), u.ID, tag.ID); err != nil {
			writeAPIError(w, err)
			return
		}
		s.writeAudit(r, "identity.tag.unassign", "", nil, map[string]any{
			"id": tag.ID, "apiName": tag.APIName, "principalId": u.ID,
		})
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) assertClientMemberCapability(w http.ResponseWriter, r *http.Request, principalType string) bool {
	cap := principalTypeCapability(principalType)
	if cap == authz.CapIdentityUsers {
		return true
	}
	actor := ActorFromContext(r.Context())
	if actor == nil || (!authz.HasAdminPrivilege(actor) && (s.systemAz == nil || s.systemAz.AssertCapability(r.Context(), actor, cap) != nil)) {
		writeErr(w, http.StatusForbidden, "CAPABILITY_REQUIRED", "capability "+cap+" required")
		return false
	}
	return true
}

func (s *Server) replacePrincipalDirectoryTags(w http.ResponseWriter, r *http.Request, userID, principalType string, apiNames []string) bool {
	if !s.assertClientMemberCapability(w, r, principalType) {
		return false
	}
	pool := s.pool
	if pool == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "database not configured")
		return false
	}
	if err := db.NewDirectoryTagStore(pool).ReplaceUserTagsByAPINames(r.Context(), userID, apiNames); err != nil {
		if errors.Is(err, db.ErrNotFound) && strings.Contains(err.Error(), "DIRECTORY_TAG_NOT_FOUND") {
			writeErr(w, http.StatusNotFound, "DIRECTORY_TAG_NOT_FOUND", err.Error())
			return false
		}
		writeAPIError(w, err)
		return false
	}
	return true
}

func parseDirectoryTagAPINames(raw map[string]any) (names []string, set bool, ok bool) {
	v, exists := raw["directoryTagApiNames"]
	if !exists {
		return nil, false, true
	}
	switch t := v.(type) {
	case nil:
		return []string{}, true, true
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			s, isStr := item.(string)
			if !isStr {
				return nil, true, false
			}
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out, true, true
	default:
		return nil, true, false
	}
}
