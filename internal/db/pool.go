package db

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool wraps pgxpool for Majesta One kernel access.
type Pool struct {
	*pgxpool.Pool
}

// PoolOptions tunes pgx pool sizing. Zero values use defaults (Max=10, Min=1).
type PoolOptions struct {
	MaxConns int32
	MinConns int32
}

// DefaultPoolMaxConns is used when DB_MAX_CONNS is unset.
const DefaultPoolMaxConns int32 = 10

// DefaultPoolMinConns is used when DB_MIN_CONNS is unset.
const DefaultPoolMinConns int32 = 1

// Connect opens a pgx pool from DATABASE_URL using DB_MAX_CONNS / DB_MIN_CONNS env defaults.
func Connect(ctx context.Context, databaseURL string) (*Pool, error) {
	return ConnectWithOptions(ctx, databaseURL, PoolOptionsFromEnv())
}

// PoolOptionsFromEnv reads DB_MAX_CONNS / DB_MIN_CONNS (positive ints). Invalid or
// empty values fall back to DefaultPoolMaxConns / DefaultPoolMinConns.
func PoolOptionsFromEnv() PoolOptions {
	opts := PoolOptions{MaxConns: DefaultPoolMaxConns, MinConns: DefaultPoolMinConns}
	if v := os.Getenv("DB_MAX_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 10000 {
			opts.MaxConns = int32(n)
		}
	}
	if v := os.Getenv("DB_MIN_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 10000 {
			opts.MinConns = int32(n)
		}
	}
	if opts.MinConns > opts.MaxConns {
		opts.MinConns = opts.MaxConns
	}
	return opts
}

// ConnectWithOptions opens a pgx pool with explicit sizing.
func ConnectWithOptions(ctx context.Context, databaseURL string, opts PoolOptions) (*Pool, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	max := opts.MaxConns
	if max <= 0 {
		max = DefaultPoolMaxConns
	}
	min := opts.MinConns
	if min < 0 {
		min = DefaultPoolMinConns
	}
	if min > max {
		min = max
	}
	cfg.MaxConns = max
	cfg.MinConns = min
	cfg.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Pool{Pool: pool}, nil
}

// Close closes the pool.
func (p *Pool) Close() {
	if p != nil && p.Pool != nil {
		p.Pool.Close()
	}
}
