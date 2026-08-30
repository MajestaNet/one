package automation_test

import (
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/automation"
)

func TestValidateSourceImports_AllowsOneAndBare(t *testing.T) {
	ok := []string{
		`export default async function run(ctx) { return { ok: true }; }`,
		`import type { AutomationContext } from "one:automation";
export default async function run(ctx: AutomationContext) { return { ok: true }; }`,
		`import { log } from "one:automation";
export default async function run(ctx) { log("hi"); return { ok: true }; }`,
		`// import "npm:lodash"
export default async function run(ctx) { return { ok: true }; }`,
	}
	for _, src := range ok {
		if err := automation.ValidateSourceImports("src/automations/ok.ts", src); err != nil {
			t.Fatalf("unexpected error for %q: %v", src, err)
		}
	}
}

func TestValidateSourceImports_RejectsForbidden(t *testing.T) {
	bad := []string{
		`import _ from "npm:lodash";`,
		`import "jsr:@std/path";`,
		`import x from "https://example.com/x.ts";`,
		`import { join } from "std/path/mod.ts";`,
		`const x = require("fs");`,
		`const m = await import("npm:axios");`,
		`import "./other.ts";`,
		`eval("1")`,
	}
	for _, src := range bad {
		src = src + "\nexport default async function run(ctx) { return { ok: true }; }\n"
		err := automation.ValidateSourceImports("src/automations/bad.ts", src)
		if err == nil {
			t.Fatalf("expected error for %q", src)
		}
	}
}

func TestValidateDefinition(t *testing.T) {
	runAs := "00000000-0000-4000-8000-000000000099"
	entry := "src/automations/create_opp.ts"
	src := `export default async function run(ctx) { return { ok: true }; }`

	if err := automation.ValidateDefinition("A", "code", "async", "create", &entry, &src, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := automation.ValidateDefinition("A", "code", "async", "schedule", &entry, &src, nil, nil); err == nil {
		t.Fatal("expected schedule without runAs to fail")
	}
	if err := automation.ValidateDefinition("A", "code", "async", "schedule", &entry, &src, &runAs, nil); err != nil {
		t.Fatal(err)
	}
	if err := automation.ValidateDefinition("A", "code", "async", "create", nil, nil, nil, nil); err == nil {
		t.Fatal("expected code without source/entryFile to fail")
	}
	badEntry := "lib/foo.ts"
	if err := automation.ValidateDefinition("A", "code", "async", "create", &badEntry, &src, nil, nil); err == nil || !strings.Contains(err.Error(), "src/automations/") {
		t.Fatalf("expected entryFile path error, got %v", err)
	}
	actions := []any{map[string]any{"type": "http", "url": "https://x"}}
	if err := automation.ValidateDefinition("A", "actions", "sync", "create", nil, nil, nil, actions); err == nil {
		t.Fatal("expected sync+http to fail")
	}
	npm := `import x from "npm:lodash"; export default async function run(ctx){return{ok:true}}`
	if err := automation.ValidateDefinition("A", "code", "async", "create", &entry, &npm, nil, nil); err == nil {
		t.Fatal("expected npm import to fail")
	}
}
