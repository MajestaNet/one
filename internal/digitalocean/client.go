// Package digitalocean is a thin Apps + Databases API client for install-local
// DigitalOcean credentials (ADR-001 — not a vendor multi-tenant fleet plane).
package digitalocean

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.digitalocean.com/v2"

// Client talks to the DigitalOcean public API with a bearer token.
type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

// NewClient returns a client. Empty token → methods return ErrNotConfigured.
func NewClient(token string) *Client {
	return &Client{
		token:   strings.TrimSpace(token),
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// WithBaseURL overrides the API base (tests).
func (c *Client) WithBaseURL(u string) *Client {
	c.baseURL = strings.TrimRight(u, "/")
	return c
}

// WithHTTPClient overrides the HTTP client (tests).
func (c *Client) WithHTTPClient(h *http.Client) *Client {
	c.httpClient = h
	return c
}

// Configured reports whether a token is present.
func (c *Client) Configured() bool {
	return c != nil && c.token != ""
}

// ErrNotConfigured is returned when DIGITALOCEAN_API_TOKEN is unset.
var ErrNotConfigured = fmt.Errorf("digitalocean: API token not configured")

// APIError is a DigitalOcean HTTP error mapped for Majesta One handlers.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("digitalocean: HTTP %d", e.Status)
	}
	return fmt.Sprintf("digitalocean: HTTP %d: %s", e.Status, e.Message)
}

// App is a subset of the Apps API app object.
type App struct {
	ID             string          `json:"id"`
	DefaultIngress string          `json:"default_ingress"`
	LiveURL        string          `json:"live_url"`
	Region         *AppRegion      `json:"region"`
	Spec           json.RawMessage `json:"spec"`
	UpdatedAt      string          `json:"updated_at"`
	CreatedAt      string          `json:"created_at"`
}

// AppRegion holds region slug.
type AppRegion struct {
	Slug string `json:"slug"`
}

// AppSpec is enough of the App Spec to scale and provision.
type AppSpec struct {
	Name     string          `json:"name"`
	Region   string          `json:"region,omitempty"`
	Services []ComponentSpec `json:"services,omitempty"`
	Workers  []ComponentSpec `json:"workers,omitempty"`
	Envs     []AppEnvVar     `json:"envs,omitempty"`
}

// ComponentSpec is a service or worker component.
type ComponentSpec struct {
	Name             string       `json:"name"`
	Image            *ImageSource `json:"image,omitempty"`
	HTTPPort         int          `json:"http_port,omitempty"`
	InstanceCount    int          `json:"instance_count,omitempty"`
	InstanceSizeSlug string       `json:"instance_size_slug,omitempty"`
	Routes           []AppRoute   `json:"routes,omitempty"`
	Envs             []AppEnvVar  `json:"envs,omitempty"`
}

// ImageSource references a container image.
type ImageSource struct {
	RegistryType string `json:"registry_type,omitempty"`
	Registry     string `json:"registry,omitempty"`
	Repository   string `json:"repository,omitempty"`
	Tag          string `json:"tag,omitempty"`
	Digest       string `json:"digest,omitempty"`
}

// AppEnvVar is an app env entry.
type AppEnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
	Type  string `json:"type,omitempty"` // GENERAL | SECRET
	Scope string `json:"scope,omitempty"`
}

// AppRoute is an HTTP route.
type AppRoute struct {
	Path string `json:"path"`
}

// Database is a Managed Database cluster summary.
type Database struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Engine     string        `json:"engine"`
	Version    string        `json:"version"`
	Status     string        `json:"status"`
	Region     string        `json:"region"`
	Size       string        `json:"size"`
	NumNodes   int           `json:"num_nodes"`
	Connection *DBConnection `json:"connection,omitempty"`
}

// DBConnection holds connection details (never log passwords in Majesta One).
type DBConnection struct {
	URI      string `json:"uri"`
	Database string `json:"database"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	SSL      bool   `json:"ssl"`
}

// GetApp retrieves an app by id.
func (c *Client) GetApp(ctx context.Context, appID string) (*App, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	var out struct {
		App App `json:"app"`
	}
	if err := c.do(ctx, http.MethodGet, "/apps/"+appID, nil, &out); err != nil {
		return nil, err
	}
	return &out.App, nil
}

// CreateApp creates an app from a spec.
func (c *Client) CreateApp(ctx context.Context, spec *AppSpec) (*App, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	var out struct {
		App App `json:"app"`
	}
	if err := c.do(ctx, http.MethodPost, "/apps", map[string]any{"spec": spec}, &out); err != nil {
		return nil, err
	}
	return &out.App, nil
}

// UpdateApp replaces the app spec (used for scale / digest redeploy).
func (c *Client) UpdateApp(ctx context.Context, appID string, spec *AppSpec) (*App, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	var out struct {
		App App `json:"app"`
	}
	if err := c.do(ctx, http.MethodPut, "/apps/"+appID, map[string]any{"spec": spec}, &out); err != nil {
		return nil, err
	}
	return &out.App, nil
}

// CreateDeployment triggers a deployment of the current app spec.
func (c *Client) CreateDeployment(ctx context.Context, appID string, forceRebuild bool) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	return c.do(ctx, http.MethodPost, "/apps/"+appID+"/deployments", map[string]any{
		"force_build": forceRebuild,
	}, nil)
}

// GetDatabase retrieves a managed database cluster.
func (c *Client) GetDatabase(ctx context.Context, databaseID string) (*Database, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	var out struct {
		Database Database `json:"database"`
	}
	if err := c.do(ctx, http.MethodGet, "/databases/"+databaseID, nil, &out); err != nil {
		return nil, err
	}
	return &out.Database, nil
}

// CreateDatabase creates a Managed PostgreSQL cluster.
func (c *Client) CreateDatabase(ctx context.Context, name, region, size string, numNodes int, version string) (*Database, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	if version == "" {
		version = "16"
	}
	if numNodes < 1 {
		numNodes = 1
	}
	body := map[string]any{
		"name":      name,
		"engine":    "pg",
		"version":   version,
		"region":    region,
		"size":      size,
		"num_nodes": numNodes,
	}
	var out struct {
		Database Database `json:"database"`
	}
	if err := c.do(ctx, http.MethodPost, "/databases", body, &out); err != nil {
		return nil, err
	}
	return &out.Database, nil
}

// ResizeDatabase changes size and/or num_nodes.
func (c *Client) ResizeDatabase(ctx context.Context, databaseID, size string, numNodes int) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	body := map[string]any{}
	if size != "" {
		body["size"] = size
	}
	if numNodes > 0 {
		body["num_nodes"] = numNodes
	}
	if len(body) == 0 {
		return fmt.Errorf("digitalocean: resize requires size and/or num_nodes")
	}
	return c.do(ctx, http.MethodPut, "/databases/"+databaseID+"/resize", body, nil)
}

// AccountOK probes the token with GET /account (reachability; no token echo).
func (c *Client) AccountOK(ctx context.Context) (bool, error) {
	if !c.Configured() {
		return false, ErrNotConfigured
	}
	err := c.do(ctx, http.MethodGet, "/account", nil, &struct{}{})
	if err != nil {
		var ae *APIError
		if AsAPIError(err, &ae) && (ae.Status == 401 || ae.Status == 403) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// AsAPIError extracts *APIError.
func AsAPIError(err error, target **APIError) bool {
	if err == nil {
		return false
	}
	ae, ok := err.(*APIError)
	if !ok {
		return false
	}
	*target = ae
	return true
}

// ParseAppSpec unmarshals app.spec JSON into AppSpec.
func ParseAppSpec(raw json.RawMessage) (*AppSpec, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("digitalocean: empty app spec")
	}
	var spec AppSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

// PublicURL prefers live_url then default_ingress.
func (a *App) PublicURL() string {
	if a == nil {
		return ""
	}
	if a.LiveURL != "" {
		return strings.TrimRight(a.LiveURL, "/")
	}
	return strings.TrimRight(a.DefaultIngress, "/")
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode >= 400 {
		msg := strings.TrimSpace(string(raw))
		var wrapped struct {
			Message string `json:"message"`
			ID      string `json:"id"`
		}
		if json.Unmarshal(raw, &wrapped) == nil && wrapped.Message != "" {
			msg = wrapped.Message
		}
		// Never include Authorization or token material.
		msg = strings.ReplaceAll(msg, c.token, "[redacted]")
		return &APIError{Status: res.StatusCode, Message: msg}
	}
	if out == nil || res.StatusCode == http.StatusNoContent || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("digitalocean: decode: %w", err)
	}
	return nil
}
