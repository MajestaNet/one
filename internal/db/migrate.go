package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

const migrationsTable = "one_schema_migrations"

// Journal matches migrations/meta/_journal.json (ordered SQL apply list).
type Journal struct {
	Version string         `json:"version"`
	Dialect string         `json:"dialect"`
	Entries []JournalEntry `json:"entries"`
}

// JournalEntry is one ordered migration.
type JournalEntry struct {
	Idx  int    `json:"idx"`
	Tag  string `json:"tag"`
	When int64  `json:"when"`
}

// ResolveMigrationsPath finds the SQL migrations directory.
// Order: MIGRATIONS_PATH env (absolute or relative to cwd / ancestors), then common relative paths.
func ResolveMigrationsPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv("MIGRATIONS_PATH")); p != "" {
		if resolved, ok := resolveExistingDir(p); ok {
			return resolved, nil
		}
		return "", fmt.Errorf("MIGRATIONS_PATH %q is not a directory", p)
	}
	candidates := []string{
		"migrations",
		"../migrations",
		"../../migrations",
		"/migrations",
	}
	// Also search upward from cwd for go.mod + migrations/.
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for i := 0; i < 8; i++ {
			candidates = append(candidates, filepath.Join(dir, "migrations"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	for _, c := range candidates {
		if resolved, ok := resolveExistingDir(c); ok {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("migrations directory not found (set MIGRATIONS_PATH)")
}

// resolveExistingDir returns an absolute path when p (absolute or relative) names a directory.
// Relative paths are checked against cwd first, then each ancestor (so go test package CWDs still
// resolve MIGRATIONS_PATH=migrations from the repo root).
func resolveExistingDir(p string) (string, bool) {
	try := func(candidate string) (string, bool) {
		st, err := os.Stat(candidate)
		if err != nil || !st.IsDir() {
			return "", false
		}
		abs, err := filepath.Abs(candidate)
		if err != nil {
			return candidate, true
		}
		return abs, true
	}
	if filepath.IsAbs(p) {
		return try(p)
	}
	if abs, ok := try(p); ok {
		return abs, true
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if abs, ok := try(filepath.Join(dir, p)); ok {
			return abs, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}

// Migrate applies pending SQL migrations from the journal in order.
// Idempotent: records applied tags in one_schema_migrations.
func (p *Pool) Migrate(ctx context.Context, migrationsDir string) error {
	if migrationsDir == "" {
		var err error
		migrationsDir, err = ResolveMigrationsPath()
		if err != nil {
			return err
		}
	} else if resolved, ok := resolveExistingDir(migrationsDir); ok {
		migrationsDir = resolved
	} else {
		return fmt.Errorf("migrations directory %q not found", migrationsDir)
	}
	journal, err := loadJournal(migrationsDir)
	if err != nil {
		return err
	}
	if err := p.ensureMigrationsTable(ctx); err != nil {
		return err
	}
	applied, err := p.appliedMigrations(ctx)
	if err != nil {
		return err
	}

	for _, entry := range journal.Entries {
		if applied[entry.Tag] {
			continue
		}
		file := filepath.Join(migrationsDir, entry.Tag+".sql")
		// drizzle tags are e.g. 0000_kernel; files match.
		body, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Tag, err)
		}
		sql := string(body)
		if strings.TrimSpace(sql) == "" {
			continue
		}
		if err := p.execScript(ctx, sql); err != nil {
			return fmt.Errorf("apply %s: %w", entry.Tag, err)
		}
		if _, err := p.Exec(ctx,
			`INSERT INTO `+migrationsTable+` (id, applied_at) VALUES ($1, now()) ON CONFLICT (id) DO NOTHING`,
			entry.Tag,
		); err != nil {
			return fmt.Errorf("record %s: %w", entry.Tag, err)
		}
	}
	return nil
}

// EnsureKernel is the Go equivalent of Node ensureKernelSchema: apply migrations on boot.
func (p *Pool) EnsureKernel(ctx context.Context) error {
	dir, err := ResolveMigrationsPath()
	if err != nil {
		return err
	}
	return p.Migrate(ctx, dir)
}

func loadJournal(migrationsDir string) (*Journal, error) {
	path := filepath.Join(migrationsDir, "meta", "_journal.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read journal: %w", err)
	}
	var j Journal
	if err := json.Unmarshal(raw, &j); err != nil {
		return nil, fmt.Errorf("parse journal: %w", err)
	}
	if len(j.Entries) == 0 {
		return nil, fmt.Errorf("journal has no entries")
	}
	return &j, nil
}

func (p *Pool) ensureMigrationsTable(ctx context.Context) error {
	_, err := p.Exec(ctx, `
CREATE TABLE IF NOT EXISTS `+migrationsTable+` (
  id text PRIMARY KEY,
  applied_at timestamptz NOT NULL DEFAULT now()
)`)
	return err
}

func (p *Pool) appliedMigrations(ctx context.Context) (map[string]bool, error) {
	rows, err := p.Query(ctx, `SELECT id FROM `+migrationsTable)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

const drizzleBreakpoint = "--> statement-breakpoint"

// stripBreakpointMarkers removes drizzle `--> statement-breakpoint` separators.
func stripBreakpointMarkers(sql string) string {
	return strings.ReplaceAll(sql, drizzleBreakpoint, "\n")
}

// SplitSQLChunks splits a drizzle migration on statement-breakpoint markers.
// Files with no markers are a single chunk (simple-protocol multi-statement).
// DO $$ blocks stay intact because they are not split on inner semicolons.
func SplitSQLChunks(sql string) []string {
	raw := strings.Split(sql, drizzleBreakpoint)
	out := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part == "" || !sqlChunkHasWork(part) {
			continue
		}
		out = append(out, part)
	}
	return out
}

func sqlChunkHasWork(sql string) bool {
	for _, line := range strings.Split(sql, "\n") {
		t := strings.TrimSpace(line)
		if t != "" && !strings.HasPrefix(t, "--") {
			return true
		}
	}
	return false
}

func wrapPgError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return fmt.Errorf("%s: %s", pgErr.Code, pgErr.Message)
	}
	return err
}

func execSimple(ctx context.Context, pgConn *pgconn.PgConn, sql string) error {
	mrr := pgConn.Exec(ctx, sql)
	_, err := mrr.ReadAll()
	return wrapPgError(err)
}

// execScript runs a migration file via the simple protocol.
// Drizzle breakpoints are separate Query messages so later statements see
// catalog changes (ADD COLUMN, then CREATE INDEX on that column).
func (p *Pool) execScript(ctx context.Context, sql string) error {
	chunks := SplitSQLChunks(sql)
	if len(chunks) == 0 {
		return nil
	}
	conn, err := p.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	pgConn := conn.Conn().PgConn()
	if len(chunks) == 1 {
		return execSimple(ctx, pgConn, chunks[0])
	}
	if err := execSimple(ctx, pgConn, "BEGIN"); err != nil {
		return err
	}
	for _, chunk := range chunks {
		if err := execSimple(ctx, pgConn, chunk); err != nil {
			_ = execSimple(ctx, pgConn, "ROLLBACK")
			return err
		}
	}
	return execSimple(ctx, pgConn, "COMMIT")
}

// SplitSQLStatements splits SQL on drizzle breakpoints or semicolons (for tests).
func SplitSQLStatements(sql string) []string {
	sql = stripBreakpointMarkers(sql)
	parts := strings.Split(sql, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		lines := strings.Split(p, "\n")
		meaningful := false
		for _, line := range lines {
			t := strings.TrimSpace(line)
			if t != "" && !strings.HasPrefix(t, "--") {
				meaningful = true
				break
			}
		}
		if meaningful {
			out = append(out, p)
		}
	}
	return out
}
