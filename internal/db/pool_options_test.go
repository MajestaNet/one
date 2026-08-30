package db_test

import (
	"os"
	"testing"

	"github.com/MajestaNet/ide/internal/db"
)

func TestPoolOptionsFromEnvDefaults(t *testing.T) {
	t.Setenv("DB_MAX_CONNS", "")
	t.Setenv("DB_MIN_CONNS", "")
	// Clear may not unset if already empty — set then clear via invalid
	_ = os.Unsetenv("DB_MAX_CONNS")
	_ = os.Unsetenv("DB_MIN_CONNS")
	opts := db.PoolOptionsFromEnv()
	if opts.MaxConns != db.DefaultPoolMaxConns {
		t.Fatalf("max=%d", opts.MaxConns)
	}
	if opts.MinConns != db.DefaultPoolMinConns {
		t.Fatalf("min=%d", opts.MinConns)
	}
}

func TestPoolOptionsFromEnvOverride(t *testing.T) {
	t.Setenv("DB_MAX_CONNS", "25")
	t.Setenv("DB_MIN_CONNS", "3")
	opts := db.PoolOptionsFromEnv()
	if opts.MaxConns != 25 || opts.MinConns != 3 {
		t.Fatalf("opts=%+v", opts)
	}
}

func TestPoolOptionsMinCappedByMax(t *testing.T) {
	t.Setenv("DB_MAX_CONNS", "5")
	t.Setenv("DB_MIN_CONNS", "20")
	opts := db.PoolOptionsFromEnv()
	if opts.MinConns != 5 {
		t.Fatalf("min=%d want 5", opts.MinConns)
	}
}
