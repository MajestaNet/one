package actions

import (
	"encoding/json"
	"fmt"
	"strings"
)

func validateInputSchema(schemaJSON string, input map[string]any) error {
	if strings.TrimSpace(schemaJSON) == "" {
		return nil
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		return errValidation("action input schema is invalid")
	}
	if input == nil {
		input = map[string]any{}
	}
	return validateValue(schema, input, "")
}

func validateValue(schema map[string]any, value any, path string) error {
	typ, _ := schema["type"].(string)
	switch typ {
	case "object", "":
		obj, ok := value.(map[string]any)
		if !ok {
			return errValidation(fmt.Sprintf("%s must be an object", label(path)))
		}
		if required, ok := schema["required"].([]any); ok {
			for _, raw := range required {
				key, _ := raw.(string)
				if key == "" {
					continue
				}
				v, exists := obj[key]
				if !exists || v == nil {
					return errValidation(fmt.Sprintf("%s is required", joinPath(path, key)))
				}
				if s, isStr := v.(string); isStr && strings.TrimSpace(s) == "" {
					if minLen, _ := propertySchema(schema, key)["minLength"].(float64); minLen > 0 {
						return errValidation(fmt.Sprintf("%s is required", joinPath(path, key)))
					}
				}
			}
		}
		additional := true
		if v, ok := schema["additionalProperties"].(bool); ok {
			additional = v
		}
		props, _ := schema["properties"].(map[string]any)
		for key, val := range obj {
			ps := propertySchema(schema, key)
			if len(ps) == 0 {
				if !additional {
					return errValidation(fmt.Sprintf("unknown property %s", joinPath(path, key)))
				}
				continue
			}
			if err := validateValue(ps, val, joinPath(path, key)); err != nil {
				return err
			}
		}
		_ = props
	case "string":
		s, ok := value.(string)
		if !ok {
			return errValidation(fmt.Sprintf("%s must be a string", label(path)))
		}
		if minLen, ok := schema["minLength"].(float64); ok && len(s) < int(minLen) {
			return errValidation(fmt.Sprintf("%s is required", label(path)))
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return errValidation(fmt.Sprintf("%s must be a boolean", label(path)))
		}
	}
	return nil
}

func propertySchema(schema map[string]any, key string) map[string]any {
	props, _ := schema["properties"].(map[string]any)
	if props == nil {
		return nil
	}
	raw, ok := props[key]
	if !ok {
		return nil
	}
	ps, _ := raw.(map[string]any)
	return ps
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func label(path string) string {
	if path == "" {
		return "input"
	}
	return path
}

func parseSchemaJSON(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return raw
	}
	return v
}
