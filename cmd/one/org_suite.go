package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// writeSuiteReport prints a complete --suite result (pretty JSON, never truncated)
// and returns true when the operator should treat the run as failed.
func writeSuiteReport(w io.Writer, suite string, suiteBody []byte, suiteStatus int) bool {
	fmt.Fprintf(w, "suite %s HTTP %d\n", suite, suiteStatus)
	var parsed any
	if err := json.Unmarshal(suiteBody, &parsed); err == nil {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(parsed)
	} else {
		fmt.Fprintln(w, string(suiteBody))
	}

	var report struct {
		Status string `json:"status"`
		Run    *struct {
			ID     string  `json:"id"`
			Status string  `json:"status"`
			Error  *string `json:"error"`
		} `json:"run"`
	}
	_ = json.Unmarshal(suiteBody, &report)
	runID := ""
	runStatus := strings.ToLower(strings.TrimSpace(report.Status))
	if report.Run != nil {
		runID = report.Run.ID
		if s := strings.ToLower(strings.TrimSpace(report.Run.Status)); s != "" {
			runStatus = s
		}
	}
	if runID != "" {
		fmt.Fprintf(w, "API of record: GET /deploy/v1/tests/runs/%s\n", runID)
	}
	if suiteStatus >= 300 {
		return true
	}
	switch runStatus {
	case "failed", "error":
		return true
	}
	return false
}
