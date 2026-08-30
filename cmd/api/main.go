package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/config"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/deploy"
	"github.com/MajestaNet/ide/internal/digitalocean"
	"github.com/MajestaNet/ide/internal/edge"
	"github.com/MajestaNet/ide/internal/httpapi"
	"github.com/MajestaNet/ide/internal/identity"
	"github.com/MajestaNet/ide/internal/logging"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/ops"
	oneotel "github.com/MajestaNet/ide/internal/otel"
	"github.com/MajestaNet/ide/internal/seed"
	"github.com/MajestaNet/ide/internal/version"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	logging.Setup(cfg.LogLevel)

	otelCtx := context.Background()
	otelProvider, err := oneotel.Setup(otelCtx, oneotel.Options{
		Endpoint:        cfg.OTELExporterOTLPEndpoint,
		ServiceName:     firstNonEmpty(cfg.OTELServiceName, "one-api"),
		ServiceVersion:  cfg.ProductVersion,
		CustomerID:      cfg.CustomerID,
		InstallID:       cfg.InstallID,
		InstallRole:     cfg.InstallRole,
		TracesExporter:  cfg.OTELTracesExporter,
		MetricsExporter: cfg.OTELMetricsExporter,
		LogsExporter:    cfg.OTELLogsExporter,
	})
	if err != nil {
		log.Fatalf("otel: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = otelProvider.Shutdown(shutdownCtx)
	}()

	ctx := context.Background()
	var pool *db.Pool
	var users authz.UserRepository
	var meta *metadata.Service
	var dataEng *dataengine.Service
	var deployEng *deploy.DeployEngine
	var objectAz *authz.ObjectAuthz
	var fieldAz *authz.FieldAuthz
	var systemAz *authz.SystemAuthz
	var automationAz *authz.AutomationAuthz
	var toolAz *authz.ToolAuthz
	var recordAccess *authz.RecordAccessEvaluator
	var idBackend identity.Backend = identity.NopBackend{}

	if cfg.DatabaseURL != "" {
		pool, err = db.ConnectWithOptions(ctx, cfg.DatabaseURL, db.PoolOptions{
			MaxConns: int32(cfg.DBMaxConns),
			MinConns: int32(cfg.DBMinConns),
		})
		if err != nil {
			slog.Error("db connect failed", "err", err)
			os.Exit(1)
		}
		defer pool.Close()

		if err := pool.EnsureKernel(ctx); err != nil {
			slog.Error("migrate failed", "err", err)
			os.Exit(1)
		}
		slog.Info("kernel migrations applied")

		idBackend = identity.NewBackendFromConfig(cfg.IdentitySync)
		meta = metadata.NewService(pool)

		store := db.NewUserStore(pool)
		users = &db.AuthzUsers{Store: store}
		dataEng = dataengine.NewService(pool, meta)
		objectAz = &authz.ObjectAuthz{Store: &db.ObjectPermStore{Pool: pool}}
		fieldAz = &authz.FieldAuthz{Store: &db.FieldPermStore{Pool: pool}}
		systemAz = &authz.SystemAuthz{Store: &db.AuthzSystemPerms{Store: db.NewSystemPermStore(pool)}}
		automationAz = &authz.AutomationAuthz{Store: &db.AutomationPermStore{Pool: pool}}
		toolAz = &authz.ToolAuthz{Store: &db.ToolPermStore{Pool: pool}}
		dataEng.ObjectAz = objectAz
		dataEng.AutomationAz = automationAz
		recordAccess = db.NewRecordAccessEvaluator(pool)
		deployEng = deploy.NewDeployEngine(pool, meta, dataEng, deploy.Options{
			InstallID:            cfg.InstallID,
			InstallRole:          cfg.InstallRole,
			ProductVersion:       cfg.ProductVersion,
			APIRevisionMin:       cfg.APIRevisionMin,
			APIRevisionCurrent:   cfg.APIRevisionCurrent,
			CustomerID:           cfg.CustomerID,
			PeerMode:             deploy.PeerMode(cfg.DeployPeerMode),
			ShareSecret:          cfg.DeployShareSecret,
			CustomerRepoURL:      cfg.CustomerRepoURL,
			CustomerRepoProvider: cfg.CustomerRepoProvider,
			CustomerRepoRegion:   cfg.CustomerRepoRegion,
			CustomerRepoGitUser:  cfg.CustomerRepoGitUser,
			CustomerRepoGitToken: cfg.CustomerRepoGitToken,
			RequiredTestSuites:   cfg.DeployRequiredTestSuites,
			SyncMaxFiles:         cfg.DeploySyncMaxFiles,
			SyncMaxBytes:         cfg.DeploySyncMaxBytes,
			QueueMax:             cfg.DeployQueueMax,
			JobSlotsDeploy:       cfg.JobSlotsDeploy,
		})
		if cfg.DigitalOceanAPIToken != "" {
			doClient := digitalocean.NewClient(cfg.DigitalOceanAPIToken)
			deployEng.SetCloudClient(doClient)
			if cfg.DigitalOceanAppID != "" || cfg.DigitalOceanDatabaseID != "" {
				if _, err := deployEng.PutDigitalOceanBinding(ctx, cfg.DigitalOceanAppID, cfg.DigitalOceanDatabaseID, "", ""); err != nil {
					slog.Warn("digitalocean binding from env failed", "err", err)
				}
			}
		}
	} else if cfg.IsProduction {
		slog.Error("DATABASE_URL is required in production")
		os.Exit(1)
	}

	var oidc *authz.OIDCVerifier
	if cfg.OIDCEnabled {
		oidc = authz.NewOIDCVerifier(cfg.OIDCIssuer, cfg.OIDCAudience, cfg.OIDCJWKSURI, cfg.OIDCDefaultScopes)
	}
	var one *authz.OneSigner
	if cfg.AuthJWTEnabled {
		one = &authz.OneSigner{
			SigningKey: []byte(cfg.AuthJWTSigningKey),
			Issuer:     cfg.AuthJWTIssuer,
			TTL:        time.Duration(cfg.AuthJWTTTLSeconds) * time.Second,
		}
	}
	var credentials authz.CredentialRepository
	if pool != nil {
		credentials = &db.AuthzCredentials{Store: db.NewCredentialStore(pool)}
	}
	resolver := &authz.Resolver{
		Entries:        cfg.APIKeyEntries,
		DefaultOwnerID: cfg.DefaultOwnerID,
		One:            one,
		OIDC:           oidc,
		OIDCDefault:    cfg.OIDCDefaultScopes,
		AutoProvision:  cfg.OIDCAutoProvision,
		Users:          users,
		Credentials:    credentials,
	}

	var opsEng *ops.Engine
	if pool != nil && deployEng != nil {
		// Product Ops roller is local/memory. ECS rolls live in community sdk/aws/ops.
		opsEng = ops.NewEngine(pool, deployEng, ops.Options{
			ProductVersion: cfg.ProductVersion,
			PublicURL:      cfg.PlatformPublicURL,
			Roller:         &ops.MemoryRoller{},
			Health:         ops.HTTPHealthChecker{BaseURL: cfg.PlatformPublicURL},
		})
	}

	// Product edge roller is local/memory. WAFv2 reconcile lives in community sdk/aws/edge.
	edgeRoller := edge.Roller(&edge.MemoryRoller{})

	opts := httpapi.Options{
		Config:       cfg,
		Resolver:     resolver,
		Pool:         pool,
		Metadata:     meta,
		DataEngine:   dataEng,
		Deploy:       deployEng,
		Ops:          opsEng,
		ObjectAz:     objectAz,
		FieldAz:      fieldAz,
		SystemAz:     systemAz,
		AutomationAz: automationAz,
		ToolAz:       toolAz,
		RecordAccess: recordAccess,
		EdgeRoller:   edgeRoller,
		Identity:     idBackend,
	}
	if pool != nil {
		opts.DB = pool
	}
	srv := httpapi.New(opts)
	if pool != nil {
		// Bind immediately after migrate so localhost /healthz works while seed runs.
		srv.SetReady(false)
	}

	addr := cfg.ListenAddr()
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("listen failed", "err", err)
		os.Exit(1)
	}
	slog.Info("one-api listening", "version", version.Version, "addr", addr, "appEnv", cfg.AppEnv, "ready", srv.Ready())
	go func() {
		if err := httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("serve failed", "err", err)
			os.Exit(1)
		}
	}()

	if pool != nil && meta != nil {
		slog.Info("bootstrap/seed starting", "autoSeed", cfg.AutoSeed)
		if err := seed.Bootstrap(ctx, pool, meta, seed.Options{
			OwnerID:        cfg.DefaultOwnerID,
			FeatureFlags:   cfg.FeatureFlags,
			AutoSeed:       cfg.AutoSeed,
			SkipControlIDE: !cfg.SeedControlIDE,
			Identity:       idBackend,
			EncryptionKey:  cfg.WebhookEncryptionKey,
			IdentityIssuer: cfg.OIDCIssuer, // optional identity-link issuer when an adapter is enabled
		}); err != nil {
			slog.Error("seed failed", "err", err)
			os.Exit(1)
		}
		if generated, err := db.SyncInstallClaimToken(ctx, pool, cfg.InstallClaimToken, cfg.IsProduction); err != nil {
			slog.Warn("install claim token sync failed", "err", err)
		} else if generated != "" {
			slog.Warn("generated INSTALL_CLAIM_TOKEN for local claim (store securely; not shown again)", "token", generated)
		} else if cfg.IsProduction && cfg.InstallClaimToken == "" {
			slog.Warn("INSTALL_CLAIM_TOKEN unset; day-0 claim will fail until configured")
		}
		srv.SetReady(true)
		slog.Info("bootstrap/seed complete", "autoSeed", cfg.AutoSeed)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
