package dataengine

import (
	"context"
	"strings"

	"github.com/MajestaNet/ide/internal/metadata"
)

// IndexHint is an advisory signal that a filter/sort field lacks a ready projection.
type IndexHint struct {
	FieldAPIName string `json:"fieldApiName"`
	Reason       string `json:"reason"`
}

// buildIndexHints returns warnings for filterable predicates lacking ready field_projections.
// System columns (Id, OwnerId, CreatedAt, …) are excluded — they use kernel indexes.
func (s *Service) buildIndexHints(ctx context.Context, objectAPIName string, req *QueryRequest, fields []metadata.FieldDefinition) ([]IndexHint, error) {
	ready := map[string]bool{}
	rows, err := s.ListProjections(ctx, objectAPIName)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		status, _ := row["status"].(string)
		field, _ := row["fieldApiName"].(string)
		if status == "ready" && field != "" {
			ready[field] = true
		}
	}
	fieldByName := map[string]metadata.FieldDefinition{}
	for _, f := range fields {
		fieldByName[f.APIName] = f
	}

	seen := map[string]bool{}
	var hints []IndexHint
	add := func(field, reason string) {
		if field == "" || seen[field] || isSystemQueryField(field) {
			return
		}
		seen[field] = true
		hints = append(hints, IndexHint{FieldAPIName: field, Reason: reason})
	}

	for _, f := range req.Filters {
		if strings.Contains(f.Field, ".") {
			continue
		}
		def, ok := fieldByName[f.Field]
		if !ok {
			continue
		}
		if !def.Filterable && !def.Indexed {
			continue
		}
		if ready[f.Field] {
			continue
		}
		if def.Indexed {
			add(f.Field, "indexed field lacks ready field_projection; enqueue projection.build")
		} else if def.Filterable {
			add(f.Field, "filterable field has no expression index; set indexed=true to build a projection")
		}
	}
	for _, srt := range req.Sort {
		if strings.Contains(srt.Field, ".") || isSystemQueryField(srt.Field) {
			continue
		}
		def, ok := fieldByName[srt.Field]
		if !ok {
			continue
		}
		if ready[srt.Field] {
			continue
		}
		if def.Indexed || def.Sortable {
			add(srt.Field, "sort field lacks ready field_projection")
		}
	}
	if hints == nil {
		hints = []IndexHint{}
	}
	return hints, nil
}

func isSystemQueryField(field string) bool {
	switch field {
	case "Id", "OwnerId", "CreatedById", "LastModifiedById", "CreatedAt", "UpdatedAt", "IsDeleted":
		return true
	default:
		return false
	}
}
