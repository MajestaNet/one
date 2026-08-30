package scim

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ApplyPatch applies SCIM patch operations onto a User resource (in memory).
func ApplyPatch(u *User, ops []PatchOperation) error {
	if u == nil {
		return fmt.Errorf("nil user")
	}
	for i, op := range ops {
		opName := strings.ToLower(strings.TrimSpace(op.Op))
		path := strings.TrimSpace(op.Path)
		switch opName {
		case "replace", "add":
			if err := applyReplaceAdd(u, path, op.Value); err != nil {
				return fmt.Errorf("operations[%d]: %w", i, err)
			}
		case "remove":
			if err := applyRemove(u, path); err != nil {
				return fmt.Errorf("operations[%d]: %w", i, err)
			}
		default:
			return fmt.Errorf("operations[%d]: unsupported op %q", i, op.Op)
		}
	}
	return nil
}

func applyReplaceAdd(u *User, path string, raw json.RawMessage) error {
	if path == "" {
		var overlay User
		if err := json.Unmarshal(raw, &overlay); err != nil {
			return err
		}
		mergeUser(u, overlay)
		return nil
	}
	if attr, ok := userCustomPath(path); ok {
		return patchUserCustom(u, attr, raw)
	}
	switch normalizePath(path) {
	case "active":
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return err
		}
		u.Active = &b
	case "displayname":
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		u.DisplayName = s
	case "username":
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		u.UserName = s
	case "externalid":
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		u.ExternalID = s
	case "locale":
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		u.Locale = s
	case "timezone":
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		u.Timezone = s
	case "title":
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		u.Title = s
	case "name.givenname":
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		if u.Name == nil {
			u.Name = &Name{}
		}
		u.Name.GivenName = s
	case "name.familyname":
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		if u.Name == nil {
			u.Name = &Name{}
		}
		u.Name.FamilyName = s
	case "emails", "emails[type eq \"work\"].value", "emails.value":
		if err := patchEmails(u, raw); err != nil {
			return err
		}
	case "phonenumbers", "phonenumbers[type eq \"work\"].value":
		if err := patchPhones(u, raw); err != nil {
			return err
		}
	case SchemaOnePrincipal + ":principaltype", "principaltype":
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		ensureOne(u).PrincipalType = s
	case SchemaOnePrincipal + ":roleapinames", "roleapinames":
		var names []string
		if err := json.Unmarshal(raw, &names); err != nil {
			return err
		}
		ensureOne(u).RoleAPINames = names
	case SchemaOnePrincipal + ":permissionsetapinames", "permissionsetapinames":
		var names []string
		if err := json.Unmarshal(raw, &names); err != nil {
			return err
		}
		ensureOne(u).PermissionSetAPINames = names
	case SchemaOnePrincipal + ":dataroleapiname", "dataroleapiname":
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		ensureOne(u).DataRoleAPIName = s
	case SchemaEnterpriseUser + ":department", "department":
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		if u.Enterprise == nil {
			u.Enterprise = &EnterpriseUser{}
		}
		u.Enterprise.Department = s
	case SchemaEnterpriseUser + ":employeenumber", "employeenumber":
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		if u.Enterprise == nil {
			u.Enterprise = &EnterpriseUser{}
		}
		u.Enterprise.EmployeeNumber = s
	case "groups":
		return fmt.Errorf("membership is managed on /Groups")
	default:
		return fmt.Errorf("unsupported path %q", path)
	}
	return nil
}

func applyRemove(u *User, path string) error {
	if attr, ok := userCustomPath(path); ok {
		if attr == "" {
			u.Custom = nil
			return nil
		}
		if u.Custom != nil {
			delete(u.Custom, attr)
		}
		return nil
	}
	switch normalizePath(path) {
	case "externalid":
		u.ExternalID = ""
	case "displayname":
		u.DisplayName = ""
	case "locale":
		u.Locale = ""
	case "timezone":
		u.Timezone = ""
	case "title":
		u.Title = ""
	case "name.givenname":
		if u.Name != nil {
			u.Name.GivenName = ""
		}
	case "name.familyname":
		if u.Name != nil {
			u.Name.FamilyName = ""
		}
	case "emails":
		u.Emails = nil
	case "phonenumbers":
		u.PhoneNumbers = nil
	case SchemaOnePrincipal + ":permissionsetapinames", "permissionsetapinames":
		if u.One != nil {
			u.One.PermissionSetAPINames = nil
		}
	case SchemaOnePrincipal + ":dataroleapiname", "dataroleapiname":
		if u.One != nil {
			u.One.DataRoleAPIName = ""
		}
	case SchemaEnterpriseUser + ":employeenumber", "employeenumber":
		if u.Enterprise != nil {
			u.Enterprise.EmployeeNumber = ""
		}
	case "groups":
		return fmt.Errorf("membership is managed on /Groups")
	default:
		return fmt.Errorf("unsupported remove path %q", path)
	}
	return nil
}

func userCustomPath(path string) (attr string, ok bool) {
	compacted := strings.ReplaceAll(strings.TrimSpace(path), " ", "")
	if compacted == "" {
		return "", false
	}
	lower := strings.ToLower(compacted)
	prefix := strings.ToLower(SchemaUserCustom)
	if lower == prefix {
		return "", true
	}
	if strings.HasPrefix(lower, prefix+":") || strings.HasPrefix(lower, prefix+".") {
		if len(compacted) < len(prefix)+1 {
			return "", true
		}
		return compacted[len(prefix)+1:], true
	}
	return "", false
}

func patchUserCustom(u *User, attr string, raw json.RawMessage) error {
	if u.Custom == nil {
		u.Custom = map[string]any{}
	}
	if attr == "" {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}
		for k, v := range m {
			u.Custom[k] = v
		}
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	u.Custom[attr] = v
	return nil
}

func normalizePath(path string) string {
	p := strings.ToLower(strings.TrimSpace(path))
	p = strings.ReplaceAll(p, " ", "")
	return p
}

func ensureOne(u *User) *OnePrincipal {
	if u.One == nil {
		u.One = &OnePrincipal{}
	}
	return u.One
}

func mergeUser(dst *User, src User) {
	if src.UserName != "" {
		dst.UserName = src.UserName
	}
	if src.ExternalID != "" {
		dst.ExternalID = src.ExternalID
	}
	if src.DisplayName != "" {
		dst.DisplayName = src.DisplayName
	}
	if src.Active != nil {
		dst.Active = src.Active
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
	if src.Name != nil {
		if dst.Name == nil {
			dst.Name = &Name{}
		}
		if src.Name.GivenName != "" {
			dst.Name.GivenName = src.Name.GivenName
		}
		if src.Name.FamilyName != "" {
			dst.Name.FamilyName = src.Name.FamilyName
		}
	}
	if len(src.Emails) > 0 {
		dst.Emails = src.Emails
	}
	if len(src.PhoneNumbers) > 0 {
		dst.PhoneNumbers = src.PhoneNumbers
	}
	if src.Enterprise != nil {
		if dst.Enterprise == nil {
			dst.Enterprise = &EnterpriseUser{}
		}
		if src.Enterprise.Department != "" {
			dst.Enterprise.Department = src.Enterprise.Department
		}
		if src.Enterprise.EmployeeNumber != "" {
			dst.Enterprise.EmployeeNumber = src.Enterprise.EmployeeNumber
		}
	}
	if src.One != nil {
		lat := ensureOne(dst)
		if src.One.PrincipalType != "" {
			lat.PrincipalType = src.One.PrincipalType
		}
		if src.One.RoleAPINames != nil {
			lat.RoleAPINames = src.One.RoleAPINames
		}
		if src.One.PermissionSetAPINames != nil {
			lat.PermissionSetAPINames = src.One.PermissionSetAPINames
		}
		if src.One.DataRoleAPIName != "" {
			lat.DataRoleAPIName = src.One.DataRoleAPIName
		}
	}
	if src.Custom != nil {
		if dst.Custom == nil {
			dst.Custom = map[string]any{}
		}
		for k, v := range src.Custom {
			dst.Custom[k] = v
		}
	}
	if src.Groups != nil {
		dst.Groups = src.Groups
	}
}

func patchEmails(u *User, raw json.RawMessage) error {
	var emails []Email
	if err := json.Unmarshal(raw, &emails); err == nil {
		u.Emails = emails
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return err
	}
	u.Emails = []Email{{Value: s, Primary: true, Type: "work"}}
	return nil
}

func patchPhones(u *User, raw json.RawMessage) error {
	var phones []PhoneNumber
	if err := json.Unmarshal(raw, &phones); err == nil {
		u.PhoneNumbers = phones
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return err
	}
	u.PhoneNumbers = []PhoneNumber{{Value: s, Type: "work"}}
	return nil
}
