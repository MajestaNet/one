package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func parseCoverProfile(r io.Reader) (total, covered uint64, err error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return 0, 0, fmt.Errorf("invalid coverage profile line: %q", line)
		}
		statements, statErr := strconv.ParseUint(fields[1], 10, 64)
		count, countErr := strconv.ParseUint(fields[2], 10, 64)
		if statErr != nil || countErr != nil {
			return 0, 0, fmt.Errorf("invalid coverage counters: %q", line)
		}
		total += statements
		if count > 0 {
			covered += statements
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, fmt.Errorf("read coverage profile: %w", err)
	}
	return total, covered, nil
}
