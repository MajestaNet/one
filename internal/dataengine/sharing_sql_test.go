package dataengine_test

import (
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/metadata"
)

func TestAppendSharingVisibilityDisabled(t *testing.T) {
	where := []string{}
	args := []any{}
	dataengine.AppendSharingVisibility("r", &where, &args, dataengine.QueryVisibility{})
	if len(where) != 0 || len(args) != 0 {
		t.Fatalf("expected no change, where=%v args=%v", where, args)
	}
}

func TestAppendSharingVisibilityLegacy(t *testing.T) {
	where := []string{}
	args := []any{}
	dataengine.AppendSharingVisibility("r", &where, &args, dataengine.QueryVisibility{
		Mode:   dataengine.VisibilityLegacy,
		UserID: "00000000-0000-4000-8000-0000000000aa",
	})
	if len(where) != 1 {
		t.Fatalf("where=%v", where)
	}
	clause := where[0]
	for _, want := range []string{"r.owner_id", "r.created_by_id"} {
		if !strings.Contains(clause, want) {
			t.Fatalf("missing %q in %s", want, clause)
		}
	}
	if strings.Contains(clause, "record_access_grants") {
		t.Fatal("legacy mode must not reference grants")
	}
	if len(args) != 1 {
		t.Fatalf("args=%v", args)
	}
}

func TestAppendSharingVisibilityPrivateWithHierarchy(t *testing.T) {
	where := []string{}
	args := []any{}
	dataengine.AppendSharingVisibility("r", &where, &args, dataengine.QueryVisibility{
		Mode:                   dataengine.VisibilitySharing,
		UserID:                 "00000000-0000-4000-8000-0000000000aa",
		DefaultAccess:          authz.DefaultAccessPrivate,
		HasObjectRead:          true,
		SubordinateDataRoleIDs: []string{"00000000-0000-4000-8000-0000000000bb"},
	})
	if len(where) != 1 {
		t.Fatalf("where=%v", where)
	}
	clause := where[0]
	for _, want := range []string{"r.owner_id", "r.created_by_id", "record_access_grants", "data_role_id = ANY"} {
		if !strings.Contains(clause, want) {
			t.Fatalf("missing %q in %s", want, clause)
		}
	}
	if strings.Contains(clause, "TRUE") {
		t.Fatal("private OWD must not add TRUE public clause")
	}
	if len(args) != 3 {
		t.Fatalf("args=%v", args)
	}
}

func TestAppendSharingVisibilityPublicRead(t *testing.T) {
	where := []string{}
	args := []any{}
	dataengine.AppendSharingVisibility("r", &where, &args, dataengine.QueryVisibility{
		Mode:          dataengine.VisibilitySharing,
		UserID:        "u1",
		DefaultAccess: authz.DefaultAccessPublicRead,
		HasObjectRead: true,
	})
	if !strings.Contains(where[0], "TRUE") {
		t.Fatalf("public_read should OR TRUE, got %s", where[0])
	}
}

func TestAppendSharingVisibilityChildAlias(t *testing.T) {
	where := []string{}
	args := []any{}
	dataengine.AppendSharingVisibility("c", &where, &args, dataengine.QueryVisibility{
		Mode:   dataengine.VisibilityLegacy,
		UserID: "u1",
	})
	if !strings.Contains(where[0], "c.owner_id") || !strings.Contains(where[0], "c.created_by_id") {
		t.Fatalf("expected child alias, got %s", where[0])
	}
}

func TestBuildCriteriaSQLRequiresFilterableField(t *testing.T) {
	fields := []metadata.FieldDefinition{{
		APIName: "Region__c", Label: "Region", FieldType: "text", Filterable: true,
	}}
	built, err := dataengine.BuildCriteriaSQL(&dataengine.QueryRequest{
		Object:  "Account",
		Filters: []dataengine.QueryFilter{{Field: "Region__c", Op: dataengine.OpEq, Value: "West"}},
	}, fields, "records")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(built.Text, "SELECT r.id::text") {
		t.Fatalf("sql=%s", built.Text)
	}
	if len(built.Args) < 2 {
		t.Fatalf("args=%v", built.Args)
	}
}

func TestBuildCriteriaSQLRejectsUnknownField(t *testing.T) {
	fields := []metadata.FieldDefinition{{
		APIName: "Name", Label: "Name", FieldType: "text", Filterable: true,
	}}
	_, err := dataengine.BuildCriteriaSQL(&dataengine.QueryRequest{
		Object:  "Account",
		Filters: []dataengine.QueryFilter{{Field: "Hack'); DROP TABLE--", Op: dataengine.OpEq, Value: "x"}},
	}, fields, "records")
	if err == nil {
		t.Fatal("expected rejection of unsafe field")
	}
}

func TestBuildCriteriaSQLAppliesParameterizedLimitOnlyWhenRequested(t *testing.T) {
	fields := []metadata.FieldDefinition{{APIName: "Name", FieldType: "text", Filterable: true}}
	limited, err := dataengine.BuildCriteriaSQL(&dataengine.QueryRequest{
		Object: "Account", Filters: []dataengine.QueryFilter{{Field: "Name", Op: dataengine.OpEq, Value: "A"}}, Limit: 25,
	}, fields, "records")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(limited.Text, "LIMIT $3") || len(limited.Args) != 3 || limited.Args[2] != 25 {
		t.Fatalf("sql=%s args=%v", limited.Text, limited.Args)
	}
	unlimited, err := dataengine.BuildCriteriaSQL(&dataengine.QueryRequest{
		Object: "Account", Filters: []dataengine.QueryFilter{{Field: "Name", Op: dataengine.OpEq, Value: "A"}},
	}, fields, "records")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(unlimited.Text, "LIMIT") {
		t.Fatalf("unexpected limit in %s", unlimited.Text)
	}
}

func TestBuildQueryVisibilityNilPool(t *testing.T) {
	vis, err := dataengine.BuildQueryVisibility(t.Context(), nil, &authz.Actor{ID: "u1"}, "Account", nil)
	if err != nil {
		t.Fatal(err)
	}
	if vis.Enabled() {
		t.Fatal("nil pool must not enable visibility")
	}
}

func TestBuildQueryVisibilityViewAllBypass(t *testing.T) {
	vis, err := dataengine.BuildQueryVisibility(t.Context(), nil, &authz.Actor{ID: "u1"}, "Account", map[string]struct{}{"Account": {}})
	if err != nil {
		t.Fatal(err)
	}
	if vis.Enabled() {
		t.Fatal("viewAll must disable SQL sharing filter")
	}
}

func TestBuildQueryVisibilityAdminBypass(t *testing.T) {
	vis, err := dataengine.BuildQueryVisibility(t.Context(), nil, &authz.Actor{ID: "u1", IsAdmin: true}, "Account", nil)
	if err != nil {
		t.Fatal(err)
	}
	if vis.Enabled() {
		t.Fatal("admin must disable SQL visibility filter")
	}
}
