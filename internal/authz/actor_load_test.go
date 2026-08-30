package authz_test

import (
	"context"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
)

type loadActorRepo struct {
	user *authz.UserRecord
	err  error
}

func (f *loadActorRepo) GetByID(context.Context, string) (*authz.UserRecord, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.user, nil
}
func (f *loadActorRepo) GetByEmail(context.Context, string) (*authz.UserRecord, error) {
	return nil, authz.ErrUserNotFound
}
func (f *loadActorRepo) GetByOIDCSub(context.Context, string) (*authz.UserRecord, error) {
	return nil, authz.ErrUserNotFound
}
func (f *loadActorRepo) EnsureOIDCUser(context.Context, string, string, string, string, bool) (*authz.UserRecord, error) {
	return nil, authz.ErrUserNotFound
}
func (f *loadActorRepo) ListPermissionSetIDs(context.Context, string) ([]string, error) {
	return []string{"ps-1"}, nil
}
func (f *loadActorRepo) ListRoleGrants(context.Context, string) ([]authz.Scope, bool, []string, error) {
	return []authz.Scope{authz.ScopeClient}, true, []string{"Admin"}, nil
}
func (f *loadActorRepo) EnsureAPIKeyServicePrincipal(context.Context, string, bool, []authz.Scope) (*authz.UserRecord, error) {
	return nil, authz.ErrUserNotFound
}

func TestLoadActorRequiresID(t *testing.T) {
	_, err := authz.LoadActor(context.Background(), &loadActorRepo{}, "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadActorDoesNotInventOwner(t *testing.T) {
	_, err := authz.LoadActor(context.Background(), &loadActorRepo{err: authz.ErrUserNotFound}, "00000000-0000-4000-8000-000000000099")
	if err == nil {
		t.Fatal("expected missing user to fail")
	}
}

func TestLoadActorReconstructsRolesAndAdmin(t *testing.T) {
	repo := &loadActorRepo{user: &authz.UserRecord{
		ID: "u1", Email: "a@b.c", DisplayName: "Ann", IsActive: true, PrincipalType: "user",
	}}
	actor, err := authz.LoadActor(context.Background(), repo, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if actor.ID != "u1" || !actor.IsAdmin || !actor.HasScope(authz.ScopeClient) {
		t.Fatalf("%+v", actor)
	}
	if actor.AuthMethod != "agent_run" {
		t.Fatalf("authMethod=%s", actor.AuthMethod)
	}
}

func TestLoadActorRejectsInactive(t *testing.T) {
	repo := &loadActorRepo{user: &authz.UserRecord{ID: "u1", IsActive: false}}
	if _, err := authz.LoadActor(context.Background(), repo, "u1"); err == nil {
		t.Fatal("expected inactive error")
	}
}
