package edge

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
)

// Family is an API path family controlled by exposure policy.
type Family string

const (
	FamilyClient   Family = "client"
	FamilyAuth     Family = "auth"
	FamilyMetadata Family = "metadata"
	FamilyDeploy   Family = "deploy"
	FamilyOps      Family = "ops"
)

// AllFamilies is the ordered set of families in the policy document.
var AllFamilies = []Family{FamilyClient, FamilyAuth, FamilyMetadata, FamilyDeploy, FamilyOps}

// Mode controls how a family path prefix is exposed at the edge.
type Mode string

const (
	ModePublic    Mode = "public"
	ModeAllowlist Mode = "allowlist"
	ModeBlocked   Mode = "blocked"
)

// ClientAccessMode controls which Connected Apps / credentials may mint Client tokens.
type ClientAccessMode string

const (
	// ClientAccessOpen — any valid principal credential (historical default).
	ClientAccessOpen ClientAccessMode = "open"
	// ClientAccessRegistered — enabled Connected Apps / Majesta One credentials (azp required on bearer).
	ClientAccessRegistered ClientAccessMode = "registered_clients"
	// ClientAccessIDEUsers is a stored-row leftover. Validate rejects new writes; EffectiveClientAccessMode maps it to open.
	ClientAccessIDEUsers ClientAccessMode = "ide_users"
)

var deprecatedIDEUsersOnce sync.Once

// FamilyPolicy is the desired exposure for one path family.
type FamilyPolicy struct {
	Mode  Mode     `json:"mode"`
	CIDRs []string `json:"cidrs"`
}

// Policy is the install exposure desired state.
type Policy struct {
	ClientAccessMode  ClientAccessMode `json:"clientAccessMode,omitempty"`
	RequireDeviceCert bool             `json:"requireDeviceCert,omitempty"`
	Client            FamilyPolicy     `json:"client"`
	Auth              FamilyPolicy     `json:"auth"`
	Metadata          FamilyPolicy     `json:"metadata"`
	Deploy            FamilyPolicy     `json:"deploy"`
	Ops               FamilyPolicy     `json:"ops"`
}

// Get returns the family policy by name.
func (p Policy) Get(f Family) FamilyPolicy {
	switch f {
	case FamilyClient:
		return p.Client
	case FamilyAuth:
		return p.Auth
	case FamilyMetadata:
		return p.Metadata
	case FamilyDeploy:
		return p.Deploy
	case FamilyOps:
		return p.Ops
	default:
		return FamilyPolicy{Mode: ModeBlocked}
	}
}

// Set assigns a family policy by name.
func (p *Policy) Set(f Family, fp FamilyPolicy) {
	switch f {
	case FamilyClient:
		p.Client = fp
	case FamilyAuth:
		p.Auth = fp
	case FamilyMetadata:
		p.Metadata = fp
	case FamilyDeploy:
		p.Deploy = fp
	case FamilyOps:
		p.Ops = fp
	}
}

// EffectiveClientAccessMode returns the access mode (default open).
// Stored clientAccessMode=ide_users is treated as open (BP-065); new writes are rejected by Validate.
func (p Policy) EffectiveClientAccessMode() ClientAccessMode {
	switch p.ClientAccessMode {
	case ClientAccessIDEUsers:
		deprecatedIDEUsersOnce.Do(func() {
			slog.Warn("clientAccessMode=ide_users is no longer supported; treating stored value as open")
		})
		return ClientAccessOpen
	case ClientAccessRegistered, ClientAccessOpen:
		return p.ClientAccessMode
	default:
		return ClientAccessOpen
	}
}

// DefaultPolicy is the secure-by-default install exposure:
// Client + Auth public (API consumers + token mint); Metadata / Deploy / Ops blocked
// until an admin explicitly allowlists CIDRs via /metadata/v1/install/exposure.
// ClientAccessMode defaults to open (lock down via clientAccessMode + optional Client allowlist).
func DefaultPolicy() Policy {
	pub := FamilyPolicy{Mode: ModePublic, CIDRs: []string{}}
	blocked := FamilyPolicy{Mode: ModeBlocked, CIDRs: []string{}}
	return Policy{
		ClientAccessMode:  ClientAccessOpen,
		RequireDeviceCert: false,
		Client:            pub,
		Auth:              pub,
		Metadata:          blocked,
		Deploy:            blocked,
		Ops:               blocked,
	}
}

// PathPrefixes maps families to URL path prefixes (legacy /v1 follows Client).
func PathPrefixes(f Family) []string {
	switch f {
	case FamilyClient:
		return []string{"/client/", "/v1/"}
	case FamilyAuth:
		return []string{"/auth/"}
	case FamilyMetadata:
		return []string{"/metadata/"}
	case FamilyDeploy:
		return []string{"/deploy/"}
	case FamilyOps:
		return []string{"/ops/"}
	default:
		return nil
	}
}

// Validate checks modes, CIDRs, access mode, and safety rails.
func Validate(p Policy) error {
	switch p.ClientAccessMode {
	case "", ClientAccessOpen, ClientAccessRegistered:
	case ClientAccessIDEUsers:
		return fmt.Errorf("clientAccessMode %q is no longer supported; use open or registered_clients", p.ClientAccessMode)
	default:
		return fmt.Errorf("invalid clientAccessMode %q", p.ClientAccessMode)
	}
	for _, f := range AllFamilies {
		fp := p.Get(f)
		switch fp.Mode {
		case ModePublic, ModeAllowlist, ModeBlocked:
		default:
			return fmt.Errorf("invalid mode %q for %s", fp.Mode, f)
		}
		for _, c := range fp.CIDRs {
			if _, _, err := net.ParseCIDR(strings.TrimSpace(c)); err != nil {
				return fmt.Errorf("invalid cidr %q for %s: %w", c, f, err)
			}
		}
		if fp.Mode == ModeAllowlist && len(fp.CIDRs) == 0 {
			return fmt.Errorf("%s: allowlist mode requires at least one cidr", f)
		}
	}
	// Refuse Client public while Auth is blocked — integrations cannot mint tokens.
	if p.Client.Mode == ModePublic && p.Auth.Mode == ModeBlocked {
		return fmt.Errorf("auth cannot be blocked while client is public")
	}
	// Control-plane families must not be world-public; use allowlist or blocked.
	for _, f := range []Family{FamilyMetadata, FamilyDeploy, FamilyOps} {
		if p.Get(f).Mode == ModePublic {
			return fmt.Errorf("%s cannot be public; use allowlist or blocked", f)
		}
	}
	return nil
}

// MergeCIDRs returns a de-duplicated union of CIDR strings.
func MergeCIDRs(base []string, extra ...[]string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(list []string) {
		for _, c := range list {
			c = strings.TrimSpace(c)
			if c == "" {
				continue
			}
			if _, ok := seen[c]; ok {
				continue
			}
			seen[c] = struct{}{}
			out = append(out, c)
		}
	}
	add(base)
	for _, e := range extra {
		add(e)
	}
	return out
}

// Status values for install_exposure_policy.status.
const (
	StatusPending = "pending"
	StatusApplied = "applied"
	StatusError   = "error"
)

// Roller applies a Policy to the edge (WAF or memory).
type Roller interface {
	Mode() string
	Apply(ctx context.Context, p Policy) error
}

// MemoryRoller records the last applied policy (Compose / unit tests).
type MemoryRoller struct {
	Last Policy
}

func (m *MemoryRoller) Mode() string { return "local" }

func (m *MemoryRoller) Apply(_ context.Context, p Policy) error {
	m.Last = p
	return nil
}
