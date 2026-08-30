package deploy

import (
	"testing"
)

func TestParseBundleArtifactToolsDualRead(t *testing.T) {
	raw := map[string]any{
		"manifestVersion": 1,
		"ownership":       "custom",
		"objects":         []any{},
		"fields":          []any{},
		"validationRules": []any{},
		"automations":     []any{},
		"agentPlaybooks": []any{
			map[string]any{
				"apiName":          "Coach",
				"label":            "Coach",
				"goalTemplate":     "help",
				"instructions":     "help",
				"allowedTools":     []any{},
				"objectScopes":     []any{},
				"allowedToolSpecs": []any{"Pipeline_Tool"},
				"requireApproval":  false,
				"active":           true,
				"ownership":        "custom",
			},
		},
		"tools": []any{
			map[string]any{
				"apiName":   "Pipeline_Tool",
				"label":     "Pipeline",
				"layout":    map[string]any{"mode": "sections"},
				"nodes":     []any{},
				"active":    true,
				"ownership": "custom",
			},
		},
		"permissionSets": []any{},
		"webhooks":       []any{},
		"tests":          []any{},
	}
	art, err := ParseBundleArtifact(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(art.Canvases) != 1 || art.Canvases[0].APIName != "Pipeline_Tool" {
		t.Fatalf("expected tools→canvases dual-read, got %+v", art.Canvases)
	}
	pb := art.AgentPlaybooks[0]
	if len(pb.AllowedToolSpecs) != 1 || pb.AllowedToolSpecs[0] != "Pipeline_Tool" {
		t.Fatalf("allowedToolSpecs=%v", pb.AllowedToolSpecs)
	}
	if len(pb.AllowedCanvasSpecs) != 1 || pb.AllowedCanvasSpecs[0] != "Pipeline_Tool" {
		t.Fatalf("allowedCanvasSpecs dual=%v", pb.AllowedCanvasSpecs)
	}
}

func TestParseBundleArtifactAllowedCanvasSpecsFallback(t *testing.T) {
	raw := map[string]any{
		"manifestVersion": 1,
		"ownership":       "custom",
		"objects":         []any{},
		"fields":          []any{},
		"validationRules": []any{},
		"automations":     []any{},
		"agentPlaybooks": []any{
			map[string]any{
				"apiName":            "Legacy",
				"label":              "Legacy",
				"goalTemplate":       "g",
				"instructions":       "i",
				"allowedTools":       []any{},
				"objectScopes":       []any{},
				"allowedCanvasSpecs": []any{"Old_Canvas"},
				"requireApproval":    false,
				"active":             true,
				"ownership":          "custom",
			},
		},
		"canvases": []any{
			map[string]any{
				"apiName":   "Old_Canvas",
				"label":     "Old",
				"layout":    map[string]any{"mode": "sections"},
				"nodes":     []any{},
				"active":    true,
				"ownership": "custom",
			},
		},
		"permissionSets": []any{},
		"webhooks":       []any{},
		"tests":          []any{},
	}
	art, err := ParseBundleArtifact(raw)
	if err != nil {
		t.Fatal(err)
	}
	pb := art.AgentPlaybooks[0]
	if len(pb.AllowedToolSpecs) != 1 || pb.AllowedToolSpecs[0] != "Old_Canvas" {
		t.Fatalf("expected canvas fallback into toolSpecs, got %v", pb.AllowedToolSpecs)
	}
}

func TestParseBundleAgentPlaybookPrimarySectionYAML(t *testing.T) {
	raw := map[string]any{
		"manifestVersion": 1,
		"ownership":       "custom",
		"objects":         []any{},
		"fields":          []any{},
		"validationRules": []any{},
		"automations":     []any{},
		"agentPlaybooks": []any{
			map[string]any{
				"apiName":         "SectionOnly",
				"label":           "Section only",
				"goalTemplate":    "g",
				"instructions":    "i",
				"primarySection":  "build",
				"allowedTools":    []any{"sobjects.write"},
				"objectScopes":    []any{},
				"requireApproval": false,
				"active":          true,
				"ownership":       "custom",
			},
		},
		"permissionSets": []any{},
		"webhooks":       []any{},
		"tests":          []any{},
	}
	art, err := ParseBundleArtifact(raw)
	if err != nil {
		t.Fatal(err)
	}
	pb := art.AgentPlaybooks[0]
	if pb.JobClass != "" {
		t.Fatalf("legacy YAML must not invent jobClass, got %q", pb.JobClass)
	}
	if pb.PrimarySection != "build" || pb.HarnessID != "harness.operate.query" {
		t.Fatalf("primarySection YAML should use section catalog: %+v", pb)
	}
	foundRead := false
	for _, tool := range pb.AllowedTools {
		if tool == "sobjects.read" {
			foundRead = true
		}
	}
	if !foundRead {
		t.Fatalf("section floor dropped: %v", pb.AllowedTools)
	}
}

func TestParseBundleAgentPlaybookJobClassYAML(t *testing.T) {
	raw := map[string]any{
		"manifestVersion": 1,
		"ownership":       "custom",
		"objects":         []any{},
		"fields":          []any{},
		"validationRules": []any{},
		"automations":     []any{},
		"agentPlaybooks": []any{
			map[string]any{
				"apiName":      "JobOnly",
				"label":        "Job only",
				"jobClass":     "customize",
				"allowedTools": []any{},
				"active":       true,
				"ownership":    "custom",
			},
		},
		"permissionSets": []any{},
		"webhooks":       []any{},
		"tests":          []any{},
	}
	art, err := ParseBundleArtifact(raw)
	if err != nil {
		t.Fatal(err)
	}
	pb := art.AgentPlaybooks[0]
	if pb.JobClass != "customize" || pb.PrimarySection != "build" || pb.HarnessID != "harness.customize.metadata" {
		t.Fatalf("jobClass YAML: %+v", pb)
	}
}
