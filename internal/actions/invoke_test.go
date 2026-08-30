package actions_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/actions"
	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/packages"
	"github.com/MajestaNet/ide/internal/seed"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestInvokeRequiresActor(t *testing.T) {
	svc := actions.New(actions.Options{})
	_, err := svc.Invoke(context.Background(), nil, "lead.convert", map[string]any{"leadId": "x"})
	var ae *actions.Error
	if !errors.As(err, &ae) || ae.Code != "FORBIDDEN" || ae.Status != http.StatusForbidden {
		t.Fatalf("got %v", err)
	}
}

func TestErrorZeroValue(t *testing.T) {
	var ae *actions.Error
	if ae.Error() != "action error" {
		t.Fatalf("nil error: %q", ae.Error())
	}
	if (&actions.Error{Code: "X"}).Error() != "X" {
		t.Fatal("code fallback")
	}
}

func TestInvokeSchemaRejectsUnknownAndWrongTypes(t *testing.T) {
	packages.Register(packages.Module{
		Name:     "test_actions_types",
		Version:  "0.0.1",
		Optional: true,
		Actions: []packages.ActionDef{{
			APIName:  "test.types",
			Label:    "Types",
			SyncSafe: true,
			InputJSONSchema: `{
				"type":"object","additionalProperties":false,
				"properties":{
					"flag":{"type":"boolean"},
					"name":{"type":"string","minLength":2}
				}
			}`,
		}},
	})
	t.Cleanup(func() {
		packages.Register(packages.Module{Name: "test_actions_types", Version: "0.0.1", Optional: true})
	})
	svc := actions.New(actions.Options{})
	svc.SetHandler("test.types", func(context.Context, *actions.Service, *authz.Actor, map[string]any) (map[string]any, error) {
		return map[string]any{"ok": true}, nil
	})
	actor := &authz.Actor{ID: "00000000-0000-4000-8000-000000000001", IsAdmin: true}
	_, err := svc.Invoke(context.Background(), actor, "test.types", map[string]any{"extra": 1})
	var ae *actions.Error
	if !errors.As(err, &ae) || !strings.Contains(ae.Message, "unknown property") {
		t.Fatalf("unknown: %v", err)
	}
	_, err = svc.Invoke(context.Background(), actor, "test.types", map[string]any{"flag": "yes"})
	if !errors.As(err, &ae) || !strings.Contains(ae.Message, "boolean") {
		t.Fatalf("flag type: %v", err)
	}
	_, err = svc.Invoke(context.Background(), actor, "test.types", map[string]any{"name": "x"})
	if !errors.As(err, &ae) || !strings.Contains(ae.Message, "name") {
		t.Fatalf("minLength: %v", err)
	}
	out, err := svc.Invoke(context.Background(), actor, "test.types", map[string]any{"flag": true, "name": "ok"})
	if err != nil || out["ok"] != true {
		t.Fatalf("ok path: %v %v", out, err)
	}
}

func TestLeadConvertViaService(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()
	if _, err := seed.EnablePackage(ctx, d.Meta, "lead_marketing"); err != nil {
		t.Fatalf("enable lead_marketing: %v", err)
	}
	_, _ = seed.DisablePackage(ctx, d.Meta, "sales")
	t.Cleanup(func() {
		_, _ = seed.DisablePackage(ctx, d.Meta, "lead_marketing")
		_, _ = seed.DisablePackage(ctx, d.Meta, "sales")
		_, _ = d.Pool.Exec(ctx, `UPDATE metadata_automations SET active=true WHERE object_api_name='Lead'`)
	})
	if _, err := d.Pool.Exec(ctx, `UPDATE metadata_automations SET active=false WHERE object_api_name='Lead'`); err != nil {
		t.Fatal(err)
	}
	_, _ = d.Pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1 WHERE id = 1`)

	data := dataengine.NewService(d.Pool, d.Meta)
	objectAz := &authz.ObjectAuthz{Store: &db.ObjectPermStore{Pool: d.Pool}}
	fieldAz := &authz.FieldAuthz{Store: &db.FieldPermStore{Pool: d.Pool}}
	recordAccess := db.NewRecordAccessEvaluator(d.Pool)
	svc := actions.New(actions.Options{
		Meta: d.Meta, Data: data, ObjectAz: objectAz, FieldAz: fieldAz, RecordAccess: recordAccess,
	})
	data.Actions = svc
	actor := &authz.Actor{
		ID: testutil.DefaultOwnerID, IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient},
	}
	lead, err := data.Create(ctx, "Lead", map[string]any{
		"LastName": "ConvertViaSvc", "Company": "Svc Co " + time.Now().Format("150405"), "Status": "New",
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.Invoke(ctx, actor, "lead.convert", map[string]any{"leadId": lead["Id"]})
	if err != nil {
		t.Fatal(err)
	}
	if out["alreadyConverted"] != false || out["accountId"] == nil || out["contactId"] == nil {
		t.Fatalf("out=%v", out)
	}
	again, err := svc.Invoke(ctx, actor, "lead.convert", map[string]any{"leadId": lead["Id"]})
	if err != nil {
		t.Fatal(err)
	}
	if again["alreadyConverted"] != true {
		t.Fatalf("idempotent=%v", again)
	}
	_, err = svc.Invoke(ctx, actor, "lead.convert", map[string]any{
		"leadId": lead["Id"], "createOpportunity": true,
	})
	var ae *actions.Error
	if !errors.As(err, &ae) || ae.Code != "PACKAGE_NOT_ENABLED" {
		t.Fatalf("sales gate: %v", err)
	}
}

func TestLeadConvertCreatesOpportunity(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()
	if _, err := seed.EnablePackage(ctx, d.Meta, "lead_marketing"); err != nil {
		t.Fatal(err)
	}
	if _, err := seed.EnablePackage(ctx, d.Meta, "catalog"); err != nil {
		t.Fatal(err)
	}
	if _, err := seed.EnablePackage(ctx, d.Meta, "sales"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = seed.DisablePackage(ctx, d.Meta, "sales")
		_, _ = seed.DisablePackage(ctx, d.Meta, "catalog")
		_, _ = seed.DisablePackage(ctx, d.Meta, "lead_marketing")
		_, _ = d.Pool.Exec(ctx, `UPDATE metadata_automations SET active=true WHERE object_api_name = ANY($1::text[])`,
			[]string{"Lead", "Account", "Contact", "Opportunity"})
	})
	if _, err := d.Pool.Exec(ctx, `UPDATE metadata_automations SET active=false WHERE object_api_name = ANY($1::text[])`,
		[]string{"Lead", "Account", "Contact", "Opportunity"}); err != nil {
		t.Fatal(err)
	}
	_, _ = d.Pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1 WHERE id = 1`)

	data := dataengine.NewService(d.Pool, d.Meta)
	svc := actions.New(actions.Options{
		Meta: d.Meta, Data: data,
		ObjectAz:     &authz.ObjectAuthz{Store: &db.ObjectPermStore{Pool: d.Pool}},
		FieldAz:      &authz.FieldAuthz{Store: &db.FieldPermStore{Pool: d.Pool}},
		RecordAccess: db.NewRecordAccessEvaluator(d.Pool),
	})
	data.Actions = svc
	actor := &authz.Actor{ID: testutil.DefaultOwnerID, IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient}}
	lead, err := data.Create(ctx, "Lead", map[string]any{
		"LastName": "OppViaSvc", "Company": "Opp Svc " + time.Now().Format("150405"), "Status": "Qualified", "Source": "Web",
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.Invoke(ctx, actor, "lead.convert", map[string]any{
		"leadId": lead["Id"], "createOpportunity": true, "opportunityName": "From Convert",
	})
	if err != nil {
		t.Fatal(err)
	}
	oppID, _ := out["opportunityId"].(string)
	if oppID == "" {
		t.Fatalf("out=%v", out)
	}
	opp, err := data.Get(ctx, "Opportunity", oppID)
	if err != nil {
		t.Fatal(err)
	}
	if opp["Name"] != "From Convert" || opp["StageName"] != "Prospecting" {
		t.Fatalf("opp=%v", opp)
	}
}

func TestQuoteAcceptStatusOnlyViaService(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()
	if _, err := seed.EnablePackage(ctx, d.Meta, "catalog"); err != nil {
		t.Fatal(err)
	}
	if _, err := seed.EnablePackage(ctx, d.Meta, "sales"); err != nil {
		t.Fatal(err)
	}
	_, _ = seed.DisablePackage(ctx, d.Meta, "billing")
	t.Cleanup(func() {
		_, _ = seed.DisablePackage(ctx, d.Meta, "billing")
		_, _ = seed.DisablePackage(ctx, d.Meta, "sales")
		_, _ = seed.DisablePackage(ctx, d.Meta, "catalog")
		_, _ = d.Pool.Exec(ctx, `UPDATE metadata_automations SET active=true WHERE object_api_name = ANY($1::text[])`,
			[]string{"Quote", "QuoteLine", "Product", "Account"})
	})
	if _, err := d.Pool.Exec(ctx, `UPDATE metadata_automations SET active=false WHERE object_api_name = ANY($1::text[])`,
		[]string{"Quote", "QuoteLine", "Product", "Account"}); err != nil {
		t.Fatal(err)
	}
	_, _ = d.Pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1 WHERE id = 1`)

	data := dataengine.NewService(d.Pool, d.Meta)
	svc := actions.New(actions.Options{Meta: d.Meta, Data: data})
	data.Actions = svc
	actor := &authz.Actor{ID: testutil.DefaultOwnerID, IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient}}
	acct, err := data.Create(ctx, "Account", map[string]any{"Name": "Accept Svc"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	product, err := data.Create(ctx, "Product", map[string]any{"Name": "Widget", "ProductType": "Good"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	quote, err := data.Create(ctx, "Quote", map[string]any{
		"Name": "Q-Svc", "Status": "Draft", "AccountId": acct["Id"], "TotalAmount": 10.0,
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := data.Create(ctx, "QuoteLine", map[string]any{
		"QuoteId": quote["Id"], "ProductId": product["Id"], "Quantity": 1.0, "Amount": 10.0,
	}, actor); err != nil {
		t.Fatal(err)
	}
	out, err := svc.Invoke(ctx, actor, "quote.accept", map[string]any{"quoteId": quote["Id"]})
	if err != nil {
		t.Fatal(err)
	}
	if out["alreadyAccepted"] != false || out["orderId"] != nil {
		raw, _ := json.Marshal(out)
		t.Fatalf("accept=%s", raw)
	}
	_, err = svc.Invoke(ctx, actor, "quote.accept", map[string]any{"quoteId": quote["Id"], "createOrder": true})
	var ae *actions.Error
	if !errors.As(err, &ae) || ae.Code != "PACKAGE_NOT_ENABLED" {
		t.Fatalf("billing gate: %v", err)
	}
}

func TestQuoteAcceptCreatesOrder(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()
	for _, pkg := range []string{"catalog", "sales", "billing"} {
		if _, err := seed.EnablePackage(ctx, d.Meta, pkg); err != nil {
			t.Fatalf("enable %s: %v", pkg, err)
		}
	}
	t.Cleanup(func() {
		for _, pkg := range []string{"billing", "sales", "catalog"} {
			_, _ = seed.DisablePackage(ctx, d.Meta, pkg)
		}
		_, _ = d.Pool.Exec(ctx, `UPDATE metadata_automations SET active=true WHERE object_api_name = ANY($1::text[])`,
			[]string{"Quote", "QuoteLine", "Product", "Account", "Order", "OrderLine"})
	})
	if _, err := d.Pool.Exec(ctx, `UPDATE metadata_automations SET active=false WHERE object_api_name = ANY($1::text[])`,
		[]string{"Quote", "QuoteLine", "Product", "Account", "Order", "OrderLine"}); err != nil {
		t.Fatal(err)
	}
	_, _ = d.Pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1 WHERE id = 1`)

	data := dataengine.NewService(d.Pool, d.Meta)
	svc := actions.New(actions.Options{Meta: d.Meta, Data: data})
	data.Actions = svc
	actor := &authz.Actor{ID: testutil.DefaultOwnerID, IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient}}
	acct, err := data.Create(ctx, "Account", map[string]any{"Name": "Order Svc"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	product, err := data.Create(ctx, "Product", map[string]any{"Name": "Gadget", "ProductType": "Good"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	quote, err := data.Create(ctx, "Quote", map[string]any{
		"Name": "Q-Order", "Status": "Draft", "AccountId": acct["Id"], "TotalAmount": 25.0,
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := data.Create(ctx, "QuoteLine", map[string]any{
		"QuoteId": quote["Id"], "ProductId": product["Id"], "Quantity": 1.0, "Amount": 25.0,
	}, actor); err != nil {
		t.Fatal(err)
	}
	out, err := svc.Invoke(ctx, actor, "quote.accept", map[string]any{"quoteId": quote["Id"], "createOrder": true})
	if err != nil {
		t.Fatal(err)
	}
	orderID, _ := out["orderId"].(string)
	if orderID == "" || out["alreadyAccepted"] != false {
		raw, _ := json.Marshal(out)
		t.Fatalf("accept=%s", raw)
	}
	order, err := data.Get(ctx, "Order", orderID)
	if err != nil {
		t.Fatal(err)
	}
	if order["Status"] != "Activated" || order["QuoteId"] != quote["Id"] {
		t.Fatalf("order=%v", order)
	}
}
