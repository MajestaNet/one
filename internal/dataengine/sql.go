package dataengine

import (
	"fmt"
	"strings"

	"github.com/MajestaNet/ide/internal/metadata"
)

type fieldMap map[string]metadata.FieldDefinition

type joinPlan struct {
	Alias         string
	Type          string // parent | child
	Field         string
	RelatedObject string
	RelatedTable  string // records | records_hv
	Filters       []QueryFilter
	Select        []string
	Limit         int
}

type builtQuery struct {
	Text      string
	Args      []any
	Joins     []joinPlan
	PageLimit int
}

func jsonTextExpr(alias, field string) string {
	return fmt.Sprintf("(%s.data ->> '%s')", alias, field)
}

func typedExpr(alias, field, fieldType string) string {
	cast := castTypeForFieldType(fieldType)
	base := jsonTextExpr(alias, field)
	switch cast {
	case "numeric", "bigint", "boolean", "timestamptz", "date":
		return fmt.Sprintf("NULLIF(%s, '')::%s", base, cast)
	default:
		return base
	}
}

func systemColumn(alias, field string) string {
	switch field {
	case "Id":
		return alias + ".id"
	case "OwnerId":
		return alias + ".owner_id"
	case "CreatedById":
		return alias + ".created_by_id"
	case "LastModifiedById":
		return alias + ".last_modified_by_id"
	case "CreatedAt":
		return alias + ".created_at"
	case "UpdatedAt":
		return alias + ".updated_at"
	default:
		return ""
	}
}

func quoteIdent(alias string) string {
	return `"` + strings.ReplaceAll(alias, `"`, ``) + `"`
}

func pushFilter(alias string, filter QueryFilter, fields fieldMap, args *[]any, where *[]string) error {
	field, err := assertSafeFieldName(filter.Field)
	if err != nil {
		return err
	}
	sys := systemColumn(alias, field)
	def, hasDef := fields[field]
	if sys == "" && len(fields) > 0 && !hasDef {
		return validationErrorf("Unknown filter field: %s", field)
	}
	if hasDef && !def.Filterable {
		return validationErrorf("Field is not filterable: %s", field)
	}
	left := sys
	if left == "" {
		ft := "text"
		if hasDef {
			ft = def.FieldType
		}
		left = typedExpr(alias, field, ft)
	}

	switch filter.Op {
	case OpIsNull:
		*where = append(*where, left+" IS NULL")
		return nil
	case OpIsNotNull:
		*where = append(*where, left+" IS NOT NULL")
		return nil
	case OpIn:
		values, ok := filter.Value.([]any)
		if !ok {
			return validationErrorf("`in` filter requires an array value")
		}
		placeholders := make([]string, len(values))
		for i, v := range values {
			*args = append(*args, v)
			placeholders[i] = fmt.Sprintf("$%d", len(*args))
		}
		*where = append(*where, fmt.Sprintf("%s IN (%s)", left, strings.Join(placeholders, ", ")))
		return nil
	case OpLike:
		*args = append(*args, "%"+fmt.Sprint(filter.Value)+"%")
		*where = append(*where, fmt.Sprintf("%s ILIKE $%d", left, len(*args)))
		return nil
	case OpEq, OpNe, OpGt, OpGte, OpLt, OpLte:
		opMap := map[FilterOp]string{
			OpEq: "=", OpNe: "<>", OpGt: ">", OpGte: ">=", OpLt: "<", OpLte: "<=",
		}
		*args = append(*args, filter.Value)
		*where = append(*where, fmt.Sprintf("%s %s $%d", left, opMap[filter.Op], len(*args)))
		return nil
	default:
		return validationErrorf("Unsupported operator: %s", filter.Op)
	}
}

func resolveParentJoins(relationships []RelationshipQuery, primaryFields fieldMap) ([]joinPlan, error) {
	var joins []joinPlan
	i := 0
	for _, rel := range relationships {
		if rel.Type != "parent" {
			continue
		}
		field, err := assertSafeFieldName(rel.Field)
		if err != nil {
			return nil, err
		}
		def := primaryFields[field]
		related := rel.Object
		if related == "" && def.ReferenceTo != nil {
			related = *def.ReferenceTo
		}
		if related == "" {
			return nil, validationErrorf("Parent relationship %s requires object or lookup metadata referenceTo", field)
		}
		alias := rel.Alias
		if alias == "" {
			alias = fmt.Sprintf("p%d", i)
		}
		joins = append(joins, joinPlan{
			Alias: alias, Type: "parent", Field: field, RelatedObject: related,
			Filters: rel.Filters, Select: rel.Select,
		})
		i++
	}
	return joins, nil
}

func resolveChildJoins(relationships []RelationshipQuery) ([]joinPlan, error) {
	var joins []joinPlan
	i := 0
	for _, rel := range relationships {
		if rel.Type != "child" {
			continue
		}
		if rel.Object == "" {
			return nil, validationErrorf("Child relationship on %s requires object", rel.Field)
		}
		field, err := assertSafeFieldName(rel.Field)
		if err != nil {
			return nil, err
		}
		alias := rel.Alias
		if alias == "" {
			alias = fmt.Sprintf("c%d", i)
		}
		limit := rel.Limit
		if limit <= 0 {
			limit = 200
		}
		joins = append(joins, joinPlan{
			Alias: alias, Type: "child", Field: field, RelatedObject: rel.Object,
			Filters: rel.Filters, Select: rel.Select, Limit: limit,
		})
		i++
	}
	return joins, nil
}

func buildPrimarySelectSQL(req *QueryRequest, fields []metadata.FieldDefinition, primaryTable string, relatedTableFn func(objectAPIName string) (string, error), vis QueryVisibility) (*builtQuery, error) {
	table, err := quoteTable(primaryTable)
	if err != nil {
		return nil, err
	}
	fm := fieldMap{}
	for _, f := range fields {
		fm[f.APIName] = f
	}
	parentJoins, err := resolveParentJoins(req.Relationships, fm)
	if err != nil {
		return nil, err
	}
	childJoins, err := resolveChildJoins(req.Relationships)
	if err != nil {
		return nil, err
	}
	for i := range parentJoins {
		rt, err := relatedTableFn(parentJoins[i].RelatedObject)
		if err != nil {
			return nil, err
		}
		qt, err := quoteTable(rt)
		if err != nil {
			return nil, err
		}
		parentJoins[i].RelatedTable = qt
	}
	for i := range childJoins {
		rt, err := relatedTableFn(childJoins[i].RelatedObject)
		if err != nil {
			return nil, err
		}
		qt, err := quoteTable(rt)
		if err != nil {
			return nil, err
		}
		childJoins[i].RelatedTable = qt
	}

	args := []any{}
	where := []string{}

	args = append(args, req.Object)
	where = append(where, fmt.Sprintf("r.object_api_name = $%d", len(args)))
	AppendSharingVisibility("r", &where, &args, vis)

	for _, f := range req.Filters {
		if strings.Contains(f.Field, ".") {
			parts := strings.SplitN(f.Field, ".", 2)
			if len(parts) != 2 {
				return nil, validationErrorf("Invalid dotted filter field: %s", f.Field)
			}
			alias, field := parts[0], parts[1]
			var join *joinPlan
			for i := range parentJoins {
				if parentJoins[i].Alias == alias {
					join = &parentJoins[i]
					break
				}
			}
			if join == nil {
				return nil, validationErrorf("Unknown relationship alias in filter: %s", alias)
			}
			if err := pushFilter(quoteIdent(join.Alias), QueryFilter{Field: field, Op: f.Op, Value: f.Value}, fieldMap{}, &args, &where); err != nil {
				return nil, err
			}
			continue
		}
		if err := pushFilter("r", f, fm, &args, &where); err != nil {
			return nil, err
		}
	}

	joinSQLParts := make([]string, 0, len(parentJoins))
	for _, j := range parentJoins {
		args = append(args, j.RelatedObject)
		objParam := fmt.Sprintf("$%d", len(args))
		a := quoteIdent(j.Alias)
		joinSQLParts = append(joinSQLParts, fmt.Sprintf(
			`LEFT JOIN %s %s ON %s.id = NULLIF(%s, '')::uuid AND %s.object_api_name = %s`,
			j.RelatedTable, a, a, jsonTextExpr("r", j.Field), a, objParam,
		))
	}
	for _, j := range parentJoins {
		for _, f := range j.Filters {
			if err := pushFilter(quoteIdent(j.Alias), f, fieldMap{}, &args, &where); err != nil {
				return nil, err
			}
		}
	}

	if createdAt, id, ok := decodeKeysetCursor(req.Cursor); ok {
		args = append(args, createdAt, id)
		where = append(where, fmt.Sprintf("(r.created_at, r.id) < ($%d::timestamptz, $%d::uuid)", len(args)-1, len(args)))
	}

	orderParts := []string{}
	if len(req.Sort) > 0 {
		for _, s := range req.Sort {
			field, err := assertSafeFieldName(s.Field)
			if err != nil {
				return nil, err
			}
			if def, ok := fm[field]; ok && !def.Sortable {
				return nil, validationErrorf("Field is not sortable: %s", field)
			}
			sys := systemColumn("r", field)
			expr := sys
			if expr == "" {
				ft := "text"
				if def, ok := fm[field]; ok {
					ft = def.FieldType
				}
				expr = typedExpr("r", field, ft)
			}
			orderParts = append(orderParts, fmt.Sprintf("%s %s NULLS LAST", expr, strings.ToUpper(s.Direction)))
		}
		orderParts = append(orderParts, "r.id DESC")
	} else {
		orderParts = append(orderParts, "r.created_at DESC", "r.id DESC")
	}

	pageLimit := req.Limit
	args = append(args, pageLimit)

	dataExpr, err := projectJSONBExpr("r", req.Select)
	if err != nil {
		return nil, err
	}
	selectParents := ""
	if len(parentJoins) > 0 {
		parts := make([]string, 0, len(parentJoins)*2)
		for _, j := range parentJoins {
			a := quoteIdent(j.Alias)
			parentData, err := projectJSONBExpr(quoteIdent(j.Alias), j.Select)
			if err != nil {
				return nil, err
			}
			parts = append(parts,
				fmt.Sprintf(`%s AS %s`, parentData, quoteIdent(j.Alias+"_data")),
				fmt.Sprintf(`%s.id::text AS %s`, a, quoteIdent(j.Alias+"_id")),
			)
		}
		selectParents = ", " + strings.Join(parts, ", ")
	}

	joinSQL := ""
	if len(joinSQLParts) > 0 {
		joinSQL = "\n" + strings.Join(joinSQLParts, "\n")
	}

	text := strings.TrimSpace(fmt.Sprintf(`
SELECT r.id::text, r.owner_id::text, r.created_by_id::text, r.last_modified_by_id::text,
       r.created_at, r.updated_at, r.object_api_name, %s%s
FROM %s r%s
WHERE %s
ORDER BY %s
LIMIT $%d
`, dataExpr, selectParents, table, joinSQL, strings.Join(where, " AND "), strings.Join(orderParts, ", "), len(args)))

	allJoins := append(append([]joinPlan{}, parentJoins...), childJoins...)
	return &builtQuery{Text: text, Args: args, Joins: allJoins, PageLimit: pageLimit}, nil
}

// projectJSONBExpr returns SQL that selects full data or a pruned jsonb_build_object
// when select fields are requested (reduces wide JSONB I/O on list pages).
func projectJSONBExpr(alias string, fields []string) (string, error) {
	if len(fields) == 0 {
		return alias + ".data", nil
	}
	parts := make([]string, 0, len(fields))
	seen := map[string]bool{}
	for _, f := range fields {
		safe, err := assertSafeFieldName(f)
		if err != nil {
			return "", err
		}
		if isSystemQueryField(safe) || seen[safe] {
			continue
		}
		seen[safe] = true
		parts = append(parts, fmt.Sprintf("'%s', %s.data->'%s'", escapeSQLLiteral(safe), alias, escapeSQLLiteral(safe)))
	}
	if len(parts) == 0 {
		return alias + ".data", nil
	}
	return "jsonb_build_object(" + strings.Join(parts, ", ") + ")", nil
}

func buildChildSelectSQL(join joinPlan, parentIDs []string, vis QueryVisibility) (text string, args []any, err error) {
	if len(parentIDs) == 0 {
		return "", nil, nil
	}
	table := join.RelatedTable
	if table == "" {
		table = "records"
	}
	if _, err := quoteTable(table); err != nil {
		return "", nil, err
	}
	args = []any{join.RelatedObject, join.Field, parentIDs}
	where := []string{
		`c.object_api_name = $1`,
		`(c.data ->> $2) = ANY($3::text[])`,
	}
	AppendSharingVisibility("c", &where, &args, vis)
	for _, f := range join.Filters {
		if err := pushFilter("c", f, fieldMap{}, &args, &where); err != nil {
			return "", nil, err
		}
	}
	perParent := join.Limit
	if perParent <= 0 {
		perParent = 200
	}
	args = append(args, perParent)
	dataExpr, err := projectJSONBExpr("c", join.Select)
	if err != nil {
		return "", nil, err
	}
	// Per-parent limit via window so one hot parent cannot consume the whole page budget.
	text = strings.TrimSpace(fmt.Sprintf(`
SELECT id, owner_id, created_by_id, last_modified_by_id, created_at, updated_at, object_api_name, data, parent_id
FROM (
  SELECT c.id::text AS id, c.owner_id::text AS owner_id, c.created_by_id::text AS created_by_id,
         c.last_modified_by_id::text AS last_modified_by_id, c.created_at, c.updated_at,
         c.object_api_name, %s AS data, (c.data ->> $2) AS parent_id,
         ROW_NUMBER() OVER (
           PARTITION BY (c.data ->> $2)
           ORDER BY c.created_at DESC, c.id DESC
         ) AS rn
  FROM %s c
  WHERE %s
) ranked
WHERE rn <= $%d
ORDER BY created_at DESC, id DESC
`, dataExpr, table, strings.Join(where, " AND "), len(args)))
	return text, args, nil
}
