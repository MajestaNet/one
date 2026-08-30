package connectoroauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TokenHTTPClient exchanges and refreshes OAuth tokens (redirects disabled by caller).
type TokenHTTPClient struct {
	HTTPClient *http.Client
}

func (c *TokenHTTPClient) client() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

// ExchangeAuthorizationCode trades code (+ optional PKCE) for tokens.
func (c *TokenHTTPClient) ExchangeAuthorizationCode(ctx context.Context, flow Flow, redirectURI, code, codeVerifier, clientSecret string) (*StoredToken, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", flow.ClientID)
	if codeVerifier != "" {
		form.Set("code_verifier", codeVerifier)
	}
	for k, v := range flow.TokenParams {
		form.Set(k, v)
	}
	return c.postToken(ctx, flow, form, clientSecret)
}

// ClientCredentials mints a token with client credentials grant.
func (c *TokenHTTPClient) ClientCredentials(ctx context.Context, flow Flow, clientSecret string) (*StoredToken, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", flow.ClientID)
	if len(flow.Scopes) > 0 {
		form.Set("scope", strings.Join(flow.Scopes, " "))
	}
	for k, v := range flow.TokenParams {
		form.Set(k, v)
	}
	return c.postToken(ctx, flow, form, clientSecret)
}

// RefreshAccessToken refreshes using a refresh_token grant.
func (c *TokenHTTPClient) RefreshAccessToken(ctx context.Context, flow Flow, refreshToken, clientSecret string) (*StoredToken, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, fmt.Errorf("refresh token required")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", flow.ClientID)
	for k, v := range flow.TokenParams {
		form.Set(k, v)
	}
	tok, err := c.postToken(ctx, flow, form, clientSecret)
	if err != nil {
		return nil, err
	}
	if tok.RefreshToken == "" {
		tok.RefreshToken = refreshToken
	}
	return tok, nil
}

func (c *TokenHTTPClient) postToken(ctx context.Context, flow Flow, form url.Values, clientSecret string) (*StoredToken, error) {
	style := strings.ToLower(strings.TrimSpace(flow.AuthStyle))
	if style == "" {
		style = "auto"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, flow.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	switch style {
	case "header":
		req.SetBasicAuth(flow.ClientID, clientSecret)
	case "params":
		if clientSecret != "" {
			q := form
			q.Set("client_secret", clientSecret)
			req.Body = io.NopCloser(strings.NewReader(q.Encode()))
			req.ContentLength = int64(len(q.Encode()))
		}
	default: // auto: basic when secret present else body
		if clientSecret != "" {
			req.SetBasicAuth(flow.ClientID, clientSecret)
		}
	}
	res, err := c.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		// OAuth error bodies are controlled by the remote provider and may contain
		// client secrets, authorization codes, or tokens. Keep them out of errors:
		// callers can log or return these errors across trust boundaries.
		return nil, fmt.Errorf("token endpoint returned HTTP %d", res.StatusCode)
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	access, _ := raw["access_token"].(string)
	if access == "" {
		return nil, fmt.Errorf("token response missing access_token")
	}
	tok := &StoredToken{
		AccessToken:  access,
		RefreshToken: stringField(raw, "refresh_token"),
		TokenType:    stringField(raw, "token_type"),
	}
	if tok.TokenType == "" {
		tok.TokenType = "Bearer"
	}
	if exp, ok := raw["expires_in"].(float64); ok && exp > 0 {
		tok.Expiry = time.Now().UTC().Add(time.Duration(exp) * time.Second)
	}
	return tok, nil
}

func stringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// BuildAuthorizeURL constructs the provider authorization redirect.
func BuildAuthorizeURL(flow Flow, redirectURI, state, codeChallenge string) (string, error) {
	u, err := url.Parse(flow.AuthorizationURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", flow.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	if len(flow.Scopes) > 0 {
		q.Set("scope", strings.Join(flow.Scopes, " "))
	}
	if codeChallenge != "" {
		q.Set("code_challenge", codeChallenge)
		q.Set("code_challenge_method", "S256")
	}
	for k, v := range flow.AuthorizationParams {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// NeedsRefresh reports whether the token should be refreshed (60s skew).
func NeedsRefresh(tok *StoredToken) bool {
	if tok == nil || tok.AccessToken == "" {
		return true
	}
	if tok.Expiry.IsZero() {
		return false
	}
	return time.Now().UTC().After(tok.Expiry.Add(-60 * time.Second))
}
