package worker_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/worker"
)

func TestProcessJobsSharingRecalcWithoutMetadata(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatal(err)
	}

	_, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE job_type = 'sharing.recalc'`)
	payload, _ := json.Marshal(map[string]any{"scope": "hierarchy"})
	var jobID string
	if err := pool.QueryRow(ctx, `
INSERT INTO jobs (job_type, payload, status)
VALUES ('sharing.recalc', $1::jsonb, 'pending')
RETURNING id::text`, string(payload)).Scan(&jobID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM jobs WHERE id=$1::uuid`, jobID) })

	// Metadata omitted on purpose — handler must auto-construct and complete.
	n, err := worker.ProcessJobs(ctx, pool, &worker.ProcessOptions{JobLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("expected job processed")
	}
	var status, lastErr string
	_ = pool.QueryRow(ctx, `SELECT status, COALESCE(last_error,'') FROM jobs WHERE id=$1::uuid`, jobID).Scan(&status, &lastErr)
	if status != "completed" {
		t.Fatalf("status=%s err=%s", status, lastErr)
	}
	_ = metadata.NewService(pool) // keep import used if build tags change
}
