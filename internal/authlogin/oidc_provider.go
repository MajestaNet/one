package authlogin

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/identity"
)

// OIDCConfig is a customer-configured generic OIDC provider (Okta / Entra / Keycloak / …).
type OIDCConfig struct {
	Name         string // identity_links.provider; default oidc
	Issuer       string
	Audience     string // client id / aud
	ClientID     string
	ClientSecret string
	JWKSURI      string
	DisplayName  string
	AuthURL      string // optional override
	TokenURL     string // optional override
}

// OIDCProvider implements Provider for a single customer IdP.
type OIDCProvider struct {
	cfg        OIDCConfig
	mu         sync.Mutex
	jwks       keyfunc.Keyfunc
	jwksCancel context.CancelFunc
	endpoint   oauth2.Endpoint
	httpClient *http.Client
	ready      bool
}

// NewOIDCProvider constructs a generic OIDC provider (Configured requires issuer + client id + secret).
func NewOIDCProvider(cfg OIDCConfig) *OIDCProvider {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	transport := base.Clone()
	transport.Proxy = nil
	return &OIDCProvider{
		cfg: cfg,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   15 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (o *OIDCProvider) Name() string {
	if o != nil {
		if name := strings.ToLower(strings.TrimSpace(o.cfg.Name)); name != "" {
			return name
		}
	}
	return identity.ProviderOIDC
}

func (o *OIDCProvider) Configured() bool {
	return o != nil &&
		strings.TrimSpace(o.cfg.Issuer) != "" &&
		strings.TrimSpace(o.cfg.ClientID) != "" &&
		strings.TrimSpace(o.cfg.ClientSecret) != ""
}

func (o *OIDCProvider) ensureEndpoints(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.ready {
		return nil
	}
	authURL := strings.TrimSpace(o.cfg.AuthURL)
	tokenURL := strings.TrimSpace(o.cfg.TokenURL)
	jwksURI := strings.TrimSpace(o.cfg.JWKSURI)
	if err := validateOIDCEndpoint("issuer", o.cfg.Issuer); err != nil {
		return err
	}
	if authURL == "" || tokenURL == "" || jwksURI == "" {
		disc, err := authz.DiscoverOIDC(ctx, o.httpClient, o.cfg.Issuer)
		if err != nil {
			return err
		}
		if authURL == "" {
			authURL = disc.Authorization
		}
		if tokenURL == "" {
			tokenURL = disc.Token
		}
		if jwksURI == "" {
			jwksURI = disc.JWKSURI
		}
	}
	for name, endpoint := range map[string]string{
		"authorization endpoint": authURL,
		"token endpoint":         tokenURL,
		"JWKS endpoint":          jwksURI,
	} {
		if err := validateOIDCEndpoint(name, endpoint); err != nil {
			return err
		}
	}
	o.endpoint = oauth2.Endpoint{AuthURL: authURL, TokenURL: tokenURL}
	if jwksURI != "" {
		refreshCtx, cancel := context.WithCancel(context.Background())
		jwks, err := keyfunc.NewDefaultOverrideCtx(refreshCtx, []string{jwksURI}, keyfunc.Override{
			Client: o.httpClient,
		})
		if err != nil {
			cancel()
			return err
		}
		o.jwks = jwks
		o.jwksCancel = cancel
		o.cfg.JWKSURI = jwksURI
	}
	o.ready = true
	return nil
}

func (o *OIDCProvider) close() {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.jwksCancel != nil {
		o.jwksCancel()
		o.jwksCancel = nil
	}
}

func (o *OIDCProvider) AuthCodeURL(state, nonce, codeChallenge, redirectURI string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := o.ensureEndpoints(ctx); err != nil {
		return "", err
	}
	cfg := &oauth2.Config{
		ClientID:     o.cfg.ClientID,
		ClientSecret: o.cfg.ClientSecret,
		RedirectURL:  redirectURI,
		Endpoint:     o.endpoint,
		Scopes:       []string{"openid", "email", "profile"},
	}
	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("nonce", nonce),
	}
	return cfg.AuthCodeURL(state, opts...), nil
}

func validateOIDCEndpoint(name, raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || u.Host == "" || u.User != nil {
		return fmt.Errorf("OIDC %s must be an absolute HTTPS URL", name)
	}
	if u.Scheme == "https" || (u.Scheme == "http" && oidcLoopbackHost(u.Hostname())) {
		return nil
	}
	return fmt.Errorf("OIDC %s must use HTTPS (HTTP is allowed only for loopback development)", name)
}

func oidcLoopbackHost(host string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (o *OIDCProvider) Exchange(ctx context.Context, code, codeVerifier, redirectURI, expectNonce string) (*SubjectClaims, error) {
	if err := o.ensureEndpoints(ctx); err != nil {
		return nil, err
	}
	cfg := &oauth2.Config{
		ClientID:     o.cfg.ClientID,
		ClientSecret: o.cfg.ClientSecret,
		RedirectURL:  redirectURI,
		Endpoint:     o.endpoint,
		Scopes:       []string{"openid", "email", "profile"},
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, o.httpClient)
	tok, err := cfg.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return nil, err
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok || raw == "" {
		return nil, fmt.Errorf("id_token missing from token response")
	}
	if o.jwks == nil {
		return nil, fmt.Errorf("jwks unavailable")
	}
	claims, err := verifyCustomerOIDCIDToken(raw, o.jwks.Keyfunc, o.cfg.Issuer, o.cfg.Audience, o.cfg.ClientID, expectNonce)
	if err != nil {
		return nil, err
	}
	return &SubjectClaims{
		Provider:      o.Name(),
		Issuer:        strings.TrimSpace(claims.Issuer),
		Subject:       claims.Subject,
		Email:         strings.TrimSpace(claims.Email),
		EmailVerified: parseBoolish(claims.EmailVerified),
		Name:          strings.TrimSpace(claims.Name),
		Nonce:         claims.Nonce,
	}, nil
}

type customerOIDCIDTokenClaims struct {
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"`
	Name          string `json:"name"`
	Nonce         string `json:"nonce"`
	jwt.RegisteredClaims
}

func verifyCustomerOIDCIDToken(raw string, keyfunc jwt.Keyfunc, issuer, audience, clientID, expectNonce string) (*customerOIDCIDTokenClaims, error) {
	audiences := []string{}
	for _, candidate := range []string{audience, clientID} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || (len(audiences) > 0 && audiences[0] == candidate) {
			continue
		}
		audiences = append(audiences, candidate)
	}
	if len(audiences) == 0 {
		return nil, fmt.Errorf("OIDC audience is required")
	}

	var lastErr error
	for _, expectedAudience := range audiences {
		claims := &customerOIDCIDTokenClaims{}
		parsed, err := jwt.ParseWithClaims(raw, claims, keyfunc,
			jwt.WithIssuer(issuer),
			jwt.WithAudience(expectedAudience),
			jwt.WithValidMethods([]string{"RS256", "ES256"}),
			jwt.WithExpirationRequired(),
		)
		if err != nil {
			lastErr = err
			continue
		}
		if !parsed.Valid {
			lastErr = fmt.Errorf("invalid id_token")
			continue
		}
		if strings.TrimSpace(claims.Subject) == "" {
			return nil, fmt.Errorf("id_token missing sub")
		}
		if expectNonce != "" {
			if claims.Nonce == "" {
				return nil, fmt.Errorf("id_token missing nonce")
			}
			if subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(expectNonce)) != 1 {
				return nil, fmt.Errorf("id_token nonce mismatch")
			}
		}
		return claims, nil
	}
	return nil, fmt.Errorf("invalid id_token: %w", lastErr)
}

// RegisterOrReplace adds/replaces a provider on the broker (used for customer OIDC).
func (b *Broker) RegisterOrReplace(p Provider) {
	if b == nil || p == nil {
		return
	}
	b.mu.Lock()
	if b.Providers == nil {
		b.Providers = map[string]Provider{}
	}
	name := strings.ToLower(p.Name())
	previous := b.Providers[name]
	b.Providers[name] = p
	b.mu.Unlock()
	if old, ok := previous.(*OIDCProvider); ok && old != p {
		old.close()
	}
}
