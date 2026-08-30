package agentharness

import (
	"strings"
)

// Spec is the AgentSpec slice needed to compose a run-time harness overlay.
type Spec struct {
	PrimarySection  string
	JobClass        string
	HarnessID       string
	HarnessVersion  string
	Instructions    string
	AllowedTools    []string
	RequireApproval bool
}

// Applied is the run-time result of merging product harness + customer AgentSpec.
type Applied struct {
	SystemInstructions string
	AllowedTools       []string
	RequireApproval    bool
	PrimarySection     string
	JobClass           string
	HarnessID          string
	HarnessVersion     string
	VersionMismatch    bool
	Warnings           []string
}

// KnownAgentTools is the product tool vocabulary the worker/runtime recognizes today.
var KnownAgentTools = map[string]struct{}{
	"sobjects.read":  {},
	"sobjects.write": {},
	"query":          {},
	"search":         {},
	"skills.invoke":  {},
	"actions.invoke": {},
}

// Apply merges the harness floor with customer instructions/tools.
// When JobClass is set, the job-class catalog is SoR; otherwise the section catalog (BP-053).
// Customer instructions are preserved; harness preamble is prepended.
// Tool floor cannot be dropped; unknown tools are filtered out.
func Apply(spec Spec) Applied {
	var warnings []string
	var binding Binding
	jobClass := strings.TrimSpace(spec.JobClass)
	section := strings.TrimSpace(spec.PrimarySection)
	if jobClass != "" {
		b, err := BindSpec(jobClass, section)
		if err != nil {
			warnings = append(warnings, "invalid jobClass "+jobClass+": "+err.Error())
			binding, err = Bind(section)
			if err != nil {
				binding, _ = Bind(string(SectionOperate))
				if section != "" {
					warnings = append(warnings, "invalid primarySection "+section+"; applied operate harness")
				} else {
					warnings = append(warnings, "missing primarySection; applied operate harness")
				}
			}
		} else {
			binding = b
		}
	} else {
		var err error
		binding, err = Bind(section)
		if err != nil {
			binding, _ = Bind(string(SectionOperate))
			if section != "" {
				warnings = append(warnings, "invalid primarySection "+section+"; applied operate harness")
			} else {
				warnings = append(warnings, "missing primarySection; applied operate harness")
			}
		}
	}

	if spec.HarnessID != "" && spec.HarnessID != binding.HarnessID {
		warnings = append(warnings, "pinned harnessId "+spec.HarnessID+" differs from catalog "+binding.HarnessID+"; using catalog")
	}

	versionMismatch := false
	if spec.HarnessVersion != "" && spec.HarnessVersion != binding.HarnessVersion {
		versionMismatch = true
		warnings = append(warnings, "harnessVersion "+spec.HarnessVersion+" != catalog "+binding.HarnessVersion+"; floor from catalog "+binding.HarnessVersion)
	}

	tools := EnsureToolFloor(binding.ToolFloor, spec.AllowedTools)
	tools = FilterKnownTools(tools)

	return Applied{
		SystemInstructions: ComposeSystemInstructions(binding.SystemPreamble, spec.Instructions),
		AllowedTools:       tools,
		RequireApproval:    EffectiveRequireApproval(binding.RequireApprovalDefault, spec.RequireApproval),
		PrimarySection:     binding.PrimarySection,
		JobClass:           binding.JobClass,
		HarnessID:          binding.HarnessID,
		HarnessVersion:     binding.HarnessVersion,
		VersionMismatch:    versionMismatch,
		Warnings:           warnings,
	}
}

// ComposeSystemInstructions prepends the harness preamble to customer instructions.
func ComposeSystemInstructions(preamble, customerInstructions string) string {
	preamble = strings.TrimSpace(preamble)
	customer := strings.TrimSpace(customerInstructions)
	switch {
	case preamble == "" && customer == "":
		return "You are a helpful assistant operating inside a Majesta One customer install. Answer clearly and concisely."
	case preamble == "":
		return customer
	case customer == "":
		return preamble
	default:
		return preamble + "\n\n" + customer
	}
}

// FilterKnownTools keeps tools present in KnownAgentTools (order preserved).
func FilterKnownTools(tools []string) []string {
	out := make([]string, 0, len(tools))
	seen := map[string]struct{}{}
	for _, t := range tools {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, ok := KnownAgentTools[t]; !ok {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

// Meta returns a small map suitable for run output / SSE harness events.
func (a Applied) Meta() map[string]any {
	m := map[string]any{
		"primarySection": a.PrimarySection,
		"harnessId":      a.HarnessID,
		"harnessVersion": a.HarnessVersion,
	}
	if a.JobClass != "" {
		m["jobClass"] = a.JobClass
	}
	if a.VersionMismatch {
		m["versionMismatch"] = true
	}
	if len(a.Warnings) > 0 {
		m["warnings"] = a.Warnings
	}
	return m
}
