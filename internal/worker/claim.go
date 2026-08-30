package worker

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/jackc/pgx/v5"
)

// DefaultLeaseMS is the job/outbox lease duration (5 minutes).
const DefaultLeaseMS = 5 * 60 * 1000

// ClaimedJob mirrors the jobs table columns returned after a claim UPDATE.
type ClaimedJob struct {
	ID          string
	JobType     string
	Payload     []byte // raw JSONB
	Status      string
	RunAt       time.Time
	Attempts    int
	LastError   *string
	CreatedAt   time.Time
	CompletedAt *time.Time
	LockedAt    *time.Time
	LockedBy    *string
}

// ClaimedOutboxEvent mirrors the outbox_events columns returned after a claim UPDATE.
type ClaimedOutboxEvent struct {
	ID            string
	EventType     string
	ObjectAPIName *string
	RecordID      *string
	Payload       []byte // raw JSONB
	CreatedAt     time.Time
	PublishedAt   *time.Time
	Attempts      int
	LastError     *string
	LockedAt      *time.Time
	LockedBy      *string
}

// CreateWorkerID returns a unique worker identifier.
func CreateWorkerID(prefix string) string {
	if prefix == "" {
		prefix = "worker"
	}
	return fmt.Sprintf("%s-%d-%06x", prefix, os.Getpid(), rand.Int63n(1<<24))
}

// ReclaimExpiredJobLeases resets running jobs whose lease expired back to pending.
// Returns the number of reclaimed jobs.
func ReclaimExpiredJobLeases(ctx context.Context, pool *db.Pool, leaseMs int64) (int, error) {
	if leaseMs <= 0 {
		leaseMs = DefaultLeaseMS
	}
	tag, err := pool.Exec(ctx, `
UPDATE jobs
SET status = 'pending',
    locked_at = NULL,
    locked_by = NULL
WHERE status = 'running'
  AND locked_at IS NOT NULL
  AND locked_at < now() - ($1::double precision * interval '1 millisecond')`,
		leaseMs)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// ReclaimExpiredOutboxLeases clears stale locks on unpublished outbox events.
func ReclaimExpiredOutboxLeases(ctx context.Context, pool *db.Pool, leaseMs int64) (int, error) {
	if leaseMs <= 0 {
		leaseMs = DefaultLeaseMS
	}
	tag, err := pool.Exec(ctx, `
UPDATE outbox_events
SET locked_at = NULL,
    locked_by = NULL
WHERE published_at IS NULL
  AND locked_at IS NOT NULL
  AND locked_at < now() - ($1::double precision * interval '1 millisecond')`,
		leaseMs)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// ClaimJobs atomically claims up to limit pending jobs for this worker.
func ClaimJobs(ctx context.Context, pool *db.Pool, workerID string, limit int) ([]ClaimedJob, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := pool.Query(ctx, `
UPDATE jobs
SET status = 'running',
    attempts = attempts + 1,
    locked_at = now(),
    locked_by = $1
WHERE id IN (
  SELECT id FROM jobs
  WHERE status = 'pending'
    AND run_at <= now()
  ORDER BY run_at
  FOR UPDATE SKIP LOCKED
  LIMIT $2
)
RETURNING id::text, job_type, payload, status, run_at, attempts, last_error,
          created_at, completed_at, locked_at, locked_by`,
		workerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []ClaimedJob
	for rows.Next() {
		var j ClaimedJob
		if err := rows.Scan(
			&j.ID, &j.JobType, &j.Payload, &j.Status, &j.RunAt, &j.Attempts, &j.LastError,
			&j.CreatedAt, &j.CompletedAt, &j.LockedAt, &j.LockedBy,
		); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// ClaimJobByID claims one pending job by id (tests and targeted resume).
func ClaimJobByID(ctx context.Context, pool *db.Pool, workerID, jobID string) (*ClaimedJob, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return nil, nil
	}
	var j ClaimedJob
	err := pool.QueryRow(ctx, `
UPDATE jobs
SET status = 'running',
    attempts = attempts + 1,
    locked_at = now(),
    locked_by = $1
WHERE id = $2::uuid
  AND status = 'pending'
  AND run_at <= now()
RETURNING id::text, job_type, payload, status, run_at, attempts, last_error,
          created_at, completed_at, locked_at, locked_by`,
		workerID, jobID).Scan(
		&j.ID, &j.JobType, &j.Payload, &j.Status, &j.RunAt, &j.Attempts, &j.LastError,
		&j.CreatedAt, &j.CompletedAt, &j.LockedAt, &j.LockedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &j, nil
}

// ClaimOutboxEvents atomically claims up to limit unpublished outbox events.
func ClaimOutboxEvents(ctx context.Context, pool *db.Pool, workerID string, limit int, leaseMs int64) ([]ClaimedOutboxEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	if leaseMs <= 0 {
		leaseMs = DefaultLeaseMS
	}
	rows, err := pool.Query(ctx, `
UPDATE outbox_events
SET locked_at = now(),
    locked_by = $1,
    attempts = attempts + 1
WHERE id IN (
  SELECT id FROM outbox_events
  WHERE published_at IS NULL
    AND (
      locked_at IS NULL
      OR locked_at < now() - ($3::double precision * interval '1 millisecond')
    )
  ORDER BY created_at
  FOR UPDATE SKIP LOCKED
  LIMIT $2
)
RETURNING id::text, event_type, object_api_name, record_id::text, payload, created_at,
          published_at, attempts, last_error, locked_at, locked_by`,
		workerID, limit, leaseMs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []ClaimedOutboxEvent
	for rows.Next() {
		var e ClaimedOutboxEvent
		if err := rows.Scan(
			&e.ID, &e.EventType, &e.ObjectAPIName, &e.RecordID, &e.Payload,
			&e.CreatedAt, &e.PublishedAt, &e.Attempts, &e.LastError,
			&e.LockedAt, &e.LockedBy,
		); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
