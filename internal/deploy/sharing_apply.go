package deploy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/MajestaNet/ide/internal/db"
)

// applySharingMetadata imports data roles, object OWD, and sharing rules from a snapshot map.
func applySharingMetadata(ctx context.Context, pool *db.Pool, snapshot map[string]any, dryRun bool) error {
	if dryRun || pool == nil {
		return nil
	}
	sharing := db.NewSharingStore(pool)
	roles := db.NewDataRoleStore(pool)

	if raw, ok := snapshot["dataRoles"].([]any); ok {
		for _, item := range raw {
			m, _ := item.(map[string]any)
			if m == nil {
				continue
			}
			apiName, _ := m["apiName"].(string)
			label, _ := m["label"].(string)
			if apiName == "" {
				continue
			}
			var parentID *string
			if p, ok := m["parentDataRoleApiName"].(string); ok && p != "" {
				parent, err := roles.GetDataRoleByAPIName(ctx, p)
				if err != nil {
					return fmt.Errorf("sharing data role parent %s: %w", p, err)
				}
				parentID = &parent.ID
			}
			if _, err := roles.GetDataRoleByAPIName(ctx, apiName); err == db.ErrNotFound {
				if _, err := roles.CreateDataRole(ctx, apiName, label, parentID); err != nil {
					return fmt.Errorf("create data role %s: %w", apiName, err)
				}
			} else if err != nil {
				return err
			} else {
				if _, err := roles.UpdateDataRole(ctx, apiName, label, parentID, parentID == nil && m["parentDataRoleApiName"] == ""); err != nil {
					return fmt.Errorf("update data role %s: %w", apiName, err)
				}
			}
		}
	}

	if raw, ok := snapshot["objectSharingSettings"].([]any); ok {
		for _, item := range raw {
			m, _ := item.(map[string]any)
			if m == nil {
				continue
			}
			obj, _ := m["objectApiName"].(string)
			access, _ := m["defaultAccess"].(string)
			rulesEnabled, _ := m["sharingRulesEnabled"].(bool)
			if obj == "" {
				continue
			}
			if err := sharing.EnsureObjectSharingSettings(ctx, obj); err != nil {
				return fmt.Errorf("ensure object sharing %s: %w", obj, err)
			}
			re := rulesEnabled
			if _, err := sharing.UpdateObjectSharingSettings(ctx, obj, access, &re); err != nil {
				return fmt.Errorf("update object sharing %s: %w", obj, err)
			}
		}
	}

	if raw, ok := snapshot["sharingRules"].([]any); ok {
		for _, item := range raw {
			m, _ := item.(map[string]any)
			if m == nil {
				continue
			}
			objectAPIName, _ := m["objectApiName"].(string)
			apiName, _ := m["apiName"].(string)
			label, _ := m["label"].(string)
			active, _ := m["active"].(bool)
			accessLevel, _ := m["accessLevel"].(string)
			roleAPIName, _ := m["sharedToDataRoleApiName"].(string)
			sortOrderF, _ := m["sortOrder"].(float64)
			if objectAPIName == "" || apiName == "" || roleAPIName == "" {
				continue
			}
			role, err := roles.GetDataRoleByAPIName(ctx, roleAPIName)
			if err != nil {
				return fmt.Errorf("sharing rule %s grantee %s: %w", apiName, roleAPIName, err)
			}
			criteria, _ := json.Marshal(m["criteria"])
			if _, err := sharing.GetSharingRule(ctx, objectAPIName, apiName); err == db.ErrNotFound {
				if _, err := sharing.CreateSharingRule(ctx, db.SharingRule{
					ObjectAPIName:      objectAPIName,
					APIName:            apiName,
					Label:              label,
					Active:             active,
					AccessLevel:        accessLevel,
					SharedToDataRoleID: role.ID,
					Criteria:           criteria,
					SortOrder:          int(sortOrderF),
				}); err != nil {
					return fmt.Errorf("create sharing rule %s.%s: %w", objectAPIName, apiName, err)
				}
			} else if err != nil {
				return err
			} else {
				if _, err := sharing.UpdateSharingRule(ctx, objectAPIName, apiName, map[string]any{
					"label": label, "active": active, "accessLevel": accessLevel,
					"sharedToDataRoleId": role.ID, "criteria": m["criteria"], "sortOrder": sortOrderF,
				}); err != nil {
					return fmt.Errorf("update sharing rule %s.%s: %w", objectAPIName, apiName, err)
				}
			}
		}
	}
	if err := db.EnqueueSharingRecalc(ctx, pool, map[string]any{"scope": "full"}); err != nil {
		return fmt.Errorf("enqueue sharing.recalc: %w", err)
	}
	return nil
}
