package authz

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

const (
	GrantClientCredentials = "client_credentials"
	GrantAuthorizationCode = "authorization_code"
	GrantPassword          = "password"
	GrantRefreshToken      = "refresh_token"
	GrantTokenExchange     = "token_exchange"

	// ScopeOfflineAccess is the OAuth scope that opts a public Connected App into refresh tokens.
	ScopeOfflineAccess = "offline_access"

	DefaultRefreshIdleSeconds = 2592000 // 30d
	DefaultRefreshAbsSeconds  = 7776000 // 90d
	DefaultRefreshBytes       = 32
)

var (
	// ErrInvalidRefresh is a generic refresh failure (missing, expired, frozen, azp mismatch).
	ErrInvalidRefresh = errors.New("invalid grant")
	// ErrRefreshReuse is returned when a rotated refresh token is presented (theft signal).
	ErrRefreshReuse = errors.New("refresh token reuse")
)

// RefreshToken is a hashed kernel refresh-token row (never plaintext).
type RefreshToken struct {
	ID              string
	FamilyID        string
	UserID          string
	Azp             string
	TokenHash       string
	DeviceID        string
	ExpiresAt       time.Time
	FamilyExpiresAt time.Time
	RevokedAt       *time.Time
	ReplacedBy      string
	CreatedAt       time.Time
	LastUsedAt      *time.Time
}

// IssuedRefresh is the one-time plaintext plus persisted metadata.
type IssuedRefresh struct {
	Raw       string
	ExpiresIn int64
	Token     RefreshToken
}

// RefreshRepository persists hashed refresh tokens.
type RefreshRepository interface {
	Insert(ctx context.Context, rec *RefreshToken) error
	GetByHash(ctx context.Context, hash string) (*RefreshToken, error)
	RotatePresented(ctx context.Context, presentedHash string, next *RefreshToken) (*RefreshToken, error)
	Revoke(ctx context.Context, id string) error
	RevokeFamily(ctx context.Context, familyID string) error
	RevokeAllForUser(ctx context.Context, userID string) (int64, error)
}

// IssueRefreshOpts configures a new refresh family.
type IssueRefreshOpts struct {
	UserID   string
	Azp      string
	DeviceID string
	IdleTTL  time.Duration
	AbsTTL   time.Duration
	Bytes    int
}

// ShouldIssueRefresh reports whether this mint should include an opaque refresh token.
// Public Connected Apps (including one.controlIde) need offline_access. Generic install
// sessions (azp=one.install on password / token_exchange) still receive refresh.
func ShouldIssueRefresh(azp, grant string, requestedScopes []string, clientKind string) bool {
	grant = strings.TrimSpace(grant)
	if grant == GrantClientCredentials || grant == GrantRefreshToken {
		return false
	}
	if strings.TrimSpace(azp) == InstallAzp && installSessionGrant(grant) {
		return true
	}
	if !strings.EqualFold(strings.TrimSpace(clientKind), "public") {
		return false
	}
	if !containsOfflineAccess(requestedScopes) {
		return false
	}
	switch grant {
	case GrantAuthorizationCode, GrantPassword, GrantTokenExchange,
		"urn:ietf:params:oauth:grant-type:token-exchange":
		return true
	default:
		return false
	}
}

func installSessionGrant(grant string) bool {
	switch grant {
	case GrantPassword, GrantTokenExchange, "urn:ietf:params:oauth:grant-type:token-exchange":
		return true
	default:
		return false
	}
}

func containsOfflineAccess(scopes []string) bool {
	for _, s := range scopes {
		for _, part := range strings.Fields(s) {
			if strings.EqualFold(part, ScopeOfflineAccess) {
				return true
			}
		}
	}
	return false
}

// HashRefreshToken returns hex(SHA-256(raw opaque token)).
func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// EqualRefreshHash compares two SHA-256 hex digests in constant time.
func EqualRefreshHash(stored, computed string) bool {
	if len(stored) != len(computed) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(stored), []byte(computed)) == 1
}

// GenerateRefreshToken returns a base64url (no pad) token and its hash.
func GenerateRefreshToken(n int) (raw, hash string, err error) {
	if n < 16 {
		n = DefaultRefreshBytes
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashRefreshToken(raw), nil
}

// RefreshRateLimitKey is the auth-token limiter key for a presented refresh token.
func RefreshRateLimitKey(raw string) string {
	h := HashRefreshToken(raw)
	if len(h) < 16 {
		return "refresh:unknown"
	}
	return "refresh:" + h[:16]
}

// ClampRefreshIdle returns now+idle, never past familyExpiresAt.
func ClampRefreshIdle(now time.Time, idleTTL time.Duration, familyExpiresAt time.Time) time.Time {
	idle := now.Add(idleTTL)
	if idle.After(familyExpiresAt) {
		return familyExpiresAt
	}
	return idle
}

// IssueRefreshToken creates a new refresh family and returns the plaintext once.
func IssueRefreshToken(ctx context.Context, store RefreshRepository, opts IssueRefreshOpts) (*IssuedRefresh, error) {
	if store == nil {
		return nil, ErrInvalidRefresh
	}
	if strings.TrimSpace(opts.UserID) == "" || strings.TrimSpace(opts.Azp) == "" {
		return nil, ErrInvalidRefresh
	}
	idle := opts.IdleTTL
	if idle <= 0 {
		idle = time.Duration(DefaultRefreshIdleSeconds) * time.Second
	}
	abs := opts.AbsTTL
	if abs <= 0 {
		abs = time.Duration(DefaultRefreshAbsSeconds) * time.Second
	}
	raw, hash, err := GenerateRefreshToken(opts.Bytes)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	familyExp := now.Add(abs)
	rec := &RefreshToken{
		UserID:          opts.UserID,
		Azp:             strings.TrimSpace(opts.Azp),
		TokenHash:       hash,
		DeviceID:        strings.TrimSpace(opts.DeviceID),
		ExpiresAt:       ClampRefreshIdle(now, idle, familyExp),
		FamilyExpiresAt: familyExp,
	}
	if err := store.Insert(ctx, rec); err != nil {
		return nil, err
	}
	return &IssuedRefresh{
		Raw:       raw,
		ExpiresIn: refreshExpiresIn(now, rec.ExpiresAt),
		Token:     *rec,
	}, nil
}

// RotateRefreshToken consumes the presented token, issues a sibling, and revokes the old row.
func RotateRefreshToken(ctx context.Context, store RefreshRepository, raw, clientID string, idleTTL time.Duration, bytes int) (*IssuedRefresh, error) {
	if store == nil {
		return nil, ErrInvalidRefresh
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrInvalidRefresh
	}
	hash := HashRefreshToken(raw)
	presented, err := store.GetByHash(ctx, hash)
	if err != nil {
		return nil, ErrInvalidRefresh
	}
	if presented == nil || !EqualRefreshHash(presented.TokenHash, hash) {
		return nil, ErrInvalidRefresh
	}
	if cid := strings.TrimSpace(clientID); cid != "" && cid != presented.Azp {
		return nil, ErrInvalidRefresh
	}
	if idleTTL <= 0 {
		idleTTL = time.Duration(DefaultRefreshIdleSeconds) * time.Second
	}
	nextRaw, nextHash, err := GenerateRefreshToken(bytes)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	next := &RefreshToken{
		FamilyID:        presented.FamilyID,
		UserID:          presented.UserID,
		Azp:             presented.Azp,
		TokenHash:       nextHash,
		DeviceID:        presented.DeviceID,
		ExpiresAt:       ClampRefreshIdle(now, idleTTL, presented.FamilyExpiresAt),
		FamilyExpiresAt: presented.FamilyExpiresAt,
	}
	saved, err := store.RotatePresented(ctx, hash, next)
	if err != nil {
		if errors.Is(err, ErrRefreshReuse) {
			return nil, ErrRefreshReuse
		}
		return nil, ErrInvalidRefresh
	}
	return &IssuedRefresh{
		Raw:       nextRaw,
		ExpiresIn: refreshExpiresIn(now, saved.ExpiresAt),
		Token:     *saved,
	}, nil
}

func refreshExpiresIn(now, expiresAt time.Time) int64 {
	sec := int64(expiresAt.Sub(now).Seconds())
	if sec < 0 {
		return 0
	}
	return sec
}
