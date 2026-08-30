package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/MajestaNet/ide/internal/customerrepo"
)

func cmdProject(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: one project <init>\n")
		os.Exit(2)
	}
	switch args[0] {
	case "init":
		cmdProjectInit(args[1:])
	default:
		fatal(fmt.Errorf("unknown project subcommand %q", args[0]))
	}
}

func cmdProjectInit(args []string) {
	fs := flag.NewFlagSet("project init", flag.ExitOnError)
	dir := fs.String("dir", ".", "target directory")
	customerID := fs.String("customer-id", "", "customerId for one.yaml")
	fromOrg := fs.Bool("from-org", false, "after scaffold, retrieve from connected org")
	force := fs.Bool("force", false, "allow writing into a non-empty directory")
	alias := fs.String("alias", "", "org alias when --from-org")
	baseURL := fs.String("base-url", "", "base URL when --from-org")
	token := fs.String("token", "", "JWT when --from-org")
	apiKey := fs.String("api-key", "", "API key when --from-org")
	_ = fs.Parse(args)

	abs := absDir(*dir)
	if err := customerrepo.InitProject(abs, *customerID, *force); err != nil {
		fatal(err)
	}
	fmt.Printf("initialized one/v1 at %s\n", abs)

	if *fromOrg {
		auth, err := resolveOrgAuth(*alias, *baseURL, *token, *apiKey)
		if err != nil {
			fatal(err)
		}
		raw, status, err := orgGET(auth, "/deploy/v1/packages/export")
		if err != nil {
			fatal(err)
		}
		if status >= 300 {
			fatal(fmt.Errorf("export: HTTP %d: %s", status, truncate(string(raw), 400)))
		}
		if err := customerrepo.ExtractZipToDir(bytes.NewReader(raw), int64(len(raw)), abs); err != nil {
			fatal(err)
		}
		fmt.Printf("retrieved org export into %s\n", abs)
	}
}

func cmdChange(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: one change <create>\n")
		os.Exit(2)
	}
	switch args[0] {
	case "create":
		cmdChangeCreate(args[1:])
	default:
		fatal(fmt.Errorf("unknown change subcommand %q", args[0]))
	}
}

func cmdChangeCreate(args []string) {
	fs := flag.NewFlagSet("change create", flag.ExitOnError)
	dir := fs.String("dir", ".", "customer repo root")
	title := fs.String("title", "", "change title")
	summary := fs.String("summary", "", "change summary")
	risk := fs.String("risk", "low", "risk level")
	branch := fs.Bool("branch", true, "create git branch change/<slug> when .git exists")
	_ = parseFlagSet(fs, args)
	if fs.NArg() < 1 {
		fatal(fmt.Errorf("usage: one change create <slug> [--title ...] [--summary ...]"))
	}
	slug := fs.Arg(0)
	meta := customerrepo.ChangeMeta{
		Title:   *title,
		Summary: *summary,
		Risk:    *risk,
	}
	p, err := customerrepo.CreateChange(*dir, slug, meta)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("wrote %s\n", p)
	if !*branch {
		return
	}
	abs := absDir(*dir)
	if _, err := os.Stat(filepath.Join(abs, ".git")); err != nil {
		fmt.Fprintf(os.Stderr, "note: no .git — create branch change/%s manually\n", strings.TrimPrefix(slug, "change/"))
		return
	}
	name := "change/" + strings.TrimPrefix(slug, "change/")
	cmd := exec.Command("git", "-C", abs, "checkout", "-b", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: git branch: %v (%s)\n", err, truncate(string(out), 200))
		return
	}
	fmt.Printf("checked out %s\n", name)
}
