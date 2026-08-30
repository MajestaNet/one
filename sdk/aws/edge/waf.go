package edge

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	"github.com/aws/aws-sdk-go-v2/service/wafv2/types"
)

// WAFConfig configures AWS WAFv2 reconciliation.
type WAFConfig struct {
	WebACLName      string
	WebACLID        string
	IPSetPrefix     string            // name prefix; IDs looked up via IPSetIDs
	IPSetIDs        map[string]string // family -> IP set id
	Scope           string            // REGIONAL (default) or CLOUDFRONT
	Region          string
	BreakglassCIDRs []string
}

// Configured reports whether AWS WAF reconcile is possible.
func (c WAFConfig) Configured() bool {
	return c.WebACLName != "" && c.WebACLID != "" && len(c.IPSetIDs) > 0
}

// WAFAPI is the subset of WAFv2 used by AWSWAFRoller (mockable).
type WAFAPI interface {
	GetIPSet(ctx context.Context, params *wafv2.GetIPSetInput, optFns ...func(*wafv2.Options)) (*wafv2.GetIPSetOutput, error)
	UpdateIPSet(ctx context.Context, params *wafv2.UpdateIPSetInput, optFns ...func(*wafv2.Options)) (*wafv2.UpdateIPSetOutput, error)
	GetWebACL(ctx context.Context, params *wafv2.GetWebACLInput, optFns ...func(*wafv2.Options)) (*wafv2.GetWebACLOutput, error)
	UpdateWebACL(ctx context.Context, params *wafv2.UpdateWebACLInput, optFns ...func(*wafv2.Options)) (*wafv2.UpdateWebACLOutput, error)
	ListIPSets(ctx context.Context, params *wafv2.ListIPSetsInput, optFns ...func(*wafv2.Options)) (*wafv2.ListIPSetsOutput, error)
}

// AWSWAFRoller reconciles Policy into WAFv2 IP sets + WebACL path rules.
type AWSWAFRoller struct {
	Config WAFConfig
	Client WAFAPI
	mu     sync.Mutex
}

func (r *AWSWAFRoller) Mode() string { return "aws" }

// NewAWSWAFRoller returns a WAFv2 roller for the given config.
// Callers should check Configured before use; MemoryRoller stays in the product tree.
func NewAWSWAFRoller(c WAFConfig) *AWSWAFRoller {
	return &AWSWAFRoller{Config: c}
}

func (r *AWSWAFRoller) client(ctx context.Context) (WAFAPI, error) {
	if r.Client != nil {
		return r.Client, nil
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(r.Config.Region))
	if err != nil {
		return nil, err
	}
	return wafv2.NewFromConfig(cfg), nil
}

func (r *AWSWAFRoller) scope() types.Scope {
	if strings.EqualFold(r.Config.Scope, "CLOUDFRONT") {
		return types.ScopeCloudfront
	}
	return types.ScopeRegional
}

// Apply updates per-family IP sets and rebuilds WebACL rules for path exposure.
func (r *AWSWAFRoller) Apply(ctx context.Context, p Policy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cli, err := r.client(ctx)
	if err != nil {
		return err
	}
	scope := r.scope()
	prefix := r.Config.IPSetPrefix
	if prefix == "" {
		prefix = "one-exp-"
	}

	ipARNs := map[Family]string{}
	for _, f := range AllFamilies {
		fp := p.Get(f)
		cidrs := append([]string{}, fp.CIDRs...)
		if len(r.Config.BreakglassCIDRs) > 0 && (fp.Mode == ModeAllowlist || f == FamilyMetadata) {
			cidrs = append(cidrs, r.Config.BreakglassCIDRs...)
		}
		name := prefix + string(f)
		id := r.Config.IPSetIDs[string(f)]
		if id == "" {
			return fmt.Errorf("missing IP set id for family %s", f)
		}
		arn, err := r.updateIPSet(ctx, cli, scope, name, id, cidrs)
		if err != nil {
			return fmt.Errorf("ipset %s: %w", name, err)
		}
		ipARNs[f] = arn
	}
	return r.updateWebACL(ctx, cli, scope, p, ipARNs)
}

func (r *AWSWAFRoller) updateIPSet(ctx context.Context, cli WAFAPI, scope types.Scope, name, id string, cidrs []string) (string, error) {
	out, err := cli.GetIPSet(ctx, &wafv2.GetIPSetInput{
		Name:  aws.String(name),
		Id:    aws.String(id),
		Scope: scope,
	})
	if err != nil {
		return "", err
	}
	addrs := make([]string, 0, len(cidrs))
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c != "" {
			addrs = append(addrs, c)
		}
	}
	// WAFv2 requires ≥1 address; TEST-NET placeholder when empty (unused for public/blocked rules).
	if len(addrs) == 0 {
		addrs = []string{"192.0.2.0/32"}
	}
	_, err = cli.UpdateIPSet(ctx, &wafv2.UpdateIPSetInput{
		Name:        out.IPSet.Name,
		Id:          out.IPSet.Id,
		Scope:       scope,
		LockToken:   out.LockToken,
		Addresses:   addrs,
		Description: out.IPSet.Description,
	})
	if err != nil {
		return "", err
	}
	if out.IPSet.ARN != nil {
		return *out.IPSet.ARN, nil
	}
	return "", nil
}

func (r *AWSWAFRoller) updateWebACL(ctx context.Context, cli WAFAPI, scope types.Scope, p Policy, ipARNs map[Family]string) error {
	acl, err := cli.GetWebACL(ctx, &wafv2.GetWebACLInput{
		Name:  aws.String(r.Config.WebACLName),
		Id:    aws.String(r.Config.WebACLID),
		Scope: scope,
	})
	if err != nil {
		return err
	}

	rules := []types.Rule{
		{
			Name:     aws.String("one-allow-health"),
			Priority: 0,
			Action:   &types.RuleAction{Allow: &types.AllowAction{}},
			Statement: &types.Statement{
				OrStatement: &types.OrStatement{
					Statements: []types.Statement{
						{ByteMatchStatement: pathStartsWith("/healthz")},
						{ByteMatchStatement: pathStartsWith("/readyz")},
					},
				},
			},
			VisibilityConfig: &types.VisibilityConfig{
				SampledRequestsEnabled:   true,
				CloudWatchMetricsEnabled: true,
				MetricName:               aws.String("oneAllowHealth"),
			},
		},
	}

	priority := int32(10)
	for _, f := range AllFamilies {
		fp := p.Get(f)
		ruleName := "one-exp-" + string(f)
		metric := "oneExp" + string(f)
		pathStmt := familyPathStatement(f)

		switch fp.Mode {
		case ModePublic:
			rules = append(rules, types.Rule{
				Name:      aws.String(ruleName),
				Priority:  priority,
				Action:    &types.RuleAction{Allow: &types.AllowAction{}},
				Statement: pathStmt,
				VisibilityConfig: &types.VisibilityConfig{
					SampledRequestsEnabled:   true,
					CloudWatchMetricsEnabled: true,
					MetricName:               aws.String(metric),
				},
			})
			priority++
		case ModeBlocked:
			rules = append(rules, types.Rule{
				Name:      aws.String(ruleName),
				Priority:  priority,
				Action:    &types.RuleAction{Block: &types.BlockAction{}},
				Statement: pathStmt,
				VisibilityConfig: &types.VisibilityConfig{
					SampledRequestsEnabled:   true,
					CloudWatchMetricsEnabled: true,
					MetricName:               aws.String(metric),
				},
			})
			priority++
		case ModeAllowlist:
			arn := ipARNs[f]
			rules = append(rules, types.Rule{
				Name:     aws.String(ruleName),
				Priority: priority,
				Action:   &types.RuleAction{Allow: &types.AllowAction{}},
				Statement: &types.Statement{
					AndStatement: &types.AndStatement{
						Statements: []types.Statement{
							*pathStmt,
							{
								IPSetReferenceStatement: &types.IPSetReferenceStatement{
									ARN: aws.String(arn),
								},
							},
						},
					},
				},
				VisibilityConfig: &types.VisibilityConfig{
					SampledRequestsEnabled:   true,
					CloudWatchMetricsEnabled: true,
					MetricName:               aws.String(metric),
				},
			})
			priority++
			rules = append(rules, types.Rule{
				Name:      aws.String(ruleName + "-block"),
				Priority:  priority,
				Action:    &types.RuleAction{Block: &types.BlockAction{}},
				Statement: pathStmt,
				VisibilityConfig: &types.VisibilityConfig{
					SampledRequestsEnabled:   true,
					CloudWatchMetricsEnabled: true,
					MetricName:               aws.String(metric + "Block"),
				},
			})
			priority++
		}
	}

	_, err = cli.UpdateWebACL(ctx, &wafv2.UpdateWebACLInput{
		Name:             acl.WebACL.Name,
		Id:               acl.WebACL.Id,
		Scope:            scope,
		LockToken:        acl.LockToken,
		DefaultAction:    acl.WebACL.DefaultAction,
		Description:      acl.WebACL.Description,
		Rules:            rules,
		VisibilityConfig: acl.WebACL.VisibilityConfig,
	})
	return err
}

func pathStartsWith(prefix string) *types.ByteMatchStatement {
	return &types.ByteMatchStatement{
		SearchString:         []byte(prefix),
		FieldToMatch:         &types.FieldToMatch{UriPath: &types.UriPath{}},
		TextTransformations:  []types.TextTransformation{{Priority: 0, Type: types.TextTransformationTypeNone}},
		PositionalConstraint: types.PositionalConstraintStartsWith,
	}
}

func familyPathStatement(f Family) *types.Statement {
	prefs := PathPrefixes(f)
	if len(prefs) == 1 {
		return &types.Statement{ByteMatchStatement: pathStartsWith(prefs[0])}
	}
	stmts := make([]types.Statement, 0, len(prefs))
	for _, p := range prefs {
		stmts = append(stmts, types.Statement{ByteMatchStatement: pathStartsWith(p)})
	}
	return &types.Statement{OrStatement: &types.OrStatement{Statements: stmts}}
}
