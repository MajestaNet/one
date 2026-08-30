package dataengine

import (
	"encoding/json"
	"time"
)

// SObjectRecord is the Client API record shape.
type SObjectRecord map[string]any

func toSObject(
	id string,
	ownerID *string,
	createdByID, lastModifiedByID, objectAPIName string,
	createdAt, updatedAt time.Time,
	data map[string]any,
	selectFields []string,
) SObjectRecord {
	var owner any
	if ownerID != nil && *ownerID != "" {
		owner = *ownerID
	}
	rec := SObjectRecord{
		"Id":               id,
		"attributes":       map[string]any{"type": objectAPIName, "url": "/client/v1/sobjects/" + objectAPIName + "/" + id},
		"OwnerId":          owner,
		"CreatedById":      createdByID,
		"LastModifiedById": lastModifiedByID,
		"CreatedAt":        createdAt.UTC().Format(time.RFC3339Nano),
		"UpdatedAt":        updatedAt.UTC().Format(time.RFC3339Nano),
	}
	projected := projectData(data, selectFields)
	for k, v := range projected {
		rec[k] = v
	}
	return rec
}

func projectData(data map[string]any, selectFields []string) map[string]any {
	if data == nil {
		return map[string]any{}
	}
	if len(selectFields) == 0 {
		out := make(map[string]any, len(data))
		for k, v := range data {
			out[k] = v
		}
		return out
	}
	out := map[string]any{}
	for _, key := range selectFields {
		if v, ok := data[key]; ok {
			out[key] = v
		}
	}
	return out
}

func decodeJSONBMap(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

// optionalOwnerID extracts OwnerId from input. Omitted or null/empty → nil (no owner).
func optionalOwnerID(input map[string]any) (*string, bool) {
	v, ok := input["OwnerId"]
	if !ok {
		return nil, false
	}
	if v == nil {
		return nil, true
	}
	s, ok := v.(string)
	if !ok {
		return nil, true
	}
	if s == "" {
		return nil, true
	}
	return &s, true
}
