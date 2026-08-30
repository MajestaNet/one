package digitalocean

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientCRUD(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v2/account", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"account":{"email":"a@b.c"}}`))
	})
	mux.HandleFunc("GET /v2/apps/app-1", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"app": map[string]any{
				"id":       "app-1",
				"live_url": "https://one.example.com",
				"spec": map[string]any{
					"name":   "one-prod",
					"region": "nyc",
					"services": []map[string]any{{
						"name":               "api",
						"instance_count":     2,
						"instance_size_slug": "apps-s-1vcpu-1gb",
						"image":              map[string]any{"registry_type": "GHCR", "repository": "one-api", "tag": "0.1.0"},
					}},
					"workers": []map[string]any{{
						"name":               "worker",
						"instance_count":     1,
						"instance_size_slug": "apps-s-1vcpu-1gb",
						"image":              map[string]any{"registry_type": "GHCR", "repository": "one-worker", "tag": "0.1.0"},
					}},
				},
			},
		})
	})
	mux.HandleFunc("PUT /v2/apps/app-1", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		spec, _ := body["spec"].(map[string]any)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"app": map[string]any{"id": "app-1", "spec": spec, "live_url": "https://one.example.com"},
		})
	})
	mux.HandleFunc("POST /v2/apps", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"app": map[string]any{
				"id":       "app-new",
				"live_url": "https://new.ondigitalocean.app",
				"spec":     body["spec"],
			},
		})
	})
	mux.HandleFunc("POST /v2/databases", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"database": map[string]any{
				"id": "db-1", "name": "one-dev", "engine": "pg", "version": "16",
				"status": "online", "region": "nyc", "size": "db-s-1vcpu-1gb", "num_nodes": 1,
				"connection": map[string]any{"uri": "postgres://u:p@host:25060/defaultdb?sslmode=require"},
			},
		})
	})
	mux.HandleFunc("PUT /v2/databases/db-1/resize", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("GET /v2/databases/db-1", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"database": map[string]any{"id": "db-1", "size": "db-s-1vcpu-2gb", "num_nodes": 2, "region": "nyc", "status": "online"},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewClient("test-token").WithBaseURL(srv.URL + "/v2")
	ctx := context.Background()

	ok, err := c.AccountOK(ctx)
	if err != nil || !ok {
		t.Fatalf("AccountOK: ok=%v err=%v", ok, err)
	}

	app, err := c.GetApp(ctx, "app-1")
	if err != nil || app.ID != "app-1" || app.PublicURL() != "https://one.example.com" {
		t.Fatalf("GetApp: %+v err=%v", app, err)
	}
	spec, err := ParseAppSpec(app.Spec)
	if err != nil || len(spec.Services) != 1 {
		t.Fatalf("ParseAppSpec: %+v err=%v", spec, err)
	}
	spec.Services[0].InstanceCount = 3
	updated, err := c.UpdateApp(ctx, "app-1", spec)
	if err != nil || updated.ID != "app-1" {
		t.Fatalf("UpdateApp: %+v err=%v", updated, err)
	}

	created, err := c.CreateApp(ctx, &AppSpec{Name: "peer", Region: "nyc"})
	if err != nil || created.ID != "app-new" {
		t.Fatalf("CreateApp: %+v err=%v", created, err)
	}

	db, err := c.CreateDatabase(ctx, "one-dev", "nyc", "db-s-1vcpu-1gb", 1, "16")
	if err != nil || db.ID != "db-1" {
		t.Fatalf("CreateDatabase: %+v err=%v", db, err)
	}
	if err := c.ResizeDatabase(ctx, "db-1", "db-s-1vcpu-2gb", 2); err != nil {
		t.Fatalf("ResizeDatabase: %v", err)
	}
	got, err := c.GetDatabase(ctx, "db-1")
	if err != nil || got.NumNodes != 2 {
		t.Fatalf("GetDatabase: %+v err=%v", got, err)
	}
}

func TestClientNotConfigured(t *testing.T) {
	t.Parallel()
	c := NewClient("")
	_, err := c.GetApp(context.Background(), "x")
	if err != ErrNotConfigured {
		t.Fatalf("got %v", err)
	}
}

func TestAPIErrorRedactsToken(t *testing.T) {
	t.Parallel()
	token := "super-secret-token-value"
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v2/apps/x", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"bad super-secret-token-value"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := NewClient(token).WithBaseURL(srv.URL + "/v2")
	_, err := c.GetApp(context.Background(), "x")
	ae, ok := err.(*APIError)
	if !ok || ae.Status != 401 {
		t.Fatalf("expected APIError 401, got %v", err)
	}
	if stringsContains(ae.Message, token) {
		t.Fatalf("token leaked in error: %q", ae.Message)
	}
}

func stringsContains(s, sub string) bool {
	return len(sub) > 0 && (s == sub || len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})())
}
