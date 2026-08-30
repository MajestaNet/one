package db

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var hvIdentSafe = regexp.MustCompile(`[^a-z0-9_]+`)

// RecordsTableFlexible is the default CRM-scale flexible store.
const RecordsTableFlexible = "records"

// RecordsTableHighVolume is the LIST/RANGE partitioned high-volume store (ADR-013).
const RecordsTableHighVolume = "records_hv"

// StorageModeFlexible is metadata_objects.storage_mode for shared records.
const StorageModeFlexible = "flexible"

// StorageModeHighVolume is metadata_objects.storage_mode for records_hv.
const StorageModeHighVolume = "high_volume"

// StorageModeKernel is metadata_objects.storage_mode for identity tables (User).
// Rows are not stored in records / records_hv (ADR-026).
const StorageModeKernel = "kernel"

// IsKernelStorage reports whether the object is a kernel identity table.
func IsKernelStorage(mode string) bool {
	return mode == StorageModeKernel
}

// RecordsTableForStorageMode returns the physical table for a storage mode.
func RecordsTableForStorageMode(mode string) string {
	if mode == StorageModeHighVolume {
		return RecordsTableHighVolume
	}
	return RecordsTableFlexible
}

// EnsureHighVolumePartition attaches a LIST partition under records_hv for objectAPIName
// when missing. Range-partitioned objects are created by kernel migrations and recorded
// in high_volume_objects — this is a no-op for them.
// Product/worker DDL only; never used for customer custom field columns.
func EnsureHighVolumePartition(ctx context.Context, pool *Pool, objectAPIName string) error {
	if objectAPIName == "" {
		return fmt.Errorf("objectAPIName required")
	}
	if err := assertSafeObjectAPIName(objectAPIName); err != nil {
		return err
	}

	var existing string
	err := pool.QueryRow(ctx, `
SELECT partition_name FROM high_volume_objects WHERE object_api_name = $1`, objectAPIName).Scan(&existing)
	if err == nil && existing != "" {
		return nil
	}

	partName := highVolumePartitionName(objectAPIName)
	// Simple LIST leaf (no RANGE). Range-partitioned objects must ship via migrations.
	ddl := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES IN ('%s')`,
		partName, RecordsTableHighVolume, escapeLiteral(objectAPIName),
	)
	if _, err := pool.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("create high_volume partition %s: %w", partName, err)
	}
	_, err = pool.Exec(ctx, `
INSERT INTO high_volume_objects (object_api_name, partition_name, range_partitioned)
VALUES ($1, $2, false)
ON CONFLICT (object_api_name) DO NOTHING`, objectAPIName, partName)
	return err
}

func highVolumePartitionName(objectAPIName string) string {
	raw := strings.ToLower("records_hv_o_" + objectAPIName)
	name := hvIdentSafe.ReplaceAllString(raw, "_")
	if len(name) > 60 {
		name = name[:60]
	}
	return name
}

func assertSafeObjectAPIName(name string) error {
	if name == "" || len(name) > 128 {
		return fmt.Errorf("invalid object api name")
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return fmt.Errorf("invalid object api name: %s", name)
	}
	return nil
}

func escapeLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// HighVolumeRangeHorizonYears is how many calendar years ahead of "now" to ensure exist.
const HighVolumeRangeHorizonYears = 3

// EnsureHighVolumeRangePartitions attaches yearly RANGE partitions for range-partitioned
// HV objects from the latest known year through now+HighVolumeRangeHorizonYears.
// Keeps near-term years attached so there is no DEFAULT write sink (0037 dropped HV DEFAULT).
func EnsureHighVolumeRangePartitions(ctx context.Context, pool *Pool, now time.Time) (created []string, err error) {
	if pool == nil {
		return nil, fmt.Errorf("pool required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	rows, err := pool.Query(ctx, `
SELECT object_api_name, partition_name FROM high_volume_objects WHERE range_partitioned = true`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type hvObj struct {
		objectAPIName string
		parentPart    string
	}
	var objs []hvObj
	for rows.Next() {
		var o hvObj
		if err := rows.Scan(&o.objectAPIName, &o.parentPart); err != nil {
			return nil, err
		}
		objs = append(objs, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	targetEndYear := now.UTC().Year() + HighVolumeRangeHorizonYears
	for _, o := range objs {
		if err := assertSafeObjectAPIName(o.objectAPIName); err != nil {
			return created, err
		}
		parent := o.parentPart
		if !safeHVIdent(parent) {
			return created, fmt.Errorf("unsafe partition parent name %q", parent)
		}
		startYear, err := latestNamedRangeYear(ctx, pool, parent)
		if err != nil {
			return created, err
		}
		if startYear == 0 {
			startYear = now.UTC().Year()
		} else {
			startYear++ // next year after latest named partition
		}
		for y := startYear; y <= targetEndYear; y++ {
			leaf := fmt.Sprintf("%s_%d", parent, y)
			if !safeHVIdent(leaf) {
				return created, fmt.Errorf("unsafe leaf name %q", leaf)
			}
			from := fmt.Sprintf("%d-01-01", y)
			to := fmt.Sprintf("%d-01-01", y+1)
			ddl := fmt.Sprintf(
				`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')`,
				leaf, parent, from, to,
			)
			if _, err := pool.Exec(ctx, ddl); err != nil {
				return created, fmt.Errorf("create range partition %s: %w", leaf, err)
			}
			created = append(created, leaf)
		}
	}
	return created, nil
}

func latestNamedRangeYear(ctx context.Context, pool *Pool, parent string) (int, error) {
	// pg_inherits: child.inhparent = parent oid; filter child names like parent_YYYY
	prefix := parent + "_"
	rows, err := pool.Query(ctx, `
SELECT c.relname
FROM pg_class c
JOIN pg_inherits i ON i.inhrelid = c.oid
JOIN pg_class p ON p.oid = i.inhparent
WHERE p.relname = $1`, parent)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	maxYear := 0
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return 0, err
		}
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		suffix := strings.TrimPrefix(name, prefix)
		if len(suffix) != 4 {
			continue
		}
		y := 0
		for _, r := range suffix {
			if r < '0' || r > '9' {
				y = 0
				break
			}
			y = y*10 + int(r-'0')
		}
		if y > maxYear {
			maxYear = y
		}
	}
	return maxYear, rows.Err()
}

func safeHVIdent(name string) bool {
	if name == "" || len(name) > 63 {
		return false
	}
	for i, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		if i > 0 && r >= 'A' && r <= 'Z' {
			continue
		}
		return false
	}
	return true
}
