package config_test

import (
	"testing"

	"github.com/MajestaNet/ide/internal/config"
)

func TestLoadRetentionAndPool(t *testing.T) {
	cfg, err := config.LoadFromEnv([]string{
		"APP_ENV=development",
		"API_KEYS=ops:client+admin",
		"DB_MAX_CONNS=20",
		"DB_MIN_CONNS=2",
		"RETENTION_JOBS_DAYS=14",
		"RETENTION_OUTBOX_DAYS=0",
		"RETENTION_AUDIT_LOG_DAYS=365",
		"RETENTION_BATCH_SIZE=1000",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DBMaxConns != 20 || cfg.DBMinConns != 2 {
		t.Fatalf("pool=%d/%d", cfg.DBMaxConns, cfg.DBMinConns)
	}
	if cfg.RetentionJobsDays != 14 {
		t.Fatalf("retention jobs=%d", cfg.RetentionJobsDays)
	}
	if cfg.RetentionOutboxDays != 0 {
		t.Fatalf("outbox days=%d want 0 (disabled)", cfg.RetentionOutboxDays)
	}
	if cfg.RetentionAuditLogDays != 365 || cfg.RetentionBatchSize != 1000 {
		t.Fatalf("audit/batch=%d/%d", cfg.RetentionAuditLogDays, cfg.RetentionBatchSize)
	}
}
