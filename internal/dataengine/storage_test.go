package dataengine

import (
	"testing"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
)

func TestHighVolumeQueryGuardrails(t *testing.T) {
	fields := []metadata.FieldDefinition{
		{APIName: "ParentId", Indexed: true, Filterable: true},
		{APIName: "Body", Indexed: false, Filterable: false},
	}
	if err := assertHighVolumeQueryGuardrails(db.StorageModeFlexible, &QueryRequest{}, fields); err != nil {
		t.Fatal(err)
	}
	if err := assertHighVolumeQueryGuardrails(db.StorageModeHighVolume, &QueryRequest{}, fields); err == nil {
		t.Fatal("expected error for unbounded hv query")
	}
	if err := assertHighVolumeQueryGuardrails(db.StorageModeHighVolume, &QueryRequest{
		Filters: []QueryFilter{{Field: "ParentId", Op: OpEq, Value: "00000000-0000-4000-8000-000000000001"}},
	}, fields); err != nil {
		t.Fatal(err)
	}
	if err := assertHighVolumeQueryGuardrails(db.StorageModeHighVolume, &QueryRequest{
		Filters: []QueryFilter{{Field: "CreatedAt", Op: OpGte, Value: "2026-01-01"}},
	}, fields); err != nil {
		t.Fatal(err)
	}
	if err := assertHighVolumeQueryGuardrails(db.StorageModeHighVolume, &QueryRequest{
		Mode:    "locator",
		Filters: []QueryFilter{{Field: "ParentId", Op: OpEq, Value: "00000000-0000-4000-8000-000000000001"}},
	}, fields); err == nil {
		t.Fatal("expected locator without time bound to fail")
	}
	if err := assertHighVolumeQueryGuardrails(db.StorageModeHighVolume, &QueryRequest{
		Mode: "locator",
		Filters: []QueryFilter{
			{Field: "ParentId", Op: OpEq, Value: "00000000-0000-4000-8000-000000000001"},
			{Field: "CreatedAt", Op: OpGte, Value: "2026-01-01"},
		},
	}, fields); err != nil {
		t.Fatal(err)
	}
}

func TestLooksLikeUUID(t *testing.T) {
	if !looksLikeUUID("00000000-0000-4000-8000-000000000001") {
		t.Fatal("expected valid uuid")
	}
	if looksLikeUUID("not-a-uuid") {
		t.Fatal("expected invalid")
	}
}

func TestQuoteTable(t *testing.T) {
	if _, err := quoteTable("records"); err != nil {
		t.Fatal(err)
	}
	if _, err := quoteTable("records_hv"); err != nil {
		t.Fatal(err)
	}
	if _, err := quoteTable("evil"); err == nil {
		t.Fatal("expected reject")
	}
}
