package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/deploy"
)

func TestWriteValidateSummaryHighlightsActionable(t *testing.T) {
	var buf bytes.Buffer
	if err := writeValidateSummary(&buf, &deploy.ValidateLocalResult{
		OK:       true,
		Checksum: "abc",
		BundleID: "b1",
		Diff: &deploy.DiffReport{
			Entries: []deploy.DiffEntry{
				{Kind: deploy.DiffAdd, Path: "objects.Referral__c"},
				{Kind: deploy.DiffRemove, Path: "agentPlaybooks.RunCoach"},
				{Kind: deploy.DiffBaseline, Path: "baseline.objects.Account"},
			},
			Counts: struct {
				Add      int `json:"add"`
				Change   int `json:"change"`
				Remove   int `json:"remove"`
				Baseline int `json:"baseline"`
			}{Add: 1, Remove: 1, Baseline: 1},
		},
	}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "ok=true") || !strings.Contains(out, "+1 add") {
		t.Fatalf("summary=%s", out)
	}
	if !strings.Contains(out, "+ objects.Referral__c") {
		t.Fatalf("missing actionable path: %s", out)
	}
	if strings.Contains(out, "agentPlaybooks.RunCoach") || strings.Contains(out, "baseline.objects.Account") {
		t.Fatalf("informational rows should stay off the path list: %s", out)
	}
	if !strings.Contains(out, "not deleted in v1") {
		t.Fatalf("expected delete-by-absence note: %s", out)
	}
}
