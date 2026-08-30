package worker_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/worker"
)

func setupWorkerDB(t *testing.T) (context.Context, *db.Pool) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE status IN ('pending', 'running', 'failed')`)
	// Shared DATABASE_URL accumulates unpublished outbox rows from other packages
	// (httpapi claim/principal tests). Claim/process limits then starve these fixtures.
	_, _ = pool.Exec(ctx, `DELETE FROM outbox_events WHERE published_at IS NULL`)
	return ctx, pool
}

func TestCreateWorkerID(t *testing.T) {
	id1 := worker.CreateWorkerID("test")
	id2 := worker.CreateWorkerID("test")
	if id1 == "" {
		t.Fatal("expected non-empty worker ID")
	}
	if id1 == id2 {
		// Very unlikely but possible; just log rather than fail.
		t.Logf("worker IDs collided (improbable): %s", id1)
	}
}

func TestReclaimExpiredJobLeases(t *testing.T) {
	ctx, pool := setupWorkerDB(t)

	// Insert a fake running job with an expired lock.
	var jobID string
	err := pool.QueryRow(ctx, `
INSERT INTO jobs (job_type, payload, status, locked_at, locked_by)
VALUES ('test.claim', '{}', 'running', now() - interval '10 minutes', 'dead-worker')
RETURNING id::text`).Scan(&jobID)
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE id=$1::uuid`, jobID) })

	count, err := worker.ReclaimExpiredJobLeases(ctx, pool, worker.DefaultLeaseMS)
	if err != nil {
		t.Fatalf("ReclaimExpiredJobLeases: %v", err)
	}
	if count == 0 {
		t.Fatal("expected at least one reclaimed job")
	}

	// Verify it was reset to pending.
	var status string
	_ = pool.QueryRow(ctx, `SELECT status FROM jobs WHERE id=$1::uuid`, jobID).Scan(&status)
	if status != "pending" {
		t.Fatalf("expected pending, got %s", status)
	}
}

func TestClaimJobs(t *testing.T) {
	ctx, pool := setupWorkerDB(t)

	var jobID string
	err := pool.QueryRow(ctx, `
INSERT INTO jobs (job_type, payload, status) VALUES ('test.claim', '{}', 'pending')
RETURNING id::text`).Scan(&jobID)
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE id=$1::uuid`, jobID) })

	wid := worker.CreateWorkerID("test")
	jobs, err := worker.ClaimJobs(ctx, pool, wid, 10)
	if err != nil {
		t.Fatalf("ClaimJobs: %v", err)
	}
	found := false
	for _, j := range jobs {
		if j.ID == jobID {
			found = true
			if j.JobType != "test.claim" {
				t.Fatalf("unexpected job_type: %s", j.JobType)
			}
			if j.Status != "running" {
				t.Fatalf("expected running, got %s", j.Status)
			}
		}
	}
	if !found {
		t.Fatal("inserted job not found in claimed batch")
	}
}

func TestClaimJobByID(t *testing.T) {
	ctx, pool := setupWorkerDB(t)
	var otherID, targetID string
	if err := pool.QueryRow(ctx, `
INSERT INTO jobs (job_type, payload, status) VALUES ('test.other', '{}', 'pending')
RETURNING id::text`).Scan(&otherID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
INSERT INTO jobs (job_type, payload, status) VALUES ('test.target', '{}', 'pending')
RETURNING id::text`).Scan(&targetID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE id=$1::uuid OR id=$2::uuid`, otherID, targetID)
	})
	got, err := worker.ClaimJobByID(ctx, pool, "claim-one", targetID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != targetID || got.JobType != "test.target" {
		t.Fatalf("claimed %+v", got)
	}
	var otherStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM jobs WHERE id=$1::uuid`, otherID).Scan(&otherStatus); err != nil {
		t.Fatal(err)
	}
	if otherStatus != "pending" {
		t.Fatalf("other job status=%s", otherStatus)
	}
}

func TestReclaimExpiredOutboxLeases(t *testing.T) {
	ctx, pool := setupWorkerDB(t)

	// We need a valid user for outbox event insertion (some schemas require it).
	var evID string
	err := pool.QueryRow(ctx, `
INSERT INTO outbox_events (event_type, payload, locked_at, locked_by)
VALUES ('test.event', '{}', now() - interval '10 minutes', 'dead-worker')
RETURNING id::text`).Scan(&evID)
	if err != nil {
		t.Fatalf("insert outbox event: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM outbox_events WHERE id=$1::uuid`, evID) })

	count, err := worker.ReclaimExpiredOutboxLeases(ctx, pool, worker.DefaultLeaseMS)
	if err != nil {
		t.Fatalf("ReclaimExpiredOutboxLeases: %v", err)
	}
	if count == 0 {
		t.Fatal("expected at least one reclaimed outbox event")
	}
}

func TestClaimOutboxEvents(t *testing.T) {
	ctx, pool := setupWorkerDB(t)
	_, _ = pool.Exec(ctx, `
UPDATE outbox_events
SET published_at = COALESCE(published_at, now()), locked_at = NULL, locked_by = NULL
WHERE published_at IS NULL`)

	var evID string
	err := pool.QueryRow(ctx, `
INSERT INTO outbox_events (event_type, payload) VALUES ('test.outbox', '{"x":1}')
RETURNING id::text`).Scan(&evID)
	if err != nil {
		t.Fatalf("insert outbox: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM outbox_events WHERE id=$1::uuid`, evID) })

	wid := worker.CreateWorkerID("test")
	// Use a large limit so leftover events from other tests don't crowd out our event.
	_, err = worker.ClaimOutboxEvents(ctx, pool, wid, 1000, worker.DefaultLeaseMS)
	if err != nil {
		t.Fatalf("ClaimOutboxEvents: %v", err)
	}

	// Verify our event was claimed (locked by this worker).
	var lockedBy *string
	var eventType string
	_ = pool.QueryRow(ctx, `SELECT locked_by, event_type FROM outbox_events WHERE id=$1::uuid`, evID).
		Scan(&lockedBy, &eventType)
	if lockedBy == nil || *lockedBy != wid {
		t.Fatalf("expected event locked by %s, got %v", wid, lockedBy)
	}
	if eventType != "test.outbox" {
		t.Fatalf("unexpected event_type: %s", eventType)
	}
}
