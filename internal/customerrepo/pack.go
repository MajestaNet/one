// Package customerrepo packs and unpacks one/v1 trees to/from Deploy BundleArtifacts.
package customerrepo

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/MajestaNet/ide/internal/automation"
	"github.com/MajestaNet/ide/internal/deploy"
	"github.com/MajestaNet/ide/internal/packages"
	"gopkg.in/yaml.v3"
)

const (
	RepoFormat = "one/v1"

	// Archive uploads are compressed attacker-controlled input. Keep extraction
	// bounded independently of the HTTP body limit so a small zip/tar bomb cannot
	// exhaust the API host before Deploy admission can inspect the artifact.
	maxArchiveExpandedBytes int64 = 128 << 20
	maxArchiveFiles               = 10_000
)

// Manifest is one.yaml at the repo root.
type Manifest struct {
	CustomerID          string   `yaml:"customerId" json:"customerId"`
	PackageName         string   `yaml:"packageName" json:"packageName"`
	ProductVersionRange string   `yaml:"productVersionRange" json:"productVersionRange"`
	RepoFormat          string   `yaml:"repoFormat" json:"repoFormat"`
	DefaultOrg          string   `yaml:"defaultOrg,omitempty" json:"defaultOrg,omitempty"`
	RequiredTestSuites  []string `yaml:"requiredTestSuites,omitempty" json:"requiredTestSuites,omitempty"`
	APIVersion          string   `yaml:"apiVersion,omitempty" json:"apiVersion,omitempty"`
}

// PackOptions configure PackFromDir.
type PackOptions struct {
	// CustomerIDOverride stamps the artifact when set (else one.yaml customerId).
	CustomerIDOverride string
	SourceInstallID    string
	SourceInstallRole  string
	// IncludePaths, when non-empty, packs only files whose repo-relative path equals
	// or is under one of the listed prefixes (forward-slash). Allowed roots:
	// metadata/, src/, tests/.
	IncludePaths []string
	// ManifestName loads manifests/<name>.yaml path list and merges into IncludePaths.
	ManifestName string
}

// PackFromDir walks a one/v1 directory tree and returns a BundleArtifact.
func PackFromDir(root string, opts PackOptions) (*deploy.BundleArtifact, *Manifest, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, err
	}
	man, err := readManifest(filepath.Join(abs, "one.yaml"))
	if err != nil {
		return nil, nil, err
	}
	if man.RepoFormat != RepoFormat {
		return nil, nil, fmt.Errorf("unsupported repoFormat %q (expected %s)", man.RepoFormat, RepoFormat)
	}
	if man.PackageName == "" {
		man.PackageName = deploy.DefaultCustomerPackage
	}
	if packages.IsManagedPackageName(&man.PackageName) {
		return nil, nil, fmt.Errorf("packageName %q is managed and cannot be used for customer repos", man.PackageName)
	}

	include, err := resolveIncludePaths(abs, opts)
	if err != nil {
		return nil, nil, err
	}
	allow := pathAllowFn(include)

	art := &deploy.BundleArtifact{
		ManifestVersion:    1,
		Ownership:          "custom",
		DefaultPackageName: man.PackageName,
		Objects:            []deploy.SnapshotObject{},
		Fields:             []deploy.SnapshotField{},
		ValidationRules:    []deploy.SnapshotRule{},
		Automations:        []deploy.SnapshotAutomation{},
		AgentPlaybooks:     []deploy.SnapshotAgentPlaybook{},
		PermissionSets:     []deploy.SnapshotPermissionSet{},
		Webhooks:           []deploy.SnapshotWebhook{},
		Tests:              []deploy.SnapshotTestSuite{},
		Sources:            map[string]string{},
	}
	if man.ProductVersionRange != "" {
		r := man.ProductVersionRange
		art.ProductVersionRange = &r
	}
	tid := man.CustomerID
	if opts.CustomerIDOverride != "" {
		tid = opts.CustomerIDOverride
	}
	if tid != "" {
		art.CustomerID = &tid
	}
	if opts.SourceInstallID != "" {
		art.SourceInstallID = &opts.SourceInstallID
	}
	if opts.SourceInstallRole != "" {
		art.SourceInstallRole = &opts.SourceInstallRole
	}

	metaRoot := filepath.Join(abs, "metadata")
	if err := walkYAML(filepath.Join(metaRoot, "objects"), abs, allow, func(p string, data []byte) error {
		var o deploy.SnapshotObject
		if err := yaml.Unmarshal(data, &o); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if err := forceCustomOwnership(&o.Ownership, o.PackageName, p); err != nil {
			return err
		}
		if o.APIName == "" {
			o.APIName = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		}
		art.Objects = append(art.Objects, o)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	if err := walkYAML(filepath.Join(metaRoot, "fields"), abs, allow, func(p string, data []byte) error {
		var f deploy.SnapshotField
		if err := yaml.Unmarshal(data, &f); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if err := forceCustomOwnership(&f.Ownership, f.PackageName, p); err != nil {
			return err
		}
		rel, _ := filepath.Rel(filepath.Join(metaRoot, "fields"), p)
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if f.ObjectAPIName == "" && len(parts) >= 2 {
			f.ObjectAPIName = parts[0]
		}
		if f.APIName == "" {
			f.APIName = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		}
		art.Fields = append(art.Fields, f)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	if err := walkYAML(filepath.Join(metaRoot, "validation-rules"), abs, allow, func(p string, data []byte) error {
		var r deploy.SnapshotRule
		if err := yaml.Unmarshal(data, &r); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if err := forceCustomOwnership(&r.Ownership, r.PackageName, p); err != nil {
			return err
		}
		rel, _ := filepath.Rel(filepath.Join(metaRoot, "validation-rules"), p)
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if r.ObjectAPIName == "" && len(parts) >= 2 {
			r.ObjectAPIName = parts[0]
		}
		if r.APIName == "" {
			r.APIName = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		}
		art.ValidationRules = append(art.ValidationRules, r)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	if err := walkYAML(filepath.Join(metaRoot, "automations"), abs, allow, func(p string, data []byte) error {
		var a deploy.SnapshotAutomation
		if err := yaml.Unmarshal(data, &a); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if err := forceCustomOwnership(&a.Ownership, a.PackageName, p); err != nil {
			return err
		}
		if a.APIName == "" {
			a.APIName = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		}
		art.Automations = append(art.Automations, a)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	// Guest TypeScript sources (ADR-014): src/automations + tests/automations.
	if err := walkTS(filepath.Join(abs, "src", "automations"), "src/automations", abs, allow, art.Sources); err != nil {
		return nil, nil, err
	}
	if err := walkTS(filepath.Join(abs, "tests", "automations"), "tests/automations", abs, allow, art.Sources); err != nil {
		return nil, nil, err
	}
	for path, body := range art.Sources {
		if err := automation.ValidateSourceImports(path, body); err != nil {
			return nil, nil, err
		}
	}
	for i := range art.Automations {
		a := &art.Automations[i]
		if a.EntryFile != nil && *a.EntryFile != "" {
			if body, ok := art.Sources[*a.EntryFile]; ok {
				src := body
				a.Source = &src
			}
		}
		if err := automation.ValidateDefinition(
			a.APIName, a.Runtime, a.Execution, a.TriggerEvent,
			a.EntryFile, a.Source, a.RunAsPrincipalID, a.Actions,
		); err != nil {
			return nil, nil, err
		}
	}

	if err := walkYAML(filepath.Join(metaRoot, "permission-sets"), abs, allow, func(p string, data []byte) error {
		var ps deploy.SnapshotPermissionSet
		if err := yaml.Unmarshal(data, &ps); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if err := forceCustomOwnership(&ps.Ownership, ps.PackageName, p); err != nil {
			return err
		}
		if ps.APIName == "" {
			ps.APIName = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		}
		art.PermissionSets = append(art.PermissionSets, ps)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	if err := walkYAML(filepath.Join(metaRoot, "webhooks"), abs, allow, func(p string, data []byte) error {
		var wh deploy.SnapshotWebhook
		if err := yaml.Unmarshal(data, &wh); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if err := forceCustomOwnership(&wh.Ownership, wh.PackageName, p); err != nil {
			return err
		}
		if wh.APIName == "" {
			wh.APIName = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		}
		art.Webhooks = append(art.Webhooks, wh)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	if err := walkYAML(filepath.Join(metaRoot, "connectors"), abs, allow, func(p string, data []byte) error {
		var c deploy.SnapshotConnector
		if err := yaml.Unmarshal(data, &c); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if err := forceCustomOwnership(&c.Ownership, c.PackageName, p); err != nil {
			return err
		}
		if c.APIName == "" {
			c.APIName = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		}
		art.Connectors = append(art.Connectors, c)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	playbooksDir := filepath.Join(metaRoot, "agents", "playbooks")
	if err := walkYAML(playbooksDir, abs, allow, func(p string, data []byte) error {
		var pb deploy.SnapshotAgentPlaybook
		if err := yaml.Unmarshal(data, &pb); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if err := forceCustomOwnership(&pb.Ownership, pb.PackageName, p); err != nil {
			return err
		}
		if pb.APIName == "" {
			pb.APIName = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		}
		// Dual-read allowedToolSpecs / allowedCanvasSpecs (ADR-021).
		merged := pb.AllowedToolSpecs
		if len(merged) == 0 {
			merged = pb.AllowedCanvasSpecs
		}
		if merged == nil {
			merged = []string{}
		}
		pb.AllowedToolSpecs = merged
		pb.AllowedCanvasSpecs = merged
		art.AgentPlaybooks = append(art.AgentPlaybooks, pb)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	canvasesDir := filepath.Join(metaRoot, "canvases")
	toolsDir := filepath.Join(metaRoot, "tools")
	seenTools := map[string]struct{}{}
	appendTool := func(cs deploy.SnapshotCanvasSpec, p string) error {
		if err := forceCustomOwnership(&cs.Ownership, cs.PackageName, p); err != nil {
			return err
		}
		if cs.APIName == "" {
			cs.APIName = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		}
		if _, ok := seenTools[cs.APIName]; ok {
			return nil
		}
		seenTools[cs.APIName] = struct{}{}
		art.Canvases = append(art.Canvases, cs)
		return nil
	}
	// Prefer metadata/tools/ (ToolSpec); still accept deprecated metadata/canvases/.
	if err := walkYAML(toolsDir, abs, allow, func(p string, data []byte) error {
		cs, err := parseCanvasSpecYAML(data)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		return appendTool(cs, p)
	}); err != nil {
		return nil, nil, err
	}
	if err := walkYAML(canvasesDir, abs, allow, func(p string, data []byte) error {
		cs, err := parseCanvasSpecYAML(data)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		return appendTool(cs, p)
	}); err != nil {
		return nil, nil, err
	}

	experiencesDir := filepath.Join(metaRoot, "experiences")
	if err := walkYAML(experiencesDir, abs, allow, func(p string, data []byte) error {
		ex, err := parseExperienceYAML(data)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if err := forceCustomOwnership(&ex.Ownership, ex.PackageName, p); err != nil {
			return err
		}
		if ex.APIName == "" {
			ex.APIName = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		}
		art.Experiences = append(art.Experiences, ex)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	if err := walkYAML(filepath.Join(metaRoot, "data-roles"), abs, allow, func(p string, data []byte) error {
		var r deploy.SnapshotDataRole
		if err := yaml.Unmarshal(data, &r); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if r.APIName == "" {
			r.APIName = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		}
		art.DataRoles = append(art.DataRoles, r)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	if err := walkYAML(filepath.Join(metaRoot, "object-sharing"), abs, allow, func(p string, data []byte) error {
		var o deploy.SnapshotObjectSharing
		if err := yaml.Unmarshal(data, &o); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if o.ObjectAPIName == "" {
			o.ObjectAPIName = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		}
		art.ObjectSharingSettings = append(art.ObjectSharingSettings, o)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	if err := walkYAML(filepath.Join(metaRoot, "sharing-rules"), abs, allow, func(p string, data []byte) error {
		var raw struct {
			ObjectAPIName           string `yaml:"objectApiName"`
			APIName                 string `yaml:"apiName"`
			Label                   string `yaml:"label"`
			Active                  bool   `yaml:"active"`
			AccessLevel             string `yaml:"accessLevel"`
			SharedToDataRoleAPIName string `yaml:"sharedToDataRoleApiName"`
			Criteria                any    `yaml:"criteria"`
			SortOrder               int    `yaml:"sortOrder"`
		}
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		rel, _ := filepath.Rel(filepath.Join(metaRoot, "sharing-rules"), p)
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if raw.ObjectAPIName == "" && len(parts) >= 2 {
			raw.ObjectAPIName = parts[0]
		}
		if raw.APIName == "" {
			raw.APIName = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		}
		crit, err := json.Marshal(raw.Criteria)
		if err != nil {
			return fmt.Errorf("%s: criteria: %w", p, err)
		}
		if raw.Criteria == nil {
			crit = []byte("null")
		}
		art.SharingRules = append(art.SharingRules, deploy.SnapshotSharingRule{
			ObjectAPIName:           raw.ObjectAPIName,
			APIName:                 raw.APIName,
			Label:                   raw.Label,
			Active:                  raw.Active,
			AccessLevel:             raw.AccessLevel,
			SharedToDataRoleAPIName: raw.SharedToDataRoleAPIName,
			Criteria:                crit,
			SortOrder:               raw.SortOrder,
		})
		return nil
	}); err != nil {
		return nil, nil, err
	}

	// .one/baseline is read-only managed reference — never packed into promoteable artifacts.

	if err := walkYAML(filepath.Join(abs, "tests"), abs, allow, func(p string, data []byte) error {
		var t deploy.SnapshotTestSuite
		if err := yaml.Unmarshal(data, &t); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if err := forceCustomOwnership(&t.Ownership, t.PackageName, p); err != nil {
			return err
		}
		if t.APIName == "" {
			t.APIName = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		}
		art.Tests = append(art.Tests, t)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	return art, man, nil
}

// UnpackToDir writes a BundleArtifact as a one/v1 tree under root.
func UnpackToDir(root string, art *deploy.BundleArtifact, man Manifest) error {
	if man.RepoFormat == "" {
		man.RepoFormat = RepoFormat
	}
	if man.PackageName == "" {
		man.PackageName = art.DefaultPackageName
		if man.PackageName == "" {
			man.PackageName = deploy.DefaultCustomerPackage
		}
	}
	if art.CustomerID != nil && man.CustomerID == "" {
		man.CustomerID = *art.CustomerID
	}
	if art.ProductVersionRange != nil && man.ProductVersionRange == "" {
		man.ProductVersionRange = *art.ProductVersionRange
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	mb, err := yaml.Marshal(man)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "one.yaml"), mb, 0o644); err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Join(root, ".one"), 0o755)
	_ = os.WriteFile(filepath.Join(root, ".one", "ignore"), []byte("# secrets and records must never be committed\n"), 0o644)

	for _, o := range art.Objects {
		p := filepath.Join(root, "metadata", "objects", o.APIName+".yaml")
		if err := writeYAML(p, o); err != nil {
			return err
		}
	}
	for _, f := range art.Fields {
		p := filepath.Join(root, "metadata", "fields", f.ObjectAPIName, f.APIName+".yaml")
		if err := writeYAML(p, f); err != nil {
			return err
		}
	}
	for _, r := range art.ValidationRules {
		p := filepath.Join(root, "metadata", "validation-rules", r.ObjectAPIName, r.APIName+".yaml")
		if err := writeYAML(p, r); err != nil {
			return err
		}
	}
	for _, a := range art.Automations {
		// Do not write embedded source into YAML (lives under src/automations via Sources).
		clone := a
		clone.Source = nil
		p := filepath.Join(root, "metadata", "automations", a.APIName+".yaml")
		if err := writeYAML(p, clone); err != nil {
			return err
		}
	}
	for rel, body := range art.Sources {
		rel = path.Clean("/" + rel)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" || strings.Contains(rel, "..") {
			continue
		}
		if !strings.HasPrefix(rel, "src/automations/") && !strings.HasPrefix(rel, "tests/automations/") {
			continue
		}
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			return err
		}
	}
	for _, ps := range art.PermissionSets {
		p := filepath.Join(root, "metadata", "permission-sets", ps.APIName+".yaml")
		if err := writeYAML(p, ps); err != nil {
			return err
		}
	}
	for _, wh := range art.Webhooks {
		p := filepath.Join(root, "metadata", "webhooks", wh.APIName+".yaml")
		if err := writeYAML(p, wh); err != nil {
			return err
		}
	}
	for _, c := range art.Connectors {
		p := filepath.Join(root, "metadata", "connectors", c.APIName+".yaml")
		if err := writeYAML(p, c); err != nil {
			return err
		}
	}
	for _, pb := range art.AgentPlaybooks {
		p := filepath.Join(root, "metadata", "agents", "playbooks", pb.APIName+".yaml")
		if err := writeYAML(p, pb); err != nil {
			return err
		}
	}
	for _, cs := range art.Canvases {
		var layout, nodes, bindings any
		if len(cs.Layout) > 0 {
			_ = json.Unmarshal(cs.Layout, &layout)
		}
		if len(cs.Nodes) > 0 {
			_ = json.Unmarshal(cs.Nodes, &nodes)
		}
		if len(cs.DataBindings) > 0 {
			_ = json.Unmarshal(cs.DataBindings, &bindings)
		}
		out := map[string]any{
			"apiVersion":   "one.tool/v1",
			"kind":         "ToolSpec",
			"apiName":      cs.APIName,
			"label":        cs.Label,
			"description":  cs.Description,
			"icon":         cs.Icon,
			"sortOrder":    cs.SortOrder,
			"layout":       layout,
			"nodes":        nodes,
			"dataBindings": bindings,
			"active":       cs.Active,
			"ownership":    cs.Ownership,
		}
		if cs.PackageName != nil {
			out["packageName"] = *cs.PackageName
		}
		p := filepath.Join(root, "metadata", "tools", cs.APIName+".yaml")
		if err := writeYAML(p, out); err != nil {
			return err
		}
	}
	for _, ex := range art.Experiences {
		out := map[string]any{
			"apiName":             ex.APIName,
			"label":               ex.Label,
			"description":         ex.Description,
			"homeUrl":             ex.HomeURL,
			"connectedAppApiName": ex.ConnectedAppAPIName,
			"allowedOrigins":      ex.AllowedOrigins,
			"active":              ex.Active,
			"ownership":           ex.Ownership,
		}
		if ex.PackageName != nil {
			out["packageName"] = *ex.PackageName
		}
		p := filepath.Join(root, "metadata", "experiences", ex.APIName+".yaml")
		if err := writeYAML(p, out); err != nil {
			return err
		}
	}
	for _, r := range art.DataRoles {
		p := filepath.Join(root, "metadata", "data-roles", r.APIName+".yaml")
		if err := writeYAML(p, r); err != nil {
			return err
		}
	}
	for _, o := range art.ObjectSharingSettings {
		name := o.ObjectAPIName
		if name == "" {
			continue
		}
		p := filepath.Join(root, "metadata", "object-sharing", name+".yaml")
		if err := writeYAML(p, o); err != nil {
			return err
		}
	}
	for _, r := range art.SharingRules {
		var crit any
		if len(r.Criteria) > 0 {
			_ = json.Unmarshal(r.Criteria, &crit)
		}
		out := map[string]any{
			"objectApiName":           r.ObjectAPIName,
			"apiName":                 r.APIName,
			"label":                   r.Label,
			"active":                  r.Active,
			"accessLevel":             r.AccessLevel,
			"sharedToDataRoleApiName": r.SharedToDataRoleAPIName,
			"criteria":                crit,
			"sortOrder":               r.SortOrder,
		}
		p := filepath.Join(root, "metadata", "sharing-rules", r.ObjectAPIName, r.APIName+".yaml")
		if err := writeYAML(p, out); err != nil {
			return err
		}
	}
	for _, t := range art.Tests {
		p := filepath.Join(root, "tests", t.APIName+".yaml")
		if err := writeYAML(p, t); err != nil {
			return err
		}
	}
	if art.Baseline != nil {
		if err := writeBaseline(root, art.Baseline); err != nil {
			return err
		}
	}
	return nil
}

func writeBaseline(root string, bl *deploy.ManagedBaseline) error {
	base := filepath.Join(root, ".one", "baseline")
	if err := os.MkdirAll(base, 0o755); err != nil {
		return err
	}
	man := map[string]any{
		"productVersion": bl.ProductVersion,
		"generatedAt":    bl.GeneratedAt,
	}
	if bl.SourceInstallID != "" {
		man["sourceInstallId"] = bl.SourceInstallID
	}
	if err := writeYAML(filepath.Join(base, "manifest.yaml"), man); err != nil {
		return err
	}
	for _, o := range bl.Objects {
		p := filepath.Join(base, "objects", o.APIName+".yaml")
		if err := writeYAML(p, o); err != nil {
			return err
		}
	}
	for _, f := range bl.Fields {
		p := filepath.Join(base, "fields", f.ObjectAPIName, f.APIName+".yaml")
		if err := writeYAML(p, f); err != nil {
			return err
		}
	}
	return nil
}

// PackArchive detects zip or gzip-tar and packs into a BundleArtifact.
func PackArchive(r io.ReaderAt, size int64, contentType string, opts PackOptions) (*deploy.BundleArtifact, *Manifest, error) {
	tmpdir, err := os.MkdirTemp("", "one-pack-*")
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = os.RemoveAll(tmpdir) }()

	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "zip") || looksLikeZip(r):
		if err := extractZip(r, size, tmpdir); err != nil {
			return nil, nil, err
		}
	default:
		if err := extractTarGz(r, size, tmpdir); err != nil {
			return nil, nil, err
		}
	}
	root, err := findRepoRoot(tmpdir)
	if err != nil {
		return nil, nil, err
	}
	return PackFromDir(root, opts)
}

// ExtractZipToDir unpacks a one export zip into dest (overwrites files).
func ExtractZipToDir(r io.ReaderAt, size int64, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	return extractZip(r, size, dest)
}

// WriteZipArchive unpacks artifact to a zip writer.
func WriteZipArchive(w io.Writer, art *deploy.BundleArtifact, man Manifest) error {
	tmpdir, err := os.MkdirTemp("", "one-export-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpdir) }()
	if err := UnpackToDir(tmpdir, art, man); err != nil {
		return err
	}
	zw := zip.NewWriter(w)
	walkErr := filepath.WalkDir(tmpdir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(tmpdir, p)
		if err != nil {
			return err
		}
		fw, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		_, err = fw.Write(b)
		return err
	})
	if closeErr := zw.Close(); closeErr != nil && walkErr == nil {
		return closeErr
	}
	return walkErr
}

func forceCustomOwnership(ownership *string, pkgName *string, pathHint string) error {
	if packages.IsManagedPackageName(pkgName) {
		return fmt.Errorf("managed packageName in customer file %s", pathHint)
	}
	*ownership = "custom"
	return nil
}

func readManifest(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read one.yaml: %w", err)
	}
	var m Manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse one.yaml: %w", err)
	}
	return &m, nil
}

func writeYAML(p string, v any) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}

func walkYAML(dir, repoRoot string, allow func(rel string) bool, fn func(path string, data []byte) error) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		if allow != nil {
			rel, rerr := filepath.Rel(repoRoot, p)
			if rerr != nil {
				return rerr
			}
			if !allow(filepath.ToSlash(rel)) {
				return nil
			}
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return fn(p, data)
	})
}

// walkTS loads .ts files under dir into sources keyed by repo-relative posix paths
// (prefix + relative path), e.g. src/automations/foo.ts.
func walkTS(dir, prefix, repoRoot string, allow func(rel string) bool, sources map[string]string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(p)) != ".ts" {
			return nil
		}
		if allow != nil {
			repoRel, rerr := filepath.Rel(repoRoot, p)
			if rerr != nil {
				return rerr
			}
			if !allow(filepath.ToSlash(repoRel)) {
				return nil
			}
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		key := path.Join(prefix, filepath.ToSlash(rel))
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sources[key] = string(data)
		return nil
	})
}

func looksLikeZip(r io.ReaderAt) bool {
	var hdr [4]byte
	if _, err := r.ReadAt(hdr[:], 0); err != nil {
		return false
	}
	return hdr[0] == 'P' && hdr[1] == 'K'
}

func extractZip(r io.ReaderAt, size int64, dest string) error {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return fmt.Errorf("zip: %w", err)
	}
	// UncompressedSize64 is attacker-controlled; treat it as fail-fast for
	// honest archives only. The LimitReader below is the real expanded-size cap.
	var declaredBytes uint64
	fileCount := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		fileCount++
		if fileCount > maxArchiveFiles {
			return fmt.Errorf("archive exceeds %d files", maxArchiveFiles)
		}
		if f.UncompressedSize64 > uint64(maxArchiveExpandedBytes)-declaredBytes {
			return fmt.Errorf("archive expands beyond %d bytes", maxArchiveExpandedBytes)
		}
		declaredBytes += f.UncompressedSize64
	}

	var expandedBytes int64
	for _, f := range zr.File {
		name := path.Clean("/" + f.Name)
		name = strings.TrimPrefix(name, "/")
		if name == "" || strings.Contains(name, "..") {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		if !strings.HasPrefix(target, dest+string(os.PathSeparator)) && target != dest {
			return fmt.Errorf("zip path escape: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0o755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			_ = rc.Close()
			return err
		}
		remaining := maxArchiveExpandedBytes - expandedBytes
		written, copyErr := io.Copy(out, io.LimitReader(rc, remaining+1))
		expandedBytes += written
		closeOut := out.Close()
		closeRC := rc.Close()
		if copyErr != nil {
			return copyErr
		}
		if expandedBytes > maxArchiveExpandedBytes {
			return fmt.Errorf("archive expands beyond %d bytes", maxArchiveExpandedBytes)
		}
		if closeOut != nil {
			return closeOut
		}
		if closeRC != nil {
			return closeRC
		}
	}
	return nil
}

func extractTarGz(r io.ReaderAt, size int64, dest string) error {
	section := io.NewSectionReader(r, 0, size)
	var reader io.Reader = section
	gz, err := gzip.NewReader(section)
	if err == nil {
		// Cap decompressed tar bytes (headers + padding + file data) so a gzip
		// junk stream between members cannot exhaust the host.
		reader = io.LimitReader(gz, maxArchiveExpandedBytes+1)
		defer func() { _ = gz.Close() }()
	} else {
		_, _ = section.Seek(0, io.SeekStart)
	}
	tr := tar.NewReader(reader)
	var expandedBytes int64
	fileCount := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		name := path.Clean("/" + hdr.Name)
		name = strings.TrimPrefix(name, "/")
		if name == "" || strings.Contains(name, "..") {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		if !strings.HasPrefix(target, dest+string(os.PathSeparator)) && target != dest {
			return fmt.Errorf("tar path escape: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			fileCount++
			if fileCount > maxArchiveFiles {
				return fmt.Errorf("archive exceeds %d files", maxArchiveFiles)
			}
			if hdr.Size < 0 || hdr.Size > maxArchiveExpandedBytes-expandedBytes {
				return fmt.Errorf("archive expands beyond %d bytes", maxArchiveExpandedBytes)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			remaining := maxArchiveExpandedBytes - expandedBytes
			written, copyErr := io.Copy(out, io.LimitReader(tr, remaining+1))
			expandedBytes += written
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if expandedBytes > maxArchiveExpandedBytes {
				return fmt.Errorf("archive expands beyond %d bytes", maxArchiveExpandedBytes)
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	return nil
}

func findRepoRoot(dir string) (string, error) {
	var found string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if d.Name() == "one.yaml" {
			found = filepath.Dir(p)
			return fs.SkipAll
		}
		return nil
	})
	if found == "" {
		return "", fmt.Errorf("archive missing one.yaml")
	}
	return found, nil
}

func parseCanvasSpecYAML(data []byte) (deploy.SnapshotCanvasSpec, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return deploy.SnapshotCanvasSpec{}, err
	}
	// Nested ToolSpec document unwrap (ADR-021).
	if nested, ok := doc["document"].(map[string]any); ok {
		for _, key := range []string{"layout", "nodes", "dataBindings"} {
			if _, have := doc[key]; !have {
				if v, ok := nested[key]; ok {
					doc[key] = v
				}
			}
		}
	}
	toRaw := func(key string) (json.RawMessage, error) {
		v, ok := doc[key]
		if !ok || v == nil {
			return nil, nil
		}
		b, err := json.Marshal(v)
		return json.RawMessage(b), err
	}
	layout, err := toRaw("layout")
	if err != nil {
		return deploy.SnapshotCanvasSpec{}, fmt.Errorf("layout: %w", err)
	}
	nodes, err := toRaw("nodes")
	if err != nil {
		return deploy.SnapshotCanvasSpec{}, fmt.Errorf("nodes: %w", err)
	}
	bindings, err := toRaw("dataBindings")
	if err != nil {
		return deploy.SnapshotCanvasSpec{}, fmt.Errorf("dataBindings: %w", err)
	}
	cs := deploy.SnapshotCanvasSpec{
		Layout:       layout,
		Nodes:        nodes,
		DataBindings: bindings,
	}
	if v, ok := doc["apiName"].(string); ok {
		cs.APIName = v
	}
	if v, ok := doc["label"].(string); ok {
		cs.Label = v
	}
	if v, ok := doc["description"].(string); ok {
		cs.Description = v
	}
	if v, ok := doc["icon"].(string); ok {
		cs.Icon = v
	}
	switch v := doc["sortOrder"].(type) {
	case int:
		cs.SortOrder = v
	case int64:
		cs.SortOrder = int(v)
	case float64:
		cs.SortOrder = int(v)
	}
	if v, ok := doc["ownership"].(string); ok {
		cs.Ownership = v
	}
	if v, ok := doc["active"].(bool); ok {
		cs.Active = v
	} else {
		cs.Active = true
	}
	if v, ok := doc["packageName"].(string); ok && v != "" {
		cs.PackageName = &v
	}
	if v, ok := doc["packageApiName"].(string); ok && v != "" && cs.PackageName == nil {
		cs.PackageName = &v
	}
	return cs, nil
}

func parseExperienceYAML(data []byte) (deploy.SnapshotExperience, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return deploy.SnapshotExperience{}, err
	}
	ex := deploy.SnapshotExperience{AllowedOrigins: []string{}}
	if v, ok := doc["apiName"].(string); ok {
		ex.APIName = v
	}
	if v, ok := doc["label"].(string); ok {
		ex.Label = v
	}
	if v, ok := doc["description"].(string); ok {
		ex.Description = v
	}
	if v, ok := doc["homeUrl"].(string); ok {
		ex.HomeURL = v
	}
	if v, ok := doc["connectedAppApiName"].(string); ok {
		ex.ConnectedAppAPIName = v
	}
	if v, ok := doc["ownership"].(string); ok {
		ex.Ownership = v
	}
	if v, ok := doc["active"].(bool); ok {
		ex.Active = v
	} else {
		ex.Active = true
	}
	if v, ok := doc["packageName"].(string); ok && v != "" {
		ex.PackageName = &v
	}
	if raw, ok := doc["allowedOrigins"].([]any); ok {
		for _, item := range raw {
			if s, ok := item.(string); ok && s != "" {
				ex.AllowedOrigins = append(ex.AllowedOrigins, s)
			}
		}
	}
	return ex, nil
}

// BytesReaderAt adapts a byte slice for PackArchive.
type BytesReaderAt struct {
	B []byte
}

func (b BytesReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 || off >= int64(len(b.B)) {
		return 0, io.EOF
	}
	n := copy(p, b.B[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// ReadAllLimited reads up to max bytes from r.
func ReadAllLimited(r io.Reader, max int64) ([]byte, error) {
	var buf bytes.Buffer
	_, err := io.CopyN(&buf, r, max+1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if int64(buf.Len()) > max {
		return nil, fmt.Errorf("archive exceeds %d bytes", max)
	}
	return buf.Bytes(), nil
}
