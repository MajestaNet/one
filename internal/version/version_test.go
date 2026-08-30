package version

import "testing"

func TestVersionIsSet(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must be non-empty (ldflags override in release builds)")
	}
}
