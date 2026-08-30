package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/actions"
	"github.com/MajestaNet/ide/internal/authlogin"
	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/config"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/deploy"
	"github.com/MajestaNet/ide/internal/edge"
	"github.com/MajestaNet/ide/internal/httpapi"
	"github.com/MajestaNet/ide/internal/identity"
)

// ServerOptions configures NewTestServer.
type ServerOptions struct {
	// APIKeys is a comma-separated API_KEYS string (required).
	APIKeys string
	// EnableJWT wires OneSigner + AuthJWT* on config (needed for /auth/v1/token).
	EnableJWT bool
	// JWTSigningKey defaults to a fixed test key when EnableJWT is true.
	JWTSigningKey string
	// JWTIssuer defaults to http://localhost:8080/auth/v1.
	JWTIssuer string
	// EdgeRoller optional; principals exposure tests pass a MemoryRoller.
	EdgeRoller edge.Roller
	// Identity optional; defaults to Nop when nil (httpapi.New also Nops).
	Identity identity.Backend
	// LoginBroker optional social login broker (ADR-015).
	LoginBroker *authlogin.Broker
	// CustomerID / InstallID / InstallRole override defaults t1 / i1 / dev.
	CustomerID  string
	InstallID   string
	InstallRole string
	// Deploy isolation knobs (BP-033 Phase 1); zero uses engine defaults.
	DeploySyncMaxFiles int
	DeploySyncMaxBytes int64
	DeployQueueMax     int
	JobSlotsDeploy     int
}

// TestServer is a wired httpapi.Handler plus commonly needed deps.
type TestServer struct {
	Handler  http.Handler
	Config   *config.Config
	Pool     *db.Pool
	Resolver *authz.Resolver
	Deploy   *deploy.DeployEngine
	Data     *dataengine.Service
}

// MustKeys parses an API_KEYS string or fails the test.
func MustKeys(t *testing.T, raw string) []authz.APIKeyEntry {
	t.Helper()
	e, err := authz.ParseAPIKeyEntries(raw)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// NewTestConfig builds a minimal config suitable for httptest integration tests.
func NewTestConfig(t *testing.T, opts ServerOptions) *config.Config {
	t.Helper()
	if opts.APIKeys == "" {
		t.Fatal("NewTestConfig: APIKeys required")
	}
	customer := opts.CustomerID
	if customer == "" {
		customer = "t1"
	}
	install := opts.InstallID
	if install == "" {
		install = "i1"
	}
	role := opts.InstallRole
	if role == "" {
		role = "dev"
	}
	cfg := &config.Config{
		Port:               8080,
		ProductVersion:     "0.1.0",
		CustomerID:         customer,
		InstallID:          install,
		InstallRole:        role,
		APIKeyEntries:      MustKeys(t, opts.APIKeys),
		DefaultOwnerID:     DefaultOwnerID,
		RequestBodyLimit:   1 << 20,
		RateLimitPerMinute: 0,
		DeployPeerMode:     "customer",
	}
	if opts.EnableJWT {
		key := opts.JWTSigningKey
		if key == "" {
			key = "test-signing-key-32bytes-minimum!!"
		}
		issuer := opts.JWTIssuer
		if issuer == "" {
			issuer = "http://localhost:8080/auth/v1"
		}
		cfg.AuthJWTSigningKey = key
		cfg.AuthJWTIssuer = issuer
		cfg.AuthJWTTTLSeconds = 3600
		cfg.AuthJWTEnabled = true
		cfg.AuthRefreshIdleSeconds = 2592000
		cfg.AuthRefreshAbsSeconds = 7776000
		cfg.AuthRefreshBytes = 32
	}
	return cfg
}

// NewTestServer wires metadata, dataengine, deploy, authz, and httpapi against d.
func NewTestServer(t *testing.T, d *Database, opts ServerOptions) *TestServer {
	t.Helper()
	if d == nil || d.Pool == nil || d.Meta == nil {
		t.Fatal("NewTestServer: nil database")
	}
	cfg := NewTestConfig(t, opts)
	userStore := db.NewUserStore(d.Pool)
	resolver := &authz.Resolver{
		Entries:        cfg.APIKeyEntries,
		DefaultOwnerID: cfg.DefaultOwnerID,
		Users:          &db.AuthzUsers{Store: userStore},
	}
	if opts.EnableJWT {
		credStore := db.NewCredentialStore(d.Pool)
		resolver.One = &authz.OneSigner{
			SigningKey: []byte(cfg.AuthJWTSigningKey),
			Issuer:     cfg.AuthJWTIssuer,
			TTL:        time.Duration(cfg.AuthJWTTTLSeconds) * time.Second,
		}
		resolver.Credentials = &db.AuthzCredentials{Store: credStore}
	}
	dataEng := dataengine.NewService(d.Pool, d.Meta)
	objectAz := &authz.ObjectAuthz{Store: &db.ObjectPermStore{Pool: d.Pool}}
	fieldAz := &authz.FieldAuthz{Store: &db.FieldPermStore{Pool: d.Pool}}
	recordAccess := db.NewRecordAccessEvaluator(d.Pool)
	systemAz := &authz.SystemAuthz{Store: &db.AuthzSystemPerms{Store: db.NewSystemPermStore(d.Pool)}}
	actionSvc := actions.New(actions.Options{
		Meta:         d.Meta,
		Data:         dataEng,
		ObjectAz:     objectAz,
		FieldAz:      fieldAz,
		RecordAccess: recordAccess,
	})
	dataEng.Actions = actionSvc
	dataEng.ObjectAz = objectAz
	deployEng := deploy.NewDeployEngine(d.Pool, d.Meta, dataEng, deploy.Options{
		InstallID:      cfg.InstallID,
		InstallRole:    cfg.InstallRole,
		ProductVersion: cfg.ProductVersion,
		CustomerID:     cfg.CustomerID,
		PeerMode:       deploy.PeerModeCustomer,
		SyncMaxFiles:   opts.DeploySyncMaxFiles,
		SyncMaxBytes:   opts.DeploySyncMaxBytes,
		QueueMax:       opts.DeployQueueMax,
		JobSlotsDeploy: opts.JobSlotsDeploy,
	})
	s := httpapi.New(httpapi.Options{
		Config:       cfg,
		Resolver:     resolver,
		DB:           d.Pool,
		Pool:         d.Pool,
		Metadata:     d.Meta,
		DataEngine:   dataEng,
		Deploy:       deployEng,
		ObjectAz:     objectAz,
		FieldAz:      fieldAz,
		SystemAz:     systemAz,
		AutomationAz: &authz.AutomationAuthz{Store: &db.AutomationPermStore{Pool: d.Pool}},
		ToolAz:       &authz.ToolAuthz{Store: &db.ToolPermStore{Pool: d.Pool}},
		RecordAccess: recordAccess,
		Actions:      actionSvc,
		EdgeRoller:   opts.EdgeRoller,
		Identity:     opts.Identity,
		LoginBroker:  opts.LoginBroker,
	})
	return &TestServer{
		Handler:  s.Handler(),
		Config:   cfg,
		Pool:     d.Pool,
		Resolver: resolver,
		Deploy:   deployEng,
		Data:     dataEng,
	}
}

// AuthRequest performs an httptest request with Authorization Bearer and optional JSON body.
func AuthRequest(h http.Handler, method, path, bearer string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Authorization", "Bearer "+bearer)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}
