package deploy_test

import (
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/deploy"
	"github.com/MajestaNet/ide/internal/digitalocean"
)

func TestGetEnvironmentAPIRevision(t *testing.T) {
	t.Parallel()
	eng := deploy.NewDeployEngine(nil, nil, nil, deploy.Options{
		InstallID: "local", CustomerID: "t", ProductVersion: "0.4.0",
		APIRevisionMin: 12, APIRevisionCurrent: 14,
	})
	env := eng.GetEnvironment()
	if env.ApiRevision.Min != 12 || env.ApiRevision.Current != 14 {
		t.Fatalf("apiRevision=%+v", env.ApiRevision)
	}
	if env.Capabilities["crossEnvironmentPromote"] {
		t.Fatal("expected crossEnvironmentPromote false (repo→org only)")
	}
	if !strings.Contains(env.Notes, "repo → org") {
		t.Fatalf("notes should describe repo→org: %q", env.Notes)
	}
}

func TestGetEnvironmentDigitalOceanCapability(t *testing.T) {
	t.Parallel()
	eng := deploy.NewDeployEngine(nil, nil, nil, deploy.Options{
		InstallID: "local", CustomerID: "t", ProductVersion: "0.1.0",
	})
	env := eng.GetEnvironment()
	if env.Capabilities["digitaloceanCloud"] || env.Capabilities["cloud"] {
		t.Fatal("expected cloud capabilities false")
	}
	if env.CloudHost != "" {
		t.Fatalf("expected empty cloudHost, got %q", env.CloudHost)
	}
	eng.SetCloudClient(digitalocean.NewClient("tok"))
	env = eng.GetEnvironment()
	if !env.Capabilities["digitaloceanCloud"] || !env.Capabilities["cloud"] {
		t.Fatal("expected cloud + digitaloceanCloud true when token set")
	}
	if env.CloudHost != "digitalocean" {
		t.Fatalf("cloudHost: %q", env.CloudHost)
	}
}
