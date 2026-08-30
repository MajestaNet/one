package dataengine_test

import (
	"context"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
)

func TestCompositeFailsClosedWithoutAuthz(t *testing.T) {
	svc := &dataengine.Service{}
	actor := &authz.Actor{ID: "u1", IsAdmin: true}
	res, err := svc.Composite(context.Background(), []dataengine.CompositeSubrequest{{
		Method: "GET", Object: "Account", ID: "00000000-0000-4000-8000-000000000001", ReferenceID: "r1",
	}}, actor, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.CompositeResponse) != 1 {
		t.Fatalf("got %#v", res)
	}
	if res.CompositeResponse[0]["status"] != 403 {
		t.Fatalf("expected 403 fail-closed, got %#v", res.CompositeResponse[0])
	}
}

func TestCompositeObjectAssertDeny(t *testing.T) {
	svc := &dataengine.Service{}
	actor := &authz.Actor{ID: "u1"}
	az := &dataengine.CompositeAuthz{
		AssertObjectAccess: func(context.Context, *authz.Actor, string, authz.CrudAction) error {
			return authz.ErrForbidden
		},
		CanViewRecord: func(context.Context, *authz.Actor, string, string, string, string, map[string]struct{}) (bool, error) {
			return true, nil
		},
		GetViewAllObjects: func(context.Context, *authz.Actor) (map[string]struct{}, error) {
			return map[string]struct{}{}, nil
		},
	}
	res, err := svc.Composite(context.Background(), []dataengine.CompositeSubrequest{{
		Method: "GET", Object: "Account", ID: "00000000-0000-4000-8000-000000000001", ReferenceID: "r1",
	}}, actor, az)
	if err != nil {
		t.Fatal(err)
	}
	if res.CompositeResponse[0]["status"] != 403 {
		t.Fatalf("expected 403, got %#v", res.CompositeResponse[0])
	}
}

func TestCompositeRejectsUnauthorizedOwnerAssignmentBeforeWrite(t *testing.T) {
	svc := &dataengine.Service{}
	actor := &authz.Actor{ID: "00000000-0000-4000-8000-000000000001"}
	az := &dataengine.CompositeAuthz{
		AssertObjectAccess: func(context.Context, *authz.Actor, string, authz.CrudAction) error { return nil },
		GetModifyAllObjects: func(context.Context, *authz.Actor) (map[string]struct{}, error) {
			return map[string]struct{}{}, nil
		},
	}
	res, err := svc.Composite(context.Background(), []dataengine.CompositeSubrequest{{
		Method: "POST", Object: "Account", ReferenceID: "r1",
		Body: map[string]any{"Name": "Acme", "OwnerId": "00000000-0000-4000-8000-000000000002"},
	}}, actor, az)
	if err != nil {
		t.Fatal(err)
	}
	if res.CompositeResponse[0]["status"] != 403 {
		t.Fatalf("expected 403, got %#v", res.CompositeResponse[0])
	}
}
