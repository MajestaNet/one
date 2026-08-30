// Command check-go-coverage enforces a minimum aggregate statement coverage.
// It intentionally parses Go's stable coverprofile format using only the
// standard library so the CI guard introduces no runtime or tool dependency.
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: check-go-coverage <coverprofile> <minimum-percent>")
		os.Exit(2)
	}
	minimum, err := strconv.ParseFloat(os.Args[2], 64)
	if err != nil || minimum < 0 || minimum > 100 {
		fmt.Fprintln(os.Stderr, "minimum-percent must be between 0 and 100")
		os.Exit(2)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "open coverage profile: %v\n", err)
		os.Exit(2)
	}
	defer func() { _ = f.Close() }()

	percent, err := evaluateCoverage(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
	}
	fmt.Printf("Go statement coverage: %.1f%% (minimum %.1f%%)\n", percent, minimum)
	if percent+1e-9 < minimum {
		os.Exit(1)
	}
}

func evaluateCoverage(r io.Reader) (float64, error) {
	total, covered, err := parseCoverProfile(r)
	if err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, fmt.Errorf("coverage profile contains no statements")
	}
	return float64(covered) * 100 / float64(total), nil
}
