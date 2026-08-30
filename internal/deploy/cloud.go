package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// GetCloudStatus returns adapter/binding status (never echoes credentials).
func (e *DeployEngine) GetCloudStatus(ctx context.Context) (*CloudStatus, error) {
	st := &CloudStatus{
		Host:       e.CloudHostName(),
		Configured: e.CloudConfigured(),
		Capabilities: map[string]bool{
			"bind":           true,
			"scaleApp":       true,
			"resizeDatabase": true,
			"provisionPeer":  true,
			"redeploy":       true,
		},
	}
	b, err := e.getCloudBinding(ctx)
	if err != nil {
		return nil, err
	}
	st.Binding = b
	if !st.Configured {
		return st, nil
	}
	ok, err := e.host.AccountOK(ctx)
	if err != nil {
		mapped := mapCloudErr(err)
		// Soft-fail known provider auth/not-found so status remains usable.
		if mapped != err {
			f := false
			st.Reachable = &f
			return st, nil
		}
		return nil, mapped
	}
	st.Reachable = &ok
	return st, nil
}

// PutCloudBinding binds this install to existing cloud app/database resource ids.
func (e *DeployEngine) PutCloudBinding(ctx context.Context, in BindInput) (*CloudBinding, error) {
	in.Normalize()
	appID := strings.TrimSpace(in.AppResourceID)
	dbID := strings.TrimSpace(in.DatabaseResourceID)
	if appID == "" && dbID == "" {
		return nil, newValidationError("appResourceId or databaseResourceId is required")
	}
	resolved := &CloudBinding{
		Host:               e.CloudHostName(),
		AppResourceID:      appID,
		DatabaseResourceID: dbID,
		Region:             strings.TrimSpace(in.Region),
		DisplayName:        strings.TrimSpace(in.DisplayName),
		ProviderMeta:       in.ProviderMeta,
	}
	if e.CloudConfigured() {
		r, err := e.host.ResolveBinding(ctx, in)
		if err != nil {
			return nil, err
		}
		resolved = r
		if resolved.Host == "" {
			resolved.Host = e.CloudHostName()
		}
	}
	if resolved.Host == "" {
		resolved.Host = "digitalocean"
	}
	return e.upsertCloudBinding(ctx, resolved)
}

// GetCloudApp returns a live summary for the bound app.
func (e *DeployEngine) GetCloudApp(ctx context.Context) (*AppSummary, error) {
	if err := e.requireCloud(); err != nil {
		return nil, err
	}
	b, err := e.requireBoundApp(ctx)
	if err != nil {
		return nil, err
	}
	return e.host.Describe(ctx, b.AppResourceID)
}

// ScaleCloudApp updates instance count/size on the bound app.
func (e *DeployEngine) ScaleCloudApp(ctx context.Context, in ScaleInput) (*AppSummary, error) {
	if err := e.requireCloud(); err != nil {
		return nil, err
	}
	if in.APIInstanceCount == nil && in.EffectiveAPISize() == nil &&
		in.WorkerInstanceCount == nil && in.EffectiveWorkerSize() == nil {
		return nil, newValidationError("at least one scale field is required")
	}
	b, err := e.requireBoundApp(ctx)
	if err != nil {
		return nil, err
	}
	return e.host.Scale(ctx, b.AppResourceID, in)
}

// ResizeCloudDatabase resizes the bound managed database.
func (e *DeployEngine) ResizeCloudDatabase(ctx context.Context, in ResizeDatabaseInput) (*DatabaseSummary, error) {
	if err := e.requireCloud(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.EffectiveSize()) == "" && in.NumNodes <= 0 {
		return nil, newValidationError("sizeClass/size and/or numNodes is required")
	}
	b, err := e.requireBoundDatabase(ctx)
	if err != nil {
		return nil, err
	}
	return e.host.ResizeDatabase(ctx, b.DatabaseResourceID, in)
}

// RedeployCloudApp is a temporary operator helper (product rolls prefer /ops/v1).
func (e *DeployEngine) RedeployCloudApp(ctx context.Context, in RedeployInput) (*AppSummary, error) {
	if err := e.requireCloud(); err != nil {
		return nil, err
	}
	b, err := e.requireBoundApp(ctx)
	if err != nil {
		return nil, err
	}
	if in.APIRevisionCurrent < 1 {
		in.APIRevisionCurrent = e.apiRevisionCurrent
	}
	if in.APIRevisionMin < 1 {
		in.APIRevisionMin = e.apiRevisionMin
	}
	return e.host.Redeploy(ctx, b.AppResourceID, in)
}

// ProvisionCloudEnvironment creates a peer app+DB and registers a Deploy peer.
func (e *DeployEngine) ProvisionCloudEnvironment(ctx context.Context, in ProvisionPeerInput) (*ProvisionPeerResult, error) {
	if err := e.requireCloud(); err != nil {
		return nil, err
	}
	in.Normalize()
	installID := strings.TrimSpace(in.InstallID)
	if installID == "" {
		return nil, newValidationError("installId is required")
	}
	if installID == e.installID {
		return nil, newValidationError("installId must differ from this install")
	}
	if strings.TrimSpace(in.APIKeysSecret) == "" {
		return nil, newValidationError("apiKeys secret is required for the peer install")
	}
	if strings.TrimSpace(in.JWTSigningKey) == "" {
		return nil, newValidationError("authJwtSigningKey is required for the peer install")
	}
	if peers, perr := e.ListPeers(ctx); perr == nil {
		for _, p := range peers {
			if p.InstallID == installID {
				return nil, newValidationError("installId already registered as a peer; choose a unique installId")
			}
		}
	}
	role := strings.TrimSpace(in.InstallRole)
	if role == "" {
		role = "dev"
	}
	region := strings.TrimSpace(in.Region)
	if region == "" {
		if b, _ := e.getCloudBinding(ctx); b != nil && b.Region != "" {
			region = b.Region
		} else {
			region = "nyc"
		}
	}
	appName := strings.TrimSpace(in.DisplayName)
	if appName == "" {
		appName = "one-" + role
	}
	dbSize := MapDBSizeClass(in.DatabaseSizeClass)
	if dbSize == "" {
		dbSize = MapDBSizeClass("small")
	}
	nodes := in.DatabaseNodes
	if nodes < 1 {
		nodes = 1
	}
	productVersion := strings.TrimSpace(in.ProductVersion)
	if productVersion == "" {
		productVersion = e.productVersion
	}
	apiTag := strings.TrimSpace(in.APITag)
	if apiTag == "" {
		apiTag = productVersion
	}
	workerTag := strings.TrimSpace(in.WorkerTag)
	if workerTag == "" {
		workerTag = productVersion
	}
	if apiTag == "latest" || workerTag == "latest" {
		return nil, newValidationError("image tags must not be latest")
	}
	apiSize := MapAppSizeClass(in.APISizeClass)
	if apiSize == "" {
		apiSize = MapAppSizeClass("small")
	}
	workerSize := MapAppSizeClass(in.WorkerSizeClass)
	if workerSize == "" {
		workerSize = MapAppSizeClass("small")
	}

	runID, err := e.insertProvisionRun(ctx, e.CloudHostName(), installID, role, in.CreatedBy)
	if err != nil {
		return nil, err
	}

	hostResult, err := e.host.ProvisionPeer(ctx, ProvisionPeerHostInput{
		CustomerID:         e.customerID,
		InstallID:          installID,
		InstallRole:        role,
		DisplayName:        appName,
		Region:             region,
		DatabaseSize:       dbSize,
		DatabaseNodes:      nodes,
		APISize:            apiSize,
		WorkerSize:         workerSize,
		APITag:             apiTag,
		WorkerTag:          workerTag,
		APIDigest:          strings.TrimSpace(in.APIDigest),
		WorkerDigest:       strings.TrimSpace(in.WorkerDigest),
		ProductVersion:     productVersion,
		APIRevisionMin:     e.apiRevisionMin,
		APIRevisionCurrent: e.apiRevisionCurrent,
		APIKeysSecret:      in.APIKeysSecret,
		JWTSigningKey:      in.JWTSigningKey,
		DeployShareSecret:  in.DeployShareSecret,
	})
	if err != nil {
		_ = e.failProvisionRun(ctx, runID, err.Error())
		return nil, err
	}

	publicURL := hostResult.PublicURL
	active := true
	label := appName
	var peerBase *string
	if publicURL != "" {
		if normalized, nerr := normalizePeerBaseURL(publicURL); nerr == nil {
			if serr := assertSafePeerBaseURL(normalized); serr == nil {
				peerBase = &normalized
			}
		}
	}
	peer, err := e.UpsertPeer(ctx, struct {
		InstallID   string
		Label       *string
		InstallRole *string
		BaseURL     *string
		Active      *bool
		CustomerID  *string
	}{
		InstallID:   installID,
		Label:       &label,
		InstallRole: &role,
		BaseURL:     peerBase,
		Active:      &active,
	})
	if err != nil {
		_ = e.failProvisionRun(ctx, runID, err.Error())
		return nil, err
	}
	_ = e.completeProvisionRun(ctx, runID, hostResult.AppResourceID, hostResult.DatabaseResourceID, publicURL)

	return (&ProvisionPeerResult{
		Peer:               peer,
		AppResourceID:      hostResult.AppResourceID,
		DatabaseResourceID: hostResult.DatabaseResourceID,
		PublicURL:          publicURL,
		RunID:              runID,
		App:                hostResult.App,
	}).withCompatAliases(), nil
}

// ListCloudEnvironments returns peers plus provision audit rows.
func (e *DeployEngine) ListCloudEnvironments(ctx context.Context) (*EnvironmentList, error) {
	peers, err := e.ListPeers(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := e.pool.Query(ctx, `
		SELECT id::text, COALESCE(host,''), peer_install_id, COALESCE(install_role,''),
		       COALESCE(app_resource_id,''), COALESCE(database_resource_id,''),
		       COALESCE(base_url,''), status, COALESCE(error,''), created_at
		FROM deploy_cloud_provision_runs
		ORDER BY created_at DESC
		LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []ProvisionRun
	for rows.Next() {
		var r ProvisionRun
		if err := rows.Scan(&r.ID, &r.Host, &r.PeerInstallID, &r.InstallRole, &r.AppResourceID, &r.DatabaseResourceID, &r.BaseURL, &r.Status, &r.Error, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.AppID = r.AppResourceID
		r.DatabaseID = r.DatabaseResourceID
		runs = append(runs, r)
	}
	if runs == nil {
		runs = []ProvisionRun{}
	}
	if peers == nil {
		peers = []PeerRow{}
	}
	return &EnvironmentList{Peers: peers, ProvisionRuns: runs}, nil
}

// ---- DigitalOcean compatibility wrappers (alias routes / older callers) ----

// DigitalOceanCloudBinding is a deprecated alias shape kept for older clients.
type DigitalOceanCloudBinding = CloudBinding

// DigitalOceanCloudStatus is a deprecated alias shape.
type DigitalOceanCloudStatus = CloudStatus

// DigitalOceanAppSummary is a deprecated alias shape.
type DigitalOceanAppSummary = AppSummary

// ScaleDigitalOceanAppInput is a deprecated alias shape.
type ScaleDigitalOceanAppInput = ScaleInput

// ResizeDigitalOceanDatabaseInput is a deprecated alias shape.
type ResizeDigitalOceanDatabaseInput = ResizeDatabaseInput

// RedeployDigitalOceanAppInput is a deprecated alias shape.
type RedeployDigitalOceanAppInput = RedeployInput

// ProvisionDigitalOceanEnvironmentInput is a deprecated alias shape.
type ProvisionDigitalOceanEnvironmentInput = ProvisionPeerInput

// ProvisionDigitalOceanEnvironmentResult is a deprecated alias shape.
type ProvisionDigitalOceanEnvironmentResult = ProvisionPeerResult

// GetDigitalOceanStatus is a compatibility wrapper.
func (e *DeployEngine) GetDigitalOceanStatus(ctx context.Context) (*CloudStatus, error) {
	return e.GetCloudStatus(ctx)
}

// PutDigitalOceanBinding is a compatibility wrapper.
func (e *DeployEngine) PutDigitalOceanBinding(ctx context.Context, appID, databaseID, region, appName string) (*CloudBinding, error) {
	return e.PutCloudBinding(ctx, BindInput{AppID: appID, DatabaseID: databaseID, Region: region, AppName: appName})
}

// GetDigitalOceanApp is a compatibility wrapper.
func (e *DeployEngine) GetDigitalOceanApp(ctx context.Context) (*AppSummary, error) {
	return e.GetCloudApp(ctx)
}

// ScaleDigitalOceanApp is a compatibility wrapper.
func (e *DeployEngine) ScaleDigitalOceanApp(ctx context.Context, in ScaleInput) (*AppSummary, error) {
	return e.ScaleCloudApp(ctx, in)
}

// ResizeDigitalOceanDatabase is a compatibility wrapper returning a safe summary.
func (e *DeployEngine) ResizeDigitalOceanDatabase(ctx context.Context, in ResizeDatabaseInput) (*DatabaseSummary, error) {
	return e.ResizeCloudDatabase(ctx, in)
}

// RedeployDigitalOceanApp is a compatibility wrapper.
func (e *DeployEngine) RedeployDigitalOceanApp(ctx context.Context, in RedeployInput) (*AppSummary, error) {
	return e.RedeployCloudApp(ctx, in)
}

// ProvisionDigitalOceanEnvironment is a compatibility wrapper.
func (e *DeployEngine) ProvisionDigitalOceanEnvironment(ctx context.Context, in ProvisionPeerInput) (*ProvisionPeerResult, error) {
	return e.ProvisionCloudEnvironment(ctx, in)
}

// ListDigitalOceanEnvironments is a compatibility wrapper returning map shape for older clients.
func (e *DeployEngine) ListDigitalOceanEnvironments(ctx context.Context) (map[string]any, error) {
	list, err := e.ListCloudEnvironments(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"peers": list.Peers, "provisionRuns": list.ProvisionRuns}, nil
}

func (e *DeployEngine) requireCloud() error {
	if !e.CloudConfigured() {
		return newValidationError("cloud credentials are not configured on this install")
	}
	return nil
}

func (e *DeployEngine) requireBoundApp(ctx context.Context) (*CloudBinding, error) {
	b, err := e.getCloudBinding(ctx)
	if err != nil {
		return nil, err
	}
	if b == nil || b.AppResourceID == "" {
		return nil, newValidationError("no cloud app bound; PUT /deploy/v1/cloud/binding first")
	}
	return b, nil
}

func (e *DeployEngine) requireBoundDatabase(ctx context.Context) (*CloudBinding, error) {
	b, err := e.getCloudBinding(ctx)
	if err != nil {
		return nil, err
	}
	if b == nil || b.DatabaseResourceID == "" {
		return nil, newValidationError("no cloud database bound; PUT /deploy/v1/cloud/binding first")
	}
	return b, nil
}

func (e *DeployEngine) getCloudBinding(ctx context.Context) (*CloudBinding, error) {
	var b CloudBinding
	var appID, dbID, region, name, host *string
	var meta []byte
	err := e.pool.QueryRow(ctx, `
		SELECT host, app_resource_id, database_resource_id, region, display_name, provider_meta, updated_at, created_at
		FROM deploy_cloud_binding WHERE singleton = true`).Scan(
		&host, &appID, &dbID, &region, &name, &meta, &b.UpdatedAt, &b.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Fallback for installs that have not yet applied 0036.
			return e.getLegacyDigitalOceanBinding(ctx)
		}
		return nil, err
	}
	if host != nil {
		b.Host = *host
	}
	if appID != nil {
		b.AppResourceID = *appID
	}
	if dbID != nil {
		b.DatabaseResourceID = *dbID
	}
	if region != nil {
		b.Region = *region
	}
	if name != nil {
		b.DisplayName = *name
	}
	if len(meta) > 0 && string(meta) != "null" {
		_ = json.Unmarshal(meta, &b.ProviderMeta)
	}
	if b.AppResourceID == "" && b.DatabaseResourceID == "" {
		return nil, nil
	}
	return b.withCompatAliases(), nil
}

func (e *DeployEngine) getLegacyDigitalOceanBinding(ctx context.Context) (*CloudBinding, error) {
	var b CloudBinding
	var appID, dbID, region, appName *string
	err := e.pool.QueryRow(ctx, `
		SELECT app_id, database_id, region, app_name, updated_at, created_at
		FROM deploy_digitalocean_cloud WHERE singleton = true`).Scan(
		&appID, &dbID, &region, &appName, &b.UpdatedAt, &b.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	b.Host = "digitalocean"
	if appID != nil {
		b.AppResourceID = *appID
	}
	if dbID != nil {
		b.DatabaseResourceID = *dbID
	}
	if region != nil {
		b.Region = *region
	}
	if appName != nil {
		b.DisplayName = *appName
	}
	if b.AppResourceID == "" && b.DatabaseResourceID == "" {
		return nil, nil
	}
	return b.withCompatAliases(), nil
}

func (e *DeployEngine) upsertCloudBinding(ctx context.Context, in *CloudBinding) (*CloudBinding, error) {
	existing, _ := e.getCloudBinding(ctx)
	if existing != nil {
		if in.AppResourceID == "" {
			in.AppResourceID = existing.AppResourceID
		}
		if in.DatabaseResourceID == "" {
			in.DatabaseResourceID = existing.DatabaseResourceID
		}
		if in.Region == "" {
			in.Region = existing.Region
		}
		if in.DisplayName == "" {
			in.DisplayName = existing.DisplayName
		}
		if in.Host == "" {
			in.Host = existing.Host
		}
		if in.ProviderMeta == nil {
			in.ProviderMeta = existing.ProviderMeta
		}
	}
	if in.Host == "" {
		in.Host = e.CloudHostName()
	}
	if in.Host == "" {
		in.Host = "digitalocean"
	}
	meta, _ := json.Marshal(in.ProviderMeta)
	if in.ProviderMeta == nil {
		meta = []byte("{}")
	}
	_, err := e.pool.Exec(ctx, `
		INSERT INTO deploy_cloud_binding (singleton, host, app_resource_id, database_resource_id, region, display_name, provider_meta, updated_at, created_at)
		VALUES (true, $1, NULLIF($2,''), NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), $6::jsonb, now(), now())
		ON CONFLICT (singleton) DO UPDATE SET
		  host = EXCLUDED.host,
		  app_resource_id = EXCLUDED.app_resource_id,
		  database_resource_id = EXCLUDED.database_resource_id,
		  region = COALESCE(EXCLUDED.region, deploy_cloud_binding.region),
		  display_name = COALESCE(EXCLUDED.display_name, deploy_cloud_binding.display_name),
		  provider_meta = EXCLUDED.provider_meta,
		  updated_at = now()`,
		in.Host, in.AppResourceID, in.DatabaseResourceID, in.Region, in.DisplayName, string(meta))
	if err != nil {
		// Fallback write to legacy table when 0036 is not applied yet.
		if _, err2 := e.pool.Exec(ctx, `
			INSERT INTO deploy_digitalocean_cloud (singleton, app_id, database_id, region, app_name, updated_at, created_at)
			VALUES (true, NULLIF($1,''), NULLIF($2,''), NULLIF($3,''), NULLIF($4,''), now(), now())
			ON CONFLICT (singleton) DO UPDATE SET
			  app_id = EXCLUDED.app_id,
			  database_id = EXCLUDED.database_id,
			  region = COALESCE(EXCLUDED.region, deploy_digitalocean_cloud.region),
			  app_name = COALESCE(EXCLUDED.app_name, deploy_digitalocean_cloud.app_name),
			  updated_at = now()`,
			in.AppResourceID, in.DatabaseResourceID, in.Region, in.DisplayName); err2 != nil {
			return nil, err
		}
		return e.getCloudBinding(ctx)
	}
	// Keep legacy table in sync for older tooling.
	_, _ = e.pool.Exec(ctx, `
		INSERT INTO deploy_digitalocean_cloud (singleton, app_id, database_id, region, app_name, updated_at, created_at)
		VALUES (true, NULLIF($1,''), NULLIF($2,''), NULLIF($3,''), NULLIF($4,''), now(), now())
		ON CONFLICT (singleton) DO UPDATE SET
		  app_id = EXCLUDED.app_id,
		  database_id = EXCLUDED.database_id,
		  region = COALESCE(EXCLUDED.region, deploy_digitalocean_cloud.region),
		  app_name = COALESCE(EXCLUDED.app_name, deploy_digitalocean_cloud.app_name),
		  updated_at = now()`,
		in.AppResourceID, in.DatabaseResourceID, in.Region, in.DisplayName)
	return e.getCloudBinding(ctx)
}

func (e *DeployEngine) insertProvisionRun(ctx context.Context, host, peerInstallID, role string, createdBy *string) (string, error) {
	if host == "" {
		host = "digitalocean"
	}
	var id string
	err := e.pool.QueryRow(ctx, `
		INSERT INTO deploy_cloud_provision_runs (host, peer_install_id, install_role, status, created_by)
		VALUES ($1, $2, $3, 'pending', $4)
		RETURNING id::text`, host, peerInstallID, role, createdBy).Scan(&id)
	if err != nil {
		err2 := e.pool.QueryRow(ctx, `
			INSERT INTO deploy_digitalocean_provision_runs (peer_install_id, install_role, status, created_by)
			VALUES ($1, $2, 'pending', $3)
			RETURNING id::text`, peerInstallID, role, createdBy).Scan(&id)
		return id, err2
	}
	return id, nil
}

func (e *DeployEngine) failProvisionRun(ctx context.Context, id, msg string) error {
	_, err := e.pool.Exec(ctx, `
		UPDATE deploy_cloud_provision_runs SET status='failed', error=$2 WHERE id=$1::uuid`, id, truncate(msg, 2000))
	if err != nil {
		_, err = e.pool.Exec(ctx, `
			UPDATE deploy_digitalocean_provision_runs SET status='failed', error=$2 WHERE id=$1::uuid`, id, truncate(msg, 2000))
	}
	return err
}

func (e *DeployEngine) completeProvisionRun(ctx context.Context, id, appID, dbID, baseURL string) error {
	_, err := e.pool.Exec(ctx, `
		UPDATE deploy_cloud_provision_runs
		SET status='succeeded', app_resource_id=$2, database_resource_id=$3, base_url=$4, error=NULL
		WHERE id=$1::uuid`, id, appID, dbID, baseURL)
	if err != nil {
		_, err = e.pool.Exec(ctx, `
			UPDATE deploy_digitalocean_provision_runs
			SET status='succeeded', app_id=$2, database_id=$3, base_url=$4, error=NULL
			WHERE id=$1::uuid`, id, appID, dbID, baseURL)
	} else {
		_, _ = e.pool.Exec(ctx, `
			UPDATE deploy_digitalocean_provision_runs
			SET status='succeeded', app_id=$2, database_id=$3, base_url=$4, error=NULL
			WHERE id=$1::uuid`, id, appID, dbID, baseURL)
	}
	return err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
