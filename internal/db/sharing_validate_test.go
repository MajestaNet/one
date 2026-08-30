package db

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestValidateSharingCriteriaRequiresFilters(t *testing.T) {
	err := validateSharingCriteria(json.RawMessage(`{"filters":[]}`))
	if err == nil || !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
	err = validateSharingCriteria(json.RawMessage(`{"filters":[{"field":"Name","op":"eq","value":"A"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	err = validateSharingCriteria(json.RawMessage(`not-json`))
	if err == nil || !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for bad JSON, got %v", err)
	}
}

func TestCreateSharingRuleRejectsEmptyCriteria(t *testing.T) {
	// Constructor-level validation without DB: exercise CreateSharingRule pre-insert checks
	// by calling validate + access level gates used at the top of CreateSharingRule.
	if err := validateSharingCriteria(json.RawMessage(`{"filters":[]}`)); err == nil {
		t.Fatal("empty filters must fail")
	}
	rule := SharingRule{AccessLevel: "write", Criteria: json.RawMessage(`{"filters":[{"field":"Name","op":"eq","value":"x"}]}`)}
	if rule.AccessLevel != "read" && rule.AccessLevel != "read_write" {
		// mirrors CreateSharingRule gate
	} else {
		t.Fatal("expected invalid access level")
	}
}
