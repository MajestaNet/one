package dataengine_test

import (
	"testing"

	"github.com/MajestaNet/ide/internal/dataengine"
)

func TestAssertSafeFieldName(t *testing.T) {
	ok := []string{"Name", "OwnerId", "CreatedById", "LastModifiedById", "Custom_Field", "a1"}
	for _, f := range ok {
		if _, err := dataengine.AssertSafeFieldName(f); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
	}
	bad := []string{"", "1Name", "Name;drop", "Name'", "Name space", "Name-1"}
	for _, f := range bad {
		if _, err := dataengine.AssertSafeFieldName(f); err == nil {
			t.Fatalf("expected rejection for %q", f)
		}
	}
}
