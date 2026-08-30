// Package testutil provides shared Postgres and HTTP harnesses for Majesta One integration tests.
//
// Prefer these helpers for new DB-gated suites (especially under internal/httpapi).
// Unit tests that do not need a live database should keep local fakes.
//
// Typical flow:
//
//	db := testutil.RequireDatabase(t)
//	testutil.BootstrapCore(t, db, testutil.BootstrapOptions{})
//	srv := testutil.NewTestServer(t, db, testutil.ServerOptions{APIKeys: "admin-key+admin"})
//	rr := testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/me", "admin-key", nil)
package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/identity"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/seed"
)

// inferenceConfigLockKey is a session advisory lock for the singleton install_inference_config row.
// go test runs packages as separate processes, so a process-local mutex is not enough.
const inferenceConfigLockKey int64 = 87006006

// LockInferenceConfig serializes tests that mutate or depend on install_inference_config.
// Hold is tied to one pool connection until the test ends.
func LockInferenceConfig(t *testing.T, pool *db.Pool) {
	t.Helper()
	if pool == nil {
		t.Fatal("LockInferenceConfig: nil pool")
	}
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("LockInferenceConfig: %v", err)
	}
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, inferenceConfigLockKey); err != nil {
		conn.Release()
		t.Fatalf("LockInferenceConfig: %v", err)
	}
	t.Cleanup(func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, inferenceConfigLockKey)
		conn.Release()
	})
}

// ResetInferenceConfig sets the singleton inference source to none.
func ResetInferenceConfig(t *testing.T, pool *db.Pool) {
	t.Helper()
	_, _ = pool.Exec(context.Background(), `UPDATE install_inference_config SET active_source='none', default_provider_api_name=NULL WHERE id=1`)
}

// DefaultOwnerID matches .env.example DEFAULT_OWNER_ID for local/dev fixtures.
const DefaultOwnerID = "00000000-0000-4000-8000-000000000001"

// Database is a migrated pool plus metadata service for integration tests.
type Database struct {
	Pool *db.Pool
	Meta *metadata.Service
}

// RequireDatabase skips the test when DATABASE_URL is unset, then connects and ensures kernel migrations.
func RequireDatabase(t *testing.T) *Database {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := t.Context()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatal(err)
	}
	return &Database{Pool: pool, Meta: metadata.NewService(pool)}
}

// BootstrapOptions customizes core package seed for a test.
// Zero value seeds with DefaultOwnerID, feature flag "agents", and AutoSeed true.
type BootstrapOptions struct {
	OwnerID      string
	FeatureFlags []string
	// SkipAutoSeed leaves AutoSeed false (rarely needed).
	SkipAutoSeed bool
	// SkipControlIDE skips EnsureControlIDE even when AutoSeed is on.
	SkipControlIDE bool
	// Identity optional Cognito/memory backend for managed integration seed.
	Identity identity.Backend
	// EncryptionKey for revealable secrets.
	EncryptionKey string
}

// BootstrapCore seeds managed core onto the test database.
func BootstrapCore(t *testing.T, d *Database, opts BootstrapOptions) {
	t.Helper()
	if d == nil || d.Pool == nil || d.Meta == nil {
		t.Fatal("BootstrapCore: nil database")
	}
	owner := opts.OwnerID
	if owner == "" {
		owner = DefaultOwnerID
	}
	flags := opts.FeatureFlags
	if flags == nil {
		flags = []string{"agents"}
	}
	if err := seed.Bootstrap(t.Context(), d.Pool, d.Meta, seed.Options{
		OwnerID:        owner,
		FeatureFlags:   flags,
		AutoSeed:       !opts.SkipAutoSeed,
		SkipControlIDE: opts.SkipControlIDE,
		Identity:       opts.Identity,
		EncryptionKey:  opts.EncryptionKey,
	}); err != nil {
		t.Fatal(err)
	}
}
