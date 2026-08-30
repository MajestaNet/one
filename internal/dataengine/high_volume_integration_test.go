package dataengine_test

import (
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestHighVolumeObjectRoutesAndGuardrails(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()

	obj := "HvEvent" + time.Now().Format("150405")
	t.Cleanup(func() {
		_, _ = d.Pool.Exec(ctx, `DELETE FROM records_hv WHERE object_api_name=$1`, obj)
		_, _ = d.Pool.Exec(ctx, `DELETE FROM metadata_fields WHERE object_api_name=$1`, obj)
		_, _ = d.Pool.Exec(ctx, `DELETE FROM metadata_objects WHERE api_name=$1`, obj)
		_, _ = d.Pool.Exec(ctx, `DELETE FROM high_volume_objects WHERE object_api_name=$1`, obj)
	})

	if _, err := d.Meta.InsertObject(ctx, metadata.ObjectDefinition{
		APIName: obj, Label: "HV Event", PluralLabel: "HV Events", StorageMode: db.StorageModeHighVolume,
	}, metadata.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Meta.InsertField(ctx, metadata.FieldDefinition{
		ObjectAPIName: obj, APIName: "Body", Label: "Body", FieldType: "textarea",
		Required: true, Filterable: false,
	}, metadata.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Meta.InsertField(ctx, metadata.FieldDefinition{
		ObjectAPIName: obj, APIName: "AccountId", Label: "Account", FieldType: "lookup",
		ReferenceTo: strPtr("Account"), Filterable: true, Indexed: true,
	}, metadata.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	got, err := d.Meta.GetObject(ctx, obj)
	if err != nil {
		t.Fatal(err)
	}
	if got.StorageMode != db.StorageModeHighVolume {
		t.Fatalf("storageMode=%s", got.StorageMode)
	}

	svc := dataengine.NewService(d.Pool, d.Meta)
	actor := &authz.Actor{ID: testutil.DefaultOwnerID, IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient}}

	acct, err := svc.Create(ctx, "Account", map[string]any{"Name": "HV Parent Co"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	acctID, _ := acct["Id"].(string)

	rec, err := svc.Create(ctx, obj, map[string]any{
		"Body":      "append-heavy row",
		"AccountId": acctID,
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := rec["Id"].(string)
	if id == "" {
		t.Fatalf("record=%v", rec)
	}

	var hvCount, flexCount int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM records_hv WHERE id=$1::uuid AND object_api_name=$2`, id, obj).Scan(&hvCount); err != nil {
		t.Fatal(err)
	}
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM records WHERE id=$1::uuid`, id).Scan(&flexCount); err != nil {
		t.Fatal(err)
	}
	if hvCount != 1 || flexCount != 0 {
		t.Fatalf("hv=%d flex=%d", hvCount, flexCount)
	}

	loaded, err := svc.Get(ctx, obj, id)
	if err != nil || loaded["Body"] != "append-heavy row" {
		t.Fatalf("get=%v err=%v", loaded, err)
	}

	if _, err := svc.Query(ctx, []byte(`{"object":"`+obj+`","limit":10}`), dataengine.QueryVisibility{}); err == nil {
		t.Fatal("expected high_volume guardrail error")
	}

	qraw := []byte(`{
		"object":"` + obj + `",
		"limit":10,
		"filters":[{"field":"AccountId","op":"eq","value":"` + acctID + `"}]
	}`)
	res, err := svc.Query(ctx, qraw, dataengine.QueryVisibility{})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalSize < 1 {
		t.Fatalf("query result=%+v", res)
	}
}

func strPtr(s string) *string { return &s }
