package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCanvasSpecCRUDAndRejectUnknownKind(t *testing.T) {
	_, pool, srv := setupPlaybookTest(t)
	const name = "TestCanvasSpec__c"
	_, _ = pool.Exec(context.Background(), `DELETE FROM metadata_canvases WHERE api_name=$1`, name)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM metadata_canvases WHERE api_name=$1`, name)
	})

	bad, _ := json.Marshal(map[string]any{
		"apiName": name,
		"label":   "Bad",
		"layout":  map[string]any{"mode": "sections"},
		"nodes":   []map[string]any{{"id": "x", "kind": "rawHtml", "props": map[string]any{}}},
	})
	reqBad := httptest.NewRequest(http.MethodPost, "/metadata/v1/canvases", bytes.NewReader(bad))
	reqBad.Header.Set("Authorization", "Bearer admin")
	reqBad.Header.Set("Content-Type", "application/json")
	recBad := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recBad, reqBad)
	if recBad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown kind, got %d %s", recBad.Code, recBad.Body.String())
	}

	body, _ := json.Marshal(map[string]any{
		"apiName": name,
		"label":   "Top Accounts",
		"layout": map[string]any{
			"mode":     "sections",
			"sections": []map[string]any{{"id": "main", "nodeIds": []string{"stat-1"}}},
		},
		"nodes": []map[string]any{
			{"id": "stat-1", "kind": "stat", "props": map[string]any{"value": 0, "label": "Count"}},
		},
		"dataBindings": []map[string]any{
			{"id": "bind-1", "objectApiName": "Account", "query": map[string]any{"limit": 25}},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/metadata/v1/canvases", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}

	reqGet := httptest.NewRequest(http.MethodGet, "/metadata/v1/canvases/"+name, nil)
	reqGet.Header.Set("Authorization", "Bearer admin")
	recGet := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("get: %d %s", recGet.Code, recGet.Body.String())
	}
}

func TestToolSpecCRUDViaToolsPathAndChrome(t *testing.T) {
	_, pool, srv := setupPlaybookTest(t)
	const name = "TestToolSpec__c"
	_, _ = pool.Exec(context.Background(), `DELETE FROM metadata_canvases WHERE api_name=$1`, name)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM metadata_canvases WHERE api_name=$1`, name)
	})

	body, _ := json.Marshal(map[string]any{
		"apiName":     name,
		"label":       "Pipeline Tool",
		"description": "Phase 2 chrome",
		"icon":        "pipeline",
		"sortOrder":   10,
		"layout": map[string]any{
			"mode":     "sections",
			"sections": []map[string]any{{"id": "main", "nodeIds": []string{"stat-1"}}},
		},
		"nodes": []map[string]any{
			{"id": "stat-1", "kind": "stat", "props": map[string]any{"value": 1, "label": "Open"}},
		},
		"dataBindings": []any{},
	})
	req := httptest.NewRequest(http.MethodPost, "/metadata/v1/tools", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create tools: %d %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created["icon"] != "pipeline" {
		t.Fatalf("icon=%v", created["icon"])
	}
	if int(created["sortOrder"].(float64)) != 10 {
		t.Fatalf("sortOrder=%v", created["sortOrder"])
	}

	reqList := httptest.NewRequest(http.MethodGet, "/metadata/v1/tools", nil)
	reqList.Header.Set("Authorization", "Bearer admin")
	recList := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("list tools: %d %s", recList.Code, recList.Body.String())
	}
	var listed map[string]any
	if err := json.Unmarshal(recList.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	tools, _ := listed["tools"].([]any)
	if len(tools) == 0 {
		t.Fatal("expected tools list key")
	}
	if _, ok := listed["canvases"]; ok {
		t.Fatal("tools list must not emit canvases key")
	}

	reqGet := httptest.NewRequest(http.MethodGet, "/metadata/v1/tools/"+name, nil)
	reqGet.Header.Set("Authorization", "Bearer admin")
	recGet := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("get tools: %d %s", recGet.Code, recGet.Body.String())
	}

	patch, _ := json.Marshal(map[string]any{"sortOrder": 3, "icon": "queue"})
	reqPatch := httptest.NewRequest(http.MethodPatch, "/metadata/v1/tools/"+name, bytes.NewReader(patch))
	reqPatch.Header.Set("Authorization", "Bearer admin")
	reqPatch.Header.Set("Content-Type", "application/json")
	recPatch := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recPatch, reqPatch)
	if recPatch.Code != http.StatusOK {
		t.Fatalf("patch tools: %d %s", recPatch.Code, recPatch.Body.String())
	}
	var patched map[string]any
	_ = json.Unmarshal(recPatch.Body.Bytes(), &patched)
	if patched["icon"] != "queue" || int(patched["sortOrder"].(float64)) != 3 {
		t.Fatalf("patch chrome: %+v", patched)
	}

	reqDel := httptest.NewRequest(http.MethodDelete, "/metadata/v1/tools/"+name, nil)
	reqDel.Header.Set("Authorization", "Bearer admin")
	recDel := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusNoContent && recDel.Code != http.StatusOK {
		t.Fatalf("delete tools: %d %s", recDel.Code, recDel.Body.String())
	}
}
