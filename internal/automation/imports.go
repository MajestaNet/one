// Package automation holds guest TypeScript automation helpers (ADR-014).
// Import ban + validation (Phase 2); Deno guest executor (Phase 4).
package automation

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	RuntimeActions = "actions"
	RuntimeCode    = "code"
	ExecutionAsync = "async"
	ExecutionSync  = "sync"

	// AllowedModule is the only import specifier permitted in guest TS (types or values).
	AllowedModule = "one:automation"
)

// Outbound action types forbidden when execution=sync (non-rollbackable I/O).
var outboundActionTypes = map[string]struct{}{
	"http":      {},
	"httpget":   {},
	"httppost":  {},
	"webhook":   {},
	"connector": {},
	"email":     {},
	"outbound":  {},
	"fetch":     {},
}

var (
	// from "…" / from '…' (import/export)
	reFromSpec = regexp.MustCompile(`\bfrom\s+["']([^"']+)["']`)
	// side-effect import "…"
	reImportSide = regexp.MustCompile(`(?m)^\s*import\s*["']([^"']+)["']`)
	// require("…")
	reRequire = regexp.MustCompile(`\brequire\s*\(\s*["']([^"']+)["']\s*\)`)
	// dynamic import("…")
	reDynImport = regexp.MustCompile(`\bimport\s*\(\s*["']([^"']+)["']\s*\)`)
	// eval / Function constructor
	reEval = regexp.MustCompile(`\beval\s*\(|\bnew\s+Function\s*\(`)
)

// ValidateSourceImports rejects third-party / remote / std imports per ADR-014 v1 ban.
// Allowed: no imports, or only one:automation (including type-only).
func ValidateSourceImports(path, source string) error {
	if source == "" {
		return nil
	}
	stripped := stripTSComments(source)
	if reEval.MatchString(stripped) {
		return fmt.Errorf("%s: eval / Function constructor is forbidden", label(path))
	}
	var specs []string
	for _, re := range []*regexp.Regexp{reFromSpec, reImportSide, reRequire, reDynImport} {
		for _, m := range re.FindAllStringSubmatch(stripped, -1) {
			if len(m) > 1 {
				specs = append(specs, m[1])
			}
		}
	}
	for _, spec := range specs {
		if !allowedImportSpec(spec) {
			return fmt.Errorf("%s: forbidden import %q (only %q is allowed; no npm/jsr/std/url/relative deps)",
				label(path), spec, AllowedModule)
		}
	}
	return nil
}

func allowedImportSpec(spec string) bool {
	return spec == AllowedModule
}

func label(path string) string {
	if path == "" {
		return "automation source"
	}
	return path
}

// stripTSComments removes // and /* */ comments so import scans ignore commented code.
// String literals are preserved so URLs like https://… are not treated as // comments.
func stripTSComments(src string) string {
	var b strings.Builder
	b.Grow(len(src))
	inLine, inBlock := false, false
	var strDelim byte // 0 | '\'' | '"' | '`'
	escaped := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inLine {
			if c == '\n' {
				inLine = false
				b.WriteByte('\n')
			}
			continue
		}
		if inBlock {
			if c == '*' && i+1 < len(src) && src[i+1] == '/' {
				inBlock = false
				i++
			}
			continue
		}
		if strDelim != 0 {
			b.WriteByte(c)
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' && strDelim != '`' {
				escaped = true
				continue
			}
			if c == strDelim {
				strDelim = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			strDelim = c
			b.WriteByte(c)
			continue
		case '/':
			if i+1 < len(src) {
				switch src[i+1] {
				case '/':
					inLine = true
					i++
					continue
				case '*':
					inBlock = true
					i++
					continue
				}
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

// NormalizeRuntime returns actions|code (default actions).
func NormalizeRuntime(r string) string {
	switch strings.ToLower(strings.TrimSpace(r)) {
	case RuntimeCode:
		return RuntimeCode
	default:
		return RuntimeActions
	}
}

// NormalizeExecution returns async|sync (default async).
func NormalizeExecution(e string) string {
	switch strings.ToLower(strings.TrimSpace(e)) {
	case ExecutionSync:
		return ExecutionSync
	default:
		return ExecutionAsync
	}
}

// ActionType extracts a lowercase type from an automation action element.
func ActionType(a any) string {
	switch v := a.(type) {
	case string:
		return strings.ToLower(strings.TrimSpace(v))
	case map[string]any:
		if t, ok := v["type"].(string); ok {
			return strings.ToLower(strings.TrimSpace(t))
		}
		if t, ok := v["action"].(string); ok {
			return strings.ToLower(strings.TrimSpace(t))
		}
	}
	return ""
}

// HasOutboundAction reports whether actions include a non-rollbackable outbound type.
func HasOutboundAction(actions []any) (string, bool) {
	for _, a := range actions {
		t := ActionType(a)
		if t == "" {
			continue
		}
		if _, ok := outboundActionTypes[t]; ok {
			return t, true
		}
	}
	return "", false
}

// ValidateDefinition checks Phase 2 rules for one automation definition.
func ValidateDefinition(
	apiName, runtime, execution, triggerEvent string,
	entryFile, source, runAsPrincipalID *string,
	actions []any,
) error {
	rt := NormalizeRuntime(runtime)
	ex := NormalizeExecution(execution)
	trigger := strings.ToLower(strings.TrimSpace(triggerEvent))

	if ex == ExecutionSync {
		if t, ok := HasOutboundAction(actions); ok {
			return fmt.Errorf("automation %s: execution=sync forbids outbound action type %q", apiName, t)
		}
	}
	if trigger == "schedule" {
		if runAsPrincipalID == nil || strings.TrimSpace(*runAsPrincipalID) == "" {
			return fmt.Errorf("automation %s: schedule trigger requires runAsPrincipalId", apiName)
		}
	}
	if rt == RuntimeCode {
		ef := ""
		if entryFile != nil {
			ef = strings.TrimSpace(*entryFile)
		}
		src := ""
		if source != nil {
			src = *source
		}
		if ef == "" && src == "" {
			return fmt.Errorf("automation %s: runtime=code requires entryFile and/or source", apiName)
		}
		if ef != "" && !strings.HasPrefix(ef, "src/automations/") {
			return fmt.Errorf("automation %s: entryFile must be under src/automations/ (got %q)", apiName, ef)
		}
		path := ef
		if path == "" {
			path = apiName
		}
		if err := ValidateSourceImports(path, src); err != nil {
			return err
		}
	}
	return nil
}
