package metadata

import (
	"fmt"
	"strings"
)

// Canonical field types (Majesta One catalog). No multipicklist.
const (
	FieldTypeText         = "text"
	FieldTypeTextarea     = "textarea"
	FieldTypeEmail        = "email"
	FieldTypePhone        = "phone"
	FieldTypeURL          = "url"
	FieldTypeBoolean      = "boolean"
	FieldTypeInteger      = "integer"
	FieldTypeNumber       = "number"
	FieldTypeCurrency     = "currency"
	FieldTypePercent      = "percent"
	FieldTypeDate         = "date"
	FieldTypeDateTime     = "datetime"
	FieldTypeTime         = "time"
	FieldTypePicklist     = "picklist"
	FieldTypeLookup       = "lookup"
	FieldTypeMasterDetail = "master_detail"
	FieldTypeJSON         = "json"
	FieldTypeAutonumber   = "autonumber"
	FieldTypeRichText     = "richtext"
	FieldTypeAddress      = "address"
	FieldTypeGeolocation  = "geolocation"
)

// FieldTypeInfo describes one canonical type for GET /metadata/v1/field-types.
type FieldTypeInfo struct {
	APIName             string   `json:"apiName"`
	Label               string   `json:"label"`
	Category            string   `json:"category"` // core | enhancement
	DefaultFilterable   bool     `json:"defaultFilterable"`
	DefaultSortable     bool     `json:"defaultSortable"`
	DefaultIndexed      bool     `json:"defaultIndexed"`
	RequiresReferenceTo bool     `json:"requiresReferenceTo"`
	SupportsPicklist    bool     `json:"supportsPicklistValues"`
	SupportsLength      bool     `json:"supportsLength"`
	SupportsPrecision   bool     `json:"supportsPrecisionScale"`
	SupportsAutonumber  bool     `json:"supportsAutonumber"`
	CompoundComponents  []string `json:"compoundComponents,omitempty"`
	QueryCast           string   `json:"queryCast"`
	Notes               string   `json:"notes,omitempty"`
}

var fieldTypeAliasMap = map[string]string{
	"string":    FieldTypeText,
	"double":    FieldTypeNumber,
	"reference": FieldTypeLookup,
	"int":       FieldTypeInteger,
	"long":      FieldTypeInteger,
	"float":     FieldTypeNumber,
	"checkbox":  FieldTypeBoolean,
	"currency":  FieldTypeCurrency, // identity
}

var canonicalFieldTypes = []FieldTypeInfo{
	{APIName: FieldTypeText, Label: "Text", Category: "core", DefaultFilterable: true, DefaultSortable: true, SupportsLength: true, QueryCast: "text"},
	{APIName: FieldTypeTextarea, Label: "Text Area", Category: "core", DefaultFilterable: false, DefaultSortable: false, SupportsLength: true, QueryCast: "text", Notes: "Long text; default length 32000"},
	{APIName: FieldTypeEmail, Label: "Email", Category: "core", DefaultFilterable: true, DefaultSortable: true, SupportsLength: true, QueryCast: "text"},
	{APIName: FieldTypePhone, Label: "Phone", Category: "core", DefaultFilterable: true, DefaultSortable: true, SupportsLength: true, QueryCast: "text"},
	{APIName: FieldTypeURL, Label: "URL", Category: "core", DefaultFilterable: true, DefaultSortable: true, SupportsLength: true, QueryCast: "text"},
	{APIName: FieldTypeBoolean, Label: "Checkbox", Category: "core", DefaultFilterable: true, DefaultSortable: true, QueryCast: "boolean"},
	{APIName: FieldTypeInteger, Label: "Integer", Category: "core", DefaultFilterable: true, DefaultSortable: true, QueryCast: "bigint"},
	{APIName: FieldTypeNumber, Label: "Number", Category: "core", DefaultFilterable: true, DefaultSortable: true, SupportsPrecision: true, QueryCast: "numeric"},
	{APIName: FieldTypeCurrency, Label: "Currency", Category: "core", DefaultFilterable: true, DefaultSortable: true, SupportsPrecision: true, QueryCast: "numeric"},
	{APIName: FieldTypePercent, Label: "Percent", Category: "core", DefaultFilterable: true, DefaultSortable: true, SupportsPrecision: true, QueryCast: "numeric", Notes: "Stores percent points (12.5 = 12.5%)"},
	{APIName: FieldTypeDate, Label: "Date", Category: "core", DefaultFilterable: true, DefaultSortable: true, QueryCast: "date"},
	{APIName: FieldTypeDateTime, Label: "Date/Time", Category: "core", DefaultFilterable: true, DefaultSortable: true, QueryCast: "timestamptz"},
	{APIName: FieldTypeTime, Label: "Time", Category: "core", DefaultFilterable: true, DefaultSortable: true, QueryCast: "text", Notes: "HH:MM:SS optional fractional seconds"},
	{APIName: FieldTypePicklist, Label: "Picklist", Category: "core", DefaultFilterable: true, DefaultSortable: true, SupportsPicklist: true, QueryCast: "text"},
	{APIName: FieldTypeLookup, Label: "Lookup", Category: "core", DefaultFilterable: true, DefaultSortable: true, DefaultIndexed: true, RequiresReferenceTo: true, QueryCast: "text"},
	{APIName: FieldTypeMasterDetail, Label: "Master-Detail", Category: "core", DefaultFilterable: true, DefaultSortable: true, DefaultIndexed: true, RequiresReferenceTo: true, QueryCast: "text"},
	{APIName: FieldTypeJSON, Label: "JSON", Category: "enhancement", DefaultFilterable: false, DefaultSortable: false, QueryCast: "text", Notes: "Arbitrary JSON; equality filters only in v1"},
	{APIName: FieldTypeAutonumber, Label: "Auto Number", Category: "enhancement", DefaultFilterable: true, DefaultSortable: true, DefaultIndexed: true, SupportsAutonumber: true, QueryCast: "text", Notes: "System-assigned on create; immutable"},
	{APIName: FieldTypeRichText, Label: "Rich Text", Category: "enhancement", DefaultFilterable: false, DefaultSortable: false, SupportsLength: true, QueryCast: "text", Notes: "HTML sanitized on write"},
	{APIName: FieldTypeAddress, Label: "Address", Category: "enhancement", DefaultFilterable: false, DefaultSortable: false, QueryCast: "text", CompoundComponents: []string{"street", "city", "state", "postalCode", "country"}},
	{APIName: FieldTypeGeolocation, Label: "Geolocation", Category: "enhancement", DefaultFilterable: false, DefaultSortable: false, QueryCast: "text", CompoundComponents: []string{"latitude", "longitude"}},
}

var fieldTypeByName map[string]FieldTypeInfo

func init() {
	fieldTypeByName = make(map[string]FieldTypeInfo, len(canonicalFieldTypes))
	for _, t := range canonicalFieldTypes {
		fieldTypeByName[t.APIName] = t
	}
}

// ListFieldTypes returns the canonical catalog for Metadata API discovery.
func ListFieldTypes() []FieldTypeInfo {
	out := make([]FieldTypeInfo, len(canonicalFieldTypes))
	copy(out, canonicalFieldTypes)
	return out
}

// IsCanonicalFieldType reports whether name is in the allowlist.
func IsCanonicalFieldType(name string) bool {
	_, ok := fieldTypeByName[name]
	return ok
}

// ExternalIDEligibleTypes are field types that may be marked externalId (BP-041).
var ExternalIDEligibleTypes = map[string]struct{}{
	FieldTypeText:       {},
	FieldTypeEmail:      {},
	FieldTypePhone:      {},
	FieldTypeURL:        {},
	FieldTypeAutonumber: {},
	FieldTypeInteger:    {},
}

// IsExternalIDEligible reports whether a field type may be an external ID upsert key.
func IsExternalIDEligible(fieldType string) bool {
	_, ok := ExternalIDEligibleTypes[strings.TrimSpace(fieldType)]
	return ok
}

// SearchableEligibleTypes may be marked searchable for Client find (BP-043).
var SearchableEligibleTypes = map[string]struct{}{
	FieldTypeText:       {},
	FieldTypeEmail:      {},
	FieldTypePhone:      {},
	FieldTypeURL:        {},
	FieldTypeAutonumber: {},
}

// IsSearchableEligible reports whether a field type may be marked searchable.
func IsSearchableEligible(fieldType string) bool {
	_, ok := SearchableEligibleTypes[strings.TrimSpace(fieldType)]
	return ok
}

// ApplySearchableRules validates searchable and forces filterable when set.
// It does not auto-set indexed (btree indexes stay independent of find).
func ApplySearchableRules(f *FieldDefinition) error {
	if f == nil || !f.Searchable {
		return nil
	}
	if !IsSearchableEligible(f.FieldType) {
		return fmt.Errorf("%w: searchable is not allowed on fieldType %q", ErrValidation, f.FieldType)
	}
	f.Filterable = true
	return nil
}

// ApplyExternalIDRules validates externalId and forces unique+indexed+filterable when set.
func ApplyExternalIDRules(f *FieldDefinition) error {
	if f == nil || !f.ExternalID {
		return nil
	}
	if !IsExternalIDEligible(f.FieldType) {
		return fmt.Errorf("%w: externalId is not allowed on fieldType %q", ErrValidation, f.FieldType)
	}
	f.UniqueField = true
	f.Indexed = true
	f.Filterable = true
	return nil
}

// FieldTypeInfoByName returns catalog metadata for a type.
func FieldTypeInfoByName(name string) (FieldTypeInfo, bool) {
	info, ok := fieldTypeByName[name]
	return info, ok
}

// NormalizeFieldTypeAlias maps legacy aliases to Majesta One names.
// Returns ok=false when the alias is unknown and not already canonical.
func NormalizeFieldTypeAlias(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if IsCanonicalFieldType(name) {
		return name, true
	}
	if mapped, ok := fieldTypeAliasMap[strings.ToLower(name)]; ok {
		return mapped, true
	}
	return name, false
}

// ValidateFieldTypeCreate checks fieldType + type-specific required attributes on create.
// Applies catalog defaults for filterable/sortable/indexed/length when unset flags are still at zero-value
// only when applyDefaults is true — InsertField should pass applyDefaults after reading raw JSON presence.
func ValidateFieldTypeCreate(input *FieldDefinition) error {
	if input == nil {
		return fmt.Errorf("%w: field definition required", ErrValidation)
	}
	ft := strings.TrimSpace(input.FieldType)
	if ft == "" {
		return fmt.Errorf("%w: fieldType is required", ErrValidation)
	}
	if ft == "polymorphic_lookup" {
		return fmt.Errorf("%w: fieldType %q is retired; use lookup with referenceTo", ErrValidation, ft)
	}
	if !IsCanonicalFieldType(ft) {
		if mapped, ok := NormalizeFieldTypeAlias(ft); ok && mapped != ft {
			return fmt.Errorf("%w: fieldType %q is not accepted; use %q", ErrValidation, ft, mapped)
		}
		return fmt.Errorf("%w: unknown fieldType %q; see GET /metadata/v1/field-types", ErrValidation, ft)
	}
	info := fieldTypeByName[ft]
	if info.RequiresReferenceTo {
		if input.ReferenceTo == nil || strings.TrimSpace(*input.ReferenceTo) == "" {
			return fmt.Errorf("%w: %s requires referenceTo", ErrValidation, ft)
		}
	}
	if ft == FieldTypeAutonumber {
		if input.AutonumberFormat == nil || strings.TrimSpace(*input.AutonumberFormat) == "" {
			fmtStr := "{00000}"
			input.AutonumberFormat = &fmtStr
		}
		if input.AutonumberStart == nil {
			start := 1
			input.AutonumberStart = &start
		}
		if *input.AutonumberStart < 0 {
			return fmt.Errorf("%w: autonumberStart must be >= 0", ErrValidation)
		}
	}
	// Empty picklistValues are allowed on create; values can be patched later.
	return nil
}

// ApplyFieldTypeDefaults sets filterable/sortable/indexed/length from catalog when requested.
func ApplyFieldTypeDefaults(input *FieldDefinition, filterableSet, sortableSet, indexedSet bool) {
	info, ok := fieldTypeByName[input.FieldType]
	if !ok {
		return
	}
	if !filterableSet {
		input.Filterable = info.DefaultFilterable
	}
	if !sortableSet {
		input.Sortable = info.DefaultSortable
	}
	if !indexedSet && info.DefaultIndexed {
		input.Indexed = true
	}
	if info.SupportsLength && input.Length == nil {
		switch input.FieldType {
		case FieldTypeTextarea, FieldTypeRichText:
			n := 32000
			input.Length = &n
		case FieldTypeText, FieldTypeEmail, FieldTypePhone, FieldTypeURL:
			n := 255
			input.Length = &n
		}
	}
}

// QueryCastForFieldType returns the SQL cast used by DataEngine filters.
func QueryCastForFieldType(fieldType string) string {
	if info, ok := fieldTypeByName[fieldType]; ok && info.QueryCast != "" {
		return info.QueryCast
	}
	switch fieldType {
	case FieldTypeNumber, FieldTypeCurrency, FieldTypePercent:
		return "numeric"
	case FieldTypeInteger:
		return "bigint"
	case FieldTypeBoolean:
		return "boolean"
	case FieldTypeDate:
		return "date"
	case FieldTypeDateTime:
		return "timestamptz"
	default:
		return "text"
	}
}

// AddressComponentKeys are the only keys allowed on address compound values.
var AddressComponentKeys = []string{"street", "city", "state", "postalCode", "country"}

// GeolocationComponentKeys are the only keys allowed on geolocation values.
var GeolocationComponentKeys = []string{"latitude", "longitude"}
