package db

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MajestaNet/ide/internal/authz"
)

// RefreshTokenStore persists hashed refresh tokens (BP-063).
type RefreshTokenStore struct {
	pool *Pool
}

// NewRefreshTokenStore constructs a refresh token store.
func NewRefreshTokenStore(pool *Pool) *RefreshTokenStore {
	return &RefreshTokenStore{pool: pool}
}

var _ authz.RefreshRepository = (*RefreshTokenStore)(nil)

const refreshSelectCols = `id::text, family_id::text, user_id::text, azp, token_hash, COALESCE(device_id, ''), expires_at, family_expires_at, revoked_at, COALESCE(replaced_by::text, ''), created_at, last_used_at`

func scanRefreshToken(row pgx.Row) (*authz.RefreshToken, error) {
	var rec authz.RefreshToken
	var lastUsed *time.Time
	err := row.Scan(
		&rec.ID, &rec.FamilyID, &rec.UserID, &rec.Azp, &rec.TokenHash, &rec.DeviceID,
		&rec.ExpiresAt, &rec.FamilyExpiresAt, &rec.RevokedAt, &rec.ReplacedBy,
		&rec.CreatedAt, &lastUsed,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rec.LastUsedAt = lastUsed
	return &rec, nil
}

// Insert stores a new refresh token. Empty FamilyID lets Postgres generate one.
func (s *RefreshTokenStore) Insert(ctx context.Context, rec *authz.RefreshToken) error {
	if rec == nil {
		return ErrValidation
	}
	err := s.pool.QueryRow(ctx, `
INSERT INTO refresh_tokens (family_id, user_id, azp, token_hash, device_id, expires_at, family_expires_at)
VALUES (COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2::uuid, $3, $4, NULLIF($5, ''), $6, $7)
RETURNING id::text, family_id::text, created_at`,
		rec.FamilyID, rec.UserID, rec.Azp, rec.TokenHash, rec.DeviceID, rec.ExpiresAt, rec.FamilyExpiresAt,
	).Scan(&rec.ID, &rec.FamilyID, &rec.CreatedAt)
	return err
}

// GetByHash loads a row by SHA-256 hex digest.
func (s *RefreshTokenStore) GetByHash(ctx context.Context, hash string) (*authz.RefreshToken, error) {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return nil, ErrNotFound
	}
	row := s.pool.QueryRow(ctx, `SELECT `+refreshSelectCols+` FROM refresh_tokens WHERE token_hash = $1`, hash)
	return scanRefreshToken(row)
}

// RotatePresented atomically consumes a presented hash and inserts next.
// Reuse of a rotated token revokes the entire family.
func (s *RefreshTokenStore) RotatePresented(ctx context.Context, presentedHash string, next *authz.RefreshToken) (*authz.RefreshToken, error) {
	if next == nil || strings.TrimSpace(presentedHash) == "" {
		return nil, authz.ErrInvalidRefresh
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var rec authz.RefreshToken
	var lastUsed *time.Time
	var isActive bool
	var frozenAt *time.Time
	err = tx.QueryRow(ctx, `
SELECT rt.id::text, rt.family_id::text, rt.user_id::text, rt.azp, rt.token_hash, COALESCE(rt.device_id, ''),
       rt.expires_at, rt.family_expires_at, rt.revoked_at, COALESCE(rt.replaced_by::text, ''),
       rt.created_at, rt.last_used_at, u.is_active, u.frozen_at
FROM refresh_tokens rt
JOIN users u ON u.id = rt.user_id
WHERE rt.token_hash = $1
FOR UPDATE OF rt`, presentedHash).Scan(
		&rec.ID, &rec.FamilyID, &rec.UserID, &rec.Azp, &rec.TokenHash, &rec.DeviceID,
		&rec.ExpiresAt, &rec.FamilyExpiresAt, &rec.RevokedAt, &rec.ReplacedBy,
		&rec.CreatedAt, &lastUsed, &isActive, &frozenAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, authz.ErrInvalidRefresh
	}
	if err != nil {
		return nil, err
	}
	rec.LastUsedAt = lastUsed

	now := time.Now().UTC()
	if rec.RevokedAt != nil && rec.ReplacedBy != "" {
		if _, err := tx.Exec(ctx, `
UPDATE refresh_tokens SET revoked_at = now()
WHERE family_id = $1::uuid AND revoked_at IS NULL`, rec.FamilyID); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return nil, authz.ErrRefreshReuse
	}
	if rec.RevokedAt != nil || !rec.ExpiresAt.After(now) || !rec.FamilyExpiresAt.After(now) {
		return nil, authz.ErrInvalidRefresh
	}
	if !isActive || frozenAt != nil {
		return nil, authz.ErrInvalidRefresh
	}

	next.FamilyID = rec.FamilyID
	next.UserID = rec.UserID
	next.Azp = rec.Azp
	if next.DeviceID == "" {
		next.DeviceID = rec.DeviceID
	}
	next.FamilyExpiresAt = rec.FamilyExpiresAt
	if next.ExpiresAt.After(rec.FamilyExpiresAt) || next.ExpiresAt.IsZero() {
		next.ExpiresAt = rec.FamilyExpiresAt
	}

	err = tx.QueryRow(ctx, `
INSERT INTO refresh_tokens (family_id, user_id, azp, token_hash, device_id, expires_at, family_expires_at)
VALUES ($1::uuid, $2::uuid, $3, $4, NULLIF($5, ''), $6, $7)
RETURNING id::text, created_at`,
		next.FamilyID, next.UserID, next.Azp, next.TokenHash, next.DeviceID, next.ExpiresAt, next.FamilyExpiresAt,
	).Scan(&next.ID, &next.CreatedAt)
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
UPDATE refresh_tokens
SET revoked_at = now(), replaced_by = $2::uuid, last_used_at = now()
WHERE id = $1::uuid`, rec.ID, next.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return next, nil
}

// Revoke marks a single row revoked (does not follow replaced_by).
func (s *RefreshTokenStore) Revoke(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrValidation
	}
	_, err := s.pool.Exec(ctx, `
UPDATE refresh_tokens SET revoked_at = now()
WHERE id = $1::uuid AND revoked_at IS NULL`, id)
	return err
}

// RevokeFamily revokes every active token in the family.
func (s *RefreshTokenStore) RevokeFamily(ctx context.Context, familyID string) error {
	familyID = strings.TrimSpace(familyID)
	if familyID == "" {
		return ErrValidation
	}
	_, err := s.pool.Exec(ctx, `
UPDATE refresh_tokens SET revoked_at = now()
WHERE family_id = $1::uuid AND revoked_at IS NULL`, familyID)
	return err
}

// RevokeAllForUser revokes every active refresh token for the principal.
func (s *RefreshTokenStore) RevokeAllForUser(ctx context.Context, userID string) (int64, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return 0, ErrValidation
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE refresh_tokens SET revoked_at = now()
WHERE user_id = $1::uuid AND revoked_at IS NULL`, userID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
