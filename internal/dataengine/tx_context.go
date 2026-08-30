package dataengine

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type ctxKey int

const (
	txCtxKey ctxKey = iota + 10
	syncDepthCtxKey
	syncOpsCtxKey
)

// dbTX is the subset of pgx.Tx / pool used by write paths.
type dbTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func withTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txCtxKey, tx)
}

func txFrom(ctx context.Context) pgx.Tx {
	tx, _ := ctx.Value(txCtxKey).(pgx.Tx)
	return tx
}

func (s *Service) querier(ctx context.Context) dbTX {
	if tx := txFrom(ctx); tx != nil {
		return tx
	}
	return s.pool
}

func syncDepth(ctx context.Context) int {
	d, _ := ctx.Value(syncDepthCtxKey).(int)
	return d
}

func withSyncDepth(ctx context.Context, d int) context.Context {
	return context.WithValue(ctx, syncDepthCtxKey, d)
}

type syncOpsCounter struct {
	n int
}

func syncOps(ctx context.Context) *syncOpsCounter {
	c, _ := ctx.Value(syncOpsCtxKey).(*syncOpsCounter)
	return c
}

func withSyncOps(ctx context.Context, c *syncOpsCounter) context.Context {
	return context.WithValue(ctx, syncOpsCtxKey, c)
}

func bumpSyncOps(ctx context.Context) error {
	c := syncOps(ctx)
	if c == nil {
		return nil
	}
	c.n++
	if c.n > maxSyncOps {
		return fmt.Errorf("sync automation exceeded max mutations (%d)", maxSyncOps)
	}
	return nil
}

// CountSyncMutation increments the sync automation mutation cap when a counter is present.
func CountSyncMutation(ctx context.Context) error {
	return bumpSyncOps(ctx)
}

// InWriteTx reports whether ctx already carries an open DataEngine write transaction.
func InWriteTx(ctx context.Context) bool {
	return txFrom(ctx) != nil
}

// EnsurePartitions attaches the physical records partition for objectAPIName (DDL; not in a write txn).
func (s *Service) EnsurePartitions(ctx context.Context, objectAPIName string) error {
	return s.ensureRecordPartitionsForWrite(ctx, objectAPIName, "create")
}

// RunInTx runs fn on the existing write transaction, or begins and commits one.
func (s *Service) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if txFrom(ctx) != nil {
		return fn(ctx)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(withTx(ctx, tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
