package db

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/MajestaNet/ide/internal/authz"
)

// CredentialStore persists principal_credentials.
type CredentialStore struct {
	pool *Pool
}

// NewCredentialStore constructs a credential store.
func NewCredentialStore(pool *Pool) *CredentialStore {
	return &CredentialStore{pool: pool}
}

// Credential is a principal_credentials row.
type Credential struct {
	ID             string
	UserID         string
	CredentialKind string
	SecretHash     string
	Label          *string
	ExpiresAt      *time.Time
	RevokedAt      *time.Time
	CreatedAt      time.Time
}

// CreateClientSecret hashes and stores a client_secret for userID. Returns plaintext once.
func (s *CredentialStore) CreateClientSecret(ctx context.Context, userID, plaintext, label string) (*Credential, error) {
	return s.createHashed(ctx, userID, "client_secret", plaintext, label)
}

// CreatePassword hashes and stores a password credential for a human principal.
func (s *CredentialStore) CreatePassword(ctx context.Context, userID, plaintext string) (*Credential, error) {
	plaintext = strings.TrimSpace(plaintext)
	if len(plaintext) < 10 {
		return nil, fmt.Errorf("%w: password must be at least 10 characters", ErrValidation)
	}
	return s.createHashed(ctx, userID, "password", plaintext, "password")
}

// SetPassword revokes all active password credentials for userID and stores a new hash.
func (s *CredentialStore) SetPassword(ctx context.Context, userID, plaintext string) (*Credential, error) {
	plaintext = strings.TrimSpace(plaintext)
	if len(plaintext) < 10 {
		return nil, fmt.Errorf("%w: password must be at least 10 characters", ErrValidation)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
UPDATE principal_credentials
SET revoked_at = now()
WHERE user_id = $1::uuid
  AND credential_kind = 'password'
  AND revoked_at IS NULL`, userID)
	if err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	var id string
	err = tx.QueryRow(ctx, `
INSERT INTO principal_credentials (user_id, credential_kind, secret_hash, label)
VALUES ($1::uuid, 'password', $2, 'password')
RETURNING id::text`, userID, string(hash)).Scan(&id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// RevokeActivePasswords marks all active password credentials for userID as revoked.
func (s *CredentialStore) RevokeActivePasswords(ctx context.Context, userID string) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
UPDATE principal_credentials
SET revoked_at = now()
WHERE user_id = $1::uuid
  AND credential_kind = 'password'
  AND revoked_at IS NULL`, userID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *CredentialStore) createHashed(ctx context.Context, userID, kind, plaintext, label string) (*Credential, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	var id string
	err = s.pool.QueryRow(ctx, `
INSERT INTO principal_credentials (user_id, credential_kind, secret_hash, label)
VALUES ($1::uuid, $2, $3, NULLIF($4, ''))
RETURNING id::text`, userID, kind, string(hash), label).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// VerifyPassword checks plaintext against any active password credential for userID.
func (s *CredentialStore) VerifyPassword(ctx context.Context, userID, plaintext string) (bool, error) {
	creds, err := s.ListActiveByUserID(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, c := range creds {
		if c.CredentialKind != "password" {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(c.SecretHash), []byte(plaintext)) == nil {
			return true, nil
		}
	}
	return false, nil
}

// GenerateClientSecret creates a random client_secret, stores the hash, and returns the credential plus plaintext once.
func (s *CredentialStore) GenerateClientSecret(ctx context.Context, userID, label string) (cred *Credential, plaintext string, err error) {
	plaintext, err = generateSecret()
	if err != nil {
		return nil, "", err
	}
	cred, err = s.CreateClientSecret(ctx, userID, plaintext, label)
	if err != nil {
		return nil, "", err
	}
	return cred, plaintext, nil
}

func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// CredentialMeta is a credential row without the secret hash (safe for Metadata list).
type CredentialMeta struct {
	ID             string
	UserID         string
	CredentialKind string
	Label          *string
	ExpiresAt      *time.Time
	RevokedAt      *time.Time
	CreatedAt      time.Time
}

// ListMetaByUserID returns credential metadata (no hashes) for a principal.
func (s *CredentialStore) ListMetaByUserID(ctx context.Context, userID string) ([]CredentialMeta, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id::text, user_id::text, credential_kind, label, expires_at, revoked_at, created_at
FROM principal_credentials
WHERE user_id = $1::uuid
ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CredentialMeta
	for rows.Next() {
		var c CredentialMeta
		if err := rows.Scan(&c.ID, &c.UserID, &c.CredentialKind, &c.Label, &c.ExpiresAt, &c.RevokedAt, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Revoke sets revoked_at on a credential owned by userID.
func (s *CredentialStore) Revoke(ctx context.Context, userID, credentialID string) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE principal_credentials
SET revoked_at = now()
WHERE id = $1::uuid AND user_id = $2::uuid AND revoked_at IS NULL`, credentialID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeAllForUser revokes every active credential for a principal.
func (s *CredentialStore) RevokeAllForUser(ctx context.Context, userID string) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
UPDATE principal_credentials
SET revoked_at = now()
WHERE user_id = $1::uuid AND revoked_at IS NULL`, userID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// GetByID loads a credential by id.
func (s *CredentialStore) GetByID(ctx context.Context, id string) (*Credential, error) {
	row := s.pool.QueryRow(ctx, `
SELECT id::text, user_id::text, credential_kind, secret_hash, label, expires_at, revoked_at, created_at
FROM principal_credentials WHERE id = $1::uuid`, id)
	return scanCredential(row)
}

// ListActiveByUserID returns non-revoked, non-expired credentials for a user.
func (s *CredentialStore) ListActiveByUserID(ctx context.Context, userID string) ([]authz.CredentialRecord, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id::text, user_id::text, credential_kind, secret_hash, COALESCE(label, '')
FROM principal_credentials
WHERE user_id = $1::uuid
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > now())`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []authz.CredentialRecord
	for rows.Next() {
		var c authz.CredentialRecord
		if err := rows.Scan(&c.ID, &c.UserID, &c.CredentialKind, &c.SecretHash, &c.Label); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// VerifyClientSecret checks plaintext against any active credential for userID.
func (s *CredentialStore) VerifyClientSecret(ctx context.Context, userID, plaintext string) (bool, error) {
	creds, err := s.ListActiveByUserID(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, c := range creds {
		if c.CredentialKind != "client_secret" && c.CredentialKind != "bootstrap_api_key" {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(c.SecretHash), []byte(plaintext)) == nil {
			return true, nil
		}
	}
	return false, nil
}

func scanCredential(row pgx.Row) (*Credential, error) {
	var c Credential
	err := row.Scan(&c.ID, &c.UserID, &c.CredentialKind, &c.SecretHash, &c.Label, &c.ExpiresAt, &c.RevokedAt, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// AuthzCredentials adapts CredentialStore to authz.CredentialRepository.
type AuthzCredentials struct {
	Store *CredentialStore
}

var _ authz.CredentialRepository = (*AuthzCredentials)(nil)

func (a *AuthzCredentials) ListActiveByUserID(ctx context.Context, userID string) ([]authz.CredentialRecord, error) {
	return a.Store.ListActiveByUserID(ctx, userID)
}
