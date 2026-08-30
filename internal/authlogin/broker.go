// Package authlogin implements the ADR-015 thin Go social login broker.
//
// Libraries: golang.org/x/oauth2 for authorization-code exchange;
// existing MicahParks/keyfunc + golang-jwt for ID-token JWKS verify
// (equivalent to coreos/go-oidc verification without a second JWT stack).
package authlogin

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"

	"github.com/MajestaNet/ide/internal/identity"
)

const (
	IssuerGoogle = "https://accounts.google.com"
	IssuerApple  = "https://appleid.apple.com"
	IssuerSlack  = "https://slack.com"
)

// SubjectClaims are verified IdP identity claims used for Majesta One linking.
type SubjectClaims struct {
	Provider      string
	Issuer        string
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Nonce         string
	Extra         map[string]string
}

// ClaimMap returns IdP claims for JIT field mapping (standard + Extra).
func (c *SubjectClaims) ClaimMap() map[string]string {
	out := map[string]string{}
	if c == nil {
		return out
	}
	if c.Email != "" {
		out["email"] = c.Email
	}
	if c.Name != "" {
		out["name"] = c.Name
	}
	if c.Subject != "" {
		out["sub"] = c.Subject
		out["subject"] = c.Subject
	}
	if c.Provider != "" {
		out["provider"] = c.Provider
	}
	for k, v := range c.Extra {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		out[k] = v
	}
	return out
}

// Provider exchanges an authorization code with an external IdP and returns verified claims.
type Provider interface {
	Name() string
	Configured() bool
	AuthCodeURL(state, nonce, codeChallenge, redirectURI string) (string, error)
	Exchange(ctx context.Context, code, codeVerifier, redirectURI, expectNonce string) (*SubjectClaims, error)
}

// Broker selects configured social providers.
type Broker struct {
	Providers map[string]Provider
	mu        sync.RWMutex
}

func (b *Broker) Get(name string) (Provider, bool) {
	if b == nil {
		return nil, false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	p, ok := b.Providers[strings.ToLower(strings.TrimSpace(name))]
	return p, ok && p != nil && p.Configured()
}

func (b *Broker) EnabledNames() []string {
	if b == nil {
		return nil
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]string, 0, len(b.Providers))
	for name, p := range b.Providers {
		if p != nil && p.Configured() {
			out = append(out, name)
		}
	}
	return out
}

// GoogleConfig configures Sign in with Google.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
}

// AppleConfig configures Sign in with Apple.
type AppleConfig struct {
	ClientID   string // Services ID
	TeamID     string
	KeyID      string
	PrivateKey string // PEM EC private key
}

// NewBroker builds Google/Apple/dev providers from config (nil entries omitted).
func NewBroker(googleCfg GoogleConfig, appleCfg AppleConfig, enableDev bool) *Broker {
	b := &Broker{Providers: map[string]Provider{}}
	if g := NewGoogleProvider(googleCfg); g.Configured() {
		b.Providers[identity.ProviderGoogle] = g
	}
	if a := NewAppleProvider(appleCfg); a.Configured() {
		b.Providers[identity.ProviderApple] = a
	}
	if enableDev {
		b.Providers[identity.ProviderDev] = NewDevProvider()
	}
	return b
}

const (
	IssuerDev      = "https://one.local/auth/dev"
	devLoginCode   = "one-dev-login"
	DevSubject     = "local-dev-user"
	DevEmail       = "dev@one.local"
	DevDisplayName = "Local Developer"
)

// DevProvider is a local-only IdP stand-in (no Google/Apple credentials required).
type DevProvider struct{}

func NewDevProvider() *DevProvider { return &DevProvider{} }

func (d *DevProvider) Name() string     { return identity.ProviderDev }
func (d *DevProvider) Configured() bool { return true }

func (d *DevProvider) AuthCodeURL(state, nonce, codeChallenge, redirectURI string) (string, error) {
	_ = nonce
	_ = codeChallenge
	u, err := url.Parse(redirectURI)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("code", devLoginCode)
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (d *DevProvider) Exchange(_ context.Context, code, _, _, expectNonce string) (*SubjectClaims, error) {
	if code != devLoginCode {
		return nil, errors.New("invalid dev login code")
	}
	return &SubjectClaims{
		Provider:      identity.ProviderDev,
		Issuer:        IssuerDev,
		Subject:       DevSubject,
		Email:         DevEmail,
		EmailVerified: true,
		Name:          DevDisplayName,
		Nonce:         expectNonce,
	}, nil
}

// --- Google ---

type GoogleProvider struct {
	cfg      GoogleConfig
	mu       sync.Mutex
	jwks     keyfunc.Keyfunc
	endpoint oauth2.Endpoint
}

func NewGoogleProvider(cfg GoogleConfig) *GoogleProvider {
	return &GoogleProvider{
		cfg: cfg,
		endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
	}
}

func (g *GoogleProvider) Name() string { return identity.ProviderGoogle }
func (g *GoogleProvider) Configured() bool {
	return g != nil && strings.TrimSpace(g.cfg.ClientID) != "" && strings.TrimSpace(g.cfg.ClientSecret) != ""
}

func (g *GoogleProvider) oauth(redirectURI string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     g.cfg.ClientID,
		ClientSecret: g.cfg.ClientSecret,
		RedirectURL:  redirectURI,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     g.endpoint,
	}
}

func (g *GoogleProvider) AuthCodeURL(state, nonce, codeChallenge, redirectURI string) (string, error) {
	if !g.Configured() {
		return "", errors.New("google login not configured")
	}
	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "select_account"),
	}
	return g.oauth(redirectURI).AuthCodeURL(state, opts...), nil
}

func (g *GoogleProvider) Exchange(ctx context.Context, code, codeVerifier, redirectURI, expectNonce string) (*SubjectClaims, error) {
	if !g.Configured() {
		return nil, errors.New("google login not configured")
	}
	tok, err := g.oauth(redirectURI).Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	)
	if err != nil {
		return nil, fmt.Errorf("google token exchange: %w", err)
	}
	raw, _ := tok.Extra("id_token").(string)
	if raw == "" {
		return nil, errors.New("google response missing id_token")
	}
	return verifyOIDCIDToken(ctx, &g.mu, &g.jwks,
		"https://www.googleapis.com/oauth2/v3/certs",
		[]string{IssuerGoogle, "accounts.google.com"},
		g.cfg.ClientID, raw, expectNonce, identity.ProviderGoogle)
}

// --- Apple ---

type AppleProvider struct {
	cfg  AppleConfig
	mu   sync.Mutex
	jwks keyfunc.Keyfunc
	key  *ecdsa.PrivateKey
}

func NewAppleProvider(cfg AppleConfig) *AppleProvider {
	return &AppleProvider{cfg: cfg}
}

func (a *AppleProvider) Name() string { return identity.ProviderApple }
func (a *AppleProvider) Configured() bool {
	return a != nil &&
		strings.TrimSpace(a.cfg.ClientID) != "" &&
		strings.TrimSpace(a.cfg.TeamID) != "" &&
		strings.TrimSpace(a.cfg.KeyID) != "" &&
		strings.TrimSpace(a.cfg.PrivateKey) != ""
}

func (a *AppleProvider) privateKey() (*ecdsa.PrivateKey, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.key != nil {
		return a.key, nil
	}
	block, _ := pem.Decode([]byte(a.cfg.PrivateKey))
	if block == nil {
		return nil, errors.New("apple private key: invalid PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Try EC private key PEM
		ec, err2 := x509.ParseECPrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("apple private key: %v / %w", err, err2)
		}
		a.key = ec
		return a.key, nil
	}
	ec, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("apple private key must be ECDSA")
	}
	a.key = ec
	return a.key, nil
}

func (a *AppleProvider) clientSecret() (string, error) {
	key, err := a.privateKey()
	if err != nil {
		return "", err
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": a.cfg.TeamID,
		"iat": now.Unix(),
		"exp": now.Add(5 * time.Minute).Unix(),
		"aud": "https://appleid.apple.com",
		"sub": a.cfg.ClientID,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	t.Header["kid"] = a.cfg.KeyID
	return t.SignedString(key)
}

func (a *AppleProvider) oauth(redirectURI, clientSecret string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     a.cfg.ClientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURI,
		Scopes:       []string{"openid", "name", "email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://appleid.apple.com/auth/authorize",
			TokenURL: "https://appleid.apple.com/auth/token",
		},
	}
}

func (a *AppleProvider) AuthCodeURL(state, nonce, codeChallenge, redirectURI string) (string, error) {
	if !a.Configured() {
		return "", errors.New("apple login not configured")
	}
	secret, err := a.clientSecret()
	if err != nil {
		return "", err
	}
	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("response_mode", "query"),
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("nonce", nonce),
	}
	_ = secret // secret used at token exchange; authorize is public
	return a.oauth(redirectURI, secret).AuthCodeURL(state, opts...), nil
}

func (a *AppleProvider) Exchange(ctx context.Context, code, codeVerifier, redirectURI, expectNonce string) (*SubjectClaims, error) {
	if !a.Configured() {
		return nil, errors.New("apple login not configured")
	}
	secret, err := a.clientSecret()
	if err != nil {
		return nil, err
	}
	tok, err := a.oauth(redirectURI, secret).Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	)
	if err != nil {
		return nil, fmt.Errorf("apple token exchange: %w", err)
	}
	raw, _ := tok.Extra("id_token").(string)
	if raw == "" {
		return nil, errors.New("apple response missing id_token")
	}
	return verifyOIDCIDToken(ctx, &a.mu, &a.jwks,
		"https://appleid.apple.com/auth/keys",
		[]string{IssuerApple},
		a.cfg.ClientID, raw, expectNonce, identity.ProviderApple)
}

// --- shared verify ---

type idTokenClaims struct {
	Email          string `json:"email"`
	EmailVerified  any    `json:"email_verified"`
	Name           string `json:"name"`
	Nonce          string `json:"nonce"`
	NonceSupported bool   `json:"nonce_supported"`
	jwt.RegisteredClaims
}

func verifyOIDCIDToken(
	ctx context.Context,
	mu *sync.Mutex,
	jwksSlot *keyfunc.Keyfunc,
	jwksURI string,
	allowedIssuers []string,
	audience, raw, expectNonce, provider string,
) (*SubjectClaims, error) {
	kf, err := loadJWKS(ctx, mu, jwksSlot, jwksURI)
	if err != nil {
		return nil, err
	}
	claims := &idTokenClaims{}
	parsed, err := jwt.ParseWithClaims(raw, claims, kf, jwt.WithValidMethods([]string{"RS256", "ES256"}), jwt.WithExpirationRequired(), jwt.WithAudience(audience))
	if err != nil {
		return nil, fmt.Errorf("invalid id_token: %w", err)
	}
	if !parsed.Valid {
		return nil, errors.New("invalid id_token")
	}
	issOK := false
	for _, want := range allowedIssuers {
		if claims.Issuer == want {
			issOK = true
			break
		}
	}
	if !issOK {
		return nil, fmt.Errorf("id_token iss %q rejected", claims.Issuer)
	}
	if claims.Subject == "" {
		return nil, errors.New("id_token missing sub")
	}
	if expectNonce != "" {
		if claims.Nonce == "" {
			return nil, errors.New("id_token missing nonce")
		}
		if claims.Nonce != expectNonce {
			return nil, errors.New("id_token nonce mismatch")
		}
	}
	email := strings.TrimSpace(claims.Email)
	verified := parseBoolish(claims.EmailVerified)
	name := strings.TrimSpace(claims.Name)
	return &SubjectClaims{
		Provider:      provider,
		Issuer:        claims.Issuer,
		Subject:       claims.Subject,
		Email:         email,
		EmailVerified: verified,
		Name:          name,
		Nonce:         claims.Nonce,
	}, nil
}

func loadJWKS(ctx context.Context, mu *sync.Mutex, slot *keyfunc.Keyfunc, uri string) (jwt.Keyfunc, error) {
	mu.Lock()
	defer mu.Unlock()
	if *slot != nil {
		return (*slot).Keyfunc, nil
	}
	k, err := keyfunc.NewDefaultCtx(ctx, []string{uri})
	if err != nil {
		return nil, fmt.Errorf("jwks %s: %w", uri, err)
	}
	*slot = k
	return k.Keyfunc, nil
}

func parseBoolish(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true") || t == "1"
	default:
		return false
	}
}

// RandomURLToken returns a high-entropy opaque token (base64url, no pad).
func RandomURLToken(nbytes int) (string, error) {
	if nbytes < 16 {
		nbytes = 32
	}
	b := make([]byte, nbytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// PKCEChallengeS256 returns the S256 challenge for verifier.
func PKCEChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// EmailDomainAllowed returns true when allowlist is empty or email's domain is listed.
// When email is empty, returns okEmailless (caller policy).
func EmailDomainAllowed(email string, allowlist []string, okEmailless bool) bool {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return okEmailless
	}
	if len(allowlist) == 0 {
		return true
	}
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return false
	}
	domain := email[at+1:]
	for _, d := range allowlist {
		if domain == d {
			return true
		}
	}
	return false
}

// CallbackURL builds Majesta One provider callback URL.
func CallbackURL(publicBase, provider string) string {
	return strings.TrimRight(publicBase, "/") + "/auth/v1/callback/" + url.PathEscape(provider)
}

// InferProviderFromIssuer maps OIDC issuer URLs to identity_links.provider.
func InferProviderFromIssuer(issuer string) string {
	iss := strings.ToLower(strings.TrimSpace(issuer))
	switch {
	case strings.Contains(iss, "accounts.google.com"):
		return identity.ProviderGoogle
	case strings.Contains(iss, "appleid.apple.com"):
		return identity.ProviderApple
	case strings.Contains(iss, "cognito-idp."):
		return identity.ProviderCognito
	case IsSlackIssuer(iss):
		return identity.ProviderSlack
	default:
		return identity.ProviderOIDC
	}
}

// IsSlackIssuer reports whether iss is Slack's OpenID issuer (exact, slash-normalized).
func IsSlackIssuer(iss string) bool {
	n := strings.TrimRight(strings.ToLower(strings.TrimSpace(iss)), "/")
	return n == "https://slack.com"
}

// SlackConfig configures Sign in with Slack (OpenID Connect user identity).
type SlackConfig struct {
	ClientID     string
	ClientSecret string
}

// NewSlackProvider builds a LoginProvider for Slack OpenID Connect.
// Bot tokens (xoxb-*) are not AuthN and are rejected on exchange.
func NewSlackProvider(cfg SlackConfig) *OIDCProvider {
	return NewOIDCProvider(OIDCConfig{
		Name:         identity.ProviderSlack,
		Issuer:       IssuerSlack,
		Audience:     strings.TrimSpace(cfg.ClientID),
		ClientID:     strings.TrimSpace(cfg.ClientID),
		ClientSecret: strings.TrimSpace(cfg.ClientSecret),
		AuthURL:      "https://slack.com/openid/connect/authorize",
		TokenURL:     "https://slack.com/api/openid.connect.token",
		JWKSURI:      "https://slack.com/openid/connect/keys",
		DisplayName:  "Slack",
	})
}

// FakeProvider is a test double that returns fixed claims after Exchange.
type FakeProvider struct {
	ProviderName string
	Claims       SubjectClaims
	FailExchange error
}

func (f *FakeProvider) Name() string     { return f.ProviderName }
func (f *FakeProvider) Configured() bool { return true }
func (f *FakeProvider) AuthCodeURL(state, nonce, codeChallenge, redirectURI string) (string, error) {
	u, _ := url.Parse("https://idp.test/authorize")
	q := u.Query()
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", codeChallenge)
	q.Set("redirect_uri", redirectURI)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
func (f *FakeProvider) Exchange(ctx context.Context, code, codeVerifier, redirectURI, expectNonce string) (*SubjectClaims, error) {
	if f.FailExchange != nil {
		return nil, f.FailExchange
	}
	c := f.Claims
	if c.Provider == "" {
		c.Provider = f.ProviderName
	}
	if expectNonce != "" {
		c.Nonce = expectNonce
	}
	return &c, nil
}
