package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// IdentityLink maps an external IdP subject to a Majesta One user.
type IdentityLink struct {
	ID        string
	UserID    string
	Provider  string
	Issuer    *string
	Subject   string
	CreatedAt time.Time
}

// IdentityLinkStore persists identity_links.
type IdentityLinkStore struct {
	pool *Pool
}

// NewIdentityLinkStore constructs a link store.
func NewIdentityLinkStore(pool *Pool) *IdentityLinkStore {
	return &IdentityLinkStore{pool: pool}
}

// Upsert creates or returns an identity link for provider+issuer+subject.
// An existing external identity can never be silently rebound to a different
// Majesta One principal.
func (s *IdentityLinkStore) Upsert(ctx context.Context, userID, provider, issuer, subject string) (*IdentityLink, error) {
	userID = strings.TrimSpace(userID)
	provider = strings.TrimSpace(provider)
	issuer = strings.TrimSpace(issuer)
	subject = strings.TrimSpace(subject)
	if userID == "" || provider == "" || subject == "" {
		return nil, fmt.Errorf("%w: userID, provider, and subject are required", ErrValidation)
	}
	var id string
	err := s.pool.QueryRow(ctx, `
INSERT INTO identity_links (user_id, provider, issuer, subject)
VALUES ($1::uuid, $2, $3, $4)
ON CONFLICT (provider, issuer, subject) DO NOTHING
RETURNING id::text`, userID, provider, issuer, subject).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		existing, getErr := s.GetBySubject(ctx, provider, issuer, subject)
		if getErr != nil {
			return nil, getErr
		}
		if existing.UserID != userID {
			return nil, fmt.Errorf("%w: external identity is already linked to another principal", ErrConflict)
		}
		return existing, nil
	}
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// GetByID loads a link.
func (s *IdentityLinkStore) GetByID(ctx context.Context, id string) (*IdentityLink, error) {
	row := s.pool.QueryRow(ctx, `
SELECT id::text, user_id::text, provider, issuer, subject, created_at
FROM identity_links WHERE id = $1::uuid`, id)
	return scanIdentityLink(row)
}

// GetBySubject finds a link by provider and subject (issuer optional).
func (s *IdentityLinkStore) GetBySubject(ctx context.Context, provider, issuer, subject string) (*IdentityLink, error) {
	var row pgx.Row
	if issuer != "" {
		row = s.pool.QueryRow(ctx, `
SELECT id::text, user_id::text, provider, issuer, subject, created_at
FROM identity_links
WHERE provider = $1 AND issuer = $2 AND subject = $3`, provider, issuer, subject)
	} else {
		row = s.pool.QueryRow(ctx, `
SELECT id::text, user_id::text, provider, issuer, subject, created_at
FROM identity_links
WHERE provider = $1 AND subject = $2
ORDER BY created_at ASC LIMIT 1`, provider, subject)
	}
	return scanIdentityLink(row)
}

// GetByIssuerSubject finds a link by issuer+subject across providers (IdP-agnostic exchange).
func (s *IdentityLinkStore) GetByIssuerSubject(ctx context.Context, issuer, subject string) (*IdentityLink, error) {
	issuer = strings.TrimSpace(issuer)
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return nil, ErrNotFound
	}
	var row pgx.Row
	if issuer != "" {
		row = s.pool.QueryRow(ctx, `
SELECT id::text, user_id::text, provider, issuer, subject, created_at
FROM identity_links
WHERE issuer = $1 AND subject = $2
ORDER BY created_at ASC LIMIT 1`, issuer, subject)
	} else {
		row = s.pool.QueryRow(ctx, `
SELECT id::text, user_id::text, provider, issuer, subject, created_at
FROM identity_links
WHERE subject = $1
ORDER BY created_at ASC LIMIT 1`, subject)
	}
	return scanIdentityLink(row)
}

// ListByUserID returns all links for a principal.
func (s *IdentityLinkStore) ListByUserID(ctx context.Context, userID string) ([]IdentityLink, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id::text, user_id::text, provider, issuer, subject, created_at
FROM identity_links WHERE user_id = $1::uuid ORDER BY created_at ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IdentityLink
	for rows.Next() {
		var l IdentityLink
		if err := rows.Scan(&l.ID, &l.UserID, &l.Provider, &l.Issuer, &l.Subject, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func scanIdentityLink(row pgx.Row) (*IdentityLink, error) {
	var l IdentityLink
	err := row.Scan(&l.ID, &l.UserID, &l.Provider, &l.Issuer, &l.Subject, &l.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}
