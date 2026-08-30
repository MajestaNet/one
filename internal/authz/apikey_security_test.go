package authz_test

import (
	"context"
	"errors"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
)

type apiKeyUserRepo struct {
	ensureErr error
	permErr   error
	secret    string
}

func (f *apiKeyUserRepo) GetByID(context.Context, string) (*authz.UserRecord, error) {
	return nil, authz.ErrUserNotFound
}

func (f *apiKeyUserRepo) GetByEmail(context.Context, string) (*authz.UserRecord, error) {
	return nil, authz.ErrUserNotFound
}

func (f *apiKeyUserRepo) GetByOIDCSub(context.Context, string) (*authz.UserRecord, error) {
	return nil, authz.ErrUserNotFound
}

func (f *apiKeyUserRepo) EnsureOIDCUser(context.Context, string, string, string, string, bool) (*authz.UserRecord, error) {
	return nil, authz.ErrUserNotFound
}

func (f *apiKeyUserRepo) ListPermissionSetIDs(context.Context, string) ([]string, error) {
	if f.permErr != nil {
		return nil, f.permErr
	}
	return []string{"ps-client"}, nil
}

func (f *apiKeyUserRepo) ListRoleGrants(context.Context, string) ([]authz.Scope, bool, []string, error) {
	return []authz.Scope{authz.ScopeClient}, false, []string{"BootstrapKey_test"}, nil
}

func (f *apiKeyUserRepo) EnsureAPIKeyServicePrincipal(_ context.Context, secret string, _ bool, _ []authz.Scope) (*authz.UserRecord, error) {
	f.secret = secret
	if f.ensureErr != nil {
		return nil, f.ensureErr
	}
	return &authz.UserRecord{
		ID:            "00000000-0000-4000-8000-000000000010",
		Email:         "safe@one.local",
		DisplayName:   "Bootstrap API Key",
		IsActive:      true,
		PrincipalType: "service",
	}, nil
}

func TestResolveAPIKeyDoesNotExposePlaintext(t *testing.T) {
	const secret = "production-secret-that-must-not-leak"
	repo := &apiKeyUserRepo{}
	r := &authz.Resolver{
		Entries:        mustParseKeys(t, secret+":client"),
		DefaultOwnerID: "00000000-0000-4000-8000-000000000001",
		Users:          repo,
	}
	actor, err := r.ResolveAPIKey(secret)
	if err != nil {
		t.Fatal(err)
	}
	if repo.secret != secret {
		t.Fatal("repository must receive the secret for one-way identifier derivation")
	}
	if actor.APIKeyName == secret || actor.APIKeyName == "" {
		t.Fatalf("apiKeyName leaked or omitted: %q", actor.APIKeyName)
	}
	if actor.APIKeyName != authz.APIKeyIdentifier(secret) {
		t.Fatalf("apiKeyName=%q", actor.APIKeyName)
	}
}

func TestResolveAPIKeyFailsClosedWhenPrincipalSyncFails(t *testing.T) {
	const secret = "production-secret-that-must-not-fallback"
	repo := &apiKeyUserRepo{ensureErr: errors.New("database unavailable")}
	r := &authz.Resolver{
		Entries:        mustParseKeys(t, secret+":client"),
		DefaultOwnerID: "00000000-0000-4000-8000-000000000001",
		Users:          repo,
	}
	if _, err := r.ResolveAPIKey(secret); err == nil {
		t.Fatal("principal sync failure must not fall back to the shared bootstrap admin")
	}
}

func TestResolveAPIKeyFailsClosedWhenPermissionSetsCannotLoad(t *testing.T) {
	const secret = "production-secret-with-authz-read-failure"
	repo := &apiKeyUserRepo{permErr: errors.New("database unavailable")}
	r := &authz.Resolver{
		Entries:        mustParseKeys(t, secret+":client"),
		DefaultOwnerID: "00000000-0000-4000-8000-000000000001",
		Users:          repo,
	}
	if _, err := r.ResolveAPIKey(secret); err == nil {
		t.Fatal("permission-set read failure must reject authentication")
	}
}
