package metadata_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/metadata"
)

func TestValidateAutomationDescription(t *testing.T) {
	if err := metadata.ValidateAutomationDescription(""); err != nil {
		t.Fatalf("empty: %v", err)
	}
	if err := metadata.ValidateAutomationDescription(strings.Repeat("a", 500)); err != nil {
		t.Fatalf("500 ascii: %v", err)
	}
	if err := metadata.ValidateAutomationDescription(strings.Repeat("a", 501)); err == nil || !errors.Is(err, metadata.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
	// Unicode: 501 runes should fail even when byte length differs.
	if err := metadata.ValidateAutomationDescription(strings.Repeat("é", 501)); err == nil || !errors.Is(err, metadata.ErrValidation) {
		t.Fatalf("expected unicode length error, got %v", err)
	}
	if err := metadata.ValidateAutomationDescription(strings.Repeat("é", 500)); err != nil {
		t.Fatalf("500 unicode runes: %v", err)
	}
}
