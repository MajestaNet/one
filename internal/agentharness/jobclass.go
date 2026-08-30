package agentharness

import (
	"fmt"
	"slices"
	"strings"
)

// JobClass is the product SoR for harness floors (ADR-030 / BP-064).
type JobClass string

const (
	JobClassQuery     JobClass = "query"
	JobClassCustomize JobClass = "customize"
	JobClassShip      JobClass = "ship"
	JobClassGovern    JobClass = "govern"
	JobClassOperate   JobClass = "operate"
	JobClassSkill     JobClass = "skill"
)

// JobCatalogVersion is the job-class catalog revision (independent of CatalogVersion).
const JobCatalogVersion = "1"

var jobCatalog = []Definition{
	{
		ID:       "harness.query.read",
		JobClass: JobClassQuery,
		Section:  SectionOperate,
		Version:  JobCatalogVersion,
		Label:    "Query / ask",
		Job:      "Ask / query business data",
		SystemPreamble: `You are a Majesta One query agent on this install.
Prefer describe, query, and search before proposing writes. Surface matching records clearly.
Stay within AgentSpec allowlists and the caller's AuthZ. Do not invent schema or bypass approval gates.`,
		ToolFloor:              []string{"sobjects.read", "query", "search"},
		RequireApprovalDefault: true,
		ContextPackHints:       []string{"activeEnv", "selection"},
	},
	{
		ID:       "harness.customize.metadata",
		JobClass: JobClassCustomize,
		Section:  SectionBuild,
		Version:  JobCatalogVersion,
		Label:    "Customize metadata",
		Job:      "Shape customer-owned metadata",
		SystemPreamble: `You are a Majesta One customize agent on this install.
Help design customer-owned objects, fields, and validation. Never mutate managed package definitions.
Prefer dry-run and approval for writes. You are not a Majesta One product engineer.`,
		ToolFloor:              []string{"sobjects.read", "query"},
		RequireApprovalDefault: true,
		ContextPackHints:       []string{"openObject", "packageContext"},
	},
	{
		ID:       "harness.ship.release",
		JobClass: JobClassShip,
		Section:  SectionShip,
		Version:  JobCatalogVersion,
		Label:    "Ship / release",
		Job:      "Validate / pack / deploy vs org",
		SystemPreamble: `You are a Majesta One ship agent on this install.
Advise on change sets and promote readiness. Deploy verbs succeed only when the principal has deploy scope.
Always require human approval before org deploy. Prefer one and Deploy HTTP as the Ship path of record.`,
		ToolFloor:              []string{"sobjects.read", "query"},
		RequireApprovalDefault: true,
		ContextPackHints:       []string{"changeSet", "envPeers"},
	},
	{
		ID:       "harness.govern.admin",
		JobClass: JobClassGovern,
		Section:  SectionGovern,
		Version:  JobCatalogVersion,
		Label:    "Govern / admin",
		Job:      "Identity / permission sets / install policy",
		SystemPreamble: `You are a Majesta One govern agent on this install.
Help with principals, roles, permission sets, and install policy. Prefer read before write.
High-risk identity or AuthZ changes always require human approval.`,
		ToolFloor:              []string{"sobjects.read", "query"},
		RequireApprovalDefault: true,
		ContextPackHints:       []string{"principals", "capsSummary"},
	},
	{
		ID:       "harness.operate.mutate",
		JobClass: JobClassOperate,
		Section:  SectionRun,
		Version:  JobCatalogVersion,
		Label:    "Operate / mutate",
		Job:      "Record mutate + platform actions",
		SystemPreamble: `You are a Majesta One operate agent on this install.
Help with record reads and, when the customer widens the allowlist, writes and platform actions.
Prefer query before CRM writes. High-risk mutations require human approval.`,
		ToolFloor:              []string{"sobjects.read", "query"},
		RequireApprovalDefault: true,
		ContextPackHints:       []string{"selection", "boardHandoff"},
	},
	{
		ID:       "harness.skill.invoke",
		JobClass: JobClassSkill,
		Version:  JobCatalogVersion,
		Label:    "Skill invoke",
		Job:      "Invoke named automations only",
		SystemPreamble: `You are a Majesta One skill agent on this install.
You may invoke only named automations in the AgentSpec allowedSkills list, and only when the principal's permission sets grant canRun.
Do not invent additional tools or bypass AuthZ.`,
		ToolFloor:              []string{"skills.invoke"},
		RequireApprovalDefault: true,
		ContextPackHints:       []string{"allowedSkills"},
	},
}

// JobCatalog returns a copy of product job-class harness definitions.
func JobCatalog() []Definition {
	out := make([]Definition, len(jobCatalog))
	copy(out, jobCatalog)
	return out
}

// ParseJobClass validates a job class string.
func ParseJobClass(raw string) (JobClass, error) {
	jc := JobClass(strings.TrimSpace(strings.ToLower(raw)))
	switch jc {
	case JobClassQuery, JobClassCustomize, JobClassShip, JobClassGovern, JobClassOperate, JobClassSkill:
		return jc, nil
	default:
		return "", fmt.Errorf("jobClass must be one of query|customize|ship|govern|operate|skill")
	}
}

// CanonicalPrimarySection validates a stored primarySection token without collapsing aliases.
func CanonicalPrimarySection(raw string) (string, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	switch s {
	case "operate", "run", "build", "ship", "govern", "settings":
		return s, nil
	default:
		return "", fmt.Errorf("primarySection must be one of operate|run|build|ship|govern|settings")
	}
}

// JobClassForSection maps a stored primarySection token to a job class (BP-064 compat map).
func JobClassForSection(primarySection string) JobClass {
	switch strings.TrimSpace(strings.ToLower(primarySection)) {
	case "operate":
		return JobClassQuery
	case "run":
		return JobClassOperate
	case "build":
		return JobClassCustomize
	case "ship":
		return JobClassShip
	case "govern", "settings":
		return JobClassGovern
	default:
		return ""
	}
}

// SectionForJobClass maps a job class to a stored primarySection alias.
// skill has no section home (empty).
func SectionForJobClass(jc JobClass) string {
	switch jc {
	case JobClassQuery:
		return "operate"
	case JobClassCustomize:
		return "build"
	case JobClassShip:
		return "ship"
	case JobClassGovern:
		return "govern"
	case JobClassOperate:
		return "run"
	default:
		return ""
	}
}

// ForJobClass returns the job-class harness definition.
func ForJobClass(jc JobClass) (Definition, bool) {
	for _, d := range jobCatalog {
		if d.JobClass == jc {
			return d, true
		}
	}
	return Definition{}, false
}

func bindingFromDef(def Definition, primarySection string, jc JobClass) Binding {
	floor := slices.Clone(def.ToolFloor)
	return Binding{
		PrimarySection:         primarySection,
		JobClass:               string(jc),
		HarnessID:              def.ID,
		HarnessVersion:         def.Version,
		ToolFloor:              floor,
		RequireApprovalDefault: def.RequireApprovalDefault,
		SystemPreamble:         def.SystemPreamble,
	}
}

// BindJobClass resolves the product harness for a job class.
func BindJobClass(jobClass string) (Binding, error) {
	jc, err := ParseJobClass(jobClass)
	if err != nil {
		return Binding{}, err
	}
	def, ok := ForJobClass(jc)
	if !ok {
		return Binding{}, fmt.Errorf("no harness registered for jobClass %q", jc)
	}
	return bindingFromDef(def, SectionForJobClass(jc), jc), nil
}

// BindSpec accepts jobClass XOR primarySection (both allowed when they agree) and fills the other.
// Settings keeps harness.settings.install until the section/job-class catalogs merge.
func BindSpec(jobClass, primarySection string) (Binding, error) {
	jcRaw := strings.TrimSpace(jobClass)
	secRaw := strings.TrimSpace(primarySection)
	if jcRaw == "" && secRaw == "" {
		return Binding{}, fmt.Errorf("jobClass or primarySection is required")
	}

	var jc JobClass
	if jcRaw != "" {
		parsed, err := ParseJobClass(jcRaw)
		if err != nil {
			return Binding{}, err
		}
		jc = parsed
	}

	var storedSection string
	if secRaw != "" {
		canonical, err := CanonicalPrimarySection(secRaw)
		if err != nil {
			return Binding{}, err
		}
		storedSection = canonical
	}

	if jcRaw != "" && storedSection != "" {
		expected := JobClassForSection(storedSection)
		if expected != jc {
			return Binding{}, fmt.Errorf("jobClass %s does not match primarySection %s (expected %s)", jc, storedSection, expected)
		}
	}
	if jc == "" {
		jc = JobClassForSection(storedSection)
		if jc == "" {
			return Binding{}, fmt.Errorf("cannot derive jobClass from primarySection %q", storedSection)
		}
	}
	if storedSection == "" {
		storedSection = SectionForJobClass(jc)
	}

	// settings → govern job class, but the read-only settings floor stays until catalog merge.
	if storedSection == string(SectionSettings) {
		b, err := Bind(string(SectionSettings))
		if err != nil {
			return Binding{}, err
		}
		b.JobClass = string(JobClassGovern)
		b.PrimarySection = string(SectionSettings)
		return b, nil
	}

	def, ok := ForJobClass(jc)
	if !ok {
		return Binding{}, fmt.Errorf("no harness registered for jobClass %q", jc)
	}
	return bindingFromDef(def, storedSection, jc), nil
}
