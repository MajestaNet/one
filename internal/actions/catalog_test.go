package actions_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/actions"
	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/automation"
	"github.com/MajestaNet/ide/internal/packages"
	_ "github.com/MajestaNet/ide/internal/seed"
)

func TestCatalogEmptyWithoutPackages(t *testing.T) {
	svc := actions.New(actions.Options{})
	list, err := svc.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range list {
		if item.APIName == "lead.convert" {
			t.Fatal("lead.convert must be hidden when lead_marketing is not enabled")
		}
	}
}

func TestDescribeUnknownAndDisabled(t *testing.T) {
	svc := actions.New(actions.Options{})
	_, err := svc.Describe(context.Background(), "does.not.exist")
	var ae *actions.Error
	if !errors.As(err, &ae) || ae.Code != "ACTION_NOT_FOUND" || ae.Status != 404 {
		t.Fatalf("unknown describe: %v", err)
	}
	_, err = svc.Describe(context.Background(), "lead.convert")
	if !errors.As(err, &ae) || ae.Code != "PACKAGE_NOT_ENABLED" || ae.Status != 409 {
		t.Fatalf("disabled describe: %v", err)
	}
	if ae.Details["packageName"] != "lead_marketing" {
		t.Fatalf("details=%v", ae.Details)
	}
}

func TestInvokeUnknown(t *testing.T) {
	svc := actions.New(actions.Options{})
	actor := &authz.Actor{ID: "00000000-0000-4000-8000-000000000001", IsAdmin: true}
	_, err := svc.Invoke(context.Background(), actor, "no.such", nil)
	var ae *actions.Error
	if !errors.As(err, &ae) || ae.Code != "ACTION_NOT_FOUND" {
		t.Fatalf("got %v", err)
	}
}

func TestInvokeKnownPackDisabled(t *testing.T) {
	svc := actions.New(actions.Options{})
	actor := &authz.Actor{ID: "00000000-0000-4000-8000-000000000001", IsAdmin: true}
	_, err := svc.Invoke(context.Background(), actor, "lead.convert", map[string]any{"leadId": "x"})
	var ae *actions.Error
	if !errors.As(err, &ae) || ae.Code != "PACKAGE_NOT_ENABLED" {
		t.Fatalf("got %v", err)
	}
}

func TestInvokeAsyncOnlyRejectedInSyncGuest(t *testing.T) {
	packages.Register(packages.Module{
		Name:     "test_actions_async",
		Version:  "0.0.1",
		Optional: true,
		Actions: []packages.ActionDef{{
			APIName:          "test.asyncOnly",
			Label:            "Async Only",
			RequiresPackages: nil,
			SyncSafe:         false,
		}},
	})
	t.Cleanup(func() {
		packages.Register(packages.Module{Name: "test_actions_async", Version: "0.0.1", Optional: true})
	})
	svc := actions.New(actions.Options{})
	svc.SetHandler("test.asyncOnly", func(context.Context, *actions.Service, *authz.Actor, map[string]any) (map[string]any, error) {
		return map[string]any{"ok": true}, nil
	})
	actor := &authz.Actor{ID: "00000000-0000-4000-8000-000000000001", IsAdmin: true}
	ctx := automation.WithSyncGuest(context.Background())
	_, err := svc.Invoke(ctx, actor, "test.asyncOnly", map[string]any{})
	var ae *actions.Error
	if !errors.As(err, &ae) || ae.Code != "VALIDATION_FAILED" || !strings.Contains(ae.Message, "sync") {
		t.Fatalf("got %v", err)
	}
}

func TestActionsByNameHasConvert(t *testing.T) {
	all, err := packages.ActionsByName()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := all["lead.convert"]; !ok {
		t.Fatal("expected lead.convert in registry")
	}
	if _, ok := all["quote.accept"]; !ok {
		t.Fatal("expected quote.accept in registry")
	}
}

func TestInvokeSchemaValidation(t *testing.T) {
	packages.Register(packages.Module{
		Name:     "test_actions_schema",
		Version:  "0.0.1",
		Optional: true,
		Actions: []packages.ActionDef{{
			APIName:          "test.schema",
			Label:            "Schema",
			RequiresPackages: nil,
			SyncSafe:         true,
			InputJSONSchema:  `{"type":"object","additionalProperties":false,"required":["leadId"],"properties":{"leadId":{"type":"string","minLength":1}}}`,
		}},
	})
	t.Cleanup(func() {
		packages.Register(packages.Module{Name: "test_actions_schema", Version: "0.0.1", Optional: true})
	})
	svc := actions.New(actions.Options{})
	svc.SetHandler("test.schema", func(context.Context, *actions.Service, *authz.Actor, map[string]any) (map[string]any, error) {
		return map[string]any{"ok": true}, nil
	})
	actor := &authz.Actor{ID: "00000000-0000-4000-8000-000000000001", IsAdmin: true}
	_, err := svc.Invoke(context.Background(), actor, "test.schema", map[string]any{})
	var ae *actions.Error
	if !errors.As(err, &ae) || ae.Code != "VALIDATION_FAILED" {
		t.Fatalf("missing leadId: %v", err)
	}
	out, err := svc.Invoke(context.Background(), actor, "test.schema", map[string]any{"leadId": "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if out["ok"] != true {
		t.Fatalf("out=%v", out)
	}
}
