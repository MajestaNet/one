package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MajestaNet/ide/internal/datapack"
)

func cmdDatapack(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: one datapack <validate|apply> ...\n")
		os.Exit(2)
	}
	switch args[0] {
	case "validate":
		cmdDatapackValidate(args[1:])
	case "apply":
		cmdDatapackApply(args[1:])
	default:
		fatal(fmt.Errorf("unknown datapack subcommand %q", args[0]))
	}
}

func cmdDatapackValidate(args []string) {
	fs := flag.NewFlagSet("datapack validate", flag.ExitOnError)
	dir := fs.String("dir", ".", "customer repo root (for environments/)")
	_ = fs.Parse(args)
	if fs.NArg() < 1 {
		fatal(fmt.Errorf("usage: one datapack validate <packDir> [-dir repoRoot]"))
	}
	packPath := fs.Arg(0)
	repoRoot := absDir(*dir)
	m, packDir, err := datapack.LoadManifest(packPath)
	if err != nil {
		fatal(err)
	}
	if errs := datapack.Validate(m, packDir, repoRoot); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "error: %v\n", e)
		}
		os.Exit(1)
	}
	fmt.Printf("datapack %s ok (%d steps, sourceEnv=%s)\n", m.Name, len(m.Steps), m.SourceEnv)
}

func cmdDatapackApply(args []string) {
	fs := flag.NewFlagSet("datapack apply", flag.ExitOnError)
	dir := fs.String("dir", ".", "customer repo root")
	alias := fs.String("alias", "", "target org alias")
	sourceAlias := fs.String("source-alias", "", "source org alias (Connected App / auth login)")
	offline := fs.Bool("offline", false, "use step file: only (no peer pull)")
	baseURL := fs.String("base-url", "", "override target base URL")
	token := fs.String("token", "", "override target token")
	apiKey := fs.String("api-key", "", "override target API key")
	_ = fs.Parse(args)
	if fs.NArg() < 1 {
		fatal(fmt.Errorf("usage: one datapack apply <packDir> --alias <target> [--source-alias <src>] [--offline]"))
	}
	packPath := fs.Arg(0)
	repoRoot := absDir(*dir)
	m, packDir, err := datapack.LoadManifest(packPath)
	if err != nil {
		fatal(err)
	}
	if errs := datapack.Validate(m, packDir, repoRoot); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "error: %v\n", e)
		}
		os.Exit(1)
	}

	target, err := resolveOrgAuth(*alias, *baseURL, *token, *apiKey)
	if err != nil {
		fatal(err)
	}
	opts := datapack.ApplyOptions{
		RepoRoot: repoRoot,
		PackDir:  packDir,
		Offline:  *offline,
		Target: &datapack.OrgClient{
			BaseURL:        target.BaseURL,
			Bearer:         target.bearer(),
			ApiRevisionPin: target.ApiRevisionPin,
		},
		OnStep: func(st datapack.Step, pulled, upserted, failed int) {
			fmt.Printf("step %s (%s): pulled=%d upserted=%d failed=%d\n", st.ID, st.Object, pulled, upserted, failed)
		},
	}
	if !*offline {
		srcAlias := *sourceAlias
		if srcAlias == "" && m.SourceEnv != "" {
			// Prefer an auth alias matching sourceEnv when present.
			srcAlias = m.SourceEnv
		}
		if srcAlias == "" {
			fatal(fmt.Errorf("--source-alias required unless --offline (pack sourceEnv=%q)", m.SourceEnv))
		}
		src, err := resolveOrgAuth(srcAlias, "", "", "")
		if err != nil {
			fatal(fmt.Errorf("source org: %w", err))
		}
		// Prefer environments/*.yaml baseUrl when alias base differs? Use saved alias URL (Connected App).
		_ = filepath.Separator
		opts.Source = &datapack.OrgClient{
			BaseURL:        src.BaseURL,
			Bearer:         src.bearer(),
			ApiRevisionPin: src.ApiRevisionPin,
		}
		fmt.Printf("source=%s (%s) → target=%s (%s)\n", srcAlias, src.BaseURL, target.Alias, target.BaseURL)
	}

	report, err := datapack.Apply(m, opts)
	if err != nil {
		if report != nil {
			_ = json.NewEncoder(os.Stdout).Encode(report)
		}
		fatal(err)
	}
	_ = json.NewEncoder(os.Stdout).Encode(report)
}
