package authz

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	AuthMethodOneJWT = "one_jwt"
	OneAudience      = "one"
)

// OneClaims are One-issued access token claims (ADR-006).
type OneClaims struct {
	PrincipalType string   `json:"principal_type,omitempty"`
	Scopes        []string `json:"scopes"`
	Roles         []string `json:"roles,omitempty"`
	Admin         bool     `json:"admin"`
	Azp           string   `json:"azp,omitempty"` // Connected App / client that minted the token
	jwt.RegisteredClaims
}

// OneSigner mints and verifies install-local Majesta One JWTs (HS256).
type OneSigner struct {
	SigningKey []byte
	Issuer     string
	TTL        time.Duration
}

// Enabled reports whether mint/verify is configured.
func (s *OneSigner) Enabled() bool {
	return s != nil && len(s.SigningKey) > 0 && s.Issuer != ""
}

// MintAccessToken issues a Majesta One access JWT for the actor.
// Azp is taken from actor.Azp when set (Connected App apiName or client id).
func (s *OneSigner) MintAccessToken(actor *Actor) (token string, expiresIn int64, err error) {
	if !s.Enabled() {
		return "", 0, fmt.Errorf("one jwt signer not configured")
	}
	if actor == nil || actor.ID == "" {
		return "", 0, fmt.Errorf("actor required")
	}
	if len(actor.Scopes) == 0 {
		return "", 0, fmt.Errorf("actor requires at least one API scope")
	}
	switch actor.PrincipalType {
	case "user", "service", "agent":
	default:
		return "", 0, fmt.Errorf("invalid principal type %q", actor.PrincipalType)
	}
	ttl := s.TTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	now := time.Now()
	exp := now.Add(ttl)
	scopeStrs := make([]string, 0, len(actor.Scopes))
	for _, sc := range actor.Scopes {
		scopeStrs = append(scopeStrs, string(sc))
	}
	claims := OneClaims{
		PrincipalType: actor.PrincipalType,
		Scopes:        scopeStrs,
		Roles:         append([]string(nil), actor.Roles...),
		Admin:         actor.IsAdmin,
		Azp:           actor.Azp,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.Issuer,
			Subject:   actor.ID,
			Audience:  jwt.ClaimStrings{OneAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(s.SigningKey)
	if err != nil {
		return "", 0, err
	}
	return signed, int64(ttl.Seconds()), nil
}

// Verify parses and validates a Majesta One access JWT.
func (s *OneSigner) Verify(token string) (*OneClaims, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("one jwt signer not configured")
	}
	parsed, err := jwt.ParseWithClaims(token, &OneClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return s.SigningKey, nil
	}, jwt.WithIssuer(s.Issuer), jwt.WithAudience(OneAudience), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*OneClaims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("invalid one jwt")
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("missing sub")
	}
	if claims.IssuedAt == nil {
		return nil, fmt.Errorf("missing iat")
	}
	switch claims.PrincipalType {
	case "user", "service", "agent":
	default:
		return nil, fmt.Errorf("invalid principal_type")
	}
	return claims, nil
}

// ScopesFromClaims converts claim scope strings to Scope values (ignores admin).
func ScopesFromClaims(raw []string) ([]Scope, bool) {
	seen := map[Scope]struct{}{}
	var scopes []Scope
	admin := false
	for _, c := range raw {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if c == "admin" {
			admin = true
			continue
		}
		s := Scope(c)
		switch s {
		case ScopeClient, ScopeMetadata, ScopeDeploy, ScopeOps:
			if _, ok := seen[s]; !ok {
				seen[s] = struct{}{}
				scopes = append(scopes, s)
			}
		}
	}
	return scopes, admin
}
