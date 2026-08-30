package canvas_test

import (
	"encoding/json"
	"testing"

	"github.com/MajestaNet/ide/internal/canvas"
)

func TestSanitizeNodesJSONStripsBakedPayloads(t *testing.T) {
	raw := json.RawMessage(`[
	  {"id":"t","kind":"recordTable","props":{"columns":[{"key":"Name"}],"rows":[{"Name":"Secret"}]}},
	  {"id":"c","kind":"recordCard","props":{"fields":{"Name":"Secret"},"recordId":"a1"}},
	  {"id":"s","kind":"stat","props":{"value":9,"label":"Open"}},
	  {"id":"p","kind":"pipelineLane","props":{"stage":"Prospect","cards":[{"id":"1","title":"X"}]}},
	  {"id":"a","kind":"actionChipGroup","props":{"actions":[{"label":"Go","prompt":"hi"}]}}
	]`)
	out, err := canvas.SanitizeNodesJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	var list []map[string]any
	if err := json.Unmarshal(out, &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 5 {
		t.Fatalf("got %d nodes", len(list))
	}
	tableProps := list[0]["props"].(map[string]any)
	if _, ok := tableProps["rows"]; ok {
		t.Fatal("rows should be stripped")
	}
	if tableProps["columns"] == nil {
		t.Fatal("columns must be preserved")
	}
	cardProps := list[1]["props"].(map[string]any)
	if _, ok := cardProps["fields"]; ok {
		t.Fatal("fields should be stripped")
	}
	if _, ok := cardProps["recordId"]; ok {
		t.Fatal("recordId should be stripped")
	}
	statProps := list[2]["props"].(map[string]any)
	if statProps["value"] != nil {
		t.Fatalf("stat value should be null, got %#v", statProps["value"])
	}
	if statProps["label"] != "Open" {
		t.Fatal("stat label should remain")
	}
	laneProps := list[3]["props"].(map[string]any)
	if _, ok := laneProps["cards"]; ok {
		t.Fatal("cards should be stripped")
	}
	actions := list[4]["props"].(map[string]any)["actions"]
	if actions == nil {
		t.Fatal("action chips must remain")
	}
}
