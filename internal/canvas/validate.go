package canvas

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AllowedNodeKinds matches ADR-018 / tools/control-ide canvas/types.ts.
var AllowedNodeKinds = map[string]struct{}{
	"stat":             {},
	"recordTable":      {},
	"recordCard":       {},
	"relatedList":      {},
	"queryResult":      {},
	"mutationProposal": {},
	"messageThread":    {},
	"pipelineLane":     {},
	"actionChipGroup":  {},
	"markdownNote":     {},
	"sectionHeader":    {},
}

// SpecBody is the layout + nodes + bindings stored on metadata_canvases (no runtime id).
type SpecBody struct {
	Layout       json.RawMessage `json:"layout"`
	Nodes        json.RawMessage `json:"nodes"`
	DataBindings json.RawMessage `json:"dataBindings,omitempty"`
}

// ValidateSpecBody rejects unknown node kinds and requires layout.mode.
func ValidateSpecBody(layout, nodes, dataBindings json.RawMessage) error {
	if len(layout) == 0 {
		return fmt.Errorf("layout is required")
	}
	var layoutObj map[string]any
	if err := json.Unmarshal(layout, &layoutObj); err != nil {
		return fmt.Errorf("layout: invalid JSON")
	}
	mode, _ := layoutObj["mode"].(string)
	if mode != "sections" && mode != "spatial" {
		return fmt.Errorf(`layout.mode must be "sections" or "spatial"`)
	}
	if len(nodes) == 0 {
		return fmt.Errorf("nodes must be a non-empty array")
	}
	var nodeList []map[string]any
	if err := json.Unmarshal(nodes, &nodeList); err != nil {
		return fmt.Errorf("nodes: must be an array")
	}
	seen := map[string]struct{}{}
	for i, n := range nodeList {
		id, _ := n["id"].(string)
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("nodes[%d].id is required", i)
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("duplicate node id %q", id)
		}
		seen[id] = struct{}{}
		kind, _ := n["kind"].(string)
		if _, ok := AllowedNodeKinds[kind]; !ok {
			return fmt.Errorf("nodes[%d].kind %q is not allowlisted (ADR-018)", i, kind)
		}
		props, ok := n["props"].(map[string]any)
		if !ok {
			return fmt.Errorf("nodes[%d].props must be an object", i)
		}
		if props == nil {
			return fmt.Errorf("nodes[%d].props must be an object", i)
		}
	}
	if len(dataBindings) > 0 {
		var bindings []map[string]any
		if err := json.Unmarshal(dataBindings, &bindings); err != nil {
			return fmt.Errorf("dataBindings: must be an array")
		}
		for i, b := range bindings {
			id, _ := b["id"].(string)
			obj, _ := b["objectApiName"].(string)
			if strings.TrimSpace(id) == "" {
				return fmt.Errorf("dataBindings[%d].id is required", i)
			}
			if strings.TrimSpace(obj) == "" {
				return fmt.Errorf("dataBindings[%d].objectApiName is required", i)
			}
		}
	}
	return nil
}
