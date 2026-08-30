package deploy_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/automation"
	"github.com/MajestaNet/ide/internal/customerrepo"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/deploy"
)

func TestAutomationUnitAndContractSuite(t *testing.T) {
	if _, err := automation.FindDeno(""); err != nil {
		t.Skip(err.Error())
	}
	ctx, pool, meta, data, engine := setupDeployTest(t)
	ownerID := "00000000-0000-4000-8000-000000000001"
	if _, err := db.NewUserStore(pool).EnsureBootstrapAdmin(ctx, ownerID, "admin@one.local", "Admin"); err != nil {
		t.Fatal(err)
	}

	const parent = "AutoStepParent__c"
	const child = "AutoStepChild__c"
	const autoName = "CreateChild_On_Parent"
	const suiteName = "CreateChildAutomation"

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM customer_test_runs WHERE suite_api_name=$1`, suiteName)
		_, _ = pool.Exec(ctx, `DELETE FROM customer_tests WHERE api_name=$1`, suiteName)
		_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE payload->>'apiName'=$1`, autoName)
		_, _ = pool.Exec(ctx, `DELETE FROM records WHERE object_api_name = ANY($1::text[])`, []string{parent, child})
		_, _ = pool.Exec(ctx, `DELETE FROM automation_permissions WHERE automation_api_name=$1`, autoName)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_automations WHERE api_name=$1`, autoName)
		_, _ = pool.Exec(ctx, `DELETE FROM customer_source_files WHERE path LIKE 'src/automations/%' OR path LIKE 'tests/automations/%'`)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_fields WHERE object_api_name = ANY($1::text[])`, []string{parent, child})
		_, _ = pool.Exec(ctx, `DELETE FROM object_permissions WHERE object_api_name = ANY($1::text[])`, []string{parent, child})
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_objects WHERE api_name = ANY($1::text[])`, []string{parent, child})
	}
	cleanup()
	t.Cleanup(cleanup)

	for _, obj := range []struct{ api, label, plural string }{
		{parent, "Auto Step Parent", "Auto Step Parents"},
		{child, "Auto Step Child", "Auto Step Children"},
	} {
		if _, err := pool.Exec(ctx, `
INSERT INTO metadata_objects (api_name, label, plural_label, storage_mode, ownership, features)
VALUES ($1,$2,$3,'flexible','custom','{}'::jsonb)`, obj.api, obj.label, obj.plural); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `
INSERT INTO metadata_fields (object_api_name, api_name, label, field_type, required, ownership, filterable, sortable)
VALUES ($1,'Name','Name','text',true,'custom',true,true)`, obj.api); err != nil {
			t.Fatal(err)
		}
		_ = db.EnsureObjectInDataAccessCatalog(ctx, pool, obj.api)
	}
	_, _ = pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1 WHERE id = 1`)

	autoSrc := `
export default async function run(ctx) {
  await ctx.createRecord({
    objectApiName: "` + child + `",
    data: { Name: String(ctx.trigger.data?.Name || "") },
  });
  return { ok: true };
}
`
	testSrc := `
export default async function run(ctx) {
  await ctx.runUnderTest({
    trigger: {
      action: "create",
      objectApiName: "` + parent + `",
      recordId: "p1",
      data: { Name: "UnitAcme" },
    },
  });
  const { calls } = await ctx.getCalls({ method: "createRecord" });
  if (!calls || calls.length !== 1) throw new Error("expected 1 create");
  if (calls[0].objectApiName !== "` + child + `") throw new Error("wrong object");
  if (calls[0].data?.Name !== "UnitAcme") throw new Error("wrong name");
  return { ok: true };
}
`
	entry := "src/automations/create_child_on_parent.ts"
	testFile := "tests/automations/create_child_on_parent_test.ts"
	if _, err := pool.Exec(ctx, `
INSERT INTO metadata_automations (
  api_name, label, object_api_name, trigger_event, active, actions, ownership, package_name,
  runtime, execution, entry_file, source
) VALUES ($1,'x',$2,'create',true,'[]'::jsonb,'custom','customer.default','code','async',$3,$4)`,
		autoName, parent, entry, autoSrc); err != nil {
		t.Fatal(err)
	}
	_ = db.EnsureAutomationInAccessCatalog(ctx, pool, autoName)
	if err := deploy.UpsertCustomerSources(ctx, pool, map[string]string{
		entry:    autoSrc,
		testFile: testSrc,
	}, false); err != nil {
		t.Fatal(err)
	}

	steps := []any{
		map[string]any{"type": "objectExists", "objectApiName": parent},
		map[string]any{"type": "objectExists", "objectApiName": child},
		map[string]any{
			"type": "automationUnitPass", "automationApiName": autoName, "testFile": testFile,
		},
		map[string]any{
			"type": "automationContract", "automationApiName": autoName,
			"objectApiName": parent, "data": map[string]any{"Name": "ContractAcme"},
			"expectObjectApiName": child, "expectMinRows": 1,
			"filters": []any{map[string]any{"field": "Name", "op": "eq", "value": "ContractAcme"}},
		},
	}
	stepsJSON, _ := json.Marshal(steps)
	if _, err := pool.Exec(ctx, `
INSERT INTO customer_tests (api_name, label, active, steps, ownership, package_name)
VALUES ($1,'gate',true,$2::jsonb,'custom','customer.default')`, suiteName, string(stepsJSON)); err != nil {
		t.Fatal(err)
	}

	_ = meta
	data.ObjectAz = &authz.ObjectAuthz{Store: &db.ObjectPermStore{Pool: pool}}
	actor := &authz.Actor{
		ID:      ownerID,
		IsAdmin: true,
		Scopes:  []authz.Scope{authz.ScopeClient, authz.ScopeDeploy},
	}
	res, err := engine.StartTestRun(ctx, struct {
		SuiteAPIName string
		Actor        *authz.Actor
		Async        bool
		Trigger      string
	}{SuiteAPIName: suiteName, Actor: actor, Async: false, Trigger: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run == nil || res.Run.Status != "passed" {
		t.Fatalf("expected passed, got %+v", res.Run)
	}
}

func TestPackTemplateCreateAccountFromContactExample(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "customer-repo-template")
	if _, err := os.Stat(filepath.Join(root, "one.yaml")); err != nil {
		t.Skip("customer-repo-template not found")
	}
	art, _, err := customerrepo.PackFromDir(root, customerrepo.PackOptions{})
	if err != nil {
		t.Fatalf("pack template: %v", err)
	}
	foundObj := false
	for _, o := range art.Objects {
		if o.APIName == "Referral__c" {
			foundObj = true
		}
	}
	if !foundObj {
		t.Fatal("missing Referral__c in template pack")
	}
	foundLookups := 0
	for _, f := range art.Fields {
		if f.ObjectAPIName == "Referral__c" && (f.APIName == "ContactId" || f.APIName == "AccountId") {
			foundLookups++
			if f.FieldType != "lookup" {
				t.Fatalf("field %s type=%s", f.APIName, f.FieldType)
			}
		}
	}
	if foundLookups != 2 {
		t.Fatalf("expected ContactId+AccountId lookups, got %d", foundLookups)
	}
	foundAuto := false
	for _, a := range art.Automations {
		if a.APIName == "CreateAccount_From_Contact" {
			foundAuto = true
			if a.Runtime != "code" {
				t.Fatalf("runtime=%s", a.Runtime)
			}
		}
	}
	if !foundAuto {
		t.Fatal("missing CreateAccount_From_Contact in template pack")
	}
	if art.Sources["src/automations/create_account_from_contact.ts"] == "" {
		t.Fatal("missing automation source")
	}
	if art.Sources["tests/automations/create_account_from_contact_test.ts"] == "" {
		t.Fatal("missing unit test source")
	}
	foundSuite := false
	for _, s := range art.Tests {
		if s.APIName == "CreateAccountFromContact" {
			foundSuite = true
		}
	}
	if !foundSuite {
		t.Fatal("missing CreateAccountFromContact suite")
	}
}
