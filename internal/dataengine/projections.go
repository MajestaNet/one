package dataengine

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/MajestaNet/ide/internal/db"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9_]+`)

// ProjectionBuildResult is the outcome of BuildFieldProjections.
type ProjectionBuildResult struct {
	Built  []string            `json:"built"`
	Errors []map[string]string `json:"errors"`
}

type projectionCacheEntry struct {
	at   time.Time
	rows []map[string]any
}

const projectionCacheTTL = 2 * time.Second

var projectionCache sync.Map // objectAPIName -> projectionCacheEntry

// BuildFieldProjections creates expression indexes for indexed metadata fields.
func (s *Service) BuildFieldProjections(ctx context.Context, objectAPIName string) (*ProjectionBuildResult, error) {
	obj, err := s.meta.GetObject(ctx, objectAPIName)
	if err != nil {
		return nil, err
	}
	if db.IsKernelStorage(obj.StorageMode) {
		return &ProjectionBuildResult{Built: []string{}, Errors: []map[string]string{}}, nil
	}
	table, err := quoteTable(db.RecordsTableForStorageMode(obj.StorageMode))
	if err != nil {
		return nil, err
	}
	fields, err := s.meta.GetFields(ctx, objectAPIName)
	if err != nil {
		return nil, err
	}
	result := &ProjectionBuildResult{Built: []string{}, Errors: []map[string]string{}}
	if _, err := assertSafeFieldName(objectAPIName); err != nil {
		return nil, validationErrorf("Invalid object api name: %s", objectAPIName)
	}
	for _, f := range fields {
		if !f.Indexed {
			continue
		}
		if _, err := assertSafeFieldName(f.APIName); err != nil {
			result.Errors = append(result.Errors, map[string]string{"field": f.APIName, "error": err.Error()})
			continue
		}
		cast := castTypeForFieldType(f.FieldType)
		unique := f.ExternalID || f.UniqueField
		indexName := projectionIndexName(objectAPIName, f.APIName)
		if unique {
			indexName = projectionUniqueIndexName(objectAPIName, f.APIName)
		}
		_, err := s.pool.Exec(ctx, `
INSERT INTO field_projections (object_api_name, field_api_name, index_name, cast_type, status)
VALUES ($1,$2,$3,$4,'pending')
ON CONFLICT (object_api_name, field_api_name) DO NOTHING`, objectAPIName, f.APIName, indexName, cast)
		if err != nil {
			result.Errors = append(result.Errors, map[string]string{"field": f.APIName, "error": err.Error()})
			continue
		}
		var ddl string
		indexVerb := "CREATE INDEX"
		if unique {
			indexVerb = "CREATE UNIQUE INDEX"
		}
		switch cast {
		case "numeric", "bigint", "boolean", "timestamptz", "date":
			ddl = fmt.Sprintf(
				`%s CONCURRENTLY IF NOT EXISTS %s ON %s ((NULLIF(data ->> '%s','')::%s)) WHERE object_api_name = '%s' AND NULLIF(data ->> '%s','') IS NOT NULL`,
				indexVerb, indexName, table, escapeSQLLiteral(f.APIName), cast, escapeSQLLiteral(objectAPIName), escapeSQLLiteral(f.APIName),
			)
		default:
			if unique {
				ddl = fmt.Sprintf(
					`%s CONCURRENTLY IF NOT EXISTS %s ON %s ((data ->> '%s')) WHERE object_api_name = '%s' AND NULLIF(data ->> '%s','') IS NOT NULL`,
					indexVerb, indexName, table, escapeSQLLiteral(f.APIName), escapeSQLLiteral(objectAPIName), escapeSQLLiteral(f.APIName),
				)
			} else {
				ddl = fmt.Sprintf(
					`%s CONCURRENTLY IF NOT EXISTS %s ON %s ((data ->> '%s')) WHERE object_api_name = '%s'`,
					indexVerb, indexName, table, escapeSQLLiteral(f.APIName), escapeSQLLiteral(objectAPIName),
				)
			}
		}
		indexErr := error(nil)
		if _, e := s.pool.Exec(ctx, ddl); e != nil {
			// CONCURRENTLY cannot run inside a transaction; fall back for test/tx paths.
			fallback := strings.Replace(ddl, " CONCURRENTLY", "", 1)
			if fallback != ddl {
				if _, e2 := s.pool.Exec(ctx, fallback); e2 == nil {
					e = nil
				} else {
					e = e2
				}
			}
			indexErr = e
		}
		if indexErr != nil {
			_, _ = s.pool.Exec(ctx, `
UPDATE field_projections SET status='failed', last_error=$3
WHERE object_api_name=$1 AND field_api_name=$2`, objectAPIName, f.APIName, indexErr.Error())
			result.Errors = append(result.Errors, map[string]string{"field": f.APIName, "error": indexErr.Error()})
			continue
		}
		_, _ = s.pool.Exec(ctx, `
UPDATE field_projections SET status='ready', built_at=$3, last_error=NULL
WHERE object_api_name=$1 AND field_api_name=$2`, objectAPIName, f.APIName, time.Now())
		result.Built = append(result.Built, f.APIName)
		s.invalidateProjectionCache(objectAPIName)
	}
	return result, nil
}

// RebuildProjections is an alias used by HTTP handlers.
func (s *Service) RebuildProjections(ctx context.Context, objectAPIName string) (*ProjectionBuildResult, error) {
	return s.BuildFieldProjections(ctx, objectAPIName)
}

func projectionIndexName(objectAPIName, fieldAPIName string) string {
	raw := strings.ToLower("proj_" + objectAPIName + "_" + fieldAPIName)
	name := nonAlnum.ReplaceAllString(raw, "_")
	if len(name) > 60 {
		name = name[:60]
	}
	return name
}

func projectionUniqueIndexName(objectAPIName, fieldAPIName string) string {
	raw := strings.ToLower("proj_u_" + objectAPIName + "_" + fieldAPIName)
	name := nonAlnum.ReplaceAllString(raw, "_")
	if len(name) > 60 {
		name = name[:60]
	}
	return name
}

func escapeSQLLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// AssertSafeFieldName is exported for identifier-safety tests.
func AssertSafeFieldName(field string) (string, error) {
	return assertSafeFieldName(field)
}

// ListProjections returns field_projections rows for an object.
func (s *Service) ListProjections(ctx context.Context, objectAPIName string) ([]map[string]any, error) {
	if _, err := s.meta.GetObject(ctx, objectAPIName); err != nil {
		return nil, err
	}
	if v, ok := projectionCache.Load(objectAPIName); ok {
		ent := v.(projectionCacheEntry)
		if time.Since(ent.at) < projectionCacheTTL {
			return ent.rows, nil
		}
	}
	rows, err := s.pool.Query(ctx, `
SELECT id::text, object_api_name, field_api_name, index_name, cast_type, status, last_error, created_at, built_at
FROM field_projections WHERE object_api_name=$1 ORDER BY field_api_name`, objectAPIName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, obj, field, indexName, castType, status string
		var lastError *string
		var createdAt time.Time
		var builtAt *time.Time
		if err := rows.Scan(&id, &obj, &field, &indexName, &castType, &status, &lastError, &createdAt, &builtAt); err != nil {
			return nil, err
		}
		m := map[string]any{
			"id": id, "objectApiName": obj, "fieldApiName": field,
			"indexName": indexName, "castType": castType, "status": status,
			"createdAt": createdAt,
		}
		if lastError != nil {
			m["lastError"] = *lastError
		}
		if builtAt != nil {
			m["builtAt"] = *builtAt
		}
		out = append(out, m)
	}
	if out == nil {
		out = []map[string]any{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	projectionCache.Store(objectAPIName, projectionCacheEntry{at: time.Now(), rows: out})
	return out, nil
}

func (s *Service) invalidateProjectionCache(objectAPIName string) {
	projectionCache.Delete(objectAPIName)
}
