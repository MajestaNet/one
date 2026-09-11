package dataengine_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/actions"
	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/automation"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/seed"
)

const (
	leadConvertAuto = "Lead_ConvertOnConvertedStatus"
	quoteAcceptAuto = "Quote_AcceptOnStatusAccepted"
)

func requireDenoAndDB(t *testing.T) {
	t.Helper()
	if _, err := automation.FindDeno(""); err != nil {
		t.Skip(err.Error())
	}
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL not set")
	}
}

func bootstrapManagedAutoEngine(t *testing.T, ctx context.Context) (*db.Pool, *metadata.Service, *dataengine.Service, *authz.Actor) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })
	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatal(err)
	}
	meta := metadata.NewService(pool)
	if err := seed.Bootstrap(ctx, pool, meta, seed.Options{
		OwnerID:  "00000000-0000-4000-8000-000000000001",
		AutoSeed: true,
	}); err != nil {
		t.Fatal(err)
	}
	// Shared DATABASE_URL can retain custom Lead/Quote wraps from other tests.
	_, _ = pool.Exec(ctx, `
UPDATE metadata_automations SET active=false
WHERE ownership='custom' AND object_api_name = ANY($1::text[])`,
		[]string{"Lead", "Quote", "QuoteLine"})
	svc := dataengine.NewService(pool, meta)
	actionSvc := actions.New(actions.Options{Meta: meta, Data: svc})
	svc.Actions = actionSvc
	actor := &authz.Actor{
		ID:      "00000000-0000-4000-8000-000000000001",
		IsAdmin: true,
		Scopes:  []authz.Scope{authz.ScopeClient},
	}
	return pool, meta, svc, actor
}

func setManagedAutoActive(t *testing.T, ctx context.Context, pool *db.Pool, apiName string, active bool) {
	t.Helper()
	if _, err := pool.Exec(ctx, `UPDATE metadata_automations SET active=$2 WHERE api_name=$1`, apiName, active); err != nil {
		t.Fatal(err)
	}
	_, _ = pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1 WHERE id = 1`)
}

func TestLeadConvertOnConvertedStatusDefaultOn(t *testing.T) {
	requireDenoAndDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, meta, svc, actor := bootstrapManagedAutoEngine(t, ctx)
	if _, err := seed.EnablePackage(ctx, meta, "lead_marketing"); err != nil {
		t.Fatal(err)
	}
	setManagedAutoActive(t, ctx, pool, leadConvertAuto, true)

	const company = "BP069 DefaultOn Co"
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM records WHERE object_api_name IN ('Lead','Account','Contact') AND (
			data->>'Company'=$1 OR data->>'Name'=$1)`, company)
	})

	created, err := svc.Create(ctx, "Lead", map[string]any{
		"LastName": "DefaultOn", "Company": company, "Status": "New",
	}, actor)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	leadID, _ := created["Id"].(string)
	if _, err := svc.Update(ctx, "Lead", leadID, map[string]any{"Status": "Converted"}, actor); err != nil {
		t.Fatalf("update: %v", err)
	}
	updated, err := svc.Get(ctx, "Lead", leadID)
	if err != nil {
		t.Fatal(err)
	}
	if updated["Status"] != "Converted" {
		t.Fatalf("status=%v", updated["Status"])
	}
	acctID, _ := updated["AccountId"].(string)
	contactID, _ := updated["ContactId"].(string)
	if acctID == "" || contactID == "" {
		t.Fatalf("expected convert parties, got AccountId=%v ContactId=%v", updated["AccountId"], updated["ContactId"])
	}
	if _, ok := updated["OpportunityId"]; ok && updated["OpportunityId"] != nil && updated["OpportunityId"] != "" {
		t.Fatalf("v1 wrap must not create Opportunity: %v", updated["OpportunityId"])
	}
	var oppN int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM records WHERE object_api_name='Opportunity' AND data->>'AccountId'=$1`, acctID).Scan(&oppN)
	if oppN != 0 {
		t.Fatalf("unexpected Opportunity count %d", oppN)
	}
}

func TestLeadConvertOnConvertedStatusInactiveSkip(t *testing.T) {
	requireDenoAndDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, meta, svc, actor := bootstrapManagedAutoEngine(t, ctx)
	if _, err := seed.EnablePackage(ctx, meta, "lead_marketing"); err != nil {
		t.Fatal(err)
	}
	setManagedAutoActive(t, ctx, pool, leadConvertAuto, false)
	t.Cleanup(func() {
		setManagedAutoActive(t, context.Background(), pool, leadConvertAuto, true)
	})

	const company = "BP069 InactiveSkip Co"
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM records WHERE object_api_name IN ('Lead','Account','Contact') AND (
			data->>'Company'=$1 OR data->>'Name'=$1)`, company)
	})

	created, err := svc.Create(ctx, "Lead", map[string]any{
		"LastName": "Inactive", "Company": company, "Status": "New",
	}, actor)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	leadID, _ := created["Id"].(string)
	if _, err := svc.Update(ctx, "Lead", leadID, map[string]any{"Status": "Converted"}, actor); err != nil {
		t.Fatalf("update: %v", err)
	}
	updated, err := svc.Get(ctx, "Lead", leadID)
	if err != nil {
		t.Fatal(err)
	}
	if updated["Status"] != "Converted" {
		t.Fatalf("status=%v", updated["Status"])
	}
	if updated["AccountId"] != nil && updated["AccountId"] != "" {
		t.Fatalf("inactive wrap must not convert, AccountId=%v", updated["AccountId"])
	}
}

func TestLeadConvertOnConvertedStatusPackDisabledSkip(t *testing.T) {
	requireDenoAndDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, meta, svc, actor := bootstrapManagedAutoEngine(t, ctx)
	if _, err := seed.EnablePackage(ctx, meta, "lead_marketing"); err != nil {
		t.Fatal(err)
	}
	setManagedAutoActive(t, ctx, pool, leadConvertAuto, true)
	if _, err := seed.DisablePackage(ctx, meta, "lead_marketing"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = seed.EnablePackage(context.Background(), meta, "lead_marketing")
		setManagedAutoActive(t, context.Background(), pool, leadConvertAuto, true)
	})

	const company = "BP069 PackOff Co"
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM records WHERE object_api_name IN ('Lead','Account','Contact') AND (
			data->>'Company'=$1 OR data->>'Name'=$1)`, company)
	})

	created, err := svc.Create(ctx, "Lead", map[string]any{
		"LastName": "PackOff", "Company": company, "Status": "New",
	}, actor)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	leadID, _ := created["Id"].(string)
	if _, err := svc.Update(ctx, "Lead", leadID, map[string]any{"Status": "Converted"}, actor); err != nil {
		t.Fatalf("update: %v", err)
	}
	updated, err := svc.Get(ctx, "Lead", leadID)
	if err != nil {
		t.Fatal(err)
	}
	if updated["Status"] != "Converted" {
		t.Fatalf("status=%v", updated["Status"])
	}
	if updated["AccountId"] != nil && updated["AccountId"] != "" {
		t.Fatalf("pack-disabled wrap must not convert, AccountId=%v", updated["AccountId"])
	}
	var active bool
	if err := pool.QueryRow(ctx, `SELECT active FROM metadata_automations WHERE api_name=$1`, leadConvertAuto).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if !active {
		t.Fatal("soft-disable must leave automation active=true")
	}
}

func TestLeadConvertOnConvertedStatusAlreadyConvertedNoop(t *testing.T) {
	requireDenoAndDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, meta, svc, actor := bootstrapManagedAutoEngine(t, ctx)
	if _, err := seed.EnablePackage(ctx, meta, "lead_marketing"); err != nil {
		t.Fatal(err)
	}
	setManagedAutoActive(t, ctx, pool, leadConvertAuto, true)

	const company = "BP069 AlreadyConverted Co"
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM records WHERE object_api_name IN ('Lead','Account','Contact') AND (
			data->>'Company'=$1 OR data->>'Name'=$1)`, company)
	})

	created, err := svc.Create(ctx, "Lead", map[string]any{
		"LastName": "Already", "Company": company, "Status": "New",
	}, actor)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	leadID, _ := created["Id"].(string)
	if _, err := svc.Update(ctx, "Lead", leadID, map[string]any{"Status": "Converted"}, actor); err != nil {
		t.Fatalf("convert: %v", err)
	}
	converted, err := svc.Get(ctx, "Lead", leadID)
	if err != nil {
		t.Fatal(err)
	}
	acctID, _ := converted["AccountId"].(string)
	if acctID == "" {
		t.Fatal("expected AccountId after convert")
	}
	if _, err := svc.Update(ctx, "Lead", leadID, map[string]any{"Description": "second write"}, actor); err != nil {
		t.Fatalf("second update: %v", err)
	}
	again, err := svc.Get(ctx, "Lead", leadID)
	if err != nil {
		t.Fatal(err)
	}
	if again["AccountId"] != acctID {
		t.Fatalf("AccountId changed %v → %v", acctID, again["AccountId"])
	}
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM records WHERE object_api_name='Account' AND data->>'Name'=$1`, company).Scan(&n)
	if n != 1 {
		t.Fatalf("expected 1 Account, got %d", n)
	}
}

func TestQuoteAcceptOnStatusAcceptedIdempotent(t *testing.T) {
	requireDenoAndDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, meta, svc, actor := bootstrapManagedAutoEngine(t, ctx)
	if _, err := seed.EnablePackage(ctx, meta, "catalog"); err != nil {
		t.Fatal(err)
	}
	if _, err := seed.EnablePackage(ctx, meta, "sales"); err != nil {
		t.Fatal(err)
	}
	setManagedAutoActive(t, ctx, pool, quoteAcceptAuto, true)

	const prefix = "BP069 QuoteAccept"
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM records WHERE object_api_name IN ('QuoteLine','Quote','Product','Account') AND (
			data->>'Name' LIKE $1 || '%')`, prefix)
	})

	acct, err := svc.Create(ctx, "Account", map[string]any{"Name": prefix + " Co"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	product, err := svc.Create(ctx, "Product", map[string]any{"Name": prefix + " Product"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	quote, err := svc.Create(ctx, "Quote", map[string]any{
		"Name": prefix + " Q", "Status": "Draft", "AccountId": acct["Id"], "TotalAmount": 10.0,
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, "QuoteLine", map[string]any{
		"QuoteId": quote["Id"], "ProductId": product["Id"], "Quantity": 1.0, "Amount": 10.0,
	}, actor); err != nil {
		t.Fatalf("quote line: %v", err)
	}

	quoteID, _ := quote["Id"].(string)
	if _, err := svc.Update(ctx, "Quote", quoteID, map[string]any{"Status": "Accepted"}, actor); err != nil {
		t.Fatalf("accept wrap: %v", err)
	}
	accepted, err := svc.Get(ctx, "Quote", quoteID)
	if err != nil {
		t.Fatal(err)
	}
	if accepted["Status"] != "Accepted" {
		t.Fatalf("status=%v", accepted["Status"])
	}
	acceptedAt, _ := accepted["AcceptedAt"].(string)
	if acceptedAt == "" {
		t.Fatalf("expected AcceptedAt, got %#v", accepted)
	}

	if _, err := svc.Update(ctx, "Quote", quoteID, map[string]any{"Description": "second write"}, actor); err != nil {
		t.Fatalf("second update: %v", err)
	}
	again, err := svc.Get(ctx, "Quote", quoteID)
	if err != nil {
		t.Fatal(err)
	}
	if again["Status"] != "Accepted" {
		t.Fatalf("second status=%v", again["Status"])
	}
	if again["AcceptedAt"] != acceptedAt {
		t.Fatalf("AcceptedAt changed %v → %v", acceptedAt, again["AcceptedAt"])
	}
}
