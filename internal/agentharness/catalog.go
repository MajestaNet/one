// Package agentharness is the product-managed harness catalog.
// BP-064 job classes are the SoR; BP-053 primarySection remains a compatibility alias.
// Customers own AgentSpec instructions; Majesta One owns floors (tools, preamble, approval defaults).
package agentharness

import (
	"fmt"
	"slices"
	"strings"
)

// Section is one of the four Control IDE launcher sections (+ legacy aliases run/ship).
type Section string

const (
	SectionOperate  Section = "operate"
	SectionRun      Section = "run" // legacy alias → operate
	SectionBuild    Section = "build"
	SectionShip     Section = "ship" // legacy alias → build
	SectionGovern   Section = "govern"
	SectionSettings Section = "settings"
)

// CatalogVersion is the product harness catalog revision (pinned onto AgentSpecs).
// Bump this (and each Definition.Version) when floors/preambles change — see
// docs/architecture/agent-section-harness-build-plan.md harness version bump playbook.
const CatalogVersion = "5"

// Definition is one One-managed section harness.
type Definition struct {
	ID                     string   `json:"id"`
	Section                Section  `json:"section,omitempty"`
	JobClass               JobClass `json:"jobClass,omitempty"`
	Version                string   `json:"version"`
	Label                  string   `json:"label"`
	Job                    string   `json:"job"`
	SystemPreamble         string   `json:"systemPreamble"`
	ToolFloor              []string `json:"toolFloor"`
	RequireApprovalDefault bool     `json:"requireApprovalDefault"`
	ContextPackHints       []string `json:"contextPackHints,omitempty"`
	ChromeHints            []string `json:"chromeHints,omitempty"`
}

// Binding is the resolved harness attachment written onto an AgentSpec.
type Binding struct {
	PrimarySection         string
	JobClass               string
	HarnessID              string
	HarnessVersion         string
	ToolFloor              []string
	RequireApprovalDefault bool
	SystemPreamble         string
}

var catalog = []Definition{
	{
		ID:      "harness.run.tools",
		Section: SectionOperate,
		Version: CatalogVersion,
		Label:   "Operate graph",
		Job:     "Guide personal graph / ToolSpec composition and use",
		SystemPreamble: "You are operating inside Majesta One Control IDE Operate.\n" +
			"Operate opens on the user's personal reference-only graph. Prefer graph.get, graph.pin, graph.pinCollection, graph.cluster, graph.mountTool, graph.link, graph.unlink, graph.annotate, and graph.layout to organize that home. Pin a collection node when the user needs an object or saved list on the graph; never explode query rows onto the canvas. Never place query results, record fields, rows, or hydrated cards into graph operations.\n" +
			"When runGraph or refs-only selection context is present, end each substantive turn with a visible graph.pin, graph.link, graph.annotate, or staged proposal that the user can Apply. Emit those actions inside a final oneEffects JSON fence (graphCalls / proposal / boardHandoff as needed). If no topology change is appropriate, explain why honestly in prose and omit graphCalls.\n" +
			"Proposal mutation payloads stay in run evidence and IDE session staging; never put operations or data maps into the graph.\n" +
			"Act explicitly as a Curator when rebuilding My day, maintaining next/watches/blocks, or raising signal insights. Act as a Doer when staging Client-shaped mutation proposals; never silently mutate CRM through graph state.\n" +
			"After a successful personal workflow becomes repeatable, suggest graph.publishSubgraph so the user can publish an org ToolSpec playbook. The personal graph remains private; never describe it as team-shared.\n" +
			"Help the user compose or revise declarative Tools (ToolSpecs) when a reusable tool is requested. Prefer query before CRM writes.\n" +
			"Stay within allowlists and current Client AuthZ; high-risk CRM mutations require human approval.",
		ToolFloor:              []string{"sobjects.read", "query"},
		RequireApprovalDefault: true,
		ContextPackHints:       []string{"runGraph", "graphSelection", "signalBindings", "activeToolSpec", "sessionTool"},
		ChromeHints:            []string{"runGraphHome", "liveSignal", "proposalApply", "publishCoach", "toolHandoffCards"},
	},
	{
		ID:      "harness.operate.query",
		Section: SectionBuild,
		Version: CatalogVersion,
		Label:   "Build inspect",
		Job:     "Query / ask on business data in the active install",
		SystemPreamble: `You are operating inside Majesta One Control IDE Build (inspect).
Prefer query and describe before writes. Surface matching records clearly for human approval when writes are allowed.
Stay within the bound AgentSpec allowlists and AuthZ. Do not invent schema or bypass approval gates.`,
		ToolFloor:              []string{"sobjects.read", "query"},
		RequireApprovalDefault: true,
		ContextPackHints:       []string{"activeEnv", "selection", "boardHandoff"},
		ChromeHints:            []string{"inlineResults", "approve"},
	},
	{
		ID:      "harness.build.metadata",
		Section: SectionBuild,
		Version: CatalogVersion,
		Label:   "Build metadata",
		Job:     "Shape customer-owned metadata safely",
		SystemPreamble: `You are operating inside Majesta One Control IDE Build.
Help design customer-owned objects, fields, and validation. Never mutate managed package definitions.
Prefer dry-run and approval for writes. You are not a Majesta One product engineer.`,
		ToolFloor:              []string{"sobjects.read", "query"},
		RequireApprovalDefault: true,
		ContextPackHints:       []string{"openObject", "packageContext"},
		ChromeHints:            []string{"buildDualWrite"},
	},
	{
		ID:      "harness.ship.release",
		Section: SectionBuild,
		Version: CatalogVersion,
		Label:   "Build ship",
		Job:     "Validate and guide ship / promote workflows",
		SystemPreamble: `You are operating inside Majesta One Control IDE Build (ship).
Advise on change sets, peers, and promote readiness. Do not execute deploy cloud mutations via agent tools.
Prefer read and explain; deep-link humans to Build deploy panels for privileged actions.`,
		ToolFloor:              []string{"sobjects.read", "query"},
		RequireApprovalDefault: true,
		ContextPackHints:       []string{"changeSet", "envPeers"},
		ChromeHints:            []string{"linkShipPanels"},
	},
	{
		ID:      "harness.govern.admin",
		Section: SectionGovern,
		Version: CatalogVersion,
		Label:   "Govern admin",
		Job:     "Identity, permission sets, and install policy guidance",
		SystemPreamble: `You are operating inside Majesta One Control IDE Govern.
Help with principals, roles, permission sets, and install policy. Prefer read before write.
High-risk identity or AuthZ changes always require human approval.`,
		ToolFloor:              []string{"sobjects.read", "query"},
		RequireApprovalDefault: true,
		ContextPackHints:       []string{"principals", "capsSummary"},
		ChromeHints:            []string{"strongApprove"},
	},
	{
		ID:      "harness.settings.install",
		Section: SectionSettings,
		Version: CatalogVersion,
		Label:   "Account / settings",
		Job:     "Account, hosting, and inference orientation",
		SystemPreamble: `You are operating inside Majesta One Control IDE Account (settings).
Orient the user on Account, Hosting, and Inference surfaces. Prefer read-only guidance.
Never echo Hosting secrets or API keys. Deep-link humans to Settings tools for privileged changes.`,
		ToolFloor:              []string{"sobjects.read", "query"},
		RequireApprovalDefault: true,
		ContextPackHints:       []string{"ide.settings", "cloudHost"},
		ChromeHints:            []string{"accountDock", "noSecretEcho"},
	},
}

// Catalog returns a copy of all product harness definitions.
func Catalog() []Definition {
	out := make([]Definition, len(catalog))
	copy(out, catalog)
	return out
}

// ParseSection validates and normalizes a primary section string (BP-057 aliases).
func ParseSection(raw string) (Section, error) {
	s := Section(strings.TrimSpace(strings.ToLower(raw)))
	switch s {
	case SectionRun:
		return SectionOperate, nil
	case SectionShip:
		return SectionBuild, nil
	case SectionOperate, SectionBuild, SectionGovern, SectionSettings:
		return s, nil
	default:
		return "", fmt.Errorf("primarySection must be one of operate|build|govern|settings (legacy run|ship accepted)")
	}
}

// ForSection returns the harness bound to a section.
func ForSection(section Section) (Definition, bool) {
	for _, d := range catalog {
		if d.Section == section {
			return d, true
		}
	}
	return Definition{}, false
}

// ForID returns a harness by id (section catalog first, then job-class catalog).
func ForID(id string) (Definition, bool) {
	id = strings.TrimSpace(id)
	for _, d := range catalog {
		if d.ID == id {
			return d, true
		}
	}
	for _, d := range jobCatalog {
		if d.ID == id {
			return d, true
		}
	}
	return Definition{}, false
}

// Bind resolves the product harness for a required primary section.
func Bind(primarySection string) (Binding, error) {
	section, err := ParseSection(primarySection)
	if err != nil {
		return Binding{}, err
	}
	def, ok := ForSection(section)
	if !ok {
		return Binding{}, fmt.Errorf("no harness registered for section %q", section)
	}
	floor := slices.Clone(def.ToolFloor)
	return Binding{
		PrimarySection: string(section),
		// JobClass stays empty on the section-catalog path so existing YAML/rows
		// keep CatalogVersion floors until they opt into jobClass (BP-064).
		HarnessID:              def.ID,
		HarnessVersion:         def.Version,
		ToolFloor:              floor,
		RequireApprovalDefault: def.RequireApprovalDefault,
		SystemPreamble:         def.SystemPreamble,
	}, nil
}

// UnionTools returns unique tools with floor first, then extra customer tools.
func UnionTools(floor, customer []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(floor)+len(customer))
	add := func(list []string) {
		for _, t := range list {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			if _, ok := seen[t]; ok {
				continue
			}
			seen[t] = struct{}{}
			out = append(out, t)
		}
	}
	add(floor)
	add(customer)
	return out
}

// EnsureToolFloor adds any missing floor tools to customerTools (order: floor then extras).
func EnsureToolFloor(floor, customerTools []string) []string {
	return UnionTools(floor, customerTools)
}

// EffectiveRequireApproval applies the harness approval floor (cannot drop below default true).
func EffectiveRequireApproval(harnessDefault, customer bool) bool {
	if harnessDefault {
		return true
	}
	return customer
}

// StarterSection maps known starter apiNames to sections (backfill / seed).
func StarterSection(apiName string) (Section, bool) {
	switch strings.TrimSpace(apiName) {
	case "AdminSetup":
		return SectionGovern, true
	case "MetadataBuilder":
		return SectionBuild, true
	case "RunCoach":
		return SectionOperate, true
	case "ShipGuide":
		return SectionBuild, true
	case "AccountGuide":
		return SectionSettings, true
	default:
		return "", false
	}
}
