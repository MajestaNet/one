package datapack_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MajestaNet/ide/internal/datapack"
)

func TestValidatePeerSourcedPack(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "environments", "test.yaml"), `
installId: acme-test
installRole: test
baseUrl: https://test.example
`)
	packDir := filepath.Join(root, "data", "crm-seed")
	mustWrite(t, filepath.Join(packDir, "datapack.yaml"), `
apiVersion: one-datapack/v1
name: crm-seed
sourceEnv: test
steps:
  - id: accounts
    object: Account
    operation: upsert
    externalIdField: ERP_Id__c
    query:
      select: [Id, Name, ERP_Id__c]
  - id: contacts
    object: Contact
    operation: upsert
    externalIdField: ERP_Id__c
    after: [accounts]
    query:
      select: [Id, ERP_Id__c, AccountId]
    references:
      - field: AccountId
        toObject: Account
        toExternalIdField: ERP_Id__c
`)
	m, dir, err := datapack.LoadManifest(packDir)
	if err != nil {
		t.Fatal(err)
	}
	if errs := datapack.Validate(m, dir, root); len(errs) > 0 {
		t.Fatalf("validate: %v", errs)
	}
	ordered, err := datapack.OrderSteps(m.Steps)
	if err != nil {
		t.Fatal(err)
	}
	if ordered[0].ID != "accounts" || ordered[1].ID != "contacts" {
		t.Fatalf("order=%v", ordered)
	}
}

func TestValidateMissingSourceEnv(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "environments"), 0o755)
	packDir := filepath.Join(root, "data", "x")
	mustWrite(t, filepath.Join(packDir, "datapack.yaml"), `
apiVersion: one-datapack/v1
name: x
sourceEnv: missing
steps:
  - id: a
    object: Account
    operation: upsert
    externalIdField: ERP_Id__c
    query:
      select: [Id]
`)
	m, dir, err := datapack.LoadManifest(packDir)
	if err != nil {
		t.Fatal(err)
	}
	errs := datapack.Validate(m, dir, root)
	if len(errs) == 0 {
		t.Fatal("expected sourceEnv error")
	}
}

func TestValidateRejectsNilDuplicateAndCycle(t *testing.T) {
	if errs := datapack.Validate(nil, "", ""); len(errs) == 0 {
		t.Fatal("expected nil manifest error")
	}
	m := &datapack.Manifest{
		APIVersion: "wrong",
		Steps: []datapack.Step{
			{ID: "a", Object: "Account", Operation: "upsert", ExternalIDField: "ERP_Id__c", File: "a.jsonl", After: []string{"b"}},
			{ID: "a", Object: "Contact", Operation: "nope"},
		},
	}
	errs := datapack.Validate(m, t.TempDir(), "")
	if len(errs) == 0 {
		t.Fatal("expected validation errors")
	}
	cycle := []datapack.Step{
		{ID: "a", Object: "Account", Operation: "insert", File: "a.jsonl", After: []string{"b"}},
		{ID: "b", Object: "Contact", Operation: "insert", File: "b.jsonl", After: []string{"a"}},
	}
	if _, err := datapack.OrderSteps(cycle); err == nil {
		t.Fatal("expected cycle")
	}
	if _, err := datapack.OrderSteps([]datapack.Step{
		{ID: "child", Object: "Contact", After: []string{"missing"}},
	}); err == nil {
		t.Fatal("expected unknown after")
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
