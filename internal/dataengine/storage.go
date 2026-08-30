package dataengine

import (
	"context"
	"fmt"
	"strings"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
)

// recordsTableForObject returns the physical store for an object api name.
func (s *Service) recordsTableForObject(ctx context.Context, objectAPIName string) (string, error) {
	obj, err := s.meta.GetObject(ctx, objectAPIName)
	if err != nil {
		return "", err
	}
	if err := rejectKernelStorage(objectAPIName, obj.StorageMode); err != nil {
		return "", err
	}
	return db.RecordsTableForStorageMode(obj.StorageMode), nil
}

// assertHighVolumeQueryGuardrails rejects unbounded high-volume list scans.
// Allowed: Id equality, CreatedAt/UpdatedAt bound, or any filter on an indexed field.
// Locator mode additionally requires a CreatedAt/UpdatedAt bound (ADR-013 time-bounded locators).
func assertHighVolumeQueryGuardrails(storageMode string, req *QueryRequest, fields []metadata.FieldDefinition) error {
	if storageMode != db.StorageModeHighVolume {
		return nil
	}
	filters := req.Filters
	if req.Mode == "locator" && !hasTimeBoundFilter(filters) {
		return validationErrorf("high_volume locator queries require a CreatedAt or UpdatedAt bound (max effective rows %d)", HighVolumeLocatorMaxRows)
	}
	indexed := map[string]bool{}
	for _, f := range fields {
		if f.Indexed {
			indexed[f.APIName] = true
		}
	}
	for _, f := range filters {
		if strings.Contains(f.Field, ".") {
			continue
		}
		switch f.Field {
		case "Id":
			if f.Op == OpEq || f.Op == OpIn {
				return nil
			}
		case "CreatedAt", "UpdatedAt":
			switch f.Op {
			case OpEq, OpGt, OpGte, OpLt, OpLte, OpIn:
				return nil
			}
		default:
			if indexed[f.Field] {
				switch f.Op {
				case OpEq, OpIn, OpGt, OpGte, OpLt, OpLte:
					return nil
				}
			}
		}
	}
	return validationErrorf("high_volume object queries require a selective filter (Id, CreatedAt/UpdatedAt bound, or indexed field)")
}

// assertFlexibleQueryGuardrails enforces Tier A access-pattern discipline on the
// shared records store: unindexed LIKE is rejected (forces expression indexes).
func assertFlexibleQueryGuardrails(storageMode string, req *QueryRequest, fields []metadata.FieldDefinition) error {
	if storageMode == db.StorageModeHighVolume {
		return nil
	}
	indexed := map[string]bool{}
	for _, f := range fields {
		if f.Indexed {
			indexed[f.APIName] = true
		}
	}
	for _, f := range req.Filters {
		if f.Op != OpLike || strings.Contains(f.Field, ".") {
			continue
		}
		if isSystemQueryField(f.Field) {
			continue
		}
		if !indexed[f.Field] {
			return validationErrorf("LIKE filters on flexible objects require indexed=true on field %s (expression index)", f.Field)
		}
	}
	return nil
}

func hasTimeBoundFilter(filters []QueryFilter) bool {
	for _, f := range filters {
		if f.Field != "CreatedAt" && f.Field != "UpdatedAt" {
			continue
		}
		switch f.Op {
		case OpEq, OpGt, OpGte, OpLt, OpLte, OpIn:
			return true
		}
	}
	return false
}

// quoteTable returns a safe physical table identifier (fixed allowlist).
func quoteTable(table string) (string, error) {
	switch table {
	case db.RecordsTableFlexible, db.RecordsTableHighVolume:
		return table, nil
	default:
		return "", fmt.Errorf("unknown records table: %s", table)
	}
}
