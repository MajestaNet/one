package config_test

import (
	"testing"

	"github.com/MajestaNet/ide/internal/config"
)

const (
	strongAPIKey        = "prod-api-key-0123456789abcdef-0123456789abcdef"
	strongEncryptionKey = "prod-encryption-key-0123456789abcdef-0123456789abcdef"
)

func TestLoadFromEnv(t *testing.T) {
	cfg, err := config.LoadFromEnv([]string{
		"APP_ENV=development",
		"API_KEYS=ops:client+metadata+deploy+admin,agent:client",
		"OIDC_ISSUER=https://cognito-idp.us-east-1.amazonaws.com/us-east-1_Example",
		"OIDC_AUDIENCE=app",
		"OIDC_DEFAULT_SCOPES=client",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.OIDCEnabled {
		t.Fatal("expected oidc enabled")
	}
	if cfg.OIDCJWKSURI != "" {
		t.Fatalf("expected empty OIDC_JWKS_URI so discovery supplies jwks_uri, got %q", cfg.OIDCJWKSURI)
	}
	if len(cfg.APIKeyEntries) != 2 {
		t.Fatalf("keys=%d", len(cfg.APIKeyEntries))
	}
	if len(cfg.AuthLoginProviders) != 1 || cfg.AuthLoginProviders[0] != "dev" {
		t.Fatalf("expected default dev login provider, got %#v", cfg.AuthLoginProviders)
	}
	if !cfg.AuthAutoProvisionUsers {
		t.Fatal("expected auto-provision on for default local dev login")
	}
	if cfg.RequestBodyLimit != 1<<20 {
		t.Fatalf("body limit=%d", cfg.RequestBodyLimit)
	}
	if cfg.APIRevisionCurrent != 1 || cfg.APIRevisionMin != 1 {
		t.Fatalf("default api revision window current=%d min=%d", cfg.APIRevisionCurrent, cfg.APIRevisionMin)
	}
	if !cfg.AutoSeed || !cfg.SeedControlIDE {
		t.Fatalf("expected AUTO_SEED and SEED_CONTROL_IDE on by default, auto=%v seedIDE=%v", cfg.AutoSeed, cfg.SeedControlIDE)
	}
	if cfg.OTELLogsExporter != "" {
		t.Fatalf("expected default OTEL_LOGS_EXPORTER empty (none), got %q", cfg.OTELLogsExporter)
	}
	if cfg.AdmissionClientRPMShare != 0.7 {
		t.Fatalf("admission share=%v", cfg.AdmissionClientRPMShare)
	}
	if cfg.DeploySyncMaxFiles != 50 || cfg.DeploySyncMaxBytes != 2097152 {
		t.Fatalf("deploy sync gate files=%d bytes=%d", cfg.DeploySyncMaxFiles, cfg.DeploySyncMaxBytes)
	}
	if cfg.DeployQueueMax != 8 || cfg.JobSlotsDeploy != 1 {
		t.Fatalf("deploy queue=%d slots=%d", cfg.DeployQueueMax, cfg.JobSlotsDeploy)
	}
}

func TestLoadAdmissionAndDeployIsolationKnobs(t *testing.T) {
	cfg, err := config.LoadFromEnv([]string{
		"APP_ENV=development",
		"API_KEYS=ops:client+admin",
		"ADMISSION_CLIENT_RPM_SHARE=0.8",
		"DEPLOY_SYNC_MAX_FILES=10",
		"DEPLOY_SYNC_MAX_BYTES=4096",
		"DEPLOY_QUEUE_MAX=3",
		"JOB_SLOTS_DEPLOY=2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AdmissionClientRPMShare != 0.8 {
		t.Fatalf("share=%v", cfg.AdmissionClientRPMShare)
	}
	if cfg.DeploySyncMaxFiles != 10 || cfg.DeploySyncMaxBytes != 4096 {
		t.Fatalf("gate files=%d bytes=%d", cfg.DeploySyncMaxFiles, cfg.DeploySyncMaxBytes)
	}
	if cfg.DeployQueueMax != 3 || cfg.JobSlotsDeploy != 2 {
		t.Fatalf("queue=%d slots=%d", cfg.DeployQueueMax, cfg.JobSlotsDeploy)
	}

	cfg, err = config.LoadFromEnv([]string{
		"APP_ENV=development",
		"API_KEYS=ops:client+admin",
		"ADMISSION_CLIENT_RPM_SHARE=0",
		"ADMISSION_CLIENT_RPM_SHARE=1.5",
	})
	if err != nil {
		t.Fatal(err)
	}
	// last value in environ map wins; 1.5 is clamped to default
	if cfg.AdmissionClientRPMShare != 0.7 {
		t.Fatalf("invalid share should default, got %v", cfg.AdmissionClientRPMShare)
	}
}

func TestAdmissionLaneLimits(t *testing.T) {
	client, remainder := config.AdmissionLaneLimits(600, 0.7)
	if client != 420 || remainder != 180 {
		t.Fatalf("got client=%d remainder=%d", client, remainder)
	}
	client, remainder = config.AdmissionLaneLimits(10, 0.7)
	if client != 7 || remainder != 3 {
		t.Fatalf("got client=%d remainder=%d", client, remainder)
	}
	client, remainder = config.AdmissionLaneLimits(0, 0.7)
	if client != 0 || remainder != 0 {
		t.Fatalf("unlimited rate should yield zero lanes, got %d/%d", client, remainder)
	}
}

func TestLoadSeedControlIDEOff(t *testing.T) {
	cfg, err := config.LoadFromEnv([]string{
		"APP_ENV=development",
		"API_KEYS=ops:client+admin",
		"AUTO_SEED=1",
		"SEED_CONTROL_IDE=0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AutoSeed {
		t.Fatal("expected AUTO_SEED on")
	}
	if cfg.SeedControlIDE {
		t.Fatal("expected SEED_CONTROL_IDE off")
	}
}

func TestLoadAPIRevisionWindow(t *testing.T) {
	cfg, err := config.LoadFromEnv([]string{
		"APP_ENV=development",
		"API_KEYS=ops:client+admin",
		"API_REVISION_CURRENT=14",
		"API_REVISION_MIN=12",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIRevisionCurrent != 14 || cfg.APIRevisionMin != 12 {
		t.Fatalf("window current=%d min=%d", cfg.APIRevisionCurrent, cfg.APIRevisionMin)
	}

	cfg, err = config.LoadFromEnv([]string{
		"APP_ENV=development",
		"API_KEYS=ops:client+admin",
		"API_REVISION=9",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIRevisionCurrent != 9 || cfg.APIRevisionMin != 9 {
		t.Fatalf("API_REVISION fallback current=%d min=%d", cfg.APIRevisionCurrent, cfg.APIRevisionMin)
	}

	_, err = config.LoadFromEnv([]string{
		"APP_ENV=development",
		"API_KEYS=ops:client+admin",
		"API_REVISION_CURRENT=10",
		"API_REVISION_MIN=12",
	})
	if err == nil {
		t.Fatal("expected min>current error")
	}
}

func TestProductionRequiresDatabase(t *testing.T) {
	_, err := config.LoadFromEnv([]string{
		"APP_ENV=production",
		"API_KEYS=" + strongAPIKey + "+admin",
		"WEBHOOK_ENCRYPTION_KEY=" + strongEncryptionKey,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProductionRejectsDevKeys(t *testing.T) {
	_, err := config.LoadFromEnv([]string{
		"APP_ENV=production",
		"DATABASE_URL=postgres://x",
		"API_KEYS=dev-admin-key+admin",
	})
	if err == nil {
		t.Fatal("expected error for dev keys in production")
	}
}

func TestLoadAuthJWTConfig(t *testing.T) {
	cfg, err := config.LoadFromEnv([]string{
		"APP_ENV=development",
		"API_KEYS=ops:client+admin",
		"PLATFORM_PUBLIC_URL=https://acme.example",
		"AUTH_JWT_SIGNING_KEY=super-secret-signing-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AuthJWTEnabled {
		t.Fatal("expected auth jwt enabled")
	}
	if cfg.AuthJWTIssuer != "https://acme.example/auth/v1" {
		t.Fatalf("issuer=%s", cfg.AuthJWTIssuer)
	}
	if cfg.AuthJWTTTLSeconds != 3600 {
		t.Fatalf("ttl=%d", cfg.AuthJWTTTLSeconds)
	}
	if cfg.AuthRefreshIdleSeconds != 2592000 {
		t.Fatalf("refresh idle=%d", cfg.AuthRefreshIdleSeconds)
	}
	if cfg.AuthRefreshAbsSeconds != 7776000 {
		t.Fatalf("refresh abs=%d", cfg.AuthRefreshAbsSeconds)
	}
	if cfg.AuthRefreshBytes != 32 {
		t.Fatalf("refresh bytes=%d", cfg.AuthRefreshBytes)
	}
}

func TestProductionRejectsDevAuthJWTKey(t *testing.T) {
	_, err := config.LoadFromEnv([]string{
		"APP_ENV=production",
		"DATABASE_URL=postgres://x",
		"API_KEYS=" + strongAPIKey + "+admin",
		"AUTH_JWT_SIGNING_KEY=dev-one-jwt-hmac-secret-change-me",
	})
	if err == nil {
		t.Fatal("expected error for dev AUTH_JWT_SIGNING_KEY in production")
	}
}

func TestProductionAllowsWithoutInboundDeployTrust(t *testing.T) {
	// Inbound artifact promote / peer push removed — share secret + allowlist are optional.
	cfg, err := config.LoadFromEnv([]string{
		"APP_ENV=production",
		"DATABASE_URL=postgres://x",
		"API_KEYS=" + strongAPIKey + "+admin",
		"WEBHOOK_ENCRYPTION_KEY=" + strongEncryptionKey,
		"DEPLOY_PEER_MODE=customer",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DeployPeerMode != "customer" {
		t.Fatalf("peerMode=%s", cfg.DeployPeerMode)
	}
}

func TestProductionRejectsWeakAPIKey(t *testing.T) {
	_, err := config.LoadFromEnv([]string{
		"APP_ENV=production",
		"DATABASE_URL=postgres://x",
		"API_KEYS=short-prod-key+admin",
		"WEBHOOK_ENCRYPTION_KEY=" + strongEncryptionKey,
	})
	if err == nil {
		t.Fatal("expected weak API key to be rejected")
	}
}

func TestProductionRejectsWeakJWTSigningKey(t *testing.T) {
	_, err := config.LoadFromEnv([]string{
		"APP_ENV=production",
		"DATABASE_URL=postgres://x",
		"API_KEYS=" + strongAPIKey + "+admin",
		"AUTH_JWT_SIGNING_KEY=short-signing-key",
		"WEBHOOK_ENCRYPTION_KEY=" + strongEncryptionKey,
	})
	if err == nil {
		t.Fatal("expected weak JWT signing key to be rejected")
	}
}

func TestProductionRequiresStrongEncryptionKey(t *testing.T) {
	_, err := config.LoadFromEnv([]string{
		"APP_ENV=production",
		"DATABASE_URL=postgres://x",
		"API_KEYS=" + strongAPIKey + "+admin",
	})
	if err == nil {
		t.Fatal("expected missing at-rest encryption key to be rejected")
	}
}

func TestProductionRejectsWeakInstallClaimToken(t *testing.T) {
	_, err := config.LoadFromEnv([]string{
		"APP_ENV=production",
		"DATABASE_URL=postgres://x",
		"API_KEYS=" + strongAPIKey + "+admin",
		"WEBHOOK_ENCRYPTION_KEY=" + strongEncryptionKey,
		"INSTALL_CLAIM_TOKEN=short-claim-token",
	})
	if err == nil {
		t.Fatal("expected weak install claim token to be rejected")
	}
}

func TestLoadSlackLoginConfig(t *testing.T) {
	cfg, err := config.LoadFromEnv([]string{
		"APP_ENV=development",
		"API_KEYS=ops:client+admin",
		"AUTH_LOGIN_PROVIDERS=slack",
		"AUTH_SLACK_CLIENT_ID=slack-client",
		"AUTH_SLACK_CLIENT_SECRET=slack-secret",
		"OIDC_JWKS_URI=https://login.microsoftonline.com/tid/discovery/v2.0/keys",
		"OIDC_ISSUER=https://login.microsoftonline.com/tid/v2.0",
		"OIDC_AUDIENCE=app",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.LoginProviderEnabled("slack") {
		t.Fatal("expected slack login provider enabled")
	}
	if cfg.AuthSlackClientID != "slack-client" || cfg.AuthSlackClientSecret != "slack-secret" {
		t.Fatalf("slack credentials: id=%q secret=%q", cfg.AuthSlackClientID, cfg.AuthSlackClientSecret)
	}
	if cfg.OIDCJWKSURI != "https://login.microsoftonline.com/tid/discovery/v2.0/keys" {
		t.Fatalf("explicit JWKS URI not preserved: %q", cfg.OIDCJWKSURI)
	}
}

func TestOIDCRequiresAudience(t *testing.T) {
	_, err := config.LoadFromEnv([]string{
		"APP_ENV=development",
		"API_KEYS=ops:client+admin",
		"OIDC_ISSUER=https://cognito-idp.us-east-1.amazonaws.com/us-east-1_Example",
	})
	if err == nil {
		t.Fatal("expected error when OIDC_AUDIENCE missing")
	}
}

func TestOIDCAutoProvisionDefaultsOff(t *testing.T) {
	cfg, err := config.LoadFromEnv([]string{
		"APP_ENV=development",
		"API_KEYS=ops:client+admin",
		"OIDC_ISSUER=https://cognito-idp.us-east-1.amazonaws.com/us-east-1_Example",
		"OIDC_AUDIENCE=app",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OIDCAutoProvision {
		t.Fatal("expected OIDC auto-provision off by default")
	}
}

func TestLoadOTELLogsExporter(t *testing.T) {
	cfg, err := config.LoadFromEnv([]string{
		"APP_ENV=development",
		"API_KEYS=ops:client+admin",
		"OTEL_LOGS_EXPORTER=otlp",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OTELLogsExporter != "otlp" {
		t.Fatalf("OTELLogsExporter=%q", cfg.OTELLogsExporter)
	}
}
