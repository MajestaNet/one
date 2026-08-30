package datapack

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MajestaNet/ide/internal/customerrepo"
	"gopkg.in/yaml.v3"
)

const APIVersion = "one-datapack/v1"

// Manifest is data/<pack>/datapack.yaml.
type Manifest struct {
	APIVersion  string   `yaml:"apiVersion" json:"apiVersion"`
	Name        string   `yaml:"name" json:"name"`
	Version     string   `yaml:"version,omitempty" json:"version,omitempty"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	SourceEnv   string   `yaml:"sourceEnv,omitempty" json:"sourceEnv,omitempty"`
	Requires    Requires `yaml:"requires,omitempty" json:"requires,omitempty"`
	Steps       []Step   `yaml:"steps" json:"steps"`
}

// Requires lists soft prerequisites on the target org.
type Requires struct {
	Objects []string `yaml:"objects,omitempty" json:"objects,omitempty"`
}

// Step is one ordered object load.
type Step struct {
	ID              string      `yaml:"id" json:"id"`
	Object          string      `yaml:"object" json:"object"`
	Operation       string      `yaml:"operation" json:"operation"`
	ExternalIDField string      `yaml:"externalIdField" json:"externalIdField"`
	File            string      `yaml:"file,omitempty" json:"file,omitempty"`
	Query           *StepQuery  `yaml:"query,omitempty" json:"query,omitempty"`
	After           []string    `yaml:"after,omitempty" json:"after,omitempty"`
	References      []Reference `yaml:"references,omitempty" json:"references,omitempty"`
}

// StepQuery describes a live pull from the source peer.
type StepQuery struct {
	Select []string `yaml:"select" json:"select"`
}

// Reference rewrites a lookup via parent external id.
type Reference struct {
	Field             string `yaml:"field" json:"field"`
	From              string `yaml:"from,omitempty" json:"from,omitempty"` // offline helper column
	ToObject          string `yaml:"toObject" json:"toObject"`
	ToExternalIDField string `yaml:"toExternalIdField" json:"toExternalIdField"`
}

// LoadManifest reads datapack.yaml from dir (or the file itself).
func LoadManifest(path string) (*Manifest, string /*dir*/, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", err
	}
	dir := path
	file := filepath.Join(path, "datapack.yaml")
	if !info.IsDir() {
		dir = filepath.Dir(path)
		file = path
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, "", err
	}
	var m Manifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, "", fmt.Errorf("parse datapack: %w", err)
	}
	return &m, dir, nil
}

// Validate checks manifest shape, DAG, and optional sourceEnv resolution against repoRoot.
func Validate(m *Manifest, packDir, repoRoot string) []error {
	var errs []error
	if m == nil {
		return []error{fmt.Errorf("manifest is nil")}
	}
	if strings.TrimSpace(m.APIVersion) != APIVersion {
		errs = append(errs, fmt.Errorf("apiVersion must be %s", APIVersion))
	}
	if strings.TrimSpace(m.Name) == "" {
		errs = append(errs, fmt.Errorf("name is required"))
	}
	if len(m.Steps) == 0 {
		errs = append(errs, fmt.Errorf("at least one step is required"))
	}
	ids := map[string]struct{}{}
	for i, st := range m.Steps {
		if strings.TrimSpace(st.ID) == "" {
			errs = append(errs, fmt.Errorf("steps[%d]: id is required", i))
			continue
		}
		if _, ok := ids[st.ID]; ok {
			errs = append(errs, fmt.Errorf("duplicate step id %q", st.ID))
		}
		ids[st.ID] = struct{}{}
		if strings.TrimSpace(st.Object) == "" {
			errs = append(errs, fmt.Errorf("step %s: object is required", st.ID))
		}
		op := strings.ToLower(strings.TrimSpace(st.Operation))
		if op == "" {
			op = "upsert"
		}
		if op != "upsert" && op != "insert" && op != "update" && op != "delete" {
			errs = append(errs, fmt.Errorf("step %s: invalid operation %q", st.ID, st.Operation))
		}
		if op == "upsert" && strings.TrimSpace(st.ExternalIDField) == "" {
			errs = append(errs, fmt.Errorf("step %s: externalIdField is required for upsert", st.ID))
		}
		if st.File == "" && st.Query == nil && strings.TrimSpace(m.SourceEnv) == "" {
			errs = append(errs, fmt.Errorf("step %s: need file, query, or pack sourceEnv", st.ID))
		}
		if st.File != "" {
			fp := filepath.Join(packDir, st.File)
			if _, err := os.Stat(fp); err != nil {
				errs = append(errs, fmt.Errorf("step %s: file %s: %w", st.ID, st.File, err))
			}
		}
	}
	if orderErrs := validateDAG(m.Steps); len(orderErrs) > 0 {
		errs = append(errs, orderErrs...)
	}
	if env := strings.TrimSpace(m.SourceEnv); env != "" && repoRoot != "" {
		envs, err := customerrepo.LoadEnvironments(repoRoot)
		if err != nil {
			errs = append(errs, fmt.Errorf("sourceEnv %q: load environments: %w", env, err))
		} else if ResolveEnvironment(envs, env) == nil {
			errs = append(errs, fmt.Errorf("sourceEnv %q not found under environments/", env))
		}
	}
	return errs
}

// ResolveEnvironment finds an environment by file stem, alias, installRole, or installId.
func ResolveEnvironment(envs []customerrepo.EnvironmentFile, key string) *customerrepo.EnvironmentFile {
	key = strings.TrimSpace(key)
	for i := range envs {
		e := &envs[i]
		if e.FileStem == key || e.Alias == key || e.InstallRole == key || e.InstallID == key {
			return e
		}
	}
	return nil
}

func validateDAG(steps []Step) []error {
	byID := map[string]Step{}
	for _, st := range steps {
		byID[st.ID] = st
	}
	var errs []error
	for _, st := range steps {
		for _, dep := range st.After {
			if _, ok := byID[dep]; !ok {
				errs = append(errs, fmt.Errorf("step %s: after %q unknown", st.ID, dep))
			}
		}
	}
	if len(errs) > 0 {
		return errs
	}
	// Kahn topo — detect cycles
	indeg := map[string]int{}
	children := map[string][]string{}
	for _, st := range steps {
		if _, ok := indeg[st.ID]; !ok {
			indeg[st.ID] = 0
		}
		for _, dep := range st.After {
			indeg[st.ID]++
			children[dep] = append(children[dep], st.ID)
		}
	}
	var q []string
	for id, d := range indeg {
		if d == 0 {
			q = append(q, id)
		}
	}
	sort.Strings(q)
	seen := 0
	for len(q) > 0 {
		id := q[0]
		q = q[1:]
		seen++
		for _, c := range children[id] {
			indeg[c]--
			if indeg[c] == 0 {
				q = append(q, c)
				sort.Strings(q)
			}
		}
	}
	if seen != len(steps) {
		errs = append(errs, fmt.Errorf("steps contain a cycle"))
	}
	return errs
}

// OrderSteps returns steps in dependency order.
func OrderSteps(steps []Step) ([]Step, error) {
	if errs := validateDAG(steps); len(errs) > 0 {
		return nil, errs[0]
	}
	byID := map[string]Step{}
	indeg := map[string]int{}
	children := map[string][]string{}
	for _, st := range steps {
		byID[st.ID] = st
		indeg[st.ID] = 0
	}
	for _, st := range steps {
		for _, dep := range st.After {
			indeg[st.ID]++
			children[dep] = append(children[dep], st.ID)
		}
	}
	var q []string
	for id, d := range indeg {
		if d == 0 {
			q = append(q, id)
		}
	}
	sort.Strings(q)
	var out []Step
	for len(q) > 0 {
		id := q[0]
		q = q[1:]
		out = append(out, byID[id])
		for _, c := range children[id] {
			indeg[c]--
			if indeg[c] == 0 {
				q = append(q, c)
				sort.Strings(q)
			}
		}
	}
	return out, nil
}
