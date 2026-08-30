package main

import (
	"fmt"
	"io"

	"github.com/MajestaNet/ide/internal/deploy"
)

const validateSummaryActionableLimit = 8

func writeValidateSummary(w io.Writer, result *deploy.ValidateLocalResult) error {
	if result == nil {
		return nil
	}
	status := "ok=false"
	if result.OK {
		status = "ok=true"
	}
	if _, err := fmt.Fprintf(w, "org validate %s checksum=%s bundleId=%s\n", status, result.Checksum, result.BundleID); err != nil {
		return err
	}
	if result.Diff == nil {
		return nil
	}
	c := result.Diff.Counts
	if _, err := fmt.Fprintf(w, "  actionable: +%d add, ~%d change\n", c.Add, c.Change); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  informational: -%d remove (not deleted in v1), %d baseline\n", c.Remove, c.Baseline); err != nil {
		return err
	}
	n := 0
	for _, e := range result.Diff.Entries {
		if e.Kind != deploy.DiffAdd && e.Kind != deploy.DiffChange {
			continue
		}
		if n >= validateSummaryActionableLimit {
			if _, err := fmt.Fprintln(w, "  …"); err != nil {
				return err
			}
			break
		}
		mark := "+"
		if e.Kind == deploy.DiffChange {
			mark = "~"
		}
		if _, err := fmt.Fprintf(w, "  %s %s\n", mark, e.Path); err != nil {
			return err
		}
		n++
	}
	return nil
}
