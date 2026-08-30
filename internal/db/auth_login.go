package db

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// AuthLoginState is ephemeral OAuth state for the Majesta One → IdP hop.
type AuthLoginState struct {
	Provider            string
	ClientID            string
	RedirectURI         string
	ClientState         string
	CodeChallenge       string
	CodeChallengeMethod string
	Nonce               string
	IDPCodeVerifier     string
	ExpiresAt           time.Time
}

// AuthAuthorizationCode is a one-time Majesta One auth code row (code stored hashed).
type AuthAuthorizationCode struct {
	UserID              string
	ClientID            string
	RedirectURI         string
	CodeChallenge       string
	CodeChallengeMethod string
	Azp                 string
	IdentityProvider    string
	IdentitySubject     string
	ExpiresAt           time.Time
	UsedAt              *time.Time
}

// AuthLoginStore persists social-broker OAuth state and authorization codes.
type AuthLoginStore struct {
	pool *Pool
}

func NewAuthLoginStore(pool *Pool) *AuthLoginStore {
	return &AuthLoginStore{pool: pool}
}

// HashOpaqueToken SHA-256 hex-encodes opaque OAuth values before persistence.
func HashOpaqueToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// PutLoginState inserts a single-use login state (keyed by hashed state).
func (s *AuthLoginStore) PutLoginState(ctx context.Context, rawState string, st AuthLoginState, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	st.ExpiresAt = time.Now().UTC().Add(ttl)
	_, err := s.pool.Exec(ctx, `
INSERT INTO auth_login_states (
  state_hash, provider, client_id, redirect_uri, client_state,
  code_challenge, code_challenge_method, nonce, idp_code_verifier, expires_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		HashOpaqueToken(rawState),
		st.Provider, st.ClientID, st.RedirectURI, st.ClientState,
		st.CodeChallenge, st.CodeChallengeMethod, st.Nonce, st.IDPCodeVerifier, st.ExpiresAt,
	)
	return err
}

// TakeLoginState loads and deletes a login state (single-use). Returns ErrNotFound if missing/expired.
func (s *AuthLoginStore) TakeLoginState(ctx context.Context, rawState string) (*AuthLoginState, error) {
	row := s.pool.QueryRow(ctx, `
DELETE FROM auth_login_states
WHERE state_hash = $1 AND expires_at > now()
RETURNING provider, client_id, redirect_uri, client_state, code_challenge, code_challenge_method, nonce, idp_code_verifier, expires_at`,
		HashOpaqueToken(rawState))
	var st AuthLoginState
	err := row.Scan(
		&st.Provider, &st.ClientID, &st.RedirectURI, &st.ClientState,
		&st.CodeChallenge, &st.CodeChallengeMethod, &st.Nonce, &st.IDPCodeVerifier, &st.ExpiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// PutAuthorizationCode stores a hashed one-time Majesta One auth code.
func (s *AuthLoginStore) PutAuthorizationCode(ctx context.Context, rawCode string, code AuthAuthorizationCode, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	code.ExpiresAt = time.Now().UTC().Add(ttl)
	_, err := s.pool.Exec(ctx, `
INSERT INTO auth_authorization_codes (
  code_hash, user_id, client_id, redirect_uri, code_challenge, code_challenge_method,
  azp, identity_provider, identity_subject, expires_at
) VALUES ($1,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10)`,
		HashOpaqueToken(rawCode),
		code.UserID, code.ClientID, code.RedirectURI, code.CodeChallenge, code.CodeChallengeMethod,
		code.Azp, code.IdentityProvider, code.IdentitySubject, code.ExpiresAt,
	)
	return err
}

// ConsumeAuthorizationCode validates PKCE and marks the code used atomically.
func (s *AuthLoginStore) ConsumeAuthorizationCode(ctx context.Context, rawCode, clientID, redirectURI, codeVerifier string) (*AuthAuthorizationCode, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var c AuthAuthorizationCode
	var usedAt *time.Time
	err = tx.QueryRow(ctx, `
SELECT user_id::text, client_id, redirect_uri, code_challenge, code_challenge_method,
       azp, identity_provider, identity_subject, expires_at, used_at
FROM auth_authorization_codes
WHERE code_hash = $1
FOR UPDATE`, HashOpaqueToken(rawCode)).Scan(
		&c.UserID, &c.ClientID, &c.RedirectURI, &c.CodeChallenge, &c.CodeChallengeMethod,
		&c.Azp, &c.IdentityProvider, &c.IdentitySubject, &c.ExpiresAt, &usedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if usedAt != nil {
		return nil, fmt.Errorf("%w: authorization code already used", ErrConflict)
	}
	if time.Now().UTC().After(c.ExpiresAt) {
		return nil, fmt.Errorf("%w: authorization code expired", ErrNotFound)
	}
	if c.ClientID != clientID {
		return nil, fmt.Errorf("%w: client_id mismatch", ErrValidation)
	}
	if c.RedirectURI != redirectURI {
		return nil, fmt.Errorf("%w: redirect_uri mismatch", ErrValidation)
	}
	if c.CodeChallengeMethod != "S256" {
		return nil, fmt.Errorf("%w: unsupported code_challenge_method", ErrValidation)
	}
	if !VerifyPKCES256(codeVerifier, c.CodeChallenge) {
		return nil, fmt.Errorf("%w: invalid code_verifier", ErrValidation)
	}
	tag, err := tx.Exec(ctx, `
UPDATE auth_authorization_codes SET used_at = now()
WHERE code_hash = $1 AND used_at IS NULL`, HashOpaqueToken(rawCode))
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() != 1 {
		return nil, fmt.Errorf("%w: authorization code already used", ErrConflict)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	c.UsedAt = usedAt
	return &c, nil
}

// VerifyPKCES256 checks S256(code_verifier) equals the stored challenge (base64url, no pad).
func VerifyPKCES256(verifier, challenge string) bool {
	if verifier == "" || challenge == "" {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	enc := base64.RawURLEncoding.EncodeToString(sum[:])
	return enc == strings.TrimRight(challenge, "=")
}
