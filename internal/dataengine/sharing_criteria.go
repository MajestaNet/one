package dataengine

import (
	"fmt"
	"strings"

	"github.com/MajestaNet/ide/internal/metadata"
)

// BuildCriteriaSQL builds a SELECT id query for sharing rule recalc (criteria match only).
func BuildCriteriaSQL(req *QueryRequest, fields []metadata.FieldDefinition, primaryTable string) (*builtQuery, error) {
	table, err := quoteTable(primaryTable)
	if err != nil {
		return nil, err
	}
	fm := fieldMap{}
	for _, f := range fields {
		fm[f.APIName] = f
	}
	args := []any{}
	where := []string{}
	args = append(args, req.Object)
	where = append(where, fmt.Sprintf("r.object_api_name = $%d", len(args)))
	for _, f := range req.Filters {
		if err := pushFilter("r", f, fm, &args, &where); err != nil {
			return nil, err
		}
	}
	limitSQL := ""
	if req.Limit > 0 {
		args = append(args, req.Limit)
		limitSQL = fmt.Sprintf("\nLIMIT $%d", len(args))
	}
	text := strings.TrimSpace(fmt.Sprintf(`
SELECT r.id::text
FROM %s r
WHERE %s%s
`, table, strings.Join(where, " AND "), limitSQL))
	return &builtQuery{Text: text, Args: args}, nil
}
