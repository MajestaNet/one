package main

import (
	"flag"
	"testing"
)

func TestParseFlagSetAllowsFlagsAfterSlug(t *testing.T) {
	fs := flag.NewFlagSet("change create", flag.ContinueOnError)
	title := fs.String("title", "", "")
	summary := fs.String("summary", "", "")
	risk := fs.String("risk", "low", "")
	if err := parseFlagSet(fs, []string{"referral-notes", "--title", "Add notes", "--summary", "field", "--risk", "medium"}); err != nil {
		t.Fatal(err)
	}
	if fs.NArg() != 1 || fs.Arg(0) != "referral-notes" {
		t.Fatalf("args=%v", fs.Args())
	}
	if *title != "Add notes" || *summary != "field" || *risk != "medium" {
		t.Fatalf("title=%q summary=%q risk=%q", *title, *summary, *risk)
	}
}

func TestParseFlagSetFlagsBeforeSlug(t *testing.T) {
	fs := flag.NewFlagSet("change create", flag.ContinueOnError)
	title := fs.String("title", "", "")
	if err := parseFlagSet(fs, []string{"--title", "Hello", "my-slug"}); err != nil {
		t.Fatal(err)
	}
	if fs.Arg(0) != "my-slug" || *title != "Hello" {
		t.Fatalf("arg=%q title=%q", fs.Arg(0), *title)
	}
}
