package customerrepo_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/customerrepo"
)

func TestSelectivePackIncludePaths(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "one.yaml"), `
customerId: acme
packageName: customer.default
repoFormat: one/v1
`)
	mustWrite(t, filepath.Join(root, "metadata", "objects", "Keep__c.yaml"), `
apiName: Keep__c
label: Keep
pluralLabel: Keeps
storageMode: flexible
ownership: custom
`)
	mustWrite(t, filepath.Join(root, "metadata", "objects", "Skip__c.yaml"), `
apiName: Skip__c
label: Skip
pluralLabel: Skips
storageMode: flexible
ownership: custom
`)
	mustWrite(t, filepath.Join(root, "manifests", "keep.yaml"), `
paths:
  - metadata/objects/Keep__c.yaml
`)

	art, _, err := customerrepo.PackFromDir(root, customerrepo.PackOptions{
		ManifestName: "keep",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(art.Objects) != 1 || art.Objects[0].APIName != "Keep__c" {
		t.Fatalf("expected only Keep__c, got %+v", art.Objects)
	}
}

func TestLoadEnvironmentsOrder(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "environments", "prod.yaml"), `
installId: p1
installRole: prod
baseUrl: https://prod.example
`)
	mustWrite(t, filepath.Join(root, "environments", "test.yaml"), `
installId: t1
installRole: test
baseUrl: https://test.example
`)
	envs, err := customerrepo.LoadEnvironments(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(envs) != 2 || envs[0].Alias != "test" || envs[1].Alias != "prod" {
		t.Fatalf("order: %+v", envs)
	}
}

func TestCreateChangeAndInitProject(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "proj")
	if err := customerrepo.InitProject(sub, "acme", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(sub, "one.yaml")); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"AGENTS.md",
		"skills/connect/SKILL.md",
		"skills/query/SKILL.md",
		"skills/customize/SKILL.md",
		"skills/ship/SKILL.md",
		"skills/govern/SKILL.md",
		"skills/skill/SKILL.md",
		"metadata/tools/Open_Opportunities_By_Stage.yaml",
		"metadata/tools/Top_Accounts_Overview.yaml",
	} {
		b, err := os.ReadFile(filepath.Join(sub, rel))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if len(b) == 0 {
			t.Fatalf("%s is empty", rel)
		}
	}
	agents, err := os.ReadFile(filepath.Join(sub, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(agents)
	if !strings.Contains(body, "POST /mcp") || !strings.Contains(body, "one org validate") {
		t.Fatalf("AGENTS.md missing builder connect/ship language: %s", body)
	}
	if !strings.Contains(body, "skills/skill/SKILL.md") || !strings.Contains(body, "`skill`") {
		t.Fatalf("AGENTS.md missing skill job-class row: %s", body)
	}
	if strings.Contains(body, "internal/") || strings.Contains(body, "BP-") || strings.Contains(body, ".cursor/agents") {
		t.Fatalf("customer AGENTS.md must not reference vendor paths: %s", body)
	}
	p, err := customerrepo.CreateChange(sub, "add-field", customerrepo.ChangeMeta{
		Title: "Add field", Summary: "demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
}
