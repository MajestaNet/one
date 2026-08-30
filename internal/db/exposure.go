package db

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MajestaNet/ide/internal/edge"
)

// ExposurePolicyRow is the singleton install_exposure_policy row.
type ExposurePolicyRow struct {
	Policy    edge.Policy
	Status    string
	LastError *string
	UpdatedAt time.Time
	AppliedAt *time.Time
}

// ExposureStore persists install exposure desired state.
type ExposureStore struct {
	pool *Pool
}

// NewExposureStore constructs an exposure store.
func NewExposureStore(pool *Pool) *ExposureStore {
	return &ExposureStore{pool: pool}
}

// Get returns the singleton policy row, creating a default if missing.
func (s *ExposureStore) Get(ctx context.Context) (*ExposurePolicyRow, error) {
	row, err := s.scan(ctx)
	if err == nil {
		return row, nil
	}
	if err != ErrNotFound {
		return nil, err
	}
	def := edge.DefaultPolicy()
	if err := s.Put(ctx, def, edge.StatusApplied, nil); err != nil {
		return nil, err
	}
	return s.scan(ctx)
}

func (s *ExposureStore) scan(ctx context.Context) (*ExposurePolicyRow, error) {
	var raw []byte
	var status string
	var lastErr *string
	var updated time.Time
	var applied *time.Time
	err := s.pool.QueryRow(ctx, `
SELECT policy, status, last_error, updated_at, applied_at
FROM install_exposure_policy WHERE id = 1`).Scan(&raw, &status, &lastErr, &updated, &applied)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var p edge.Policy
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	normalizePolicy(&p)
	return &ExposurePolicyRow{
		Policy:    p,
		Status:    status,
		LastError: lastErr,
		UpdatedAt: updated,
		AppliedAt: applied,
	}, nil
}

func normalizePolicy(p *edge.Policy) {
	def := edge.DefaultPolicy()
	if p.ClientAccessMode == "" {
		p.ClientAccessMode = def.ClientAccessMode
	}
	for _, f := range edge.AllFamilies {
		fp := p.Get(f)
		if fp.Mode == "" {
			fp.Mode = def.Get(f).Mode
		}
		if fp.CIDRs == nil {
			fp.CIDRs = []string{}
		}
		p.Set(f, fp)
	}
}

// Put writes desired policy and status.
func (s *ExposureStore) Put(ctx context.Context, p edge.Policy, status string, lastErr *string) error {
	normalizePolicy(&p)
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
INSERT INTO install_exposure_policy (id, policy, status, last_error, updated_at, applied_at)
VALUES (1, $1::jsonb, $2, $3, now(), CASE WHEN $2 = 'applied' THEN now() ELSE NULL END)
ON CONFLICT (id) DO UPDATE SET
  policy = EXCLUDED.policy,
  status = EXCLUDED.status,
  last_error = EXCLUDED.last_error,
  updated_at = now(),
  applied_at = CASE WHEN EXCLUDED.status = 'applied' THEN now() ELSE install_exposure_policy.applied_at END`,
		string(raw), status, lastErr)
	return err
}

// MarkStatus updates status/error without changing policy JSON.
func (s *ExposureStore) MarkStatus(ctx context.Context, status string, lastErr *string) error {
	_, err := s.pool.Exec(ctx, `
UPDATE install_exposure_policy SET
  status = $1,
  last_error = $2,
  updated_at = now(),
  applied_at = CASE WHEN $1 = 'applied' THEN now() ELSE applied_at END
WHERE id = 1`, status, lastErr)
	return err
}
