package customerrepo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MajestaNet/ide/internal/canvas"
	"github.com/MajestaNet/ide/internal/deploy"
)

func TestParseCanvasSpecYAMLAndRoundTrip(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "deploy", "customer-repo-template", "metadata", "tools", "Open_Opportunities_By_Stage.yaml"))
	if err != nil {
		t.Skip(err)
	}
	cs, err := parseCanvasSpecYAML(raw)
	if err != nil {
		t.Fatal(err)
	}
	if cs.APIName != "Open_Opportunities_By_Stage" {
		t.Fatalf("apiName=%s", cs.APIName)
	}
	if err := canvas.ValidateSpecBody(cs.Layout, cs.Nodes, cs.DataBindings); err != nil {
		t.Fatalf("validate: %v", err)
	}

	dir := t.TempDir()
	art := &deploy.BundleArtifact{
		Canvases: []deploy.SnapshotCanvasSpec{cs},
	}
	if err := UnpackToDir(dir, art, Manifest{PackageName: "customer.default"}); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "metadata", "tools", cs.APIName+".yaml")
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	again, err := parseCanvasSpecYAML(body)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if again.APIName != cs.APIName || again.Label != cs.Label {
		t.Fatalf("round-trip mismatch: %+v", again)
	}
	if err := canvas.ValidateSpecBody(again.Layout, again.Nodes, again.DataBindings); err != nil {
		t.Fatalf("re-validate: %v", err)
	}
}

func TestPackPrefersToolsOverDeprecatedCanvases(t *testing.T) {
	root := t.TempDir()
	writeFile := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("one.yaml", "packageName: customer.default\nrepoFormat: one/v1\n")
	toolYAML := `apiName: Shared_Tool
label: From tools
layout:
  mode: sections
  sections: []
nodes: []
dataBindings: []
active: true
ownership: custom
`
	canvasYAML := `apiName: Shared_Tool
label: From canvases
layout:
  mode: sections
  sections: []
nodes: []
dataBindings: []
active: true
ownership: custom
`
	writeFile("metadata/tools/Shared_Tool.yaml", toolYAML)
	writeFile("metadata/canvases/Shared_Tool.yaml", canvasYAML)
	writeFile("metadata/canvases/Legacy_Only.yaml", `apiName: Legacy_Only
label: Legacy
layout:
  mode: sections
  sections: []
nodes: []
dataBindings: []
active: true
ownership: custom
`)

	art, _, err := PackFromDir(root, PackOptions{})
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]deploy.SnapshotCanvasSpec{}
	for _, cs := range art.Canvases {
		byName[cs.APIName] = cs
	}
	if got := byName["Shared_Tool"].Label; got != "From tools" {
		t.Fatalf("expected tools/ to win, got label=%q", got)
	}
	if _, ok := byName["Legacy_Only"]; !ok {
		t.Fatal("expected deprecated canvases/ still packed when unique")
	}
}
