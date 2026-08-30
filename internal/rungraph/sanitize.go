package rungraph

import (
	"encoding/json"
	"fmt"
)

type sanitizeContext struct {
	parentKey string
	inBinding bool
}

// SanitizeDocument recursively strips baked Client payload keys. Binding
// definitions retain fields/filter values, and record refs retain recordId.
func SanitizeDocument(raw json.RawMessage) (json.RawMessage, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("document: invalid JSON")
	}
	sanitizeValue(value, sanitizeContext{})
	out, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("document: sanitize: %w", err)
	}
	return out, nil
}

func sanitizeValue(value any, ctx sanitizeContext) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			childInBinding := ctx.inBinding || key == "dataBindings"
			if _, baked := bakedPayloadKeys[key]; baked && !keepBakedKey(key, ctx) {
				delete(typed, key)
				continue
			}
			sanitizeValue(child, sanitizeContext{parentKey: key, inBinding: childInBinding})
		}
	case []any:
		for _, child := range typed {
			sanitizeValue(child, ctx)
		}
	}
}

func keepBakedKey(key string, ctx sanitizeContext) bool {
	if key == "recordId" && ctx.parentKey == "ref" {
		return true
	}
	if !ctx.inBinding {
		return false
	}
	// These names are legitimate parts of query definitions. Result-shaped
	// keys remain forbidden even inside a binding.
	switch key {
	case "fields", "recordIds", "recordId", "value":
		return true
	default:
		return false
	}
}
