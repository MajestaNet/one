package worker

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/MajestaNet/ide/internal/db"
)

// RunOptions configures the worker polling loop.
type RunOptions struct {
	PollMs         int
	ProcessOptions *ProcessOptions
}

var (
	hvRollMu       sync.Mutex
	hvRollLastRun  time.Time
	hvRollInterval = 1 * time.Hour
)

// Run starts the worker polling loop. It blocks until ctx is cancelled.
func Run(ctx context.Context, pool *db.Pool, opts RunOptions) {
	pollMs := opts.PollMs
	if pollMs <= 0 {
		pollMs = 2000
	}
	ticker := time.NewTicker(time.Duration(pollMs) * time.Millisecond)
	defer ticker.Stop()

	log.Printf("[worker] poll loop started poll_ms=%d", pollMs)
	for {
		select {
		case <-ctx.Done():
			log.Printf("[worker] poll loop stopped: %v", ctx.Err())
			return
		case <-ticker.C:
			maybeRollHighVolumePartitions(ctx, pool)
			maybeRunRetention(ctx, pool, opts.ProcessOptions)
			if err := runOnce(ctx, pool, opts.ProcessOptions); err != nil {
				log.Printf("[worker] poll error: %v", err)
			}
		}
	}
}

func maybeRollHighVolumePartitions(ctx context.Context, pool *db.Pool) {
	hvRollMu.Lock()
	defer hvRollMu.Unlock()
	if time.Since(hvRollLastRun) < hvRollInterval {
		return
	}
	created, err := db.EnsureHighVolumeRangePartitions(ctx, pool, time.Now().UTC())
	hvRollLastRun = time.Now()
	if err != nil {
		log.Printf("[worker] hv.partition.roll error: %v", err)
		return
	}
	if len(created) > 0 {
		log.Printf("[worker] hv.partition.roll created=%v", created)
	}
}

func runOnce(ctx context.Context, pool *db.Pool, opts *ProcessOptions) error {
	jobs, err := ProcessJobs(ctx, pool, opts)
	if err != nil {
		return err
	}
	outbox, err := ProcessOutbox(ctx, pool, opts)
	if err != nil {
		return err
	}
	if jobs > 0 || outbox > 0 {
		log.Printf("[worker] processed jobs=%d outbox=%d", jobs, outbox)
	}
	return nil
}
