package connectoroauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Auth type constants (install_connectors.auth_type).
const (
	AuthStaticBearer            = "static_bearer"
	AuthOAuth2ClientCredentials = "oauth2_client_credentials"
	AuthOAuth2AuthorizationCode = "oauth2_authorization_code"
)

// Flow is the secret-free OAuth configuration stored on install_connectors.oauth_flow.
type Flow struct {
	AuthorizationURL    string            `json:"authorizationUrl,omitempty"`
	TokenURL            string            `json:"tokenUrl,omitempty"`
	ClientID            string            `json:"clientId,omitempty"`
	Scopes              []string          `json:"scopes,omitempty"`
	PKCE                bool              `json:"pkce,omitempty"`
	AuthStyle           string            `json:"authStyle,omitempty"` // auto|header|params
	AuthorizationParams map[string]string `json:"authorizationParams,omitempty"`
	TokenParams         map[string]string `json:"tokenParams,omitempty"`
}

// StoredToken is the encrypted token envelope.
type StoredToken struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	TokenType    string    `json:"tokenType,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
}

var reservedOAuthParams = map[string]struct{}{
	"client_id": {}, "client_secret": {}, "redirect_uri": {}, "code": {},
	"code_verifier": {}, "code_challenge": {}, "code_challenge_method": {},
	"grant_type": {}, "refresh_token": {}, "state": {}, "response_type": {},
	"scope": {},
}

// NormalizeAuthType returns a canonical auth type (default static_bearer).
func NormalizeAuthType(v string) string {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case AuthOAuth2ClientCredentials, "client_credentials":
		return AuthOAuth2ClientCredentials
	case AuthOAuth2AuthorizationCode, "authorization_code":
		return AuthOAuth2AuthorizationCode
	case AuthStaticBearer, "bearer", "static", "":
		return AuthStaticBearer
	default:
		return strings.TrimSpace(strings.ToLower(v))
	}
}

// ValidateAuthType rejects unknown types.
func ValidateAuthType(v string) error {
	switch NormalizeAuthType(v) {
	case AuthStaticBearer, AuthOAuth2ClientCredentials, AuthOAuth2AuthorizationCode:
		return nil
	default:
		return fmt.Errorf("unsupported authType %q", v)
	}
}

// ValidateFlow checks OAuth flow fields for the given auth type.
func ValidateFlow(authType string, flow Flow) error {
	authType = NormalizeAuthType(authType)
	if authType == AuthStaticBearer {
		return nil
	}
	if strings.TrimSpace(flow.TokenURL) == "" {
		return fmt.Errorf("oauthFlow.tokenUrl required")
	}
	if u, err := url.Parse(flow.TokenURL); err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("oauthFlow.tokenUrl must be https")
	}
	if strings.TrimSpace(flow.ClientID) == "" {
		return fmt.Errorf("oauthFlow.clientId required")
	}
	if authType == AuthOAuth2AuthorizationCode {
		if strings.TrimSpace(flow.AuthorizationURL) == "" {
			return fmt.Errorf("oauthFlow.authorizationUrl required for authorization_code")
		}
		if u, err := url.Parse(flow.AuthorizationURL); err != nil || u.Scheme != "https" || u.Host == "" {
			return fmt.Errorf("oauthFlow.authorizationUrl must be https")
		}
	}
	for k := range flow.AuthorizationParams {
		if _, bad := reservedOAuthParams[strings.ToLower(k)]; bad {
			return fmt.Errorf("oauthFlow.authorizationParams reserves %q", k)
		}
	}
	for k := range flow.TokenParams {
		if _, bad := reservedOAuthParams[strings.ToLower(k)]; bad {
			return fmt.Errorf("oauthFlow.tokenParams reserves %q", k)
		}
	}
	return nil
}

// ConfigHash binds authorize state to the current flow + secret ref.
func ConfigHash(authType string, flow Flow, secretRef string) string {
	raw, _ := json.Marshal(struct {
		AuthType  string `json:"authType"`
		Flow      Flow   `json:"flow"`
		SecretRef string `json:"secretRef"`
	}{AuthType: NormalizeAuthType(authType), Flow: flow, SecretRef: secretRef})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// HashState SHA-256 hex-encodes the opaque state token.
func HashState(state string) string {
	sum := sha256.Sum256([]byte(state))
	return hex.EncodeToString(sum[:])
}

// NewState returns a URL-safe random state string.
func NewState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// NewPKCE returns verifier and S256 challenge.
func NewPKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

// ParseFlowJSON decodes oauth_flow JSONB.
func ParseFlowJSON(raw []byte) (Flow, error) {
	var f Flow
	if len(raw) == 0 || string(raw) == "null" {
		return f, nil
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return f, err
	}
	return f, nil
}

// FlowJSON encodes flow for persistence.
func FlowJSON(f Flow) []byte {
	b, _ := json.Marshal(f)
	return b
}
