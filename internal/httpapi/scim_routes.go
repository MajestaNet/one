package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/scim"
)

func (s *Server) registerSCIMRoutes() {
	authClient := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeClient, h))
	}
	s.mux.Handle("GET /scim/v2/ServiceProviderConfig", authClient(s.handleSCIMServiceProviderConfig))
	s.mux.Handle("GET /scim/v2/Schemas", authClient(s.handleSCIMSchemas))
	s.mux.Handle("GET /scim/v2/Schemas/{id}", authClient(s.handleSCIMSchema))
	s.mux.Handle("GET /scim/v2/ResourceTypes", authClient(s.handleSCIMResourceTypes))
	s.mux.Handle("GET /scim/v2/ResourceTypes/{id}", authClient(s.handleSCIMResourceType))
	s.mux.Handle("POST /scim/v2/Users", authClient(s.handleSCIMCreateUser))
	s.mux.Handle("GET /scim/v2/Users", authClient(s.handleSCIMListUsers))
	s.mux.Handle("GET /scim/v2/Users/{id}", authClient(s.handleSCIMGetUser))
	s.mux.Handle("PUT /scim/v2/Users/{id}", authClient(s.handleSCIMReplaceUser))
	s.mux.Handle("PATCH /scim/v2/Users/{id}", authClient(s.handleSCIMPatchUser))
	s.mux.Handle("DELETE /scim/v2/Users/{id}", authClient(s.handleSCIMDeleteUser))
	s.registerSCIMGroupRoutes(authClient)
}

func writeSCIMJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeSCIMError(w http.ResponseWriter, status int, scimType, detail string) {
	writeSCIMJSON(w, status, scim.NewError(status, scimType, detail))
}

func (s *Server) scimLocationBase(r *http.Request) string {
	return "/scim/v2"
}

func (s *Server) assertSCIMCapability(w http.ResponseWriter, r *http.Request, cap string) bool {
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeSCIMError(w, http.StatusForbidden, "forbidden", "capability "+cap+" required")
		return false
	}
	if authz.HasAdminPrivilege(actor) {
		return true
	}
	if s.systemAz == nil {
		writeSCIMError(w, http.StatusForbidden, "forbidden", "capability "+cap+" required")
		return false
	}
	if err := s.systemAz.AssertCapability(r.Context(), actor, cap); err != nil {
		writeSCIMError(w, http.StatusForbidden, "forbidden", "capability "+cap+" required")
		return false
	}
	return true
}

func principalTypeCapability(pt string) string {
	switch pt {
	case "service", "agent":
		return authz.CapIdentityIntegrations
	default:
		return authz.CapIdentityUsers
	}
}

func (s *Server) handleSCIMServiceProviderConfig(w http.ResponseWriter, _ *http.Request) {
	writeSCIMJSON(w, http.StatusOK, scim.ServiceProviderConfig())
}

func (s *Server) handleSCIMSchemas(w http.ResponseWriter, r *http.Request) {
	schemas := scim.Schemas(s.scimCustomAttributes(r.Context()))
	writeSCIMJSON(w, http.StatusOK, map[string]any{
		"schemas":      []string{scim.SchemaListResponse},
		"totalResults": len(schemas),
		"Resources":    schemas,
	})
}

func (s *Server) handleSCIMSchema(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sch, ok := scim.FindSchema(id, s.scimCustomAttributes(r.Context()))
	if !ok {
		writeSCIMError(w, http.StatusNotFound, "invalidValue", "schema not found")
		return
	}
	writeSCIMJSON(w, http.StatusOK, sch)
}

func (s *Server) handleSCIMResourceTypes(w http.ResponseWriter, _ *http.Request) {
	types := scim.ResourceTypes()
	writeSCIMJSON(w, http.StatusOK, map[string]any{
		"schemas":      []string{scim.SchemaListResponse},
		"totalResults": len(types),
		"Resources":    types,
	})
}

func (s *Server) handleSCIMResourceType(w http.ResponseWriter, r *http.Request) {
	rt, ok := scim.FindResourceType(r.PathValue("id"))
	if !ok {
		writeSCIMError(w, http.StatusNotFound, "invalidValue", "resource type not found")
		return
	}
	writeSCIMJSON(w, http.StatusOK, rt)
}

func (s *Server) handleSCIMCreateUser(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body scim.User
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalidSyntax", "invalid JSON body")
		return
	}
	if body.Groups != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalidValue", "membership is managed on /Groups")
		return
	}
	prov, err := s.loadProvisioning(r.Context())
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	in, err := scim.ToCreateInput(body, scimDefaultRole(body, prov))
	if err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalidValue", err.Error())
		return
	}
	scimEffectivePermissionSets(body, &in, prov)
	if !s.assertSCIMCapability(w, r, principalTypeCapability(in.PrincipalType)) {
		return
	}
	if len(in.RoleAPINames) > 0 || len(in.PermissionSetAPINames) > 0 || scimEffectiveDataRole(body, prov) != "" {
		if !s.assertSCIMCapability(w, r, authz.CapAuthzManage) {
			return
		}
	}
	actor := ActorFromContext(r.Context())
	custom, err := s.applySCIMCustomData(r.Context(), actor, body.Custom, "create")
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	in.Data = custom
	store := db.NewUserStore(pool)
	u, err := store.CreateWithGrants(r.Context(), in)
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	if err := s.assignDataRoleByAPIName(r.Context(), u.ID, scimEffectiveDataRole(body, prov)); err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	if body.Active != nil && !*body.Active {
		u, err = store.DeactivatePrincipal(r.Context(), u.ID)
		if err != nil {
			s.writeSCIMStoreError(w, err)
			return
		}
	}
	_ = s.syncPrincipalIdentity(r, u)
	roles, psNames, err := principalGrantNames(r, store, u.ID)
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	s.writeAudit(r, "scim.user.create", "", nil, map[string]any{
		"id": u.ID, "principalType": u.PrincipalType, "userName": in.UserName,
	})
	_ = db.EnqueueOutbox(r.Context(), pool, db.EventPrincipalCreated, u.ID, map[string]any{
		"userId": u.ID, "email": u.Email, "principalType": u.PrincipalType, "source": "scim",
	})
	writeSCIMJSON(w, http.StatusCreated, s.scimToUser(r.Context(), actor, u, roles, psNames, s.scimLocationBase(r)))
}

func (s *Server) handleSCIMGetUser(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	store := db.NewUserStore(pool)
	u, err := store.GetByID(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	if !s.canReadPrincipalType(r, u.PrincipalType) {
		writeSCIMError(w, http.StatusForbidden, "forbidden", "capability required for principalType")
		return
	}
	roles, psNames, err := principalGrantNames(r, store, u.ID)
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	writeSCIMJSON(w, http.StatusOK, s.scimToUser(r.Context(), ActorFromContext(r.Context()), u, roles, psNames, s.scimLocationBase(r)))
}

func (s *Server) canReadPrincipalType(r *http.Request, pt string) bool {
	actor := ActorFromContext(r.Context())
	if actor == nil {
		return false
	}
	if authz.HasAdminPrivilege(actor) {
		return true
	}
	cap := principalTypeCapability(pt)
	if s.systemAz == nil {
		return false
	}
	return s.systemAz.AssertCapability(r.Context(), actor, cap) == nil
}

func (s *Server) handleSCIMListUsers(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeSCIMError(w, http.StatusForbidden, "forbidden", "authentication required")
		return
	}
	filt, err := scim.ParseFilter(r.URL.Query().Get("filter"))
	if err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalidFilter", err.Error())
		return
	}
	f := db.ListPrincipalsFilter{
		Email:         filt.Email,
		UserName:      filt.UserName,
		ExternalID:    filt.ExternalID,
		PrincipalType: filt.PrincipalType,
	}
	if filt.Active != nil {
		f.IsActive = filt.Active
	}
	store := db.NewUserStore(pool)
	list, err := store.List(r.Context(), f)
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
	resources := make([]scim.User, 0)
	for i := range list {
		u := &list[i]
		if !s.canReadPrincipalType(r, u.PrincipalType) {
			continue
		}
		if filt.Active != nil {
			want := *filt.Active
			if u.CanAuthenticate() != want {
				continue
			}
		}
		roles, psNames, err := principalGrantNames(r, store, u.ID)
		if err != nil {
			s.writeSCIMStoreError(w, err)
			return
		}
		resources = append(resources, s.scimToUser(r.Context(), actor, u, roles, psNames, s.scimLocationBase(r)))
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
	writeSCIMJSON(w, http.StatusOK, scim.ListResponse{
		Schemas:      []string{scim.SchemaListResponse},
		TotalResults: total,
		StartIndex:   startIndex,
		ItemsPerPage: len(page),
		Resources:    page,
	})
}

func (s *Server) handleSCIMReplaceUser(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	id := r.PathValue("id")
	var body scim.User
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalidSyntax", "invalid JSON body")
		return
	}
	if body.Groups != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalidValue", "membership is managed on /Groups")
		return
	}
	store := db.NewUserStore(pool)
	before, err := store.GetByID(r.Context(), id)
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	if !s.assertSCIMCapability(w, r, principalTypeCapability(before.PrincipalType)) {
		return
	}
	in := scimReplaceToUpdate(body)
	if body.One != nil && (len(body.One.RoleAPINames) > 0 || body.One.PermissionSetAPINames != nil || strings.TrimSpace(body.One.DataRoleAPIName) != "") {
		if !s.assertSCIMCapability(w, r, authz.CapAuthzManage) {
			return
		}
	}
	actor := ActorFromContext(r.Context())
	if body.Custom != nil {
		custom, err := s.applySCIMCustomData(r.Context(), actor, body.Custom, "update")
		if err != nil {
			s.writeSCIMStoreError(w, err)
			return
		}
		in.DataPatch = custom
	}
	u, err := store.Update(r.Context(), id, in)
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	if body.One != nil && len(body.One.RoleAPINames) > 0 {
		if err := store.ReplaceRolesByAPINames(r.Context(), id, body.One.RoleAPINames); err != nil {
			s.writeSCIMStoreError(w, err)
			return
		}
	}
	if body.One != nil && body.One.PermissionSetAPINames != nil {
		if err := store.ReplacePermissionSetsByAPINames(r.Context(), id, body.One.PermissionSetAPINames); err != nil {
			s.writeSCIMStoreError(w, err)
			return
		}
	}
	if body.One != nil && strings.TrimSpace(body.One.DataRoleAPIName) != "" {
		if err := s.assignDataRoleByAPIName(r.Context(), id, body.One.DataRoleAPIName); err != nil {
			s.writeSCIMStoreError(w, err)
			return
		}
	}
	if body.Active != nil {
		if !*body.Active {
			u, err = s.deprovisionSCIM(r, store, pool, id, before)
		} else if !before.CanAuthenticate() {
			trueVal := true
			u, err = store.Update(r.Context(), id, db.UpdatePrincipalInput{IsActive: &trueVal})
			if err == nil && before.PrincipalType == "user" && s.identity != nil && s.identity.Enabled() {
				_ = s.identity.SetUserActive(r.Context(), before.Email, true)
			}
		}
		if err != nil {
			s.writeSCIMStoreError(w, err)
			return
		}
	}
	roles, psNames, err := principalGrantNames(r, store, u.ID)
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	dataRoleChanged := body.One != nil && strings.TrimSpace(body.One.DataRoleAPIName) != ""
	s.writeSCIMUserUpdateAudit(r, u, principalChangedFieldAPINames(before, u, dataRoleChanged))
	writeSCIMJSON(w, http.StatusOK, s.scimToUser(r.Context(), ActorFromContext(r.Context()), u, roles, psNames, s.scimLocationBase(r)))
}

func scimReplaceToUpdate(body scim.User) db.UpdatePrincipalInput {
	email := body.PrimaryEmail()
	in := db.UpdatePrincipalInput{
		DisplayName: strPtr(body.DisplayName),
		UserName:    strPtr(body.UserName),
		ExternalID:  strPtr(body.ExternalID),
		Locale:      strPtr(body.Locale),
		Timezone:    strPtr(body.Timezone),
		Title:       strPtr(body.Title),
	}
	if email != "" {
		in.Email = &email
	}
	if body.Name != nil {
		in.GivenName = &body.Name.GivenName
		in.FamilyName = &body.Name.FamilyName
	}
	if len(body.PhoneNumbers) > 0 {
		v := body.PhoneNumbers[0].Value
		in.PhoneNumber = &v
	}
	if body.Enterprise != nil {
		if body.Enterprise.Department != "" {
			in.Department = &body.Enterprise.Department
		}
		if body.Enterprise.EmployeeNumber != "" {
			in.EmployeeNumber = &body.Enterprise.EmployeeNumber
		}
	}
	return in
}

func strPtr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func (s *Server) handleSCIMPatchUser(w http.ResponseWriter, r *http.Request) {
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
	if scim.PatchTouchesGroups(req.Operations) {
		writeSCIMError(w, http.StatusBadRequest, "invalidValue", "membership is managed on /Groups")
		return
	}
	store := db.NewUserStore(pool)
	before, err := store.GetByID(r.Context(), id)
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	if !s.assertSCIMCapability(w, r, principalTypeCapability(before.PrincipalType)) {
		return
	}
	roles, psNames, err := principalGrantNames(r, store, before.ID)
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	actor := ActorFromContext(r.Context())
	visibleBefore := map[string]any{}
	for k, v := range before.Data {
		visibleBefore[k] = v
	}
	if s.fieldAz != nil && actor != nil && len(visibleBefore) > 0 {
		stripped, err := s.fieldAz.StripUnreadableFields(r.Context(), actor, "User", visibleBefore)
		if err != nil {
			s.writeSCIMStoreError(w, err)
			return
		}
		visibleBefore = stripped
	}
	cur := s.scimToUser(r.Context(), actor, before, roles, psNames, s.scimLocationBase(r))
	if err := scim.ApplyPatch(&cur, req.Operations); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalidPath", err.Error())
		return
	}
	if cur.One != nil && (cur.One.RoleAPINames != nil || cur.One.PermissionSetAPINames != nil || strings.TrimSpace(cur.One.DataRoleAPIName) != "") {
		if !s.assertSCIMCapability(w, r, authz.CapAuthzManage) {
			return
		}
	}
	in := scimReplaceToUpdate(cur)
	custom, err := s.applySCIMCustomData(r.Context(), actor, cur.Custom, "update")
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	if patch := customDataPatch(visibleBefore, custom); patch != nil {
		in.DataPatch = patch
	}
	u, err := store.Update(r.Context(), id, in)
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	if cur.One != nil && cur.One.RoleAPINames != nil {
		if err := store.ReplaceRolesByAPINames(r.Context(), id, cur.One.RoleAPINames); err != nil {
			s.writeSCIMStoreError(w, err)
			return
		}
	}
	if cur.One != nil && cur.One.PermissionSetAPINames != nil {
		if err := store.ReplacePermissionSetsByAPINames(r.Context(), id, cur.One.PermissionSetAPINames); err != nil {
			s.writeSCIMStoreError(w, err)
			return
		}
	}
	if cur.One != nil && strings.TrimSpace(cur.One.DataRoleAPIName) != "" {
		if err := s.assignDataRoleByAPIName(r.Context(), id, cur.One.DataRoleAPIName); err != nil {
			s.writeSCIMStoreError(w, err)
			return
		}
	}
	if cur.Active != nil {
		if !*cur.Active {
			u, err = s.deprovisionSCIM(r, store, pool, id, before)
		} else if !before.CanAuthenticate() {
			trueVal := true
			u, err = store.Update(r.Context(), id, db.UpdatePrincipalInput{IsActive: &trueVal})
			if err == nil && before.PrincipalType == "user" && s.identity != nil && s.identity.Enabled() {
				_ = s.identity.SetUserActive(r.Context(), before.Email, true)
			}
		}
		if err != nil {
			s.writeSCIMStoreError(w, err)
			return
		}
	}
	roles, psNames, err = principalGrantNames(r, store, u.ID)
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	dataRoleChanged := cur.One != nil && strings.TrimSpace(cur.One.DataRoleAPIName) != ""
	s.writeSCIMUserUpdateAudit(r, u, principalChangedFieldAPINames(before, u, dataRoleChanged))
	writeSCIMJSON(w, http.StatusOK, s.scimToUser(r.Context(), ActorFromContext(r.Context()), u, roles, psNames, s.scimLocationBase(r)))
}

func (s *Server) handleSCIMDeleteUser(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	id := r.PathValue("id")
	store := db.NewUserStore(pool)
	before, err := store.GetByID(r.Context(), id)
	if err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	if !s.assertSCIMCapability(w, r, principalTypeCapability(before.PrincipalType)) {
		return
	}
	if _, err := s.deprovisionSCIM(r, store, pool, id, before); err != nil {
		s.writeSCIMStoreError(w, err)
		return
	}
	s.writeAudit(r, "scim.user.deprovision", "", nil, map[string]any{"id": id})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deprovisionSCIM(r *http.Request, store *db.UserStore, pool *db.Pool, id string, before *db.User) (*db.User, error) {
	u, err := store.DeactivatePrincipal(r.Context(), id)
	if err != nil {
		return nil, err
	}
	if _, err := db.NewCredentialStore(pool).RevokeAllForUser(r.Context(), id); err != nil {
		return nil, err
	}
	if err := s.revokeRefreshForUser(r.Context(), id); err != nil {
		return nil, err
	}
	if before.PrincipalType == "user" && s.identity != nil && s.identity.Enabled() {
		_ = s.identity.SetUserActive(r.Context(), before.Email, false)
	}
	return u, nil
}

func (s *Server) writeSCIMStoreError(w http.ResponseWriter, err error) {
	var ve *dataengine.ValidationError
	switch {
	case errors.Is(err, db.ErrNotFound):
		writeSCIMError(w, http.StatusNotFound, "invalidValue", "resource not found")
	case errors.Is(err, db.ErrConflict), errors.Is(err, db.ErrLastIdentityAdmin), errors.Is(err, db.ErrPrincipalRequiresRole), errors.Is(err, db.ErrPrincipalFrozen):
		writeSCIMError(w, http.StatusConflict, "uniqueness", err.Error())
	case errors.Is(err, db.ErrValidation), errors.As(err, &ve):
		writeSCIMError(w, http.StatusBadRequest, "invalidValue", err.Error())
	case errors.Is(err, authz.ErrForbidden):
		writeSCIMError(w, http.StatusForbidden, "forbidden", err.Error())
	default:
		slog.Error("scim store error", "err", err)
		writeSCIMError(w, http.StatusInternalServerError, "", "internal error")
	}
}
