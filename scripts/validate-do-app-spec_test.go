package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func scriptsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	return filepath.Dir(file)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	return filepath.Dir(scriptsDir(t))
}

const validSpec = `
name: one-prod
region: nyc
services:
  - name: api
    image:
      registry_type: GHCR
      registry: majestanet
      repository: one-api
      tag: "0.1.0"
    http_port: 8080
    instance_count: 2
    instance_size_slug: apps-s-1vcpu-1gb
    envs:
      - key: PRODUCT_VERSION
        value: "0.1.0"
      - key: API_REVISION_CURRENT
        value: "1"
      - key: API_REVISION_MIN
        value: "1"
      - key: CUSTOMER_ID
        value: "c"
      - key: INSTALL_ID
        value: "i"
      - key: INSTALL_ROLE
        value: "prod"
      - key: PLATFORM_PUBLIC_URL
        value: "https://example.ondigitalocean.app"
      - key: DATABASE_URL
        type: SECRET
      - key: API_KEYS
        type: SECRET
      - key: AUTH_JWT_SIGNING_KEY
        type: SECRET
      - key: INSTALL_CLAIM_TOKEN
        type: SECRET
workers:
  - name: worker
    image:
      registry_type: GHCR
      registry: majestanet
      repository: one-worker
      tag: "0.1.0"
    instance_count: 1
    instance_size_slug: apps-s-1vcpu-1gb
    envs:
      - key: PRODUCT_VERSION
        value: "0.1.0"
      - key: API_REVISION_CURRENT
        value: "1"
      - key: API_REVISION_MIN
        value: "1"
      - key: CUSTOMER_ID
        value: "c"
      - key: INSTALL_ID
        value: "i"
      - key: INSTALL_ROLE
        value: "prod"
      - key: DATABASE_URL
        type: SECRET
`

func TestValidateCheckedInExample(t *testing.T) {
	path := filepath.Join(repoRoot(t), "deploy", "digitalocean", "app.yaml")
	if err := validate(path, validateOptions{}); err != nil {
		t.Fatalf("example spec must pass without -strict-digest: %v", err)
	}
	if err := validate(path, validateOptions{strictDigest: true}); err == nil {
		t.Fatal("example spec must fail -strict-digest (commented digest placeholders)")
	}
}

func TestValidateBytes(t *testing.T) {
	const pinnedAPI = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	const pinnedWorker = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	pinned := strings.Replace(validSpec, `repository: one-api
      tag: "0.1.0"`, `repository: one-api
      tag: "1.2.3"
      digest: `+pinnedAPI, 1)
	pinned = strings.Replace(pinned, `repository: one-worker
      tag: "0.1.0"`, `repository: one-worker
      tag: "1.2.3"
      digest: `+pinnedWorker, 1)

	tests := []struct {
		name         string
		yaml         string
		strictDigest bool
		wantErr      string
	}{
		{
			name: "valid example shape",
			yaml: validSpec,
		},
		{
			name:         "strict digest missing",
			yaml:         validSpec,
			strictDigest: true,
			wantErr:      "image.digest required",
		},
		{
			name:         "strict digest ok",
			yaml:         pinned,
			strictDigest: true,
		},
		{
			name:         "strict digest placeholder",
			yaml:         strings.Replace(validSpec, `tag: "0.1.0"`, "tag: \"0.1.0\"\n      digest: sha256:REPLACE_WITH_API_DIGEST", 1),
			strictDigest: true,
			wantErr:      "sha256:<64 hex>",
		},
		{
			name:    "missing PLATFORM_PUBLIC_URL",
			yaml:    strings.Replace(validSpec, "      - key: PLATFORM_PUBLIC_URL\n        value: \"https://example.ondigitalocean.app\"\n", "", 1),
			wantErr: "PLATFORM_PUBLIC_URL",
		},
		{
			name:    "API_KEYS not SECRET",
			yaml:    strings.Replace(validSpec, "      - key: API_KEYS\n        type: SECRET", "      - key: API_KEYS\n        value: \"plain\"", 1),
			wantErr: "API_KEYS must be type SECRET",
		},
		{
			name:    "AUTH_JWT_SIGNING_KEY not SECRET",
			yaml:    strings.Replace(validSpec, "      - key: AUTH_JWT_SIGNING_KEY\n        type: SECRET", "      - key: AUTH_JWT_SIGNING_KEY\n        value: \"plain\"", 1),
			wantErr: "AUTH_JWT_SIGNING_KEY must be type SECRET",
		},
		{
			name:    "INSTALL_CLAIM_TOKEN missing",
			yaml:    strings.Replace(validSpec, "      - key: INSTALL_CLAIM_TOKEN\n        type: SECRET\n", "", 1),
			wantErr: "INSTALL_CLAIM_TOKEN",
		},
		{
			name:    "latest tag refused",
			yaml:    strings.Replace(validSpec, `tag: "0.1.0"`, `tag: "latest"`, 1),
			wantErr: "not empty or latest",
		},
		{
			name:    "DATABASE_URL not SECRET",
			yaml:    strings.Replace(validSpec, "      - key: DATABASE_URL\n        type: SECRET", "      - key: DATABASE_URL\n        value: postgres://x", 1),
			wantErr: "DATABASE_URL must be type SECRET",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBytes([]byte(tt.yaml), validateOptions{strictDigest: tt.strictDigest})
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	if err := os.WriteFile(path, []byte(validSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validate(path, validateOptions{}); err != nil {
		t.Fatal(err)
	}
}
