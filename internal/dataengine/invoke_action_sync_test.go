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

func TestSyncInvokeActionConvertAndRollback(t *testing.T) {
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
	if _, err := seed.EnablePackage(ctx, meta, "lead_marketing"); err != nil {
		t.Fatal(err)
	}

	ownerID := "00000000-0000-4000-8000-000000000001"
	svc := dataengine.NewService(pool, meta)
	actionSvc := actions.New(actions.Options{Meta: meta, Data: svc})
	svc.Actions = actionSvc
	actor := &authz.Actor{ID: ownerID, IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient}}

	const autoOK = "OnLeadConvert_CopyRegion"
	const autoFail = "OnLeadConvert_FailAfter"
	cleanup := func() {
		_, _ = pool.Exec(ctx, `UPDATE metadata_automations SET active=false WHERE api_name = ANY($1::text[])`, []string{autoOK, autoFail})
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_automations WHERE api_name = ANY($1::text[])`, []string{autoOK, autoFail})
		_, _ = pool.Exec(ctx, `DELETE FROM automation_permissions WHERE automation_api_name = ANY($1::text[])`, []string{autoOK, autoFail})
		_, _ = pool.Exec(ctx, `DELETE FROM records WHERE object_api_name IN ('Lead','Account','Contact') AND data->>'Company' LIKE 'InvokeAct%'`)
		_, _ = pool.Exec(ctx, `DELETE FROM records WHERE object_api_name='Account' AND data->>'Name' LIKE 'InvokeAct%'`)
		_, _ = pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1 WHERE id = 1`)
	}
	cleanup()
	t.Cleanup(func() {
		cleanup()
		pool.Close()
	})

	srcOK := `
export default async function run(ctx) {
  const converted = await ctx.invokeAction({
    apiName: "lead.convert",
    input: { leadId: ctx.trigger.recordId },
  });
  await ctx.updateRecord({
    objectApiName: "Account",
    recordId: String(converted.accountId),
    data: { Description: "copied-from-lead" },
  });
  return { ok: true, data: converted };
}
`
	if _, err := pool.Exec(ctx, `
INSERT INTO metadata_automations (
  api_name, label, object_api_name, trigger_event, active, actions, ownership, package_name,
  runtime, execution, entry_file, source
) VALUES ($1,'ok','Lead','create',true,'[]'::jsonb,'custom','customer.default','code','sync',
  'src/automations/OnLeadConvert_CopyRegion.ts',$2)`, autoOK, srcOK); err != nil {
		t.Fatal(err)
	}
	_ = db.EnsureAutomationInAccessCatalog(ctx, pool, autoOK)

	srcFail := `
export default async function run(ctx) {
  await ctx.invokeAction({
    apiName: "lead.convert",
    input: { leadId: ctx.trigger.recordId },
  });
  throw new Error("forced guest failure");
}
`
	if _, err := pool.Exec(ctx, `
INSERT INTO metadata_automations (
  api_name, label, object_api_name, trigger_event, active, actions, ownership, package_name,
  runtime, execution, entry_file, source
) VALUES ($1,'fail','Lead','create',true,'[]'::jsonb,'custom','customer.default','code','sync',
  'src/automations/OnLeadConvert_FailAfter.ts',$2)`, autoFail, srcFail); err != nil {
		t.Fatal(err)
	}
	_ = db.EnsureAutomationInAccessCatalog(ctx, pool, autoFail)
	_, _ = pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1 WHERE id = 1`)

	_, _ = pool.Exec(ctx, `UPDATE metadata_automations SET active=false WHERE api_name=$1`, autoFail)
	_, _ = pool.Exec(ctx, `UPDATE metadata_automations SET active=true WHERE api_name=$1`, autoOK)

	created, err := svc.Create(ctx, "Lead", map[string]any{
		"LastName": "Sync", "Company": "InvokeAct Co", "Status": "New",
	}, actor)
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	leadID, _ := created["Id"].(string)
	lead, err := svc.Get(ctx, "Lead", leadID)
	if err != nil {
		t.Fatal(err)
	}
	if lead["Status"] != "Converted" {
		t.Fatalf("expected converted lead, got %v", lead["Status"])
	}
	acctID, _ := lead["AccountId"].(string)
	acct, err := svc.Get(ctx, "Account", acctID)
	if err != nil {
		t.Fatal(err)
	}
	if acct["Description"] != "copied-from-lead" {
		t.Fatalf("account description=%v", acct["Description"])
	}

	_, _ = pool.Exec(ctx, `UPDATE metadata_automations SET active=false WHERE api_name=$1`, autoOK)
	_, _ = pool.Exec(ctx, `UPDATE metadata_automations SET active=true WHERE api_name=$1`, autoFail)

	_, err = svc.Create(ctx, "Lead", map[string]any{
		"LastName": "Rollback", "Company": "InvokeAct Rollback", "Status": "New",
	}, actor)
	if err == nil {
		t.Fatal("expected sync failure")
	}
	var n int
	_ = pool.QueryRow(ctx, `
SELECT count(*) FROM records WHERE object_api_name='Lead' AND data->>'Company'='InvokeAct Rollback'`).Scan(&n)
	if n != 0 {
		t.Fatalf("lead should roll back, found %d", n)
	}
	_ = pool.QueryRow(ctx, `
SELECT count(*) FROM records WHERE object_api_name='Account' AND data->>'Name'='InvokeAct Rollback'`).Scan(&n)
	if n != 0 {
		t.Fatalf("account should roll back, found %d", n)
	}
}
