package db

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// InstallAuthSettings is the singleton install AuthN configuration (organization_settings).
type InstallAuthSettings struct {
	ClaimedAt            *time.Time
	ClaimTokenHash       string
	OIDCIssuer           string
	OIDCAudience         string
	OIDCJWKSURI          string
	OIDCDisplayName      string
	OIDCClientID         string
	OIDCClientSecretEnc  string
	JITProvisionUsers    bool
	JITDefaultRole       string
	AllowedEmailDomains  []string
	SocialProviders      []string
	PasswordLoginEnabled bool
	RecordSharingEnabled bool
	Provisioning         ProvisioningConfig
}

// ProvisioningConfig is install-local JIT/SCIM defaults and IdP claim maps.
type ProvisioningConfig struct {
	JITDefaultPermissionSetAPINames  []string                   `json:"jitDefaultPermissionSetApiNames,omitempty"`
	JITDefaultDataRoleAPIName        string                     `json:"jitDefaultDataRoleApiName,omitempty"`
	SCIMDefaultRoleAPIName           string                     `json:"scimDefaultRoleApiName,omitempty"`
	SCIMDefaultPermissionSetAPINames []string                   `json:"scimDefaultPermissionSetApiNames,omitempty"`
	SCIMDefaultDataRoleAPIName       string                     `json:"scimDefaultDataRoleApiName,omitempty"`
	ClaimMappings                    []ProvisioningClaimMapping `json:"claimMappings,omitempty"`
}

// ProvisioningClaimMapping maps an IdP claim onto a User field apiName (JIT create only).
type ProvisioningClaimMapping struct {
	Claim        string `json:"claim"`
	FieldAPIName string `json:"fieldApiName"`
}

// InstallAuthStatus is the public, secret-free view of install claim / login options.
type InstallAuthStatus struct {
	Claimed              bool     `json:"claimed"`
	SSOConfigured        bool     `json:"ssoConfigured"`
	PasswordLoginEnabled bool     `json:"passwordLoginEnabled"`
	SocialProviders      []string `json:"socialProviders"`
	IdPDisplayName       string   `json:"idpDisplayName,omitempty"`
	JITEnabled           bool     `json:"jitEnabled"`
}

// InstallAuthStore reads/writes organization_settings AuthN columns.
type InstallAuthStore struct {
	pool *Pool
}

// NewInstallAuthStore constructs an InstallAuthStore.
func NewInstallAuthStore(pool *Pool) *InstallAuthStore {
	return &InstallAuthStore{pool: pool}
}

const installAuthSelect = `
SELECT
  claimed_at,
  COALESCE(claim_token_hash, ''),
  COALESCE(oidc_issuer, ''),
  COALESCE(oidc_audience, ''),
  COALESCE(oidc_jwks_uri, ''),
  COALESCE(oidc_display_name, ''),
  COALESCE(oidc_client_id, ''),
  COALESCE(oidc_client_secret_enc, ''),
  jit_provision_users,
  COALESCE(NULLIF(jit_default_role, ''), 'StandardUser'),
  COALESCE(allowed_email_domains, '{}'),
  COALESCE(social_providers, '{}'),
  password_login_enabled,
  record_sharing_enabled,
  COALESCE(provisioning, '{}'::jsonb)
FROM organization_settings WHERE id = true`

func scanInstallAuth(row pgx.Row) (*InstallAuthSettings, error) {
	var s InstallAuthSettings
	var provisioning []byte
	err := row.Scan(
		&s.ClaimedAt,
		&s.ClaimTokenHash,
		&s.OIDCIssuer,
		&s.OIDCAudience,
		&s.OIDCJWKSURI,
		&s.OIDCDisplayName,
		&s.OIDCClientID,
		&s.OIDCClientSecretEnc,
		&s.JITProvisionUsers,
		&s.JITDefaultRole,
		&s.AllowedEmailDomains,
		&s.SocialProviders,
		&s.PasswordLoginEnabled,
		&s.RecordSharingEnabled,
		&provisioning,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if s.AllowedEmailDomains == nil {
		s.AllowedEmailDomains = []string{}
	}
	if s.SocialProviders == nil {
		s.SocialProviders = []string{}
	}
	s.Provisioning = ParseProvisioningConfig(provisioning)
	return &s, nil
}

// ParseProvisioningConfig unmarshals install provisioning JSON (empty on error).
func ParseProvisioningConfig(raw []byte) ProvisioningConfig {
	var p ProvisioningConfig
	if len(raw) == 0 || string(raw) == "null" {
		return p
	}
	_ = json.Unmarshal(raw, &p)
	return NormalizeProvisioningConfig(p)
}

// NormalizeProvisioningConfig trims names and drops empty claim mappings.
func NormalizeProvisioningConfig(p ProvisioningConfig) ProvisioningConfig {
	p.JITDefaultDataRoleAPIName = strings.TrimSpace(p.JITDefaultDataRoleAPIName)
	p.SCIMDefaultRoleAPIName = strings.TrimSpace(p.SCIMDefaultRoleAPIName)
	p.SCIMDefaultDataRoleAPIName = strings.TrimSpace(p.SCIMDefaultDataRoleAPIName)
	p.JITDefaultPermissionSetAPINames = normalizeAPINameList(p.JITDefaultPermissionSetAPINames)
	p.SCIMDefaultPermissionSetAPINames = normalizeAPINameList(p.SCIMDefaultPermissionSetAPINames)
	if len(p.ClaimMappings) == 0 {
		p.ClaimMappings = nil
		return p
	}
	out := make([]ProvisioningClaimMapping, 0, len(p.ClaimMappings))
	for _, m := range p.ClaimMappings {
		m.Claim = strings.TrimSpace(m.Claim)
		m.FieldAPIName = strings.TrimSpace(m.FieldAPIName)
		if m.Claim == "" || m.FieldAPIName == "" {
			continue
		}
		out = append(out, m)
	}
	if len(out) == 0 {
		p.ClaimMappings = nil
	} else {
		p.ClaimMappings = out
	}
	return p
}

func normalizeAPINameList(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, n := range in {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Get returns the singleton install auth settings.
func (s *InstallAuthStore) Get(ctx context.Context) (*InstallAuthSettings, error) {
	return scanInstallAuth(s.pool.QueryRow(ctx, installAuthSelect))
}

// PublicStatus returns the secret-free install status for GET /auth/v1/install/status.
func (s *InstallAuthSettings) PublicStatus() InstallAuthStatus {
	social := append([]string(nil), s.SocialProviders...)
	if social == nil {
		social = []string{}
	}
	name := strings.TrimSpace(s.OIDCDisplayName)
	if name == "" && strings.TrimSpace(s.OIDCIssuer) != "" {
		name = "SSO"
	}
	return InstallAuthStatus{
		Claimed:              s.ClaimedAt != nil,
		SSOConfigured:        strings.TrimSpace(s.OIDCIssuer) != "" && strings.TrimSpace(s.OIDCAudience) != "",
		PasswordLoginEnabled: s.PasswordLoginEnabled,
		SocialProviders:      social,
		IdPDisplayName:       name,
		JITEnabled:           s.JITProvisionUsers,
	}
}

// SetClaimTokenHash stores the bcrypt hash of the install claim token (unclaimed installs only).
func (s *InstallAuthStore) SetClaimTokenHash(ctx context.Context, hash string) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE organization_settings
SET claim_token_hash = $1
WHERE id = true AND claimed_at IS NULL`, hash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}

// VerifyClaimToken checks plaintext against the stored claim_token_hash.
func (s *InstallAuthStore) VerifyClaimToken(ctx context.Context, plaintext string) (bool, error) {
	st, err := s.Get(ctx)
	if err != nil {
		return false, err
	}
	if st.ClaimedAt != nil {
		return false, ErrConflict
	}
	if st.ClaimTokenHash == "" || plaintext == "" {
		return false, nil
	}
	return bcrypt.CompareHashAndPassword([]byte(st.ClaimTokenHash), []byte(plaintext)) == nil, nil
}

// MarkClaimed sets claimed_at and clears the claim token hash.
func (s *InstallAuthStore) MarkClaimed(ctx context.Context) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE organization_settings
SET claimed_at = now(), claim_token_hash = NULL, password_login_enabled = true
WHERE id = true AND claimed_at IS NULL`)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}

// InstallAuthUpdate is a partial update for Metadata PUT /install/auth.
type InstallAuthUpdate struct {
	OIDCIssuer            *string
	OIDCAudience          *string
	OIDCJWKSURI           *string
	OIDCDisplayName       *string
	OIDCClientID          *string
	OIDCClientSecretEnc   *string // set only when rotating; omit to keep
	ClearOIDCClientSecret bool
	JITProvisionUsers     *bool
	JITDefaultRole        *string
	AllowedEmailDomains   *[]string
	SocialProviders       *[]string
	PasswordLoginEnabled  *bool
	Provisioning          *ProvisioningConfig // nil = keep existing
}

// Update applies customer AuthN settings (requires identity.manage at HTTP layer).
func (s *InstallAuthStore) Update(ctx context.Context, u InstallAuthUpdate) (*InstallAuthSettings, error) {
	cur, err := s.Get(ctx)
	if err != nil {
		return nil, err
	}
	issuer := cur.OIDCIssuer
	audience := cur.OIDCAudience
	jwks := cur.OIDCJWKSURI
	display := cur.OIDCDisplayName
	clientID := cur.OIDCClientID
	secretEnc := cur.OIDCClientSecretEnc
	jit := cur.JITProvisionUsers
	jitRole := cur.JITDefaultRole
	domains := cur.AllowedEmailDomains
	social := cur.SocialProviders
	pw := cur.PasswordLoginEnabled
	prov := cur.Provisioning

	if u.OIDCIssuer != nil {
		issuer = strings.TrimSpace(*u.OIDCIssuer)
	}
	if u.OIDCAudience != nil {
		audience = strings.TrimSpace(*u.OIDCAudience)
	}
	if u.OIDCJWKSURI != nil {
		jwks = strings.TrimSpace(*u.OIDCJWKSURI)
	}
	if u.OIDCDisplayName != nil {
		display = strings.TrimSpace(*u.OIDCDisplayName)
	}
	if u.OIDCClientID != nil {
		clientID = strings.TrimSpace(*u.OIDCClientID)
	}
	if u.ClearOIDCClientSecret {
		secretEnc = ""
	} else if u.OIDCClientSecretEnc != nil {
		secretEnc = *u.OIDCClientSecretEnc
	}
	if u.JITProvisionUsers != nil {
		jit = *u.JITProvisionUsers
	}
	if u.JITDefaultRole != nil {
		jitRole = strings.TrimSpace(*u.JITDefaultRole)
		if jitRole == "" {
			jitRole = "StandardUser"
		}
	}
	if u.AllowedEmailDomains != nil {
		domains = normalizeDomainList(*u.AllowedEmailDomains)
	}
	if u.SocialProviders != nil {
		social = normalizeSocialList(*u.SocialProviders)
	}
	if u.PasswordLoginEnabled != nil {
		pw = *u.PasswordLoginEnabled
	}
	if u.Provisioning != nil {
		prov = NormalizeProvisioningConfig(*u.Provisioning)
	}
	provJSON, err := json.Marshal(prov)
	if err != nil {
		return nil, err
	}

	_, err = s.pool.Exec(ctx, `
UPDATE organization_settings SET
  oidc_issuer = NULLIF($1, ''),
  oidc_audience = NULLIF($2, ''),
  oidc_jwks_uri = NULLIF($3, ''),
  oidc_display_name = NULLIF($4, ''),
  oidc_client_id = NULLIF($5, ''),
  oidc_client_secret_enc = NULLIF($6, ''),
  jit_provision_users = $7,
  jit_default_role = $8,
  allowed_email_domains = $9,
  social_providers = $10,
  password_login_enabled = $11,
  provisioning = $12::jsonb
WHERE id = true`,
		issuer, audience, jwks, display, clientID, secretEnc,
		jit, jitRole, domains, social, pw, provJSON)
	if err != nil {
		return nil, err
	}
	return s.Get(ctx)
}

func normalizeDomainList(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, d := range in {
		d = strings.ToLower(strings.TrimSpace(d))
		d = strings.TrimPrefix(d, "@")
		if d == "" {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	return out
}

func normalizeSocialList(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, p := range in {
		p = strings.ToLower(strings.TrimSpace(p))
		switch p {
		case "google", "apple", "slack", "dev":
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			out = append(out, p)
		}
	}
	return out
}

// HashClaimToken bcrypt-hashes a claim token plaintext.
func HashClaimToken(plaintext string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// GenerateClaimToken returns a URL-safe random claim token.
func GenerateClaimToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate claim token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// SyncInstallClaimToken ensures an unclaimed install has a claim_token_hash.
// When envToken is non-empty, its hash is stored. When empty and !isProduction,
// a token is generated and returned for one-time logging. Production with empty
// envToken leaves hash empty (claim refuses until configured).
func SyncInstallClaimToken(ctx context.Context, pool *Pool, envToken string, isProduction bool) (generatedPlaintext string, err error) {
	if pool == nil {
		return "", nil
	}
	store := NewInstallAuthStore(pool)
	st, err := store.Get(ctx)
	if err != nil {
		return "", err
	}
	if st.ClaimedAt != nil {
		return "", nil
	}
	token := strings.TrimSpace(envToken)
	if token == "" {
		if isProduction {
			return "", nil
		}
		token, err = GenerateClaimToken()
		if err != nil {
			return "", err
		}
		generatedPlaintext = token
	}
	hash, err := HashClaimToken(token)
	if err != nil {
		return "", err
	}
	if err := store.SetClaimTokenHash(ctx, hash); err != nil {
		if errors.Is(err, ErrConflict) {
			return "", nil
		}
		return "", err
	}
	return generatedPlaintext, nil
}

// EffectiveJIT returns JIT settings: DB first, then env fallbacks.
func (s *InstallAuthSettings) EffectiveJIT(envAuto bool, envRole string, envDomains []string) (enabled bool, role string, domains []string) {
	enabled = s.JITProvisionUsers
	role = s.JITDefaultRole
	domains = s.AllowedEmailDomains
	// If DB has never enabled JIT and has no domains configured, fall back to env for lab installs.
	if !s.JITProvisionUsers && len(s.AllowedEmailDomains) == 0 && (strings.TrimSpace(s.OIDCIssuer) == "") {
		enabled = envAuto
		if envRole != "" {
			role = envRole
		}
		domains = envDomains
	}
	if role == "" {
		role = "StandardUser"
	}
	return enabled, role, domains
}

// EffectiveSocialProviders merges DB social_providers with env AUTH_LOGIN_PROVIDERS.
// DB list wins when non-empty; otherwise env is used (lab/dev).
func (s *InstallAuthSettings) EffectiveSocialProviders(envProviders []string) []string {
	if len(s.SocialProviders) > 0 {
		return append([]string(nil), s.SocialProviders...)
	}
	return append([]string(nil), envProviders...)
}
