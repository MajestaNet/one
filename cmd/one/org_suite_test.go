package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteSuiteReportPrettyPrintsAndDetectsFailure(t *testing.T) {
	body := []byte(`{"run":{"id":"run-1","status":"failed","suiteApiName":"CreateAccountFromContact","results":[{"index":8,"type":"automationContract","status":"failed","message":"contract fixture create: Opportunity requires AccountId and/or ContactId","detail":{"objectApiName":"Opportunity"}}]},"mode":"sync"}`)
	var buf bytes.Buffer
	failed := writeSuiteReport(&buf, "CreateAccountFromContact", body, 201)
	out := buf.String()
	if !failed {
		t.Fatal("failed suite status must be treated as failure")
	}
	if strings.Contains(out, "…") {
		t.Fatalf("must not truncate step detail: %s", out)
	}
	if !strings.Contains(out, `"objectApiName": "Opportunity"`) {
		t.Fatalf("expected full objectApiName in pretty JSON: %s", out)
	}
	if !strings.Contains(out, "GET /deploy/v1/tests/runs/run-1") {
		t.Fatalf("expected API of record pointer: %s", out)
	}
	if !strings.Contains(out, "suite CreateAccountFromContact HTTP 201") {
		t.Fatalf("expected status line: %s", out)
	}
}

func TestWriteSuiteReportPassedIsNotFailure(t *testing.T) {
	body := []byte(`{"run":{"id":"run-ok","status":"passed"}}`)
	var buf bytes.Buffer
	if writeSuiteReport(&buf, "OkSuite", body, 201) {
		t.Fatalf("passed suite should not fail: %s", buf.String())
	}
}

func TestWriteSuiteReportHTTPErrorIsFailure(t *testing.T) {
	var buf bytes.Buffer
	if !writeSuiteReport(&buf, "Broken", []byte(`{"error":"nope"}`), 500) {
		t.Fatal("HTTP 500 must fail")
	}
}
