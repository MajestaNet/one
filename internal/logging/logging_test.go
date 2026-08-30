package logging

import "testing"

func TestSetupLevels(t *testing.T) {
	for _, level := range []string{"debug", "warn", "warning", "error", "INFO", "  ", "nonsense"} {
		Setup(level)
	}
}
