package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPrincipalCanvasDocumentCRUD(t *testing.T) {
	_, pool, srv := setupPlaybookTest(t)
	const canvasID = "test-principal-canvas-1"
	_, _ = pool.Exec(context.Background(), `DELETE FROM principal_canvas_documents WHERE canvas_id=$1`, canvasID)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM principal_canvas_documents WHERE canvas_id=$1`, canvasID)
	})

	doc := map[string]any{
		"apiVersion": "one.canvas/v1",
		"id":         canvasID,
		"title":      "My working set",
		"layout": map[string]any{
			"mode": "spatial",
			"positions": map[string]any{
				"stat-1": map[string]any{"x": 0, "y": 0, "w": 200, "h": 120},
			},
		},
		"nodes": []map[string]any{
			{"id": "stat-1", "kind": "stat", "props": map[string]any{"value": 2, "label": "Open"}},
		},
	}
	body, _ := json.Marshal(doc)
	reqPut := httptest.NewRequest(http.MethodPut, "/client/v1/canvases/"+canvasID, bytes.NewReader(body))
	reqPut.Header.Set("Authorization", "Bearer admin")
	reqPut.Header.Set("Content-Type", "application/json")
	recPut := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recPut, reqPut)
	if recPut.Code != http.StatusOK {
		t.Fatalf("put: %d %s", recPut.Code, recPut.Body.String())
	}

	reqList := httptest.NewRequest(http.MethodGet, "/client/v1/canvases", nil)
	reqList.Header.Set("Authorization", "Bearer admin")
	recList := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("list: %d %s", recList.Code, recList.Body.String())
	}

	reqGet := httptest.NewRequest(http.MethodGet, "/client/v1/canvases/"+canvasID, nil)
	reqGet.Header.Set("Authorization", "Bearer admin")
	recGet := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("get: %d %s", recGet.Code, recGet.Body.String())
	}

	reqDel := httptest.NewRequest(http.MethodDelete, "/client/v1/canvases/"+canvasID, nil)
	reqDel.Header.Set("Authorization", "Bearer admin")
	recDel := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", recDel.Code, recDel.Body.String())
	}
}

func TestPrincipalCanvasRejectsUnknownKind(t *testing.T) {
	_, _, srv := setupPlaybookTest(t)
	doc := map[string]any{
		"apiVersion": "one.canvas/v1",
		"id":         "bad-canvas",
		"title":      "Bad",
		"layout":     map[string]any{"mode": "sections"},
		"nodes": []map[string]any{
			{"id": "x", "kind": "iframe", "props": map[string]any{}},
		},
	}
	body, _ := json.Marshal(doc)
	req := httptest.NewRequest(http.MethodPut, "/client/v1/canvases/bad-canvas", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d %s", rec.Code, rec.Body.String())
	}
}
