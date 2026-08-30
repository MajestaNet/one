package canvas

import (
	"encoding/json"
	"fmt"
	"strings"
)

const DocumentAPIVersion = "one.canvas/v1"

// ValidateDocument rejects unknown node kinds and requires one.canvas/v1 fields.
func ValidateDocument(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("document body is required")
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("document: invalid JSON")
	}
	apiVersion, _ := doc["apiVersion"].(string)
	if apiVersion != DocumentAPIVersion {
		return fmt.Errorf(`apiVersion must be %q`, DocumentAPIVersion)
	}
	id, _ := doc["id"].(string)
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("id is required")
	}
	title, _ := doc["title"].(string)
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("title is required")
	}
	layout, ok := doc["layout"].(map[string]any)
	if !ok {
		return fmt.Errorf("layout is required")
	}
	layoutJSON, _ := json.Marshal(layout)
	nodesJSON, _ := json.Marshal(doc["nodes"])
	bindingsJSON, _ := json.Marshal(doc["dataBindings"])
	if err := ValidateSpecBody(layoutJSON, nodesJSON, bindingsJSON); err != nil {
		return err
	}
	return nil
}
