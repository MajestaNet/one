// Command one — product CLI for one/v1 repos (CI + local DX).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "pack":
		cmdPack(os.Args[2:])
	case "unpack":
		cmdUnpack(os.Args[2:])
	case "validate":
		cmdValidate(os.Args[2:])
	case "org":
		cmdOrg(os.Args[2:])
	case "auth":
		cmdAuth(os.Args[2:])
	case "project":
		cmdProject(os.Args[2:])
	case "change":
		cmdChange(os.Args[2:])
	case "datapack":
		cmdDatapack(os.Args[2:])
	case "version", "--version", "-version":
		printCliCompatVersion()
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `one — customer DX CLI for one/v1

Usage:
  one auth login --base-url <url> (--token|--api-key) <cred> [--alias name]
  one auth logout [--alias name]
  one org list
  one org use <alias>
  one project init [-dir path] [--customer-id id] [--from-org] [--force]
  one change create <slug> [-dir path] [--title ...] [--summary ...]
  one pack -dir <path> [-out artifact.json] [--metadata path ...] [--manifest name]
  one unpack -artifact artifact.json -dir <path>
  one validate -dir <path> [--metadata path ...] [--manifest name]
  one org validate -dir <path> [--alias name] [--base-url ...] [--token|--api-key ...] [--metadata ...] [--manifest ...]
  one org deploy  -dir <path> [auth flags] [--suite name] [--skip-validate] [--dry-run] [--metadata ...] [--manifest ...]
  one org retrieve -dir <path> [auth flags] [--force] [--baseline-only]
  one datapack validate <packDir> [-dir repoRoot]
  one datapack apply <packDir> --alias <target> [--source-alias <src>] [--offline] [-dir repoRoot]
  one version

Config: ~/.config/one/config.json (+ OS keychain, else credentials.json mode 0600)
Env:    ONE_ORG, ONE_BASE_URL, ONE_TOKEN, ONE_API_KEY
        ONE_CREDENTIAL_STORE=auto|file|keychain (default auto)

Peer-sourced data packs (BP-041): store sourceEnv → environments/<role>.yaml in the
customer repo; apply pulls from the source Connected App / auth alias into the target.
`)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// multiString collects repeated --metadata flags.
type multiString []string

func (m *multiString) String() string { return strings.Join(*m, ",") }
func (m *multiString) Set(v string) error {
	*m = append(*m, v)
	return nil
}

func addPackFlags(fs *flag.FlagSet, paths *multiString, manifest *string) {
	fs.Var(paths, "metadata", "path prefix under metadata/, src/, or tests/ (repeatable)")
	fs.StringVar(manifest, "manifest", "", "manifests/<name>.yaml selective path list")
}

func absDir(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		fatal(err)
	}
	return abs
}
