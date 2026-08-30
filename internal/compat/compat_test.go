package compat_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/compat"
)

func TestNormalizeWindow(t *testing.T) {
	w, err := compat.NormalizeWindow(12, 14)
	if err != nil {
		t.Fatal(err)
	}
	if w.Min != 12 || w.Current != 14 {
		t.Fatalf("window=%v", w)
	}
	if _, err := compat.NormalizeWindow(15, 14); err == nil {
		t.Fatal("expected min>current error")
	}
}

func TestAPIRevisionWindowRecommendedAlias(t *testing.T) {
	raw, err := json.Marshal(compat.APIRevisionWindow{Min: 12, Current: 14})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["min"] != float64(12) || got["current"] != float64(14) || got["recommended"] != float64(14) {
		t.Fatalf("apiRevision=%v", got)
	}
}

func TestPinInWindow(t *testing.T) {
	w := compat.APIRevisionWindow{Min: 12, Current: 14}
	if !compat.PinInWindow(12, w) || !compat.PinInWindow(14, w) {
		t.Fatal("expected in-window pins ok")
	}
	if compat.PinInWindow(11, w) || compat.PinInWindow(15, w) {
		t.Fatal("expected out-of-window pins rejected")
	}
}

func TestResolvePin(t *testing.T) {
	w := compat.APIRevisionWindow{Min: 12, Current: 14}
	pin, explicit, err := compat.ResolvePin("", w)
	if err != nil || explicit || pin != 14 {
		t.Fatalf("default pin: pin=%d explicit=%v err=%v", pin, explicit, err)
	}
	pin, explicit, err = compat.ResolvePin("12", w)
	if err != nil || !explicit || pin != 12 {
		t.Fatalf("explicit pin: pin=%d explicit=%v err=%v", pin, explicit, err)
	}
	_, _, err = compat.ResolvePin("bad", w)
	if err == nil {
		t.Fatal("expected unparsable header error")
	}
}

func TestSplitRevisionPath(t *testing.T) {
	cases := []struct {
		in       string
		wantPath string
		wantPin  int
		found    bool
	}{
		{"/client/v1/sobjects/Account", "/client/v1/sobjects/Account", 0, false},
		{"/client/v1/r12/sobjects/Account", "/client/v1/sobjects/Account", 12, true},
		{"/client/v1/r12", "/client/v1", 12, true},
		{"/v1/r14/me", "/v1/me", 14, true},
		{"/auth/v1/r12/token", "/auth/v1/token", 12, true},
		{"/mcp/r12/tools", "/mcp/tools", 12, true},
		{"/scim/v2/Users", "/scim/v2/Users", 0, false},
		{"/client/v1/records/x", "/client/v1/records/x", 0, false},
		{"/version", "/version", 0, false},
	}
	for _, tc := range cases {
		gotPath, gotPin, found := compat.SplitRevisionPath(tc.in)
		if gotPath != tc.wantPath || gotPin != tc.wantPin || found != tc.found {
			t.Fatalf("%s: path=%s pin=%d found=%v", tc.in, gotPath, gotPin, found)
		}
	}
}

func TestPathRequiresRevision(t *testing.T) {
	if !compat.PathRequiresRevision("/client/v1/me") || !compat.PathRequiresRevision("/auth/v1/token") {
		t.Fatal("family paths must require revision")
	}
	if !compat.PathRequiresRevision("/client/v1/r12/me") || !compat.PathRequiresRevision("/mcp") {
		t.Fatal("alias and MCP must require revision")
	}
	if compat.PathRequiresRevision("/version") || compat.PathRequiresRevision("/healthz") || compat.PathRequiresRevision("/scim/v2/Users") {
		t.Fatal("discovery/health/SCIM must stay revision-agnostic")
	}
}

func TestUnsupportedCTA(t *testing.T) {
	w := compat.APIRevisionWindow{Min: 12, Current: 14}
	if !strings.Contains(compat.UnsupportedCTA(11, w), "Control IDE") {
		t.Fatal("pin below min should CTA client migrate")
	}
	if !strings.Contains(compat.UnsupportedCTA(15, w), "/ops/v1") {
		t.Fatal("pin above current should CTA install upgrade")
	}
}

func TestSelectClientPin(t *testing.T) {
	w := compat.APIRevisionWindow{Min: 12, Current: 14}
	pin, code, err := compat.SelectClientPin(12, 14, w)
	if err != nil || code != "" || pin != 14 {
		t.Fatalf("pin=%d code=%s err=%v", pin, code, err)
	}
	_, code, err = compat.SelectClientPin(12, 14, compat.APIRevisionWindow{Min: 12, Current: 11})
	if err == nil || code != "INSTALL_REVISION_TOO_OLD" {
		t.Fatalf("expected install too old, code=%s err=%v", code, err)
	}
	_, code, err = compat.SelectClientPin(12, 11, w)
	if err == nil || code != "API_REVISION_UNSUPPORTED" {
		t.Fatalf("expected unsupported pin, code=%s err=%v", code, err)
	}
}

func TestProductTestedAgainst(t *testing.T) {
	status, code := compat.ProductTestedAgainst("0.4.1", "0.4.2", 2)
	if status != "ok" || code != "" {
		t.Fatalf("status=%s code=%s", status, code)
	}
	status, code = compat.ProductTestedAgainst("0.3.9", "0.4.2", 2)
	if status != "ok" || code != "" {
		t.Fatalf("within N=2 minors: status=%s code=%s", status, code)
	}
	status, code = compat.ProductTestedAgainst("0.2.0", "0.4.2", 2)
	if status != "warn" || code != "PRODUCT_OUTSIDE_TESTED" {
		t.Fatalf("outside window: status=%s code=%s", status, code)
	}
	status, code = compat.ProductTestedAgainst("1.0.0", "0.4.2", 2)
	if status != "warn" || code != "PRODUCT_OUTSIDE_TESTED" {
		t.Fatalf("major skew: status=%s code=%s", status, code)
	}
}
