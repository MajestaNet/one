package metadata

import (
	"fmt"
	"unicode/utf8"
)

// MaxAutomationDescriptionRunes is the Unicode length cap for metadata_automations.description.
const MaxAutomationDescriptionRunes = 500

// ValidateAutomationDescription enforces the Unicode-safe description length (ADR-033).
func ValidateAutomationDescription(description string) error {
	if utf8.RuneCountInString(description) > MaxAutomationDescriptionRunes {
		return fmt.Errorf("%w: description must be at most %d characters", ErrValidation, MaxAutomationDescriptionRunes)
	}
	return nil
}
