package inference

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/egress"
	"github.com/MajestaNet/ide/internal/secretcrypt"
)

// InstallConfig is the singleton inference routing row.
type InstallConfig struct {
	ActiveSource           ActiveSource
	DOEnabled              bool
	DOMode                 *Mode
	DefaultProviderAPIName *string
	UpdatedAt              time.Time
}

// Provider is a BYO OpenAI-compatible endpoint.
type Provider struct {
	APIName      string
	Label        string
	BaseURL      string
	SecretRef    *string
	DefaultModel string
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// GetConfig loads the singleton config (inserts default row if missing).
func GetConfig(ctx context.Context, pool *db.Pool) (*InstallConfig, error) {
	_, _ = pool.Exec(ctx, `INSERT INTO install_inference_config (id) VALUES (1) ON CONFLICT (id) DO NOTHING`)
	var c InstallConfig
	var src string
	var mode *string
	err := pool.QueryRow(ctx, `
SELECT active_source, do_enabled, do_mode, default_provider_api_name, updated_at
FROM install_inference_config WHERE id=1`).Scan(
		&src, &c.DOEnabled, &mode, &c.DefaultProviderAPIName, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.ActiveSource = ActiveSource(src)
	if mode != nil && *mode != "" {
		m := Mode(*mode)
		c.DOMode = &m
	}
	return &c, nil
}

// PutDOConfig enables/disables Native DO Inference and optionally sets mode.
// Disabling DO does not wipe a BYO (or none) active_source; if DO was active and a
// default BYO provider is set, source becomes byo, otherwise none.
func PutDOConfig(ctx context.Context, pool *db.Pool, enabled bool, mode Mode) (*InstallConfig, error) {
	if enabled {
		if err := ValidateMode(mode); err != nil {
			return nil, err
		}
	}
	var modeArg *string
	if enabled {
		m := string(mode)
		modeArg = &m
	}
	_, _ = pool.Exec(ctx, `INSERT INTO install_inference_config (id) VALUES (1) ON CONFLICT (id) DO NOTHING`)
	_, err := pool.Exec(ctx, `
UPDATE install_inference_config SET
  do_enabled = $1,
  do_mode = $2,
  active_source = CASE
    WHEN $1 THEN 'digitalocean'
    WHEN active_source = 'digitalocean' AND COALESCE(default_provider_api_name, '') <> '' THEN 'byo'
    WHEN active_source = 'digitalocean' THEN 'none'
    ELSE active_source
  END,
  updated_at = now()
WHERE id = 1`, enabled, modeArg)
	if err != nil {
		return nil, err
	}
	return GetConfig(ctx, pool)
}

// PatchBYOConfig sets active source to byo (or none) and default provider.
func PatchBYOConfig(ctx context.Context, pool *db.Pool, active bool, defaultProvider *string) (*InstallConfig, error) {
	src := SourceNone
	if active {
		src = SourceBYO
		if defaultProvider == nil || strings.TrimSpace(*defaultProvider) == "" {
			return nil, fmt.Errorf("inference: defaultProviderApiName required when activating BYO")
		}
	}
	_, err := pool.Exec(ctx, `
INSERT INTO install_inference_config (id, active_source, default_provider_api_name, updated_at)
VALUES (1, $1, $2, now())
ON CONFLICT (id) DO UPDATE SET
  active_source = EXCLUDED.active_source,
  default_provider_api_name = COALESCE(EXCLUDED.default_provider_api_name, install_inference_config.default_provider_api_name),
  updated_at = now()`, src, defaultProvider)
	if err != nil {
		return nil, err
	}
	return GetConfig(ctx, pool)
}

// ListProviders returns BYO providers (no secrets).
func ListProviders(ctx context.Context, pool *db.Pool) ([]Provider, error) {
	rows, err := pool.Query(ctx, `
SELECT api_name, label, base_url, secret_ref, default_model, active, created_at, updated_at
FROM install_inference_providers ORDER BY api_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Provider
	for rows.Next() {
		var p Provider
		if err := rows.Scan(&p.APIName, &p.Label, &p.BaseURL, &p.SecretRef, &p.DefaultModel, &p.Active, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetProvider loads one provider.
func GetProvider(ctx context.Context, pool *db.Pool, apiName string) (*Provider, error) {
	var p Provider
	err := pool.QueryRow(ctx, `
SELECT api_name, label, base_url, secret_ref, default_model, active, created_at, updated_at
FROM install_inference_providers WHERE api_name=$1`, apiName).Scan(
		&p.APIName, &p.Label, &p.BaseURL, &p.SecretRef, &p.DefaultModel, &p.Active, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpsertProvider inserts or updates a BYO provider.
func UpsertProvider(ctx context.Context, pool *db.Pool, p Provider) error {
	_, err := pool.Exec(ctx, `
INSERT INTO install_inference_providers (api_name, label, base_url, secret_ref, default_model, active, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,now())
ON CONFLICT (api_name) DO UPDATE SET
  label=EXCLUDED.label,
  base_url=EXCLUDED.base_url,
  secret_ref=COALESCE(EXCLUDED.secret_ref, install_inference_providers.secret_ref),
  default_model=EXCLUDED.default_model,
  active=EXCLUDED.active,
  updated_at=now()`,
		p.APIName, p.Label, p.BaseURL, p.SecretRef, p.DefaultModel, p.Active)
	return err
}

// DeleteProvider removes a BYO provider. If that row is the install default,
// null the default and set active_source=none when source was byo (same transaction).
func DeleteProvider(ctx context.Context, pool *db.Pool, apiName string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, _ = tx.Exec(ctx, `INSERT INTO install_inference_config (id) VALUES (1) ON CONFLICT (id) DO NOTHING`)
	var defaultName *string
	var src string
	if err := tx.QueryRow(ctx, `
SELECT default_provider_api_name, active_source
FROM install_inference_config WHERE id=1 FOR UPDATE`).Scan(&defaultName, &src); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM install_inference_providers WHERE api_name=$1`, apiName); err != nil {
		return err
	}
	if defaultName != nil && *defaultName == apiName {
		newSrc := src
		if src == string(SourceBYO) {
			newSrc = string(SourceNone)
		}
		if _, err := tx.Exec(ctx, `
UPDATE install_inference_config
SET active_source = $1, default_provider_api_name = NULL, updated_at = now()
WHERE id = 1`, newSrc); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ResolveOptions supplies install-local credentials for routing.
type ResolveOptions struct {
	DOAPIToken string
	EncKey     string
	// AllowDevLocal permits http://localhost (and host.docker.internal) BYO
	// providers when APP_ENV is not production. Production must leave false.
	AllowDevLocal bool
}

// Resolve picks the active OpenAI-compatible route.
func Resolve(ctx context.Context, pool *db.Pool, opts ResolveOptions) (*Route, error) {
	cfg, err := GetConfig(ctx, pool)
	if err != nil {
		return nil, err
	}
	switch cfg.ActiveSource {
	case SourceDigitalOcean:
		if !cfg.DOEnabled {
			return nil, ErrNotConfigured
		}
		token := strings.TrimSpace(opts.DOAPIToken)
		if token == "" {
			return nil, ErrDOTokenMissing
		}
		mode := ModeDev
		if cfg.DOMode != nil {
			mode = *cfg.DOMode
		}
		model, err := ModelForMode(mode)
		if err != nil {
			return nil, err
		}
		return &Route{
			Source:        SourceDigitalOcean,
			BaseURL:       DOInferenceBaseURL,
			APIKey:        token,
			Model:         model,
			DOMode:        mode,
			BillingNotice: BillingNotice,
			Prepaid:       true,
		}, nil
	case SourceBYO:
		if cfg.DefaultProviderAPIName == nil || *cfg.DefaultProviderAPIName == "" {
			return nil, ErrNotConfigured
		}
		p, err := GetProvider(ctx, pool, *cfg.DefaultProviderAPIName)
		if err != nil || !p.Active {
			return nil, ErrNotConfigured
		}
		if p.SecretRef == nil || *p.SecretRef == "" {
			return nil, fmt.Errorf("inference: provider %q missing secretRef", p.APIName)
		}
		if err := ValidateProviderBaseURL(p.BaseURL, opts.AllowDevLocal); err != nil {
			return nil, fmt.Errorf("inference: provider %q: %w", p.APIName, err)
		}
		host, err := ProviderHost(p.BaseURL)
		if err != nil {
			return nil, err
		}
		// Loopback Ollama in development skips install egress allowlist; all other
		// BYO hosts remain fail-closed on the allowlist.
		if !opts.AllowDevLocal || !egress.IsDevLocalHost(host) {
			allow, err := db.ListEgressHostPatterns(ctx, pool)
			if err != nil {
				return nil, err
			}
			if !egress.HostAllowed(host, allow) {
				return nil, ErrEgressDenied
			}
		}
		ciphertext, err := db.GetInstallSecretCiphertext(ctx, pool, *p.SecretRef)
		if err != nil {
			return nil, fmt.Errorf("inference: secret %q: %w", *p.SecretRef, err)
		}
		plain, err := secretcrypt.Decrypt(ciphertext, opts.EncKey)
		if err != nil {
			return nil, fmt.Errorf("inference: decrypt secret: %w", err)
		}
		return &Route{
			Source:        SourceBYO,
			BaseURL:       strings.TrimRight(p.BaseURL, "/"),
			APIKey:        plain,
			Model:         p.DefaultModel,
			ProviderName:  p.APIName,
			AllowDevLocal: opts.AllowDevLocal,
		}, nil
	default:
		return nil, ErrNotConfigured
	}
}
