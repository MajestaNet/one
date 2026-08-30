package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/testutil"
	"github.com/MajestaNet/ide/internal/worker"
)

func TestBP041ExternalIDWriteRules(t *testing.T) {
	h := newBP041Harness(t)
	suffix := h.suffix
	for _, tc := range []struct {
		apiName, fieldType string
		wantOK             bool
	}{
		{"ExtText" + suffix, "text", true},
		{"ExtEmail" + suffix, "email", true},
		{"ExtInt" + suffix, "integer", true},
		{"ExtBool" + suffix, "boolean", false},
		{"ExtArea" + suffix, "textarea", false},
		{"ExtLook" + suffix, "lookup", false},
	} {
		body := map[string]any{
			"objectApiName": "Contact",
			"apiName":       tc.apiName,
			"label":         tc.apiName,
			"fieldType":     tc.fieldType,
			"externalId":    true,
		}
		if tc.fieldType == "lookup" {
			body["referenceTo"] = "Account"
		}
		rr := h.auth(http.MethodPost, "/metadata/v1/fields", "admin-key", body)
		if tc.wantOK && rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
			t.Fatalf("field %s type %s: want success, got %d %s", tc.apiName, tc.fieldType, rr.Code, rr.Body.String())
		}
		if !tc.wantOK && rr.Code != http.StatusBadRequest {
			t.Fatalf("field %s type %s: want 400, got %d %s", tc.apiName, tc.fieldType, rr.Code, rr.Body.String())
		}
	}
	h.buildProjections("Contact")
	rr := h.auth(http.MethodGet, "/metadata/v1/projections/Contact", "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list projections %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "ready") {
		t.Fatalf("expected ready projection, got %s", rr.Body.String())
	}
}

func TestBP041RESTUpsertAuthZMatrix(t *testing.T) {
	h := newBP041Harness(t)
	ext := "ExtAuth" + h.suffix
	h.createExtField("Contact", ext, "text")
	h.buildProjections("Contact")

	key := "k-" + h.suffix
	rr := h.auth(http.MethodPatch, "/client/v1/sobjects/Contact/"+ext+"/"+key, "admin-key", map[string]any{
		"LastName": "Created",
		ext:        key,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("PATCH create %d %s", rr.Code, rr.Body.String())
	}
	var rec map[string]any
	decodeJSON(t, rr, &rec)
	if rec["created"] != true {
		t.Fatalf("created flag: %v", rec["created"])
	}
	rr = h.auth(http.MethodPost, "/client/v1/sobjects/Contact/upsert", "admin-key", map[string]any{
		"externalIdField": ext,
		"externalId":      key,
		"LastName":        "Updated",
		ext:               key,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("POST upsert update %d %s", rr.Code, rr.Body.String())
	}
	decodeJSON(t, rr, &rec)
	if rec["created"] != false {
		t.Fatalf("repeat created=%v", rec["created"])
	}
	rr = h.auth(http.MethodGet, "/client/v1/sobjects/Contact/"+ext+"/"+key, "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET by ext %d %s", rr.Code, rr.Body.String())
	}

	fullTok := h.mintToken(h.createPS("Full"+h.suffix, true, true, true, true, false, false, ext))
	rr = h.auth(http.MethodPost, "/client/v1/sobjects/Contact/upsert", fullTok, map[string]any{
		"externalIdField": ext, "externalId": "nonadmin-" + h.suffix, "LastName": "NonAdmin", ext: "nonadmin-" + h.suffix,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("non-admin create %d %s", rr.Code, rr.Body.String())
	}

	createTok := h.mintToken(h.createPS("CreateOnly"+h.suffix, true, true, false, false, false, false, ext))
	createKey := "createonly-" + h.suffix
	rr = h.auth(http.MethodPost, "/client/v1/sobjects/Contact/upsert", createTok, map[string]any{
		"externalIdField": ext, "externalId": createKey, "LastName": "Once", ext: createKey,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create-only first %d %s", rr.Code, rr.Body.String())
	}
	rr = h.auth(http.MethodPost, "/client/v1/sobjects/Contact/upsert", createTok, map[string]any{
		"externalIdField": ext, "externalId": createKey, "LastName": "Twice", ext: createKey,
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("create-only update want 403, got %d %s", rr.Code, rr.Body.String())
	}

	updateTok := h.mintToken(h.createPS("UpdateOnly"+h.suffix, false, true, true, false, false, false, ext))
	rr = h.auth(http.MethodPost, "/client/v1/sobjects/Contact/upsert", updateTok, map[string]any{
		"externalIdField": ext, "externalId": "upd-new-" + h.suffix, "LastName": "Nope", ext: "upd-new-" + h.suffix,
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("update-only create want 403, got %d %s", rr.Code, rr.Body.String())
	}
	existKey := "upd-exist-" + h.suffix
	rr = h.auth(http.MethodPost, "/client/v1/sobjects/Contact/upsert", "admin-key", map[string]any{
		"externalIdField": ext, "externalId": existKey, "LastName": "AdminSeed", ext: existKey,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("admin seed for update-only %d %s", rr.Code, rr.Body.String())
	}
	rr = h.auth(http.MethodPost, "/client/v1/sobjects/Contact/upsert", updateTok, map[string]any{
		"externalIdField": ext, "externalId": existKey, "LastName": "UpdatedByPS", ext: existKey,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("update-only update %d %s", rr.Code, rr.Body.String())
	}

	secret := "Secret" + h.suffix
	h.createPlainField("Contact", secret, "text")
	flsTok := h.mintToken(h.createPS("FLS"+h.suffix, true, true, true, true, false, false, ext))
	rr = h.auth(http.MethodPost, "/client/v1/sobjects/Contact/upsert", flsTok, map[string]any{
		"externalIdField": ext, "externalId": "fls-" + h.suffix, "LastName": "FLS", secret: "nope", ext: "fls-" + h.suffix,
	})
	if rr.Code != http.StatusForbidden && rr.Code != http.StatusBadRequest {
		t.Fatalf("FLS deny write want 403/400, got %d %s", rr.Code, rr.Body.String())
	}

	enable := h.auth(http.MethodPost, "/metadata/v1/sharing/enable", "admin-key", map[string]any{"confirm": true})
	if enable.Code != http.StatusOK && enable.Code != http.StatusConflict {
		t.Fatalf("enable sharing %d %s", enable.Code, enable.Body.String())
	}
	owd := h.auth(http.MethodPatch, "/metadata/v1/sharing/objects/Contact", "admin-key", map[string]any{"defaultAccess": "private"})
	if owd.Code != http.StatusOK {
		t.Fatalf("OWD private %d %s", owd.Code, owd.Body.String())
	}
	shareKey := "share-" + h.suffix
	rr = h.auth(http.MethodPost, "/client/v1/sobjects/Contact/upsert", "admin-key", map[string]any{
		"externalIdField": ext, "externalId": shareKey, "LastName": "OwnedByAdmin", ext: shareKey,
	})
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Fatalf("admin share seed %d %s", rr.Code, rr.Body.String())
	}
	rr = h.auth(http.MethodPost, "/client/v1/sobjects/Contact/upsert", fullTok, map[string]any{
		"externalIdField": ext, "externalId": shareKey, "LastName": "Steal", ext: shareKey,
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("sharing deny want 403, got %d %s", rr.Code, rr.Body.String())
	}

	rr = h.auth(http.MethodDelete, "/client/v1/sobjects/Contact/"+ext+"/"+key, "admin-key", nil)
	if rr.Code != http.StatusNoContent && rr.Code != http.StatusOK {
		t.Fatalf("DELETE by ext %d %s", rr.Code, rr.Body.String())
	}
}

func TestBP041DuplicateExternalID(t *testing.T) {
	h := newBP041Harness(t)
	ext := "ExtDup" + h.suffix
	h.createExtField("Contact", ext, "text")
	ctx := t.Context()
	dup := "dup-" + h.suffix
	owner := testutil.DefaultOwnerID
	payload, _ := json.Marshal(map[string]any{"LastName": "A", ext: dup})
	for i := 0; i < 2; i++ {
		if _, err := h.d.Pool.Exec(ctx, `
INSERT INTO records (object_api_name, owner_id, created_by_id, last_modified_by_id, data)
VALUES ('Contact', $1::uuid, $1::uuid, $1::uuid, $2::jsonb)`, owner, payload); err != nil {
			t.Fatalf("insert dirty row: %v", err)
		}
	}
	h.buildProjections("Contact")
	rr := h.auth(http.MethodGet, "/metadata/v1/projections/Contact", "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("projections %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "failed") {
		t.Fatalf("expected failed unique projection, got %s", rr.Body.String())
	}
	rr = h.auth(http.MethodPost, "/client/v1/sobjects/Contact/upsert", "admin-key", map[string]any{
		"externalIdField": ext, "externalId": dup, "LastName": "Conflict", ext: dup,
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("duplicate upsert want 409, got %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "DUPLICATE_EXTERNAL_ID") {
		t.Fatalf("want DUPLICATE_EXTERNAL_ID, got %s", rr.Body.String())
	}
}

func TestBP041IngestLifecycleAndChunking(t *testing.T) {
	h := newBP041Harness(t)
	ext := "ExtIng" + h.suffix
	h.createExtField("Contact", ext, "text")
	h.buildProjections("Contact")

	job := h.createIngest("Contact", "upsert", ext, false)
	var nd strings.Builder
	for i := 0; i < 501; i++ {
		b, _ := json.Marshal(map[string]any{"LastName": fmt.Sprintf("Row%d", i), ext: fmt.Sprintf("ing-%s-%d", h.suffix, i)})
		nd.Write(b)
		nd.WriteByte('\n')
	}
	nd.WriteString("{not-json\n")
	rr := h.putRaw("/client/v1/jobs/ingest/"+job+"/batches", "admin-key", nd.String())
	if rr.Code != http.StatusAccepted && rr.Code != http.StatusOK {
		t.Fatalf("batch %d %s", rr.Code, rr.Body.String())
	}
	rr = h.auth(http.MethodPatch, "/client/v1/jobs/ingest/"+job, "admin-key", map[string]any{"state": "UploadComplete"})
	if rr.Code != http.StatusOK {
		t.Fatalf("close %d %s", rr.Code, rr.Body.String())
	}
	h.runWorker()
	st := h.getIngest(job, "admin-key")
	if st["state"] != "JobComplete" {
		t.Fatalf("state=%v body=%v", st["state"], st)
	}
	if intFrom(st["successCount"]) != 501 {
		t.Fatalf("successCount=%v", st["successCount"])
	}
	if intFrom(st["failureCount"]) != 1 {
		t.Fatalf("failureCount=%v (bad row should not roll back chunk)", st["failureCount"])
	}
	okBody := h.getRaw("/client/v1/jobs/ingest/"+job+"/successfulResults", "admin-key")
	if strings.Count(okBody, `"Id"`) != 501 {
		t.Fatalf("expected 501 success rows, got %d id hits body=%s", strings.Count(okBody, `"Id"`), truncate(okBody, 200))
	}

	openID := h.createIngest("Contact", "upsert", ext, false)
	rr = h.auth(http.MethodDelete, "/client/v1/jobs/ingest/"+openID, "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("abort %d %s", rr.Code, rr.Body.String())
	}
	if h.getIngest(openID, "admin-key")["state"] != "Aborted" {
		t.Fatalf("abort state %v", h.getIngest(openID, "admin-key"))
	}

	otherTok := h.mintToken(h.createPS("IngestOther"+h.suffix, true, true, true, true, true, true, ext))
	rr = h.auth(http.MethodGet, "/client/v1/jobs/ingest/"+job, otherTok, nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("non-owner GET want 403, got %d %s", rr.Code, rr.Body.String())
	}
	rr = h.auth(http.MethodGet, "/client/v1/jobs/ingest/"+job, "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("admin GET %d %s", rr.Code, rr.Body.String())
	}

	ctx := t.Context()
	owner := testutil.DefaultOwnerID
	for i := 0; i < 2; i++ {
		if _, err := h.d.Pool.Exec(ctx, `
INSERT INTO ingest_jobs (actor_id, object_api_name, operation, state)
VALUES ($1::uuid, 'Contact', 'upsert', 'InProgress')`, owner); err != nil {
			t.Fatalf("hold InProgress: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = h.d.Pool.Exec(context.Background(), `DELETE FROM ingest_jobs WHERE state='InProgress' AND actor_id=$1::uuid AND row_count=0`, owner)
	})
	capped := h.createIngest("Contact", "upsert", ext, false)
	h.putRaw("/client/v1/jobs/ingest/"+capped+"/batches", "admin-key",
		fmt.Sprintf("{\"LastName\":\"Cap\",\"%s\":\"cap-%s\"}\n", ext, h.suffix))
	rr = h.auth(http.MethodPatch, "/client/v1/jobs/ingest/"+capped, "admin-key", map[string]any{"state": "UploadComplete"})
	if rr.Code != http.StatusOK {
		t.Fatalf("cap close %d %s", rr.Code, rr.Body.String())
	}
	h.runWorker()
	if h.getIngest(capped, "admin-key")["state"] != "UploadComplete" {
		t.Fatalf("capped job should stay UploadComplete, got %v", h.getIngest(capped, "admin-key")["state"])
	}

	if _, err := h.d.Pool.Exec(ctx, `DELETE FROM ingest_jobs WHERE state='InProgress' AND actor_id=$1::uuid AND row_count=0`, owner); err != nil {
		t.Fatalf("clear InProgress holds: %v", err)
	}
	h.runWorker()

	big := h.createIngest("Contact", "upsert", ext, false)
	var bigND strings.Builder
	for i := 0; i < 2000; i++ {
		b, _ := json.Marshal(map[string]any{"LastName": fmt.Sprintf("Big%d", i), ext: fmt.Sprintf("big-%s-%d", h.suffix, i)})
		bigND.Write(b)
		bigND.WriteByte('\n')
	}
	h.putRaw("/client/v1/jobs/ingest/"+big+"/batches", "admin-key", bigND.String())
	h.auth(http.MethodPatch, "/client/v1/jobs/ingest/"+big, "admin-key", map[string]any{"state": "UploadComplete"})
	h.runWorker()
	got := h.getIngest(big, "admin-key")
	if got["state"] != "JobComplete" {
		t.Fatalf("2k job state=%v err=%v", got["state"], got["errorMessage"])
	}
	if intFrom(got["successCount"]) != 2000 {
		t.Fatalf("2k successCount=%v", got["successCount"])
	}
}

func TestBP041CompositeUPSERT(t *testing.T) {
	h := newBP041Harness(t)
	accExt := "AccExt" + h.suffix
	conExt := "ConExt" + h.suffix
	h.createExtField("Account", accExt, "text")
	h.createExtField("Contact", conExt, "text")
	h.buildProjections("Account")
	h.buildProjections("Contact")
	rr := h.auth(http.MethodPost, "/client/v1/composite", "admin-key", map[string]any{
		"compositeRequest": []map[string]any{
			{"method": "UPSERT", "object": "Account", "referenceId": "acct", "body": map[string]any{
				"externalIdField": accExt, "externalId": "A-" + h.suffix, "Name": "Acme " + h.suffix, accExt: "A-" + h.suffix,
			}},
			{"method": "UPSERT", "object": "Contact", "referenceId": "con", "body": map[string]any{
				"externalIdField": conExt, "externalId": "C-" + h.suffix, "LastName": "Child",
				conExt: "C-" + h.suffix, "AccountId": "@{acct.Id}",
			}},
		},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("composite %d %s", rr.Code, rr.Body.String())
	}
	var out struct {
		CompositeResponse []map[string]any `json:"compositeResponse"`
	}
	decodeJSON(t, rr, &out)
	if len(out.CompositeResponse) != 2 {
		t.Fatalf("subrequests=%d body=%s", len(out.CompositeResponse), rr.Body.String())
	}
	if intFrom(out.CompositeResponse[0]["status"]) != 201 && intFrom(out.CompositeResponse[0]["status"]) != 200 {
		t.Fatalf("account status %v", out.CompositeResponse[0]["status"])
	}
	if intFrom(out.CompositeResponse[1]["status"]) != 201 && intFrom(out.CompositeResponse[1]["status"]) != 200 {
		t.Fatalf("contact status %v body=%s", out.CompositeResponse[1]["status"], rr.Body.String())
	}
}

type bp041Harness struct {
	t      *testing.T
	d      *testutil.Database
	srv    *testutil.TestServer
	suffix string
}

func newBP041Harness(t *testing.T) *bp041Harness {
	t.Helper()
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys:   "admin-key+admin",
		EnableJWT: true,
	})
	h := &bp041Harness{t: t, d: d, srv: srv, suffix: time.Now().Format("150405000")}
	ctx := t.Context()
	_, _ = d.Pool.Exec(ctx, `
UPDATE ingest_jobs
SET state='Failed', error_message='test harness reset', completed_at=now()
WHERE state IN ('Open','UploadComplete','InProgress')`)
	_, _ = d.Pool.Exec(ctx, `
UPDATE jobs
SET status='failed', last_error='test harness reset', completed_at=now()
WHERE job_type='ingest.process' AND status='pending'`)
	for _, obj := range []string{"Contact", "Account"} {
		_ = h.auth(http.MethodPatch, "/metadata/v1/sharing/objects/"+obj, "admin-key", map[string]any{
			"defaultAccess": "public_read_write",
		})
	}
	_, _ = d.Pool.Exec(ctx, `
UPDATE jobs SET status='failed', last_error='bp041 start drain', completed_at=now()
WHERE status='pending' AND job_type IN ('sharing.recalc','projection.build','search.reindex')`)
	t.Cleanup(func() {
		c := context.Background()
		// Restore OWD first — those PATCHes enqueue sharing.recalc. Then fail leftover
		// pending jobs so later tests (automation, outbox) are not starved.
		for _, obj := range []string{"Contact", "Account"} {
			_ = h.auth(http.MethodPatch, "/metadata/v1/sharing/objects/"+obj, "admin-key", map[string]any{
				"defaultAccess": "public_read_write",
			})
		}
		_, _ = d.Pool.Exec(c, `
UPDATE jobs SET status='failed', last_error='bp041 cleanup', completed_at=now()
WHERE status='pending' AND job_type IN ('sharing.recalc','projection.build','ingest.process','search.reindex')`)
		_, _ = d.Pool.Exec(c, `
UPDATE ingest_jobs SET state='Failed', error_message='bp041 cleanup', completed_at=now()
WHERE state IN ('Open','UploadComplete','InProgress')`)
	})
	return h
}

func (h *bp041Harness) auth(method, path, bearer string, body any) *httptest.ResponseRecorder {
	return testutil.AuthRequest(h.srv.Handler, method, path, bearer, body)
}

func (h *bp041Harness) createExtField(object, apiName, fieldType string) {
	h.t.Helper()
	rr := h.auth(http.MethodPost, "/metadata/v1/fields", "admin-key", map[string]any{
		"objectApiName": object, "apiName": apiName, "label": apiName, "fieldType": fieldType, "externalId": true,
	})
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		h.t.Fatalf("create field %s.%s %d %s", object, apiName, rr.Code, rr.Body.String())
	}
}

func (h *bp041Harness) createPlainField(object, apiName, fieldType string) {
	h.t.Helper()
	rr := h.auth(http.MethodPost, "/metadata/v1/fields", "admin-key", map[string]any{
		"objectApiName": object, "apiName": apiName, "label": apiName, "fieldType": fieldType,
	})
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		h.t.Fatalf("create field %s.%s %d %s", object, apiName, rr.Code, rr.Body.String())
	}
}

func (h *bp041Harness) buildProjections(object string) {
	h.t.Helper()
	rr := h.auth(http.MethodPost, "/metadata/v1/projections/"+object+"/build", "admin-key", nil)
	if rr.Code != http.StatusAccepted && rr.Code != http.StatusOK {
		h.t.Fatalf("build projections %s %d %s", object, rr.Code, rr.Body.String())
	}
}

func (h *bp041Harness) createPS(apiName string, canCreate, canRead, canUpdate, canDelete, viewAll, modifyAll bool, extField string) string {
	h.t.Helper()
	rr := h.auth(http.MethodPost, "/metadata/v1/permissions/sets", "admin-key", map[string]any{
		"apiName": apiName, "label": apiName,
		"objectPermissions": []map[string]any{
			{
				"objectApiName": "Contact", "canCreate": canCreate, "canRead": canRead,
				"canUpdate": canUpdate, "canDelete": canDelete, "viewAll": viewAll, "modifyAll": modifyAll,
			},
			{
				"objectApiName": "Account", "canCreate": canCreate, "canRead": canRead,
				"canUpdate": canUpdate, "canDelete": canDelete, "viewAll": viewAll, "modifyAll": modifyAll,
			},
		},
		"fieldPermissions": []map[string]any{
			{"objectApiName": "Contact", "fieldApiName": "LastName", "canRead": true, "canEdit": true},
			{"objectApiName": "Contact", "fieldApiName": extField, "canRead": true, "canEdit": true},
			{"objectApiName": "Account", "fieldApiName": "Name", "canRead": true, "canEdit": true},
		},
	})
	if rr.Code != http.StatusCreated {
		h.t.Fatalf("create PS %s %d %s", apiName, rr.Code, rr.Body.String())
	}
	return apiName
}

func (h *bp041Harness) mintToken(psName string) string {
	h.t.Helper()
	email := strings.ToLower(psName) + "@example.com"
	rr := h.auth(http.MethodPost, "/client/v1/principals", "admin-key", map[string]any{
		"email": email, "displayName": psName, "principalType": "service",
		"roleApiNames": []string{"StandardUser"}, "permissionSetApiNames": []string{psName},
	})
	if rr.Code != http.StatusCreated {
		h.t.Fatalf("principal %d %s", rr.Code, rr.Body.String())
	}
	var p map[string]any
	decodeJSON(h.t, rr, &p)
	pid, _ := p["id"].(string)
	rr = h.auth(http.MethodPost, "/client/v1/principals/"+pid+"/credentials", "admin-key", map[string]any{"label": "t"})
	if rr.Code != http.StatusCreated {
		h.t.Fatalf("credential %d %s", rr.Code, rr.Body.String())
	}
	var cred map[string]any
	decodeJSON(h.t, rr, &cred)
	secret, _ := cred["clientSecret"].(string)
	rr = h.auth(http.MethodPost, "/auth/v1/token", "", map[string]any{
		"grant_type": "client_credentials", "client_id": pid, "client_secret": secret,
	})
	if rr.Code != http.StatusOK {
		h.t.Fatalf("token %d %s", rr.Code, rr.Body.String())
	}
	var tok map[string]any
	decodeJSON(h.t, rr, &tok)
	access, _ := tok["access_token"].(string)
	if access == "" {
		h.t.Fatal("empty access token")
	}
	return access
}

func (h *bp041Harness) createIngest(object, op, ext string, allOrNone bool) string {
	h.t.Helper()
	rr := h.auth(http.MethodPost, "/client/v1/jobs/ingest", "admin-key", map[string]any{
		"object": object, "operation": op, "externalIdField": ext,
		"contentType": "application/x-ndjson", "allOrNone": allOrNone,
	})
	if rr.Code != http.StatusCreated {
		h.t.Fatalf("create ingest %d %s", rr.Code, rr.Body.String())
	}
	var job map[string]any
	decodeJSON(h.t, rr, &job)
	id, _ := job["id"].(string)
	if id == "" {
		h.t.Fatalf("missing ingest id: %s", rr.Body.String())
	}
	return id
}

func (h *bp041Harness) getIngest(id, bearer string) map[string]any {
	h.t.Helper()
	rr := h.auth(http.MethodGet, "/client/v1/jobs/ingest/"+id, bearer, nil)
	if rr.Code != http.StatusOK {
		h.t.Fatalf("get ingest %d %s", rr.Code, rr.Body.String())
	}
	var job map[string]any
	decodeJSON(h.t, rr, &job)
	return job
}

func (h *bp041Harness) putRaw(path, bearer, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/x-ndjson")
	rr := httptest.NewRecorder()
	h.srv.Handler.ServeHTTP(rr, req)
	return rr
}

func (h *bp041Harness) getRaw(path, bearer string) string {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+bearer)
	rr := httptest.NewRecorder()
	h.srv.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		h.t.Fatalf("GET %s %d %s", path, rr.Code, rr.Body.String())
	}
	b, _ := io.ReadAll(rr.Body)
	return string(b)
}

func (h *bp041Harness) runWorker() {
	h.t.Helper()
	ctx := h.t.Context()
	_, _ = h.d.Pool.Exec(ctx, `
UPDATE jobs SET status='failed', last_error='bp041 drain', completed_at=now()
WHERE status='pending' AND job_type <> 'ingest.process'`)
	opts := &worker.ProcessOptions{
		DataEngine: h.srv.Data,
		ObjectAz:   &authz.ObjectAuthz{Store: &db.ObjectPermStore{Pool: h.d.Pool}},
		FieldAz:    &authz.FieldAuthz{Store: &db.FieldPermStore{Pool: h.d.Pool}},
		JobLimit:   50,
	}
	for i := 0; i < 16; i++ {
		n, err := worker.ProcessJobs(ctx, h.d.Pool, opts)
		if err != nil {
			h.t.Fatalf("ProcessJobs: %v", err)
		}
		if n == 0 {
			break
		}
	}
	_, _ = h.d.Pool.Exec(ctx, `
UPDATE jobs SET status='failed', last_error='bp041 leftover', completed_at=now()
WHERE status='pending' AND job_type IN ('sharing.recalc','projection.build','search.reindex')`)
}

func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder, dest any) {
	t.Helper()
	if err := json.Unmarshal(rr.Body.Bytes(), dest); err != nil {
		t.Fatalf("decode: %v body=%s", err, rr.Body.String())
	}
}

func intFrom(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
