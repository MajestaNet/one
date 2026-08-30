package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/compat"
	"github.com/MajestaNet/ide/internal/config"
)

const (
	laneClient   = "client"
	laneMetadata = "metadata"
	laneDeploy   = "deploy"
	laneAuth     = "auth"
	laneOps      = "ops"
	mcpPeekLimit = 64 << 10
)

type admissionLimiters struct {
	client    *rateLimiter
	remainder *rateLimiter
}

func newAdmissionLimiters(cfg *config.Config) *admissionLimiters {
	if cfg == nil || cfg.RateLimitPerMinute <= 0 {
		return nil
	}
	client, remainder := config.AdmissionLaneLimits(cfg.RateLimitPerMinute, cfg.AdmissionClientRPMShare)
	return &admissionLimiters{
		client:    newRateLimiter(client),
		remainder: newRemainderLimiter(remainder),
	}
}

// newRemainderLimiter treats 0 as deny-all (Client took the whole RPM budget).
func newRemainderLimiter(perMinute int) *rateLimiter {
	if perMinute <= 0 {
		return &rateLimiter{limit: 0, window: time.Minute, hits: map[string][]time.Time{}}
	}
	return newRateLimiter(perMinute)
}

func (a *admissionLimiters) allow(lane, key string) bool {
	if a == nil {
		return true
	}
	switch lane {
	case laneClient:
		return a.client.allow(key)
	case laneMetadata, laneDeploy, laneOps:
		return a.remainder.allow(key)
	case laneAuth, "":
		return true
	default:
		return a.remainder.allow(key)
	}
}

func classifyAdmissionPath(path string) string {
	rewritten, _, _ := compat.SplitRevisionPath(path)
	if rewritten != "" {
		path = rewritten
	}
	if bootProbePath(path) {
		return ""
	}
	switch {
	case strings.HasPrefix(path, "/client/v1"):
		return laneClient
	case strings.HasPrefix(path, "/metadata/v1"):
		return laneMetadata
	case strings.HasPrefix(path, "/deploy/v1"):
		return laneDeploy
	case strings.HasPrefix(path, "/ops/v1"):
		return laneOps
	case strings.HasPrefix(path, "/auth/v1"):
		return laneAuth
	case path == "/mcp" || strings.HasPrefix(path, "/mcp/"):
		return laneDeploy // body may override
	case strings.HasPrefix(path, "/scim/"):
		return laneClient
	case strings.HasPrefix(path, "/v1/") || path == "/v1":
		return v1AliasLane(path)
	default:
		return laneClient
	}
}

func v1AliasLane(path string) string {
	rest := strings.TrimPrefix(path, "/v1")
	rest = strings.TrimPrefix(rest, "/")
	first, next, _ := strings.Cut(rest, "/")
	switch first {
	case "objects", "fields", "field-types", "validation-rules", "snapshot",
		"automations", "permissions", "packages", "webhooks", "projections",
		"metadata", "connectors", "install", "sharing", "inference":
		return laneMetadata
	case "agents":
		if strings.HasPrefix(next, "playbooks") || strings.HasPrefix(next, "harnesses") {
			return laneMetadata
		}
		return laneClient
	default:
		return laneClient
	}
}

func peekMCPLane(r *http.Request) (string, *http.Request) {
	if r.Body == nil || r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodDelete {
		return laneDeploy, r
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, mcpPeekLimit))
	r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(raw), r.Body))
	if err != nil || len(bytes.TrimSpace(raw)) == 0 {
		return laneDeploy, r
	}
	return mcpToolLane(raw), r
}

func mcpToolLane(raw []byte) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return laneDeploy
	}
	if raw[0] == '[' {
		var msgs []json.RawMessage
		if err := json.Unmarshal(raw, &msgs); err != nil {
			return laneDeploy
		}
		lane := laneClient
		for _, msg := range msgs {
			next := mcpToolLane(msg)
			lane = moreConservativeLane(lane, next)
		}
		return lane
	}
	var envelope struct {
		Method string `json:"method"`
		Params struct {
			Name string `json:"name"`
		} `json:"params"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return laneDeploy
	}
	if envelope.Method != "tools/call" || envelope.Params.Name == "" {
		return laneDeploy
	}
	return mcpNamedToolLane(envelope.Params.Name)
}

func moreConservativeLane(a, b string) string {
	rank := func(lane string) int {
		switch lane {
		case laneDeploy:
			return 3
		case laneMetadata, laneOps:
			return 2
		default:
			return 1
		}
	}
	if rank(b) > rank(a) {
		return b
	}
	return a
}

func mcpNamedToolLane(name string) string {
	switch name {
	case "org_validate", "org_deploy", "pack", "org_retrieve":
		return laneDeploy
	case "upsert_object", "upsert_field", "list_objects_metadata", "get_object_metadata":
		return laneMetadata
	default:
		return laneClient
	}
}
