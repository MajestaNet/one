package canvas_test

import (
	"encoding/json"
	"testing"

	"github.com/MajestaNet/ide/internal/canvas"
)

func TestValidateSpecBodyRejectsUnknownKind(t *testing.T) {
	layout := json.RawMessage(`{"mode":"sections"}`)
	nodes := json.RawMessage(`[{"id":"x","kind":"rawHtml","props":{}}]`)
	err := canvas.ValidateSpecBody(layout, nodes, nil)
	if err == nil {
		t.Fatal("expected error for rawHtml")
	}
}

func TestValidateSpecBodyAcceptsPhase1Kinds(t *testing.T) {
	layout := json.RawMessage(`{"mode":"sections","sections":[{"id":"s1","nodeIds":["n1"]}]}`)
	nodes := json.RawMessage(`[{"id":"n1","kind":"stat","props":{"value":1}}]`)
	if err := canvas.ValidateSpecBody(layout, nodes, nil); err != nil {
		t.Fatal(err)
	}
}
