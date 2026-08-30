package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/scim"
)

func (s *Server) loadProvisioning(ctx context.Context) (db.ProvisioningConfig, error) {
	if s.pool == nil {
		return db.ProvisioningConfig{}, nil
	}
	st, err := db.NewInstallAuthStore(s.pool).Get(ctx)
	if err != nil {
		return db.ProvisioningConfig{}, err
	}
	if st == nil {
		return db.ProvisioningConfig{}, nil
	}
	return st.Provisioning, nil
}

func (s *Server) validateProvisioning(ctx context.Context, pool *db.Pool, p db.ProvisioningConfig) error {
	p = db.NormalizeProvisioningConfig(p)
	store := db.NewUserStore(pool)
	checkRole := func(name, label string) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil
		}
		if _, err := store.GetRoleByAPIName(ctx, name); err != nil {
			return fmt.Errorf("%w: %s %q not found", db.ErrValidation, label, name)
		}
		return nil
	}
	checkPS := func(names []string, label string) error {
		for _, name := range names {
			if _, err := store.PermissionSetIDByAPIName(ctx, name); err != nil {
				return fmt.Errorf("%w: %s %q not found", db.ErrValidation, label, name)
			}
		}
		return nil
	}
	checkDataRole := func(name, label string) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil
		}
		if _, err := db.NewDataRoleStore(pool).GetDataRoleByAPIName(ctx, name); err != nil {
			return fmt.Errorf("%w: %s %q not found", db.ErrValidation, label, name)
		}
		return nil
	}
	if err := checkRole(p.SCIMDefaultRoleAPIName, "scimDefaultRoleApiName"); err != nil {
		return err
	}
	if err := checkPS(p.JITDefaultPermissionSetAPINames, "jitDefaultPermissionSetApiNames"); err != nil {
		return err
	}
	if err := checkPS(p.SCIMDefaultPermissionSetAPINames, "scimDefaultPermissionSetApiNames"); err != nil {
		return err
	}
	if err := checkDataRole(p.JITDefaultDataRoleAPIName, "jitDefaultDataRoleApiName"); err != nil {
		return err
	}
	if err := checkDataRole(p.SCIMDefaultDataRoleAPIName, "scimDefaultDataRoleApiName"); err != nil {
		return err
	}
	if len(p.ClaimMappings) == 0 {
		return nil
	}
	fields, err := s.loadUserFields(ctx)
	if err != nil {
		return err
	}
	known := map[string]struct{}{}
	for _, f := range fields {
		known[f.APIName] = struct{}{}
	}
	for _, m := range p.ClaimMappings {
		if _, ok := known[m.FieldAPIName]; !ok {
			return fmt.Errorf("%w: claim mapping fieldApiName %q is not a User field", db.ErrValidation, m.FieldAPIName)
		}
	}
	return nil
}

func (s *Server) scimCustomAttributes(ctx context.Context) []scim.CustomAttribute {
	fields, err := s.loadUserFields(ctx)
	if err != nil {
		return nil
	}
	out := make([]scim.CustomAttribute, 0)
	for _, f := range customerUserCustomFieldDefs(fields) {
		out = append(out, scim.CustomAttribute{
			Name: f.APIName,
			Type: scim.AttributeType(f.FieldType),
		})
	}
	return out
}

func (s *Server) scimDataRoleAPIName(ctx context.Context, userID string) string {
	if s.pool == nil || userID == "" {
		return ""
	}
	store := db.NewDataRoleStore(s.pool)
	id, err := store.GetUserDataRoleID(ctx, userID)
	if err != nil || id == nil || *id == "" {
		return ""
	}
	role, err := store.GetDataRoleByID(ctx, *id)
	if err != nil {
		return ""
	}
	return role.APIName
}

func (s *Server) scimToUser(ctx context.Context, actor *authz.Actor, u *db.User, roles, psNames []string, locationBase string) scim.User {
	out := scim.ToUser(u, roles, psNames, s.scimDataRoleAPIName(ctx, u.ID), locationBase)
	if s.pool != nil && u != nil {
		refs, err := db.NewDirectoryTagStore(s.pool).ListGroupRefsForUser(ctx, u.ID)
		if err == nil && len(refs) > 0 {
			out.Groups = make([]scim.GroupRef, len(refs))
			base := strings.TrimRight(locationBase, "/")
			for i, ref := range refs {
				out.Groups[i] = scim.GroupRef{
					Value:   ref.ID,
					Display: ref.DisplayName,
					Ref:     base + "/Groups/" + ref.ID,
				}
			}
		}
	}
	if len(out.Custom) == 0 || s.fieldAz == nil || actor == nil {
		return out
	}
	stripped, err := s.fieldAz.StripUnreadableFields(ctx, actor, "User", out.Custom)
	if err != nil {
		// A permission-store failure must never expose the unfiltered custom data.
		out.Custom = nil
	} else {
		out.Custom = stripped
	}
	if len(out.Custom) == 0 {
		out.Custom = nil
		filtered := out.Schemas[:0]
		for _, sch := range out.Schemas {
			if sch != scim.SchemaUserCustom {
				filtered = append(filtered, sch)
			}
		}
		out.Schemas = filtered
	}
	return out
}

func (s *Server) applySCIMCustomData(ctx context.Context, actor *authz.Actor, custom map[string]any, mode string) (map[string]any, error) {
	if len(custom) == 0 {
		return nil, nil
	}
	fields, err := s.loadUserFields(ctx)
	if err != nil {
		return nil, err
	}
	normalized, err := extractUserCustomData(custom, fields, mode)
	if err != nil {
		return nil, err
	}
	if err := s.assertUserCustomEditable(ctx, actor, normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

func customDataPatch(before, after map[string]any) map[string]any {
	if len(before) == 0 && len(after) == 0 {
		return nil
	}
	patch := map[string]any{}
	for k, v := range after {
		patch[k] = v
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			patch[k] = nil
		}
	}
	if len(patch) == 0 {
		return nil
	}
	return patch
}

func (s *Server) assignDataRoleByAPIName(ctx context.Context, userID, apiName string) error {
	apiName = strings.TrimSpace(apiName)
	if apiName == "" || s.pool == nil {
		return nil
	}
	store := db.NewDataRoleStore(s.pool)
	role, err := store.GetDataRoleByAPIName(ctx, apiName)
	if err != nil {
		return err
	}
	if err := store.SetUserDataRole(ctx, userID, &role.ID); err != nil {
		return err
	}
	_ = db.EnqueueSharingRecalc(ctx, s.pool, map[string]any{"scope": "hierarchy"})
	return nil
}

func scimDefaultRole(body scim.User, prov db.ProvisioningConfig) string {
	if len(body.RoleNames()) > 0 {
		return ""
	}
	if name := strings.TrimSpace(prov.SCIMDefaultRoleAPIName); name != "" {
		return name
	}
	return "StandardUser"
}

func scimEffectivePermissionSets(body scim.User, in *db.CreatePrincipalInput, prov db.ProvisioningConfig) {
	if body.One != nil && body.One.PermissionSetAPINames != nil {
		in.PermissionSetAPINames = body.One.PermissionSetAPINames
		return
	}
	in.PermissionSetAPINames = append([]string(nil), prov.SCIMDefaultPermissionSetAPINames...)
}

func scimEffectiveDataRole(body scim.User, prov db.ProvisioningConfig) string {
	if body.One != nil && strings.TrimSpace(body.One.DataRoleAPIName) != "" {
		return strings.TrimSpace(body.One.DataRoleAPIName)
	}
	return strings.TrimSpace(prov.SCIMDefaultDataRoleAPIName)
}

func claimValue(claims map[string]string, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if v, ok := claims[name]; ok {
		return strings.TrimSpace(v)
	}
	lower := strings.ToLower(name)
	for k, v := range claims {
		if strings.ToLower(k) == lower {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func applyClaimMappings(fields []metadata.FieldDefinition, mappings []db.ProvisioningClaimMapping, claims map[string]string) (db.CreatePrincipalInput, map[string]any) {
	var in db.CreatePrincipalInput
	data := map[string]any{}
	if len(mappings) == 0 || len(claims) == 0 {
		return in, nil
	}
	byName := make(map[string]metadata.FieldDefinition, len(fields))
	for _, f := range fields {
		byName[f.APIName] = f
	}
	for _, m := range mappings {
		val := claimValue(claims, m.Claim)
		if val == "" {
			continue
		}
		def, ok := byName[m.FieldAPIName]
		if !ok {
			continue
		}
		col := ""
		if def.KernelColumn != nil {
			col = strings.TrimSpace(*def.KernelColumn)
		}
		if col == "" {
			data[def.APIName] = val
			continue
		}
		applyKernelString(&in, col, val)
	}
	if len(data) == 0 {
		data = nil
	}
	return in, data
}

func applyKernelString(in *db.CreatePrincipalInput, column, val string) {
	switch column {
	case "email":
		in.Email = val
	case "display_name":
		in.DisplayName = val
	case "user_name":
		in.UserName = val
	case "external_id":
		in.ExternalID = val
	case "given_name":
		in.GivenName = val
	case "family_name":
		in.FamilyName = val
	case "phone_number":
		in.PhoneNumber = val
	case "locale":
		in.Locale = val
	case "timezone":
		in.Timezone = val
	case "title":
		in.Title = val
	case "department":
		in.Department = val
	case "employee_number":
		in.EmployeeNumber = val
	}
}

func mergeCreateInput(dst *db.CreatePrincipalInput, src db.CreatePrincipalInput) {
	if src.Email != "" {
		dst.Email = src.Email
	}
	if src.DisplayName != "" {
		dst.DisplayName = src.DisplayName
	}
	if src.UserName != "" {
		dst.UserName = src.UserName
	}
	if src.ExternalID != "" {
		dst.ExternalID = src.ExternalID
	}
	if src.GivenName != "" {
		dst.GivenName = src.GivenName
	}
	if src.FamilyName != "" {
		dst.FamilyName = src.FamilyName
	}
	if src.PhoneNumber != "" {
		dst.PhoneNumber = src.PhoneNumber
	}
	if src.Locale != "" {
		dst.Locale = src.Locale
	}
	if src.Timezone != "" {
		dst.Timezone = src.Timezone
	}
	if src.Title != "" {
		dst.Title = src.Title
	}
	if src.Department != "" {
		dst.Department = src.Department
	}
	if src.EmployeeNumber != "" {
		dst.EmployeeNumber = src.EmployeeNumber
	}
}

func (s *Server) applyJITCreateProvisioning(ctx context.Context, users *db.UserStore, email, displayName, role string, claims map[string]string, prov db.ProvisioningConfig) (*db.User, error) {
	if role == "" {
		role = "StandardUser"
	}
	in := db.CreatePrincipalInput{
		Email:                 email,
		DisplayName:           displayName,
		PrincipalType:         "user",
		RoleAPINames:          []string{role},
		PermissionSetAPINames: append([]string(nil), prov.JITDefaultPermissionSetAPINames...),
	}
	fields, err := s.loadUserFields(ctx)
	if err != nil {
		return nil, err
	}
	mapped, data := applyClaimMappings(fields, prov.ClaimMappings, claims)
	mergeCreateInput(&in, mapped)
	if len(data) > 0 {
		normalized, err := extractUserCustomData(data, fields, "create")
		if err != nil {
			return nil, err
		}
		in.Data = normalized
	}
	u, err := users.CreateWithGrants(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := s.assignDataRoleByAPIName(ctx, u.ID, prov.JITDefaultDataRoleAPIName); err != nil {
		return nil, err
	}
	return users.GetByID(ctx, u.ID)
}

func oidcClaimsMap(rawToken string, email, name, preferredUsername, subject string) map[string]string {
	out := map[string]string{}
	if email != "" {
		out["email"] = email
	}
	if name != "" {
		out["name"] = name
	}
	if preferredUsername != "" {
		out["preferred_username"] = preferredUsername
	}
	if subject != "" {
		out["sub"] = subject
		out["subject"] = subject
	}
	parts := strings.Split(rawToken, ".")
	if len(parts) < 2 {
		return out
	}
	payload, err := decodeJWTPayload(parts[1])
	if err != nil {
		return out
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return out
	}
	for k, v := range raw {
		if s, ok := stringifyClaim(v); ok && s != "" {
			out[k] = s
		}
	}
	return out
}

func decodeJWTPayload(segment string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(segment)
}

func stringifyClaim(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case float64:
		return strings.TrimSpace(strings.TrimSuffix(fmt.Sprintf("%v", t), ".0")), true
	case bool:
		if t {
			return "true", true
		}
		return "false", true
	default:
		return "", false
	}
}
