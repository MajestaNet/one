package main

import (
	"strings"
	"testing"
)

func TestParseCoverProfileAndEvaluate(t *testing.T) {
	src := `mode: set
github.com/MajestaNet/ide/internal/foo/a.go:1.1,2.2 10 1
github.com/MajestaNet/ide/internal/foo/b.go:3.1,4.2 10 0

`
	percent, err := evaluateCoverage(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if percent < 49.9 || percent > 50.1 {
		t.Fatalf("percent=%v", percent)
	}
	if _, err := evaluateCoverage(strings.NewReader("mode: set\n")); err == nil {
		t.Fatal("expected empty profile error")
	}
	if _, err := evaluateCoverage(strings.NewReader("not a cover line\n")); err == nil {
		t.Fatal("expected invalid line")
	}
	if _, err := evaluateCoverage(strings.NewReader("file.go:1.1,2.2 x 1\n")); err == nil {
		t.Fatal("expected invalid counters")
	}
}
