// Package scim implements a SCIM 2.0 adapter over Majesta One principals (RFC 7643/7644).
package scim

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/db"
)

const (
	SchemaCoreUser       = "urn:ietf:params:scim:schemas:core:2.0:User"
	SchemaCoreGroup      = "urn:ietf:params:scim:schemas:core:2.0:Group"
	SchemaEnterpriseUser = "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"
	SchemaOnePrincipal   = "urn:ietf:params:scim:schemas:extension:one:2.0:Principal"
	SchemaUserCustom     = "urn:ietf:params:scim:schemas:extension:one:2.0:UserCustom"
	SchemaListResponse   = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	SchemaError          = "urn:ietf:params:scim:api:messages:2.0:Error"
	SchemaPatchOp        = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
)

// User is a SCIM User resource.
type User struct {
	Schemas      []string        `json:"schemas"`
	ID           string          `json:"id,omitempty"`
	ExternalID   string          `json:"externalId,omitempty"`
	UserName     string          `json:"userName,omitempty"`
	Name         *Name           `json:"name,omitempty"`
	DisplayName  string          `json:"displayName,omitempty"`
	Emails       []Email         `json:"emails,omitempty"`
	PhoneNumbers []PhoneNumber   `json:"phoneNumbers,omitempty"`
	Active       *bool           `json:"active,omitempty"`
	Locale       string          `json:"locale,omitempty"`
	Timezone     string          `json:"timezone,omitempty"`
	Title        string          `json:"title,omitempty"`
	Meta         *Meta           `json:"meta,omitempty"`
	Enterprise   *EnterpriseUser `json:"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User,omitempty"`
	One          *OnePrincipal   `json:"urn:ietf:params:scim:schemas:extension:one:2.0:Principal,omitempty"`
	Custom       map[string]any  `json:"urn:ietf:params:scim:schemas:extension:one:2.0:UserCustom,omitempty"`
	Groups       []GroupRef      `json:"groups,omitempty"`
}

// GroupRef is the read-only User.groups membership projection (RFC 7643).
type GroupRef struct {
	Value   string `json:"value,omitempty"`
	Display string `json:"display,omitempty"`
	Ref     string `json:"$ref,omitempty"`
}

// Name is SCIM name complex attribute.
type Name struct {
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
}

// Email is a SCIM email multi-valued attribute.
type Email struct {
	Value   string `json:"value,omitempty"`
	Type    string `json:"type,omitempty"`
	Primary bool   `json:"primary,omitempty"`
}

// PhoneNumber is a SCIM phoneNumbers entry.
type PhoneNumber struct {
	Value string `json:"value,omitempty"`
	Type  string `json:"type,omitempty"`
}

// Meta is SCIM resource metadata.
type Meta struct {
	ResourceType string `json:"resourceType,omitempty"`
	Created      string `json:"created,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	Location     string `json:"location,omitempty"`
}

// EnterpriseUser is the enterprise extension.
type EnterpriseUser struct {
	Department     string `json:"department,omitempty"`
	EmployeeNumber string `json:"employeeNumber,omitempty"`
}

// OnePrincipal is the Majesta One extension for principalType and AuthZ grants.
type OnePrincipal struct {
	PrincipalType         string   `json:"principalType,omitempty"`
	RoleAPINames          []string `json:"roleApiNames,omitempty"`
	PermissionSetAPINames []string `json:"permissionSetApiNames,omitempty"`
	DataRoleAPIName       string   `json:"dataRoleApiName,omitempty"`
}

// ListResponse is a SCIM ListResponse.
type ListResponse struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	StartIndex   int      `json:"startIndex"`
	ItemsPerPage int      `json:"itemsPerPage"`
	Resources    []User   `json:"Resources"`
}

// ErrorBody is a SCIM error response.
type ErrorBody struct {
	Schemas  []string `json:"schemas"`
	Status   string   `json:"status"`
	ScimType string   `json:"scimType,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}

// PatchRequest is a SCIM PatchOp payload.
type PatchRequest struct {
	Schemas    []string         `json:"schemas"`
	Operations []PatchOperation `json:"Operations"`
}

// PatchOperation is one patch op.
type PatchOperation struct {
	Op    string          `json:"op"`
	Path  string          `json:"path,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
}

// ToUser maps a Majesta One principal to a SCIM User.
func ToUser(u *db.User, roles, psNames []string, dataRoleAPIName, locationBase string) User {
	active := u.CanAuthenticate()
	schemas := []string{SchemaCoreUser, SchemaEnterpriseUser, SchemaOnePrincipal}
	custom := copyCustomData(u.Data)
	if len(custom) > 0 {
		schemas = append(schemas, SchemaUserCustom)
	}
	out := User{
		Schemas:     schemas,
		ID:          u.ID,
		DisplayName: u.DisplayName,
		Active:      &active,
		Meta: &Meta{
			ResourceType: "User",
			Created:      u.CreatedAt.UTC().Format(time.RFC3339),
			LastModified: u.UpdatedAt.UTC().Format(time.RFC3339),
			Location:     strings.TrimRight(locationBase, "/") + "/Users/" + u.ID,
		},
		One: &OnePrincipal{
			PrincipalType:         u.PrincipalType,
			RoleAPINames:          roles,
			PermissionSetAPINames: psNames,
			DataRoleAPIName:       strings.TrimSpace(dataRoleAPIName),
		},
		Custom: custom,
	}
	if u.ExternalID != nil {
		out.ExternalID = *u.ExternalID
	}
	if u.UserName != nil {
		out.UserName = *u.UserName
	} else if u.Email != "" {
		out.UserName = u.Email
	}
	if u.Email != "" {
		out.Emails = []Email{{Value: u.Email, Type: "work", Primary: true}}
	}
	if u.GivenName != nil || u.FamilyName != nil {
		out.Name = &Name{}
		if u.GivenName != nil {
			out.Name.GivenName = *u.GivenName
		}
		if u.FamilyName != nil {
			out.Name.FamilyName = *u.FamilyName
		}
	}
	if u.PhoneNumber != nil && *u.PhoneNumber != "" {
		out.PhoneNumbers = []PhoneNumber{{Value: *u.PhoneNumber, Type: "work"}}
	}
	if u.Locale != nil {
		out.Locale = *u.Locale
	}
	if u.Timezone != nil {
		out.Timezone = *u.Timezone
	}
	if u.Title != nil {
		out.Title = *u.Title
	}
	ent := &EnterpriseUser{}
	if u.Department != nil {
		ent.Department = *u.Department
	}
	if u.EmployeeNumber != nil {
		ent.EmployeeNumber = *u.EmployeeNumber
	}
	if ent.Department != "" || ent.EmployeeNumber != "" {
		out.Enterprise = ent
	}
	return out
}

func copyCustomData(data map[string]any) map[string]any {
	if len(data) == 0 {
		return nil
	}
	out := make(map[string]any, len(data))
	for k, v := range data {
		if strings.TrimSpace(k) == "" || v == nil {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// PrimaryEmail returns the primary email value.
func (u User) PrimaryEmail() string {
	for _, e := range u.Emails {
		if e.Primary && strings.TrimSpace(e.Value) != "" {
			return strings.TrimSpace(e.Value)
		}
	}
	for _, e := range u.Emails {
		if strings.TrimSpace(e.Value) != "" {
			return strings.TrimSpace(e.Value)
		}
	}
	return ""
}

// PrincipalTypeOrUser returns one principalType or "user".
func (u User) PrincipalTypeOrUser() string {
	if u.One != nil && strings.TrimSpace(u.One.PrincipalType) != "" {
		return strings.TrimSpace(u.One.PrincipalType)
	}
	return "user"
}

// RoleNames returns role api names from the extension.
func (u User) RoleNames() []string {
	if u.One == nil {
		return nil
	}
	return u.One.RoleAPINames
}

// PermissionSetNames returns permission set api names from the extension.
func (u User) PermissionSetNames() []string {
	if u.One == nil {
		return nil
	}
	return u.One.PermissionSetAPINames
}

// ToCreateInput maps a SCIM create payload to db.CreatePrincipalInput.
// defaultRole is used when Majesta One roleApiNames is omitted for principalType=user.
func ToCreateInput(u User, defaultRole string) (db.CreatePrincipalInput, error) {
	pt := u.PrincipalTypeOrUser()
	switch pt {
	case "user", "service", "agent":
	default:
		return db.CreatePrincipalInput{}, fmt.Errorf("invalid principalType %q", pt)
	}
	roles := u.RoleNames()
	if len(roles) == 0 {
		if pt == "user" {
			role := strings.TrimSpace(defaultRole)
			if role == "" {
				role = "StandardUser"
			}
			roles = []string{role}
		} else {
			return db.CreatePrincipalInput{}, fmt.Errorf("roleApiNames required for principalType %s", pt)
		}
	}
	in := db.CreatePrincipalInput{
		Email:                 u.PrimaryEmail(),
		DisplayName:           strings.TrimSpace(u.DisplayName),
		PrincipalType:         pt,
		UserName:              strings.TrimSpace(u.UserName),
		ExternalID:            strings.TrimSpace(u.ExternalID),
		Locale:                strings.TrimSpace(u.Locale),
		Timezone:              strings.TrimSpace(u.Timezone),
		Title:                 strings.TrimSpace(u.Title),
		RoleAPINames:          roles,
		PermissionSetAPINames: u.PermissionSetNames(),
		Data:                  copyCustomData(u.Custom),
	}
	if u.Name != nil {
		in.GivenName = strings.TrimSpace(u.Name.GivenName)
		in.FamilyName = strings.TrimSpace(u.Name.FamilyName)
	}
	if len(u.PhoneNumbers) > 0 {
		in.PhoneNumber = strings.TrimSpace(u.PhoneNumbers[0].Value)
	}
	if u.Enterprise != nil {
		in.Department = strings.TrimSpace(u.Enterprise.Department)
		in.EmployeeNumber = strings.TrimSpace(u.Enterprise.EmployeeNumber)
	}
	if in.DisplayName == "" {
		in.DisplayName = firstNonEmpty(in.UserName, in.Email)
	}
	if strings.TrimSpace(in.UserName) == "" {
		return db.CreatePrincipalInput{}, fmt.Errorf("userName is required")
	}
	return in, nil
}

// NewError builds a SCIM error body.
func NewError(status int, scimType, detail string) ErrorBody {
	return ErrorBody{
		Schemas:  []string{SchemaError},
		Status:   fmt.Sprintf("%d", status),
		ScimType: scimType,
		Detail:   detail,
	}
}
