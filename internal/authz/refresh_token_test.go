package authz_test

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
)

func TestShouldIssueRefresh(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, azp, grant, kind string
		scopes                 []string
		want                   bool
	}{
		{"refresh_token not a new family", authz.ControlIDEAzp, authz.GrantRefreshToken, "", nil, false},
		{"install password", authz.InstallAzp, authz.GrantPassword, "", nil, true},
		{"install token exchange", authz.InstallAzp, authz.GrantTokenExchange, "", nil, true},
		{"install urn token exchange", authz.InstallAzp, "urn:ietf:params:oauth:grant-type:token-exchange", "", nil, true},
		{"install auth code without offline_access", authz.InstallAzp, authz.GrantAuthorizationCode, "public", nil, false},
		{"control ide password without offline_access", authz.ControlIDEAzp, authz.GrantPassword, "public", nil, false},
		{"control ide pkce without offline_access", authz.ControlIDEAzp, authz.GrantAuthorizationCode, "public", nil, false},
		{"control ide exchange without offline_access", authz.ControlIDEAzp, authz.GrantTokenExchange, "public", nil, false},
		{"control ide password with offline_access", authz.ControlIDEAzp, authz.GrantPassword, "public", []string{authz.ScopeOfflineAccess}, true},
		{"control ide pkce with offline_access", authz.ControlIDEAzp, authz.GrantAuthorizationCode, "public", []string{authz.ScopeOfflineAccess}, true},
		{"control ide exchange with offline_access", authz.ControlIDEAzp, authz.GrantTokenExchange, "public", []string{authz.ScopeOfflineAccess}, true},
		{"public offline_access password", "cx.app", authz.GrantPassword, "public", []string{authz.ScopeOfflineAccess}, true},
		{"public without offline_access", "cx.app", authz.GrantPassword, "public", []string{"client"}, false},
		{"confidential with offline_access", "svc.app", authz.GrantPassword, "confidential", []string{authz.ScopeOfflineAccess}, false},
		{"unknown grant public", "cx.app", "foo", "public", []string{authz.ScopeOfflineAccess}, false},
		{"client_credentials never", authz.InstallAzp, authz.GrantClientCredentials, "public", []string{authz.ScopeOfflineAccess}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := authz.ShouldIssueRefresh(tc.azp, tc.grant, tc.scopes, tc.kind)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestGenerateAndHashRefreshToken(t *testing.T) {
	t.Parallel()
	raw, hash, err := authz.GenerateRefreshToken(32)
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" || strings.Contains(raw, "=") {
		t.Fatalf("expected unpadded base64url, got %q", raw)
	}
	if _, err := base64.RawURLEncoding.DecodeString(raw); err != nil {
		t.Fatalf("raw not base64url: %v", err)
	}
	if hash != authz.HashRefreshToken(raw) {
		t.Fatal("hash mismatch")
	}
	if !authz.EqualRefreshHash(hash, authz.HashRefreshToken(raw)) {
		t.Fatal("constant-time compare failed")
	}
	if authz.EqualRefreshHash(hash, authz.HashRefreshToken(raw+"x")) {
		t.Fatal("expected mismatch")
	}
	key := authz.RefreshRateLimitKey(raw)
	if !strings.HasPrefix(key, "refresh:") || strings.Contains(key, raw) {
		t.Fatalf("rate key=%s", key)
	}
}

func TestClampRefreshIdle(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	family := now.Add(2 * time.Hour)
	got := authz.ClampRefreshIdle(now, 30*24*time.Hour, family)
	if !got.Equal(family) {
		t.Fatalf("got %s want family cap %s", got, family)
	}
	got = authz.ClampRefreshIdle(now, time.Hour, now.Add(90*24*time.Hour))
	if !got.Equal(now.Add(time.Hour)) {
		t.Fatalf("got %s", got)
	}
}
