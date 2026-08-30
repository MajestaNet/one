// Package packages is the managed-module registry (product image catalog).
// Install/enable logic lives in internal/seed to avoid import cycles with metadata.
package packages

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ObjectDef is a managed object definition shipped in a module.
type ObjectDef struct {
	APIName     string
	Label       string
	PluralLabel string
	StorageMode string // flexible (default) | high_volume | kernel
	Features    map[string]bool
	Fields      []FieldDef
}

// FieldDef is a managed field definition.
type FieldDef struct {
	APIName          string
	Label            string
	FieldType        string
	Required         bool
	UniqueField      bool
	ExternalID       bool
	Indexed          bool
	Filterable       bool
	Sortable         bool
	Searchable       bool
	Length           *int
	PicklistValues   []string
	ReferenceTo      *string
	RelationshipName *string
	KernelColumn     string // users.* column for storage_mode=kernel objects
	AutonumberFormat *string
	AutonumberStart  *int
}

// FieldExtension adds managed fields to an object owned by another package
// (e.g. crm_bridge Case.OpportunityId). Does not SyncObjectManaged the host object.
type FieldExtension struct {
	ObjectAPIName string
	Fields        []FieldDef
}

// ActionDef is a product Go platform action declared on a managed module (ADR-029).
// Definitions are compile-time; availability is package_installs.enabled at runtime.
type ActionDef struct {
	APIName          string // dotted noun.verb, e.g. "lead.convert"
	Label            string
	Description      string
	RequiresPackages []string // all must be enabled
	OptionalPackages []string // options may require these
	Objects          []string // describe/docs; AuthZ is still per record
	SyncSafe         bool
	InputJSONSchema  string // draft-07 object schema, product-owned
	OutputJSONSchema string
}

// RegisteredAction is an ActionDef plus the module that declared it.
type RegisteredAction struct {
	Module string
	Def    ActionDef
}

// Module is one optional (or core) managed package shipped in the product image.
type Module struct {
	Name              string
	Version           string
	Label             string
	Description       string
	DependsOn         []string
	Optional          bool // false for always-on core
	DocumentationPath string
	Objects           []ObjectDef
	// Actions are package-gated Client verbs (ADR-029). No kernel definitions table.
	Actions []ActionDef
	// FieldExtensions are managed fields on objects owned by a dependency package.
	FieldExtensions []FieldExtension
	// AutoEnable: when true, Majesta One enables this package automatically once every
	// DependsOn package is installed (e.g. crm_bridge when sales+service are on).
	// Customers should not need to enable/disable these bridge packages manually.
	AutoEnable bool
	// AgentSpecTemplates are cloned to ownership=custom on enable (never overwrite existing api_name).
	AgentSpecTemplates []AgentSpecTemplate
	// CanvasSpecTemplates are upserted as ownership=managed on enable (product-owned; package-gated).
	CanvasSpecTemplates []CanvasSpecTemplate
}

// AgentSpecTemplate is a starter AgentSpec definition shipped in the product image.
type AgentSpecTemplate struct {
	APIName         string
	Label           string
	GoalTemplate    string
	Instructions    string
	PrimarySection  string // operate|run|build|ship|govern|settings (BP-053)
	JobClass        string // query|customize|ship|govern|operate|skill (BP-064)
	AllowedTools    []string
	ObjectScopes    []string
	RequireApproval bool
}

// CanvasSpecTemplate is a managed CanvasSpec shipped with a package (ADR-018 Phase 5).
type CanvasSpecTemplate struct {
	APIName      string
	Label        string
	Description  string
	LayoutJSON   string // JSON object for layout
	NodesJSON    string // JSON array for nodes
	BindingsJSON string // JSON array for dataBindings (optional)
}

var (
	mu       sync.RWMutex
	registry = map[string]Module{}
)

// Register adds or replaces a module in the image registry.
func Register(m Module) {
	if m.Name == "" {
		panic("packages: module name required")
	}
	mu.Lock()
	defer mu.Unlock()
	registry[m.Name] = m
}

// Get returns a registered module.
func Get(name string) (Module, bool) {
	mu.RLock()
	defer mu.RUnlock()
	m, ok := registry[name]
	return m, ok
}

// List returns all registered modules sorted by name.
func List() []Module {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Module, 0, len(registry))
	for _, m := range registry {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ListOptional returns optional modules only.
func ListOptional() []Module {
	all := List()
	out := make([]Module, 0, len(all))
	for _, m := range all {
		if m.Optional {
			out = append(out, m)
		}
	}
	return out
}

// IsManagedPackageName reports whether a package name is product-managed
// (registered module, core, or legacy alias).
func IsManagedPackageName(pkg *string) bool {
	if pkg == nil {
		return false
	}
	name := *pkg
	if name == "" {
		return false
	}
	switch name {
	case "core", "platform", "crm", "erp":
		return true
	}
	mu.RLock()
	_, ok := registry[name]
	mu.RUnlock()
	return ok
}

// ActionsByName returns every registered platform action keyed by apiName.
// Duplicate apiNames across modules are an error (product catalog must be unique).
func ActionsByName() (map[string]RegisteredAction, error) {
	mu.RLock()
	defer mu.RUnlock()
	out := make(map[string]RegisteredAction)
	var dups []string
	for _, m := range registry {
		for _, def := range m.Actions {
			if def.APIName == "" {
				continue
			}
			if _, exists := out[def.APIName]; exists {
				dups = append(dups, def.APIName)
				continue
			}
			out[def.APIName] = RegisteredAction{Module: m.Name, Def: def}
		}
	}
	if len(dups) > 0 {
		sort.Strings(dups)
		return out, fmt.Errorf("duplicate platform action apiName: %s", strings.Join(dups, ", "))
	}
	return out, nil
}

// AssertKnownOptional returns an error if name is not an optional registered module.
func AssertKnownOptional(name string) (Module, error) {
	m, ok := Get(name)
	if !ok {
		return Module{}, fmt.Errorf("unknown package: %s", name)
	}
	if !m.Optional {
		return Module{}, fmt.Errorf("package %s is not optionally enableable", name)
	}
	return m, nil
}
