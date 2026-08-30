package db

import (
	"context"
	"fmt"
	"strings"
)

// EnsureFlexiblePartition attaches a LIST partition under records for objectAPIName
// when missing. Known core/module objects are created by migration 0036 and recorded
// in flexible_objects — this is a no-op for them. There is no DEFAULT partition
// (0037): inserts fail until a dedicated leaf exists. Product/worker DDL only.
func EnsureFlexiblePartition(ctx context.Context, pool *Pool, objectAPIName string) error {
	if objectAPIName == "" {
		return fmt.Errorf("objectAPIName required")
	}
	if err := assertSafeObjectAPIName(objectAPIName); err != nil {
		return err
	}

	var existing string
	err := pool.QueryRow(ctx, `
SELECT partition_name FROM flexible_objects WHERE object_api_name = $1`, objectAPIName).Scan(&existing)
	if err == nil && existing != "" {
		return nil
	}

	partName := flexiblePartitionName(objectAPIName)
	ddl := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES IN ('%s')`,
		partName, RecordsTableFlexible, escapeLiteral(objectAPIName),
	)
	if _, err := pool.Exec(ctx, ddl); err != nil {
		var again string
		if qerr := pool.QueryRow(ctx, `
SELECT partition_name FROM flexible_objects WHERE object_api_name = $1`, objectAPIName).Scan(&again); qerr == nil && again != "" {
			return nil
		}
		return fmt.Errorf("create flexible partition %s: %w", partName, err)
	}
	_, err = pool.Exec(ctx, `
INSERT INTO flexible_objects (object_api_name, partition_name)
VALUES ($1, $2)
ON CONFLICT (object_api_name) DO NOTHING`, objectAPIName, partName)
	return err
}

func flexiblePartitionName(objectAPIName string) string {
	raw := strings.ToLower("records_o_" + objectAPIName)
	name := hvIdentSafe.ReplaceAllString(raw, "_")
	if len(name) > 60 {
		name = name[:60]
	}
	return name
}
