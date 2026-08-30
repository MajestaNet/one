package dataengine

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/MajestaNet/ide/internal/db"
)

// ensureRecordPartitionsForWrite attaches physical partitions before opening a
// records write transaction. Partition DDL needs ACCESS EXCLUSIVE on the parent
// table and must not run while another connection holds an open records txn.
func (s *Service) ensureRecordPartitionsForWrite(ctx context.Context, objectAPIName, triggerEvent string) error {
	obj, err := s.meta.GetObject(ctx, objectAPIName)
	if err != nil {
		return err
	}
	if err := ensureObjectPartition(ctx, s.pool, obj.APIName, obj.StorageMode); err != nil {
		return err
	}
	targets, err := s.syncAutomationCreateTargets(ctx, objectAPIName, triggerEvent)
	if err != nil {
		return err
	}
	for _, target := range targets {
		tobj, err := s.meta.GetObject(ctx, target)
		if err != nil {
			return err
		}
		if err := ensureObjectPartition(ctx, s.pool, tobj.APIName, tobj.StorageMode); err != nil {
			return err
		}
	}
	return nil
}

func ensureObjectPartition(ctx context.Context, pool *db.Pool, objectAPIName, storageMode string) error {
	if err := rejectKernelStorage(objectAPIName, storageMode); err != nil {
		return err
	}
	if storageMode == db.StorageModeHighVolume {
		if err := db.EnsureHighVolumePartition(ctx, pool, objectAPIName); err != nil {
			return fmt.Errorf("ensure high_volume partition: %w", err)
		}
		return nil
	}
	if err := db.EnsureFlexiblePartition(ctx, pool, objectAPIName); err != nil {
		return fmt.Errorf("ensure flexible partition: %w", err)
	}
	return nil
}

func (s *Service) syncAutomationCreateTargets(ctx context.Context, objectAPIName, triggerEvent string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
SELECT actions FROM metadata_automations
WHERE object_api_name = $1 AND active = true AND execution = 'sync'
  AND trigger_event IN ($2, 'write')`, objectAPIName, triggerEvent)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	var out []string
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var actions []map[string]any
		if err := json.Unmarshal(raw, &actions); err != nil {
			continue
		}
		for _, a := range actions {
			typ, _ := a["type"].(string)
			if typ != "createRecord" && typ != "create" {
				continue
			}
			target, _ := a["objectApiName"].(string)
			if target == "" {
				continue
			}
			if !seen[target] {
				seen[target] = true
				out = append(out, target)
			}
		}
	}
	return out, rows.Err()
}
