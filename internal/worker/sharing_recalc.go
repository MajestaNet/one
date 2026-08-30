package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
)

func processSharingRecalc(ctx context.Context, pool *db.Pool, meta *metadata.Service, payload map[string]any) error {
	sharing := db.NewSharingStore(pool)
	roles := db.NewDataRoleStore(pool)
	org, err := sharing.GetOrganizationSettings(ctx)
	if err != nil {
		return err
	}
	if !org.RecordSharingEnabled {
		return nil
	}
	scope, _ := payload["scope"].(string)
	switch scope {
	case "full", "hierarchy":
		// hierarchy changes require full rule grant rebuild (membership/descendant set changed).
		rules, err := listAllActiveRules(ctx, pool)
		if err != nil {
			return err
		}
		for _, rule := range rules {
			if err := recalcRule(ctx, pool, meta, roles, rule); err != nil {
				return err
			}
		}
	case "rule":
		ruleID, _ := payload["ruleId"].(string)
		objectAPIName, _ := payload["objectApiName"].(string)
		if ruleID != "" && objectAPIName != "" {
			rule, err := sharing.GetSharingRule(ctx, objectAPIName, ruleAPINameFromPayload(ctx, pool, ruleID))
			if err != nil {
				// load by id fallback
				rule, err = getRuleByID(ctx, pool, ruleID)
			}
			if err != nil {
				return err
			}
			return recalcRule(ctx, pool, meta, roles, *rule)
		}
	case "object":
		objectAPIName, _ := payload["objectApiName"].(string)
		rules, err := sharing.ListActiveSharingRulesForObject(ctx, objectAPIName)
		if err != nil {
			return err
		}
		for _, rule := range rules {
			if err := recalcRule(ctx, pool, meta, roles, rule); err != nil {
				return err
			}
		}
	case "record":
		objectAPIName, _ := payload["objectApiName"].(string)
		recordID, _ := payload["recordId"].(string)
		rules, err := sharing.ListActiveSharingRulesForObject(ctx, objectAPIName)
		if err != nil {
			return err
		}
		for _, rule := range rules {
			if err := recalcRuleForRecord(ctx, pool, meta, roles, rule, recordID); err != nil {
				return err
			}
		}
	}
	return nil
}

func listAllActiveRules(ctx context.Context, pool *db.Pool) ([]db.SharingRule, error) {
	rows, err := pool.Query(ctx, `
SELECT id::text, object_api_name, api_name, label, active, access_level,
       shared_to_data_role_id::text, criteria, sort_order, created_at, updated_at
FROM sharing_rules WHERE active = true ORDER BY object_api_name, sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []db.SharingRule
	for rows.Next() {
		var r db.SharingRule
		if err := rows.Scan(&r.ID, &r.ObjectAPIName, &r.APIName, &r.Label, &r.Active, &r.AccessLevel,
			&r.SharedToDataRoleID, &r.Criteria, &r.SortOrder, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func getRuleByID(ctx context.Context, pool *db.Pool, id string) (*db.SharingRule, error) {
	var r db.SharingRule
	err := pool.QueryRow(ctx, `
SELECT id::text, object_api_name, api_name, label, active, access_level,
       shared_to_data_role_id::text, criteria, sort_order, created_at, updated_at
FROM sharing_rules WHERE id = $1::uuid`, id,
	).Scan(&r.ID, &r.ObjectAPIName, &r.APIName, &r.Label, &r.Active, &r.AccessLevel,
		&r.SharedToDataRoleID, &r.Criteria, &r.SortOrder, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func ruleAPINameFromPayload(ctx context.Context, pool *db.Pool, ruleID string) string {
	r, err := getRuleByID(ctx, pool, ruleID)
	if err != nil {
		return ""
	}
	return r.APIName
}

func recalcRule(ctx context.Context, pool *db.Pool, meta *metadata.Service, roles *db.DataRoleStore, rule db.SharingRule) error {
	var builtText string
	var builtArgs []any
	var userIDs []string
	if rule.Active {
		filters, err := parseRuleFilters(rule.Criteria)
		if err != nil {
			return err
		}
		obj, err := meta.GetObject(ctx, rule.ObjectAPIName)
		if err != nil {
			return err
		}
		fields, err := meta.GetFields(ctx, rule.ObjectAPIName)
		if err != nil {
			return err
		}
		built, err := dataengine.BuildCriteriaSQL(&dataengine.QueryRequest{
			Object: rule.ObjectAPIName, Filters: filters,
		}, fields, db.RecordsTableForStorageMode(obj.StorageMode))
		if err != nil {
			return err
		}
		builtText, builtArgs = built.Text, built.Args

		subRoles, err := roles.ListSubordinateDataRoleIDs(ctx, rule.SharedToDataRoleID)
		if err != nil {
			return err
		}
		userIDs, err = roles.ListUserIDsInDataRoles(ctx, subRoles)
		if err != nil {
			return err
		}
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
DELETE FROM record_access_grants
WHERE object_api_name=$1 AND row_cause='rule' AND source_id=$2::uuid`, rule.ObjectAPIName, rule.ID); err != nil {
		return err
	}
	if rule.Active && len(userIDs) > 0 {
		args := append([]any{}, builtArgs...)
		args = append(args, rule.ObjectAPIName, userIDs, rule.AccessLevel, rule.ID)
		objParam, usersParam := len(builtArgs)+1, len(builtArgs)+2
		accessParam, ruleParam := len(builtArgs)+3, len(builtArgs)+4
		query := fmt.Sprintf(`
INSERT INTO record_access_grants
  (record_id, object_api_name, user_id, access_level, row_cause, source_id)
SELECT matched.id::uuid, $%d, target.user_id, $%d, 'rule', $%d::uuid
FROM (%s) matched
CROSS JOIN unnest($%d::uuid[]) AS target(user_id)
ON CONFLICT (object_api_name, record_id, user_id, row_cause, source_id)
DO UPDATE SET access_level=EXCLUDED.access_level`, objParam, accessParam, ruleParam, builtText, usersParam)
		if _, err := tx.Exec(ctx, query, args...); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func recalcRuleForRecord(ctx context.Context, pool *db.Pool, meta *metadata.Service, roles *db.DataRoleStore, rule db.SharingRule, recordID string) error {
	match := false
	var err error
	if rule.Active {
		match, err = recordMatchesRule(ctx, pool, meta, rule, recordID)
	}
	if err != nil || !match {
		if err != nil {
			return err
		}
	}
	var userIDs []string
	if match {
		subRoles, err := roles.ListSubordinateDataRoleIDs(ctx, rule.SharedToDataRoleID)
		if err != nil {
			return err
		}
		userIDs, err = roles.ListUserIDsInDataRoles(ctx, subRoles)
		if err != nil {
			return err
		}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
DELETE FROM record_access_grants
WHERE object_api_name=$1 AND row_cause='rule' AND source_id=$2::uuid AND record_id=$3::uuid`,
		rule.ObjectAPIName, rule.ID, recordID); err != nil {
		return err
	}
	if len(userIDs) > 0 {
		if _, err := tx.Exec(ctx, `
INSERT INTO record_access_grants
  (record_id, object_api_name, user_id, access_level, row_cause, source_id)
SELECT $1::uuid, $2, target.user_id, $3, 'rule', $4::uuid
FROM unnest($5::uuid[]) AS target(user_id)
ON CONFLICT (object_api_name, record_id, user_id, row_cause, source_id)
DO UPDATE SET access_level=EXCLUDED.access_level`,
			recordID, rule.ObjectAPIName, rule.AccessLevel, rule.ID, userIDs); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func recordMatchesRule(ctx context.Context, pool *db.Pool, meta *metadata.Service, rule db.SharingRule, recordID string) (bool, error) {
	filters, err := parseRuleFilters(rule.Criteria)
	if err != nil {
		return false, err
	}
	obj, err := meta.GetObject(ctx, rule.ObjectAPIName)
	if err != nil {
		return false, err
	}
	fields, err := meta.GetFields(ctx, rule.ObjectAPIName)
	if err != nil {
		return false, err
	}
	table := db.RecordsTableForStorageMode(obj.StorageMode)
	filters = append(filters, dataengine.QueryFilter{Field: "Id", Op: dataengine.OpEq, Value: recordID})
	req := &dataengine.QueryRequest{Object: rule.ObjectAPIName, Filters: filters, Limit: 1}
	built, err := dataengine.BuildCriteriaSQL(req, fields, table)
	if err != nil {
		return false, err
	}
	args := append(built.Args, recordID)
	q := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM (%s) sub WHERE sub.id = $%d::uuid)", built.Text, len(args))
	var ok bool
	err = pool.QueryRow(ctx, q, args...).Scan(&ok)
	return ok, err
}

func parseRuleFilters(raw json.RawMessage) ([]dataengine.QueryFilter, error) {
	var body struct {
		Filters []dataengine.QueryFilter `json:"filters"`
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("sharing rule criteria missing")
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	if len(body.Filters) == 0 {
		// Fail closed: empty criteria must not share every record.
		return nil, fmt.Errorf("sharing rule criteria.filters must be non-empty")
	}
	return body.Filters, nil
}
