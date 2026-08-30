package integration

import (
	"fmt"
	"strings"
)

var publicForbiddenScopeHints = map[string]struct{}{
	"metadata": {},
	"deploy":   {},
	"ops":      {},
	"admin":    {},
}

var publicAllowedScopeHints = map[string]struct{}{
	"client":         {},
	"openid":         {},
	"email":          {},
	"profile":        {},
	"offline_access": {},
}

// normalizePublicScopes defaults Experience (public) clients to client scope only.
func normalizePublicScopes(hint []string) []string {
	if len(hint) == 0 {
		return []string{"client"}
	}
	out := make([]string, 0, len(hint))
	seen := map[string]struct{}{}
	for _, s := range hint {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	if len(out) == 0 {
		return []string{"client"}
	}
	return out
}

func validatePublicScopes(hint []string) error {
	for _, s := range hint {
		low := strings.ToLower(strings.TrimSpace(s))
		if _, forbidden := publicForbiddenScopeHints[low]; forbidden {
			return fmt.Errorf("%w: public clients cannot request scope hint %q", ErrValidation, s)
		}
		if _, ok := publicAllowedScopeHints[low]; !ok {
			return fmt.Errorf("%w: unsupported scope hint %q for public client", ErrValidation, s)
		}
	}
	return nil
}

func validatePublicClientRoles(roleAPINames []string) error {
	for _, r := range roleAPINames {
		r = strings.TrimSpace(r)
		if r != "StandardUser" {
			return fmt.Errorf("%w: public clients may only use StandardUser role (got %q)", ErrValidation, r)
		}
	}
	return nil
}

func applyPublicClientDefaults(in *CreateInput) error {
	if in.ClientKind != ClientPublic {
		return nil
	}
	in.AllowedScopesHint = normalizePublicScopes(in.AllowedScopesHint)
	if err := validatePublicScopes(in.AllowedScopesHint); err != nil {
		return err
	}
	if len(in.RoleAPINames) > 0 {
		return validatePublicClientRoles(in.RoleAPINames)
	}
	return nil
}

func validatePublicPatchScopes(hint []string) error {
	normalized := normalizePublicScopes(hint)
	return validatePublicScopes(normalized)
}
