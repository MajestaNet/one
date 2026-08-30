package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/testutil"
)

func TestClientSearch(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})

	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys:   "admin-key+admin,agent-key:client,meta-key:metadata",
		EnableJWT: true,
	})
	h := srv.Handler
	auth := func(method, path, key string, body any) *httptest.ResponseRecorder {
		return testutil.AuthRequest(h, method, path, key, body)
	}

	suffix := time.Now().Format("150405.000")
	acctName := "Acme SearchCo " + suffix
	rr := auth(http.MethodPost, "/client/v1/sobjects/Account", "admin-key", map[string]any{
		"Name":  acctName,
		"Phone": "(415) 555-0199",
	})
	if rr.Code != 201 && rr.Code != 200 {
		t.Fatalf("create account %d %s", rr.Code, rr.Body.String())
	}
	rr = auth(http.MethodPost, "/client/v1/sobjects/Contact", "admin-key", map[string]any{
		"LastName":  "SearchSolo" + suffix,
		"FirstName": "Jane",
		"Email":     "jane.search+" + suffix + "@example.com",
	})
	if rr.Code != 201 && rr.Code != 200 {
		t.Fatalf("create contact %d %s", rr.Code, rr.Body.String())
	}

	rr = auth(http.MethodPost, "/client/v1/search", "admin-key", map[string]any{"q": "acme searchco"})
	if rr.Code != 200 {
		t.Fatalf("search %d %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	hits, _ := body["hits"].([]any)
	if len(hits) == 0 {
		t.Fatalf("expected hits, body=%s", rr.Body.String())
	}

	rr = auth(http.MethodPost, "/client/v1/search", "admin-key", map[string]any{"q": "415555"})
	if rr.Code != 200 {
		t.Fatalf("phone search %d %s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if hits, _ = body["hits"].([]any); len(hits) == 0 {
		t.Fatalf("expected phone hits, body=%s", rr.Body.String())
	}

	rr = auth(http.MethodPost, "/client/v1/search", "admin-key", map[string]any{
		"q":       "searchsolo",
		"objects": []string{"Contact"},
	})
	if rr.Code != 200 {
		t.Fatalf("filtered search %d %s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	hits, _ = body["hits"].([]any)
	for _, h := range hits {
		m, _ := h.(map[string]any)
		if m["object"] != "Contact" {
			t.Fatalf("object filter leaked %v", h)
		}
	}
	if len(hits) == 0 {
		t.Fatalf("expected contact hits, body=%s", rr.Body.String())
	}

	rr = auth(http.MethodPost, "/client/v1/search", "admin-key", map[string]any{"q": "a"})
	if rr.Code != 400 {
		t.Fatalf("short q status %d %s", rr.Code, rr.Body.String())
	}

	rr = auth(http.MethodPost, "/client/v1/search", "admin-key", map[string]any{
		"q": "acme", "objects": []string{"DoesNotExist"},
	})
	if rr.Code != 400 {
		t.Fatalf("unknown object status %d %s", rr.Code, rr.Body.String())
	}

	rr = auth(http.MethodPost, "/client/v1/search", "meta-key", map[string]any{"q": "acme"})
	if rr.Code != 401 && rr.Code != 403 {
		t.Fatalf("metadata key should not search, status %d %s", rr.Code, rr.Body.String())
	}

	rr = auth(http.MethodPost, "/client/v1/search", "agent-key", map[string]any{"q": "acme searchco"})
	if rr.Code != 200 {
		t.Fatalf("client-scope search %d %s", rr.Code, rr.Body.String())
	}

	email := "search-std-" + suffix + "@example.com"
	rr = auth(http.MethodPost, "/client/v1/principals", "admin-key", map[string]any{
		"email": email, "displayName": "Search Std", "principalType": "service",
		"roleApiNames": []string{"StandardUser"},
	})
	if rr.Code != 201 {
		t.Fatalf("create principal %d %s", rr.Code, rr.Body.String())
	}
	var principal map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &principal)
	pid, _ := principal["id"].(string)
	rr = auth(http.MethodPost, "/client/v1/principals/"+pid+"/credentials", "admin-key", map[string]any{"label": "search"})
	if rr.Code != 201 {
		t.Fatalf("credential %d %s", rr.Code, rr.Body.String())
	}
	var cred map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &cred)
	secret, _ := cred["clientSecret"].(string)
	tokBody, _ := json.Marshal(map[string]any{
		"grant_type": "client_credentials", "client_id": pid, "client_secret": secret,
	})
	tokReq := httptest.NewRequest(http.MethodPost, "/auth/v1/token", bytes.NewReader(tokBody))
	tokReq.Header.Set("Content-Type", "application/json")
	tokRR := httptest.NewRecorder()
	h.ServeHTTP(tokRR, tokReq)
	if tokRR.Code != 200 {
		t.Fatalf("token %d %s", tokRR.Code, tokRR.Body.String())
	}
	var tok map[string]any
	_ = json.Unmarshal(tokRR.Body.Bytes(), &tok)
	access, _ := tok["access_token"].(string)

	rr = auth(http.MethodPost, "/client/v1/search", access, map[string]any{"q": "acme searchco"})
	if rr.Code != 200 {
		t.Fatalf("std search %d %s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	hits, _ = body["hits"].([]any)
	if len(hits) != 0 {
		t.Fatalf("StandardUser without object grants should omit hits, got %s", rr.Body.String())
	}

	rr = auth(http.MethodPost, "/client/v1/search", access, map[string]any{
		"q": "acme", "objects": []string{"Account"},
	})
	if rr.Code != 400 {
		t.Fatalf("named unread object want 400, got %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "cannot read object") && !strings.Contains(rr.Body.String(), "VALIDATION_ERROR") {
		t.Fatalf("expected cannot-read validation, body=%s", rr.Body.String())
	}
}
