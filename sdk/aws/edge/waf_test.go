package edge_test

import (
	"context"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	"github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	"github.com/MajestaNet/ide/sdk/aws/edge"
)

type mockWAF struct {
	mu             sync.Mutex
	ipSets         map[string]*types.IPSet
	ipLock         map[string]string
	webACL         *types.WebACL
	webLock        string
	updateIPCalls  int
	updateACLCalls int
}

func newMockWAF() *mockWAF {
	m := &mockWAF{
		ipSets:  map[string]*types.IPSet{},
		ipLock:  map[string]string{},
		webLock: "web-lock",
	}
	for _, f := range edge.AllFamilies {
		name := "one-exp-" + string(f)
		id := "id-" + string(f)
		arn := "arn:aws:wafv2:us-east-1:123456789012:regional/ipset/" + name + "/" + id
		m.ipSets[name] = &types.IPSet{
			Name:             aws.String(name),
			Id:               aws.String(id),
			ARN:              aws.String(arn),
			IPAddressVersion: types.IPAddressVersionIpv4,
			Addresses:        []string{"192.0.2.0/32"},
		}
		m.ipLock[name] = "lock-" + string(f)
	}
	m.webACL = &types.WebACL{
		Name: aws.String("one-acl"),
		Id:   aws.String("acl-id"),
		DefaultAction: &types.DefaultAction{
			Block: &types.BlockAction{},
		},
		VisibilityConfig: &types.VisibilityConfig{
			SampledRequestsEnabled:   true,
			CloudWatchMetricsEnabled: true,
			MetricName:               aws.String("oneAcl"),
		},
	}
	return m
}

func (m *mockWAF) GetIPSet(_ context.Context, in *wafv2.GetIPSetInput, _ ...func(*wafv2.Options)) (*wafv2.GetIPSetOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ip := m.ipSets[*in.Name]
	return &wafv2.GetIPSetOutput{IPSet: ip, LockToken: aws.String(m.ipLock[*in.Name])}, nil
}

func (m *mockWAF) UpdateIPSet(_ context.Context, in *wafv2.UpdateIPSetInput, _ ...func(*wafv2.Options)) (*wafv2.UpdateIPSetOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateIPCalls++
	m.ipSets[*in.Name].Addresses = in.Addresses
	m.ipLock[*in.Name] = "lock2"
	return &wafv2.UpdateIPSetOutput{NextLockToken: aws.String("lock2")}, nil
}

func (m *mockWAF) GetWebACL(_ context.Context, _ *wafv2.GetWebACLInput, _ ...func(*wafv2.Options)) (*wafv2.GetWebACLOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &wafv2.GetWebACLOutput{WebACL: m.webACL, LockToken: aws.String(m.webLock)}, nil
}

func (m *mockWAF) UpdateWebACL(_ context.Context, in *wafv2.UpdateWebACLInput, _ ...func(*wafv2.Options)) (*wafv2.UpdateWebACLOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateACLCalls++
	m.webACL.Rules = in.Rules
	m.webLock = "web-lock2"
	return &wafv2.UpdateWebACLOutput{NextLockToken: aws.String("web-lock2")}, nil
}

func (m *mockWAF) ListIPSets(context.Context, *wafv2.ListIPSetsInput, ...func(*wafv2.Options)) (*wafv2.ListIPSetsOutput, error) {
	return &wafv2.ListIPSetsOutput{}, nil
}

func TestAWSWAFRollerApply(t *testing.T) {
	mock := newMockWAF()
	ids := map[string]string{}
	for _, f := range edge.AllFamilies {
		ids[string(f)] = "id-" + string(f)
	}
	r := edge.NewAWSWAFRoller(edge.WAFConfig{
		WebACLName:  "one-acl",
		WebACLID:    "acl-id",
		IPSetPrefix: "one-exp-",
		IPSetIDs:    ids,
		Region:      "us-east-1",
	})
	r.Client = mock
	p := edge.Policy{
		Client:   edge.FamilyPolicy{Mode: edge.ModePublic},
		Auth:     edge.FamilyPolicy{Mode: edge.ModePublic},
		Metadata: edge.FamilyPolicy{Mode: edge.ModeAllowlist, CIDRs: []string{"10.0.0.0/8"}},
		Deploy:   edge.FamilyPolicy{Mode: edge.ModeBlocked},
		Ops:      edge.FamilyPolicy{Mode: edge.ModeBlocked},
	}
	if err := r.Apply(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	if mock.updateIPCalls != len(edge.AllFamilies) {
		t.Fatalf("updateIPCalls=%d", mock.updateIPCalls)
	}
	if mock.updateACLCalls != 1 {
		t.Fatalf("updateACLCalls=%d", mock.updateACLCalls)
	}
	addrs := mock.ipSets["one-exp-metadata"].Addresses
	if len(addrs) != 1 || addrs[0] != "10.0.0.0/8" {
		t.Fatalf("metadata addrs=%v", addrs)
	}
}

func TestNewAWSWAFRoller(t *testing.T) {
	r := edge.NewAWSWAFRoller(edge.WAFConfig{WebACLName: "acl"})
	if r == nil || r.Mode() != "aws" {
		t.Fatalf("unexpected roller: %#v", r)
	}
	if r.Config.Configured() {
		t.Fatal("empty IP sets should not be Configured")
	}
}
