package worker_test

import (
	"testing"

	"github.com/MajestaNet/ide/internal/worker"
)

func TestToolAllowedAndObjectInScope(t *testing.T) {
	tools := []string{"sobjects.read", "query"}
	if !worker.ToolAllowed("query", tools) {
		t.Fatal("expected query allowed")
	}
	if worker.ToolAllowed("sobjects.write", tools) {
		t.Fatal("write should not be allowed")
	}
	if !worker.ObjectInScope("Account", nil) {
		t.Fatal("empty scopes = all")
	}
	if !worker.ObjectInScope("Account", []string{"Account", "Contact"}) {
		t.Fatal("Account in scope")
	}
	if worker.ObjectInScope("Opportunity", []string{"Account"}) {
		t.Fatal("Opportunity out of scope")
	}
}
