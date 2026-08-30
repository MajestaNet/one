package datapack_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/datapack"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/testutil"
	"github.com/MajestaNet/ide/internal/worker"
)

func TestApplyLargeStepUsesIngestJob(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{APIKeys: "admin-key+admin"})
	_, _ = d.Pool.Exec(t.Context(), `
UPDATE ingest_jobs SET state='Failed', error_message='test harness reset', completed_at=now()
WHERE state IN ('Open','UploadComplete','InProgress')`)
	_, _ = d.Pool.Exec(t.Context(), `
UPDATE jobs SET status='failed', last_error='test harness reset', completed_at=now()
WHERE job_type='ingest.process' AND status='pending'`)
	suffix := time.Now().Format("150405000")
	ext := "PackExt" + suffix
	rr := testutil.AuthRequest(srv.Handler, "POST", "/metadata/v1/fields", "admin-key", map[string]any{
		"objectApiName": "Contact", "apiName": ext, "label": ext, "fieldType": "text", "externalId": true,
	})
	if rr.Code != 201 && rr.Code != 200 {
		t.Fatalf("create field %d %s", rr.Code, rr.Body.String())
	}
	rr = testutil.AuthRequest(srv.Handler, "POST", "/metadata/v1/projections/Contact/build", "admin-key", nil)
	if rr.Code != 202 && rr.Code != 200 {
		t.Fatalf("build projections %d %s", rr.Code, rr.Body.String())
	}

	httpSrv := httptest.NewServer(srv.Handler)
	t.Cleanup(httpSrv.Close)
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(context.Background(), `
UPDATE jobs SET status='failed', last_error='bp041 leftover', completed_at=now()
WHERE status='pending' AND job_type IN ('sharing.recalc','projection.build','ingest.process','search.reindex')`)
	})

	packDir := t.TempDir()
	var lines strings.Builder
	for i := 0; i < 501; i++ {
		b, _ := json.Marshal(map[string]any{"LastName": fmt.Sprintf("Pack%d", i), ext: fmt.Sprintf("p-%s-%d", suffix, i)})
		lines.Write(b)
		lines.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(packDir, "contacts.jsonl"), []byte(lines.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	yaml := fmt.Sprintf(`
apiVersion: one-datapack/v1
name: bp041-large
steps:
  - id: contacts
    object: Contact
    operation: upsert
    externalIdField: %s
    file: contacts.jsonl
`, ext)
	if err := os.WriteFile(filepath.Join(packDir, "datapack.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	m, dir, err := datapack.LoadManifest(packDir)
	if err != nil {
		t.Fatal(err)
	}
	runWorker := func() {
		_, _ = d.Pool.Exec(t.Context(), `
UPDATE jobs SET status='failed', last_error='bp041 drain', completed_at=now()
WHERE status='pending' AND job_type <> 'ingest.process'`)
		opts := &worker.ProcessOptions{
			DataEngine: srv.Data,
			ObjectAz:   &authz.ObjectAuthz{Store: &db.ObjectPermStore{Pool: d.Pool}},
			FieldAz:    &authz.FieldAuthz{Store: &db.FieldPermStore{Pool: d.Pool}},
			JobLimit:   50,
		}
		for i := 0; i < 16; i++ {
			n, err := worker.ProcessJobs(t.Context(), d.Pool, opts)
			if err != nil {
				t.Fatalf("ProcessJobs: %v", err)
			}
			if n == 0 {
				break
			}
		}
		_, _ = d.Pool.Exec(t.Context(), `
UPDATE jobs SET status='failed', last_error='bp041 leftover', completed_at=now()
WHERE status='pending' AND job_type IN ('sharing.recalc','projection.build','search.reindex')`)
	}
	report, err := datapack.Apply(m, datapack.ApplyOptions{
		PackDir:      dir,
		Offline:      true,
		Target:       &datapack.OrgClient{BaseURL: httpSrv.URL, Bearer: "admin-key"},
		OnIngestWait: runWorker,
	})
	if err != nil {
		t.Fatalf("Apply: %v report=%v", err, report)
	}
	if len(report.Steps) != 1 || report.Steps[0].Upserted != 501 {
		t.Fatalf("upserted=%v failed=%v err=%q", report.Steps[0].Upserted, report.Steps[0].Failed, report.Steps[0].Error)
	}
	var n int
	if err := d.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM ingest_jobs WHERE object_api_name='Contact' AND row_count >= 501`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatal("expected an ingest job for the 501-row datapack step")
	}
}
