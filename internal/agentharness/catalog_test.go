package agentharness_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/agentharness"
)

func TestCatalogHasSixHarnessesFourSections(t *testing.T) {
	cats := agentharness.Catalog()
	if len(cats) != 6 {
		t.Fatalf("want 6 harnesses, got %d", len(cats))
	}
	seen := map[agentharness.Section]int{}
	for _, d := range cats {
		if d.ID == "" || d.Version == "" || len(d.ToolFloor) == 0 || d.SystemPreamble == "" {
			t.Fatalf("incomplete harness: %+v", d)
		}
		seen[d.Section]++
	}
	for _, want := range []agentharness.Section{
		agentharness.SectionOperate,
		agentharness.SectionBuild,
		agentharness.SectionGovern,
		agentharness.SectionSettings,
	} {
		if seen[want] < 1 {
			t.Fatalf("missing launcher section %s (counts %#v)", want, seen)
		}
	}
	if seen[agentharness.SectionBuild] < 3 {
		t.Fatalf("expected multiple build harnesses, got %d", seen[agentharness.SectionBuild])
	}
}

func TestBindRequiresValidSection(t *testing.T) {
	if _, err := agentharness.Bind(""); err == nil {
		t.Fatal("expected error for empty section")
	}
	if _, err := agentharness.Bind("crm"); err == nil {
		t.Fatal("expected error for unknown section")
	}
	b, err := agentharness.Bind("Operate")
	if err != nil {
		t.Fatal(err)
	}
	if b.PrimarySection != "operate" || b.HarnessID != "harness.run.tools" || b.HarnessVersion != agentharness.CatalogVersion {
		t.Fatalf("unexpected bind: %+v", b)
	}
	if b.JobClass != "" {
		t.Fatalf("section Bind must not invent jobClass, got %q", b.JobClass)
	}
	bShip, err := agentharness.Bind("ship")
	if err != nil {
		t.Fatal(err)
	}
	if bShip.PrimarySection != "build" {
		t.Fatalf("ship alias want build, got %s", bShip.PrimarySection)
	}
}

func TestRunHarnessPrefersReferenceOnlyGraphOperations(t *testing.T) {
	d, ok := agentharness.ForID("harness.run.tools")
	if !ok {
		t.Fatal("missing run harness")
	}
	for _, want := range []string{
		"graph.pin",
		"graph.pinCollection",
		"graph.annotate",
		"reference-only",
		"Never place query results",
		"refs-only selection",
		"staged proposal",
		"explain why honestly",
		"never put operations or data maps",
		"Curator",
		"Doer",
		"graph.publishSubgraph",
		"personal graph remains private",
	} {
		if !strings.Contains(d.SystemPreamble, want) {
			t.Fatalf("run preamble missing %q: %s", want, d.SystemPreamble)
		}
	}
	if !slices.Equal(d.ToolFloor, []string{"sobjects.read", "query"}) {
		t.Fatalf("Run tool floor must remain AuthZ-safe: %v", d.ToolFloor)
	}
	if !slices.Contains(d.ContextPackHints, "graphSelection") ||
		!slices.Contains(d.ContextPackHints, "signalBindings") ||
		!slices.Contains(d.ChromeHints, "proposalApply") ||
		!slices.Contains(d.ChromeHints, "publishCoach") {
		t.Fatalf("run harness missing graph selection/proposal hints: %+v", d)
	}
}

func TestUnionToolsPreservesFloor(t *testing.T) {
	out := agentharness.EnsureToolFloor(
		[]string{"sobjects.read", "query"},
		[]string{"query", "sobjects.write"},
	)
	if len(out) != 3 || out[0] != "sobjects.read" || out[1] != "query" || out[2] != "sobjects.write" {
		t.Fatalf("union=%v", out)
	}
}

func TestEffectiveRequireApprovalFloor(t *testing.T) {
	if !agentharness.EffectiveRequireApproval(true, false) {
		t.Fatal("harness default true must win")
	}
	if agentharness.EffectiveRequireApproval(false, false) {
		t.Fatal("both false")
	}
	if !agentharness.EffectiveRequireApproval(false, true) {
		t.Fatal("customer true when harness false")
	}
}

func TestStarterSectionMap(t *testing.T) {
	cases := map[string]agentharness.Section{
		"AdminSetup":      agentharness.SectionGovern,
		"MetadataBuilder": agentharness.SectionBuild,
		"RunCoach":        agentharness.SectionOperate,
		"ShipGuide":       agentharness.SectionBuild,
		"AccountGuide":    agentharness.SectionSettings,
	}
	for name, want := range cases {
		got, ok := agentharness.StarterSection(name)
		if !ok || got != want {
			t.Fatalf("%s: got %s ok=%v want %s", name, got, ok, want)
		}
	}
}

func TestBindSpecXORAndAliasMap(t *testing.T) {
	if _, err := agentharness.BindSpec("", ""); err == nil {
		t.Fatal("expected error when both empty")
	}
	b, err := agentharness.BindSpec("query", "")
	if err != nil {
		t.Fatal(err)
	}
	if b.JobClass != "query" || b.PrimarySection != "operate" || b.HarnessID != "harness.query.read" {
		t.Fatalf("jobClass only: %+v", b)
	}
	b2, err := agentharness.BindSpec("", "operate")
	if err != nil {
		t.Fatal(err)
	}
	if b2.JobClass != "query" || b2.PrimarySection != "operate" || b2.HarnessID != "harness.query.read" {
		t.Fatalf("section only: %+v", b2)
	}
	b3, err := agentharness.BindSpec("operate", "run")
	if err != nil {
		t.Fatal(err)
	}
	if b3.JobClass != "operate" || b3.PrimarySection != "run" || b3.HarnessID != "harness.operate.mutate" {
		t.Fatalf("run/operate: %+v", b3)
	}
	if _, err := agentharness.BindSpec("query", "build"); err == nil {
		t.Fatal("expected mismatch error")
	}
	settings, err := agentharness.BindSpec("govern", "settings")
	if err != nil {
		t.Fatal(err)
	}
	if settings.HarnessID != "harness.settings.install" || settings.JobClass != "govern" {
		t.Fatalf("settings: %+v", settings)
	}
	skill, err := agentharness.BindSpec("skill", "")
	if err != nil {
		t.Fatal(err)
	}
	if skill.HarnessID != "harness.skill.invoke" || skill.PrimarySection != "" {
		t.Fatalf("skill: %+v", skill)
	}
}

func TestJobCatalogHasSixClasses(t *testing.T) {
	cats := agentharness.JobCatalog()
	if len(cats) != 6 {
		t.Fatalf("want 6 job classes, got %d", len(cats))
	}
	seen := map[agentharness.JobClass]bool{}
	for _, d := range cats {
		if d.ID == "" || d.JobClass == "" || len(d.ToolFloor) == 0 {
			t.Fatalf("incomplete job harness: %+v", d)
		}
		seen[d.JobClass] = true
	}
	for _, want := range []agentharness.JobClass{
		agentharness.JobClassQuery,
		agentharness.JobClassCustomize,
		agentharness.JobClassShip,
		agentharness.JobClassGovern,
		agentharness.JobClassOperate,
		agentharness.JobClassSkill,
	} {
		if !seen[want] {
			t.Fatalf("missing job class %s", want)
		}
	}
}

func TestJobClassForSectionMap(t *testing.T) {
	cases := map[string]agentharness.JobClass{
		"operate":  agentharness.JobClassQuery,
		"run":      agentharness.JobClassOperate,
		"build":    agentharness.JobClassCustomize,
		"ship":     agentharness.JobClassShip,
		"govern":   agentharness.JobClassGovern,
		"settings": agentharness.JobClassGovern,
	}
	for sec, want := range cases {
		if got := agentharness.JobClassForSection(sec); got != want {
			t.Fatalf("%s: got %s want %s", sec, got, want)
		}
	}
}
