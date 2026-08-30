package dataengine

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/metadata"
)

// FilterOp is a query filter operator.
type FilterOp string

const (
	OpEq        FilterOp = "eq"
	OpNe        FilterOp = "ne"
	OpGt        FilterOp = "gt"
	OpGte       FilterOp = "gte"
	OpLt        FilterOp = "lt"
	OpLte       FilterOp = "lte"
	OpLike      FilterOp = "like"
	OpIn        FilterOp = "in"
	OpIsNull    FilterOp = "is_null"
	OpIsNotNull FilterOp = "is_not_null"
)

// QueryFilter is one predicate.
type QueryFilter struct {
	Field string   `json:"field"`
	Op    FilterOp `json:"op"`
	Value any      `json:"value,omitempty"`
}

// SortSpec is ORDER BY.
type SortSpec struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

// RelationshipQuery is a parent or child join request.
type RelationshipQuery struct {
	Type    string        `json:"type"` // parent | child
	Field   string        `json:"field"`
	Object  string        `json:"object,omitempty"`
	Alias   string        `json:"alias,omitempty"`
	Filters []QueryFilter `json:"filters"`
	Select  []string      `json:"select,omitempty"`
	Limit   int           `json:"limit,omitempty"`
}

// QueryRequest is POST /client/v1/query body.
type QueryRequest struct {
	Object         string              `json:"object"`
	Select         []string            `json:"select,omitempty"`
	Filters        []QueryFilter       `json:"filters"`
	Sort           []SortSpec          `json:"sort"`
	Relationships  []RelationshipQuery `json:"relationships"`
	Limit          int                 `json:"limit"`
	Cursor         string              `json:"cursor,omitempty"`
	IncludeDeleted bool                `json:"includeDeleted"`
	Mode           string              `json:"mode"`
}

// ParseQueryRequest validates and defaults a query body.
func ParseQueryRequest(raw json.RawMessage) (*QueryRequest, error) {
	var req QueryRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, validationErrorf("Invalid query body: %v", err)
	}
	if strings.TrimSpace(req.Object) == "" {
		return nil, validationErrorf("object is required")
	}
	if req.Mode == "" {
		req.Mode = "standard"
	}
	if req.Mode != "standard" && req.Mode != "locator" {
		return nil, validationErrorf("mode must be standard or locator")
	}
	if req.Limit <= 0 {
		req.Limit = Limits.DefaultPageSize
	}
	if req.Limit > Limits.StandardMaxRows {
		return nil, validationErrorf("limit exceeds max %d", Limits.StandardMaxRows)
	}
	if req.Mode == "locator" && req.Limit > Limits.LocatorPageSize {
		return nil, validationErrorf("locator mode page size max is %d", Limits.LocatorPageSize)
	}
	if len(req.Filters) > Limits.MaxFilterConditions {
		return nil, validationErrorf("too many filters (max %d)", Limits.MaxFilterConditions)
	}
	if len(req.Sort) > Limits.MaxSortFields {
		return nil, validationErrorf("too many sort fields (max %d)", Limits.MaxSortFields)
	}
	if len(req.Select) > Limits.MaxSelectFields {
		return nil, validationErrorf("too many select fields (max %d)", Limits.MaxSelectFields)
	}
	if len(req.Relationships) > Limits.MaxJoins {
		return nil, validationErrorf("too many relationships (max %d)", Limits.MaxJoins)
	}
	childCount := 0
	totalChildRowsPerParent := 0
	for i := range req.Relationships {
		rel := &req.Relationships[i]
		if rel.Type != "parent" && rel.Type != "child" {
			return nil, validationErrorf("relationships[%d].type must be parent or child", i)
		}
		if strings.TrimSpace(rel.Field) == "" {
			return nil, validationErrorf("relationships[%d].field is required", i)
		}
		if _, err := assertSafeFieldName(rel.Field); err != nil {
			return nil, err
		}
		if rel.Alias != "" {
			if err := assertSafeAlias(rel.Alias); err != nil {
				return nil, err
			}
		}
		if rel.Type == "child" {
			childCount++
			if strings.TrimSpace(rel.Object) == "" {
				return nil, validationErrorf("Child relationship on %s requires object", rel.Field)
			}
		}
		if len(rel.Filters) > Limits.MaxFilterConditions {
			return nil, validationErrorf("relationships[%d] has too many filters (max %d)", i, Limits.MaxFilterConditions)
		}
		if len(rel.Select) > Limits.MaxSelectFields {
			return nil, validationErrorf("relationships[%d] has too many select fields (max %d)", i, Limits.MaxSelectFields)
		}
		if rel.Limit <= 0 {
			rel.Limit = 200
		}
		if rel.Type == "child" {
			if rel.Limit > Limits.MaxChildRowsPerParent {
				return nil, validationErrorf("relationships[%d].limit exceeds max %d", i, Limits.MaxChildRowsPerParent)
			}
			totalChildRowsPerParent += rel.Limit
		}
		for j, f := range rel.Filters {
			if err := validateFilterOp(f, i, j); err != nil {
				return nil, err
			}
		}
	}
	if childCount > Limits.MaxChildRelationships {
		return nil, validationErrorf("too many child relationships (max %d)", Limits.MaxChildRelationships)
	}
	if req.Limit > 0 && totalChildRowsPerParent > 0 && req.Limit > Limits.MaxChildRowsPerQuery/totalChildRowsPerParent {
		return nil, validationErrorf("child relationship row budget exceeds max %d", Limits.MaxChildRowsPerQuery)
	}
	if len(req.Sort) > 0 && req.Cursor != "" {
		return nil, validationErrorf("cursor pagination is only supported with default CreatedAt sort; omit sort or omit cursor")
	}
	if req.Cursor != "" {
		createdAt, id, ok := decodeKeysetCursor(req.Cursor)
		if !ok {
			return nil, validationErrorf("cursor is invalid")
		}
		if _, err := time.Parse(time.RFC3339Nano, createdAt); err != nil || !looksLikeUUID(id) {
			return nil, validationErrorf("cursor is invalid")
		}
	}
	for i, f := range req.Filters {
		if strings.TrimSpace(f.Field) == "" {
			return nil, validationErrorf("filters[%d].field is required", i)
		}
		if err := validateFilterOp(f, -1, i); err != nil {
			return nil, err
		}
	}
	for i, s := range req.Sort {
		if strings.TrimSpace(s.Field) == "" {
			return nil, validationErrorf("sort[%d].field is required", i)
		}
		if s.Direction == "" {
			req.Sort[i].Direction = "asc"
		}
		d := strings.ToLower(req.Sort[i].Direction)
		if d != "asc" && d != "desc" {
			return nil, validationErrorf("sort direction must be asc or desc")
		}
		req.Sort[i].Direction = d
	}
	return &req, nil
}

func validateFilterOp(f QueryFilter, relIdx, filterIdx int) error {
	switch f.Op {
	case OpEq, OpNe, OpGt, OpGte, OpLt, OpLte, OpLike, OpIn, OpIsNull, OpIsNotNull:
	default:
		return validationErrorf("Unsupported operator: %s", f.Op)
	}
	if f.Op == OpIn {
		arr, ok := f.Value.([]any)
		if !ok {
			return validationErrorf("`in` filter requires an array value")
		}
		if len(arr) == 0 {
			return validationErrorf("`in` filter requires at least one value")
		}
		if len(arr) > Limits.MaxInListSize {
			return validationErrorf("in list exceeds max %d", Limits.MaxInListSize)
		}
	}
	_ = relIdx
	_ = filterIdx
	return nil
}

func assertSafeAlias(alias string) error {
	if len(alias) == 0 {
		return validationErrorf("Invalid relationship alias: %s", alias)
	}
	for i, c := range alias {
		ok := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (i > 0 && ((c >= '0' && c <= '9') || c == '_'))
		if !ok {
			return validationErrorf("Invalid relationship alias: %s", alias)
		}
	}
	return nil
}

func assertSafeFieldName(field string) (string, error) {
	switch field {
	case "Id", "OwnerId", "CreatedById", "LastModifiedById", "CreatedAt", "UpdatedAt":
		return field, nil
	}
	if len(field) == 0 {
		return "", validationErrorf("Invalid field name: %s", field)
	}
	for i, c := range field {
		ok := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (i > 0 && ((c >= '0' && c <= '9') || c == '_'))
		if !ok {
			return "", validationErrorf("Invalid field name: %s", field)
		}
	}
	return field, nil
}

func encodeKeysetCursor(createdAtISO, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(createdAtISO + "|" + id))
}

func decodeKeysetCursor(cursor string) (createdAt, id string, ok bool) {
	if cursor == "" {
		return "", "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(cursor)
		if err != nil {
			return "", "", false
		}
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func castTypeForFieldType(fieldType string) string {
	return metadata.QueryCastForFieldType(fieldType)
}
