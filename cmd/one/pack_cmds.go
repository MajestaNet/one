package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MajestaNet/ide/internal/customerrepo"
	"github.com/MajestaNet/ide/internal/deploy"
)

func packOpts(paths multiString, manifest string) customerrepo.PackOptions {
	return customerrepo.PackOptions{
		IncludePaths: []string(paths),
		ManifestName: manifest,
	}
}

func cmdPack(args []string) {
	fs := flag.NewFlagSet("pack", flag.ExitOnError)
	dir := fs.String("dir", ".", "customer repo root")
	out := fs.String("out", "", "write BundleArtifact JSON (default stdout)")
	var paths multiString
	var manifest string
	addPackFlags(fs, &paths, &manifest)
	_ = fs.Parse(args)
	art, _, err := customerrepo.PackFromDir(*dir, packOpts(paths, manifest))
	if err != nil {
		fatal(err)
	}
	b, err := json.MarshalIndent(art, "", "  ")
	if err != nil {
		fatal(err)
	}
	if *out == "" {
		_, _ = os.Stdout.Write(b)
		_, _ = os.Stdout.Write([]byte("\n"))
		return
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		fatal(err)
	}
}

func cmdUnpack(args []string) {
	fs := flag.NewFlagSet("unpack", flag.ExitOnError)
	artifact := fs.String("artifact", "", "BundleArtifact JSON path")
	dir := fs.String("dir", ".", "output directory")
	_ = fs.Parse(args)
	if *artifact == "" {
		fatal(fmt.Errorf("-artifact is required"))
	}
	raw, err := os.ReadFile(*artifact)
	if err != nil {
		fatal(err)
	}
	var art deploy.BundleArtifact
	if err := json.Unmarshal(raw, &art); err != nil {
		fatal(err)
	}
	man := customerrepo.Manifest{RepoFormat: customerrepo.RepoFormat}
	if art.CustomerID != nil {
		man.CustomerID = *art.CustomerID
	}
	if art.DefaultPackageName != "" {
		man.PackageName = art.DefaultPackageName
	}
	if art.ProductVersionRange != nil {
		man.ProductVersionRange = *art.ProductVersionRange
	}
	if err := customerrepo.UnpackToDir(*dir, &art, man); err != nil {
		fatal(err)
	}
	fmt.Printf("unpacked to %s\n", filepath.Clean(*dir))
}

func cmdValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	dir := fs.String("dir", ".", "customer repo root")
	var paths multiString
	var manifest string
	addPackFlags(fs, &paths, &manifest)
	_ = fs.Parse(args)
	art, _, err := customerrepo.PackFromDir(*dir, packOpts(paths, manifest))
	if err != nil {
		fatal(err)
	}
	raw, err := json.Marshal(art)
	if err != nil {
		fatal(err)
	}
	var asAny any
	if err := json.Unmarshal(raw, &asAny); err != nil {
		fatal(err)
	}
	parsed, err := deploy.ParseBundleArtifact(asAny)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("ok: objects=%d fields=%d rules=%d automations=%d playbooks=%d permissionSets=%d webhooks=%d canvases=%d experiences=%d tests=%d\n",
		len(parsed.Objects), len(parsed.Fields), len(parsed.ValidationRules), len(parsed.Automations),
		len(parsed.AgentPlaybooks), len(parsed.PermissionSets), len(parsed.Webhooks),
		len(parsed.Canvases), len(parsed.Experiences), len(parsed.Tests))
}
