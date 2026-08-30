package deploy_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/deploy"
	"github.com/MajestaNet/ide/internal/digitalocean"
)

type mockCloud struct {
	apps map[string]*digitalocean.App
	dbs  map[string]*digitalocean.Database
}

func (m *mockCloud) Configured() bool { return true }

func (m *mockCloud) AccountOK(ctx context.Context) (bool, error) { return true, nil }

func (m *mockCloud) GetApp(ctx context.Context, appID string) (*digitalocean.App, error) {
	a, ok := m.apps[appID]
	if !ok {
		return nil, &digitalocean.APIError{Status: 404, Message: "not found"}
	}
	return a, nil
}

func (m *mockCloud) CreateApp(ctx context.Context, spec *digitalocean.AppSpec) (*digitalocean.App, error) {
	raw, _ := json.Marshal(spec)
	app := &digitalocean.App{
		ID:      "app-peer",
		LiveURL: "https://example.com",
		Spec:    raw,
		Region:  &digitalocean.AppRegion{Slug: spec.Region},
	}
	m.apps[app.ID] = app
	return app, nil
}

func (m *mockCloud) UpdateApp(ctx context.Context, appID string, spec *digitalocean.AppSpec) (*digitalocean.App, error) {
	raw, _ := json.Marshal(spec)
	live := "https://example.com"
	if prev, ok := m.apps[appID]; ok && prev.LiveURL != "" {
		live = prev.LiveURL
	}
	app := &digitalocean.App{
		ID:      appID,
		LiveURL: live,
		Spec:    raw,
		Region:  &digitalocean.AppRegion{Slug: spec.Region},
	}
	m.apps[appID] = app
	return app, nil
}

func (m *mockCloud) CreateDeployment(ctx context.Context, appID string, forceRebuild bool) error {
	return nil
}

func (m *mockCloud) GetDatabase(ctx context.Context, databaseID string) (*digitalocean.Database, error) {
	d, ok := m.dbs[databaseID]
	if !ok {
		return nil, &digitalocean.APIError{Status: 404, Message: "not found"}
	}
	return d, nil
}

func (m *mockCloud) CreateDatabase(ctx context.Context, name, region, size string, numNodes int, version string) (*digitalocean.Database, error) {
	db := &digitalocean.Database{
		ID: "db-peer", Name: name, Region: region, Size: size, NumNodes: numNodes, Status: "online",
		Connection: &digitalocean.DBConnection{URI: "postgres://u:p@host:25060/defaultdb?sslmode=require"},
	}
	m.dbs[db.ID] = db
	return db, nil
}

func (m *mockCloud) ResizeDatabase(ctx context.Context, databaseID, size string, numNodes int) error {
	d, ok := m.dbs[databaseID]
	if !ok {
		return &digitalocean.APIError{Status: 404, Message: "not found"}
	}
	if size != "" {
		d.Size = size
	}
	if numNodes > 0 {
		d.NumNodes = numNodes
	}
	return nil
}

func sampleAppJSON() json.RawMessage {
	spec := digitalocean.AppSpec{
		Name:   "one-prod",
		Region: "nyc",
		Services: []digitalocean.ComponentSpec{{
			Name: "api", InstanceCount: 2, InstanceSizeSlug: "apps-s-1vcpu-1gb",
			Image: &digitalocean.ImageSource{RegistryType: "GHCR", Repository: "one-api", Tag: "0.1.0"},
		}},
		Workers: []digitalocean.ComponentSpec{{
			Name: "worker", InstanceCount: 1, InstanceSizeSlug: "apps-s-1vcpu-1gb",
			Image: &digitalocean.ImageSource{RegistryType: "GHCR", Repository: "one-worker", Tag: "0.1.0"},
		}},
	}
	raw, _ := json.Marshal(spec)
	return raw
}

func TestCloudHostFlow(t *testing.T) {
	ctx, _, _, _, engine := setupDeployTest(t)
	mock := &mockCloud{
		apps: map[string]*digitalocean.App{
			"app-1": {ID: "app-1", LiveURL: "https://example.com", Spec: sampleAppJSON(), Region: &digitalocean.AppRegion{Slug: "nyc"}},
		},
		dbs: map[string]*digitalocean.Database{
			"db-1": {ID: "db-1", Region: "nyc", Size: "db-s-1vcpu-1gb", NumNodes: 1, Status: "online"},
		},
	}
	engine.SetCloudClient(mock)

	env := engine.GetEnvironment()
	if !env.Capabilities["digitaloceanCloud"] || !env.Capabilities["cloud"] {
		t.Fatal("expected cloud + digitaloceanCloud capabilities")
	}
	if env.CloudHost != "digitalocean" {
		t.Fatalf("cloudHost: %q", env.CloudHost)
	}

	st, err := engine.GetCloudStatus(ctx)
	if err != nil || !st.Configured || st.Reachable == nil || !*st.Reachable {
		t.Fatalf("status: %+v err=%v", st, err)
	}

	b, err := engine.PutCloudBinding(ctx, deploy.BindInput{AppResourceID: "app-1", DatabaseResourceID: "db-1"})
	if err != nil || b.AppResourceID != "app-1" || b.DatabaseResourceID != "db-1" {
		t.Fatalf("binding: %+v err=%v", b, err)
	}
	if b.AppID != "app-1" {
		t.Fatalf("compat appId alias missing: %+v", b)
	}

	app, err := engine.GetCloudApp(ctx)
	if err != nil || app.APIInstances != 2 {
		t.Fatalf("app: %+v err=%v", app, err)
	}
	if app.APISizeClass != "small" {
		t.Fatalf("expected apiSizeClass small, got %q", app.APISizeClass)
	}

	n := 3
	medium := "medium"
	scaled, err := engine.ScaleCloudApp(ctx, deploy.ScaleInput{APIInstanceCount: &n, APISizeClass: &medium})
	if err != nil || scaled.APIInstances != 3 {
		t.Fatalf("scale: %+v err=%v", scaled, err)
	}
	if scaled.APISizeClass != "medium" {
		t.Fatalf("expected medium size class, got %q (size=%q)", scaled.APISizeClass, scaled.APISize)
	}

	db, err := engine.ResizeCloudDatabase(ctx, deploy.ResizeDatabaseInput{SizeClass: "medium", NumNodes: 2})
	if err != nil || db.NumNodes != 2 {
		t.Fatalf("resize: %+v err=%v", db, err)
	}
	if db.Size != "db-s-1vcpu-2gb" {
		t.Fatalf("expected mapped db slug, got %q", db.Size)
	}
	if db.DatabaseResourceID == "" || db.DatabaseID == "" {
		t.Fatalf("missing database ids: %+v", db)
	}

	installID := "peer-dev-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	prov, err := engine.ProvisionCloudEnvironment(ctx, deploy.ProvisionPeerInput{
		InstallID:     installID,
		InstallRole:   "dev",
		Region:        "nyc",
		APIKeysSecret: "peer-key+admin",
		JWTSigningKey: "peer-jwt-secret",
	})
	if err != nil || prov.AppResourceID == "" || prov.Peer == nil || prov.Peer.InstallID != installID {
		t.Fatalf("provision: %+v err=%v", prov, err)
	}
	if prov.AppID == "" {
		t.Fatalf("compat appId missing: %+v", prov)
	}

	listed, err := engine.ListCloudEnvironments(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if listed == nil || listed.ProvisionRuns == nil {
		t.Fatal("missing provisionRuns")
	}

	// DO compatibility wrappers still work.
	st2, err := engine.GetDigitalOceanStatus(ctx)
	if err != nil || !st2.Configured {
		t.Fatalf("do status wrapper: %+v err=%v", st2, err)
	}
}

func TestCloudHostDisabled(t *testing.T) {
	ctx, _, _, _, engine := setupDeployTest(t)
	env := engine.GetEnvironment()
	if env.Capabilities["digitaloceanCloud"] || env.Capabilities["cloud"] {
		t.Fatal("expected cloud capabilities false without token")
	}
	if env.CloudHost != "" {
		t.Fatalf("expected empty cloudHost, got %q", env.CloudHost)
	}
	_, err := engine.ScaleCloudApp(ctx, deploy.ScaleInput{})
	if err == nil {
		t.Fatal("expected error without cloud client")
	}
}

func TestMapSizeClasses(t *testing.T) {
	if got := deploy.MapAppSizeClass("small"); got != "apps-s-1vcpu-1gb" {
		t.Fatalf("app small: %q", got)
	}
	if got := deploy.MapDBSizeClass("large"); got != "db-s-2vcpu-4gb" {
		t.Fatalf("db large: %q", got)
	}
	if got := deploy.MapAppSizeClass("apps-s-1vcpu-1gb"); got != "apps-s-1vcpu-1gb" {
		t.Fatalf("passthrough: %q", got)
	}
}
