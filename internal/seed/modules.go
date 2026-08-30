package seed

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/packages"
)

// PackageStatus is the catalog + install view for one managed package.
type PackageStatus struct {
	Name              string   `json:"name"`
	Label             string   `json:"label"`
	Description       string   `json:"description"`
	Version           string   `json:"version"`
	InstalledVersion  string   `json:"installedVersion,omitempty"`
	DependsOn         []string `json:"dependsOn,omitempty"`
	Optional          bool     `json:"optional"`
	AutoEnable        bool     `json:"autoEnable,omitempty"`
	Enabled           bool     `json:"enabled"`
	DocumentationPath string   `json:"documentationPath,omitempty"`
	ObjectAPINames    []string `json:"objectApiNames"`
	// Objects is the image-registry shape (lookups included) even when the
	// package is not enabled on this install — Explorer visualizes from this.
	Objects        []packages.CatalogObject `json:"objects,omitempty"`
	ActionAPINames []string                 `json:"actionApiNames,omitempty"`
}

// ListPackageStatuses returns image registry modules with install state.
func ListPackageStatuses(ctx context.Context, meta *metadata.Service) ([]PackageStatus, error) {
	out := make([]PackageStatus, 0)
	for _, m := range packages.List() {
		st, err := packageStatus(ctx, meta, m)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}

// GetPackageStatus returns one package status.
func GetPackageStatus(ctx context.Context, meta *metadata.Service, name string) (PackageStatus, error) {
	m, ok := packages.Get(name)
	if !ok {
		return PackageStatus{}, fmt.Errorf("unknown package: %s", name)
	}
	return packageStatus(ctx, meta, m)
}

func packageStatus(ctx context.Context, meta *metadata.Service, m packages.Module) (PackageStatus, error) {
	ver, enabled, err := meta.GetPackageInstall(ctx, m.Name)
	if err != nil {
		return PackageStatus{}, err
	}
	objs := make([]string, 0, len(m.Objects))
	for _, o := range m.Objects {
		objs = append(objs, o.APIName)
	}
	actions := make([]string, 0, len(m.Actions))
	for _, a := range m.Actions {
		if a.APIName != "" {
			actions = append(actions, a.APIName)
		}
	}
	return PackageStatus{
		Name:              m.Name,
		Label:             m.Label,
		Description:       m.Description,
		Version:           m.Version,
		InstalledVersion:  ver,
		DependsOn:         m.DependsOn,
		Optional:          m.Optional,
		AutoEnable:        m.AutoEnable,
		Enabled:           enabled && ver != "",
		DocumentationPath: m.DocumentationPath,
		ObjectAPINames:    objs,
		Objects:           packages.CatalogObjects(m),
		ActionAPINames:    actions,
	}, nil
}

// EnablePackage installs/migrates an optional managed module on this install.
// After a successful enable, any AutoEnable modules whose DependsOn are all
// installed are enabled as well (e.g. crm_bridge when sales and service are on).
func EnablePackage(ctx context.Context, meta *metadata.Service, name string) (PackageStatus, error) {
	m, err := packages.AssertKnownOptional(name)
	if err != nil {
		return PackageStatus{}, err
	}
	if err := enablePackageOnce(ctx, meta, m); err != nil {
		return PackageStatus{}, err
	}
	if err := autoEnableSatisfiedBridges(ctx, meta); err != nil {
		return PackageStatus{}, err
	}
	return packageStatus(ctx, meta, m)
}

func enablePackageOnce(ctx context.Context, meta *metadata.Service, m packages.Module) error {
	for _, dep := range m.DependsOn {
		depVer, err := meta.GetPackageInstallVersion(ctx, dep)
		if err != nil {
			return err
		}
		if depVer == "" {
			return fmt.Errorf("dependency not installed: %s", dep)
		}
	}
	if err := syncModuleDefs(ctx, meta, m); err != nil {
		return err
	}
	if err := cloneAgentsStarterAfterEnable(ctx, meta, m); err != nil {
		return err
	}
	return meta.RecordPackageInstall(ctx, m.Name, m.Version)
}

// autoEnableSatisfiedBridges enables every AutoEnable optional module whose
// DependsOn packages are all installed. Idempotent.
func autoEnableSatisfiedBridges(ctx context.Context, meta *metadata.Service) error {
	for _, m := range packages.ListOptional() {
		if !m.AutoEnable {
			continue
		}
		ready := true
		for _, dep := range m.DependsOn {
			depVer, depEnabled, err := meta.GetPackageInstall(ctx, dep)
			if err != nil {
				return err
			}
			if depVer == "" || !depEnabled {
				ready = false
				break
			}
		}
		if !ready {
			continue
		}
		ver, enabled, err := meta.GetPackageInstall(ctx, m.Name)
		if err != nil {
			return err
		}
		if enabled && ver == m.Version {
			// Still re-sync defs so additive upgrades land even if already enabled.
			if err := syncModuleDefs(ctx, meta, m); err != nil {
				return fmt.Errorf("auto-enable sync %s: %w", m.Name, err)
			}
			continue
		}
		if err := enablePackageOnce(ctx, meta, m); err != nil {
			return fmt.Errorf("auto-enable %s: %w", m.Name, err)
		}
		slog.Info("seed: auto-enabled package", "name", m.Name, "version", m.Version)
	}
	return nil
}

// DisablePackage soft-disables an optional module (keeps metadata/records).
// AutoEnable bridge packages cannot be soft-disabled while their dependencies
// remain installed — disable a parent cloud module instead.
func DisablePackage(ctx context.Context, meta *metadata.Service, name string) (PackageStatus, error) {
	m, err := packages.AssertKnownOptional(name)
	if err != nil {
		return PackageStatus{}, err
	}
	if m.AutoEnable {
		return PackageStatus{}, fmt.Errorf("package %s is auto-enabled when its dependencies are installed; disable a dependency instead", name)
	}
	ver, _, err := meta.GetPackageInstall(ctx, name)
	if err != nil {
		return PackageStatus{}, err
	}
	if ver == "" {
		return PackageStatus{}, fmt.Errorf("package not installed: %s", name)
	}
	if err := meta.SetPackageEnabled(ctx, name, false); err != nil {
		return PackageStatus{}, err
	}
	return packageStatus(ctx, meta, m)
}

// MigrateEnabledModules re-runs additive migrate for enabled optional packages.
// Packages are ordered so dependencies migrate before dependents.
// Also auto-enables any AutoEnable bridges whose dependencies are already installed.
func MigrateEnabledModules(ctx context.Context, meta *metadata.Service) error {
	if err := autoEnableSatisfiedBridges(ctx, meta); err != nil {
		return err
	}
	names, err := meta.ListEnabledPackageInstalls(ctx)
	if err != nil {
		return err
	}
	mods, err := orderModulesByDeps(names)
	if err != nil {
		return err
	}
	for _, m := range mods {
		if !m.Optional {
			continue
		}
		if err := syncModuleDefs(ctx, meta, m); err != nil {
			return fmt.Errorf("migrate package %s: %w", m.Name, err)
		}
		if err := cloneAgentsStarterAfterEnable(ctx, meta, m); err != nil {
			return fmt.Errorf("clone agent specs %s: %w", m.Name, err)
		}
		if err := meta.RecordPackageInstall(ctx, m.Name, m.Version); err != nil {
			return fmt.Errorf("record package %s: %w", m.Name, err)
		}
		slog.Info("seed: optional package migrated", "name", m.Name, "version", m.Version)
	}
	return nil
}

// orderModulesByDeps returns registered modules for names in dependency-first order.
func orderModulesByDeps(names []string) ([]packages.Module, error) {
	set := map[string]packages.Module{}
	for _, name := range names {
		m, ok := packages.Get(name)
		if !ok {
			return nil, fmt.Errorf("enabled package %s is not present in this product image", name)
		}
		set[name] = m
	}
	visited := map[string]bool{}
	var out []packages.Module
	var visit func(string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		m, ok := set[name]
		if !ok {
			return nil // dependency outside enabled set (e.g. core) — skip
		}
		visited[name] = true
		for _, dep := range m.DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}
		out = append(out, m)
		return nil
	}
	// Stable: sort names first so tie-breaking is deterministic.
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	for _, name := range sorted {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return out, nil
}
