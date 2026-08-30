package scim

// CustomAttribute is a customer User field advertised on the UserCustom schema.
type CustomAttribute struct {
	Name string
	Type string
}

// ServiceProviderConfig returns the SCIM ServiceProviderConfig resource.
func ServiceProviderConfig() map[string]any {
	return map[string]any{
		"schemas":          []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		"documentationUri": "https://docs.one.local/scim",
		"patch":            map[string]any{"supported": true},
		"bulk": map[string]any{
			"supported":      false,
			"maxOperations":  0,
			"maxPayloadSize": 0,
		},
		"filter": map[string]any{
			"supported":  true,
			"maxResults": 200,
		},
		"changePassword": map[string]any{"supported": false},
		"sort":           map[string]any{"supported": false},
		"etag":           map[string]any{"supported": false},
		"authenticationSchemes": []map[string]any{
			{
				"type":        "oauthbearertoken",
				"name":        "OAuth Bearer Token",
				"description": "Majesta One JWT via POST /auth/v1/token (client credentials)",
				"specUri":     "http://www.rfc-editor.org/info/rfc6750",
				"primary":     true,
			},
		},
	}
}

// ResourceTypes returns SCIM ResourceTypes.
func ResourceTypes() []map[string]any {
	return []map[string]any{resourceTypeUser(), resourceTypeGroup()}
}

func resourceTypeUser() map[string]any {
	return map[string]any{
		"schemas":     []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"},
		"id":          "User",
		"name":        "User",
		"endpoint":    "/Users",
		"description": "Majesta One principal (user | service | agent)",
		"schema":      SchemaCoreUser,
		"schemaExtensions": []map[string]any{
			{"schema": SchemaEnterpriseUser, "required": false},
			{"schema": SchemaOnePrincipal, "required": false},
			{"schema": SchemaUserCustom, "required": false},
		},
		"meta": map[string]any{"resourceType": "ResourceType", "location": "/scim/v2/ResourceTypes/User"},
	}
}

func resourceTypeGroup() map[string]any {
	return map[string]any{
		"schemas":     []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"},
		"id":          "Group",
		"name":        "Group",
		"endpoint":    "/Groups",
		"description": "Majesta One directory tag (non-AuthZ)",
		"schema":      SchemaCoreGroup,
		"meta":        map[string]any{"resourceType": "ResourceType", "location": "/scim/v2/ResourceTypes/Group"},
	}
}

// Schemas returns supported SCIM schemas, including UserCustom from customer User fields.
func Schemas(custom []CustomAttribute) []map[string]any {
	return []map[string]any{
		schemaCoreUser(),
		schemaCoreGroup(),
		schemaEnterpriseUser(),
		schemaOnePrincipal(),
		schemaUserCustom(custom),
	}
}

func schemaCoreGroup() map[string]any {
	return map[string]any{
		"id":          SchemaCoreGroup,
		"name":        "Group",
		"description": "Directory tag (non-AuthZ membership)",
		"attributes": []map[string]any{
			{"name": "displayName", "type": "string", "required": true, "uniqueness": "server"},
			{"name": "externalId", "type": "string", "uniqueness": "server"},
			{"name": "members", "type": "complex", "multiValued": true, "subAttributes": []map[string]any{
				{"name": "value", "type": "string"},
				{"name": "$ref", "type": "reference"},
				{"name": "type", "type": "string", "canonicalValues": []string{"User"}},
				{"name": "display", "type": "string"},
			}},
		},
		"meta": map[string]any{"resourceType": "Schema", "location": "/scim/v2/Schemas/" + SchemaCoreGroup},
	}
}

func schemaCoreUser() map[string]any {
	return map[string]any{
		"id":          SchemaCoreUser,
		"name":        "User",
		"description": "User Account",
		"attributes": []map[string]any{
			{"name": "userName", "type": "string", "required": true, "uniqueness": "server"},
			{"name": "name", "type": "complex", "subAttributes": []map[string]any{
				{"name": "givenName", "type": "string"},
				{"name": "familyName", "type": "string"},
			}},
			{"name": "displayName", "type": "string"},
			{"name": "emails", "type": "complex", "multiValued": true, "subAttributes": []map[string]any{
				{"name": "value", "type": "string"},
				{"name": "type", "type": "string"},
				{"name": "primary", "type": "boolean"},
			}},
			{"name": "phoneNumbers", "type": "complex", "multiValued": true},
			{"name": "active", "type": "boolean"},
			{"name": "locale", "type": "string"},
			{"name": "timezone", "type": "string"},
			{"name": "title", "type": "string"},
			{"name": "externalId", "type": "string", "uniqueness": "server"},
			{"name": "groups", "type": "complex", "multiValued": true, "mutability": "readOnly", "subAttributes": []map[string]any{
				{"name": "value", "type": "string"},
				{"name": "$ref", "type": "reference"},
				{"name": "display", "type": "string"},
			}},
		},
		"meta": map[string]any{"resourceType": "Schema", "location": "/scim/v2/Schemas/" + SchemaCoreUser},
	}
}

func schemaEnterpriseUser() map[string]any {
	return map[string]any{
		"id":   SchemaEnterpriseUser,
		"name": "EnterpriseUser",
		"attributes": []map[string]any{
			{"name": "department", "type": "string"},
			{"name": "employeeNumber", "type": "string"},
		},
		"meta": map[string]any{"resourceType": "Schema", "location": "/scim/v2/Schemas/" + SchemaEnterpriseUser},
	}
}

func schemaOnePrincipal() map[string]any {
	return map[string]any{
		"id":          SchemaOnePrincipal,
		"name":        "OnePrincipal",
		"description": "Majesta One principal type and AuthZ grants",
		"attributes": []map[string]any{
			{"name": "principalType", "type": "string", "canonicalValues": []string{"user", "service", "agent"}},
			{"name": "roleApiNames", "type": "string", "multiValued": true},
			{"name": "permissionSetApiNames", "type": "string", "multiValued": true},
			{"name": "dataRoleApiName", "type": "string"},
		},
		"meta": map[string]any{"resourceType": "Schema", "location": "/scim/v2/Schemas/" + SchemaOnePrincipal},
	}
}

func schemaUserCustom(custom []CustomAttribute) map[string]any {
	attrs := make([]map[string]any, 0, len(custom))
	for _, a := range custom {
		if a.Name == "" {
			continue
		}
		typ := a.Type
		if typ == "" {
			typ = "string"
		}
		attrs = append(attrs, map[string]any{"name": a.Name, "type": typ})
	}
	return map[string]any{
		"id":          SchemaUserCustom,
		"name":        "UserCustom",
		"description": "Customer custom fields on the Majesta One User kernel object",
		"attributes":  attrs,
		"meta":        map[string]any{"resourceType": "Schema", "location": "/scim/v2/Schemas/" + SchemaUserCustom},
	}
}

// FindSchema returns a schema by id.
func FindSchema(id string, custom []CustomAttribute) (map[string]any, bool) {
	for _, s := range Schemas(custom) {
		if s["id"] == id {
			return s, true
		}
	}
	return nil, false
}

// FindResourceType returns a resource type by id.
func FindResourceType(id string) (map[string]any, bool) {
	switch id {
	case "User":
		return resourceTypeUser(), true
	case "Group":
		return resourceTypeGroup(), true
	default:
		return nil, false
	}
}

// AttributeType maps a Majesta One field type onto a SCIM attribute type.
func AttributeType(fieldType string) string {
	switch fieldType {
	case "boolean":
		return "boolean"
	case "number", "currency", "percent", "integer":
		return "decimal"
	case "date", "datetime":
		return "dateTime"
	default:
		return "string"
	}
}
