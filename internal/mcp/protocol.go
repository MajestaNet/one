// Package mcp is an install-local MCP adapter over Majesta One HTTP families (ADR-010).
// Protocol helpers for Streamable HTTP (stateless JSON) live here; tool AuthZ stays in gateway.go.
package mcp

import (
	"net"
	"strings"
)

// LatestProtocolVersion is the preferred MCP protocol version this gateway speaks.
const LatestProtocolVersion = "2025-03-26"

// SupportedProtocolVersions lists protocol versions we will negotiate.
var SupportedProtocolVersions = []string{
	"2025-03-26",
	"2024-11-05",
}

// ServerName / ServerVersion identify this MCP implementation to clients.
const (
	ServerName    = "one"
	ServerVersion = "1.0.0"
)

// NegotiateProtocolVersion returns a version we support. If the client asked for a
// supported version, echo it; otherwise return LatestProtocolVersion.
func NegotiateProtocolVersion(requested string) string {
	requested = strings.TrimSpace(requested)
	for _, v := range SupportedProtocolVersions {
		if requested == v {
			return v
		}
	}
	return LatestProtocolVersion
}

// InitializeResult is the MCP initialize response payload.
func InitializeResult(protocolVersion string) map[string]any {
	return map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    ServerName,
			"version": ServerVersion,
		},
		"instructions": "Majesta One MCP gateway projects Client/Metadata/Deploy builder tools under the caller's AuthZ. " +
			"Enable FEATURE_FLAGS=agents. See docs/customer-connect.md.",
	}
}

// IsNotification reports whether a JSON-RPC method is a notification (no response id).
func IsNotification(method string) bool {
	return strings.HasPrefix(method, "notifications/")
}

// IsLoopbackHost reports whether host (with or without port) is a loopback address.
func IsLoopbackHost(host string) bool {
	h := strings.TrimSpace(host)
	if h == "" {
		return false
	}
	if hostname, _, err := net.SplitHostPort(h); err == nil {
		h = hostname
	}
	h = strings.ToLower(strings.Trim(h, "[]"))
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}
