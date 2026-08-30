package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/MajestaNet/ide/internal/actions"
	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/config"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/deploy"
	"github.com/MajestaNet/ide/internal/logging"
	"github.com/MajestaNet/ide/internal/metadata"
	oneotel "github.com/MajestaNet/ide/internal/otel"
	_ "github.com/MajestaNet/ide/internal/seed"
	"github.com/MajestaNet/ide/internal/version"
	"github.com/MajestaNet/ide/internal/worker"
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
		ServiceName:     firstNonEmpty(cfg.OTELServiceName, "one-worker"),
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
		_ = otelProvider.Shutdown(context.Background())
	}()

	slog.Info("one-worker starting", "version", version.Version, "appEnv", cfg.AppEnv)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.DatabaseURL == "" {
		slog.Error("DATABASE_URL is required for the worker")
		os.Exit(1)
	}

	pool, err := db.ConnectWithOptions(ctx, cfg.DatabaseURL, db.PoolOptions{
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

	meta := metadata.NewService(pool)
	data := dataengine.NewService(pool, meta)
	objectAz := &authz.ObjectAuthz{Store: &db.ObjectPermStore{Pool: pool}}
	fieldAz := &authz.FieldAuthz{Store: &db.FieldPermStore{Pool: pool}}
	automationAz := &authz.AutomationAuthz{Store: &db.AutomationPermStore{Pool: pool}}
	data.ObjectAz = objectAz
	data.AutomationAz = automationAz
	systemAz := &authz.SystemAuthz{Store: &db.AuthzSystemPerms{Store: db.NewSystemPermStore(pool)}}
	recordAccess := db.NewRecordAccessEvaluator(pool)
	actionSvc := actions.New(actions.Options{
		Meta:         meta,
		Data:         data,
		ObjectAz:     objectAz,
		FieldAz:      fieldAz,
		RecordAccess: recordAccess,
	})
	data.Actions = actionSvc
	engine := deploy.NewDeployEngine(pool, meta, data, deploy.Options{
		InstallID:          cfg.InstallID,
		InstallRole:        cfg.InstallRole,
		ProductVersion:     cfg.ProductVersion,
		APIRevisionMin:     cfg.APIRevisionMin,
		APIRevisionCurrent: cfg.APIRevisionCurrent,
		CustomerID:         cfg.CustomerID,
		PeerMode:           deploy.PeerMode(cfg.DeployPeerMode),
		ShareSecret:        cfg.DeployShareSecret,
		RequiredTestSuites: cfg.DeployRequiredTestSuites,
		SyncMaxFiles:       cfg.DeploySyncMaxFiles,
		SyncMaxBytes:       cfg.DeploySyncMaxBytes,
		QueueMax:           cfg.DeployQueueMax,
		JobSlotsDeploy:     cfg.JobSlotsDeploy,
	})

	procOpts := &worker.ProcessOptions{
		WorkerID:               worker.CreateWorkerID("one"),
		LeaseMs:                int64(DefaultLeaseMS),
		JobLimit:               20,
		OutboxLimit:            50,
		WebhookTimeoutMs:       cfg.WebhookTimeoutMs,
		WebhookEncryptionKey:   cfg.WebhookEncryptionKey,
		DigitalOceanAPIToken:   cfg.DigitalOceanAPIToken,
		AllowDevLocalInference: !cfg.IsProduction,
		DeployEngine:           engine,
		DataEngine:             data,
		Metadata:               meta,
		ObjectAz:               objectAz,
		FieldAz:                fieldAz,
		AutomationAz:           automationAz,
		SystemAz:               systemAz,
		RecordAccess:           recordAccess,
		Actions:                actionSvc,
		Retention: &worker.RetentionOptions{
			JobsDays:     cfg.RetentionJobsDays,
			OutboxDays:   cfg.RetentionOutboxDays,
			AuditLogDays: cfg.RetentionAuditLogDays,
			BatchSize:    cfg.RetentionBatchSize,
		},
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go worker.Run(ctx, pool, worker.RunOptions{
		PollMs:         cfg.WorkerPollMS,
		ProcessOptions: procOpts,
	})

	sig := <-stop
	slog.Info("one-worker shutting down", "signal", sig.String())
	cancel()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// DefaultLeaseMS is 5 minutes in milliseconds.
const DefaultLeaseMS = 5 * 60 * 1000
