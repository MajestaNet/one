package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
)

// principalReservedKeys are Client principal JSON keys that are not users.data fields.
var principalReservedKeys = map[string]struct{}{
	"id": {}, "email": {}, "emails": {}, "displayName": {}, "userName": {}, "externalId": {},
	"name": {}, "phoneNumbers": {}, "locale": {}, "timezone": {}, "title": {}, "department": {},
	"employeeNumber": {}, "principalType": {}, "isActive": {}, "canAuthenticate": {}, "isAdmin": {},
	"createdAt": {}, "updatedAt": {}, "frozenAt": {}, "frozenReason": {},
	"roleApiName": {}, "roleApiNames": {}, "permissionSetApiNames": {}, "dataRoleApiName": {},
	"directoryTagApiNames": {},
	"identityBackend":      {}, "identityProvider": {}, "identityError": {},
	"externalSub": {}, "cognitoSub": {}, "externalAppClientId": {}, "cognitoAppClientId": {},
	"externalAppClientSecret": {}, "cognitoAppClientSecret": {},
}

func customerUserCustomFieldDefs(fields []metadata.FieldDefinition) []metadata.FieldDefinition {
	out := make([]metadata.FieldDefinition, 0)
	for _, f := range fields {
		if f.KernelColumn != nil && strings.TrimSpace(*f.KernelColumn) != "" {
			continue
		}
		out = append(out, f)
	}
	return out
}

func extractUserCustomData(raw map[string]any, fields []metadata.FieldDefinition, mode string) (map[string]any, error) {
	if raw == nil {
		return nil, nil
	}
	byName := make(map[string]metadata.FieldDefinition, len(fields))
	for _, f := range fields {
		byName[f.APIName] = f
	}
	custom := map[string]any{}
	for key, value := range raw {
		if _, reserved := principalReservedKeys[key]; reserved {
			continue
		}
		def, ok := byName[key]
		if !ok {
			return nil, fmt.Errorf("%w: unknown User field %s", db.ErrValidation, key)
		}
		if def.KernelColumn != nil && strings.TrimSpace(*def.KernelColumn) != "" {
			return nil, fmt.Errorf("%w: %s is a standard User field; use the principal profile keys", db.ErrValidation, key)
		}
		custom[key] = value
	}
	if len(custom) == 0 {
		return nil, nil
	}
	deletes := map[string]struct{}{}
	toValidate := map[string]any{}
	for k, v := range custom {
		if v == nil {
			deletes[k] = struct{}{}
			continue
		}
		toValidate[k] = v
	}
	defs := customerUserCustomFieldDefs(fields)
	normalized := map[string]any{}
	if len(toValidate) > 0 {
		var err error
		normalized, err = dataengine.NormalizeAndValidateFields(defs, toValidate, mode)
		if err != nil {
			return nil, err
		}
	}
	for k := range deletes {
		normalized[k] = nil
	}
	return normalized, nil
}

func (s *Server) loadUserFields(ctx context.Context) ([]metadata.FieldDefinition, error) {
	if s.meta == nil {
		return nil, nil
	}
	return s.meta.GetFields(ctx, "User")
}

func (s *Server) assertUserCustomEditable(ctx context.Context, actor *authz.Actor, patch map[string]any) error {
	if s.fieldAz == nil || actor == nil || len(patch) == 0 {
		return nil
	}
	keys := map[string]any{}
	for k := range patch {
		keys[k] = true
	}
	return s.fieldAz.AssertEditableFields(ctx, actor, "User", keys)
}

func (s *Server) principalJSONForActor(ctx context.Context, actor *authz.Actor, u *db.User, roleAPINames, permissionSetAPINames []string, includeCustom bool) (map[string]any, error) {
	out := principalJSON(u, roleAPINames, permissionSetAPINames)
	if s.pool != nil && u != nil {
		names, err := db.NewDirectoryTagStore(s.pool).ListAPINamesForUser(ctx, u.ID)
		if err != nil {
			return nil, err
		}
		out["directoryTagApiNames"] = names
	}
	if !includeCustom || u == nil || len(u.Data) == 0 {
		return out, nil
	}
	custom := map[string]any{}
	for k, v := range u.Data {
		if _, reserved := principalReservedKeys[k]; reserved {
			continue
		}
		custom[k] = v
	}
	if s.fieldAz != nil && actor != nil {
		stripped, err := s.fieldAz.StripUnreadableFields(ctx, actor, "User", custom)
		if err != nil {
			return nil, err
		}
		custom = stripped
	}
	for k, v := range custom {
		out[k] = v
	}
	return out, nil
}

func queryIncludes(r *http.Request, token string) bool {
	if r == nil || r.URL == nil {
		return false
	}
	want := strings.ToLower(strings.TrimSpace(token))
	if want == "" {
		return false
	}
	for _, part := range strings.Split(r.URL.Query().Get("include"), ",") {
		if strings.ToLower(strings.TrimSpace(part)) == want {
			return true
		}
	}
	return false
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func jsonValueEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	aj, errA := json.Marshal(a)
	bj, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return fmt.Sprint(a) == fmt.Sprint(b)
	}
	return string(aj) == string(bj)
}

// principalChangedFieldAPINames lists User metadata apiNames that changed (names only, never values).
func principalChangedFieldAPINames(before, after *db.User, dataRoleChanged bool) []string {
	if before == nil || after == nil {
		return nil
	}
	seen := map[string]struct{}{}
	add := func(name string, changed bool) {
		if !changed || strings.TrimSpace(name) == "" {
			return
		}
		seen[name] = struct{}{}
	}
	add("Email", strings.TrimSpace(before.Email) != strings.TrimSpace(after.Email))
	add("DisplayName", before.DisplayName != after.DisplayName)
	add("Username", derefStr(before.UserName) != derefStr(after.UserName))
	add("ExternalId", derefStr(before.ExternalID) != derefStr(after.ExternalID))
	add("GivenName", derefStr(before.GivenName) != derefStr(after.GivenName))
	add("FamilyName", derefStr(before.FamilyName) != derefStr(after.FamilyName))
	add("Phone", derefStr(before.PhoneNumber) != derefStr(after.PhoneNumber))
	add("Locale", derefStr(before.Locale) != derefStr(after.Locale))
	add("Timezone", derefStr(before.Timezone) != derefStr(after.Timezone))
	add("Title", derefStr(before.Title) != derefStr(after.Title))
	add("Department", derefStr(before.Department) != derefStr(after.Department))
	add("EmployeeNumber", derefStr(before.EmployeeNumber) != derefStr(after.EmployeeNumber))
	add("IsActive", before.IsActive != after.IsActive)
	add("PrincipalType", before.PrincipalType != after.PrincipalType)
	add("DataRoleId", dataRoleChanged)

	keys := map[string]struct{}{}
	for k := range before.Data {
		keys[k] = struct{}{}
	}
	for k := range after.Data {
		keys[k] = struct{}{}
	}
	for k := range keys {
		if _, reserved := principalReservedKeys[k]; reserved {
			continue
		}
		var bv, av any
		if before.Data != nil {
			bv = before.Data[k]
		}
		if after.Data != nil {
			av = after.Data[k]
		}
		add(k, !jsonValueEqual(bv, av))
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (s *Server) writePrincipalFieldAudit(r *http.Request, u *db.User, fields []string) {
	if u == nil {
		return
	}
	details := map[string]any{"id": u.ID, "isActive": u.IsActive}
	if len(fields) > 0 {
		details["fields"] = fields
	}
	s.writeAudit(r, "principal.update", "", nil, details)
	if len(fields) == 0 {
		return
	}
	id := u.ID
	s.writeAudit(r, "identity.user.field.patch", "User", &id, map[string]any{"fields": fields})
}

func (s *Server) writeSCIMUserUpdateAudit(r *http.Request, u *db.User, fields []string) {
	if u == nil {
		return
	}
	details := map[string]any{"id": u.ID}
	if len(fields) > 0 {
		details["fields"] = fields
	}
	id := u.ID
	s.writeAudit(r, "scim.user.update", "User", &id, details)
}
