package worker_test

import (
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/worker"
)

func TestProcessJobsIngestProcessRequiresDataEngineAndJobID(t *testing.T) {
	ctx, pool := setupWorkerDB(t)

	var missingEngine string
	if err := pool.QueryRow(ctx, `
INSERT INTO jobs (job_type, payload, status, run_at)
VALUES ('ingest.process', '{"ingestJobId":"00000000-0000-4000-8000-000000000099"}'::jsonb, 'pending', now() - interval '1 hour')
RETURNING id::text`).Scan(&missingEngine); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE id=$1::uuid`, missingEngine) })
	if _, err := worker.ProcessJobs(ctx, pool, &worker.ProcessOptions{JobID: missingEngine, WorkerID: "ingest-missing-de"}); err != nil {
		t.Fatal(err)
	}
	var status, lastErr string
	if err := pool.QueryRow(ctx, `SELECT status, COALESCE(last_error, '') FROM jobs WHERE id=$1::uuid`, missingEngine).Scan(&status, &lastErr); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || !strings.Contains(lastErr, "DataEngine not configured") {
		t.Fatalf("status=%s last_error=%q", status, lastErr)
	}

	var missingID string
	if err := pool.QueryRow(ctx, `
INSERT INTO jobs (job_type, payload, status, run_at)
VALUES ('ingest.process', '{}'::jsonb, 'pending', now() - interval '1 hour')
RETURNING id::text`).Scan(&missingID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE id=$1::uuid`, missingID) })
	if _, err := worker.ProcessJobs(ctx, pool, &worker.ProcessOptions{
		JobID: missingID, WorkerID: "ingest-missing-id", DataEngine: &dataengine.Service{},
	}); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status, COALESCE(last_error, '') FROM jobs WHERE id=$1::uuid`, missingID).Scan(&status, &lastErr); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || !strings.Contains(lastErr, "missing ingestJobId") {
		t.Fatalf("empty payload: status=%s last_error=%q", status, lastErr)
	}
}
