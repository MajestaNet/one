package deploy

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"

	"github.com/MajestaNet/ide/internal/packages"
)

// DefaultCustomerPackage is the default package name for customer-owned metadata.
const DefaultCustomerPackage = "customer.default"

func isManagedPackageName(name *string) bool {
	return packages.IsManagedPackageName(name)
}

func isManagedOwnership(ownership string) bool {
	return ownership == "managed"
}

// SnapshotObject is one object entry in a bundle artifact.
type SnapshotObject struct {
	APIName     string          `json:"apiName" yaml:"apiName"`
	Label       string          `json:"label" yaml:"label"`
	PluralLabel string          `json:"pluralLabel" yaml:"pluralLabel"`
	StorageMode string          `json:"storageMode" yaml:"storageMode"`
	PackageName *string         `json:"packageName,omitempty" yaml:"packageName,omitempty"`
	Ownership   string          `json:"ownership" yaml:"ownership"`
	Features    map[string]bool `json:"features" yaml:"features"`
	ID          *string         `json:"id,omitempty" yaml:"id,omitempty"`
}

// SnapshotField is one field entry in a bundle artifact.
type SnapshotField struct {
	ObjectAPIName        string   `json:"objectApiName" yaml:"objectApiName"`
	APIName              string   `json:"apiName" yaml:"apiName"`
	Label                string   `json:"label" yaml:"label"`
	FieldType            string   `json:"fieldType" yaml:"fieldType"`
	Required             bool     `json:"required" yaml:"required"`
	UniqueField          bool     `json:"uniqueField" yaml:"uniqueField"`
	ExternalID           bool     `json:"externalId,omitempty" yaml:"externalId,omitempty"`
	Indexed              *bool    `json:"indexed,omitempty" yaml:"indexed,omitempty"`
	Filterable           bool     `json:"filterable" yaml:"filterable"`
	Sortable             bool     `json:"sortable" yaml:"sortable"`
	Searchable           *bool    `json:"searchable,omitempty" yaml:"searchable,omitempty"`
	DefaultValue         any      `json:"defaultValue,omitempty" yaml:"defaultValue,omitempty"`
	Length               *int     `json:"length,omitempty" yaml:"length,omitempty"`
	Precision            *int     `json:"precision,omitempty" yaml:"precision,omitempty"`
	Scale                *int     `json:"scale,omitempty" yaml:"scale,omitempty"`
	PicklistValues       []string `json:"picklistValues,omitempty" yaml:"picklistValues,omitempty"`
	ReferenceTo          *string  `json:"referenceTo,omitempty" yaml:"referenceTo,omitempty"`
	RelationshipName     *string  `json:"relationshipName,omitempty" yaml:"relationshipName,omitempty"`
	PolymorphicTypeField *string  `json:"polymorphicTypeField,omitempty" yaml:"polymorphicTypeField,omitempty"`
	PackageName          *string  `json:"packageName,omitempty" yaml:"packageName,omitempty"`
	Ownership            string   `json:"ownership" yaml:"ownership"`
	ID                   *string  `json:"id,omitempty" yaml:"id,omitempty"`
}

// SnapshotRule is one validation rule in a bundle artifact.
type SnapshotRule struct {
	ObjectAPIName string  `json:"objectApiName" yaml:"objectApiName"`
	APIName       string  `json:"apiName" yaml:"apiName"`
	Label         string  `json:"label" yaml:"label"`
	Active        bool    `json:"active" yaml:"active"`
	ErrorMessage  string  `json:"errorMessage" yaml:"errorMessage"`
	Expression    any     `json:"expression" yaml:"expression"`
	PackageName   *string `json:"packageName,omitempty" yaml:"packageName,omitempty"`
	Ownership     string  `json:"ownership" yaml:"ownership"`
	ID            *string `json:"id,omitempty" yaml:"id,omitempty"`
}

// SnapshotAutomation is one automation in a bundle artifact.
type SnapshotAutomation struct {
	APIName          string  `json:"apiName" yaml:"apiName"`
	Label            string  `json:"label" yaml:"label"`
	ObjectAPIName    string  `json:"objectApiName" yaml:"objectApiName"`
	TriggerEvent     string  `json:"triggerEvent" yaml:"triggerEvent"`
	Active           bool    `json:"active" yaml:"active"`
	Condition        any     `json:"condition,omitempty" yaml:"condition,omitempty"`
	Actions          []any   `json:"actions" yaml:"actions"`
	Runtime          string  `json:"runtime,omitempty" yaml:"runtime,omitempty"`                   // actions | code
	Execution        string  `json:"execution,omitempty" yaml:"execution,omitempty"`               // async | sync
	EntryFile        *string `json:"entryFile,omitempty" yaml:"entryFile,omitempty"`               // src/automations/….ts
	Source           *string `json:"source,omitempty" yaml:"source,omitempty"`                     // embedded guest TS
	RunAsPrincipalID *string `json:"runAsPrincipalId,omitempty" yaml:"runAsPrincipalId,omitempty"` // required for schedule
	PackageName      *string `json:"packageName,omitempty" yaml:"packageName,omitempty"`
	Ownership        string  `json:"ownership" yaml:"ownership"`
	ID               *string `json:"id,omitempty" yaml:"id,omitempty"`
}

// SnapshotCanvasSpec is a declarative canvas / ToolSpec template (ADR-018 / ADR-021).
// Product name is ToolSpec; storage remains metadata_canvases.
type SnapshotCanvasSpec struct {
	APIName      string          `json:"apiName" yaml:"apiName"`
	Label        string          `json:"label" yaml:"label"`
	Description  string          `json:"description,omitempty" yaml:"description,omitempty"`
	Icon         string          `json:"icon,omitempty" yaml:"icon,omitempty"`
	SortOrder    int             `json:"sortOrder,omitempty" yaml:"sortOrder,omitempty"`
	Layout       json.RawMessage `json:"layout" yaml:"layout"`
	Nodes        json.RawMessage `json:"nodes" yaml:"nodes"`
	DataBindings json.RawMessage `json:"dataBindings,omitempty" yaml:"dataBindings,omitempty"`
	Active       bool            `json:"active" yaml:"active"`
	PackageName  *string         `json:"packageName,omitempty" yaml:"packageName,omitempty"`
	Ownership    string          `json:"ownership" yaml:"ownership"`
}

// SnapshotExperience is customer Client Experience config (ADR-019).
type SnapshotExperience struct {
	APIName             string   `json:"apiName" yaml:"apiName"`
	Label               string   `json:"label" yaml:"label"`
	Description         string   `json:"description,omitempty" yaml:"description,omitempty"`
	HomeURL             string   `json:"homeUrl" yaml:"homeUrl"`
	ConnectedAppAPIName string   `json:"connectedAppApiName" yaml:"connectedAppApiName"`
	AllowedOrigins      []string `json:"allowedOrigins,omitempty" yaml:"allowedOrigins,omitempty"`
	Active              bool     `json:"active" yaml:"active"`
	PackageName         *string  `json:"packageName,omitempty" yaml:"packageName,omitempty"`
	Ownership           string   `json:"ownership" yaml:"ownership"`
}

// SnapshotAgentPlaybook is one AgentSpec in a bundle artifact.
type SnapshotAgentPlaybook struct {
	APIName            string   `json:"apiName"`
	Label              string   `json:"label"`
	GoalTemplate       string   `json:"goalTemplate"`
	Instructions       string   `json:"instructions"`
	PrimarySection     string   `json:"primarySection,omitempty" yaml:"primarySection,omitempty"`
	JobClass           string   `json:"jobClass,omitempty" yaml:"jobClass,omitempty"`
	HarnessID          string   `json:"harnessId,omitempty" yaml:"harnessId,omitempty"`
	HarnessVersion     string   `json:"harnessVersion,omitempty" yaml:"harnessVersion,omitempty"`
	AllowedTools       []string `json:"allowedTools"`
	ObjectScopes       []string `json:"objectScopes"`
	AllowedSkills      []string `json:"allowedSkills,omitempty"`
	AllowedCanvasSpecs []string `json:"allowedCanvasSpecs,omitempty"`
	AllowedToolSpecs   []string `json:"allowedToolSpecs,omitempty"`
	RequireApproval    bool     `json:"requireApproval"`
	Active             bool     `json:"active"`
	PackageName        *string  `json:"packageName,omitempty"`
	Ownership          string   `json:"ownership"`
}

// SnapshotObjectPermission is one object CRUD grant inside a permission set.
type SnapshotObjectPermission struct {
	ObjectAPIName string `json:"objectApiName"`
	CanCreate     bool   `json:"canCreate"`
	CanRead       bool   `json:"canRead"`
	CanUpdate     bool   `json:"canUpdate"`
	CanDelete     bool   `json:"canDelete"`
	ViewAll       bool   `json:"viewAll"`
	ModifyAll     bool   `json:"modifyAll"`
}

// SnapshotFieldPermission is one field grant inside a permission set.
type SnapshotFieldPermission struct {
	ObjectAPIName string `json:"objectApiName"`
	FieldAPIName  string `json:"fieldApiName"`
	CanRead       bool   `json:"canRead"`
	CanEdit       bool   `json:"canEdit"`
}

// SnapshotPermissionSet is a non-system permission set definition (assignments excluded).
type SnapshotPermissionSet struct {
	APIName           string                     `json:"apiName"`
	Label             string                     `json:"label"`
	Description       *string                    `json:"description,omitempty"`
	SystemPermissions []string                   `json:"systemPermissions,omitempty"`
	ObjectPermissions []SnapshotObjectPermission `json:"objectPermissions,omitempty"`
	FieldPermissions  []SnapshotFieldPermission  `json:"fieldPermissions,omitempty"`
	AllAutomations    bool                       `json:"allAutomations,omitempty"`
	AutomationAccess  *SnapshotAutomationAccess  `json:"automationAccess,omitempty"`
	PackageName       *string                    `json:"packageName,omitempty"`
	Ownership         string                     `json:"ownership"`
}

// SnapshotAutomationAccess is the automation grant list on a permission set.
type SnapshotAutomationAccess struct {
	AllAutomations bool                           `json:"allAutomations"`
	Automations    []SnapshotAutomationPermission `json:"automations,omitempty"`
}

// SnapshotAutomationPermission is one automation canRun grant.
type SnapshotAutomationPermission struct {
	APIName string `json:"apiName"`
	CanRun  bool   `json:"canRun"`
}

// SnapshotWebhook is a webhook subscription config (secrets never included).
type SnapshotWebhook struct {
	APIName     string   `json:"apiName"`
	URL         string   `json:"url"`
	EventTypes  []string `json:"eventTypes"`
	Active      bool     `json:"active"`
	PackageName *string  `json:"packageName,omitempty"`
	Ownership   string   `json:"ownership"`
}

// SnapshotConnector is connector config (secret values / OAuth tokens never included — BP-047).
type SnapshotConnector struct {
	APIName        string         `json:"apiName" yaml:"apiName"`
	Label          string         `json:"label" yaml:"label"`
	BaseURL        string         `json:"baseUrl" yaml:"baseUrl"`
	SecretRef      *string        `json:"secretRef,omitempty" yaml:"secretRef,omitempty"`
	AllowedMethods []string       `json:"allowedMethods" yaml:"allowedMethods"`
	PathPrefix     string         `json:"pathPrefix,omitempty" yaml:"pathPrefix,omitempty"`
	Active         bool           `json:"active" yaml:"active"`
	AuthType       string         `json:"authType" yaml:"authType"`
	OAuthFlow      map[string]any `json:"oauthFlow,omitempty" yaml:"oauthFlow,omitempty"`
	PackageName    *string        `json:"packageName,omitempty" yaml:"packageName,omitempty"`
	Ownership      string         `json:"ownership" yaml:"ownership"`
}

// SnapshotDataRole is a customer data role for record sharing.
type SnapshotDataRole struct {
	APIName               string `json:"apiName" yaml:"apiName"`
	Label                 string `json:"label" yaml:"label"`
	ParentDataRoleAPIName string `json:"parentDataRoleApiName,omitempty" yaml:"parentDataRoleApiName,omitempty"`
}

// SnapshotObjectSharing is per-object OWD settings.
type SnapshotObjectSharing struct {
	ObjectAPIName       string `json:"objectApiName" yaml:"objectApiName"`
	DefaultAccess       string `json:"defaultAccess" yaml:"defaultAccess"`
	SharingRulesEnabled bool   `json:"sharingRulesEnabled" yaml:"sharingRulesEnabled"`
}

// SnapshotSharingRule is a criteria-based sharing rule.
type SnapshotSharingRule struct {
	ObjectAPIName           string          `json:"objectApiName" yaml:"objectApiName"`
	APIName                 string          `json:"apiName" yaml:"apiName"`
	Label                   string          `json:"label" yaml:"label"`
	Active                  bool            `json:"active" yaml:"active"`
	AccessLevel             string          `json:"accessLevel" yaml:"accessLevel"`
	SharedToDataRoleAPIName string          `json:"sharedToDataRoleApiName" yaml:"sharedToDataRoleApiName"`
	Criteria                json.RawMessage `json:"criteria" yaml:"criteria"`
	SortOrder               int             `json:"sortOrder" yaml:"sortOrder"`
}

// ManagedBaseline is a read-only managed object/field reference tree for
// one/v1 (.one/baseline). Never packed or promoted.
type ManagedBaseline struct {
	ProductVersion  string           `json:"productVersion" yaml:"productVersion"`
	GeneratedAt     string           `json:"generatedAt" yaml:"generatedAt"`
	SourceInstallID string           `json:"sourceInstallId,omitempty" yaml:"sourceInstallId,omitempty"`
	Objects         []SnapshotObject `json:"objects" yaml:"objects"`
	Fields          []SnapshotField  `json:"fields" yaml:"fields"`
}

// SnapshotTestSuite is a customer test suite embedded in a bundle.
type SnapshotTestSuite struct {
	APIName     string  `json:"apiName"`
	Label       string  `json:"label"`
	Description *string `json:"description,omitempty"`
	Active      bool    `json:"active"`
	Steps       []any   `json:"steps"`
	PackageName *string `json:"packageName,omitempty"`
	Ownership   string  `json:"ownership"`
}

// BundleArtifact is the versioned, checksummed bundle payload (manifestVersion=1).
type BundleArtifact struct {
	ManifestVersion       int                     `json:"manifestVersion"`
	Ownership             string                  `json:"ownership"`
	DefaultPackageName    string                  `json:"defaultPackageName"`
	CustomerID            *string                 `json:"customerId,omitempty"`
	ProductVersionRange   *string                 `json:"productVersionRange,omitempty"`
	SourceInstallID       *string                 `json:"sourceInstallId,omitempty"`
	SourceInstallRole     *string                 `json:"sourceInstallRole,omitempty"`
	CreatedAt             *string                 `json:"createdAt,omitempty"`
	Objects               []SnapshotObject        `json:"objects"`
	Fields                []SnapshotField         `json:"fields"`
	ValidationRules       []SnapshotRule          `json:"validationRules"`
	Automations           []SnapshotAutomation    `json:"automations"`
	AgentPlaybooks        []SnapshotAgentPlaybook `json:"agentPlaybooks"`
	Canvases              []SnapshotCanvasSpec    `json:"canvases,omitempty"`
	Experiences           []SnapshotExperience    `json:"experiences,omitempty"`
	PermissionSets        []SnapshotPermissionSet `json:"permissionSets"`
	Webhooks              []SnapshotWebhook       `json:"webhooks"`
	Connectors            []SnapshotConnector     `json:"connectors,omitempty"`
	Tests                 []SnapshotTestSuite     `json:"tests"`
	DataRoles             []SnapshotDataRole      `json:"dataRoles,omitempty"`
	ObjectSharingSettings []SnapshotObjectSharing `json:"objectSharingSettings,omitempty"`
	SharingRules          []SnapshotSharingRule   `json:"sharingRules,omitempty"`
	// Sources are repo-relative guest files packed from src/automations and tests/automations (ADR-014).
	Sources map[string]string `json:"sources,omitempty"`
	// Baseline is export/init-only managed reference metadata; validate/apply/pack ignore it.
	Baseline *ManagedBaseline `json:"baseline,omitempty"`
}

// ValidationIssue is one problem found during bundle validation.
type ValidationIssue struct {
	Severity string `json:"severity"` // "error" | "warning"
	Code     string `json:"code"`
	Message  string `json:"message"`
	Path     string `json:"path,omitempty"`
}

// ValidationReport is the output of ValidateBundleArtifact.
type ValidationReport struct {
	OK     bool              `json:"ok"`
	Issues []ValidationIssue `json:"issues"`
	Counts struct {
		Objects         int `json:"objects"`
		Fields          int `json:"fields"`
		ValidationRules int `json:"validationRules"`
		Automations     int `json:"automations"`
		AgentPlaybooks  int `json:"agentPlaybooks"`
		Canvases        int `json:"canvases"`
		Experiences     int `json:"experiences"`
		PermissionSets  int `json:"permissionSets"`
		Webhooks        int `json:"webhooks"`
		Connectors      int `json:"connectors"`
		Tests           int `json:"tests"`
	} `json:"counts"`
}

// ApplyAction is one upsert performed during ApplyBundleArtifact.
type ApplyAction struct {
	Kind          string `json:"kind"` // object|field|validationRule|automation|agentPlaybook|permissionSet|webhook|connector|test
	APIName       string `json:"apiName"`
	ObjectAPIName string `json:"objectApiName,omitempty"`
	Action        string `json:"action"` // created|updated|skipped
}

// ApplyReport is the output of ApplyBundleArtifact.
type ApplyReport struct {
	Actions []ApplyAction `json:"actions"`
	Created int           `json:"created"`
	Updated int           `json:"updated"`
	Skipped int           `json:"skipped"`
}

// canonicalJSON produces deterministic JSON with sorted keys (mirrors Node implementation).
func canonicalJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		return "", err
	}
	sorted := sortKeys(raw)
	out, err := json.Marshal(sorted)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func sortKeys(v any) any {
	switch val := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(val))
		for _, k := range keys {
			out[k] = sortKeys(val[k])
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = sortKeys(item)
		}
		return out
	default:
		return v
	}
}

// checksumArtifact returns the sha256 hex of the canonical JSON of the artifact.
func checksumArtifact(artifact any) (string, error) {
	s, err := canonicalJSON(artifact)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h), nil
}

var semverRe = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)

// parseSemver parses "major.minor.patch" prefix. Returns [0,0,0] on failure.
func parseSemver(version string) [3]int {
	m := semverRe.FindStringSubmatch(version)
	if m == nil {
		return [3]int{}
	}
	var parts [3]int
	for i := 0; i < 3; i++ {
		_, _ = fmt.Sscanf(m[i+1], "%d", &parts[i])
	}
	return parts
}

func compareSemver(a, b string) int {
	pa := parseSemver(a)
	pb := parseSemver(b)
	for i := 0; i < 3; i++ {
		if pa[i] > pb[i] {
			return 1
		}
		if pa[i] < pb[i] {
			return -1
		}
	}
	return 0
}

var geRe = regexp.MustCompile(`>=\s*(\d+\.\d+\.\d+)`)
var ltRe = regexp.MustCompile(`<\s*(\d+\.\d+\.\d+)`)
var exactRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// productVersionSatisfies checks whether version is within range (mirrors Node impl).
// Supports "*", exact "x.y.z", ">=x.y.z", and ">=x.y.z <x.y.z".
func productVersionSatisfies(version, rangeStr string) bool {
	r := regexp.MustCompile(`\s+`).ReplaceAllString(rangeStr, "")
	if r == "" || r == "*" {
		return true
	}
	if exactRe.MatchString(r) {
		return compareSemver(version, r) == 0
	}
	if m := geRe.FindStringSubmatch(rangeStr); m != nil {
		if compareSemver(version, m[1]) < 0 {
			return false
		}
	} else {
		// No >= clause and not exact/wildcard — unsatisfied.
		return false
	}
	if m := ltRe.FindStringSubmatch(rangeStr); m != nil {
		if compareSemver(version, m[1]) >= 0 {
			return false
		}
	}
	return true
}
