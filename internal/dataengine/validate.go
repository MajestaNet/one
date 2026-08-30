package dataengine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/diegoholiveira/jsonlogic/v3"
)

var systemInputFields = map[string]struct{}{
	"Id": {}, "OwnerId": {}, "CreatedAt": {}, "UpdatedAt": {},
	"CreatedById": {}, "LastModifiedById": {},
}

// immutableAuditFields cannot be set by clients; platform populates them automatically.
var immutableAuditFields = map[string]struct{}{
	"CreatedById": {}, "LastModifiedById": {}, "CreatedAt": {}, "UpdatedAt": {}, "Id": {},
}

var (
	timeOfDayRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d:[0-5]\d(\.\d{1,9})?$`)
	scriptTagRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleTagRe  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	onAttrRe    = regexp.MustCompile(`(?i)\son[a-z]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
)

// RejectImmutableSystemFields errors when clients attempt to set platform-owned audit fields.
func RejectImmutableSystemFields(input map[string]any) error {
	for key := range input {
		if _, bad := immutableAuditFields[key]; bad {
			return validationErrorf("Field %s is read-only and cannot be set", key)
		}
	}
	return nil
}

// NormalizeAndValidateFields ports packages/metadata-kernel normalizeAndValidateFields.
func NormalizeAndValidateFields(fields []metadata.FieldDefinition, input map[string]any, mode string) (map[string]any, error) {
	fieldMap := make(map[string]metadata.FieldDefinition, len(fields))
	for _, f := range fields {
		fieldMap[f.APIName] = f
	}
	result := map[string]any{}

	for key, value := range input {
		if len(key) > 0 && key[0] == '_' {
			continue
		}
		if _, skip := systemInputFields[key]; skip {
			continue
		}
		def, ok := fieldMap[key]
		if !ok {
			return nil, validationErrorf("Unknown field: %s", key)
		}
		if def.FieldType == metadata.FieldTypeAutonumber {
			return nil, validationErrorf("Field %s is autonumber and cannot be set", key)
		}
		result[key] = coerceValue(def, value)
	}

	if mode == "create" {
		for _, def := range fields {
			if _, ok := result[def.APIName]; ok {
				continue
			}
			if def.FieldType == metadata.FieldTypeAutonumber {
				continue // allocated separately
			}
			if def.DefaultValue != nil && string(def.DefaultValue) != "null" && len(def.DefaultValue) > 0 {
				var dv any
				if err := json.Unmarshal(def.DefaultValue, &dv); err == nil && dv != nil {
					result[def.APIName] = dv
					continue
				}
			}
			if def.Required {
				return nil, validationErrorf("Missing required field: %s", def.APIName)
			}
		}
	}

	for _, def := range fields {
		if v, ok := result[def.APIName]; ok {
			if err := assertFieldValue(def, v); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func coerceValue(def metadata.FieldDefinition, value any) any {
	if value == nil {
		return nil
	}
	switch def.FieldType {
	case metadata.FieldTypeNumber, metadata.FieldTypeCurrency, metadata.FieldTypePercent, metadata.FieldTypeInteger:
		switch v := value.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case json.Number:
			f, err := v.Float64()
			if err != nil {
				return math.NaN()
			}
			return f
		case string:
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return math.NaN()
			}
			return f
		default:
			f, err := strconv.ParseFloat(fmt.Sprint(v), 64)
			if err != nil {
				return math.NaN()
			}
			return f
		}
	case metadata.FieldTypeBoolean:
		switch v := value.(type) {
		case bool:
			return v
		case string:
			return v != "" && v != "false" && v != "0"
		case float64:
			return v != 0
		case float32:
			return v != 0
		case int:
			return v != 0
		case int64:
			return v != 0
		case json.Number:
			f, err := v.Float64()
			return err == nil && f != 0
		default:
			return value != nil
		}
	case metadata.FieldTypeJSON, metadata.FieldTypeAddress, metadata.FieldTypeGeolocation:
		return value
	case metadata.FieldTypeRichText:
		if s, ok := value.(string); ok {
			return sanitizeRichText(s)
		}
		return value
	default:
		return value
	}
}

func assertFieldValue(def metadata.FieldDefinition, value any) error {
	if value == nil {
		if def.Required {
			return validationErrorf("Field %s is required", def.APIName)
		}
		return nil
	}
	switch def.FieldType {
	case metadata.FieldTypeText, metadata.FieldTypeTextarea, metadata.FieldTypeEmail, metadata.FieldTypePhone,
		metadata.FieldTypeURL, metadata.FieldTypePicklist, metadata.FieldTypeLookup, metadata.FieldTypeMasterDetail,
		metadata.FieldTypeDate, metadata.FieldTypeDateTime, metadata.FieldTypeTime,
		metadata.FieldTypeRichText, metadata.FieldTypeAutonumber:
		s, ok := value.(string)
		if !ok {
			return validationErrorf("Field %s must be a string", def.APIName)
		}
		if def.Length != nil && len(s) > *def.Length {
			return validationErrorf("Field %s exceeds max length %d", def.APIName, *def.Length)
		}
		switch def.FieldType {
		case metadata.FieldTypePicklist:
			if len(def.PicklistValues) > 0 {
				found := false
				for _, p := range def.PicklistValues {
					if p == s {
						found = true
						break
					}
				}
				if !found {
					return validationErrorf("Invalid picklist value for %s", def.APIName)
				}
			}
		case metadata.FieldTypeLookup, metadata.FieldTypeMasterDetail:
			if !looksLikeUUID(s) {
				return validationErrorf("Field %s must be a UUID", def.APIName)
			}
		case metadata.FieldTypeEmail:
			if _, err := mail.ParseAddress(s); err != nil {
				return validationErrorf("Field %s must be a valid email", def.APIName)
			}
		case metadata.FieldTypeURL:
			u, err := url.ParseRequestURI(s)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				return validationErrorf("Field %s must be a valid http(s) URL", def.APIName)
			}
		case metadata.FieldTypeDate:
			if _, err := time.Parse("2006-01-02", s); err != nil {
				if _, err2 := time.Parse(time.RFC3339, s); err2 != nil {
					return validationErrorf("Field %s must be a date (YYYY-MM-DD)", def.APIName)
				}
			}
		case metadata.FieldTypeDateTime:
			if _, err := time.Parse(time.RFC3339, s); err != nil {
				if _, err2 := time.Parse(time.RFC3339Nano, s); err2 != nil {
					return validationErrorf("Field %s must be an RFC3339 datetime", def.APIName)
				}
			}
		case metadata.FieldTypeTime:
			if !timeOfDayRe.MatchString(s) {
				return validationErrorf("Field %s must be HH:MM:SS", def.APIName)
			}
		}
	case metadata.FieldTypeNumber, metadata.FieldTypeCurrency, metadata.FieldTypePercent:
		f, ok := value.(float64)
		if !ok || math.IsNaN(f) {
			return validationErrorf("Field %s must be a number", def.APIName)
		}
		if err := assertPrecisionScale(def, f); err != nil {
			return err
		}
	case metadata.FieldTypeInteger:
		f, ok := value.(float64)
		if !ok || math.IsNaN(f) || f != math.Trunc(f) {
			return validationErrorf("Field %s must be an integer", def.APIName)
		}
	case metadata.FieldTypeBoolean:
		if _, ok := value.(bool); !ok {
			return validationErrorf("Field %s must be a boolean", def.APIName)
		}
	case metadata.FieldTypeJSON:
		// any JSON value ok
	case metadata.FieldTypeAddress:
		return assertCompoundObject(def.APIName, value, metadata.AddressComponentKeys, false)
	case metadata.FieldTypeGeolocation:
		if err := assertCompoundObject(def.APIName, value, metadata.GeolocationComponentKeys, true); err != nil {
			return err
		}
		m := value.(map[string]any)
		lat, _ := toFloat(m["latitude"])
		lng, _ := toFloat(m["longitude"])
		if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
			return validationErrorf("Field %s latitude/longitude out of range", def.APIName)
		}
	}
	return nil
}

func assertPrecisionScale(def metadata.FieldDefinition, f float64) error {
	if def.Precision == nil {
		return nil
	}
	s := fmt.Sprintf("%v", f)
	s = strings.TrimPrefix(s, "-")
	parts := strings.SplitN(s, ".", 2)
	digits := len(strings.ReplaceAll(parts[0], ".", ""))
	scale := 0
	if len(parts) == 2 {
		scale = len(parts[1])
		digits += scale
	}
	if digits > *def.Precision {
		return validationErrorf("Field %s exceeds precision %d", def.APIName, *def.Precision)
	}
	if def.Scale != nil && scale > *def.Scale {
		return validationErrorf("Field %s exceeds scale %d", def.APIName, *def.Scale)
	}
	return nil
}

func assertCompoundObject(field string, value any, allowed []string, requireAll bool) error {
	m, ok := value.(map[string]any)
	if !ok {
		return validationErrorf("Field %s must be an object", field)
	}
	allowedSet := map[string]struct{}{}
	for _, k := range allowed {
		allowedSet[k] = struct{}{}
	}
	for k := range m {
		if _, ok := allowedSet[k]; !ok {
			return validationErrorf("Field %s has unknown component %s", field, k)
		}
	}
	if requireAll {
		for _, k := range allowed {
			if _, ok := m[k]; !ok {
				return validationErrorf("Field %s requires component %s", field, k)
			}
		}
	}
	return nil
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func sanitizeRichText(s string) string {
	s = scriptTagRe.ReplaceAllString(s, "")
	s = styleTagRe.ReplaceAllString(s, "")
	s = onAttrRe.ReplaceAllString(s, "")
	return s
}

func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}

// EvaluateValidationRules applies JSONLogic rules (true => invalid).
func EvaluateValidationRules(rules []metadata.ValidationRuleDefinition, data map[string]any) error {
	for _, rule := range rules {
		if !rule.Active {
			continue
		}
		if len(rule.Expression) == 0 || string(rule.Expression) == "null" {
			continue
		}
		var expr any
		if err := json.Unmarshal(rule.Expression, &expr); err != nil {
			return validationErrorf("Invalid validation rule expression for %s", rule.APIName)
		}
		ruleJSON, err := json.Marshal(expr)
		if err != nil {
			return validationErrorf("Invalid validation rule expression for %s", rule.APIName)
		}
		dataJSON, err := json.Marshal(data)
		if err != nil {
			return validationErrorf("Validation rule %s failed to evaluate: %v", rule.APIName, err)
		}
		var out bytes.Buffer
		if err := jsonlogic.Apply(bytes.NewReader(ruleJSON), bytes.NewReader(dataJSON), &out); err != nil {
			return validationErrorf("Validation rule %s failed to evaluate: %v", rule.APIName, err)
		}
		var result any
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			return validationErrorf("Validation rule %s failed to evaluate: %v", rule.APIName, err)
		}
		if result == true {
			return &ValidationError{Message: rule.ErrorMessage, Details: map[string]any{"expression": expr}}
		}
	}
	return nil
}
