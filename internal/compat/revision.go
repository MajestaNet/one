package compat

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// APIRevisionWindow is the install-supported revision range advertised to clients.
// Recommended is always an alias of Current (builders may pin either).
type APIRevisionWindow struct {
	Min     int `json:"min"`
	Current int `json:"current"`
}

// MarshalJSON always emits recommended as a copy of current so discovery
// surfaces stay in lockstep even when callers construct the window by hand.
func (w APIRevisionWindow) MarshalJSON() ([]byte, error) {
	type wire struct {
		Min         int `json:"min"`
		Current     int `json:"current"`
		Recommended int `json:"recommended"`
	}
	return json.Marshal(wire{Min: w.Min, Current: w.Current, Recommended: w.Current})
}

// HTTPAPIFamilies maps ADR-004 family majors for discovery surfaces.
type HTTPAPIFamilies struct {
	Client   string `json:"client"`
	Metadata string `json:"metadata"`
	Deploy   string `json:"deploy"`
	Ops      string `json:"ops"`
	Auth     string `json:"auth"`
}

// DefaultHTTPAPI returns the v1 family map used by current Majesta One images.
func DefaultHTTPAPI() HTTPAPIFamilies {
	return HTTPAPIFamilies{
		Client:   "v1",
		Metadata: "v1",
		Deploy:   "v1",
		Ops:      "v1",
		Auth:     "v1",
	}
}

// NormalizeWindow validates min/current and returns a normalized window.
func NormalizeWindow(min, current int) (APIRevisionWindow, error) {
	if current < 1 {
		return APIRevisionWindow{}, fmt.Errorf("api revision current must be >= 1")
	}
	if min < 1 {
		return APIRevisionWindow{}, fmt.Errorf("api revision min must be >= 1")
	}
	if min > current {
		return APIRevisionWindow{}, fmt.Errorf("api revision min (%d) cannot exceed current (%d)", min, current)
	}
	return APIRevisionWindow{Min: min, Current: current}, nil
}

// PinInWindow reports whether pin is within [min, current].
func PinInWindow(pin int, window APIRevisionWindow) bool {
	return pin >= window.Min && pin <= window.Current
}

// ParseRevisionHeader parses One-API-Revision. Returns (pin, explicit, err).
// explicit is false when the header is absent; err is set when present but unparsable.
func ParseRevisionHeader(raw string) (int, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, true, fmt.Errorf("unparsable API revision %q", raw)
	}
	return n, true, nil
}

// ResolvePin chooses the effective pin: explicit header or default current.
func ResolvePin(header string, window APIRevisionWindow) (int, bool, error) {
	return ResolveRequestPin(header, 0, false, window)
}

// FamilyPathPrefixes are ADR-004 family majors plus MCP and the flat /v1 alias.
// Longest prefixes must come first so /client/v1 wins over /v1.
var FamilyPathPrefixes = []string{
	"/client/v1",
	"/metadata/v1",
	"/deploy/v1",
	"/ops/v1",
	"/auth/v1",
	"/mcp",
	"/v1",
}

// SplitRevisionPath extracts an optional /r{N}/ segment under a family prefix.
// Example: /client/v1/r12/sobjects/Account → rewritten /client/v1/sobjects/Account, pin 12.
func SplitRevisionPath(path string) (rewritten string, pin int, found bool) {
	for _, prefix := range FamilyPathPrefixes {
		if path != prefix && !strings.HasPrefix(path, prefix+"/") {
			continue
		}
		rest := strings.TrimPrefix(path, prefix)
		if !strings.HasPrefix(rest, "/r") {
			return path, 0, false
		}
		rest = strings.TrimPrefix(rest, "/r")
		i := 0
		for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
			i++
		}
		if i == 0 {
			return path, 0, false
		}
		if i < len(rest) && rest[i] != '/' {
			return path, 0, false
		}
		n, err := strconv.Atoi(rest[:i])
		if err != nil || n < 1 {
			return path, 0, false
		}
		remainder := rest[i:]
		if remainder == "" || remainder == "/" {
			return prefix, n, true
		}
		return prefix + remainder, n, true
	}
	return path, 0, false
}

// PathRequiresRevision reports whether the path is a family (or MCP) surface
// that participates in API revision pinning. /version, /healthz, /readyz, and
// /scim/v2 are intentionally excluded.
func PathRequiresRevision(path string) bool {
	rewritten, _, _ := SplitRevisionPath(path)
	check := rewritten
	if check == "" {
		check = path
	}
	for _, prefix := range FamilyPathPrefixes {
		if check == prefix || strings.HasPrefix(check, prefix+"/") {
			return true
		}
	}
	return false
}

// ResolveRequestPin chooses the pin: One-API-Revision header, else /r{N}/, else current.
func ResolveRequestPin(header string, pathPin int, pathPinFound bool, window APIRevisionWindow) (int, bool, error) {
	pin, explicit, err := ParseRevisionHeader(header)
	if err != nil {
		return 0, explicit, err
	}
	if explicit {
		return pin, true, nil
	}
	if pathPinFound {
		return pathPin, true, nil
	}
	return window.Current, false, nil
}

// UnsupportedCTA is the machine-stable operator hint for API_REVISION_UNSUPPORTED.
func UnsupportedCTA(pin int, window APIRevisionWindow) string {
	if pin > 0 && pin < window.Min {
		return "Migrate the client API revision pin or update Control IDE"
	}
	if pin > window.Current {
		return "Upgrade the install product image (/ops/v1) or lower the client pin"
	}
	return "Fix API_REVISION_CURRENT / API_REVISION_MIN on the install"
}
