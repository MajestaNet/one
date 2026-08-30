package db_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/db"
)

func TestResolveMigrationsPath(t *testing.T) {
	dir, err := db.ResolveMigrationsPath()
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(dir, "meta", "_journal.json")
	if _, err := os.Stat(journal); err != nil {
		t.Fatalf("journal missing at %s: %v", journal, err)
	}
	if !strings.Contains(dir, "migrations") {
		t.Fatalf("unexpected path %s", dir)
	}
}

func TestResolveMigrationsPathRelativeEnvFromPackageCWD(t *testing.T) {
	t.Setenv("MIGRATIONS_PATH", "migrations")
	dir, err := db.ResolveMigrationsPath()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(dir) {
		t.Fatalf("expected absolute path, got %s", dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "meta", "_journal.json")); err != nil {
		t.Fatal(err)
	}
}

func TestSplitSQLStatements(t *testing.T) {
	sql := `
CREATE TABLE a (id int);
--> statement-breakpoint
CREATE TABLE b (id int);
`
	parts := db.SplitSQLStatements(sql)
	if len(parts) < 2 {
		t.Fatalf("got %d parts: %#v", len(parts), parts)
	}
}

func TestSplitSQLChunksKeepsDOBlocks(t *testing.T) {
	sql := `
ALTER TABLE t ADD COLUMN IF NOT EXISTS a int;
--> statement-breakpoint
DO $$
BEGIN
  PERFORM 1;
END $$;
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS t_a_idx ON t(a);
`
	chunks := db.SplitSQLChunks(sql)
	if len(chunks) != 3 {
		t.Fatalf("got %d chunks: %#v", len(chunks), chunks)
	}
	if !strings.Contains(chunks[1], "DO $$") || !strings.Contains(chunks[1], "END $$") {
		t.Fatalf("DO block was split: %#v", chunks[1])
	}
}

func TestLoadAndListMigrationFiles(t *testing.T) {
	dir, err := db.ResolveMigrationsPath()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var sqlFiles int
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			sqlFiles++
		}
	}
	if sqlFiles < 10 {
		t.Fatalf("expected >=10 sql migrations, got %d", sqlFiles)
	}
}

func TestJournalCoversAllSQLMigrations(t *testing.T) {
	dir, err := db.ResolveMigrationsPath()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "meta", "_journal.json"))
	if err != nil {
		t.Fatal(err)
	}
	var journal struct {
		Entries []struct {
			Tag string `json:"tag"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(raw, &journal); err != nil {
		t.Fatal(err)
	}
	tags := map[string]bool{}
	for _, e := range journal.Entries {
		tags[e.Tag] = true
		sqlPath := filepath.Join(dir, e.Tag+".sql")
		if _, err := os.Stat(sqlPath); err != nil {
			t.Errorf("journal tag %q missing SQL file: %v", e.Tag, err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		tag := strings.TrimSuffix(e.Name(), ".sql")
		if !tags[tag] {
			t.Errorf("SQL migration %s is not listed in meta/_journal.json", e.Name())
		}
	}
}
