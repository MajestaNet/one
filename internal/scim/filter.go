package scim

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Filter is a parsed SCIM filter subset: attr eq value [and attr eq value]...
type Filter struct {
	UserName      string
	ExternalID    string
	Email         string
	Active        *bool
	PrincipalType string
}

// ParseFilter parses a limited SCIM filter expression.
func ParseFilter(raw string) (Filter, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Filter{}, nil
	}
	parts := splitAnd(raw)
	var f Filter
	for _, part := range parts {
		attr, value, err := parseEq(part)
		if err != nil {
			return Filter{}, err
		}
		switch strings.ToLower(attr) {
		case "username":
			f.UserName = value
		case "externalid":
			f.ExternalID = value
		case "emails.value", "email":
			f.Email = value
		case "active":
			b, err := strconv.ParseBool(value)
			if err != nil {
				return Filter{}, fmt.Errorf("invalid active value %q", value)
			}
			f.Active = &b
		case "meta.created":
			// Accepted but ignored for equality list filtering in v1.
		case "urn:ietf:params:scim:schemas:extension:one:2.0:principal:principaltype",
			"principaltype",
			SchemaOnePrincipal + ":principaltype":
			f.PrincipalType = value
		default:
			return Filter{}, fmt.Errorf("unsupported filter attribute %q", attr)
		}
	}
	return f, nil
}

func splitAnd(raw string) []string {
	lower := strings.ToLower(raw)
	var parts []string
	start := 0
	for {
		idx := strings.Index(lower[start:], " and ")
		if idx < 0 {
			parts = append(parts, strings.TrimSpace(raw[start:]))
			break
		}
		parts = append(parts, strings.TrimSpace(raw[start:start+idx]))
		start = start + idx + len(" and ")
	}
	return parts
}

func parseEq(part string) (attr, value string, err error) {
	part = strings.TrimSpace(part)
	lower := strings.ToLower(part)
	idx := strings.Index(lower, " eq ")
	if idx < 0 {
		return "", "", fmt.Errorf("only eq filters supported: %q", part)
	}
	attr = strings.TrimSpace(part[:idx])
	rawVal := strings.TrimSpace(part[idx+4:])
	if len(rawVal) >= 2 && ((rawVal[0] == '"' && rawVal[len(rawVal)-1] == '"') || (rawVal[0] == '\'' && rawVal[len(rawVal)-1] == '\'')) {
		value = rawVal[1 : len(rawVal)-1]
		return attr, value, nil
	}
	// Unquoted boolean/number
	for _, r := range rawVal {
		if unicode.IsSpace(r) {
			return "", "", fmt.Errorf("invalid filter value %q", rawVal)
		}
	}
	return attr, rawVal, nil
}

// GroupFilter is the R1 Group list filter: displayName eq / externalId eq [and …].
type GroupFilter struct {
	DisplayName string
	ExternalID  string
}

// ParseGroupFilter parses the R1 Group filter subset.
func ParseGroupFilter(raw string) (GroupFilter, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return GroupFilter{}, nil
	}
	parts := splitAnd(raw)
	var f GroupFilter
	for _, part := range parts {
		attr, value, err := parseEq(part)
		if err != nil {
			return GroupFilter{}, err
		}
		switch strings.ToLower(strings.ReplaceAll(attr, " ", "")) {
		case "displayname":
			f.DisplayName = value
		case "externalid":
			f.ExternalID = value
		default:
			return GroupFilter{}, fmt.Errorf("unsupported filter attribute %q", attr)
		}
	}
	return f, nil
}
