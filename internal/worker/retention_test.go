package worker_test

import (
	"os"
	"testing"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/worker"
)

func TestRunRetentionPurgesJobs(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := t.Context()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(ctx, `
INSERT INTO jobs (job_type, payload, status, run_at, completed_at)
VALUES ('test.retention', '{}'::jsonb, 'completed', now() - interval '60 days', now() - interval '60 days')`)
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := worker.RunRetentionPurges(ctx, pool, worker.RetentionOptions{
		JobsDays:  30,
		BatchSize: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if deleted["jobs"] < 1 {
		t.Fatalf("expected jobs purge, got %#v", deleted)
	}
}
