package agentharness

import "strings"

// Hosted tool classes for the in-process /agents/runs loop (BP-006).
const (
	ToolClassRead  = "read"
	ToolClassWrite = "write"
)

// HostedLoopV1Catalog is the MCP names the hosted loop may execute.
// Values are ToolClassRead or ToolClassWrite.
var HostedLoopV1Catalog = map[string]string{
	"describe_global": ToolClassRead,
	"describe_object": ToolClassRead,
	"get_record":      ToolClassRead,
	"query":           ToolClassRead,
	"search":          ToolClassRead,
	"create_record":   ToolClassWrite,
	"update_record":   ToolClassWrite,
	"invoke_action":   ToolClassWrite,
	"invoke_skill":    ToolClassWrite,
}

// harnessToMCP expands stored harness tokens to MCP names at admission.
// Unknown tokens are ignored; MCP names that are not in HostedLoopV1Catalog are dropped.
var harnessToMCP = map[string][]string{
	"sobjects.read":  {"describe_global", "describe_object", "get_record"},
	"query":          {"query"},
	"search":         {"search"},
	"sobjects.write": {"create_record", "update_record"},
	"skills.invoke":  {"invoke_skill"},
	"actions.invoke": {"invoke_action"},
}

// ExpandToHostedMCP maps harness tokens (floor ∪ AgentSpec allowedTools) to
// admitted hosted-loop MCP names. Stored allowlists stay tokens; execution uses MCP names.
func ExpandToHostedMCP(tokens []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, tok := range FilterKnownTools(tokens) {
		names, ok := harnessToMCP[tok]
		if !ok {
			continue
		}
		for _, name := range names {
			if _, ok := HostedLoopV1Catalog[name]; !ok {
				continue
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	return out
}

// HostedToolClass returns read, write, or empty if name is not in the hosted v1 catalog.
func HostedToolClass(name string) string {
	return HostedLoopV1Catalog[strings.TrimSpace(name)]
}

// HostedToolAdmitted reports whether name is in the admitted MCP allowlist.
func HostedToolAdmitted(name string, admitted []string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, a := range admitted {
		if a == name {
			return true
		}
	}
	return false
}
