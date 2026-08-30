package packages

// CatalogField is a lookup / master-detail field from the image registry.
// Used by GET /metadata/v1/packages so Explorer can visualize objects that
// are not enabled on this install yet.
type CatalogField struct {
	APIName     string  `json:"apiName"`
	Label       string  `json:"label,omitempty"`
	FieldType   string  `json:"fieldType"`
	ReferenceTo *string `json:"referenceTo,omitempty"`
}

// CatalogObject is a registry object (or field-extension host) with enough
// shape for an object-relationship graph without enabling the package.
type CatalogObject struct {
	APIName     string         `json:"apiName"`
	Label       string         `json:"label"`
	PluralLabel string         `json:"pluralLabel,omitempty"`
	FieldCount  int            `json:"fieldCount"`
	Fields      []CatalogField `json:"fields,omitempty"`
}

// CatalogObjects returns owned objects plus field-extension hosts for a module.
// Lookup fields are included so graphs can draw edges; FieldCount is the full
// owned (or extension) field count, not only lookups.
func CatalogObjects(m Module) []CatalogObject {
	byName := make(map[string]*CatalogObject, len(m.Objects)+len(m.FieldExtensions))
	order := make([]string, 0, len(m.Objects)+len(m.FieldExtensions))

	add := func(apiName, label, plural string, extraFields int) *CatalogObject {
		if existing, ok := byName[apiName]; ok {
			existing.FieldCount += extraFields
			if existing.Label == "" && label != "" {
				existing.Label = label
			}
			if existing.PluralLabel == "" && plural != "" {
				existing.PluralLabel = plural
			}
			return existing
		}
		if label == "" {
			label = apiName
		}
		co := &CatalogObject{
			APIName:     apiName,
			Label:       label,
			PluralLabel: plural,
			FieldCount:  extraFields,
		}
		byName[apiName] = co
		order = append(order, apiName)
		return co
	}

	appendLookups := func(co *CatalogObject, fields []FieldDef) {
		for _, f := range fields {
			if f.ReferenceTo == nil {
				continue
			}
			ref := *f.ReferenceTo
			if ref == "" {
				continue
			}
			co.Fields = append(co.Fields, CatalogField{
				APIName:     f.APIName,
				Label:       f.Label,
				FieldType:   f.FieldType,
				ReferenceTo: &ref,
			})
		}
	}

	for _, o := range m.Objects {
		co := add(o.APIName, o.Label, o.PluralLabel, len(o.Fields))
		appendLookups(co, o.Fields)
	}
	for _, ext := range m.FieldExtensions {
		co := add(ext.ObjectAPIName, ext.ObjectAPIName, "", len(ext.Fields))
		appendLookups(co, ext.Fields)
	}

	out := make([]CatalogObject, 0, len(order))
	for _, api := range order {
		co := byName[api]
		if len(co.Fields) == 0 {
			co.Fields = nil
		}
		out = append(out, *co)
	}
	return out
}
