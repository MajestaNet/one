package canvas

import (
	"encoding/json"
)

// bakedPropKeys are never durable Metadata — they are Client AuthZ snapshots.
var bakedPropKeys = []string{
	"rows",
	"fields",
	"recordIds",
	"cards",
	"messages",
	"recordId",
	"value",
	"operations",
}

// SanitizeNodesJSON strips baked record payloads from ToolSpec/Canvas nodes before Metadata persist.
// Layout chrome, columns, bindings, and prompts are preserved. Empty/invalid JSON is returned unchanged.
func SanitizeNodesJSON(nodes json.RawMessage) (json.RawMessage, error) {
	if len(nodes) == 0 {
		return nodes, nil
	}
	var list []map[string]any
	if err := json.Unmarshal(nodes, &list); err != nil {
		return nodes, nil // leave invalid payloads for ValidateSpecBody
	}
	for _, node := range list {
		props, ok := node["props"].(map[string]any)
		if !ok || props == nil {
			continue
		}
		for _, key := range bakedPropKeys {
			delete(props, key)
		}
		if kind, _ := node["kind"].(string); kind == "stat" {
			props["value"] = nil
		}
		node["props"] = props
	}
	out, err := json.Marshal(list)
	if err != nil {
		return nil, err
	}
	return out, nil
}
