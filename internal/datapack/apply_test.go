package datapack

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOrgClientDoJSONSetsAPIRevisionHeader(t *testing.T) {
	var got string
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("One-API-Revision")
		auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(srv.Close)

	c := &OrgClient{BaseURL: srv.URL, Bearer: "tok", ApiRevisionPin: 1}
	raw, status, err := c.doJSON(http.MethodGet, "/client/v1/sobjects/Account/1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, raw)
	}
	if got != "1" {
		t.Fatalf("One-API-Revision=%q want 1", got)
	}
	if auth != "Bearer tok" {
		t.Fatalf("Authorization=%q", auth)
	}
}

func TestOrgClientDoJSONOmitsRevisionWhenUnpinned(t *testing.T) {
	var present bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header[http.CanonicalHeaderKey("One-API-Revision")]
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := &OrgClient{BaseURL: srv.URL, Bearer: "tok"}
	_, _, err := c.doJSON(http.MethodPost, "/client/v1/query", map[string]any{"object": "Account"})
	if err != nil {
		t.Fatal(err)
	}
	if present {
		t.Fatal("expected omitted One-API-Revision when pin is 0")
	}
}
