package dataengine

import (
	"context"
	"errors"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
)

func TestStripCompositeRecordFailsClosed(t *testing.T) {
	rec := SObjectRecord{"Id": "r1", "Secret__c": "classified"}
	az := &CompositeAuthz{
		StripUnreadableFields: func(context.Context, *authz.Actor, string, map[string]any) (map[string]any, error) {
			return nil, errors.New("permission store unavailable")
		},
	}
	visible, err := stripCompositeRecord(t.Context(), az, &authz.Actor{ID: "u1"}, "Account", rec)
	if err == nil {
		t.Fatal("expected field visibility error")
	}
	if visible != nil {
		t.Fatalf("fail-open record returned: %#v", visible)
	}
}
