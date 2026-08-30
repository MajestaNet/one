package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/identity"
)

type principalProfileBody struct {
	Email                 string   `json:"email"`
	Emails                []email  `json:"emails"`
	DisplayName           string   `json:"displayName"`
	UserName              string   `json:"userName"`
	ExternalID            string   `json:"externalId"`
	Name                  nameBody `json:"name"`
	PhoneNumbers          []phone  `json:"phoneNumbers"`
	Locale                string   `json:"locale"`
	Timezone              string   `json:"timezone"`
	Title                 string   `json:"title"`
	Department            string   `json:"department"`
	PrincipalType         string   `json:"principalType"`
	IsAdmin               bool     `json:"isAdmin"`
	RoleAPIName           string   `json:"roleApiName"`
	RoleAPINames          []string `json:"roleApiNames"`
	PermissionSetAPINames []string `json:"permissionSetApiNames"`
}

type email struct {
	Value string `json:"value"`
}

type phone struct {
	Value string `json:"value"`
}

type nameBody struct {
	GivenName  string `json:"givenName"`
	FamilyName string `json:"familyName"`
}

func (s *Server) registerPrincipalAdminRoutes(prefix string) {
	capClient := func(cap string, h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeClient, s.requireCapability(cap, h)))
	}
	s.mux.Handle("POST "+prefix+"/principals", capClient(authz.CapIdentityUsers, s.handleCreatePrincipal))
	s.mux.Handle("GET "+prefix+"/principals", capClient(authz.CapIdentityUsers, s.handleListPrincipals))
	s.mux.Handle("GET "+prefix+"/principals/{id}", capClient(authz.CapIdentityUsers, s.handleGetPrincipal))
	s.mux.Handle("PATCH "+prefix+"/principals/{id}", capClient(authz.CapIdentityUsers, s.handlePatchPrincipal))
	s.mux.Handle("POST "+prefix+"/principals/{id}/freeze", capClient(authz.CapIdentityUsers, s.handleFreezePrincipal))
	s.mux.Handle("POST "+prefix+"/principals/{id}/unfreeze", capClient(authz.CapIdentityUsers, s.handleUnfreezePrincipal))
	s.mux.Handle("POST "+prefix+"/principals/{id}/credentials", capClient(authz.CapIdentityUsers, s.handleCreatePrincipalCredential))
	s.mux.Handle("GET "+prefix+"/principals/{id}/credentials", capClient(authz.CapIdentityUsers, s.handleListPrincipalCredentials))
	s.mux.Handle("POST "+prefix+"/principals/{id}/credentials/{credId}/revoke", capClient(authz.CapIdentityUsers, s.handleRevokePrincipalCredential))
	s.mux.Handle("POST "+prefix+"/principals/{id}/password", capClient(authz.CapIdentityUsers, s.handleAdminSetPrincipalPassword))
	s.mux.Handle("GET "+prefix+"/roles", capClient(authz.CapAuthzManage, s.handleListRoles))
	s.mux.Handle("POST "+prefix+"/roles", capClient(authz.CapAuthzManage, s.handleCreateRole))
	s.mux.Handle("GET "+prefix+"/roles/{apiName}", capClient(authz.CapAuthzManage, s.handleGetRole))
	s.mux.Handle("PATCH "+prefix+"/roles/{apiName}", capClient(authz.CapAuthzManage, s.handlePatchRole))
	s.mux.Handle("DELETE "+prefix+"/roles/{apiName}", capClient(authz.CapAuthzManage, s.handleDeleteRole))
	s.mux.Handle("POST "+prefix+"/roles/assign", capClient(authz.CapAuthzManage, s.handleAssignRole))
	s.mux.Handle("POST "+prefix+"/roles/unassign", capClient(authz.CapAuthzManage, s.handleUnassignRole))
	s.mux.Handle("POST "+prefix+"/permissions/assign", capClient(authz.CapAuthzManage, s.handleAssignPermissionSet))
	s.mux.Handle("POST "+prefix+"/permissions/unassign", capClient(authz.CapAuthzManage, s.handleUnassignPermissionSet))
	s.registerDirectoryTagRoutes(prefix, capClient)
}

func principalJSON(u *db.User, roleAPINames, permissionSetAPINames []string) map[string]any {
	out := map[string]any{
		"id":              u.ID,
		"email":           u.Email,
		"displayName":     u.DisplayName,
		"principalType":   u.PrincipalType,
		"isActive":        u.IsActive,
		"canAuthenticate": u.CanAuthenticate(),
		"isAdmin":         u.IsAdmin,
		"createdAt":       u.CreatedAt,
		"updatedAt":       u.UpdatedAt,
	}
	putStringPtr(out, "userName", u.UserName)
	putStringPtr(out, "externalId", u.ExternalID)
	if u.GivenName != nil || u.FamilyName != nil {
		name := map[string]any{}
		putStringPtr(name, "givenName", u.GivenName)
		putStringPtr(name, "familyName", u.FamilyName)
		out["name"] = name
	}
	if u.PhoneNumber != nil && strings.TrimSpace(*u.PhoneNumber) != "" {
		out["phoneNumbers"] = []map[string]any{{"value": *u.PhoneNumber}}
	}
	putStringPtr(out, "locale", u.Locale)
	putStringPtr(out, "timezone", u.Timezone)
	putStringPtr(out, "title", u.Title)
	putStringPtr(out, "department", u.Department)
	putStringPtr(out, "employeeNumber", u.EmployeeNumber)
	if u.FrozenAt != nil {
		out["frozenAt"] = u.FrozenAt
	}
	putStringPtr(out, "frozenReason", u.FrozenReason)
	if roleAPINames != nil {
		out["roleApiNames"] = roleAPINames
	}
	if permissionSetAPINames != nil {
		out["permissionSetApiNames"] = permissionSetAPINames
	}
	return out
}

func putStringPtr(out map[string]any, key string, v *string) {
	if v != nil && strings.TrimSpace(*v) != "" {
		out[key] = *v
	}
}

func roleJSON(role *db.RoleInfo) map[string]any {
	scopes := role.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	return map[string]any{
		"id":       role.ID,
		"apiName":  role.APIName,
		"label":    role.Label,
		"isSystem": role.IsSystem,
		"scopes":   scopes,
	}
}

func (s *Server) identityLinkIssuer() string {
	if s.cfg != nil && s.cfg.OIDCIssuer != "" {
		return s.cfg.OIDCIssuer
	}
	return ""
}

func (s *Server) syncPrincipalIdentity(r *http.Request, u *db.User) map[string]any {
	out := map[string]any{"identityBackend": "off"}
	if s.identity == nil || !s.identity.Enabled() {
		return out
	}
	provider := identity.ProviderForBackend(s.identity.Mode())
	out["identityBackend"] = s.identity.Mode()
	out["identityProvider"] = provider
	links := db.NewIdentityLinkStore(s.pool)
	issuer := s.identityLinkIssuer()
	switch u.PrincipalType {
	case "user":
		sub, err := s.identity.ProvisionUser(r.Context(), u.Email, u.DisplayName)
		if err != nil {
			out["identityError"] = err.Error()
			return out
		}
		if _, err := links.Upsert(r.Context(), u.ID, provider, issuer, sub); err != nil {
			out["identityError"] = err.Error()
			return out
		}
		out["externalSub"] = sub
		out["cognitoSub"] = sub // compat alias
	case "service", "agent":
		clientID, clientSecret, err := s.identity.CreateAppClient(r.Context(), identity.DefaultM2MAppClientSpec(u.Email, u.PrincipalType))
		if err != nil {
			out["identityError"] = err.Error()
			return out
		}
		if _, err := links.Upsert(r.Context(), u.ID, provider, issuer, clientID); err != nil {
			out["identityError"] = err.Error()
			return out
		}
		out["externalAppClientId"] = clientID
		out["cognitoAppClientId"] = clientID // compat alias
		if clientSecret != "" {
			out["externalAppClientSecret"] = clientSecret
			out["cognitoAppClientSecret"] = clientSecret
		}
	}
	return out
}

func (s *Server) handleCreatePrincipal(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	var body principalProfileBody
	b, _ := json.Marshal(raw)
	if err := json.Unmarshal(b, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	pt := strings.TrimSpace(body.PrincipalType)
	if pt == "" {
		pt = "user"
	}
	if pt == "service" || pt == "agent" {
		actor := ActorFromContext(r.Context())
		if actor == nil || (!authz.HasAdminPrivilege(actor) && (s.systemAz == nil || s.systemAz.AssertCapability(r.Context(), actor, authz.CapIdentityIntegrations) != nil)) {
			writeErr(w, http.StatusForbidden, "CAPABILITY_REQUIRED", "capability identity.integrations required")
			return
		}
	}
	roleNames := normalizeBodyNames(append([]string{body.RoleAPIName}, body.RoleAPINames...))
	if len(roleNames) == 0 {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "roleApiNames is required")
		return
	}
	if len(roleNames) > 0 || len(body.PermissionSetAPINames) > 0 {
		actor := ActorFromContext(r.Context())
		if actor != nil && !authz.HasAdminPrivilege(actor) && (s.systemAz == nil || s.systemAz.AssertCapability(r.Context(), actor, authz.CapAuthzManage) != nil) {
			writeErr(w, http.StatusForbidden, "CAPABILITY_REQUIRED", "capability authz.manage required to assign roles/permission sets")
			return
		}
	}
	in := db.CreatePrincipalInput{
		Email:                 firstNonBlank(body.Email, firstEmail(body.Emails)),
		DisplayName:           strings.TrimSpace(body.DisplayName),
		PrincipalType:         pt,
		IsAdmin:               body.IsAdmin,
		UserName:              strings.TrimSpace(body.UserName),
		ExternalID:            strings.TrimSpace(body.ExternalID),
		GivenName:             strings.TrimSpace(body.Name.GivenName),
		FamilyName:            strings.TrimSpace(body.Name.FamilyName),
		PhoneNumber:           firstPhone(body.PhoneNumbers),
		Locale:                strings.TrimSpace(body.Locale),
		Timezone:              strings.TrimSpace(body.Timezone),
		Title:                 strings.TrimSpace(body.Title),
		Department:            strings.TrimSpace(body.Department),
		RoleAPINames:          roleNames,
		PermissionSetAPINames: normalizeBodyNames(body.PermissionSetAPINames),
	}
	if emp, ok := raw["employeeNumber"].(string); ok {
		in.EmployeeNumber = strings.TrimSpace(emp)
	}
	fields, err := s.loadUserFields(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	custom, err := extractUserCustomData(raw, fields, "create")
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if len(custom) > 0 {
		actor := ActorFromContext(r.Context())
		if err := s.assertUserCustomEditable(r.Context(), actor, custom); err != nil {
			writeAPIError(w, err)
			return
		}
		in.Data = map[string]any{}
		for k, v := range custom {
			if v == nil {
				continue
			}
			in.Data[k] = v
		}
	}
	tagNames, tagSet, tagOK := parseDirectoryTagAPINames(raw)
	if !tagOK {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "directoryTagApiNames must be an array of strings")
		return
	}
	if tagSet {
		if _, err := db.NewDirectoryTagStore(pool).GetIDsByAPINames(r.Context(), tagNames); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeErr(w, http.StatusNotFound, "DIRECTORY_TAG_NOT_FOUND", err.Error())
				return
			}
			writeAPIError(w, err)
			return
		}
		if !s.assertClientMemberCapability(w, r, pt) {
			return
		}
	}
	store := db.NewUserStore(pool)
	u, err := store.CreateWithGrants(r.Context(), in)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	roleNames, psNames, err := principalGrantNames(r, store, u.ID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	resp, err := s.principalJSONForActor(r.Context(), ActorFromContext(r.Context()), u, roleNames, psNames, true)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	for k, v := range s.syncPrincipalIdentity(r, u) {
		resp[k] = v
	}
	if tagSet {
		if !s.replacePrincipalDirectoryTags(w, r, u.ID, u.PrincipalType, tagNames) {
			return
		}
		resp, err = s.principalJSONForActor(r.Context(), ActorFromContext(r.Context()), u, roleNames, psNames, true)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		for k, v := range s.syncPrincipalIdentity(r, u) {
			resp[k] = v
		}
	}
	s.writeAudit(r, "principal.create", "", nil, map[string]any{
		"id": u.ID, "email": u.Email, "principalType": u.PrincipalType,
		"identityBackend": resp["identityBackend"],
	})
	_ = db.EnqueueOutbox(r.Context(), pool, db.EventPrincipalCreated, u.ID, map[string]any{
		"userId": u.ID, "email": u.Email, "principalType": u.PrincipalType, "source": "client",
	})
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleListPrincipals(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	f := db.ListPrincipalsFilter{
		PrincipalType: strings.TrimSpace(r.URL.Query().Get("principalType")),
		Email:         strings.TrimSpace(r.URL.Query().Get("email")),
		UserName:      strings.TrimSpace(r.URL.Query().Get("userName")),
		ExternalID:    strings.TrimSpace(r.URL.Query().Get("externalId")),
	}
	if v := strings.TrimSpace(r.URL.Query().Get("isActive")); v != "" {
		switch v {
		case "true", "1":
			b := true
			f.IsActive = &b
		case "false", "0":
			b := false
			f.IsActive = &b
		}
	}
	store := db.NewUserStore(pool)
	list, err := store.List(r.Context(), f)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	includeData := queryIncludes(r, "data")
	actor := ActorFromContext(r.Context())
	out := make([]map[string]any, 0, len(list))
	for i := range list {
		roles, psNames, err := principalGrantNames(r, store, list[i].ID)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		item, err := s.principalJSONForActor(r.Context(), actor, &list[i], roles, psNames, includeData)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"principals": out})
}

func (s *Server) handleGetPrincipal(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	store := db.NewUserStore(pool)
	u, err := store.GetByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeAPIError(w, err)
		return
	}
	roles, psNames, err := principalGrantNames(r, store, u.ID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	out, err := s.principalJSONForActor(r.Context(), ActorFromContext(r.Context()), u, roles, psNames, true)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handlePatchPrincipal(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	id := r.PathValue("id")
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	var body struct {
		Email                 *string   `json:"email"`
		Emails                []email   `json:"emails"`
		DisplayName           *string   `json:"displayName"`
		UserName              *string   `json:"userName"`
		ExternalID            *string   `json:"externalId"`
		Name                  *nameBody `json:"name"`
		PhoneNumbers          []phone   `json:"phoneNumbers"`
		Locale                *string   `json:"locale"`
		Timezone              *string   `json:"timezone"`
		Title                 *string   `json:"title"`
		Department            *string   `json:"department"`
		EmployeeNumber        *string   `json:"employeeNumber"`
		IsActive              *bool     `json:"isActive"`
		IsAdmin               *bool     `json:"isAdmin"`
		PermissionSetAPINames []string  `json:"permissionSetApiNames"`
		DataRoleAPIName       *string   `json:"dataRoleApiName"`
	}
	encoded, _ := json.Marshal(raw)
	if err := json.Unmarshal(encoded, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	store := db.NewUserStore(pool)
	before, err := store.GetByID(r.Context(), id)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	fields, err := s.loadUserFields(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	custom, err := extractUserCustomData(raw, fields, "update")
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if len(custom) > 0 {
		if err := s.assertUserCustomEditable(r.Context(), ActorFromContext(r.Context()), custom); err != nil {
			writeAPIError(w, err)
			return
		}
	}
	in := db.UpdatePrincipalInput{
		Email:                 coalescePatch(body.Email, firstEmailPtr(body.Emails)),
		DisplayName:           body.DisplayName,
		UserName:              body.UserName,
		ExternalID:            body.ExternalID,
		IsAdmin:               body.IsAdmin,
		PermissionSetAPINames: body.PermissionSetAPINames,
		EmployeeNumber:        body.EmployeeNumber,
		DataPatch:             custom,
	}
	if body.Name != nil {
		in.GivenName = &body.Name.GivenName
		in.FamilyName = &body.Name.FamilyName
	}
	if len(body.PhoneNumbers) > 0 {
		v := firstPhone(body.PhoneNumbers)
		in.PhoneNumber = &v
	}
	in.Locale = body.Locale
	in.Timezone = body.Timezone
	in.Title = body.Title
	in.Department = body.Department

	var u *db.User
	if body.IsActive != nil && !*body.IsActive {
		u, err = store.DeactivatePrincipal(r.Context(), id)
		if err == nil {
			if _, revokeErr := db.NewCredentialStore(pool).RevokeAllForUser(r.Context(), id); revokeErr != nil {
				err = revokeErr
			}
		}
		if err == nil {
			err = s.revokeRefreshForUser(r.Context(), id)
		}
		if err == nil && len(custom) > 0 {
			u, err = store.Update(r.Context(), id, db.UpdatePrincipalInput{DataPatch: custom})
		}
	} else {
		in.IsActive = body.IsActive
		u, err = store.Update(r.Context(), id, in)
	}
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if body.DataRoleAPIName != nil {
		actor := ActorFromContext(r.Context())
		if actor != nil && !authz.HasAdminPrivilege(actor) && (s.systemAz == nil || s.systemAz.AssertCapability(r.Context(), actor, authz.CapAuthzManage) != nil) {
			writeErr(w, http.StatusForbidden, "FORBIDDEN", "authz.manage required to assign data roles")
			return
		}
		roleStore := db.NewDataRoleStore(pool)
		var roleID *string
		if strings.TrimSpace(*body.DataRoleAPIName) != "" {
			role, err := roleStore.GetDataRoleByAPIName(r.Context(), *body.DataRoleAPIName)
			if err != nil {
				writeAPIError(w, err)
				return
			}
			roleID = &role.ID
		}
		if err := roleStore.SetUserDataRole(r.Context(), id, roleID); err != nil {
			writeAPIError(w, err)
			return
		}
		_ = db.EnqueueSharingRecalc(r.Context(), pool, map[string]any{"scope": "hierarchy"})
	}
	if tagNames, set, ok := parseDirectoryTagAPINames(raw); !ok {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "directoryTagApiNames must be an array of strings")
		return
	} else if set {
		if !s.replacePrincipalDirectoryTags(w, r, u.ID, u.PrincipalType, tagNames) {
			return
		}
	}
	if body.IsActive != nil && before.PrincipalType == "user" && s.identity != nil && s.identity.Enabled() {
		_ = s.identity.SetUserActive(r.Context(), before.Email, *body.IsActive)
	}
	roles, psNames, err := principalGrantNames(r, store, u.ID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	s.writePrincipalFieldAudit(r, u, principalChangedFieldAPINames(before, u, body.DataRoleAPIName != nil))
	out, err := s.principalJSONForActor(r.Context(), ActorFromContext(r.Context()), u, roles, psNames, true)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleFreezePrincipal(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	store := db.NewUserStore(pool)
	u, err := store.FreezePrincipal(r.Context(), r.PathValue("id"), body.Reason)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if err := s.revokeRefreshForUser(r.Context(), u.ID); err != nil {
		writeAPIError(w, err)
		return
	}
	if u.PrincipalType == "user" && s.identity != nil && s.identity.Enabled() {
		_ = s.identity.SetUserActive(r.Context(), u.Email, false)
	}
	roles, psNames, err := principalGrantNames(r, store, u.ID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "principal.freeze", "", nil, map[string]any{"id": u.ID})
	writeJSON(w, http.StatusOK, principalJSON(u, roles, psNames))
}

func (s *Server) handleUnfreezePrincipal(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		Reactivate *bool `json:"reactivate"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	reactivate := true
	if body.Reactivate != nil {
		reactivate = *body.Reactivate
	}
	store := db.NewUserStore(pool)
	u, err := store.UnfreezePrincipal(r.Context(), r.PathValue("id"), reactivate)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if reactivate && u.PrincipalType == "user" && s.identity != nil && s.identity.Enabled() {
		_ = s.identity.SetUserActive(r.Context(), u.Email, true)
	}
	roles, psNames, err := principalGrantNames(r, store, u.ID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "principal.unfreeze", "", nil, map[string]any{"id": u.ID, "reactivate": reactivate})
	writeJSON(w, http.StatusOK, principalJSON(u, roles, psNames))
}

func (s *Server) handleCreatePrincipalCredential(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	id := r.PathValue("id")
	store := db.NewUserStore(pool)
	if _, err := store.GetByID(r.Context(), id); err != nil {
		writeAPIError(w, err)
		return
	}
	var body struct {
		Label string `json:"label"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	creds := db.NewCredentialStore(pool)
	cred, plaintext, err := creds.GenerateClientSecret(r.Context(), id, body.Label)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "principal.credential.create", "", nil, map[string]any{
		"userId": id, "credentialId": cred.ID,
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":             cred.ID,
		"userId":         cred.UserID,
		"credentialKind": cred.CredentialKind,
		"label":          cred.Label,
		"createdAt":      cred.CreatedAt,
		"clientSecret":   plaintext,
	})
}

func (s *Server) handleListPrincipalCredentials(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	id := r.PathValue("id")
	store := db.NewUserStore(pool)
	if _, err := store.GetByID(r.Context(), id); err != nil {
		writeAPIError(w, err)
		return
	}
	creds := db.NewCredentialStore(pool)
	list, err := creds.ListMetaByUserID(r.Context(), id)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, c := range list {
		out = append(out, map[string]any{
			"id":             c.ID,
			"credentialKind": c.CredentialKind,
			"label":          c.Label,
			"expiresAt":      c.ExpiresAt,
			"revokedAt":      c.RevokedAt,
			"createdAt":      c.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"credentials": out})
}

func (s *Server) handleRevokePrincipalCredential(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	userID := r.PathValue("id")
	credID := r.PathValue("credId")
	creds := db.NewCredentialStore(pool)
	if err := creds.Revoke(r.Context(), userID, credID); err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "principal.credential.revoke", "", nil, map[string]any{
		"userId": userID, "credentialId": credID,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAdminSetPrincipalPassword(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	id := r.PathValue("id")
	store := db.NewUserStore(pool)
	u, err := store.GetByID(r.Context(), id)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if u.PrincipalType != "user" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "password credentials apply to user principals only")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	creds := db.NewCredentialStore(pool)
	if _, err := creds.SetPassword(r.Context(), id, body.Password); err != nil {
		if errors.Is(err, db.ErrValidation) {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
			return
		}
		writeAPIError(w, err)
		return
	}
	if err := s.revokeRefreshForUser(r.Context(), id); err != nil {
		writeAPIError(w, err)
		return
	}
	actor := ActorFromContext(r.Context())
	actorID := ""
	if actor != nil {
		actorID = actor.ID
	}
	s.writeAudit(r, "principal.password.set", "", nil, map[string]any{
		"userId": id, "source": "admin",
	})
	_ = db.EnqueueOutbox(r.Context(), pool, db.EventPrincipalPasswordChanged, id, map[string]any{
		"userId": id, "actorId": actorID, "source": "admin",
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "userId": id})
}

func (s *Server) handleListRoles(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	store := db.NewUserStore(pool)
	roles, err := store.ListRoles(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(roles))
	for i := range roles {
		out = append(out, roleJSON(&roles[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"roles": out})
}

func (s *Server) handleCreateRole(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		APIName string   `json:"apiName"`
		Label   string   `json:"label"`
		Scopes  []string `json:"scopes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	role, err := db.NewUserStore(pool).CreateRole(r.Context(), body.APIName, body.Label, body.Scopes)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "role.create", "", nil, map[string]any{"apiName": role.APIName})
	writeJSON(w, http.StatusCreated, roleJSON(role))
}

func (s *Server) handleGetRole(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	role, err := db.NewUserStore(pool).GetRoleByAPIName(r.Context(), r.PathValue("apiName"))
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, roleJSON(role))
}

func (s *Server) handlePatchRole(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		Label  string   `json:"label"`
		Scopes []string `json:"scopes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	role, err := db.NewUserStore(pool).UpdateRole(r.Context(), r.PathValue("apiName"), body.Label, body.Scopes)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "role.update", "", nil, map[string]any{"apiName": role.APIName})
	writeJSON(w, http.StatusOK, roleJSON(role))
}

func (s *Server) handleDeleteRole(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	force := strings.EqualFold(r.URL.Query().Get("force"), "true") || r.URL.Query().Get("force") == "1"
	apiName := r.PathValue("apiName")
	if err := db.NewUserStore(pool).DeleteRole(r.Context(), apiName, force); err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "role.delete", "", nil, map[string]any{"apiName": apiName, "force": force})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAssignRole(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		UserID      string `json:"userId"`
		RoleAPIName string `json:"roleApiName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID == "" || body.RoleAPIName == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "userId and roleApiName required")
		return
	}
	store := db.NewUserStore(pool)
	if err := store.AssignRoleByAPIName(r.Context(), body.UserID, body.RoleAPIName); err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "principal.role.assign", "", nil, map[string]any{
		"userId": body.UserID, "roleApiName": body.RoleAPIName,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleUnassignRole(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		UserID      string `json:"userId"`
		RoleAPIName string `json:"roleApiName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID == "" || body.RoleAPIName == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "userId and roleApiName required")
		return
	}
	store := db.NewUserStore(pool)
	if err := store.UnassignRoleByAPIName(r.Context(), body.UserID, body.RoleAPIName); err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "principal.role.unassign", "", nil, map[string]any{
		"userId": body.UserID, "roleApiName": body.RoleAPIName,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAssignPermissionSet(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		UserID               string `json:"userId"`
		PermissionSetID      string `json:"permissionSetId"`
		PermissionSetAPIName string `json:"permissionSetApiName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID == "" || (body.PermissionSetID == "" && body.PermissionSetAPIName == "") {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "userId and permissionSetId or permissionSetApiName required")
		return
	}
	store := db.NewUserStore(pool)
	var err error
	if body.PermissionSetAPIName != "" {
		err = store.AssignPermissionSetByAPIName(r.Context(), body.UserID, body.PermissionSetAPIName)
	} else {
		err = store.AssignPermissionSetByID(r.Context(), body.UserID, body.PermissionSetID)
	}
	if err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "principal.permission_set.assign", "", nil, map[string]any{
		"userId": body.UserID, "permissionSetId": body.PermissionSetID, "permissionSetApiName": body.PermissionSetAPIName,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleUnassignPermissionSet(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		UserID               string `json:"userId"`
		PermissionSetID      string `json:"permissionSetId"`
		PermissionSetAPIName string `json:"permissionSetApiName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID == "" || (body.PermissionSetID == "" && body.PermissionSetAPIName == "") {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "userId and permissionSetId or permissionSetApiName required")
		return
	}
	store := db.NewUserStore(pool)
	var err error
	if body.PermissionSetAPIName != "" {
		err = store.UnassignPermissionSetByAPIName(r.Context(), body.UserID, body.PermissionSetAPIName)
	} else {
		err = store.UnassignPermissionSetByID(r.Context(), body.UserID, body.PermissionSetID)
	}
	if err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "principal.permission_set.unassign", "", nil, map[string]any{
		"userId": body.UserID, "permissionSetId": body.PermissionSetID, "permissionSetApiName": body.PermissionSetAPIName,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func principalGrantNames(r *http.Request, store *db.UserStore, userID string) ([]string, []string, error) {
	_, _, roleNames, err := store.ListRoleGrants(r.Context(), userID)
	if err != nil {
		return nil, nil, err
	}
	psNames, err := store.ListPermissionSetAPINames(r.Context(), userID)
	if err != nil {
		return nil, nil, err
	}
	if roleNames == nil {
		roleNames = []string{}
	}
	if psNames == nil {
		psNames = []string{}
	}
	return roleNames, psNames, nil
}

func firstEmail(emails []email) string {
	for _, e := range emails {
		if v := strings.TrimSpace(e.Value); v != "" {
			return v
		}
	}
	return ""
}

func firstEmailPtr(emails []email) *string {
	if v := firstEmail(emails); v != "" {
		return &v
	}
	return nil
}

func firstPhone(phones []phone) string {
	for _, p := range phones {
		if v := strings.TrimSpace(p.Value); v != "" {
			return v
		}
	}
	return ""
}

func firstNonBlank(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func coalescePatch(vals ...*string) *string {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

func normalizeBodyNames(names []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}
