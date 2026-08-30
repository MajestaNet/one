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

func TestSyncInvokeActionQuoteAccept(t *testing.T) {
	if _, err := automation.FindDeno(""); err != nil {
		t.Skip(err.Error())
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
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
	if _, err := seed.EnablePackage(ctx, meta, "catalog"); err != nil {
		t.Fatal(err)
	}
	if _, err := seed.EnablePackage(ctx, meta, "sales"); err != nil {
		t.Fatal(err)
	}
	if _, err := seed.EnablePackage(ctx, meta, "billing"); err != nil {
		t.Fatal(err)
	}

	ownerID := "00000000-0000-4000-8000-000000000001"
	svc := dataengine.NewService(pool, meta)
	actionSvc := actions.New(actions.Options{Meta: meta, Data: svc})
	svc.Actions = actionSvc
	actor := &authz.Actor{ID: ownerID, IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient}}

	const autoName = "OnQuoteLine_AcceptQuote"
	cleanup := func() {
		_, _ = pool.Exec(ctx, `UPDATE metadata_automations SET active=false WHERE api_name=$1`, autoName)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_automations WHERE api_name=$1`, autoName)
		_, _ = pool.Exec(ctx, `DELETE FROM automation_permissions WHERE automation_api_name=$1`, autoName)
		_, _ = pool.Exec(ctx, `DELETE FROM records WHERE object_api_name IN ('QuoteLine','Quote','OrderLine','Order','Product','Account') AND (
			data->>'Name' LIKE 'InvokeQuote%' OR data->>'Name' LIKE 'InvokeQ Product%' OR data->>'Name' LIKE 'InvokeQ Co%')`)
		_, _ = pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1 WHERE id = 1`)
	}
	cleanup()
	t.Cleanup(func() {
		cleanup()
		pool.Close()
	})

	if err := svc.EnsurePartitions(ctx, "Order"); err != nil {
		t.Fatal(err)
	}
	if err := svc.EnsurePartitions(ctx, "OrderLine"); err != nil {
		t.Fatal(err)
	}

	src := `
export default async function run(ctx) {
  const accepted = await ctx.invokeAction({
    apiName: "quote.accept",
    input: { quoteId: String(ctx.trigger.data.QuoteId) },
  });
  return { ok: true, data: accepted };
}
`
	if _, err := pool.Exec(ctx, `
INSERT INTO metadata_automations (
  api_name, label, object_api_name, trigger_event, active, actions, ownership, package_name,
  runtime, execution, entry_file, source
) VALUES ($1,'ok','QuoteLine','create',true,'[]'::jsonb,'custom','customer.default','code','sync',
  'src/automations/OnQuoteLine_AcceptQuote.ts',$2)`, autoName, src); err != nil {
		t.Fatal(err)
	}
	_ = db.EnsureAutomationInAccessCatalog(ctx, pool, autoName)
	_, _ = pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1 WHERE id = 1`)

	acct, err := svc.Create(ctx, "Account", map[string]any{"Name": "InvokeQ Co"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	product, err := svc.Create(ctx, "Product", map[string]any{"Name": "InvokeQ Product"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	quote, err := svc.Create(ctx, "Quote", map[string]any{
		"Name": "InvokeQuote", "Status": "Draft", "AccountId": acct["Id"], "TotalAmount": 40.0,
	}, actor)
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.Create(ctx, "QuoteLine", map[string]any{
		"QuoteId": quote["Id"], "ProductId": product["Id"], "Quantity": 1.0, "Amount": 40.0,
	}, actor)
	if err != nil {
		t.Fatalf("create quote line (guest accept): %v", err)
	}

	got, err := svc.Get(ctx, "Quote", quote["Id"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if got["Status"] != "Accepted" {
		t.Fatalf("expected accepted quote, got %v", got["Status"])
	}
	orderID, _ := got["OrderId"].(string)
	if orderID == "" {
		t.Fatalf("missing OrderId: %v", got)
	}
	order, err := svc.Get(ctx, "Order", orderID)
	if err != nil {
		t.Fatal(err)
	}
	if order["Status"] != "Activated" {
		t.Fatalf("order status=%v", order["Status"])
	}
}
