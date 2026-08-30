package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/MajestaNet/ide/internal/testutil"
)

func TestPromotionsRejectInboundArtifact(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{APIKeys: "admin-key+admin"})
	rr := testutil.AuthRequest(ts.Handler, http.MethodPost, "/deploy/v1/promotions", "admin-key", map[string]any{
		"artifact": map[string]any{
			"manifestVersion":    1,
			"ownership":          "custom",
			"defaultPackageName": "customer.default",
			"objects":            []any{},
		},
		"checksum": "x",
		"dryRun":   true,
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	msg, _ := body["message"].(string)
	if msg == "" || !containsFold(msg, "inbound artifact promote removed") {
		t.Fatalf("expected inbound rejection message, body=%s", rr.Body.String())
	}
}

func containsFold(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(len(s) > 0 && (indexFold(s, sub) >= 0)))
}

func indexFold(s, sub string) int {
	// tiny case-insensitive contains without importing strings for one check
	ls, lsub := []rune(s), []rune(sub)
	for i := 0; i+len(lsub) <= len(ls); i++ {
		ok := true
		for j := 0; j < len(lsub); j++ {
			a, b := ls[i+j], lsub[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}
