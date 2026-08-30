package authz

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// OIDCVerifier validates Cognito/OIDC JWTs via JWKS.
type OIDCVerifier struct {
	Issuer   string
	Audience string
	JWKSURI  string
	Defaults []Scope

	mu         sync.Mutex
	jwks       keyfunc.Keyfunc
	localKey   jwt.Keyfunc // tests
	httpClient *http.Client
	configErr  error
}

// OIDCClaims are One-relevant JWT claims.
type OIDCClaims struct {
	Email             string          `json:"email"`
	Name              string          `json:"name"`
	PreferredUsername string          `json:"preferred_username"`
	OneScopes         json.RawMessage `json:"one_scopes"`
	CognitoGroups     []string        `json:"cognito:groups"`
	Groups            []string        `json:"groups"`
	CustomOneAdmin    any             `json:"custom:one_admin"`
	TokenUse          string          `json:"token_use"`
	EmailVerified     any             `json:"email_verified"`
	jwt.RegisteredClaims
}

// OIDCDiscovery is the subset of OpenID Provider Metadata used for exchange/JWKS.
type OIDCDiscovery struct {
	Issuer        string
	Authorization string
	Token         string
	JWKSURI       string
}

const oidcDiscoveryBodyLimit = 1 << 20

// NewOIDCVerifier constructs a verifier. jwksURI may be empty (resolved from
// OpenID discovery `jwks_uri`, never guessed as ${issuer}/.well-known/jwks.json).
func NewOIDCVerifier(issuer, audience, jwksURI string, defaults []Scope) *OIDCVerifier {
	if issuer == "" {
		return &OIDCVerifier{}
	}
	jwksURI = strings.TrimSpace(jwksURI)
	if len(defaults) == 0 {
		defaults = []Scope{ScopeClient}
	}
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	transport := base.Clone()
	transport.Proxy = nil
	v := &OIDCVerifier{
		Issuer:   issuer,
		Audience: audience,
		JWKSURI:  jwksURI,
		Defaults: defaults,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   15 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	if err := validateOIDCRemoteURL("issuer", issuer); err != nil {
		v.configErr = err
	} else if jwksURI != "" {
		if err := validateOIDCRemoteURL("JWKS URI", jwksURI); err != nil {
			v.configErr = err
		}
	}
	return v
}

// Enabled reports whether OIDC is configured.
func (v *OIDCVerifier) Enabled() bool {
	return v != nil && v.Issuer != ""
}

// SetLocalKey injects a jwt.Keyfunc for tests (skips remote JWKS).
func (v *OIDCVerifier) SetLocalKey(fn jwt.Keyfunc) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.localKey = fn
}

func (v *OIDCVerifier) keyfunc(ctx context.Context) (jwt.Keyfunc, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.localKey != nil {
		return v.localKey, nil
	}
	if v.jwks != nil {
		return v.jwks.Keyfunc, nil
	}
	jwksURI := strings.TrimSpace(v.JWKSURI)
	if jwksURI == "" {
		disc, err := DiscoverOIDC(ctx, v.httpClient, v.Issuer)
		if err != nil {
			return nil, fmt.Errorf("oidc discovery: %w", err)
		}
		jwksURI = strings.TrimSpace(disc.JWKSURI)
		if jwksURI == "" {
			return nil, fmt.Errorf("OIDC discovery missing jwks_uri")
		}
		if err := validateOIDCRemoteURL("JWKS URI", jwksURI); err != nil {
			return nil, err
		}
		v.JWKSURI = jwksURI
	}
	k, err := keyfunc.NewDefaultOverrideCtx(context.Background(), []string{jwksURI}, keyfunc.Override{
		Client: v.httpClient,
	})
	if err != nil {
		return nil, err
	}
	v.jwks = k
	return k.Keyfunc, nil
}

// Verify parses and validates a JWT, returning Majesta One claims.
// Audience is always required when OIDC is enabled.
func (v *OIDCVerifier) Verify(ctx context.Context, token string) (*OIDCClaims, error) {
	return v.verify(ctx, token, "")
}

// VerifyIDToken is Verify plus ID-token token_use rules for token exchange:
// missing token_use is accepted (Entra/Okta/Slack); token_use=access is rejected.
func (v *OIDCVerifier) VerifyIDToken(ctx context.Context, token string) (*OIDCClaims, error) {
	return v.verify(ctx, token, "id")
}

func (v *OIDCVerifier) verify(ctx context.Context, token, requireTokenUse string) (*OIDCClaims, error) {
	if !v.Enabled() {
		return nil, fmt.Errorf("OIDC is not configured")
	}
	if v.Audience == "" {
		return nil, fmt.Errorf("OIDC audience is required")
	}
	if v.configErr != nil {
		return nil, v.configErr
	}
	kf, err := v.keyfunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("jwks: %w", err)
	}

	claims := &OIDCClaims{}
	opts := []jwt.ParserOption{
		jwt.WithIssuer(v.Issuer),
		jwt.WithAudience(v.Audience),
		jwt.WithValidMethods([]string{"RS256", "ES256"}),
		jwt.WithExpirationRequired(),
	}

	parsed, err := jwt.ParseWithClaims(token, claims, kf, opts...)
	if err != nil {
		return nil, fmt.Errorf("invalid OIDC token: %w", err)
	}
	if !parsed.Valid {
		return nil, fmt.Errorf("invalid OIDC token")
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("OIDC token missing sub")
	}
	if requireTokenUse != "" {
		use := strings.ToLower(strings.TrimSpace(claims.TokenUse))
		if use != "" && use != requireTokenUse {
			return nil, fmt.Errorf("OIDC token_use=%q rejected (expected %s)", use, requireTokenUse)
		}
	}
	return claims, nil
}

// EmailIsVerified reports whether email_verified is true/1 (Slack/OIDC JIT).
func (c *OIDCClaims) EmailIsVerified() bool {
	if c == nil {
		return false
	}
	switch t := c.EmailVerified.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true") || t == "1"
	default:
		return false
	}
}

// DiscoverOIDC fetches {issuer}/.well-known/openid-configuration.
// HTTPS required (HTTP loopback only); issuer in the document must match;
// body capped at 1 MiB; redirects are not followed.
func DiscoverOIDC(ctx context.Context, client *http.Client, issuer string) (*OIDCDiscovery, error) {
	issuer = strings.TrimSpace(issuer)
	if err := validateOIDCRemoteURL("issuer", issuer); err != nil {
		return nil, err
	}
	if client == nil {
		client = oidcDiscoveryHTTPClient()
	}
	discURL := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc discovery status %d", resp.StatusCode)
	}
	var doc struct {
		Issuer string `json:"issuer"`
		Auth   string `json:"authorization_endpoint"`
		Token  string `json:"token_endpoint"`
		JWKS   string `json:"jwks_uri"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, oidcDiscoveryBodyLimit)).Decode(&doc); err != nil {
		return nil, err
	}
	discoveredIssuer := strings.TrimRight(strings.TrimSpace(doc.Issuer), "/")
	configuredIssuer := strings.TrimRight(strings.TrimSpace(issuer), "/")
	if discoveredIssuer != "" && discoveredIssuer != configuredIssuer {
		return nil, fmt.Errorf("OIDC discovery issuer mismatch")
	}
	out := &OIDCDiscovery{
		Issuer:        issuer,
		Authorization: strings.TrimSpace(doc.Auth),
		Token:         strings.TrimSpace(doc.Token),
		JWKSURI:       strings.TrimSpace(doc.JWKS),
	}
	if discoveredIssuer != "" {
		out.Issuer = discoveredIssuer
	}
	return out, nil
}

func oidcDiscoveryHTTPClient() *http.Client {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	transport := base.Clone()
	transport.Proxy = nil
	return &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func validateOIDCRemoteURL(name, raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || u.Host == "" || u.User != nil {
		return fmt.Errorf("OIDC %s must be an absolute HTTPS URL", name)
	}
	if u.Scheme == "https" || (u.Scheme == "http" && oidcRemoteLoopbackHost(u.Hostname())) {
		return nil
	}
	return fmt.Errorf("OIDC %s must use HTTPS (HTTP is allowed only for loopback development)", name)
}

func oidcRemoteLoopbackHost(host string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ResolveOIDC verifies the token and maps claims to an Actor.
func (r *Resolver) ResolveOIDC(token string) (*Actor, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	claims, err := r.OIDC.Verify(ctx, token)
	if err != nil {
		return nil, err
	}
	actor := actorFromOIDCClaims(claims, r.OIDC.Defaults)
	if r.Users == nil {
		return actor, nil
	}
	u, err := r.Users.EnsureOIDCUser(
		ctx,
		actor.ID,
		claims.Subject,
		actor.Email,
		actor.DisplayName,
		r.AutoProvision,
	)
	if err != nil {
		return nil, err
	}
	if !u.IsActive {
		return nil, fmt.Errorf("user is inactive")
	}
	if u.FrozenAt != nil {
		return nil, fmt.Errorf("user is frozen")
	}
	actor.ID = u.ID
	actor.Email = u.Email
	actor.DisplayName = u.DisplayName
	// Effective admin/scopes always from DB Roles (never IdP claims) — ADR-006.
	actor.IsAdmin = u.IsAdmin
	actor.OIDCSub = claims.Subject
	if u.PrincipalType != "" {
		actor.PrincipalType = u.PrincipalType
	} else {
		actor.PrincipalType = "user"
	}
	ids, err := r.Users.ListPermissionSetIDs(ctx, u.ID)
	if err != nil {
		return nil, fmt.Errorf("OIDC permission sets: %w", err)
	}
	actor.PermissionSetIDs = ids
	roleScopes, roleAdmin, roleNames, err := r.Users.ListRoleGrants(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	if len(roleNames) == 0 {
		return nil, ErrPrincipalNoRole
	}
	actor.Scopes = roleScopes
	actor.IsAdmin = actor.IsAdmin || roleAdmin
	actor.Roles = roleNames
	return actor, nil
}

func actorFromOIDCClaims(claims *OIDCClaims, defaults []Scope) *Actor {
	scopes := resolveScopesFromOIDC(claims, defaults)
	email := claims.Email
	if email == "" {
		email = claims.PreferredUsername
	}
	// ADR-015: do not fabricate synthetic emails; identity key is sub / identity_links.
	display := claims.Name
	if display == "" && email != "" {
		display = strings.Split(email, "@")[0]
	}
	if display == "" {
		display = "User"
	}
	return &Actor{
		ID:            oidcUserID(claims.Subject),
		Email:         email,
		DisplayName:   display,
		PrincipalType: "user",
		Scopes:        scopes,
		IsAdmin:       false, // IdP admin/group claims are never trusted
		AuthMethod:    "oidc",
		OIDCSub:       claims.Subject,
	}
}

// resolveScopesFromOIDC maps transitional IdP scope hints for the no-DB path only.
// Admin elevation from groups / custom:one_admin is intentionally ignored.
func resolveScopesFromOIDC(claims *OIDCClaims, defaults []Scope) []Scope {
	groups := append([]string{}, claims.CognitoGroups...)
	groups = append(groups, claims.Groups...)
	for i := range groups {
		groups[i] = strings.ToLower(groups[i])
	}

	if scopes := parseOneScopesClaim(claims.OneScopes); len(scopes) > 0 {
		return scopes
	}
	var fromGroups []Scope
	for _, s := range AllScopes {
		if contains(groups, "one-"+string(s)) {
			fromGroups = append(fromGroups, s)
		}
	}
	if len(fromGroups) > 0 {
		return fromGroups
	}
	if len(defaults) == 0 {
		return []Scope{ScopeClient}
	}
	return append([]Scope(nil), defaults...)
}

func parseOneScopesClaim(raw json.RawMessage) []Scope {
	if len(raw) == 0 {
		return nil
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		scopes, _, _ := parseScopeList(asString)
		return scopes
	}
	var asArr []string
	if err := json.Unmarshal(raw, &asArr); err == nil {
		scopes, _, _ := parseScopeList(strings.Join(asArr, "+"))
		return scopes
	}
	return nil
}

func oidcUserID(sub string) string {
	sum := sha256.Sum256([]byte("one-oidc:" + sub))
	h := hex.EncodeToString(sum[:])
	// UUID-shaped id (version nibble fixed) for stable principals without DB yet.
	return fmt.Sprintf("%s-%s-4%s-a%s-%s", h[0:8], h[8:12], h[13:16], h[17:20], h[20:32])
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
