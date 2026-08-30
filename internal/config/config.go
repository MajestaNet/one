package config

import (
	"fmt"
	"math"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
)

const minProductionSecretBytes = 32

// Config is the Majesta One API/worker environment contract.
type Config struct {
	AppEnv      string
	DatabaseURL string
	Port        int
	// Host is the bind host. Empty, 0.0.0.0, and :: listen dual-stack (`:port`)
	// so http://localhost works from Control IDE (Node/Electron prefers IPv6).
	Host           string
	LogLevel       string
	APIKeysRaw     string
	APIKeyEntries  []authz.APIKeyEntry
	DefaultOwnerID string
	FeatureFlags   []string
	AutoSeed       bool
	// SeedControlIDE seeds the managed Control IDE Connected App when AutoSeed is on (SEED_CONTROL_IDE, default on).
	SeedControlIDE bool
	InstallID      string
	InstallRole    string
	ProductVersion string
	// APIRevisionCurrent / APIRevisionMin advertise the client-pinnable wire window (ADR-025).
	APIRevisionCurrent   int
	APIRevisionMin       int
	CustomerID           string
	DeployPeerMode       string
	DeployShareSecret    string
	CustomerRepoURL      string
	CustomerRepoProvider string
	CustomerRepoRegion   string
	CustomerRepoGitUser  string
	CustomerRepoGitToken string
	// DeployRequiredTestSuites gates non-dry-run promote (comma-separated apiNames).
	DeployRequiredTestSuites []string
	// DigitalOceanApp Platform day-2 management (Deploy API BP-030) — install-local.
	DigitalOceanAPIToken   string
	DigitalOceanAppID      string
	DigitalOceanDatabaseID string
	WorkerPollMS           int
	WebhookTimeoutMs       int
	WebhookEncryptionKey   string
	RequestBodyLimit       int64
	RateLimitPerMinute     int
	// AdmissionClientRPMShare is the fraction of RateLimitPerMinute reserved for the Client lane (BP-033).
	// Clamped to (0,1]; default 0.7. Remainder is shared by metadata/deploy/ops.
	AdmissionClientRPMShare float64
	// DeploySyncMaxFiles is the metadata+src file count at or below which org validate/apply stays synchronous.
	DeploySyncMaxFiles int
	// DeploySyncMaxBytes is the artifact JSON/zip size at or below which org validate/apply stays synchronous.
	DeploySyncMaxBytes int64
	// DeployQueueMax is the pending+running cap for deploy-class jobs (deploy.validate / deploy.apply / customer.test.run).
	DeployQueueMax int
	// JobSlotsDeploy is the concurrent running cap for deploy-class jobs (enforced as queue depth in Phase 1).
	JobSlotsDeploy int
	// AuthTokenRateLimitPerMinute caps /auth/v1/token(+exchange) per client_id / IP.
	AuthTokenRateLimitPerMinute int
	OIDCIssuer                  string
	OIDCAudience                string
	OIDCJWKSURI                 string
	OIDCDefaultScopes           []authz.Scope
	OIDCAutoProvision           bool
	OIDCEnabled                 bool
	PlatformPublicURL           string
	AuthJWTSigningKey           string
	AuthJWTIssuer               string
	AuthJWTTTLSeconds           int
	// AuthRefreshIdleSeconds is the sliding idle TTL for opaque refresh tokens (default 30d).
	AuthRefreshIdleSeconds int
	// AuthRefreshAbsSeconds is the absolute family cap for refresh tokens (default 90d).
	AuthRefreshAbsSeconds int
	// AuthRefreshBytes is raw entropy for opaque refresh tokens (default 32).
	AuthRefreshBytes int
	AuthJWTEnabled   bool
	IsProduction     bool
	// IdentitySync selects the in-process identity write-through backend: off | memory.
	// Cloud adapters (e.g. Cognito) live in community sdk/aws — not the product binary.
	IdentitySync string
	// AdminBreakglassCIDRs is reserved for install exposure policy notes (local edge only).
	AdminBreakglassCIDRs []string

	// Social login broker (ADR-015) — Google / Apple / Slack.
	AuthLoginProviders           []string // google, apple, slack, dev
	AuthGoogleClientID           string
	AuthGoogleClientSecret       string
	AuthAppleClientID            string
	AuthAppleTeamID              string
	AuthAppleKeyID               string
	AuthApplePrivateKey          string // PEM PKCS8 EC private key
	AuthSlackClientID            string
	AuthSlackClientSecret        string
	AuthAutoProvisionUsers       bool
	AuthAutoProvisionRole        string
	AuthLoginAllowedEmailDomains []string
	// InstallClaimToken is the one-time day-0 claim secret (hashed into organization_settings).
	InstallClaimToken string

	// OpenTelemetry (BP-008) — optional OTLP; no-op when Endpoint empty.
	OTELExporterOTLPEndpoint string
	OTELServiceName          string
	OTELTracesExporter       string
	OTELMetricsExporter      string
	// OTELLogsExporter is none (default, even when endpoint is set) or otlp.
	OTELLogsExporter string

	// DB pool sizing (read by internal/db via env; also exposed for docs/tests).
	DBMaxConns int
	DBMinConns int

	// Retention / purge (worker jobs). Zero days disables that path.
	// Soft-delete retention removed in 0037 (hard-delete only).
	RetentionJobsDays     int
	RetentionOutboxDays   int
	RetentionAuditLogDays int
	RetentionBatchSize    int
}

// Load reads configuration from the process environment.
// For local `make api` / `go run`, a repo `.env` is applied first (existing
// exported variables always win).
func Load() (*Config, error) {
	ApplyDotEnv()
	return LoadFromEnv(os.Environ())
}

// ListenAddr is the TCP address passed to net/http.
func (c *Config) ListenAddr() string {
	if c == nil {
		return ":8080"
	}
	return ListenAddr(c.Host, c.Port)
}

// ListenAddr maps HOST+PORT to a net/http bind address.
// Empty HOST, 0.0.0.0, and :: are dual-stack wildcards (`:port`) so IPv4
// 127.0.0.1 and IPv6 ::1 (localhost) both connect. A specific host stays as-is.
func ListenAddr(host string, port int) string {
	if port < 0 || port > 65535 {
		port = 8080
	}
	host = strings.TrimSpace(host)
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		return net.JoinHostPort("", strconv.Itoa(port))
	default:
		return net.JoinHostPort(host, strconv.Itoa(port))
	}
}

// LoadFromEnv parses KEY=VALUE pairs (testable).
func LoadFromEnv(environ []string) (*Config, error) {
	env := map[string]string{}
	for _, e := range environ {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			env[k] = v
		}
	}

	get := func(key, def string) string {
		if v, ok := env[key]; ok && v != "" {
			return v
		}
		return def
	}

	// Prefer APP_ENV; accept ENV and legacy NODE_ENV during transition.
	appEnv := get("APP_ENV", "")
	if appEnv == "" {
		appEnv = get("ENV", "")
	}
	if appEnv == "" {
		appEnv = get("NODE_ENV", "development")
	}
	appEnv = strings.ToLower(strings.TrimSpace(appEnv))

	apiKeysRaw := get("API_KEYS", "dev-admin-key+admin")
	entries, err := authz.ParseAPIKeyEntries(apiKeysRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid API_KEYS: %w", err)
	}

	port := 8080
	if p := get("PORT", "8080"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("invalid PORT: %q", p)
		}
		port = n
	}

	flags := splitCSV(get("FEATURE_FLAGS", ""))

	workerPollMS := 2000
	if v := get("WORKER_POLL_MS", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			workerPollMS = n
		}
	}
	webhookTimeoutMs := 10000
	if v := get("WEBHOOK_TIMEOUT_MS", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			webhookTimeoutMs = n
		}
	}

	bodyLimit := parseByteSize(get("REQUEST_BODY_LIMIT", "1mb"), 1<<20)
	rateLimit := 600
	if v := get("RATE_LIMIT_PER_MINUTE", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			rateLimit = n
		}
	}
	authTokenRateLimit := 30
	if v := get("AUTH_TOKEN_RATE_LIMIT_PER_MINUTE", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			authTokenRateLimit = n
		}
	}
	admissionShare := parseUnitInterval(get("ADMISSION_CLIENT_RPM_SHARE", "0.7"), 0.7)
	deploySyncMaxFiles := parsePositiveInt(get("DEPLOY_SYNC_MAX_FILES", "50"), 50)
	deploySyncMaxBytes := parseByteSize(get("DEPLOY_SYNC_MAX_BYTES", "2097152"), 2097152)
	deployQueueMax := parsePositiveInt(get("DEPLOY_QUEUE_MAX", "8"), 8)
	jobSlotsDeploy := parsePositiveInt(get("JOB_SLOTS_DEPLOY", "1"), 1)

	issuer := strings.TrimSpace(env["OIDC_ISSUER"])
	audience := strings.TrimSpace(env["OIDC_AUDIENCE"])
	jwks := strings.TrimSpace(env["OIDC_JWKS_URI"])

	publicURL := strings.TrimRight(strings.TrimSpace(get("PLATFORM_PUBLIC_URL", "http://localhost:8080")), "/")
	authSigningKey := strings.TrimSpace(env["AUTH_JWT_SIGNING_KEY"])
	authIssuer := strings.TrimSpace(get("AUTH_JWT_ISSUER", ""))
	if authIssuer == "" && publicURL != "" {
		authIssuer = publicURL + "/auth/v1"
	}
	authTTL := 3600
	if v := get("AUTH_JWT_TTL_SECONDS", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			authTTL = n
		}
	}
	authRefreshIdle := parsePositiveInt(get("AUTH_REFRESH_IDLE_SECONDS", "2592000"), 2592000)
	authRefreshAbs := parsePositiveInt(get("AUTH_REFRESH_ABS_SECONDS", "7776000"), 7776000)
	authRefreshBytes := parsePositiveInt(get("AUTH_REFRESH_BYTES", "32"), 32)
	if authRefreshBytes < 16 {
		authRefreshBytes = 32
	}
	webhookEncKey := strings.TrimSpace(get("WEBHOOK_ENCRYPTION_KEY", ""))
	if webhookEncKey == "" {
		webhookEncKey = authSigningKey
	}

	apiRevisionCurrent := parseAPIRevisionCurrent(env, get)
	apiRevisionMin := parseAPIRevisionMin(env, get, apiRevisionCurrent)

	cfg := &Config{
		AppEnv:                       appEnv,
		DatabaseURL:                  strings.TrimSpace(env["DATABASE_URL"]),
		Port:                         port,
		Host:                         get("HOST", "0.0.0.0"),
		LogLevel:                     strings.ToLower(get("LOG_LEVEL", "info")),
		APIKeysRaw:                   apiKeysRaw,
		APIKeyEntries:                entries,
		DefaultOwnerID:               get("DEFAULT_OWNER_ID", "00000000-0000-4000-8000-000000000001"),
		FeatureFlags:                 flags,
		AutoSeed:                     get("AUTO_SEED", "1") != "0",
		SeedControlIDE:               get("SEED_CONTROL_IDE", "1") != "0",
		InstallID:                    get("INSTALL_ID", "local"),
		InstallRole:                  get("INSTALL_ROLE", "dev"),
		ProductVersion:               get("PRODUCT_VERSION", "0.1.0"),
		APIRevisionCurrent:           apiRevisionCurrent,
		APIRevisionMin:               apiRevisionMin,
		CustomerID:                   get("CUSTOMER_ID", "local-customer"),
		DeployPeerMode:               get("DEPLOY_PEER_MODE", "customer"),
		DeployShareSecret:            strings.TrimSpace(env["DEPLOY_SHARE_SECRET"]),
		CustomerRepoURL:              strings.TrimSpace(get("CUSTOMER_REPO_URL", "")),
		CustomerRepoProvider:         strings.TrimSpace(get("CUSTOMER_REPO_PROVIDER", "")),
		CustomerRepoRegion:           strings.TrimSpace(get("CUSTOMER_REPO_REGION", "")),
		CustomerRepoGitUser:          strings.TrimSpace(get("CUSTOMER_REPO_GIT_USER", "")),
		CustomerRepoGitToken:         strings.TrimSpace(env["CUSTOMER_REPO_GIT_TOKEN"]),
		DeployRequiredTestSuites:     splitCSV(get("DEPLOY_REQUIRED_TEST_SUITES", "")),
		DigitalOceanAPIToken:         strings.TrimSpace(env["DIGITALOCEAN_API_TOKEN"]),
		DigitalOceanAppID:            strings.TrimSpace(get("DIGITALOCEAN_APP_ID", "")),
		DigitalOceanDatabaseID:       strings.TrimSpace(get("DIGITALOCEAN_DATABASE_ID", "")),
		WorkerPollMS:                 workerPollMS,
		WebhookTimeoutMs:             webhookTimeoutMs,
		WebhookEncryptionKey:         webhookEncKey,
		RequestBodyLimit:             bodyLimit,
		RateLimitPerMinute:           rateLimit,
		AdmissionClientRPMShare:      admissionShare,
		DeploySyncMaxFiles:           deploySyncMaxFiles,
		DeploySyncMaxBytes:           deploySyncMaxBytes,
		DeployQueueMax:               deployQueueMax,
		JobSlotsDeploy:               jobSlotsDeploy,
		AuthTokenRateLimitPerMinute:  authTokenRateLimit,
		OIDCIssuer:                   issuer,
		OIDCAudience:                 audience,
		OIDCJWKSURI:                  jwks,
		OIDCDefaultScopes:            authz.ParseScopesCSV(get("OIDC_DEFAULT_SCOPES", "client")),
		OIDCAutoProvision:            get("OIDC_AUTO_PROVISION_USERS", "0") != "0",
		OIDCEnabled:                  issuer != "",
		PlatformPublicURL:            publicURL,
		AuthJWTSigningKey:            authSigningKey,
		AuthJWTIssuer:                authIssuer,
		AuthJWTTTLSeconds:            authTTL,
		AuthRefreshIdleSeconds:       authRefreshIdle,
		AuthRefreshAbsSeconds:        authRefreshAbs,
		AuthRefreshBytes:             authRefreshBytes,
		AuthJWTEnabled:               authSigningKey != "" && authIssuer != "",
		IsProduction:                 appEnv == "production",
		IdentitySync:                 strings.ToLower(strings.TrimSpace(get("IDENTITY_SYNC", "off"))),
		AdminBreakglassCIDRs:         splitCSV(get("ADMIN_BREAKGLASS_CIDRS", "")),
		AuthLoginProviders:           normalizeLoginProviders(splitCSV(get("AUTH_LOGIN_PROVIDERS", ""))),
		AuthGoogleClientID:           strings.TrimSpace(get("AUTH_GOOGLE_CLIENT_ID", "")),
		AuthGoogleClientSecret:       strings.TrimSpace(get("AUTH_GOOGLE_CLIENT_SECRET", "")),
		AuthAppleClientID:            strings.TrimSpace(get("AUTH_APPLE_CLIENT_ID", "")),
		AuthAppleTeamID:              strings.TrimSpace(get("AUTH_APPLE_TEAM_ID", "")),
		AuthAppleKeyID:               strings.TrimSpace(get("AUTH_APPLE_KEY_ID", "")),
		AuthApplePrivateKey:          strings.ReplaceAll(get("AUTH_APPLE_PRIVATE_KEY", ""), `\n`, "\n"),
		AuthSlackClientID:            strings.TrimSpace(get("AUTH_SLACK_CLIENT_ID", "")),
		AuthSlackClientSecret:        strings.TrimSpace(get("AUTH_SLACK_CLIENT_SECRET", "")),
		AuthAutoProvisionUsers:       get("AUTH_AUTO_PROVISION_USERS", get("OIDC_AUTO_PROVISION_USERS", "0")) != "0",
		AuthAutoProvisionRole:        get("AUTH_AUTO_PROVISION_ROLE", "StandardUser"),
		AuthLoginAllowedEmailDomains: normalizeEmailDomains(splitCSV(get("AUTH_LOGIN_ALLOWED_EMAIL_DOMAINS", ""))),
		InstallClaimToken:            strings.TrimSpace(get("INSTALL_CLAIM_TOKEN", "")),
		OTELExporterOTLPEndpoint:     strings.TrimSpace(get("OTEL_EXPORTER_OTLP_ENDPOINT", "")),
		OTELServiceName:              strings.TrimSpace(get("OTEL_SERVICE_NAME", "")),
		OTELTracesExporter:           strings.TrimSpace(get("OTEL_TRACES_EXPORTER", "")),
		OTELMetricsExporter:          strings.TrimSpace(get("OTEL_METRICS_EXPORTER", "")),
		OTELLogsExporter:             strings.TrimSpace(get("OTEL_LOGS_EXPORTER", "")),
		DBMaxConns:                   parsePositiveInt(get("DB_MAX_CONNS", "10"), 10),
		DBMinConns:                   parseNonNegInt(get("DB_MIN_CONNS", "1"), 1),
		RetentionJobsDays:            parseNonNegInt(get("RETENTION_JOBS_DAYS", "30"), 30),
		RetentionOutboxDays:          parseNonNegInt(get("RETENTION_OUTBOX_DAYS", "30"), 30),
		RetentionAuditLogDays:        parseNonNegInt(get("RETENTION_AUDIT_LOG_DAYS", "180"), 180),
		RetentionBatchSize:           parsePositiveInt(get("RETENTION_BATCH_SIZE", "5000"), 5000),
	}
	if cfg.DBMinConns > cfg.DBMaxConns {
		cfg.DBMinConns = cfg.DBMaxConns
	}
	if _, _, err := cfg.APIRevisionWindow(); err != nil {
		return nil, err
	}
	// Local default: enable the in-process `dev` login provider so Control IDE can
	// sign in without a Google Cloud OAuth app. Production must set providers explicitly.
	if !cfg.IsProduction && len(cfg.AuthLoginProviders) == 0 {
		cfg.AuthLoginProviders = []string{"dev"}
		if get("AUTH_AUTO_PROVISION_USERS", "") == "" && get("OIDC_AUTO_PROVISION_USERS", "") == "" {
			cfg.AuthAutoProvisionUsers = true
		}
	}

	if cfg.OIDCEnabled && cfg.OIDCAudience == "" {
		return nil, fmt.Errorf("OIDC_AUDIENCE is required when OIDC_ISSUER is set")
	}

	if cfg.IsProduction {
		if cfg.DatabaseURL == "" {
			return nil, fmt.Errorf("DATABASE_URL is required in production")
		}
		if err := validateProductionAPIKeys(cfg); err != nil {
			return nil, err
		}
		if cfg.AuthJWTSigningKey != "" {
			k := strings.ToLower(cfg.AuthJWTSigningKey)
			if strings.HasPrefix(k, "dev-") || k == "change-me" {
				return nil, fmt.Errorf("production AUTH_JWT_SIGNING_KEY must not use a development placeholder")
			}
			if err := validateProductionSecretLength("AUTH_JWT_SIGNING_KEY", cfg.AuthJWTSigningKey); err != nil {
				return nil, err
			}
		}
		if err := validateProductionSecretLength("WEBHOOK_ENCRYPTION_KEY (or AUTH_JWT_SIGNING_KEY fallback)", cfg.WebhookEncryptionKey); err != nil {
			return nil, err
		}
		if cfg.InstallClaimToken == "" {
			// Allowed at config load; claim endpoint refuses until SyncInstallClaimToken has a hash.
			// Operators should set INSTALL_CLAIM_TOKEN for day-0 claim (BP-037).
		} else {
			k := strings.ToLower(cfg.InstallClaimToken)
			if k == "change-me" || strings.HasPrefix(k, "dev-") {
				return nil, fmt.Errorf("production INSTALL_CLAIM_TOKEN must not use a development placeholder")
			}
			if err := validateProductionSecretLength("INSTALL_CLAIM_TOKEN", cfg.InstallClaimToken); err != nil {
				return nil, err
			}
		}
	}
	return cfg, nil
}

func validateProductionAPIKeys(cfg *Config) error {
	if len(cfg.APIKeyEntries) == 0 {
		return fmt.Errorf("API_KEYS is required in production")
	}
	for i, e := range cfg.APIKeyEntries {
		k := strings.ToLower(e.Key)
		if k == "dev-admin-key" || k == "dev-agent-key" || strings.HasPrefix(k, "dev-") {
			return fmt.Errorf("production API_KEYS entry %d must not use a development key", i+1)
		}
		if len([]byte(e.Key)) < minProductionSecretBytes {
			return fmt.Errorf("production API_KEYS entry %d must be at least %d bytes", i+1, minProductionSecretBytes)
		}
	}
	return nil
}

func validateProductionSecretLength(name, value string) error {
	if len([]byte(value)) < minProductionSecretBytes {
		return fmt.Errorf("production %s must be at least %d bytes", name, minProductionSecretBytes)
	}
	return nil
}

func parseByteSize(raw string, def int64) int64 {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return def
	}
	mult := int64(1)
	switch {
	case strings.HasSuffix(raw, "kb"):
		mult = 1024
		raw = strings.TrimSuffix(raw, "kb")
	case strings.HasSuffix(raw, "mb"):
		mult = 1024 * 1024
		raw = strings.TrimSuffix(raw, "mb")
	case strings.HasSuffix(raw, "b"):
		raw = strings.TrimSuffix(raw, "b")
	}
	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || n <= 0 {
		return def
	}
	return n * mult
}

func parseUnitInterval(raw string, def float64) float64 {
	n, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || n <= 0 || n > 1 {
		return def
	}
	return n
}

// ClampAdmissionClientRPMShare returns share in (0,1], defaulting to 0.7.
func ClampAdmissionClientRPMShare(share float64) float64 {
	if share <= 0 || share > 1 {
		return 0.7
	}
	return share
}

// AdmissionLaneLimits splits RATE_LIMIT_PER_MINUTE into the Client lane and
// the shared remainder pool used by metadata, deploy, and ops.
func AdmissionLaneLimits(ratePerMinute int, share float64) (client, remainder int) {
	if ratePerMinute <= 0 {
		return 0, 0
	}
	share = ClampAdmissionClientRPMShare(share)
	client = int(math.Ceil(float64(ratePerMinute) * share))
	if client < 1 {
		client = 1
	}
	if client > ratePerMinute {
		client = ratePerMinute
	}
	return client, ratePerMinute - client
}

func parsePositiveInt(raw string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func parseNonNegInt(raw string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 0 {
		return def
	}
	return n
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseAPIRevisionCurrent(env map[string]string, get func(key, def string) string) int {
	if v := strings.TrimSpace(get("API_REVISION_CURRENT", "")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			return n
		}
	}
	if v := strings.TrimSpace(env["API_REVISION"]); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			return n
		}
	}
	return 1
}

func parseAPIRevisionMin(env map[string]string, get func(key, def string) string, current int) int {
	if v := strings.TrimSpace(get("API_REVISION_MIN", "")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			return n
		}
	}
	return current
}

// APIRevisionWindow returns the normalized install revision window from config.
func (c *Config) APIRevisionWindow() (min, current int, err error) {
	if c == nil {
		return 0, 0, fmt.Errorf("config is nil")
	}
	current = c.APIRevisionCurrent
	if current < 1 {
		current = 1
	}
	min = c.APIRevisionMin
	if min < 1 {
		min = current
	}
	if min > current {
		return 0, 0, fmt.Errorf("API_REVISION_MIN (%d) cannot exceed API_REVISION_CURRENT (%d)", min, current)
	}
	return min, current, nil
}

func normalizeLoginProviders(in []string) []string {
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

func normalizeEmailDomains(in []string) []string {
	out := make([]string, 0, len(in))
	for _, d := range in {
		d = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(d, "@")))
		if d != "" {
			out = append(out, d)
		}
	}
	return out
}

// LoginProviderEnabled reports whether the social broker enables provider.
func (c *Config) LoginProviderEnabled(name string) bool {
	if c == nil {
		return false
	}
	name = strings.ToLower(strings.TrimSpace(name))
	for _, p := range c.AuthLoginProviders {
		if p == name {
			return true
		}
	}
	return false
}

// AgentsEnabled reports whether the agents feature (MCP gateway, agent SKU) is on.
// Empty FEATURE_FLAGS matches seed default (agents on for local/dev).
// Marketplace installs omit "agents" from FEATURE_FLAGS to keep MCP dark.
func (c *Config) AgentsEnabled() bool {
	if c == nil {
		return false
	}
	if len(c.FeatureFlags) == 0 {
		return true
	}
	for _, f := range c.FeatureFlags {
		if f == "agents" {
			return true
		}
	}
	return false
}
