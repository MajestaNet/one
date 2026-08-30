package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/compat"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/deploy/gitremote"
	"github.com/MajestaNet/ide/internal/metadata"
)

// Options configures a DeployEngine for a particular install.
type Options struct {
	InstallID            string
	InstallRole          string
	ProductVersion       string
	APIRevisionMin       int
	APIRevisionCurrent   int
	CustomerID           string
	PeerMode             PeerMode
	ShareSecret          string
	CustomerRepoURL      string
	CustomerRepoProvider string
	CustomerRepoRegion   string
	// CustomerRepoGitUser / CustomerRepoGitToken are optional HTTPS credentials for initialize-repo.
	CustomerRepoGitUser  string
	CustomerRepoGitToken string
	// RequiredTestSuites must pass (sync) before a non-dry-run promote applies (ADR-014 Phase 5).
	// Empty means no gate (backward compatible). Set via DEPLOY_REQUIRED_TEST_SUITES.
	RequiredTestSuites []string
	// SyncMaxFiles / SyncMaxBytes gate in-request org validate/apply (BP-033 Phase 1).
	SyncMaxFiles   int
	SyncMaxBytes   int64
	QueueMax       int
	JobSlotsDeploy int
}

// DeployEngine implements all deploy surface methods, mirroring the Node DeployEngine.
type DeployEngine struct {
	pool                 *db.Pool
	meta                 *metadata.Service
	data                 *dataengine.Service
	installID            string
	installRole          string
	productVersion       string
	apiRevisionMin       int
	apiRevisionCurrent   int
	customerID           string
	peerMode             PeerMode
	shareSecret          string
	customerRepoURL      string
	customerRepoProvider string
	customerRepoRegion   string
	requiredTestSuites   []string
	syncMaxFiles         int
	syncMaxBytes         int64
	queueMax             int
	jobSlotsDeploy       int
	host                 CloudHost
	cloud                CloudAPI // DO low-level client when host is digitalocean (tests/legacy)
	gitRemote            gitremote.Remote
}

// NewDeployEngine constructs a DeployEngine.
func NewDeployEngine(pool *db.Pool, meta *metadata.Service, data *dataengine.Service, opts Options) *DeployEngine {
	pm := opts.PeerMode
	if pm == "" {
		pm = PeerModeCustomer
	}
	if opts.InstallID == "" {
		opts.InstallID = "local"
	}
	if opts.InstallRole == "" {
		opts.InstallRole = "dev"
	}
	if opts.ProductVersion == "" {
		opts.ProductVersion = "0.1.0"
	}
	if opts.CustomerID == "" {
		opts.CustomerID = "local-customer"
	}
	apiMin := opts.APIRevisionMin
	apiCurrent := opts.APIRevisionCurrent
	if apiCurrent < 1 {
		apiCurrent = 1
	}
	if apiMin < 1 {
		apiMin = apiCurrent
	}
	if apiMin > apiCurrent {
		apiMin = apiCurrent
	}
	if opts.SyncMaxFiles <= 0 {
		opts.SyncMaxFiles = DefaultSyncMaxFiles
	}
	if opts.SyncMaxBytes <= 0 {
		opts.SyncMaxBytes = DefaultSyncMaxBytes
	}
	if opts.QueueMax <= 0 {
		opts.QueueMax = DefaultQueueMax
	}
	if opts.JobSlotsDeploy <= 0 {
		opts.JobSlotsDeploy = DefaultJobSlotsDeploy
	}
	eng := &DeployEngine{
		pool:                 pool,
		meta:                 meta,
		data:                 data,
		installID:            opts.InstallID,
		installRole:          opts.InstallRole,
		productVersion:       opts.ProductVersion,
		apiRevisionMin:       apiMin,
		apiRevisionCurrent:   apiCurrent,
		customerID:           opts.CustomerID,
		peerMode:             pm,
		shareSecret:          opts.ShareSecret,
		customerRepoURL:      opts.CustomerRepoURL,
		customerRepoProvider: opts.CustomerRepoProvider,
		customerRepoRegion:   opts.CustomerRepoRegion,
		requiredTestSuites:   append([]string{}, opts.RequiredTestSuites...),
		syncMaxFiles:         opts.SyncMaxFiles,
		syncMaxBytes:         opts.SyncMaxBytes,
		queueMax:             opts.QueueMax,
		jobSlotsDeploy:       opts.JobSlotsDeploy,
	}
	if opts.CustomerRepoURL != "" {
		var auth *gitremote.Auth
		if opts.CustomerRepoGitUser != "" || opts.CustomerRepoGitToken != "" {
			auth = &gitremote.Auth{Username: opts.CustomerRepoGitUser, Password: opts.CustomerRepoGitToken}
		}
		if rem, err := gitremote.NewGoGitRemote(opts.CustomerRepoURL, auth); err == nil {
			eng.gitRemote = rem
		}
	}
	return eng
}

// EnvironmentInfo is the response of GetEnvironment.
type EnvironmentInfo struct {
	ProductVersion       string                   `json:"productVersion"`
	ApiRevision          compat.APIRevisionWindow `json:"apiRevision"`
	CustomerID           string                   `json:"customerId"`
	InstallID            string                   `json:"installId"`
	InstallRole          string                   `json:"installRole"`
	PeerMode             PeerMode                 `json:"peerMode"`
	SignatureRequired    bool                     `json:"signatureRequired"`
	ApiFamilies          []string                 `json:"apiFamilies"`
	Phase                string                   `json:"phase"`
	CloudHost            string                   `json:"cloudHost,omitempty"`
	Capabilities         map[string]bool          `json:"capabilities"`
	Notes                string                   `json:"notes"`
	CustomerRepoURL      string                   `json:"customerRepoUrl,omitempty"`
	CustomerRepoProvider string                   `json:"customerRepoProvider,omitempty"`
	CustomerRepoRegion   string                   `json:"customerRepoRegion,omitempty"`
}

// GetEnvironment returns install identity and capabilities.
func (e *DeployEngine) GetEnvironment() *EnvironmentInfo {
	hostName := e.CloudHostName()
	cloudOn := e.CloudConfigured()
	caps := map[string]bool{
		"bundles":                 true,
		"promotions":              true,
		"customerTests":           true,
		"environmentRead":         true,
		"crossEnvironmentPromote": false,
		"multiEnvironment":        true,
		"productUpgrades":         true,
		"packagePack":             true,
		"packageExport":           true,
		"packageInitializeRepo":   e.customerRepoURL != "" && e.gitRemote != nil,
		"cloud":                   cloudOn,
		"digitaloceanCloud":       cloudOn && hostName == "digitalocean",
	}
	return &EnvironmentInfo{
		ProductVersion: e.productVersion,
		ApiRevision: compat.APIRevisionWindow{
			Min:     e.apiRevisionMin,
			Current: e.apiRevisionCurrent,
		},
		CustomerID:           e.customerID,
		InstallID:            e.installID,
		InstallRole:          e.installRole,
		PeerMode:             e.peerMode,
		SignatureRequired:    e.shareSecret != "",
		ApiFamilies:          []string{"client", "metadata", "deploy", "ops"},
		Phase:                "F",
		CloudHost:            hostName,
		Capabilities:         caps,
		Notes:                "Any number of installs may share CUSTOMER_ID. Customizations ship repo → org (validate then deploy on the connected install). Peer push and inbound artifact promote are not used. Product image upgrades use /ops/v1 (ADR-007), not Deploy promotions. Customer Git uses one/v1 (ADR-012). Day-2 cloud ops use host-free /deploy/v1/cloud/*.",
		CustomerRepoURL:      e.customerRepoURL,
		CustomerRepoProvider: e.customerRepoProvider,
		CustomerRepoRegion:   e.customerRepoRegion,
	}
}

// ---- Bundle rows ----

// BundleRow is a row from deploy_bundles.
type BundleRow struct {
	ID                  string          `json:"id"`
	Label               *string         `json:"label,omitempty"`
	CustomerID          string          `json:"customerId"`
	SourceInstallID     string          `json:"sourceInstallId"`
	SourceInstallRole   *string         `json:"sourceInstallRole,omitempty"`
	Origin              string          `json:"origin"`
	ProductVersion      string          `json:"productVersion"`
	ProductVersionRange string          `json:"productVersionRange"`
	Checksum            string          `json:"checksum"`
	Signature           *string         `json:"signature,omitempty"`
	Artifact            json.RawMessage `json:"artifact"`
	Status              string          `json:"status"`
	CreatedBy           *string         `json:"createdBy,omitempty"`
	CreatedAt           time.Time       `json:"createdAt"`
}

// BundleListRow is a bundle without the artifact blob.
type BundleListRow struct {
	ID                  string    `json:"id"`
	Label               *string   `json:"label,omitempty"`
	CustomerID          string    `json:"customerId"`
	SourceInstallID     string    `json:"sourceInstallId"`
	SourceInstallRole   *string   `json:"sourceInstallRole,omitempty"`
	Origin              string    `json:"origin"`
	ProductVersion      string    `json:"productVersion"`
	ProductVersionRange string    `json:"productVersionRange"`
	Checksum            string    `json:"checksum"`
	Status              string    `json:"status"`
	CreatedBy           *string   `json:"createdBy,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
}

func (e *DeployEngine) insertBundle(ctx context.Context,
	label *string,
	artifact *BundleArtifact,
	productVersionRange, origin string,
	checksum string,
	signature *string,
	createdBy *string,
) (*BundleRow, error) {
	artifactJSON, err := json.Marshal(artifact)
	if err != nil {
		return nil, fmt.Errorf("marshal artifact: %w", err)
	}
	var row BundleRow
	err = e.pool.QueryRow(ctx, `
INSERT INTO deploy_bundles (
  label, customer_id, source_install_id, source_install_role, origin,
  product_version, product_version_range, checksum, signature, artifact, status, created_by
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'ready',$11)
RETURNING id::text, label, customer_id, source_install_id, source_install_role, origin,
          product_version, product_version_range, checksum, signature, artifact, status, created_by, created_at`,
		label,
		e.customerID,
		e.installID,
		strPtrOrNil(e.installRole),
		origin,
		e.productVersion,
		productVersionRange,
		checksum,
		signature,
		string(artifactJSON),
		createdBy,
	).Scan(
		&row.ID, &row.Label, &row.CustomerID, &row.SourceInstallID, &row.SourceInstallRole,
		&row.Origin, &row.ProductVersion, &row.ProductVersionRange, &row.Checksum,
		&row.Signature, &row.Artifact, &row.Status, &row.CreatedBy, &row.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// BuildSnapshotArtifact exports current customer metadata as a BundleArtifact without storing a bundle.
func (e *DeployEngine) BuildSnapshotArtifact(ctx context.Context, productVersionRange string) (*BundleArtifact, string /*range*/, error) {
	snapshot, err := e.meta.ExportCustomerSnapshot(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("export snapshot: %w", err)
	}
	rangeStr := productVersionRange
	if rangeStr == "" {
		rangeStr = ">=" + e.productVersion
	}
	customerID := e.customerID
	sourceID := e.installID
	sourceRole := e.installRole
	createdAt := time.Now().UTC().Format(time.RFC3339)

	rawArtifact := map[string]any{
		"manifestVersion":     1,
		"ownership":           "custom",
		"defaultPackageName":  DefaultCustomerPackage,
		"customerId":          customerID,
		"productVersionRange": rangeStr,
		"sourceInstallId":     sourceID,
		"sourceInstallRole":   sourceRole,
		"createdAt":           createdAt,
	}
	for k, v := range snapshot {
		if _, exists := rawArtifact[k]; !exists {
			rawArtifact[k] = v
		}
	}
	// Merge snapshot arrays (objects/fields/etc.)
	for _, key := range []string{"objects", "fields", "validationRules", "automations", "agentPlaybooks", "tools", "canvases", "permissionSets", "webhooks", "connectors", "sources", "dataRoles", "objectSharingSettings", "sharingRules", "baseline"} {
		if v, ok := snapshot[key]; ok {
			rawArtifact[key] = v
		}
	}
	// Stamp baseline with product version + install identity for export/init trees.
	if bl, ok := rawArtifact["baseline"].(map[string]any); ok {
		if bl["productVersion"] == nil || bl["productVersion"] == "" {
			bl["productVersion"] = e.productVersion
		}
		if sourceID != "" {
			bl["sourceInstallId"] = sourceID
		}
		rawArtifact["baseline"] = bl
	}

	// Embed customer-owned test suites.
	suites, err := e.ListTestSuites(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("list test suites: %w", err)
	}
	tests := make([]map[string]any, 0)
	for _, s := range suites {
		if s.Ownership != "custom" {
			continue
		}
		var steps any
		if len(s.Steps) > 0 {
			_ = json.Unmarshal(s.Steps, &steps)
		}
		if steps == nil {
			steps = []any{}
		}
		t := map[string]any{
			"apiName": s.APIName, "label": s.Label, "active": s.Active,
			"steps": steps, "ownership": "custom",
		}
		if s.Description != nil {
			t["description"] = *s.Description
		}
		if s.PackageName != nil {
			t["packageName"] = *s.PackageName
		}
		tests = append(tests, t)
	}
	rawArtifact["tests"] = tests

	// Merge persisted guest sources (tests/automations + any src not already in snapshot).
	persisted, err := LoadCustomerSources(ctx, e.pool)
	if err != nil {
		return nil, "", fmt.Errorf("load customer sources: %w", err)
	}
	mergedSources := map[string]string{}
	if existing, ok := rawArtifact["sources"].(map[string]string); ok {
		for k, v := range existing {
			mergedSources[k] = v
		}
	} else if existingAny, ok := rawArtifact["sources"].(map[string]any); ok {
		for k, v := range existingAny {
			if s, ok := v.(string); ok {
				mergedSources[k] = s
			}
		}
	}
	for k, v := range persisted {
		mergedSources[k] = v
	}
	rawArtifact["sources"] = mergedSources

	artifact, err := ParseBundleArtifact(rawArtifact)
	if err != nil {
		return nil, "", fmt.Errorf("parse snapshot artifact: %w", err)
	}
	return artifact, rangeStr, nil
}

// CreateBundleFromSnapshot exports current customer metadata as a bundle artifact.
func (e *DeployEngine) CreateBundleFromSnapshot(ctx context.Context, input struct {
	Label               *string
	CreatedBy           *string
	ProductVersionRange string
}) (*BundleRow, error) {
	artifact, rangeStr, err := e.BuildSnapshotArtifact(ctx, input.ProductVersionRange)
	if err != nil {
		return nil, err
	}
	checksum, err := checksumArtifact(artifact)
	if err != nil {
		return nil, err
	}
	var sig *string
	if e.shareSecret != "" {
		s, err := SignArtifact(artifact, e.shareSecret)
		if err != nil {
			return nil, err
		}
		sig = &s
	}
	return e.insertBundle(ctx, input.Label, artifact, rangeStr, "local", checksum, sig, input.CreatedBy)
}

// CreateBundleFromArtifact stores an externally-provided artifact as a bundle.
func (e *DeployEngine) CreateBundleFromArtifact(ctx context.Context, input struct {
	Artifact  any
	Label     *string
	CreatedBy *string
	Origin    string
	Signature *string
}) (*BundleRow, error) {
	artifact, err := ParseBundleArtifact(input.Artifact)
	if err != nil {
		return nil, err
	}
	// Stamp missing identity fields.
	if artifact.ProductVersionRange == nil || *artifact.ProductVersionRange == "" {
		s := ">=" + e.productVersion
		artifact.ProductVersionRange = &s
	}
	if artifact.CustomerID == nil {
		artifact.CustomerID = &e.customerID
	}
	if artifact.SourceInstallID == nil {
		artifact.SourceInstallID = &e.installID
	}
	if artifact.SourceInstallRole == nil {
		artifact.SourceInstallRole = &e.installRole
	}
	if artifact.CreatedAt == nil {
		s := time.Now().UTC().Format(time.RFC3339)
		artifact.CreatedAt = &s
	}

	checksum, err := checksumArtifact(artifact)
	if err != nil {
		return nil, err
	}
	sig := input.Signature
	if sig == nil && e.shareSecret != "" {
		s, err := SignArtifact(artifact, e.shareSecret)
		if err != nil {
			return nil, err
		}
		sig = &s
	}
	origin := input.Origin
	if origin == "" {
		origin = "local"
	}
	return e.insertBundle(ctx, input.Label, artifact, *artifact.ProductVersionRange, origin, checksum, sig, input.CreatedBy)
}

// ListBundles returns up to limit bundles, newest first.
func (e *DeployEngine) ListBundles(ctx context.Context, limit int) ([]BundleListRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := e.pool.Query(ctx, `
SELECT id::text, label, customer_id, source_install_id, source_install_role, origin,
       product_version, product_version_range, checksum, status, created_by, created_at
FROM deploy_bundles ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BundleListRow
	for rows.Next() {
		var r BundleListRow
		if err := rows.Scan(
			&r.ID, &r.Label, &r.CustomerID, &r.SourceInstallID, &r.SourceInstallRole,
			&r.Origin, &r.ProductVersion, &r.ProductVersionRange, &r.Checksum,
			&r.Status, &r.CreatedBy, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetBundle returns a bundle by ID, including its artifact blob.
func (e *DeployEngine) GetBundle(ctx context.Context, id string) (*BundleRow, error) {
	var row BundleRow
	err := e.pool.QueryRow(ctx, `
SELECT id::text, label, customer_id, source_install_id, source_install_role, origin,
       product_version, product_version_range, checksum, signature, artifact, status, created_by, created_at
FROM deploy_bundles WHERE id = $1::uuid`, id).Scan(
		&row.ID, &row.Label, &row.CustomerID, &row.SourceInstallID, &row.SourceInstallRole,
		&row.Origin, &row.ProductVersion, &row.ProductVersionRange, &row.Checksum,
		&row.Signature, &row.Artifact, &row.Status, &row.CreatedBy, &row.CreatedAt,
	)
	if err != nil {
		return nil, newNotFoundError(fmt.Sprintf("Bundle not found: %s", id))
	}
	return &row, nil
}

// ValidateBundleResult is returned from ValidateBundle.
type ValidateBundleResult struct {
	BundleID string            `json:"bundleId"`
	Report   *ValidationReport `json:"report"`
}

// ValidateBundle validates a stored bundle against current metadata.
func (e *DeployEngine) ValidateBundle(ctx context.Context, bundleID string) (*ValidateBundleResult, error) {
	bundle, err := e.GetBundle(ctx, bundleID)
	if err != nil {
		return nil, err
	}
	artifact, err := ParseBundleArtifact(bundle.Artifact)
	if err != nil {
		return nil, err
	}
	report, err := ValidateBundleArtifact(ctx, e.meta, artifact, e.productVersion, bundle.ProductVersionRange)
	if err != nil {
		return nil, err
	}
	return &ValidateBundleResult{BundleID: bundleID, Report: report}, nil
}

// ---- Promotions ----

// PromotionRow is a row from deploy_promotions.
type PromotionRow struct {
	ID               string          `json:"id"`
	BundleID         string          `json:"bundleId"`
	Status           string          `json:"status"`
	DryRun           bool            `json:"dryRun"`
	Direction        string          `json:"direction"`
	SourceInstallID  *string         `json:"sourceInstallId,omitempty"`
	ValidationReport json.RawMessage `json:"validationReport,omitempty"`
	ApplyReport      json.RawMessage `json:"applyReport,omitempty"`
	Error            *string         `json:"error,omitempty"`
	CreatedBy        *string         `json:"createdBy,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
	CompletedAt      *time.Time      `json:"completedAt,omitempty"`
}

// PromoteBundleResult is returned from PromoteBundle.
type PromoteBundleResult struct {
	Promotion  *PromotionRow     `json:"promotion"`
	Validation *ValidationReport `json:"validation"`
	Apply      *ApplyReport      `json:"apply,omitempty"`
}

func (e *DeployEngine) insertPromotion(
	ctx context.Context,
	bundleID string,
	dryRun bool,
	direction string,
	sourceInstallID *string,
	validationReport *ValidationReport,
	createdBy *string,
) (*PromotionRow, error) {
	vrJSON, _ := json.Marshal(validationReport)
	var row PromotionRow
	err := e.pool.QueryRow(ctx, `
INSERT INTO deploy_promotions (bundle_id, status, dry_run, direction, source_install_id, validation_report, created_by)
VALUES ($1::uuid,'pending',$2,$3,$4,$5,$6)
RETURNING id::text, bundle_id::text, status, dry_run, direction, source_install_id,
          validation_report, apply_report, error, created_by, created_at, completed_at`,
		bundleID, dryRun, direction, sourceInstallID, string(vrJSON), createdBy,
	).Scan(
		&row.ID, &row.BundleID, &row.Status, &row.DryRun, &row.Direction, &row.SourceInstallID,
		&row.ValidationReport, &row.ApplyReport, &row.Error, &row.CreatedBy, &row.CreatedAt, &row.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert promotion: %w", err)
	}
	return &row, nil
}

func (e *DeployEngine) failPromotion(ctx context.Context, promotionID, errMsg string) (*PromotionRow, error) {
	var row PromotionRow
	now := time.Now()
	err := e.pool.QueryRow(ctx, `
UPDATE deploy_promotions SET status='failed', error=$2, completed_at=$3
WHERE id=$1::uuid
RETURNING id::text, bundle_id::text, status, dry_run, direction, source_install_id,
          validation_report, apply_report, error, created_by, created_at, completed_at`,
		promotionID, errMsg, now,
	).Scan(
		&row.ID, &row.BundleID, &row.Status, &row.DryRun, &row.Direction, &row.SourceInstallID,
		&row.ValidationReport, &row.ApplyReport, &row.Error, &row.CreatedBy, &row.CreatedAt, &row.CompletedAt,
	)
	return &row, err
}

func (e *DeployEngine) completePromotion(ctx context.Context, promotionID, status string, applyReport *ApplyReport) (*PromotionRow, error) {
	arJSON := []byte("null")
	if applyReport != nil {
		arJSON, _ = json.Marshal(applyReport)
	}
	var row PromotionRow
	now := time.Now()
	err := e.pool.QueryRow(ctx, `
UPDATE deploy_promotions SET status=$2, apply_report=$3, completed_at=$4
WHERE id=$1::uuid
RETURNING id::text, bundle_id::text, status, dry_run, direction, source_install_id,
          validation_report, apply_report, error, created_by, created_at, completed_at`,
		promotionID, status, string(arJSON), now,
	).Scan(
		&row.ID, &row.BundleID, &row.Status, &row.DryRun, &row.Direction, &row.SourceInstallID,
		&row.ValidationReport, &row.ApplyReport, &row.Error, &row.CreatedBy, &row.CreatedAt, &row.CompletedAt,
	)
	return &row, err
}

// PromoteBundle validates and optionally applies a stored bundle.
func (e *DeployEngine) PromoteBundle(ctx context.Context, input struct {
	BundleID  string
	DryRun    bool
	CreatedBy *string
}) (*PromoteBundleResult, error) {
	bundle, err := e.GetBundle(ctx, input.BundleID)
	if err != nil {
		return nil, err
	}
	artifact, err := ParseBundleArtifact(bundle.Artifact)
	if err != nil {
		return nil, err
	}
	validation, err := ValidateBundleArtifact(ctx, e.meta, artifact, e.productVersion, bundle.ProductVersionRange)
	if err != nil {
		return nil, err
	}

	promotion, err := e.insertPromotion(ctx, bundle.ID, input.DryRun, "local", &bundle.SourceInstallID, validation, input.CreatedBy)
	if err != nil {
		return nil, err
	}

	if !validation.OK {
		failed, _ := e.failPromotion(ctx, promotion.ID, "Validation failed")
		return &PromoteBundleResult{Promotion: failed, Validation: validation}, nil
	}

	// ADR-014 Phase 5: configured suites must be green before apply (skipped on dry-run).
	if !input.DryRun {
		if err := e.requireConfiguredSuitesGreen(ctx, input.CreatedBy); err != nil {
			failed, _ := e.failPromotion(ctx, promotion.ID, err.Error())
			return &PromoteBundleResult{Promotion: failed, Validation: validation}, nil
		}
	}

	applyReport, applyErr := ApplyBundleArtifact(ctx, e.pool, e.meta, artifact, input.DryRun)
	if applyErr != nil {
		failed, _ := e.failPromotion(ctx, promotion.ID, applyErr.Error())
		return nil, fmt.Errorf("%w; promotion failed: %s", applyErr, failed.ID)
	}
	status := "applied"
	if input.DryRun {
		status = "validated"
	}
	done, err := e.completePromotion(ctx, promotion.ID, status, applyReport)
	if err != nil {
		return nil, err
	}
	return &PromoteBundleResult{Promotion: done, Validation: validation, Apply: applyReport}, nil
}

// GetPromotion retrieves a promotion by ID.
func (e *DeployEngine) GetPromotion(ctx context.Context, id string) (*PromotionRow, error) {
	var row PromotionRow
	err := e.pool.QueryRow(ctx, `
SELECT id::text, bundle_id::text, status, dry_run, direction, source_install_id,
       validation_report, apply_report, error, created_by, created_at, completed_at
FROM deploy_promotions WHERE id=$1::uuid`, id).Scan(
		&row.ID, &row.BundleID, &row.Status, &row.DryRun, &row.Direction, &row.SourceInstallID,
		&row.ValidationReport, &row.ApplyReport, &row.Error, &row.CreatedBy, &row.CreatedAt, &row.CompletedAt,
	)
	if err != nil {
		return nil, newNotFoundError(fmt.Sprintf("Promotion not found: %s", id))
	}
	return &row, nil
}

// ---- Peers ----

// PeerRow is a row from deploy_peer_installs.
type PeerRow struct {
	ID          string    `json:"id"`
	InstallID   string    `json:"installId"`
	CustomerID  string    `json:"customerId"`
	Label       *string   `json:"label,omitempty"`
	InstallRole *string   `json:"installRole,omitempty"`
	BaseURL     *string   `json:"baseUrl,omitempty"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// UpsertPeer registers or updates a peer install.
func (e *DeployEngine) UpsertPeer(ctx context.Context, input struct {
	InstallID   string
	Label       *string
	InstallRole *string
	BaseURL     *string
	Active      *bool
	CustomerID  *string
}) (*PeerRow, error) {
	customerID := e.customerID
	if input.CustomerID != nil {
		customerID = *input.CustomerID
	}
	if customerID != e.customerID {
		return nil, newForbiddenError("Cannot register peers for a different CUSTOMER_ID")
	}
	active := true
	if input.Active != nil {
		active = *input.Active
	}
	var baseURL *string
	if input.BaseURL != nil && *input.BaseURL != "" {
		normalized, err := normalizePeerBaseURL(*input.BaseURL)
		if err != nil {
			return nil, newValidationErrorf("Invalid baseUrl: %v", err)
		}
		if err := assertSafePeerBaseURL(normalized); err != nil {
			return nil, newForbiddenError("Invalid baseUrl: " + err.Error())
		}
		baseURL = &normalized
	} else {
		baseURL = input.BaseURL
	}

	var row PeerRow
	err := e.pool.QueryRow(ctx, `
INSERT INTO deploy_peer_installs (install_id, customer_id, label, install_role, base_url, active)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (customer_id, install_id) DO UPDATE
  SET label=EXCLUDED.label, install_role=EXCLUDED.install_role,
      base_url=EXCLUDED.base_url, active=EXCLUDED.active, updated_at=now()
RETURNING id::text, install_id, customer_id, label, install_role, base_url, active, created_at, updated_at`,
		input.InstallID, customerID, input.Label, input.InstallRole, baseURL, active,
	).Scan(
		&row.ID, &row.InstallID, &row.CustomerID, &row.Label, &row.InstallRole,
		&row.BaseURL, &row.Active, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert peer: %w", err)
	}
	return &row, nil
}

// ListPeers returns all registered peers for this customer.
func (e *DeployEngine) ListPeers(ctx context.Context) ([]PeerRow, error) {
	rows, err := e.pool.Query(ctx, `
SELECT id::text, install_id, customer_id, label, install_role, base_url, active, created_at, updated_at
FROM deploy_peer_installs WHERE customer_id=$1 ORDER BY install_id`, e.customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PeerRow
	for rows.Next() {
		var r PeerRow
		if err := rows.Scan(
			&r.ID, &r.InstallID, &r.CustomerID, &r.Label, &r.InstallRole,
			&r.BaseURL, &r.Active, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---- Test suites ----

// TestSuiteRow is a row from customer_tests.
type TestSuiteRow struct {
	ID          string          `json:"id"`
	APIName     string          `json:"apiName"`
	Label       string          `json:"label"`
	Description *string         `json:"description,omitempty"`
	Active      bool            `json:"active"`
	Steps       json.RawMessage `json:"steps"`
	PackageName *string         `json:"packageName,omitempty"`
	Ownership   string          `json:"ownership"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// TestSuiteInput is the input for UpsertTestSuite.
type TestSuiteInput struct {
	APIName     string  `json:"apiName"`
	Label       string  `json:"label"`
	Description *string `json:"description,omitempty"`
	Active      *bool   `json:"active,omitempty"`
	Steps       []any   `json:"steps"`
	PackageName *string `json:"packageName,omitempty"`
}

// UpsertTestSuite creates or updates a customer test suite.
func (e *DeployEngine) UpsertTestSuite(ctx context.Context, input *TestSuiteInput) (*TestSuiteRow, error) {
	if input.APIName == "" {
		return nil, newValidationError("apiName is required")
	}
	if input.Label == "" {
		return nil, newValidationError("label is required")
	}
	if isManagedPackageName(input.PackageName) {
		return nil, newForbiddenError("Cannot assign managed packageName to customer tests")
	}
	if len(input.Steps) == 0 {
		return nil, newValidationError("steps must not be empty")
	}

	// Refuse overwrite of product-managed suites (e.g. PlatformSmoke).
	var existingOwnership string
	errOwn := e.pool.QueryRow(ctx, `SELECT ownership FROM customer_tests WHERE api_name=$1`, input.APIName).Scan(&existingOwnership)
	if errOwn == nil && existingOwnership == "managed" {
		return nil, newForbiddenError("Cannot overwrite managed test suite " + input.APIName)
	}

	active := true
	if input.Active != nil {
		active = *input.Active
	}
	pkgName := input.PackageName
	if pkgName == nil {
		s := DefaultCustomerPackage
		pkgName = &s
	}
	stepsJSON, _ := json.Marshal(input.Steps)

	var row TestSuiteRow
	err := e.pool.QueryRow(ctx, `
INSERT INTO customer_tests (api_name, label, description, active, steps, package_name, ownership)
VALUES ($1,$2,$3,$4,$5,$6,'custom')
ON CONFLICT (api_name) DO UPDATE
  SET label=$2, description=$3, active=$4, steps=$5, package_name=$6, ownership='custom', updated_at=now()
  WHERE customer_tests.ownership = 'custom'
RETURNING id::text, api_name, label, description, active, steps, package_name, ownership, created_at, updated_at`,
		input.APIName, input.Label, input.Description, active, string(stepsJSON), pkgName,
	).Scan(
		&row.ID, &row.APIName, &row.Label, &row.Description, &row.Active,
		&row.Steps, &row.PackageName, &row.Ownership, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert test suite: %w", err)
	}
	return &row, nil
}

// ListTestSuites returns all test suites ordered by api_name.
func (e *DeployEngine) ListTestSuites(ctx context.Context) ([]TestSuiteRow, error) {
	rows, err := e.pool.Query(ctx, `
SELECT id::text, api_name, label, description, active, steps, package_name, ownership, created_at, updated_at
FROM customer_tests ORDER BY api_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TestSuiteRow
	for rows.Next() {
		var r TestSuiteRow
		if err := rows.Scan(
			&r.ID, &r.APIName, &r.Label, &r.Description, &r.Active,
			&r.Steps, &r.PackageName, &r.Ownership, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetTestSuite returns a test suite by api_name.
func (e *DeployEngine) GetTestSuite(ctx context.Context, apiName string) (*TestSuiteRow, error) {
	var row TestSuiteRow
	err := e.pool.QueryRow(ctx, `
SELECT id::text, api_name, label, description, active, steps, package_name, ownership, created_at, updated_at
FROM customer_tests WHERE api_name=$1`, apiName).Scan(
		&row.ID, &row.APIName, &row.Label, &row.Description, &row.Active,
		&row.Steps, &row.PackageName, &row.Ownership, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		return nil, newNotFoundError(fmt.Sprintf("Test suite not found: %s", apiName))
	}
	return &row, nil
}

// ---- Test runs ----

// TestRunRow is a row from customer_test_runs.
type TestRunRow struct {
	ID           string          `json:"id"`
	SuiteAPIName string          `json:"suiteApiName"`
	Status       string          `json:"status"`
	Trigger      string          `json:"trigger"`
	Results      json.RawMessage `json:"results,omitempty"`
	Summary      json.RawMessage `json:"summary,omitempty"`
	Error        *string         `json:"error,omitempty"`
	CreatedBy    *string         `json:"createdBy,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	StartedAt    *time.Time      `json:"startedAt,omitempty"`
	CompletedAt  *time.Time      `json:"completedAt,omitempty"`
}

// StartTestRunResult is the outcome of StartTestRun.
type StartTestRunResult struct {
	Run      *TestRunRow `json:"run"`
	Mode     string      `json:"mode"` // async | sync
	Accepted bool        `json:"accepted,omitempty"`
	Status   string      `json:"status,omitempty"`
	JobID    string      `json:"jobId,omitempty"`
	Poll     string      `json:"poll,omitempty"`
}

// StartTestRun creates a test run (queued or immediate).
func (e *DeployEngine) StartTestRun(ctx context.Context, input struct {
	SuiteAPIName string
	Actor        *authz.Actor
	Async        bool
	Trigger      string
}) (*StartTestRunResult, error) {
	suite, err := e.GetTestSuite(ctx, input.SuiteAPIName)
	if err != nil {
		return nil, err
	}
	suiteMap := map[string]any{"active": suite.Active, "apiName": suite.APIName}
	if err := RequireSuiteActive(suiteMap); err != nil {
		return nil, err
	}

	async := input.Async || e.ForceAsyncTestRun(suite.Steps)
	if async {
		if err := e.assertDeployQueue(ctx); err != nil {
			return nil, err
		}
	}

	trigger := input.Trigger
	if trigger == "" {
		trigger = "api"
	}
	createdBy := ""
	if input.Actor != nil {
		createdBy = input.Actor.ID
	}

	var run TestRunRow
	err = e.pool.QueryRow(ctx, `
INSERT INTO customer_test_runs (suite_api_name, status, trigger, created_by)
VALUES ($1,'queued',$2,$3::uuid)
RETURNING id::text, suite_api_name, status, trigger, results, summary, error, created_by, created_at, started_at, completed_at`,
		suite.APIName, trigger, uuidOrNull(createdBy),
	).Scan(
		&run.ID, &run.SuiteAPIName, &run.Status, &run.Trigger,
		&run.Results, &run.Summary, &run.Error, &run.CreatedBy,
		&run.CreatedAt, &run.StartedAt, &run.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert test run: %w", err)
	}

	if async {
		jobID, err := e.insertJob(ctx, JobTypeCustomerTestRun, map[string]any{
			"runId":        run.ID,
			"suiteApiName": suite.APIName,
		})
		if err != nil {
			return nil, fmt.Errorf("insert async job: %w", err)
		}
		return &StartTestRunResult{
			Run:      &run,
			Mode:     "async",
			Accepted: true,
			Status:   "queued",
			JobID:    jobID,
			Poll:     WorkPollPrefix + jobID,
		}, nil
	}

	completed, err := e.ExecuteTestRun(ctx, run.ID, input.Actor)
	if err != nil {
		return nil, err
	}
	return &StartTestRunResult{Run: completed, Mode: "sync"}, nil
}

// ExecuteTestRun runs all steps of an existing test run record.
func (e *DeployEngine) ExecuteTestRun(ctx context.Context, runID string, actor *authz.Actor) (*TestRunRow, error) {
	run, err := e.GetTestRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run.Status == "passed" || run.Status == "failed" {
		return run, nil
	}

	suite, err := e.GetTestSuite(ctx, run.SuiteAPIName)
	if err != nil {
		return nil, err
	}

	effectiveActor := actor
	if effectiveActor == nil {
		ownerID := "00000000-0000-4000-8000-000000000001"
		if run.CreatedBy != nil {
			ownerID = *run.CreatedBy
		}
		effectiveActor = &authz.Actor{
			ID:               ownerID,
			Scopes:           []authz.Scope{authz.ScopeClient, authz.ScopeMetadata, authz.ScopeDeploy},
			PermissionSetIDs: []string{},
			IsAdmin:          true,
		}
	}

	now := time.Now()
	_, err = e.pool.Exec(ctx,
		`UPDATE customer_test_runs SET status='running', started_at=$2 WHERE id=$1::uuid`,
		runID, now)
	if err != nil {
		return nil, err
	}

	// Decode steps.
	var rawSteps []any
	if len(suite.Steps) > 0 {
		_ = json.Unmarshal(suite.Steps, &rawSteps)
	}

	results, summary, _ := RunTestStepsWithPool(ctx, rawSteps, e.meta, e.data, effectiveActor, e.pool)
	status := "passed"
	if summary.Failed > 0 {
		status = "failed"
	}

	resultsJSON, _ := json.Marshal(map[string]any{"steps": results})
	summaryJSON, _ := json.Marshal(summary)
	completedAt := time.Now()
	var row TestRunRow
	err = e.pool.QueryRow(ctx, `
UPDATE customer_test_runs
SET status=$2, results=$3::jsonb, summary=$4::jsonb, completed_at=$5
WHERE id=$1::uuid
RETURNING id::text, suite_api_name, status, trigger, results, summary, error, created_by, created_at, started_at, completed_at`,
		runID, status, string(resultsJSON), string(summaryJSON), completedAt,
	).Scan(
		&row.ID, &row.SuiteAPIName, &row.Status, &row.Trigger,
		&row.Results, &row.Summary, &row.Error, &row.CreatedBy,
		&row.CreatedAt, &row.StartedAt, &row.CompletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// requireConfiguredSuitesGreen runs each RequiredTestSuites suite synchronously and
// fails closed if any suite is missing, inactive, or reports status != passed.
func (e *DeployEngine) requireConfiguredSuitesGreen(ctx context.Context, createdBy *string) error {
	suites := e.requiredTestSuites
	if len(suites) == 0 {
		return nil
	}
	var actor *authz.Actor
	if createdBy != nil && *createdBy != "" {
		actor = &authz.Actor{
			ID:      *createdBy,
			Scopes:  []authz.Scope{authz.ScopeClient, authz.ScopeDeploy},
			IsAdmin: true,
		}
	}
	for _, name := range suites {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		res, err := e.StartTestRun(ctx, struct {
			SuiteAPIName string
			Actor        *authz.Actor
			Async        bool
			Trigger      string
		}{SuiteAPIName: name, Actor: actor, Async: false, Trigger: "promote_gate"})
		if err != nil {
			return fmt.Errorf("promote gate: suite %q: %w", name, err)
		}
		if res.Run == nil || res.Run.Status != "passed" {
			status := "unknown"
			if res.Run != nil {
				status = res.Run.Status
			}
			return fmt.Errorf("promote gate: suite %q status=%s (required passed)", name, status)
		}
	}
	return nil
}

// GetTestRun returns a test run by ID.
func (e *DeployEngine) GetTestRun(ctx context.Context, id string) (*TestRunRow, error) {
	var row TestRunRow
	err := e.pool.QueryRow(ctx, `
SELECT id::text, suite_api_name, status, trigger, results, summary, error, created_by, created_at, started_at, completed_at
FROM customer_test_runs WHERE id=$1::uuid`, id).Scan(
		&row.ID, &row.SuiteAPIName, &row.Status, &row.Trigger,
		&row.Results, &row.Summary, &row.Error, &row.CreatedBy,
		&row.CreatedAt, &row.StartedAt, &row.CompletedAt,
	)
	if err != nil {
		return nil, newNotFoundError(fmt.Sprintf("Test run not found: %s", id))
	}
	return &row, nil
}

// ListTestRuns returns up to limit test runs, newest first.
func (e *DeployEngine) ListTestRuns(ctx context.Context, limit int) ([]TestRunRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := e.pool.Query(ctx, `
SELECT id::text, suite_api_name, status, trigger, results, summary, error, created_by, created_at, started_at, completed_at
FROM customer_test_runs ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TestRunRow
	for rows.Next() {
		var r TestRunRow
		if err := rows.Scan(
			&r.ID, &r.SuiteAPIName, &r.Status, &r.Trigger,
			&r.Results, &r.Summary, &r.Error, &r.CreatedBy,
			&r.CreatedAt, &r.StartedAt, &r.CompletedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---- Helpers ----

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func uuidOrNull(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
