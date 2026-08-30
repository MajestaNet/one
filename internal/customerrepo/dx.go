package customerrepo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// PathManifest is manifests/<name>.yaml — One-native selective pack list.
type PathManifest struct {
	Paths []string `yaml:"paths" json:"paths"`
}

// EnvironmentFile is environments/<role>.yaml (non-secret install pointers).
type EnvironmentFile struct {
	Alias       string `yaml:"alias,omitempty" json:"alias,omitempty"`
	InstallID   string `yaml:"installId" json:"installId"`
	InstallRole string `yaml:"installRole" json:"installRole"`
	BaseURL     string `yaml:"baseUrl" json:"baseUrl"`
	// FileStem is the YAML filename without extension (used when Alias empty).
	FileStem string `yaml:"-" json:"-"`
}

// ChangeMeta is changes/<slug>/CHANGE.yaml.
type ChangeMeta struct {
	Title      string   `yaml:"title" json:"title"`
	Risk       string   `yaml:"risk,omitempty" json:"risk,omitempty"`
	TargetEnvs []string `yaml:"targetEnvs,omitempty" json:"targetEnvs,omitempty"`
	Summary    string   `yaml:"summary,omitempty" json:"summary,omitempty"`
}

func resolveIncludePaths(abs string, opts PackOptions) ([]string, error) {
	var paths []string
	paths = append(paths, opts.IncludePaths...)
	if opts.ManifestName != "" {
		mp, err := LoadPathManifest(abs, opts.ManifestName)
		if err != nil {
			return nil, err
		}
		paths = append(paths, mp...)
	}
	out := make([]string, 0, len(paths))
	seen := map[string]struct{}{}
	for _, p := range paths {
		n := normalizeIncludePath(p)
		if n == "" {
			continue
		}
		if !allowedIncludeRoot(n) {
			return nil, fmt.Errorf("include path %q must be under metadata/, src/, or tests/", p)
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out, nil
}

func normalizeIncludePath(p string) string {
	p = strings.TrimSpace(filepath.ToSlash(p))
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, "/")
	return strings.TrimSuffix(p, "/")
}

func allowedIncludeRoot(p string) bool {
	return p == "metadata" || strings.HasPrefix(p, "metadata/") ||
		p == "src" || strings.HasPrefix(p, "src/") ||
		p == "tests" || strings.HasPrefix(p, "tests/")
}

// pathAllowFn returns nil when include is empty (allow all).
func pathAllowFn(include []string) func(rel string) bool {
	if len(include) == 0 {
		return nil
	}
	return func(rel string) bool {
		rel = filepath.ToSlash(rel)
		for _, pref := range include {
			if rel == pref || strings.HasPrefix(rel, pref+"/") {
				return true
			}
		}
		return false
	}
}

// LoadPathManifest reads manifests/<name>.yaml and returns path prefixes.
func LoadPathManifest(root, name string) ([]string, error) {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, ".yaml")
	name = strings.TrimSuffix(name, ".yml")
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
		return nil, fmt.Errorf("invalid manifest name %q", name)
	}
	for _, ext := range []string{".yaml", ".yml"} {
		p := filepath.Join(root, "manifests", name+ext)
		b, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		var m PathManifest
		if err := yaml.Unmarshal(b, &m); err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}
		return m.Paths, nil
	}
	return nil, fmt.Errorf("manifests/%s.yaml not found", name)
}

// LoadEnvironments reads environments/*.yaml (non-secret pointers).
func LoadEnvironments(root string) ([]EnvironmentFile, error) {
	dir := filepath.Join(root, "environments")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []EnvironmentFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		low := strings.ToLower(name)
		if !strings.HasSuffix(low, ".yaml") && !strings.HasSuffix(low, ".yml") {
			continue
		}
		stem := strings.TrimSuffix(strings.TrimSuffix(name, filepath.Ext(name)), "")
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		var env EnvironmentFile
		if err := yaml.Unmarshal(b, &env); err != nil {
			return nil, fmt.Errorf("environments/%s: %w", name, err)
		}
		env.FileStem = stem
		if env.Alias == "" {
			env.Alias = stem
		}
		if env.InstallRole == "" {
			env.InstallRole = stem
		}
		out = append(out, env)
	}
	// Stable order: test, staging, prod, then alpha by alias.
	rank := func(role string) int {
		switch strings.ToLower(role) {
		case "test", "dev", "development":
			return 0
		case "staging", "uat", "stage":
			return 1
		case "prod", "production":
			return 2
		default:
			return 10
		}
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			ri, rj := rank(out[i].InstallRole), rank(out[j].InstallRole)
			if rj < ri || (rj == ri && out[j].Alias < out[i].Alias) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

// CreateChange writes changes/<slug>/CHANGE.yaml (does not create a git branch).
func CreateChange(root, slug string, meta ChangeMeta) (string, error) {
	slug = strings.TrimSpace(slug)
	slug = strings.TrimPrefix(slug, "change/")
	if slug == "" || strings.Contains(slug, "/") || strings.Contains(slug, "..") {
		return "", fmt.Errorf("invalid change slug %q", slug)
	}
	if meta.Title == "" {
		meta.Title = slug
	}
	if meta.Risk == "" {
		meta.Risk = "low"
	}
	dir := filepath.Join(root, "changes", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, "CHANGE.yaml")
	if _, err := os.Stat(p); err == nil {
		return "", fmt.Errorf("change already exists: %s", p)
	}
	b, err := yaml.Marshal(meta)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		return "", err
	}
	return p, nil
}

// ActiveChangeFromBranch returns the slug when branch is change/<slug>.
func ActiveChangeFromBranch(branch string) string {
	branch = strings.TrimSpace(branch)
	if !strings.HasPrefix(branch, "change/") {
		return ""
	}
	return strings.TrimPrefix(branch, "change/")
}
