package edge

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

// FamilyPolicy is the desired exposure for one path family.
type FamilyPolicy struct {
	Mode  Mode     `json:"mode"`
	CIDRs []string `json:"cidrs"`
}

// Policy is the install exposure desired state used by AWSWAFRoller.Apply.
type Policy struct {
	Client   FamilyPolicy `json:"client"`
	Auth     FamilyPolicy `json:"auth"`
	Metadata FamilyPolicy `json:"metadata"`
	Deploy   FamilyPolicy `json:"deploy"`
	Ops      FamilyPolicy `json:"ops"`
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
