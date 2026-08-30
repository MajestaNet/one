package authz_test

import (
	"context"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
)

type fakeSettings struct {
	enabled bool
	access  string
}

func (f fakeSettings) RecordSharingEnabled(context.Context) (bool, error) { return f.enabled, nil }
func (f fakeSettings) ObjectDefaultAccess(context.Context, string) (string, error) {
	return f.access, nil
}

type fakeHierarchy struct {
	index authz.HierarchyIndex
	roles map[string]*string
}

func (f fakeHierarchy) LoadHierarchy(context.Context) (authz.HierarchyIndex, error) {
	return f.index, nil
}
func (f fakeHierarchy) UserDataRoleID(_ context.Context, userID string) (*string, error) {
	return f.roles[userID], nil
}

type fakeGrants struct {
	read  bool
	write bool
}

func (f fakeGrants) HasRecordGrant(_ context.Context, _, _ string, needWrite bool) (bool, error) {
	if needWrite {
		return f.write, nil
	}
	return f.read, nil
}

func (f fakeGrants) HasRecordGrantForObject(_ context.Context, _, _, _ string, needWrite bool) (bool, error) {
	return f.HasRecordGrant(context.Background(), "", "", needWrite)
}

func TestRecordAccessEvaluatorSharingGrantReadOnly(t *testing.T) {
	eval := &authz.RecordAccessEvaluator{
		Settings:  fakeSettings{enabled: true, access: authz.DefaultAccessPrivate},
		Hierarchy: fakeHierarchy{index: authz.HierarchyIndex{}, roles: map[string]*string{}},
		Grants:    fakeGrants{read: true, write: false},
	}
	actor := &authz.Actor{ID: "viewer"}
	ok, err := eval.CanViewRecordFull(context.Background(), actor, "rec1", "owner", "owner", "Account", nil, true)
	if err != nil || !ok {
		t.Fatalf("read grant should allow view: ok=%v err=%v", ok, err)
	}
	ok, err = eval.CanModifyRecordFull(context.Background(), actor, "rec1", "owner", "owner", "Account", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("read grant must not allow modify")
	}
}

func TestRecordAccessEvaluatorReadWriteGrant(t *testing.T) {
	eval := &authz.RecordAccessEvaluator{
		Settings:  fakeSettings{enabled: true, access: authz.DefaultAccessPrivate},
		Hierarchy: fakeHierarchy{index: authz.HierarchyIndex{}, roles: map[string]*string{}},
		Grants:    fakeGrants{read: true, write: true},
	}
	actor := &authz.Actor{ID: "editor"}
	ok, err := eval.CanModifyRecordFull(context.Background(), actor, "rec1", "owner", "owner", "Account", nil, true)
	if err != nil || !ok {
		t.Fatalf("read_write grant should allow modify: ok=%v err=%v", ok, err)
	}
}

func TestRecordAccessEvaluatorDisabledFallsBack(t *testing.T) {
	eval := &authz.RecordAccessEvaluator{
		Settings: fakeSettings{enabled: false, access: authz.DefaultAccessPrivate},
		Grants:   fakeGrants{read: true, write: true},
	}
	actor := &authz.Actor{ID: "stranger"}
	ok, err := eval.CanViewRecordFull(context.Background(), actor, "rec1", "owner", "owner", "Account", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("sharing disabled + non-owner must deny")
	}
}
