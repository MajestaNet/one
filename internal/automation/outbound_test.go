package automation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type stubHost struct{}

func (stubHost) CreateRecord(context.Context, string, map[string]any) (string, error) {
	return "", nil
}
func (stubHost) UpdateRecord(context.Context, string, string, map[string]any) error { return nil }
func (stubHost) GetRecord(context.Context, string, string) (map[string]any, error) {
	return map[string]any{}, nil
}
func (stubHost) DeleteRecord(context.Context, string, string) error { return nil }
func (stubHost) Query(context.Context, map[string]any) (map[string]any, error) {
	return map[string]any{}, nil
}
func (stubHost) InvokeAction(context.Context, string, map[string]any) (map[string]any, error) {
	return map[string]any{}, nil
}

func TestSyncOutboundBan(t *testing.T) {
	h := SyncOutboundBan{Inner: stubHost{}}
	if _, err := h.HTTPCall(context.Background(), HTTPCallArgs{URL: "https://example.com"}); err == nil {
		t.Fatal("expected sync ban")
	}
}

func TestOutboundHTTPSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Fatalf("missing auth header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hello":"world"}`))
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	host := OutboundHost{
		Inner:          stubHost{},
		AllowlistCache: []string{u.Hostname()},
		HTTPClient:     srv.Client(),
		ValidateFunc:   func(string) error { return nil },
	}
	out, err := host.HTTPCall(context.Background(), HTTPCallArgs{
		Method:    "GET",
		URL:       srv.URL + "/x",
		SecretRef: "",
		Headers:   map[string]string{"Authorization": "Bearer tok"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out["ok"] != true {
		t.Fatalf("got %#v", out)
	}
	js, _ := out["json"].(map[string]any)
	if js["hello"] != "world" {
		t.Fatalf("json %#v", js)
	}
}

func TestDispatchHTTPRequiresOutbound(t *testing.T) {
	_, err := dispatchHostRPC(context.Background(), stubHost{}, "http", json.RawMessage(`{"method":"GET","url":"https://example.com"}`))
	if err == nil {
		t.Fatal("expected error without OutboundBridge")
	}
}

func TestIsFrozenSDKMethodHTTP(t *testing.T) {
	if !IsFrozenSDKMethod("http") || !IsFrozenSDKMethod("connector") {
		t.Fatal("http/connector should be frozen SDK methods")
	}
}
