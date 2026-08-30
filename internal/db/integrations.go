package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// IntegrationConfig is a Connected App / inbound OAuth client configuration.
type IntegrationConfig struct {
	ID                string
	APIName           string
	Label             string
	Description       string
	PrincipalID       string
	ClientKind        string // public | confidential
	OAuthFlows        []string
	CallbackURLs      []string
	LogoutURLs        []string
	AllowedScopesHint []string
	AllowedCIDRs      []string
	PKCERequired      bool
	Ownership         string // managed | customer
	PackageName       *string
	IsActive          bool
	OneSecretEnc      *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// IntegrationStore persists integration_configs.
type IntegrationStore struct {
	pool *Pool
}

// NewIntegrationStore constructs an integration store.
func NewIntegrationStore(pool *Pool) *IntegrationStore {
	return &IntegrationStore{pool: pool}
}

const integrationSelectCols = `
id::text, api_name, label, description, principal_id::text, client_kind,
oauth_flows, callback_urls, logout_urls, allowed_scopes_hint, COALESCE(allowed_cidrs, '[]'::jsonb), pkce_required,
ownership, package_name, is_active,
one_secret_enc, created_at, updated_at`

func scanIntegration(row pgx.Row) (*IntegrationConfig, error) {
	var c IntegrationConfig
	var cidrsRaw []byte
	err := row.Scan(
		&c.ID, &c.APIName, &c.Label, &c.Description, &c.PrincipalID, &c.ClientKind,
		&c.OAuthFlows, &c.CallbackURLs, &c.LogoutURLs, &c.AllowedScopesHint, &cidrsRaw, &c.PKCERequired,
		&c.Ownership, &c.PackageName, &c.IsActive,
		&c.OneSecretEnc, &c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if c.OAuthFlows == nil {
		c.OAuthFlows = []string{}
	}
	if c.CallbackURLs == nil {
		c.CallbackURLs = []string{}
	}
	if c.LogoutURLs == nil {
		c.LogoutURLs = []string{}
	}
	if c.AllowedScopesHint == nil {
		c.AllowedScopesHint = []string{}
	}
	c.AllowedCIDRs = []string{}
	if len(cidrsRaw) > 0 {
		_ = json.Unmarshal(cidrsRaw, &c.AllowedCIDRs)
		if c.AllowedCIDRs == nil {
			c.AllowedCIDRs = []string{}
		}
	}
	return &c, nil
}

// CreateIntegrationInput is the insert payload.
type CreateIntegrationInput struct {
	APIName           string
	Label             string
	Description       string
	PrincipalID       string
	ClientKind        string
	OAuthFlows        []string
	CallbackURLs      []string
	LogoutURLs        []string
	AllowedScopesHint []string
	PKCERequired      bool
	Ownership         string
	PackageName       string
	IsActive          bool
	OneSecretEnc      string
}

// Insert creates an integration_configs row.
func (s *IntegrationStore) Insert(ctx context.Context, in CreateIntegrationInput) (*IntegrationConfig, error) {
	apiName := strings.TrimSpace(in.APIName)
	if apiName == "" {
		return nil, fmt.Errorf("%w: apiName is required", ErrValidation)
	}
	label := strings.TrimSpace(in.Label)
	if label == "" {
		label = apiName
	}
	kind := strings.TrimSpace(in.ClientKind)
	if kind != "public" && kind != "confidential" {
		return nil, fmt.Errorf("%w: clientKind must be public or confidential", ErrValidation)
	}
	ownership := strings.TrimSpace(in.Ownership)
	if ownership == "" {
		ownership = "custom"
	}
	if ownership != "managed" && ownership != "custom" {
		return nil, fmt.Errorf("%w: ownership must be managed or customer", ErrValidation)
	}
	flows := in.OAuthFlows
	if flows == nil {
		flows = []string{}
	}
	callbacks := in.CallbackURLs
	if callbacks == nil {
		callbacks = []string{}
	}
	logouts := in.LogoutURLs
	if logouts == nil {
		logouts = []string{}
	}
	scopes := in.AllowedScopesHint
	if scopes == nil {
		scopes = []string{}
	}
	var pkg any
	if strings.TrimSpace(in.PackageName) != "" {
		pkg = strings.TrimSpace(in.PackageName)
	}
	var oneEnc any
	if in.OneSecretEnc != "" {
		oneEnc = in.OneSecretEnc
	}
	active := in.IsActive
	row := s.pool.QueryRow(ctx, `
INSERT INTO integration_configs (
  api_name, label, description, principal_id, client_kind,
  oauth_flows, callback_urls, logout_urls, allowed_scopes_hint, pkce_required,
  ownership, package_name, is_active, one_secret_enc
) VALUES (
  $1, $2, $3, $4::uuid, $5,
  $6, $7, $8, $9, $10,
  $11, $12, $13, $14
)
RETURNING `+integrationSelectCols,
		apiName, label, in.Description, in.PrincipalID, kind,
		flows, callbacks, logouts, scopes, in.PKCERequired,
		ownership, pkg, active, oneEnc,
	)
	c, err := scanIntegration(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("%w: apiName already exists", ErrConflict)
		}
		return nil, err
	}
	return c, nil
}

// GetByAPIName loads by api_name.
func (s *IntegrationStore) GetByAPIName(ctx context.Context, apiName string) (*IntegrationConfig, error) {
	row := s.pool.QueryRow(ctx, `
SELECT `+integrationSelectCols+` FROM integration_configs WHERE api_name = $1`, strings.TrimSpace(apiName))
	return scanIntegration(row)
}

// List returns all integrations ordered by api_name.
func (s *IntegrationStore) List(ctx context.Context) ([]IntegrationConfig, error) {
	rows, err := s.pool.Query(ctx, `
SELECT `+integrationSelectCols+` FROM integration_configs ORDER BY api_name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IntegrationConfig
	for rows.Next() {
		c, err := scanIntegration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// PatchIntegrationInput is a partial update.
type PatchIntegrationInput struct {
	Label             *string
	Description       *string
	OAuthFlows        *[]string
	CallbackURLs      *[]string
	LogoutURLs        *[]string
	AllowedScopesHint *[]string
	AllowedCIDRs      *[]string
	PKCERequired      *bool
	IsActive          *bool
	OneSecretEnc      *string
	ClearOneSecret    bool
}

// Patch updates mutable fields by api_name.
func (s *IntegrationStore) Patch(ctx context.Context, apiName string, in PatchIntegrationInput) (*IntegrationConfig, error) {
	cur, err := s.GetByAPIName(ctx, apiName)
	if err != nil {
		return nil, err
	}
	label := cur.Label
	if in.Label != nil {
		label = strings.TrimSpace(*in.Label)
		if label == "" {
			label = cur.Label
		}
	}
	desc := cur.Description
	if in.Description != nil {
		desc = *in.Description
	}
	flows := cur.OAuthFlows
	if in.OAuthFlows != nil {
		flows = *in.OAuthFlows
	}
	callbacks := cur.CallbackURLs
	if in.CallbackURLs != nil {
		callbacks = *in.CallbackURLs
	}
	logouts := cur.LogoutURLs
	if in.LogoutURLs != nil {
		logouts = *in.LogoutURLs
	}
	scopes := cur.AllowedScopesHint
	if in.AllowedScopesHint != nil {
		scopes = *in.AllowedScopesHint
	}
	cidrs := cur.AllowedCIDRs
	if in.AllowedCIDRs != nil {
		cidrs = *in.AllowedCIDRs
	}
	pkce := cur.PKCERequired
	if in.PKCERequired != nil {
		pkce = *in.PKCERequired
	}
	active := cur.IsActive
	if in.IsActive != nil {
		active = *in.IsActive
	}
	oneEnc := cur.OneSecretEnc
	if in.ClearOneSecret {
		oneEnc = nil
	} else if in.OneSecretEnc != nil {
		v := *in.OneSecretEnc
		if v == "" {
			oneEnc = nil
		} else {
			oneEnc = &v
		}
	}
	row := s.pool.QueryRow(ctx, `
UPDATE integration_configs SET
  label = $2, description = $3, oauth_flows = $4, callback_urls = $5, logout_urls = $6,
  allowed_scopes_hint = $7, allowed_cidrs = $8::jsonb, pkce_required = $9, is_active = $10,
  one_secret_enc = $11,
  updated_at = now()
WHERE api_name = $1
RETURNING `+integrationSelectCols,
		apiName, label, desc, flows, callbacks, logouts, scopes, mustJSON(cidrs), pkce, active,
		oneEnc,
	)
	return scanIntegration(row)
}

// Delete removes a customer integration by api_name. Returns ErrNotFound when missing.
func (s *IntegrationStore) Delete(ctx context.Context, apiName string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM integration_configs WHERE api_name = $1`, strings.TrimSpace(apiName))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListIntegrationAllowedCIDRs unions allowed_cidrs from active integrations.
func ListIntegrationAllowedCIDRs(ctx context.Context, pool *Pool) ([]string, error) {
	if pool == nil {
		return nil, nil
	}
	rows, err := pool.Query(ctx, `
SELECT COALESCE(allowed_cidrs, '[]'::jsonb)
FROM integration_configs
WHERE is_active = true`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]struct{}{}
	var out []string
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var cidrs []string
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &cidrs)
		}
		for _, c := range cidrs {
			c = strings.TrimSpace(c)
			if c == "" {
				continue
			}
			if _, ok := seen[c]; ok {
				continue
			}
			seen[c] = struct{}{}
			out = append(out, c)
		}
	}
	return out, rows.Err()
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	if len(b) == 0 {
		return "[]"
	}
	return string(b)
}
