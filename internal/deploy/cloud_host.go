package deploy

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/MajestaNet/ide/internal/digitalocean"
)

// CloudAPI is the low-level DigitalOcean Apps/Databases client subset (mockable).
// Prefer CloudHost for new call sites; CloudAPI remains the DO adapter backend.
type CloudAPI interface {
	Configured() bool
	AccountOK(ctx context.Context) (bool, error)
	GetApp(ctx context.Context, appID string) (*digitalocean.App, error)
	CreateApp(ctx context.Context, spec *digitalocean.AppSpec) (*digitalocean.App, error)
	UpdateApp(ctx context.Context, appID string, spec *digitalocean.AppSpec) (*digitalocean.App, error)
	CreateDeployment(ctx context.Context, appID string, forceRebuild bool) error
	GetDatabase(ctx context.Context, databaseID string) (*digitalocean.Database, error)
	CreateDatabase(ctx context.Context, name, region, size string, numNodes int, version string) (*digitalocean.Database, error)
	ResizeDatabase(ctx context.Context, databaseID, size string, numNodes int) error
}

// CloudHost is the verb-shaped install-local cloud adapter (no provider wire types in signatures).
type CloudHost interface {
	Host() string
	Configured() bool
	AccountOK(ctx context.Context) (bool, error)
	ResolveBinding(ctx context.Context, in BindInput) (*CloudBinding, error)
	Describe(ctx context.Context, appResourceID string) (*AppSummary, error)
	Scale(ctx context.Context, appResourceID string, in ScaleInput) (*AppSummary, error)
	ResizeDatabase(ctx context.Context, databaseResourceID string, in ResizeDatabaseInput) (*DatabaseSummary, error)
	Redeploy(ctx context.Context, appResourceID string, in RedeployInput) (*AppSummary, error)
	ProvisionPeer(ctx context.Context, in ProvisionPeerHostInput) (*ProvisionPeerHostResult, error)
}

// ProvisionPeerHostInput is what the engine passes to the adapter after validation defaults.
type ProvisionPeerHostInput struct {
	CustomerID         string
	InstallID          string
	InstallRole        string
	DisplayName        string
	Region             string
	DatabaseSize       string
	DatabaseNodes      int
	APISize            string
	WorkerSize         string
	APITag             string
	WorkerTag          string
	APIDigest          string
	WorkerDigest       string
	ProductVersion     string
	APIRevisionMin     int
	APIRevisionCurrent int
	APIKeysSecret      string
	JWTSigningKey      string
	DeployShareSecret  string
	PublicURLHint      string
}

// ProvisionPeerHostResult is adapter output before peer registration.
type ProvisionPeerHostResult struct {
	AppResourceID      string
	DatabaseResourceID string
	PublicURL          string
	App                *AppSummary
}

// SetCloudHost attaches an install-local cloud adapter (nil disables).
func (e *DeployEngine) SetCloudHost(h CloudHost) {
	e.host = h
	e.cloud = nil
	if do, ok := h.(*digitalOceanHost); ok {
		e.cloud = do.api
	}
}

// SetCloudClient wraps a DigitalOcean CloudAPI as the active CloudHost (product Path A).
func (e *DeployEngine) SetCloudClient(c CloudAPI) {
	if c == nil {
		e.host = nil
		e.cloud = nil
		return
	}
	e.SetCloudHost(NewDigitalOceanHost(c))
}

// CloudConfigured reports whether a cloud adapter token/role is present.
func (e *DeployEngine) CloudConfigured() bool {
	return e.host != nil && e.host.Configured()
}

// CloudHostName returns the active adapter id (e.g. "digitalocean") or "".
func (e *DeployEngine) CloudHostName() string {
	if e.host == nil || !e.host.Configured() {
		return ""
	}
	return e.host.Host()
}

// ---- DigitalOcean CloudHost adapter ----

// Majesta One size classes → DO App Platform / Managed DB slugs.
var (
	doAppSizeClass = map[string]string{
		"small":  "apps-s-1vcpu-1gb",
		"medium": "apps-s-1vcpu-2gb",
		"large":  "apps-s-2vcpu-4gb",
		"xlarge": "apps-s-4vcpu-8gb",
	}
	doDBSizeClass = map[string]string{
		"small":  "db-s-1vcpu-1gb",
		"medium": "db-s-1vcpu-2gb",
		"large":  "db-s-2vcpu-4gb",
		"xlarge": "db-s-4vcpu-8gb",
	}
	doAppSlugToClass = invertMap(doAppSizeClass)
	doDBSlugToClass  = invertMap(doDBSizeClass)
)

func invertMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

// MapAppSizeClass resolves a Majesta One size class or DO slug to an App Platform slug.
func MapAppSizeClass(classOrSlug string) string {
	classOrSlug = strings.TrimSpace(classOrSlug)
	if classOrSlug == "" {
		return ""
	}
	if slug, ok := doAppSizeClass[strings.ToLower(classOrSlug)]; ok {
		return slug
	}
	return classOrSlug
}

// MapDBSizeClass resolves a Majesta One size class or DO slug to a Managed DB slug.
func MapDBSizeClass(classOrSlug string) string {
	classOrSlug = strings.TrimSpace(classOrSlug)
	if classOrSlug == "" {
		return ""
	}
	if slug, ok := doDBSizeClass[strings.ToLower(classOrSlug)]; ok {
		return slug
	}
	return classOrSlug
}

// AppSizeClassFromSlug maps a DO slug back to a Majesta One size class when known.
func AppSizeClassFromSlug(slug string) string {
	if c, ok := doAppSlugToClass[slug]; ok {
		return c
	}
	return slug
}

// DBSizeClassFromSlug maps a DO DB slug back to a Majesta One size class when known.
func DBSizeClassFromSlug(slug string) string {
	if c, ok := doDBSlugToClass[slug]; ok {
		return c
	}
	return slug
}

type digitalOceanHost struct {
	api CloudAPI
}

// NewDigitalOceanHost wraps a DO Apps/Databases client as a CloudHost.
func NewDigitalOceanHost(api CloudAPI) CloudHost {
	return &digitalOceanHost{api: api}
}

func (h *digitalOceanHost) Host() string { return "digitalocean" }

func (h *digitalOceanHost) Configured() bool {
	return h != nil && h.api != nil && h.api.Configured()
}

func (h *digitalOceanHost) AccountOK(ctx context.Context) (bool, error) {
	return h.api.AccountOK(ctx)
}

func (h *digitalOceanHost) ResolveBinding(ctx context.Context, in BindInput) (*CloudBinding, error) {
	in.Normalize()
	out := &CloudBinding{
		Host:               "digitalocean",
		AppResourceID:      strings.TrimSpace(in.AppResourceID),
		DatabaseResourceID: strings.TrimSpace(in.DatabaseResourceID),
		Region:             strings.TrimSpace(in.Region),
		DisplayName:        strings.TrimSpace(in.DisplayName),
		ProviderMeta:       in.ProviderMeta,
	}
	if out.AppResourceID != "" {
		app, err := h.api.GetApp(ctx, out.AppResourceID)
		if err != nil {
			return nil, mapCloudErr(err)
		}
		if out.DisplayName == "" {
			if spec, err := digitalocean.ParseAppSpec(app.Spec); err == nil && spec != nil {
				out.DisplayName = spec.Name
			}
		}
		if out.Region == "" && app.Region != nil {
			out.Region = app.Region.Slug
		}
	}
	if out.DatabaseResourceID != "" {
		db, err := h.api.GetDatabase(ctx, out.DatabaseResourceID)
		if err != nil {
			return nil, mapCloudErr(err)
		}
		if out.Region == "" {
			out.Region = db.Region
		}
	}
	return out.withCompatAliases(), nil
}

func (h *digitalOceanHost) Describe(ctx context.Context, appResourceID string) (*AppSummary, error) {
	app, err := h.api.GetApp(ctx, appResourceID)
	if err != nil {
		return nil, mapCloudErr(err)
	}
	return summarizeApp(app), nil
}

func (h *digitalOceanHost) Scale(ctx context.Context, appResourceID string, in ScaleInput) (*AppSummary, error) {
	app, err := h.api.GetApp(ctx, appResourceID)
	if err != nil {
		return nil, mapCloudErr(err)
	}
	spec, err := digitalocean.ParseAppSpec(app.Spec)
	if err != nil {
		return nil, err
	}
	apiSize := in.EffectiveAPISize()
	workerSize := in.EffectiveWorkerSize()
	for i := range spec.Services {
		isAPI := spec.Services[i].Name == "api" || (spec.Services[i].Name == "" && i == 0)
		if !isAPI {
			continue
		}
		if in.APIInstanceCount != nil {
			if *in.APIInstanceCount < 1 {
				return nil, newValidationError("apiInstanceCount must be >= 1")
			}
			spec.Services[i].InstanceCount = *in.APIInstanceCount
		}
		if apiSize != nil && *apiSize != "" {
			spec.Services[i].InstanceSizeSlug = MapAppSizeClass(*apiSize)
		}
	}
	for i := range spec.Workers {
		isWorker := spec.Workers[i].Name == "worker" || (spec.Workers[i].Name == "" && i == 0)
		if !isWorker {
			continue
		}
		if in.WorkerInstanceCount != nil {
			if *in.WorkerInstanceCount < 1 {
				return nil, newValidationError("workerInstanceCount must be >= 1")
			}
			spec.Workers[i].InstanceCount = *in.WorkerInstanceCount
		}
		if workerSize != nil && *workerSize != "" {
			spec.Workers[i].InstanceSizeSlug = MapAppSizeClass(*workerSize)
		}
	}
	updated, err := h.api.UpdateApp(ctx, appResourceID, spec)
	if err != nil {
		return nil, mapCloudErr(err)
	}
	return summarizeApp(updated), nil
}

func (h *digitalOceanHost) ResizeDatabase(ctx context.Context, databaseResourceID string, in ResizeDatabaseInput) (*DatabaseSummary, error) {
	size := MapDBSizeClass(in.EffectiveSize())
	if size == "" && in.NumNodes <= 0 {
		return nil, newValidationError("sizeClass/size and/or numNodes is required")
	}
	if err := h.api.ResizeDatabase(ctx, databaseResourceID, size, in.NumNodes); err != nil {
		return nil, mapCloudErr(err)
	}
	db, err := h.api.GetDatabase(ctx, databaseResourceID)
	if err != nil {
		return nil, mapCloudErr(err)
	}
	return summarizeDatabase(db), nil
}

func (h *digitalOceanHost) Redeploy(ctx context.Context, appResourceID string, in RedeployInput) (*AppSummary, error) {
	app, err := h.api.GetApp(ctx, appResourceID)
	if err != nil {
		return nil, mapCloudErr(err)
	}
	spec, err := digitalocean.ParseAppSpec(app.Spec)
	if err != nil {
		return nil, err
	}
	changed := false
	for i := range spec.Services {
		isAPI := spec.Services[i].Name == "api" || (spec.Services[i].Name == "" && i == 0)
		if !isAPI {
			continue
		}
		if spec.Services[i].Image == nil {
			spec.Services[i].Image = &digitalocean.ImageSource{}
		}
		if in.APIDigest != "" {
			spec.Services[i].Image.Digest = in.APIDigest
			changed = true
		}
		if in.APITag != "" {
			if in.APITag == "latest" {
				return nil, newValidationError("apiTag must not be latest")
			}
			spec.Services[i].Image.Tag = in.APITag
			changed = true
		}
		if in.ProductVersion != "" {
			setEnvValue(&spec.Services[i].Envs, "PRODUCT_VERSION", in.ProductVersion)
			changed = true
		}
		if in.APIRevisionCurrent > 0 {
			setEnvValue(&spec.Services[i].Envs, "API_REVISION_CURRENT", strconv.Itoa(in.APIRevisionCurrent))
			changed = true
		}
		if in.APIRevisionMin > 0 {
			setEnvValue(&spec.Services[i].Envs, "API_REVISION_MIN", strconv.Itoa(in.APIRevisionMin))
			changed = true
		}
	}
	for i := range spec.Workers {
		isWorker := spec.Workers[i].Name == "worker" || (spec.Workers[i].Name == "" && i == 0)
		if !isWorker {
			continue
		}
		if spec.Workers[i].Image == nil {
			spec.Workers[i].Image = &digitalocean.ImageSource{}
		}
		if in.WorkerDigest != "" {
			spec.Workers[i].Image.Digest = in.WorkerDigest
			changed = true
		}
		if in.WorkerTag != "" {
			if in.WorkerTag == "latest" {
				return nil, newValidationError("workerTag must not be latest")
			}
			spec.Workers[i].Image.Tag = in.WorkerTag
			changed = true
		}
		if in.ProductVersion != "" {
			setEnvValue(&spec.Workers[i].Envs, "PRODUCT_VERSION", in.ProductVersion)
			changed = true
		}
		if in.APIRevisionCurrent > 0 {
			setEnvValue(&spec.Workers[i].Envs, "API_REVISION_CURRENT", strconv.Itoa(in.APIRevisionCurrent))
			changed = true
		}
		if in.APIRevisionMin > 0 {
			setEnvValue(&spec.Workers[i].Envs, "API_REVISION_MIN", strconv.Itoa(in.APIRevisionMin))
			changed = true
		}
	}
	if changed {
		app, err = h.api.UpdateApp(ctx, appResourceID, spec)
		if err != nil {
			return nil, mapCloudErr(err)
		}
	}
	if err := h.api.CreateDeployment(ctx, appResourceID, in.ForceRebuild); err != nil {
		return nil, mapCloudErr(err)
	}
	return summarizeApp(app), nil
}

func (h *digitalOceanHost) ProvisionPeer(ctx context.Context, in ProvisionPeerHostInput) (*ProvisionPeerHostResult, error) {
	db, err := h.api.CreateDatabase(ctx, in.DisplayName+"-pg", in.Region, in.DatabaseSize, in.DatabaseNodes, "16")
	if err != nil {
		return nil, mapCloudErr(err)
	}
	dbURI := ""
	if db.Connection != nil {
		dbURI = db.Connection.URI
	}

	spec := &digitalocean.AppSpec{
		Name:   in.DisplayName,
		Region: in.Region,
		Services: []digitalocean.ComponentSpec{{
			Name:             "api",
			HTTPPort:         8080,
			InstanceCount:    1,
			InstanceSizeSlug: in.APISize,
			Routes:           []digitalocean.AppRoute{{Path: "/"}},
			Image: &digitalocean.ImageSource{
				RegistryType: "GHCR",
				Registry:     "majestanet",
				Repository:   "one-api",
				Tag:          in.APITag,
				Digest:       strings.TrimSpace(in.APIDigest),
			},
			Envs: peerEnvs(in.CustomerID, in.InstallID, in.InstallRole, in.ProductVersion, in.APIRevisionMin, in.APIRevisionCurrent, in.PublicURLHint, in.APIKeysSecret, in.JWTSigningKey, in.DeployShareSecret, dbURI),
		}},
		Workers: []digitalocean.ComponentSpec{{
			Name:             "worker",
			InstanceCount:    1,
			InstanceSizeSlug: in.WorkerSize,
			Image: &digitalocean.ImageSource{
				RegistryType: "GHCR",
				Registry:     "majestanet",
				Repository:   "one-worker",
				Tag:          in.WorkerTag,
				Digest:       strings.TrimSpace(in.WorkerDigest),
			},
			Envs: peerEnvs(in.CustomerID, in.InstallID, in.InstallRole, in.ProductVersion, in.APIRevisionMin, in.APIRevisionCurrent, "", in.APIKeysSecret, in.JWTSigningKey, "", dbURI),
		}},
	}

	app, err := h.api.CreateApp(ctx, spec)
	if err != nil {
		return nil, mapCloudErr(err)
	}
	publicURL := app.PublicURL()
	if publicURL != "" {
		if parsed, perr := digitalocean.ParseAppSpec(app.Spec); perr == nil {
			for i := range parsed.Services {
				setEnvValue(&parsed.Services[i].Envs, "PLATFORM_PUBLIC_URL", publicURL)
			}
			if updated, uerr := h.api.UpdateApp(ctx, app.ID, parsed); uerr == nil {
				app = updated
				publicURL = app.PublicURL()
			}
		}
	}
	return &ProvisionPeerHostResult{
		AppResourceID:      app.ID,
		DatabaseResourceID: db.ID,
		PublicURL:          publicURL,
		App:                summarizeApp(app),
	}, nil
}

func mapCloudErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, digitalocean.ErrNotConfigured) {
		return newValidationError("cloud credentials are not configured on this install")
	}
	var ae *digitalocean.APIError
	if errors.As(err, &ae) {
		switch ae.Status {
		case 401, 403:
			return newForbiddenError("cloud provider rejected credentials")
		case 404:
			return newNotFoundError("cloud resource not found")
		case 429:
			return fmt.Errorf("cloud provider rate limited: %s", ae.Message)
		default:
			return newValidationErrorf("cloud provider error: %s", ae.Message)
		}
	}
	return err
}

func summarizeApp(app *digitalocean.App) *AppSummary {
	if app == nil {
		return nil
	}
	out := &AppSummary{
		AppResourceID: app.ID,
		PublicURL:     app.PublicURL(),
	}
	if app.Region != nil {
		out.Region = app.Region.Slug
	}
	spec, err := digitalocean.ParseAppSpec(app.Spec)
	if err != nil || spec == nil {
		return out.withCompatAliases()
	}
	out.Name = spec.Name
	if out.Region == "" {
		out.Region = spec.Region
	}
	for _, s := range spec.Services {
		if s.Name == "api" || out.APIInstances == 0 {
			out.APIInstances = s.InstanceCount
			out.APISizeClass = AppSizeClassFromSlug(s.InstanceSizeSlug)
			out.APISize = s.InstanceSizeSlug
			if s.Image != nil {
				out.APIImageTag = s.Image.Tag
				out.APIImageDigest = s.Image.Digest
			}
			if s.Name == "api" {
				break
			}
		}
	}
	for _, w := range spec.Workers {
		if w.Name == "worker" || out.WorkerInstances == 0 {
			out.WorkerInstances = w.InstanceCount
			out.WorkerSizeClass = AppSizeClassFromSlug(w.InstanceSizeSlug)
			out.WorkerSize = w.InstanceSizeSlug
			if w.Image != nil {
				out.WorkerImageTag = w.Image.Tag
				out.WorkerDigest = w.Image.Digest
			}
			if w.Name == "worker" {
				break
			}
		}
	}
	return out.withCompatAliases()
}

func summarizeDatabase(db *digitalocean.Database) *DatabaseSummary {
	if db == nil {
		return nil
	}
	// Intentionally omit Connection (may contain password).
	return (&DatabaseSummary{
		DatabaseResourceID: db.ID,
		Name:               db.Name,
		Region:             db.Region,
		SizeClass:          DBSizeClassFromSlug(db.Size),
		Size:               db.Size,
		NumNodes:           db.NumNodes,
		Status:             db.Status,
	}).withCompatAliases()
}

func peerEnvs(customerID, installID, role, productVersion string, apiRevMin, apiRevCurrent int, publicURL, apiKeys, jwtKey, shareSecret, dbURI string) []digitalocean.AppEnvVar {
	if apiRevCurrent < 1 {
		apiRevCurrent = 1
	}
	if apiRevMin < 1 {
		apiRevMin = apiRevCurrent
	}
	envs := []digitalocean.AppEnvVar{
		{Key: "APP_ENV", Value: "production"},
		{Key: "PORT", Value: "8080"},
		{Key: "PRODUCT_VERSION", Value: productVersion},
		{Key: "API_REVISION_CURRENT", Value: strconv.Itoa(apiRevCurrent)},
		{Key: "API_REVISION_MIN", Value: strconv.Itoa(apiRevMin)},
		{Key: "CUSTOMER_ID", Value: customerID},
		{Key: "INSTALL_ID", Value: installID},
		{Key: "INSTALL_ROLE", Value: role},
		{Key: "AUTO_SEED", Value: "1"},
		{Key: "DEPLOY_PEER_MODE", Value: "allowlist"},
	}
	if publicURL != "" {
		envs = append(envs, digitalocean.AppEnvVar{Key: "PLATFORM_PUBLIC_URL", Value: publicURL})
	}
	if dbURI != "" {
		envs = append(envs, digitalocean.AppEnvVar{Key: "DATABASE_URL", Value: dbURI, Type: "SECRET"})
	} else {
		envs = append(envs, digitalocean.AppEnvVar{Key: "DATABASE_URL", Type: "SECRET"})
	}
	if apiKeys != "" {
		envs = append(envs, digitalocean.AppEnvVar{Key: "API_KEYS", Value: apiKeys, Type: "SECRET"})
	} else {
		envs = append(envs, digitalocean.AppEnvVar{Key: "API_KEYS", Type: "SECRET"})
	}
	if jwtKey != "" {
		envs = append(envs, digitalocean.AppEnvVar{Key: "AUTH_JWT_SIGNING_KEY", Value: jwtKey, Type: "SECRET"})
	} else {
		envs = append(envs, digitalocean.AppEnvVar{Key: "AUTH_JWT_SIGNING_KEY", Type: "SECRET"})
	}
	if shareSecret != "" {
		envs = append(envs, digitalocean.AppEnvVar{Key: "DEPLOY_SHARE_SECRET", Value: shareSecret, Type: "SECRET"})
	}
	return envs
}

func setEnvValue(envs *[]digitalocean.AppEnvVar, key, value string) {
	for i := range *envs {
		if (*envs)[i].Key == key {
			(*envs)[i].Value = value
			return
		}
	}
	*envs = append(*envs, digitalocean.AppEnvVar{Key: key, Value: value})
}
