package rungraph

import (
	"encoding/json"
	"fmt"
)

// MergePatch applies RFC 7396 JSON merge-patch semantics without adding a
// runtime dependency. Arrays (including nodes and edges) replace atomically.
func MergePatch(document, patch json.RawMessage) (json.RawMessage, error) {
	var target any
	if err := json.Unmarshal(document, &target); err != nil {
		return nil, fmt.Errorf("stored document: invalid JSON")
	}
	var delta any
	if err := json.Unmarshal(patch, &delta); err != nil {
		return nil, fmt.Errorf("patch: invalid JSON")
	}
	merged := mergeValue(target, delta)
	out, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("patch: merge: %w", err)
	}
	return out, nil
}

func mergeValue(target, patch any) any {
	patchObject, ok := patch.(map[string]any)
	if !ok {
		return patch
	}
	targetObject, ok := target.(map[string]any)
	if !ok {
		targetObject = map[string]any{}
	}
	for key, value := range patchObject {
		if value == nil {
			delete(targetObject, key)
			continue
		}
		targetObject[key] = mergeValue(targetObject[key], value)
	}
	return targetObject
}
