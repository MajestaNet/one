package deploy

import "time"

// CloudBinding is the persisted install↔cloud resource binding (host-agnostic).
type CloudBinding struct {
	Host               string         `json:"host,omitempty"`
	AppResourceID      string         `json:"appResourceId,omitempty"`
	DatabaseResourceID string         `json:"databaseResourceId,omitempty"`
	Region             string         `json:"region,omitempty"`
	DisplayName        string         `json:"displayName,omitempty"`
	ProviderMeta       map[string]any `json:"providerMeta,omitempty"`
	UpdatedAt          time.Time      `json:"updatedAt,omitempty"`
	CreatedAt          time.Time      `json:"createdAt,omitempty"`
	// Compatibility aliases for DO consumers.
	AppID      string `json:"appId,omitempty"`
	DatabaseID string `json:"databaseId,omitempty"`
	AppName    string `json:"appName,omitempty"`
}

// BindInput attaches this install to existing cloud resources.
type BindInput struct {
	AppResourceID      string         `json:"appResourceId"`
	DatabaseResourceID string         `json:"databaseResourceId"`
	Region             string         `json:"region"`
	DisplayName        string         `json:"displayName"`
	ProviderMeta       map[string]any `json:"providerMeta"`
	// Compatibility aliases.
	AppID      string `json:"appId"`
	DatabaseID string `json:"databaseId"`
	AppName    string `json:"appName"`
}

// Normalize fills opaque ids from compatibility aliases.
func (in *BindInput) Normalize() {
	if in.AppResourceID == "" {
		in.AppResourceID = in.AppID
	}
	if in.DatabaseResourceID == "" {
		in.DatabaseResourceID = in.DatabaseID
	}
	if in.DisplayName == "" {
		in.DisplayName = in.AppName
	}
}

// CloudStatus is GET /cloud/status.
type CloudStatus struct {
	Host         string          `json:"host,omitempty"`
	Configured   bool            `json:"configured"`
	Reachable    *bool           `json:"reachable,omitempty"`
	Binding      *CloudBinding   `json:"binding,omitempty"`
	Capabilities map[string]bool `json:"capabilities"`
}

// AppSummary is GET /cloud/app (describe).
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
	// Compatibility aliases.
	AppID      string `json:"appId,omitempty"`
	APISize    string `json:"apiSize,omitempty"`
	WorkerSize string `json:"workerSize,omitempty"`
}

// ScaleInput scales api and/or worker components.
type ScaleInput struct {
	APIInstanceCount    *int    `json:"apiInstanceCount"`
	APISizeClass        *string `json:"apiSizeClass"`
	WorkerInstanceCount *int    `json:"workerInstanceCount"`
	WorkerSizeClass     *string `json:"workerSizeClass"`
	// Compatibility / provider escape hatch (DO slugs).
	APIInstanceSizeSlug    *string `json:"apiInstanceSizeSlug"`
	WorkerInstanceSizeSlug *string `json:"workerInstanceSizeSlug"`
}

// EffectiveAPISize returns the size class or provider slug for the API component.
func (in ScaleInput) EffectiveAPISize() *string {
	if in.APISizeClass != nil && *in.APISizeClass != "" {
		return in.APISizeClass
	}
	return in.APIInstanceSizeSlug
}

// EffectiveWorkerSize returns the size class or provider slug for the worker component.
func (in ScaleInput) EffectiveWorkerSize() *string {
	if in.WorkerSizeClass != nil && *in.WorkerSizeClass != "" {
		return in.WorkerSizeClass
	}
	return in.WorkerInstanceSizeSlug
}

// ResizeDatabaseInput resizes managed Postgres.
type ResizeDatabaseInput struct {
	SizeClass string `json:"sizeClass"`
	Size      string `json:"size"` // provider slug escape hatch / DO compat
	NumNodes  int    `json:"numNodes"`
}

// EffectiveSize returns sizeClass or raw size slug.
func (in ResizeDatabaseInput) EffectiveSize() string {
	if in.SizeClass != "" {
		return in.SizeClass
	}
	return in.Size
}

// DatabaseSummary is a safe resize/describe response (never includes connection secrets).
type DatabaseSummary struct {
	DatabaseResourceID string `json:"databaseResourceId"`
	Name               string `json:"name,omitempty"`
	Region             string `json:"region,omitempty"`
	SizeClass          string `json:"sizeClass,omitempty"`
	Size               string `json:"size,omitempty"`
	NumNodes           int    `json:"numNodes,omitempty"`
	Status             string `json:"status,omitempty"`
	// Compatibility.
	DatabaseID string `json:"databaseId,omitempty"`
}

// RedeployInput optionally updates digests then creates a deployment.
type RedeployInput struct {
	APIDigest          string `json:"apiDigest"`
	WorkerDigest       string `json:"workerDigest"`
	APITag             string `json:"apiTag"`
	WorkerTag          string `json:"workerTag"`
	ProductVersion     string `json:"productVersion"`
	APIRevisionMin     int    `json:"apiRevisionMin,omitempty"`
	APIRevisionCurrent int    `json:"apiRevisionCurrent,omitempty"`
	ForceRebuild       bool   `json:"forceRebuild"`
}

// ProvisionPeerInput creates a peer app + managed DB.
type ProvisionPeerInput struct {
	InstallID         string `json:"installId"`
	InstallRole       string `json:"installRole"`
	DisplayName       string `json:"displayName"`
	AppName           string `json:"appName"` // compat
	Region            string `json:"region"`
	DatabaseSizeClass string `json:"databaseSizeClass"`
	DatabaseSize      string `json:"databaseSize"` // compat / slug
	DatabaseNodes     int    `json:"databaseNodes"`
	APISizeClass      string `json:"apiSizeClass"`
	WorkerSizeClass   string `json:"workerSizeClass"`
	APISizeSlug       string `json:"apiInstanceSizeSlug"`
	WorkerSizeSlug    string `json:"workerInstanceSizeSlug"`
	APITag            string `json:"apiTag"`
	WorkerTag         string `json:"workerTag"`
	APIDigest         string `json:"apiDigest"`
	WorkerDigest      string `json:"workerDigest"`
	ProductVersion    string `json:"productVersion"`
	APIKeysSecret     string `json:"apiKeys"`
	JWTSigningKey     string `json:"authJwtSigningKey"`
	DeployShareSecret string `json:"deployShareSecret"`
	CreatedBy         *string
}

// Normalize fills display name and size fields from compatibility aliases.
func (in *ProvisionPeerInput) Normalize() {
	if in.DisplayName == "" {
		in.DisplayName = in.AppName
	}
	if in.APISizeClass == "" {
		in.APISizeClass = in.APISizeSlug
	}
	if in.WorkerSizeClass == "" {
		in.WorkerSizeClass = in.WorkerSizeSlug
	}
	if in.DatabaseSizeClass == "" {
		in.DatabaseSizeClass = in.DatabaseSize
	}
}

// ProvisionPeerResult is the provision response.
type ProvisionPeerResult struct {
	Peer               *PeerRow    `json:"peer"`
	AppResourceID      string      `json:"appResourceId"`
	DatabaseResourceID string      `json:"databaseResourceId"`
	PublicURL          string      `json:"publicUrl,omitempty"`
	RunID              string      `json:"runId,omitempty"`
	App                *AppSummary `json:"app,omitempty"`
	// Compatibility.
	AppID      string `json:"appId,omitempty"`
	DatabaseID string `json:"databaseId,omitempty"`
}

// EnvironmentList is GET /cloud/environments.
type EnvironmentList struct {
	Peers         []PeerRow      `json:"peers"`
	ProvisionRuns []ProvisionRun `json:"provisionRuns"`
}

// ProvisionRun is an audit row for peer provision attempts.
type ProvisionRun struct {
	ID                 string    `json:"id"`
	Host               string    `json:"host,omitempty"`
	PeerInstallID      string    `json:"peerInstallId"`
	InstallRole        string    `json:"installRole,omitempty"`
	AppResourceID      string    `json:"appResourceId,omitempty"`
	DatabaseResourceID string    `json:"databaseResourceId,omitempty"`
	BaseURL            string    `json:"baseUrl,omitempty"`
	Status             string    `json:"status"`
	Error              string    `json:"error,omitempty"`
	CreatedAt          time.Time `json:"createdAt"`
	// Compatibility.
	AppID      string `json:"appId,omitempty"`
	DatabaseID string `json:"databaseId,omitempty"`
}

// withCompatAliases mirrors opaque ids onto DO-shaped fields for clients mid-migration.
func (b *CloudBinding) withCompatAliases() *CloudBinding {
	if b == nil {
		return nil
	}
	b.AppID = b.AppResourceID
	b.DatabaseID = b.DatabaseResourceID
	b.AppName = b.DisplayName
	return b
}

func (a *AppSummary) withCompatAliases() *AppSummary {
	if a == nil {
		return nil
	}
	a.AppID = a.AppResourceID
	if a.APISize == "" {
		a.APISize = a.APISizeClass
	}
	if a.WorkerSize == "" {
		a.WorkerSize = a.WorkerSizeClass
	}
	return a
}

func (d *DatabaseSummary) withCompatAliases() *DatabaseSummary {
	if d == nil {
		return nil
	}
	d.DatabaseID = d.DatabaseResourceID
	return d
}

func (r *ProvisionPeerResult) withCompatAliases() *ProvisionPeerResult {
	if r == nil {
		return nil
	}
	r.AppID = r.AppResourceID
	r.DatabaseID = r.DatabaseResourceID
	if r.App != nil {
		r.App.withCompatAliases()
	}
	return r
}
