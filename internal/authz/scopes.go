package authz

import (
	"fmt"
	"strings"
)

// Scope is an API family scope (ADR-004).
type Scope string

const (
	ScopeClient   Scope = "client"
	ScopeMetadata Scope = "metadata"
	ScopeDeploy   Scope = "deploy"
	ScopeOps      Scope = "ops"
)

// AllScopes is the full set granted to bare API keys.
var AllScopes = []Scope{ScopeClient, ScopeMetadata, ScopeDeploy, ScopeOps}

// APIKeyEntry is one parsed API_KEYS entry.
type APIKeyEntry struct {
	Key     string
	Scopes  []Scope
	IsAdmin bool
}

// Actor is the authenticated principal.
type Actor struct {
	ID                string   `json:"id"`
	Email             string   `json:"email,omitempty"`
	DisplayName       string   `json:"displayName,omitempty"`
	APIKeyName        string   `json:"apiKeyName,omitempty"`
	PrincipalType     string   `json:"principalType,omitempty"` // user | service | agent
	Scopes            []Scope  `json:"scopes"`
	Roles             []string `json:"roles,omitempty"`
	PermissionSetIDs  []string `json:"permissionSetIds"`
	DataRoleID        string   `json:"dataRoleId,omitempty"`
	IsAdmin           bool     `json:"isAdmin"`
	AuthMethod        string   `json:"authMethod"` // api_key | oidc | one_jwt
	OIDCSub           string   `json:"oidcSub,omitempty"`
	Azp               string   `json:"azp,omitempty"`               // Connected App apiName / client that minted the JWT
	SystemPermissions []string `json:"systemPermissions,omitempty"` // effective caps (populated on /me)
}

// HasScope reports whether the actor holds the required family scope.
func (a *Actor) HasScope(required Scope) bool {
	for _, s := range a.Scopes {
		if s == required {
			return true
		}
	}
	return false
}

// ParseAPIKeyEntries parses API_KEYS (comma-separated, optional :scopes).
// Admin privilege requires an explicit +admin marker on the key or in the scope list
// (e.g. "ops-key+admin" or "ops-key:client+metadata+deploy+admin").
func ParseAPIKeyEntries(raw string) ([]APIKeyEntry, error) {
	parts := strings.Split(raw, ",")
	out := make([]APIKeyEntry, 0, len(parts))
	seenKeys := map[string]struct{}{}
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, scopeRaw, found := strings.Cut(part, ":")
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("API key entry %d has an empty key", i+1)
		}
		key, adminFromKey := stripAdminMarker(key)
		if key == "" {
			return nil, fmt.Errorf("API key entry %d has an empty key", i+1)
		}
		identifier := APIKeyIdentifier(key)
		if _, exists := seenKeys[identifier]; exists {
			return nil, fmt.Errorf("duplicate API key entry")
		}
		seenKeys[identifier] = struct{}{}
		if !found || strings.TrimSpace(scopeRaw) == "" {
			out = append(out, APIKeyEntry{
				Key:     key,
				Scopes:  append([]Scope(nil), AllScopes...),
				IsAdmin: adminFromKey,
			})
			continue
		}
		scopes, adminFromScopes, err := parseScopeList(scopeRaw)
		if err != nil {
			return nil, fmt.Errorf("API key entry %d: %w", i+1, err)
		}
		out = append(out, APIKeyEntry{
			Key:     key,
			Scopes:  scopes,
			IsAdmin: adminFromKey || adminFromScopes,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no API keys configured")
	}
	return out, nil
}

func stripAdminMarker(key string) (string, bool) {
	const marker = "+admin"
	if strings.HasSuffix(key, marker) {
		return strings.TrimSuffix(key, marker), true
	}
	return key, false
}

func parseScopeList(raw string) ([]Scope, bool, error) {
	chunks := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '+' || r == ' ' || r == ','
	})
	seen := map[Scope]struct{}{}
	var scopes []Scope
	admin := false
	for _, c := range chunks {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if c == "admin" {
			admin = true
			continue
		}
		s := Scope(c)
		switch s {
		case ScopeClient, ScopeMetadata, ScopeDeploy, ScopeOps:
			if _, ok := seen[s]; !ok {
				seen[s] = struct{}{}
				scopes = append(scopes, s)
			}
		default:
			return nil, false, fmt.Errorf("invalid scope %q", c)
		}
	}
	if len(scopes) == 0 {
		return nil, false, fmt.Errorf("no valid scopes")
	}
	return scopes, admin, nil
}

// ParseScopesCSV parses OIDC_DEFAULT_SCOPES.
func ParseScopesCSV(raw string) []Scope {
	scopes, _, err := parseScopeList(raw)
	if err != nil || len(scopes) == 0 {
		return []Scope{ScopeClient}
	}
	return scopes
}

// LooksLikeJWT reports a three-segment base64url token.
func LooksLikeJWT(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, c := range p {
			ok := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
				(c >= '0' && c <= '9') || c == '-' || c == '_'
			if !ok {
				return false
			}
		}
	}
	return true
}
