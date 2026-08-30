package deploy

import (
	"context"
	"fmt"

	"github.com/MajestaNet/ide/internal/db"
)

// UpsertCustomerSources persists pack Sources (src/automations + tests/automations) on the install.
func UpsertCustomerSources(ctx context.Context, pool *db.Pool, sources map[string]string, dryRun bool) error {
	if pool == nil || len(sources) == 0 {
		return nil
	}
	if dryRun {
		return nil
	}
	for path, body := range sources {
		if path == "" {
			continue
		}
		if _, err := pool.Exec(ctx, `
INSERT INTO customer_source_files (path, body, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (path) DO UPDATE SET body = EXCLUDED.body, updated_at = now()`, path, body); err != nil {
			return fmt.Errorf("upsert source %s: %w", path, err)
		}
	}
	return nil
}

// LoadCustomerSources returns all persisted guest source files.
func LoadCustomerSources(ctx context.Context, pool *db.Pool) (map[string]string, error) {
	out := map[string]string{}
	if pool == nil {
		return out, nil
	}
	rows, err := pool.Query(ctx, `SELECT path, body FROM customer_source_files ORDER BY path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var path, body string
		if err := rows.Scan(&path, &body); err != nil {
			return nil, err
		}
		out[path] = body
	}
	return out, rows.Err()
}

// LoadCustomerSource returns one source file body.
func LoadCustomerSource(ctx context.Context, pool *db.Pool, path string) (string, error) {
	if pool == nil || path == "" {
		return "", fmt.Errorf("source not found: %s", path)
	}
	var body string
	err := pool.QueryRow(ctx, `SELECT body FROM customer_source_files WHERE path=$1`, path).Scan(&body)
	if err != nil {
		return "", fmt.Errorf("source not found: %s", path)
	}
	return body, nil
}
