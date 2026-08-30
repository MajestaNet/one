package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/MajestaNet/ide/internal/deploy/gitremote"
)

var (
	// ErrRepoAlreadyInitialized is returned when remote main already has a baseline.
	ErrRepoAlreadyInitialized = gitremote.ErrAlreadyInitialized
	// ErrCustomerRepoNotConfigured is returned when CUSTOMER_REPO_URL is empty.
	ErrCustomerRepoNotConfigured = gitremote.ErrNotConfigured
)

// InitializeRepoResult is the response of InitializeRepo / package initialize-repo.
type InitializeRepoResult struct {
	CustomerRepoURL        string         `json:"customerRepoUrl"`
	Provider               string         `json:"provider,omitempty"`
	CommitSHA              string         `json:"commitSha"`
	ArtifactCounts         map[string]int `json:"artifactCounts"`
	BaselineProductVersion string         `json:"baselineProductVersion"`
	Forced                 bool           `json:"forced"`
}

// SetGitRemote injects a Git remote seeder (tests / wiring).
func (e *DeployEngine) SetGitRemote(r gitremote.Remote) {
	e.gitRemote = r
}

// ExportRepoArtifact builds a BundleArtifact (customer snapshot + managed baseline)
// suitable for unpacking into one/v1 and seeding a remote.
func (e *DeployEngine) ExportRepoArtifact(ctx context.Context) (*BundleArtifact, string /*productVersionRange*/, error) {
	bundle, err := e.CreateBundleFromSnapshot(ctx, struct {
		Label               *string
		CreatedBy           *string
		ProductVersionRange string
	}{})
	if err != nil {
		return nil, "", fmt.Errorf("snapshot: %w", err)
	}
	var art BundleArtifact
	if err := json.Unmarshal(bundle.Artifact, &art); err != nil {
		return nil, "", fmt.Errorf("parse artifact: %w", err)
	}
	return &art, bundle.ProductVersionRange, nil
}

// SeedCustomerRepoDir pushes an already-unpacked one/v1 directory to remote main.
func (e *DeployEngine) SeedCustomerRepoDir(ctx context.Context, localDir string, force bool) (commitSHA string, err error) {
	if e.customerRepoURL == "" || e.gitRemote == nil {
		return "", ErrCustomerRepoNotConfigured
	}
	sha, err := e.gitRemote.SeedMain(ctx, localDir, force)
	if err != nil {
		if errors.Is(err, gitremote.ErrAlreadyInitialized) {
			return "", ErrRepoAlreadyInitialized
		}
		if errors.Is(err, gitremote.ErrNotConfigured) {
			return "", ErrCustomerRepoNotConfigured
		}
		return "", err
	}
	return sha, nil
}

// BuildInitializeRepoResult assembles the API response after a successful seed.
func (e *DeployEngine) BuildInitializeRepoResult(art *BundleArtifact, commitSHA string, force bool) *InitializeRepoResult {
	baselineVer := e.productVersion
	if art != nil && art.Baseline != nil && art.Baseline.ProductVersion != "" {
		baselineVer = art.Baseline.ProductVersion
	}
	counts := map[string]int{}
	if art != nil {
		counts = map[string]int{
			"objects":         len(art.Objects),
			"fields":          len(art.Fields),
			"validationRules": len(art.ValidationRules),
			"automations":     len(art.Automations),
			"permissionSets":  len(art.PermissionSets),
			"webhooks":        len(art.Webhooks),
			"connectors":      len(art.Connectors),
			"tests":           len(art.Tests),
			"dataRoles":       len(art.DataRoles),
			"sharingRules":    len(art.SharingRules),
			"baselineObjects": baselineObjectCount(art.Baseline),
			"baselineFields":  baselineFieldCount(art.Baseline),
		}
	}
	return &InitializeRepoResult{
		CustomerRepoURL:        e.customerRepoURL,
		Provider:               e.customerRepoProvider,
		CommitSHA:              commitSHA,
		ArtifactCounts:         counts,
		BaselineProductVersion: baselineVer,
		Forced:                 force,
	}
}

func baselineObjectCount(b *ManagedBaseline) int {
	if b == nil {
		return 0
	}
	return len(b.Objects)
}

func baselineFieldCount(b *ManagedBaseline) int {
	if b == nil {
		return 0
	}
	return len(b.Fields)
}
