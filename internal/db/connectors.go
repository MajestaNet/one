package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MajestaNet/ide/internal/connectoroauth"
)

// InstallSecret is a row in install_secrets (ciphertext never returned to Metadata list).
type InstallSecret struct {
	APIName    string
	Label      string
	Ciphertext string
	HasSecret  bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// InstallConnector is a row in install_connectors.
type InstallConnector struct {
	APIName        string
	Label          string
	BaseURL        string
	SecretRef      *string
	AllowedMethods []string
	PathPrefix     string
	Active         bool
	AuthType       string
	OAuthFlow      connectoroauth.Flow
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// InstallConnectorOAuthToken is install-local encrypted OAuth token material.
type InstallConnectorOAuthToken struct {
	ConnectorAPIName string
	TokenCiphertext  string
	ExpiresAt        *time.Time
	Refreshable      bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// InstallConnectorOAuthState is a single-use authorize state row.
type InstallConnectorOAuthState struct {
	StateHash        string
	ConnectorAPIName string
	ActorID          *string
	CodeVerifier     string
	RedirectURI      string
	ConfigHash       string
	ExpiresAt        time.Time
	CreatedAt        time.Time
}

// EgressAllowEntry is one install_egress_allowlist row.
type EgressAllowEntry struct {
	ID          string
	HostPattern string
	Label       string
	CreatedAt   time.Time
}

// ListInstallSecrets returns secret metadata without ciphertext.
func ListInstallSecrets(ctx context.Context, pool *Pool) ([]InstallSecret, error) {
	rows, err := pool.Query(ctx, `
SELECT api_name, label, (ciphertext IS NOT NULL AND ciphertext <> '') AS has_secret, created_at, updated_at
FROM install_secrets ORDER BY api_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InstallSecret
	for rows.Next() {
		var s InstallSecret
		if err := rows.Scan(&s.APIName, &s.Label, &s.HasSecret, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetInstallSecretCiphertext loads ciphertext for worker/host use.
func GetInstallSecretCiphertext(ctx context.Context, pool *Pool, apiName string) (string, error) {
	var ct string
	err := pool.QueryRow(ctx, `SELECT ciphertext FROM install_secrets WHERE api_name=$1`, apiName).Scan(&ct)
	if err != nil {
		return "", err
	}
	return ct, nil
}

// UpsertInstallSecret creates or replaces ciphertext for api_name.
func UpsertInstallSecret(ctx context.Context, pool *Pool, apiName, label, ciphertext string) error {
	apiName = strings.TrimSpace(apiName)
	if apiName == "" || ciphertext == "" {
		return fmt.Errorf("apiName and ciphertext required")
	}
	if label == "" {
		label = apiName
	}
	_, err := pool.Exec(ctx, `
INSERT INTO install_secrets (api_name, label, ciphertext, updated_at)
VALUES ($1,$2,$3,now())
ON CONFLICT (api_name) DO UPDATE SET
  label = EXCLUDED.label,
  ciphertext = EXCLUDED.ciphertext,
  updated_at = now()`, apiName, label, ciphertext)
	return err
}

// DeleteInstallSecret removes a secret by api_name.
func DeleteInstallSecret(ctx context.Context, pool *Pool, apiName string) error {
	tag, err := pool.Exec(ctx, `DELETE FROM install_secrets WHERE api_name=$1`, apiName)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ListInstallConnectors returns all connectors.
func ListInstallConnectors(ctx context.Context, pool *Pool) ([]InstallConnector, error) {
	rows, err := pool.Query(ctx, `
SELECT api_name, label, base_url, secret_ref, allowed_methods, COALESCE(path_prefix,''), active,
       COALESCE(auth_type,'static_bearer'), COALESCE(oauth_flow,'{}'::jsonb), created_at, updated_at
FROM install_connectors ORDER BY api_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InstallConnector
	for rows.Next() {
		c, err := scanConnector(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetInstallConnector loads one connector by api_name.
func GetInstallConnector(ctx context.Context, pool *Pool, apiName string) (*InstallConnector, error) {
	row := pool.QueryRow(ctx, `
SELECT api_name, label, base_url, secret_ref, allowed_methods, COALESCE(path_prefix,''), active,
       COALESCE(auth_type,'static_bearer'), COALESCE(oauth_flow,'{}'::jsonb), created_at, updated_at
FROM install_connectors WHERE api_name=$1`, apiName)
	c, err := scanConnector(row)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanConnector(row scannable) (InstallConnector, error) {
	var c InstallConnector
	var methods []byte
	var flowJSON []byte
	var secretRef *string
	if err := row.Scan(&c.APIName, &c.Label, &c.BaseURL, &secretRef, &methods, &c.PathPrefix, &c.Active,
		&c.AuthType, &flowJSON, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return c, err
	}
	c.SecretRef = secretRef
	c.AuthType = connectoroauth.NormalizeAuthType(c.AuthType)
	_ = json.Unmarshal(methods, &c.AllowedMethods)
	if len(c.AllowedMethods) == 0 {
		c.AllowedMethods = []string{"GET", "POST"}
	}
	flow, err := connectoroauth.ParseFlowJSON(flowJSON)
	if err != nil {
		return c, err
	}
	c.OAuthFlow = flow
	return c, nil
}

// UpsertInstallConnector inserts or updates a connector.
func UpsertInstallConnector(ctx context.Context, pool *Pool, c InstallConnector) error {
	if strings.TrimSpace(c.APIName) == "" || strings.TrimSpace(c.BaseURL) == "" {
		return fmt.Errorf("apiName and baseUrl required")
	}
	if c.Label == "" {
		c.Label = c.APIName
	}
	methods := c.AllowedMethods
	if len(methods) == 0 {
		methods = []string{"GET", "POST"}
	}
	mj, _ := json.Marshal(methods)
	authType := connectoroauth.NormalizeAuthType(c.AuthType)
	if err := connectoroauth.ValidateAuthType(authType); err != nil {
		return err
	}
	if err := connectoroauth.ValidateFlow(authType, c.OAuthFlow); err != nil {
		return err
	}
	flowJSON := connectoroauth.FlowJSON(c.OAuthFlow)
	_, err := pool.Exec(ctx, `
INSERT INTO install_connectors (api_name, label, base_url, secret_ref, allowed_methods, path_prefix, active, auth_type, oauth_flow, updated_at)
VALUES ($1,$2,$3,$4,$5::jsonb,$6,$7,$8,$9::jsonb,now())
ON CONFLICT (api_name) DO UPDATE SET
  label = EXCLUDED.label,
  base_url = EXCLUDED.base_url,
  secret_ref = EXCLUDED.secret_ref,
  allowed_methods = EXCLUDED.allowed_methods,
  path_prefix = EXCLUDED.path_prefix,
  active = EXCLUDED.active,
  auth_type = EXCLUDED.auth_type,
  oauth_flow = EXCLUDED.oauth_flow,
  updated_at = now()`,
		c.APIName, c.Label, c.BaseURL, c.SecretRef, string(mj), c.PathPrefix, c.Active, authType, string(flowJSON))
	return err
}

// DeleteInstallConnector removes a connector.
func DeleteInstallConnector(ctx context.Context, pool *Pool, apiName string) error {
	tag, err := pool.Exec(ctx, `DELETE FROM install_connectors WHERE api_name=$1`, apiName)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// GetInstallConnectorOAuthToken loads the encrypted token row.
func GetInstallConnectorOAuthToken(ctx context.Context, pool *Pool, connectorAPIName string) (*InstallConnectorOAuthToken, error) {
	var t InstallConnectorOAuthToken
	err := pool.QueryRow(ctx, `
SELECT connector_api_name, token_ciphertext, expires_at, refreshable, created_at, updated_at
FROM install_connector_oauth_tokens WHERE connector_api_name=$1`, connectorAPIName).
		Scan(&t.ConnectorAPIName, &t.TokenCiphertext, &t.ExpiresAt, &t.Refreshable, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// UpsertInstallConnectorOAuthToken stores encrypted token material.
func UpsertInstallConnectorOAuthToken(ctx context.Context, pool *Pool, connectorAPIName, ciphertext string, expiresAt *time.Time, refreshable bool) error {
	connectorAPIName = strings.TrimSpace(connectorAPIName)
	if connectorAPIName == "" || ciphertext == "" {
		return fmt.Errorf("connectorApiName and ciphertext required")
	}
	_, err := pool.Exec(ctx, `
INSERT INTO install_connector_oauth_tokens (connector_api_name, token_ciphertext, expires_at, refreshable, updated_at)
VALUES ($1,$2,$3,$4,now())
ON CONFLICT (connector_api_name) DO UPDATE SET
  token_ciphertext = EXCLUDED.token_ciphertext,
  expires_at = EXCLUDED.expires_at,
  refreshable = EXCLUDED.refreshable,
  updated_at = now()`, connectorAPIName, ciphertext, expiresAt, refreshable)
	return err
}

// DeleteInstallConnectorOAuthToken removes stored tokens for a connector.
func DeleteInstallConnectorOAuthToken(ctx context.Context, pool *Pool, connectorAPIName string) error {
	_, err := pool.Exec(ctx, `DELETE FROM install_connector_oauth_tokens WHERE connector_api_name=$1`, connectorAPIName)
	return err
}

// PutInstallConnectorOAuthState inserts a hashed authorize state.
func PutInstallConnectorOAuthState(ctx context.Context, pool *Pool, st InstallConnectorOAuthState) error {
	if st.StateHash == "" || st.ConnectorAPIName == "" || st.RedirectURI == "" || st.ConfigHash == "" {
		return fmt.Errorf("state fields required")
	}
	_, err := pool.Exec(ctx, `
INSERT INTO install_connector_oauth_states
  (state_hash, connector_api_name, actor_id, code_verifier, redirect_uri, config_hash, expires_at)
VALUES ($1,$2,$3::uuid,$4,$5,$6,$7)`,
		st.StateHash, st.ConnectorAPIName, st.ActorID, st.CodeVerifier, st.RedirectURI, st.ConfigHash, st.ExpiresAt)
	return err
}

// TakeInstallConnectorOAuthState atomically deletes and returns a state row.
func TakeInstallConnectorOAuthState(ctx context.Context, pool *Pool, stateHash string) (*InstallConnectorOAuthState, error) {
	var st InstallConnectorOAuthState
	err := pool.QueryRow(ctx, `
DELETE FROM install_connector_oauth_states
WHERE state_hash=$1
RETURNING state_hash, connector_api_name, actor_id::text, COALESCE(code_verifier,''), redirect_uri, config_hash, expires_at, created_at`,
		stateHash).Scan(&st.StateHash, &st.ConnectorAPIName, &st.ActorID, &st.CodeVerifier, &st.RedirectURI, &st.ConfigHash, &st.ExpiresAt, &st.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// ListEgressAllowlist returns host patterns.
func ListEgressAllowlist(ctx context.Context, pool *Pool) ([]EgressAllowEntry, error) {
	rows, err := pool.Query(ctx, `
SELECT id::text, host_pattern, COALESCE(label,''), created_at
FROM install_egress_allowlist ORDER BY host_pattern`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EgressAllowEntry
	for rows.Next() {
		var e EgressAllowEntry
		if err := rows.Scan(&e.ID, &e.HostPattern, &e.Label, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListEgressHostPatterns returns only host_pattern strings for allowlist checks.
func ListEgressHostPatterns(ctx context.Context, pool *Pool) ([]string, error) {
	entries, err := ListEgressAllowlist(ctx, pool)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.HostPattern)
	}
	return out, nil
}

// AddEgressAllowEntry inserts a host pattern.
func AddEgressAllowEntry(ctx context.Context, pool *Pool, hostPattern, label string) (*EgressAllowEntry, error) {
	hostPattern = strings.TrimSpace(hostPattern)
	if hostPattern == "" {
		return nil, fmt.Errorf("hostPattern required")
	}
	var e EgressAllowEntry
	err := pool.QueryRow(ctx, `
INSERT INTO install_egress_allowlist (host_pattern, label)
VALUES ($1,$2)
RETURNING id::text, host_pattern, COALESCE(label,''), created_at`, hostPattern, label).
		Scan(&e.ID, &e.HostPattern, &e.Label, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// DeleteEgressAllowEntry removes by host_pattern.
func DeleteEgressAllowEntry(ctx context.Context, pool *Pool, hostPattern string) error {
	tag, err := pool.Exec(ctx, `DELETE FROM install_egress_allowlist WHERE host_pattern=$1`, hostPattern)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
