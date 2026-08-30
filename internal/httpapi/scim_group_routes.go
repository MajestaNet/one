package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/scim"
)

func (s *Server) registerSCIMGroupRoutes(authClient func(http.HandlerFunc) http.Handler) {
	s.mux.Handle("POST /scim/v2/Groups", authClient(s.handleSCIMCreateGroup))
	s.mux.Handle("GET /scim/v2/Groups", authClient(s.handleSCIMListGroups))
	s.mux.Handle("GET /scim/v2/Groups/{id}", authClient(s.handleSCIMGetGroup))
	s.mux.Handle("PUT /scim/v2/Groups/{id}", authClient(s.handleSCIMReplaceGroup))
	s.mux.Handle("PATCH /scim/v2/Groups/{id}", authClient(s.handleSCIMPatchGroup))
	s.mux.Handle("DELETE /scim/v2/Groups/{id}", authClient(s.handleSCIMDeleteGroup))
}

func (s *Server) handleSCIMCreateGroup(w http.ResponseWriter, r *http.Request) {
	if !s.assertSCIMCapability(w, r, authz.CapIdentityUsers) {
		return
	}
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body scim.Group
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalidSyntax", "invalid JSON body")
		return
	}
	display := strings.TrimSpace(body.DisplayName)
	if display == "" {
		writeSCIMError(w, http.StatusBadRequest, "invalidValue", "displayName is required")
		return
	}
	memberIDs, err := body.MemberUserIDs()
	if err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalidValue", err.Error())
		return
	}
	if !s.assertSCIMMemberChanges(w, r, pool, nil, memberIDs) {
		return
	}
	store := db.NewDirectoryTagStore(pool)
	tag, err := store.Create(r.Context(), db.CreateDirectoryTagInput{
		DisplayName: display,
		ExternalID:  strings.TrimSpace(body.ExternalID),
		AutoAPIName: true,
	})
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	if err := store.ReplaceMembers(r.Context(), tag.ID, memberIDs); err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	s.writeAudit(r, "scim.group.create", "", nil, map[string]any{
		"id": tag.ID, "apiName": tag.APIName, "displayName": tag.DisplayName,
	})
	s.writeSCIMGroup(w, r, store, tag, http.StatusCreated)
}

func (s *Server) handleSCIMGetGroup(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	store := db.NewDirectoryTagStore(pool)
	tag, err := store.GetByID(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	s.writeSCIMGroup(w, r, store, tag, http.StatusOK)
}

func (s *Server) handleSCIMListGroups(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	filt, err := scim.ParseGroupFilter(r.URL.Query().Get("filter"))
	if err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalidFilter", err.Error())
		return
	}
	store := db.NewDirectoryTagStore(pool)
	list, err := store.List(r.Context(), db.ListDirectoryTagsFilter{
		DisplayName: filt.DisplayName,
		ExternalID:  filt.ExternalID,
	})
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	startIndex := 1
	if v := r.URL.Query().Get("startIndex"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			startIndex = n
		}
	}
	count := 100
	if v := r.URL.Query().Get("count"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			if n < 0 {
				n = 0
			}
			if n > 200 {
				n = 200
			}
			count = n
		}
	}
	resources := make([]scim.Group, 0, len(list))
	for i := range list {
		g, err := s.scimGroupFromTag(r, store, &list[i])
		if err != nil {
			s.writeSCIMStoreError(w, err)
			return
		}
		resources = append(resources, g)
	}
	total := len(resources)
	from := startIndex - 1
	if from < 0 {
		from = 0
	}
	if from > total {
		from = total
	}
	to := from + count
	if to > total {
		to = total
	}
	page := resources[from:to]
	writeSCIMJSON(w, http.StatusOK, scim.GroupListResponse{
		Schemas:      []string{scim.SchemaListResponse},
		TotalResults: total,
		StartIndex:   startIndex,
		ItemsPerPage: len(page),
		Resources:    page,
	})
}

func (s *Server) handleSCIMReplaceGroup(w http.ResponseWriter, r *http.Request) {
	if !s.assertSCIMCapability(w, r, authz.CapIdentityUsers) {
		return
	}
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	id := r.PathValue("id")
	var body scim.Group
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalidSyntax", "invalid JSON body")
		return
	}
	store := db.NewDirectoryTagStore(pool)
	if _, err := store.GetByID(r.Context(), id); err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	existingMembers, err := store.ListAllMembers(r.Context(), id)
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	existingIDs := make([]string, 0, len(existingMembers))
	for _, m := range existingMembers {
		existingIDs = append(existingIDs, m.UserID)
	}
	display := strings.TrimSpace(body.DisplayName)
	if display == "" {
		writeSCIMError(w, http.StatusBadRequest, "invalidValue", "displayName is required")
		return
	}
	ext := strings.TrimSpace(body.ExternalID)
	var memberIDs []string
	if body.Members != nil {
		memberIDs, err = body.MemberUserIDs()
		if err != nil {
			writeSCIMError(w, http.StatusBadRequest, "invalidValue", err.Error())
			return
		}
		if !s.assertSCIMMemberChanges(w, r, pool, existingIDs, memberIDs) {
			return
		}
	}
	tag, err := store.Update(r.Context(), id, db.UpdateDirectoryTagInput{
		DisplayName: &display,
		ExternalID:  &ext,
	})
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	if body.Members != nil {
		if err := store.ReplaceMembers(r.Context(), id, memberIDs); err != nil {
			s.writeSCIMStoreError(w, err)
			return
		}
	}
	s.writeAudit(r, "scim.group.update", "", nil, map[string]any{
		"id": tag.ID, "apiName": tag.APIName,
	})
	s.writeSCIMGroup(w, r, store, tag, http.StatusOK)
}

func (s *Server) handleSCIMPatchGroup(w http.ResponseWriter, r *http.Request) {
	if !s.assertSCIMCapability(w, r, authz.CapIdentityUsers) {
		return
	}
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	id := r.PathValue("id")
	var req scim.PatchRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalidSyntax", "invalid JSON body")
		return
	}
	store := db.NewDirectoryTagStore(pool)
	tag, err := store.GetByID(r.Context(), id)
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	members, err := store.ListAllMembers(r.Context(), id)
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	cur := scim.ToGroup(tag, members, s.scimLocationBase(r))
	if err := scim.ApplyGroupPatch(&cur, req.Operations); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalidPath", err.Error())
		return
	}
	display := strings.TrimSpace(cur.DisplayName)
	if display == "" {
		writeSCIMError(w, http.StatusBadRequest, "invalidValue", "displayName is required")
		return
	}
	ext := strings.TrimSpace(cur.ExternalID)
	memberIDs, err := cur.MemberUserIDs()
	if err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalidValue", err.Error())
		return
	}
	beforeIDs := make([]string, 0, len(members))
	for _, m := range members {
		beforeIDs = append(beforeIDs, m.UserID)
	}
	if !s.assertSCIMMemberChanges(w, r, pool, beforeIDs, memberIDs) {
		return
	}
	tag, err = store.Update(r.Context(), id, db.UpdateDirectoryTagInput{
		DisplayName: &display,
		ExternalID:  &ext,
	})
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	if err := store.ReplaceMembers(r.Context(), id, memberIDs); err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	s.writeAudit(r, "scim.group.update", "", nil, map[string]any{
		"id": tag.ID, "apiName": tag.APIName,
	})
	s.writeSCIMGroup(w, r, store, tag, http.StatusOK)
}

func (s *Server) handleSCIMDeleteGroup(w http.ResponseWriter, r *http.Request) {
	if !s.assertSCIMCapability(w, r, authz.CapIdentityUsers) {
		return
	}
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	id := r.PathValue("id")
	store := db.NewDirectoryTagStore(pool)
	tag, err := store.GetByID(r.Context(), id)
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	if err := store.Delete(r.Context(), id); err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	s.writeAudit(r, "scim.group.delete", "", nil, map[string]any{
		"id": id, "apiName": tag.APIName,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) assertSCIMMemberChanges(w http.ResponseWriter, r *http.Request, pool *db.Pool, before, after []string) bool {
	beforeSet := map[string]struct{}{}
	for _, id := range before {
		id = strings.TrimSpace(id)
		if id != "" {
			beforeSet[id] = struct{}{}
		}
	}
	afterSet := map[string]struct{}{}
	for _, id := range after {
		id = strings.TrimSpace(id)
		if id != "" {
			afterSet[id] = struct{}{}
		}
	}
	changed := map[string]struct{}{}
	for id := range afterSet {
		if _, ok := beforeSet[id]; !ok {
			changed[id] = struct{}{}
		}
	}
	for id := range beforeSet {
		if _, ok := afterSet[id]; !ok {
			changed[id] = struct{}{}
		}
	}
	if len(changed) == 0 {
		return true
	}
	users := db.NewUserStore(pool)
	for id := range changed {
		u, err := users.GetByID(r.Context(), id)
		if err != nil {
			s.writeSCIMStoreError(w, err)
			return false
		}
		if !s.assertSCIMCapability(w, r, principalTypeCapability(u.PrincipalType)) {
			return false
		}
	}
	return true
}

func (s *Server) scimGroupFromTag(r *http.Request, store *db.DirectoryTagStore, tag *db.DirectoryTag) (scim.Group, error) {
	members, _, err := store.ListMembers(r.Context(), tag.ID, 200, 0)
	if err != nil {
		return scim.Group{}, err
	}
	visible := make([]db.DirectoryTagMember, 0, len(members))
	for _, m := range members {
		if s.canReadPrincipalType(r, m.PrincipalType) {
			visible = append(visible, m)
		}
	}
	return scim.ToGroup(tag, visible, s.scimLocationBase(r)), nil
}

func (s *Server) writeSCIMGroup(w http.ResponseWriter, r *http.Request, store *db.DirectoryTagStore, tag *db.DirectoryTag, status int) {
	g, err := s.scimGroupFromTag(r, store, tag)
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	writeSCIMJSON(w, status, g)
}
