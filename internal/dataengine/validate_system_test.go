package dataengine_test

import (
	"errors"
	"testing"

	"github.com/MajestaNet/ide/internal/dataengine"
)

func TestRejectImmutableSystemFields(t *testing.T) {
	if err := dataengine.RejectImmutableSystemFields(map[string]any{"Name": "x"}); err != nil {
		t.Fatal(err)
	}
	// OwnerId is allowed (optional assignment)
	if err := dataengine.RejectImmutableSystemFields(map[string]any{"OwnerId": "u1"}); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"CreatedById", "LastModifiedById", "CreatedAt", "UpdatedAt", "Id"} {
		err := dataengine.RejectImmutableSystemFields(map[string]any{field: "x"})
		var ve *dataengine.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("%s: expected ValidationError, got %v", field, err)
		}
	}
}
