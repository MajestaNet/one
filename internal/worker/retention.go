package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/MajestaNet/ide/internal/db"
)

// RetentionOptions configures purge batch jobs. Zero Days disables that purge.
// Soft-delete retention was removed in 0037 (hard-delete only).
type RetentionOptions struct {
	JobsDays     int
	OutboxDays   int
	AuditLogDays int
	BatchSize    int
}

func (o RetentionOptions) batch() int {
	if o.BatchSize <= 0 {
		return 5000
	}
	return o.BatchSize
}

// RunRetentionPurges deletes completed system rows (jobs/outbox/audit).
func RunRetentionPurges(ctx context.Context, pool *db.Pool, opts RetentionOptions) (map[string]int64, error) {
	if pool == nil {
		return nil, fmt.Errorf("pool required")
	}
	out := map[string]int64{}
	batch := opts.batch()

	if opts.JobsDays > 0 {
		n, err := purgeCompletedJobs(ctx, pool, opts.JobsDays, batch)
		if err != nil {
			return out, err
		}
		out["jobs"] = n
	}
	if opts.OutboxDays > 0 {
		n, err := purgePublishedOutbox(ctx, pool, opts.OutboxDays, batch)
		if err != nil {
			return out, err
		}
		out["outbox"] = n
	}
	if opts.AuditLogDays > 0 {
		n, err := purgeAuditLog(ctx, pool, opts.AuditLogDays, batch)
		if err != nil {
			return out, err
		}
		out["audit_log"] = n
	}
	return out, nil
}

func purgeCompletedJobs(ctx context.Context, pool *db.Pool, days, batch int) (int64, error) {
	tag, err := pool.Exec(ctx, `
DELETE FROM jobs
WHERE id IN (
  SELECT id FROM jobs
  WHERE status IN ('completed', 'failed')
    AND coalesce(completed_at, run_at) < now() - make_interval(days => $1)
  LIMIT $2
)`, days, batch)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func purgePublishedOutbox(ctx context.Context, pool *db.Pool, days, batch int) (int64, error) {
	tag, err := pool.Exec(ctx, `
DELETE FROM outbox_events
WHERE id IN (
  SELECT id FROM outbox_events
  WHERE published_at IS NOT NULL
    AND published_at < now() - make_interval(days => $1)
  LIMIT $2
)`, days, batch)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func purgeAuditLog(ctx context.Context, pool *db.Pool, days, batch int) (int64, error) {
	tag, err := pool.Exec(ctx, `
DELETE FROM audit_log
WHERE id IN (
  SELECT id FROM audit_log
  WHERE created_at < now() - make_interval(days => $1)
  LIMIT $2
)`, days, batch)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

var (
	retentionMu       sync.Mutex
	retentionLastRun  time.Time
	retentionInterval = 1 * time.Hour
)

func maybeRunRetention(ctx context.Context, pool *db.Pool, opts *ProcessOptions) {
	if opts == nil || opts.Retention == nil {
		return
	}
	retentionMu.Lock()
	defer retentionMu.Unlock()
	if time.Since(retentionLastRun) < retentionInterval {
		return
	}
	retentionLastRun = time.Now()
	deleted, err := RunRetentionPurges(ctx, pool, *opts.Retention)
	if err != nil {
		log.Printf("[worker] retention.purge error: %v", err)
		return
	}
	total := int64(0)
	for _, n := range deleted {
		total += n
	}
	if total > 0 {
		log.Printf("[worker] retention.purge deleted=%v", deleted)
	}
}
