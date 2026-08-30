package deploy

import (
	"context"
	"fmt"
)

// ValidateLocalResult is the response of ValidateLocal / packages/validate-local.
type ValidateLocalResult struct {
	BundleID   string            `json:"bundleId"`
	Checksum   string            `json:"checksum"`
	Diff       *DiffReport       `json:"diff"`
	Validation *ValidationReport `json:"validation"`
	OK         bool              `json:"ok"`
}

// ValidateLocal packs/stores a local artifact (or reuses bundleId), diffs vs install snapshot,
// and runs ValidateBundleArtifact. Does not mutate customer metadata.
func (e *DeployEngine) ValidateLocal(ctx context.Context, input struct {
	Artifact  any
	BundleID  string
	Label     *string
	CreatedBy *string
}) (*ValidateLocalResult, error) {
	var (
		bundleID string
		artifact *BundleArtifact
		checksum string
		rangeStr string
	)

	if input.BundleID != "" {
		bundle, err := e.GetBundle(ctx, input.BundleID)
		if err != nil {
			return nil, err
		}
		artifact, err = ParseBundleArtifact(bundle.Artifact)
		if err != nil {
			return nil, err
		}
		bundleID = bundle.ID
		checksum = bundle.Checksum
		rangeStr = bundle.ProductVersionRange
	} else {
		if input.Artifact == nil {
			return nil, fmt.Errorf("artifact or bundleId is required")
		}
		row, err := e.CreateBundleFromArtifact(ctx, struct {
			Artifact  any
			Label     *string
			CreatedBy *string
			Origin    string
			Signature *string
		}{
			Artifact:  input.Artifact,
			Label:     input.Label,
			CreatedBy: input.CreatedBy,
			Origin:    "customer-package",
		})
		if err != nil {
			return nil, err
		}
		artifact, err = ParseBundleArtifact(row.Artifact)
		if err != nil {
			return nil, err
		}
		bundleID = row.ID
		checksum = row.Checksum
		rangeStr = row.ProductVersionRange
	}

	install, _, err := e.BuildSnapshotArtifact(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("install snapshot: %w", err)
	}
	// Compare customer payload only — strip baseline from local pack side for add/change/remove;
	// baseline drift is reported separately via CompareArtifacts.
	diff := CompareArtifacts(artifact, install)

	validation, err := ValidateBundleArtifact(ctx, e.meta, artifact, e.productVersion, rangeStr)
	if err != nil {
		return nil, err
	}
	ok := validation != nil && validation.OK
	return &ValidateLocalResult{
		BundleID:   bundleID,
		Checksum:   checksum,
		Diff:       diff,
		Validation: validation,
		OK:         ok,
	}, nil
}
