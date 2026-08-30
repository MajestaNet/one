package authz

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"time"
)

// Resolver resolves API keys, Majesta One JWTs, and OIDC tokens to actors.
type Resolver struct {
	Entries        []APIKeyEntry
	DefaultOwnerID string
	One            *OneSigner
	OIDC           *OIDCVerifier
	OIDCDefault    []Scope
	AutoProvision  bool
	Users          UserRepository // optional; enables DB-backed principals
	Credentials    CredentialRepository
}

// ResolveBearer authenticates an Authorization bearer / x-api-key value.
func (r *Resolver) ResolveBearer(token string) (*Actor, error) {
	if token == "" {
		return nil, fmt.Errorf("missing credential")
	}
	if LooksLikeJWT(token) {
		if r.One != nil && r.One.Enabled() {
			actor, err := r.ResolveOneJWT(token)
			if err == nil {
				return actor, nil
			}
			// Fall through to OIDC when Majesta One verify fails (transitional dual AuthN).
		}
		if r.OIDC != nil && r.OIDC.Enabled() {
			return r.ResolveOIDC(token)
		}
		if r.One != nil && r.One.Enabled() {
			return nil, fmt.Errorf("invalid one jwt")
		}
	}
	return r.ResolveAPIKey(token)
}

// ResolveOneJWT verifies a One-issued access token and loads AuthZ from DB.
func (r *Resolver) ResolveOneJWT(token string) (*Actor, error) {
	if r.One == nil || !r.One.Enabled() {
		return nil, fmt.Errorf("one jwt not configured")
	}
	claims, err := r.One.Verify(token)
	if err != nil {
		return nil, err
	}
	scopes, adminFromClaims := ScopesFromClaims(claims.Scopes)
	if len(scopes) == 0 {
		return nil, fmt.Errorf("one jwt has no valid API scopes")
	}
	actor := &Actor{
		ID:            claims.Subject,
		PrincipalType: claims.PrincipalType,
		Scopes:        scopes,
		Roles:         append([]string(nil), claims.Roles...),
		IsAdmin:       claims.Admin || adminFromClaims,
		AuthMethod:    AuthMethodOneJWT,
		Azp:           claims.Azp,
	}
	if r.Users != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		u, err := r.Users.GetByID(ctx, claims.Subject)
		if err != nil {
			return nil, fmt.Errorf("one jwt principal: %w", err)
		}
		if !u.CanAuthenticate() {
			if u.FrozenAt != nil {
				return nil, fmt.Errorf("user frozen")
			}
			return nil, fmt.Errorf("user inactive")
		}
		actor.ID = u.ID
		actor.Email = u.Email
		actor.DisplayName = u.DisplayName
		if u.PrincipalType != "" {
			actor.PrincipalType = u.PrincipalType
		}
		// Effective admin and permission sets always from DB (ADR-006).
		actor.IsAdmin = u.IsAdmin
		ids, err := r.Users.ListPermissionSetIDs(ctx, u.ID)
		if err != nil {
			return nil, fmt.Errorf("one jwt permission sets: %w", err)
		}
		actor.PermissionSetIDs = ids
		roleScopes, roleAdmin, roleNames, err := r.Users.ListRoleGrants(ctx, u.ID)
		if err != nil {
			return nil, err
		}
		if len(roleNames) == 0 {
			return nil, ErrPrincipalNoRole
		}
		actor.Scopes = roleScopes
		actor.IsAdmin = actor.IsAdmin || roleAdmin
		actor.Roles = roleNames
	}
	return actor, nil
}

// ResolveAPIKey matches an opaque API key from env entries.
func (r *Resolver) ResolveAPIKey(apiKey string) (*Actor, error) {
	matched := r.matchAPIKey(apiKey)
	if matched == nil {
		return nil, fmt.Errorf("invalid API key")
	}

	isAdmin := matched.IsAdmin
	actor := &Actor{
		ID:            r.DefaultOwnerID,
		Email:         "admin@one.local",
		DisplayName:   "Admin",
		APIKeyName:    APIKeyIdentifier(matched.Key),
		PrincipalType: "service",
		Scopes:        append([]Scope(nil), matched.Scopes...),
		IsAdmin:       isAdmin,
		AuthMethod:    "api_key",
	}

	if r.Users != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		u, err := r.Users.EnsureAPIKeyServicePrincipal(ctx, matched.Key, isAdmin, matched.Scopes)
		if err != nil {
			return nil, fmt.Errorf("api key principal: %w", err)
		}
		if !u.CanAuthenticate() {
			if u.FrozenAt != nil {
				return nil, fmt.Errorf("user frozen")
			}
			return nil, fmt.Errorf("user inactive")
		}
		actor.ID = u.ID
		actor.Email = u.Email
		actor.DisplayName = u.DisplayName
		actor.IsAdmin = isAdmin || u.IsAdmin
		if u.PrincipalType != "" {
			actor.PrincipalType = u.PrincipalType
		}
		ids, err := r.Users.ListPermissionSetIDs(ctx, u.ID)
		if err != nil {
			return nil, fmt.Errorf("api key permission sets: %w", err)
		}
		actor.PermissionSetIDs = ids
		roleScopes, roleAdmin, roleNames, err := r.Users.ListRoleGrants(ctx, u.ID)
		if err != nil {
			return nil, err
		}
		if len(roleNames) == 0 {
			return nil, ErrPrincipalNoRole
		}
		if len(roleScopes) > 0 {
			actor.Scopes = roleScopes
		}
		actor.IsAdmin = actor.IsAdmin || roleAdmin
		actor.Roles = roleNames
	}
	return actor, nil
}

// MatchAPIKeyEntry returns the env API key entry for an opaque secret (or nil).
func (r *Resolver) MatchAPIKeyEntry(apiKey string) *APIKeyEntry {
	return r.matchAPIKey(apiKey)
}

func (r *Resolver) matchAPIKey(apiKey string) *APIKeyEntry {
	apiKeyHash := hashKey(apiKey)
	for i := range r.Entries {
		e := &r.Entries[i]
		entryHash := hashKey(e.Key)
		if subtle.ConstantTimeCompare(apiKeyHash, entryHash) == 1 {
			return e
		}
	}
	return nil
}

func hashKey(k string) []byte {
	sum := sha256.Sum256([]byte("one-apikey-cmp:" + k))
	return sum[:]
}

// APIKeyIdentifier returns a stable, non-secret identifier suitable for
// principal metadata and database persistence. API_KEYS plaintext must never
// be copied into actor responses, audit data, user names, or api_key_name.
func APIKeyIdentifier(secret string) string {
	return "apikey-" + hex.EncodeToString(hashKey(secret))
}
