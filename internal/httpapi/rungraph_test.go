package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MajestaNet/ide/internal/rungraph"
)

func runGraphDocument(graphKey string) map[string]any {
	return map[string]any{
		"apiVersion": rungraph.DocumentAPIVersion,
		"id":         graphKey,
		"title":      "My graph",
		"nodes": []any{
			map[string]any{
				"id":   "account-1",
				"kind": "record",
				"ref": map[string]any{
					"objectApiName": "Account",
					"recordId":      "00000000-0000-4000-8000-000000000111",
				},
			},
		},
		"edges": []any{},
	}
}

func serveRunGraphJSON(t *testing.T, srv http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	return serveRunGraphJSONHeaders(t, srv, method, path, token, body, nil)
}

func serveRunGraphJSONHeaders(t *testing.T, srv http.Handler, method, path, token string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestRunGraphHomeCreatePutPatchAndSanitize(t *testing.T) {
	_, pool, srv := setupPlaybookTest(t)
	_, _ = pool.Exec(context.Background(), `DELETE FROM principal_run_graphs WHERE graph_key='home'`)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM principal_run_graphs WHERE graph_key='home'`)
	})

	home := serveRunGraphJSON(t, srv.Handler(), http.MethodGet, "/client/v1/run-graphs/home", "admin", nil)
	if home.Code != http.StatusOK {
		t.Fatalf("get home: %d %s", home.Code, home.Body.String())
	}
	var created struct {
		GraphKey string `json:"graphKey"`
		Revision int64  `json:"revision"`
	}
	if err := json.Unmarshal(home.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.GraphKey != "home" || created.Revision != 1 {
		t.Fatalf("unexpected home response: %s", home.Body.String())
	}
	if home.Header().Get("ETag") != `"1"` {
		t.Fatalf("missing home revision ETag: %q", home.Header().Get("ETag"))
	}

	doc := runGraphDocument("home")
	doc["rows"] = []any{map[string]any{"Name": "must not persist"}}
	record := doc["nodes"].([]any)[0].(map[string]any)
	record["fields"] = map[string]any{"Name": "must not persist"}
	record["cards"] = []any{map[string]any{"title": "must not persist"}}
	put := serveRunGraphJSON(t, srv.Handler(), http.MethodPut, "/client/v1/run-graphs/home", "admin", doc)
	if put.Code != http.StatusOK {
		t.Fatalf("put: %d %s", put.Code, put.Body.String())
	}
	if bytes.Contains(put.Body.Bytes(), []byte("must not persist")) || bytes.Contains(put.Body.Bytes(), []byte(`"rows"`)) {
		t.Fatalf("put response retained baked payload: %s", put.Body.String())
	}
	stale := serveRunGraphJSONHeaders(t, srv.Handler(), http.MethodPut, "/client/v1/run-graphs/home", "admin", doc, map[string]string{"If-Match": `"1"`})
	if stale.Code != http.StatusConflict || !bytes.Contains(stale.Body.Bytes(), []byte("REVISION_CONFLICT")) {
		t.Fatalf("stale PUT should conflict: %d %s", stale.Code, stale.Body.String())
	}
	weak := serveRunGraphJSONHeaders(t, srv.Handler(), http.MethodPut, "/client/v1/run-graphs/home", "admin", doc, map[string]string{"If-Match": `W/"2"`})
	if weak.Code != http.StatusBadRequest || !bytes.Contains(weak.Body.Bytes(), []byte("strong quoted")) {
		t.Fatalf("weak If-Match must be rejected: %d %s", weak.Code, weak.Body.String())
	}

	patch := serveRunGraphJSON(t, srv.Handler(), http.MethodPatch, "/client/v1/run-graphs/home", "admin", map[string]any{"title": "Priority graph"})
	if patch.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", patch.Code, patch.Body.String())
	}
	var patched struct {
		Title    string `json:"title"`
		Revision int64  `json:"revision"`
	}
	if err := json.Unmarshal(patch.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Title != "Priority graph" || patched.Revision != 3 {
		t.Fatalf("unexpected patch response: %s", patch.Body.String())
	}

	get := serveRunGraphJSON(t, srv.Handler(), http.MethodGet, "/client/v1/run-graphs/home", "admin", nil)
	if get.Code != http.StatusOK || bytes.Contains(get.Body.Bytes(), []byte(`"fields"`)) || bytes.Contains(get.Body.Bytes(), []byte(`"cards"`)) {
		t.Fatalf("stored graph retained baked payload: %d %s", get.Code, get.Body.String())
	}
	if !bytes.Contains(get.Body.Bytes(), []byte(`"recordId":"00000000-0000-4000-8000-000000000111"`)) {
		t.Fatalf("stored graph lost record ref: %s", get.Body.String())
	}
	var audited int
	if err := pool.QueryRow(context.Background(), `
SELECT count(*) FROM audit_log WHERE action IN ('run_graph.put', 'run_graph.patch')
  AND details->>'graphKey'='home' AND (details->>'documentBytes')::int > 0`).Scan(&audited); err != nil {
		t.Fatal(err)
	}
	if audited < 2 {
		t.Fatalf("expected PUT and PATCH graph audit rows, got %d", audited)
	}
}

func TestRunGraphOwnerIsolation(t *testing.T) {
	_, pool, srv := setupPlaybookTest(t)
	const graphKey = "owner-isolation-test"
	_, _ = pool.Exec(context.Background(), `DELETE FROM principal_run_graphs WHERE graph_key=$1`, graphKey)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM principal_run_graphs WHERE graph_key=$1`, graphKey)
	})

	put := serveRunGraphJSON(t, srv.Handler(), http.MethodPut, "/client/v1/run-graphs/"+graphKey, "admin", runGraphDocument(graphKey))
	if put.Code != http.StatusOK {
		t.Fatalf("put: %d %s", put.Code, put.Body.String())
	}
	otherGet := serveRunGraphJSON(t, srv.Handler(), http.MethodGet, "/client/v1/run-graphs/"+graphKey, "clientonly", nil)
	if otherGet.Code != http.StatusNotFound {
		t.Fatalf("other principal read owner graph: %d %s", otherGet.Code, otherGet.Body.String())
	}
	otherPatch := serveRunGraphJSON(t, srv.Handler(), http.MethodPatch, "/client/v1/run-graphs/"+graphKey, "clientonly", map[string]any{"title": "stolen"})
	if otherPatch.Code != http.StatusNotFound {
		t.Fatalf("other principal patched owner graph: %d %s", otherPatch.Code, otherPatch.Body.String())
	}
}

func TestRunGraphRejectsInvalidKindAndOverCap(t *testing.T) {
	_, pool, srv := setupPlaybookTest(t)
	const graphKey = "validation-test"
	_, _ = pool.Exec(context.Background(), `DELETE FROM principal_run_graphs WHERE graph_key=$1`, graphKey)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM principal_run_graphs WHERE graph_key=$1`, graphKey)
	})

	invalid := runGraphDocument(graphKey)
	invalid["nodes"].([]any)[0].(map[string]any)["kind"] = "remoteReact"
	badKind := serveRunGraphJSON(t, srv.Handler(), http.MethodPut, "/client/v1/run-graphs/"+graphKey, "admin", invalid)
	if badKind.Code != http.StatusBadRequest {
		t.Fatalf("invalid kind: %d %s", badKind.Code, badKind.Body.String())
	}

	over := runGraphDocument(graphKey)
	nodes := make([]any, 0, rungraph.MaxNodes+1)
	for i := 0; i <= rungraph.MaxNodes; i++ {
		nodes = append(nodes, map[string]any{"id": fmt.Sprintf("cluster-%d", i), "kind": "cluster", "label": "Cluster"})
	}
	over["nodes"] = nodes
	overCap := serveRunGraphJSON(t, srv.Handler(), http.MethodPut, "/client/v1/run-graphs/"+graphKey, "admin", over)
	if overCap.Code != http.StatusBadRequest || !bytes.Contains(overCap.Body.Bytes(), []byte("nodes exceeds")) {
		t.Fatalf("over cap: %d %s", overCap.Code, overCap.Body.String())
	}
}
