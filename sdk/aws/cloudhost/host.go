// Package cloudhost is a community skeleton for an AWS CloudHost adapter.
//
// Not product GA. Not a second Path A. Wire via a custom main that calls
// product DeployEngine.SetCloudHost — do not add AWS drivers to the stock
// product binary until explicitly GA'd.
//
// Contract: docs/architecture/deploy-cloud-capability-contract.md
// Primary consumer routes: /deploy/v1/cloud/* (host-free)
package cloudhost

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// ErrNotImplemented marks verbs not yet backed by AWS APIs in this skeleton.
var ErrNotImplemented = errors.New("aws cloudhost: not implemented")

// ErrNotConfigured means install-local AWS credentials / binding inputs are missing.
var ErrNotConfigured = errors.New("aws cloudhost: not configured")

// Host is the verb-shaped adapter surface matching Majesta One Deploy CloudHost.
// Kept local to sdk/aws so this module does not import product internals.
type Host interface {
	Host() string
	Configured() bool
	AccountOK(ctx context.Context) (bool, error)
	ResolveBinding(ctx context.Context, in BindInput) (*CloudBinding, error)
	Describe(ctx context.Context, appResourceID string) (*AppSummary, error)
	Scale(ctx context.Context, appResourceID string, in ScaleInput) (*AppSummary, error)
	ResizeDatabase(ctx context.Context, databaseResourceID string, in ResizeDatabaseInput) (*DatabaseSummary, error)
	Redeploy(ctx context.Context, appResourceID string, in RedeployInput) (*AppSummary, error)
	ProvisionPeer(ctx context.Context, in ProvisionPeerInput) (*ProvisionPeerResult, error)
}

// CloudBinding mirrors Majesta One deploy.CloudBinding JSON fields.
type CloudBinding struct {
	Host               string         `json:"host,omitempty"`
	AppResourceID      string         `json:"appResourceId,omitempty"`
	DatabaseResourceID string         `json:"databaseResourceId,omitempty"`
	Region             string         `json:"region,omitempty"`
	DisplayName        string         `json:"displayName,omitempty"`
	ProviderMeta       map[string]any `json:"providerMeta,omitempty"`
}

// BindInput attaches this install to existing ECS/ALB + RDS resources.
type BindInput struct {
	AppResourceID      string         `json:"appResourceId"`
	DatabaseResourceID string         `json:"databaseResourceId"`
	Region             string         `json:"region"`
	DisplayName        string         `json:"displayName"`
	ProviderMeta       map[string]any `json:"providerMeta"`
}

// CloudStatus is documented for HTTP /deploy/v1/cloud/status consumers.
type CloudStatus struct {
	Host         string          `json:"host,omitempty"`
	Configured   bool            `json:"configured"`
	Reachable    *bool           `json:"reachable,omitempty"`
	Binding      *CloudBinding   `json:"binding,omitempty"`
	Capabilities map[string]bool `json:"capabilities"`
}

// AppSummary is GET /cloud/app.
type AppSummary struct {
	AppResourceID   string `json:"appResourceId"`
	Name            string `json:"name,omitempty"`
	Region          string `json:"region,omitempty"`
	PublicURL       string `json:"publicUrl,omitempty"`
	APIInstances    int    `json:"apiInstances,omitempty"`
	APISizeClass    string `json:"apiSizeClass,omitempty"`
	WorkerInstances int    `json:"workerInstances,omitempty"`
	WorkerSizeClass string `json:"workerSizeClass,omitempty"`
	APIImageTag     string `json:"apiImageTag,omitempty"`
	APIImageDigest  string `json:"apiImageDigest,omitempty"`
	WorkerImageTag  string `json:"workerImageTag,omitempty"`
	WorkerDigest    string `json:"workerImageDigest,omitempty"`
}

// ScaleInput scales api/worker desired count or size class.
type ScaleInput struct {
	APIInstanceCount    *int    `json:"apiInstanceCount"`
	APISizeClass        *string `json:"apiSizeClass"`
	WorkerInstanceCount *int    `json:"workerInstanceCount"`
	WorkerSizeClass     *string `json:"workerSizeClass"`
}

// ResizeDatabaseInput resizes RDS.
type ResizeDatabaseInput struct {
	SizeClass string `json:"sizeClass"`
	Size      string `json:"size"`
	NumNodes  int    `json:"numNodes"`
}

// DatabaseSummary never includes connection secrets.
type DatabaseSummary struct {
	DatabaseResourceID string `json:"databaseResourceId"`
	Name               string `json:"name,omitempty"`
	Region             string `json:"region,omitempty"`
	SizeClass          string `json:"sizeClass,omitempty"`
	Size               string `json:"size,omitempty"`
	NumNodes           int    `json:"numNodes,omitempty"`
	Status             string `json:"status,omitempty"`
}

// RedeployInput updates image digests/tags then forces a new deployment.
type RedeployInput struct {
	APIDigest      string `json:"apiDigest"`
	WorkerDigest   string `json:"workerDigest"`
	APITag         string `json:"apiTag"`
	WorkerTag      string `json:"workerTag"`
	ProductVersion string `json:"productVersion"`
	ForceRebuild   bool   `json:"forceRebuild"`
}

// ProvisionPeerInput creates a peer ECS+RDS install shape.
type ProvisionPeerInput struct {
	CustomerID        string
	InstallID         string
	InstallRole       string
	DisplayName       string
	Region            string
	DatabaseSize      string
	DatabaseNodes     int
	APISize           string
	WorkerSize        string
	APITag            string
	WorkerTag         string
	APIDigest         string
	WorkerDigest      string
	ProductVersion    string
	APIKeysSecret     string
	JWTSigningKey     string
	DeployShareSecret string
}

// ProvisionPeerResult is adapter output before Deploy peer registration.
type ProvisionPeerResult struct {
	AppResourceID      string
	DatabaseResourceID string
	PublicURL          string
	App                *AppSummary
}

// Config configures the opinionated ECS + RDS managed profile adapter.
type Config struct {
	Region          string
	Cluster         string
	APIService      string
	WorkerService   string
	TargetGroupARN  string
	LoadBalancerDNS string
	RDSInstanceID   string
	// Optional explicit resource ids used when binding is not yet persisted.
	AppResourceID      string
	DatabaseResourceID string
}

// ECSHost is the sole AWS CloudHost skeleton: opinionated ECS Fargate api+worker
// services + ALB + RDS (Majesta One requires both api and long-running worker).
// Methods validate configuration and return ErrNotImplemented until AWS API
// calls are filled in by community contributors.
type ECSHost struct {
	cfg Config
}

// NewECSHost builds an AWS CloudHost from config and common env fallbacks:
// AWS_REGION, ONE_ECS_CLUSTER, ONE_ECS_API_SERVICE, ONE_ECS_WORKER_SERVICE,
// ONE_RDS_INSTANCE_ID, ONE_ALB_DNS.
func NewECSHost(cfg Config) *ECSHost {
	if cfg.Region == "" {
		cfg.Region = firstEnv("AWS_REGION", "AWS_DEFAULT_REGION")
	}
	if cfg.Cluster == "" {
		cfg.Cluster = os.Getenv("ONE_ECS_CLUSTER")
	}
	if cfg.APIService == "" {
		cfg.APIService = os.Getenv("ONE_ECS_API_SERVICE")
	}
	if cfg.WorkerService == "" {
		cfg.WorkerService = os.Getenv("ONE_ECS_WORKER_SERVICE")
	}
	if cfg.RDSInstanceID == "" {
		cfg.RDSInstanceID = os.Getenv("ONE_RDS_INSTANCE_ID")
	}
	if cfg.LoadBalancerDNS == "" {
		cfg.LoadBalancerDNS = os.Getenv("ONE_ALB_DNS")
	}
	if cfg.AppResourceID == "" && cfg.Cluster != "" && cfg.APIService != "" {
		cfg.AppResourceID = cfg.Cluster + "/" + cfg.APIService
	}
	if cfg.DatabaseResourceID == "" {
		cfg.DatabaseResourceID = cfg.RDSInstanceID
	}
	return &ECSHost{cfg: cfg}
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func (h *ECSHost) Host() string { return "aws" }

func (h *ECSHost) Configured() bool {
	return h != nil && h.cfg.Region != "" && h.cfg.Cluster != "" && h.cfg.APIService != ""
}

func (h *ECSHost) AccountOK(ctx context.Context) (bool, error) {
	if !h.Configured() {
		return false, ErrNotConfigured
	}
	// Skeleton: treat presence of config as reachable. Replace with STS GetCallerIdentity.
	_ = ctx
	return true, nil
}

func (h *ECSHost) ResolveBinding(ctx context.Context, in BindInput) (*CloudBinding, error) {
	_ = ctx
	if !h.Configured() {
		return nil, ErrNotConfigured
	}
	appID := strings.TrimSpace(in.AppResourceID)
	dbID := strings.TrimSpace(in.DatabaseResourceID)
	if appID == "" {
		appID = h.cfg.AppResourceID
	}
	if dbID == "" {
		dbID = h.cfg.DatabaseResourceID
	}
	if appID == "" && dbID == "" {
		return nil, fmt.Errorf("%w: appResourceId or databaseResourceId required", ErrNotConfigured)
	}
	region := in.Region
	if region == "" {
		region = h.cfg.Region
	}
	name := in.DisplayName
	if name == "" {
		name = h.cfg.APIService
	}
	meta := in.ProviderMeta
	if meta == nil {
		meta = map[string]any{}
	}
	meta["cluster"] = h.cfg.Cluster
	meta["apiService"] = h.cfg.APIService
	meta["workerService"] = h.cfg.WorkerService
	meta["profile"] = "ecs-fargate-services"
	return &CloudBinding{
		Host:               "aws",
		AppResourceID:      appID,
		DatabaseResourceID: dbID,
		Region:             region,
		DisplayName:        name,
		ProviderMeta:       meta,
	}, nil
}

func (h *ECSHost) Describe(ctx context.Context, appResourceID string) (*AppSummary, error) {
	if !h.Configured() {
		return nil, ErrNotConfigured
	}
	_ = ctx
	_ = appResourceID
	url := h.cfg.LoadBalancerDNS
	if url != "" && !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}
	// Skeleton summary from config; replace with ECS DescribeServices + ALB lookup.
	return &AppSummary{
		AppResourceID:   firstNonEmpty(appResourceID, h.cfg.AppResourceID),
		Name:            h.cfg.APIService,
		Region:          h.cfg.Region,
		PublicURL:       url,
		APISizeClass:    "small",
		WorkerSizeClass: "small",
	}, nil
}

func (h *ECSHost) Scale(ctx context.Context, appResourceID string, in ScaleInput) (*AppSummary, error) {
	_ = ctx
	_ = appResourceID
	_ = in
	if !h.Configured() {
		return nil, ErrNotConfigured
	}
	return nil, fmt.Errorf("%w: Scale (UpdateService desiredCount / capacity provider)", ErrNotImplemented)
}

func (h *ECSHost) ResizeDatabase(ctx context.Context, databaseResourceID string, in ResizeDatabaseInput) (*DatabaseSummary, error) {
	_ = ctx
	_ = databaseResourceID
	_ = in
	if !h.Configured() {
		return nil, ErrNotConfigured
	}
	return nil, fmt.Errorf("%w: ResizeDatabase (ModifyDBInstance)", ErrNotImplemented)
}

func (h *ECSHost) Redeploy(ctx context.Context, appResourceID string, in RedeployInput) (*AppSummary, error) {
	_ = ctx
	_ = appResourceID
	_ = in
	if !h.Configured() {
		return nil, ErrNotConfigured
	}
	return nil, fmt.Errorf("%w: Redeploy (prefer /ops/v1; or UpdateService forceNewDeployment)", ErrNotImplemented)
}

func (h *ECSHost) ProvisionPeer(ctx context.Context, in ProvisionPeerInput) (*ProvisionPeerResult, error) {
	_ = ctx
	_ = in
	if !h.Configured() {
		return nil, ErrNotConfigured
	}
	return nil, fmt.Errorf("%w: ProvisionPeer (create ECS services + RDS in customer account)", ErrNotImplemented)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// DefaultCapabilities is the status.capabilities map for a fully featured AWS adapter.
func DefaultCapabilities() map[string]bool {
	return map[string]bool{
		"bind":           true,
		"scaleApp":       true,
		"resizeDatabase": true,
		"provisionPeer":  true,
		"redeploy":       true,
	}
}
