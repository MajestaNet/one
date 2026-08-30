package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/MajestaNet/ide/internal/testutil"
)

func TestRunGraphResolveRejectsOverBatchLimit(t *testing.T) {
	database := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, database, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, database, testutil.ServerOptions{APIKeys: "resolve-limit+admin"})
	nodes := make([]any, 0, 201)
	for i := 0; i < 201; i++ {
		nodes = append(nodes, map[string]any{
			"nodeId": fmt.Sprintf("node-%d", i), "objectApiName": "Account",
			"recordId": fmt.Sprintf("00000000-0000-4000-8000-%012d", i),
		})
	}
	response := testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/run-graphs/resolve", "resolve-limit", map[string]any{"nodes": nodes})
	if response.Code != http.StatusBadRequest || !bytes.Contains(response.Body.Bytes(), []byte("nodes exceeds max 200")) {
		t.Fatalf("expected resolve batch rejection: %d %s", response.Code, response.Body.String())
	}
}

func TestRunGraphManualAcceptanceStripThenHydrate(t *testing.T) {
	database := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, database, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, database, testutil.ServerOptions{
		APIKeys: "accept-admin+admin",
	})
	_, _ = database.Pool.Exec(context.Background(), `DELETE FROM principal_run_graphs WHERE graph_key='home'`)
	t.Cleanup(func() {
		_, _ = database.Pool.Exec(context.Background(), `DELETE FROM principal_run_graphs WHERE graph_key='home'`)
	})

	home := testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/run-graphs/home", "accept-admin", nil)
	if home.Code != http.StatusOK {
		t.Fatalf("get home: %d %s", home.Code, home.Body.String())
	}
	create := testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/sobjects/Account", "accept-admin", map[string]any{
		"Name": "Pinned account",
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create record: %d %s", create.Code, create.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	recordID, _ := created["Id"].(string)
	if recordID == "" {
		t.Fatalf("create response missing Id: %s", create.Body.String())
	}
	t.Cleanup(func() {
		_, _ = database.Pool.Exec(context.Background(), `DELETE FROM records WHERE object_api_name='Account' AND id=$1::uuid`, recordID)
	})

	doc := runGraphDocument("home")
	doc["rows"] = []any{map[string]any{"Name": "must not persist"}}
	recordNode := doc["nodes"].([]any)[0].(map[string]any)
	recordNode["ref"].(map[string]any)["recordId"] = recordID
	recordNode["fields"] = map[string]any{"Name": "must not persist"}
	put := testutil.AuthRequest(srv.Handler, http.MethodPut, "/client/v1/run-graphs/home", "accept-admin", doc)
	if put.Code != http.StatusOK {
		t.Fatalf("put home: %d %s", put.Code, put.Body.String())
	}
	get := testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/run-graphs/home", "accept-admin", nil)
	if get.Code != http.StatusOK || bytes.Contains(get.Body.Bytes(), []byte(`"rows"`)) || bytes.Contains(get.Body.Bytes(), []byte(`"fields"`)) {
		t.Fatalf("GET retained baked payload: %d %s", get.Code, get.Body.String())
	}
	if !bytes.Contains(get.Body.Bytes(), []byte(recordID)) {
		t.Fatalf("GET lost pinned record reference: %s", get.Body.String())
	}

	resolved := testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/run-graphs/resolve", "accept-admin", map[string]any{
		"projection": "card",
		"nodes": []any{
			map[string]any{"nodeId": "account-1", "objectApiName": "Account", "recordId": recordID},
		},
	})
	if resolved.Code != http.StatusOK || !bytes.Contains(resolved.Body.Bytes(), []byte(`"Name":"Pinned account"`)) {
		t.Fatalf("pinned reference did not hydrate: %d %s", resolved.Code, resolved.Body.String())
	}
}

func TestRunGraphResolveOKForbiddenAndNotFound(t *testing.T) {
	database := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, database, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, database, testutil.ServerOptions{
		APIKeys: "resolve-admin+admin,resolve-client:client",
	})

	create := testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/sobjects/Account", "resolve-admin", map[string]any{
		"Name":     "Resolve card account",
		"Industry": "Technology",
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create record: %d %s", create.Code, create.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	recordID, _ := created["Id"].(string)
	if recordID == "" {
		t.Fatalf("create response missing Id: %s", create.Body.String())
	}
	t.Cleanup(func() {
		_, _ = database.Pool.Exec(t.Context(), `DELETE FROM records WHERE object_api_name='Account' AND id=$1::uuid`, recordID)
	})

	body := map[string]any{
		"projection": "card",
		"nodes": []any{
			map[string]any{"nodeId": "visible", "objectApiName": "Account", "recordId": recordID},
			map[string]any{"nodeId": "missing", "objectApiName": "Account", "recordId": "00000000-0000-4000-8000-000000009999"},
		},
	}
	resolved := testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/run-graphs/resolve", "resolve-admin", body)
	if resolved.Code != http.StatusOK {
		t.Fatalf("resolve: %d %s", resolved.Code, resolved.Body.String())
	}
	var response struct {
		Nodes []struct {
			NodeID string         `json:"nodeId"`
			OK     bool           `json:"ok"`
			Code   string         `json:"code"`
			Record map[string]any `json:"record"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(resolved.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Nodes) != 2 || !response.Nodes[0].OK || response.Nodes[0].Record["Name"] != "Resolve card account" {
		t.Fatalf("expected hydrated card: %s", resolved.Body.String())
	}
	if response.Nodes[0].Record["Industry"] != nil {
		t.Fatalf("card projection returned non-card field: %s", resolved.Body.String())
	}
	if response.Nodes[1].OK || response.Nodes[1].Code != "NOT_FOUND" {
		t.Fatalf("expected not found entry: %s", resolved.Body.String())
	}

	forbidden := testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/run-graphs/resolve", "resolve-client", map[string]any{
		"nodes": []any{
			map[string]any{"nodeId": "forbidden", "objectApiName": "Account", "recordId": recordID},
		},
	})
	if forbidden.Code != http.StatusOK {
		t.Fatalf("forbidden resolve: %d %s", forbidden.Code, forbidden.Body.String())
	}
	response.Nodes = nil
	if err := json.Unmarshal(forbidden.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Nodes) != 1 || response.Nodes[0].OK || response.Nodes[0].Code != "FORBIDDEN" || response.Nodes[0].Record != nil {
		t.Fatalf("expected forbidden entry without record: %s", forbidden.Body.String())
	}
}
