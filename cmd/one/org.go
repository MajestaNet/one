package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/customerrepo"
	"github.com/MajestaNet/ide/internal/deploy"
)

func cmdOrg(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: one org <list|use|validate|deploy|retrieve> ...\n")
		os.Exit(2)
	}
	switch args[0] {
	case "list":
		cmdOrgList(args[1:])
	case "use":
		cmdOrgUse(args[1:])
	case "validate":
		cmdOrgValidate(args[1:])
	case "deploy":
		cmdOrgDeploy(args[1:])
	case "retrieve":
		cmdOrgRetrieve(args[1:])
	default:
		fatal(fmt.Errorf("unknown org subcommand %q", args[0]))
	}
}

func cmdOrgList(args []string) {
	_ = flag.NewFlagSet("org list", flag.ExitOnError).Parse(args)
	cfg, _, err := loadConfig()
	if err != nil {
		fatal(err)
	}
	if len(cfg.Orgs) == 0 {
		fmt.Println("(no saved orgs — run auth login)")
		return
	}
	for alias, o := range cfg.Orgs {
		mark := " "
		if alias == cfg.DefaultOrg {
			mark = "*"
		}
		fmt.Printf("%s %-16s %s\n", mark, alias, o.BaseURL)
	}
}

func cmdOrgUse(args []string) {
	fs := flag.NewFlagSet("org use", flag.ExitOnError)
	_ = fs.Parse(args)
	if fs.NArg() < 1 {
		fatal(fmt.Errorf("usage: one org use <alias>"))
	}
	alias := fs.Arg(0)
	cfg, cred, err := loadConfig()
	if err != nil {
		fatal(err)
	}
	if _, ok := cfg.Orgs[alias]; !ok {
		fatal(fmt.Errorf("unknown alias %q — auth login first", alias))
	}
	cfg.DefaultOrg = alias
	if err := saveConfig(cfg, cred); err != nil {
		fatal(err)
	}
	fmt.Printf("default org = %s (%s)\n", alias, cfg.Orgs[alias].BaseURL)
}

type orgFlags struct {
	dir      string
	alias    string
	baseURL  string
	token    string
	apiKey   string
	paths    multiString
	manifest string
	auth     *resolvedOrg
}

func parseOrgFlags(name string, args []string, extra func(*flag.FlagSet)) (*orgFlags, *flag.FlagSet) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	o := &orgFlags{}
	fs.StringVar(&o.dir, "dir", ".", "customer repo root")
	fs.StringVar(&o.alias, "alias", "", "saved org alias")
	fs.StringVar(&o.baseURL, "base-url", "", "install API base URL")
	fs.StringVar(&o.token, "token", "", "Majesta One JWT (Bearer)")
	fs.StringVar(&o.apiKey, "api-key", "", "API key (Bearer)")
	addPackFlags(fs, &o.paths, &o.manifest)
	if extra != nil {
		extra(fs)
	}
	if err := parseFlagSet(fs, args); err != nil {
		fatal(err)
	}
	return o, fs
}

func (o *orgFlags) resolve() {
	auth, err := resolveOrgAuth(o.alias, o.baseURL, o.token, o.apiKey)
	if err != nil {
		fatal(err)
	}
	o.auth = auth
}

func cmdOrgValidate(args []string) {
	o, _ := parseOrgFlags("org validate", args, nil)
	o.resolve()
	result := orgValidate(o)
	if err := writeValidateSummary(os.Stderr, result); err != nil {
		fatal(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
	if !result.OK {
		os.Exit(1)
	}
}

func cmdOrgDeploy(args []string) {
	var suite string
	var skipValidate bool
	var dryRun bool
	o, _ := parseOrgFlags("org deploy", args, func(fs *flag.FlagSet) {
		fs.StringVar(&suite, "suite", "", "optional customer test suite API name to run after apply")
		fs.BoolVar(&skipValidate, "skip-validate", false, "break-glass: skip org validate (discouraged)")
		fs.BoolVar(&dryRun, "dry-run", false, "validate path only / promote dryRun")
	})
	o.resolve()

	var bundleID, checksum string
	var packedMan *customerrepo.Manifest
	if skipValidate {
		fmt.Fprintln(os.Stderr, "warning: --skip-validate is break-glass; prefer org validate first")
		art, man, err := customerrepo.PackFromDir(o.dir, packOpts(o.paths, o.manifest))
		if err != nil {
			fatal(err)
		}
		packedMan = man
		label := fmt.Sprintf("cli-deploy-%d", time.Now().Unix())
		body, status, err := orgPOST(o.auth, "/deploy/v1/bundles", map[string]any{
			"label":    label,
			"artifact": art,
		})
		if err != nil {
			fatal(err)
		}
		if status >= 300 {
			fatal(fmt.Errorf("create bundle: HTTP %d: %s", status, body))
		}
		var row struct {
			ID       string `json:"id"`
			Checksum string `json:"checksum"`
		}
		if err := json.Unmarshal(body, &row); err != nil || row.ID == "" {
			fatal(fmt.Errorf("create bundle response: %s", body))
		}
		bundleID = row.ID
		if row.Checksum != "" {
			fmt.Fprintf(os.Stderr, "skip-validate pack checksum=%s bundleId=%s\n", row.Checksum, bundleID)
		}
	} else {
		result := orgValidate(o)
		if !result.OK {
			if err := writeValidateSummary(os.Stderr, result); err != nil {
				fatal(err)
			}
			enc := json.NewEncoder(os.Stderr)
			enc.SetIndent("", "  ")
			_ = enc.Encode(result)
			fatal(fmt.Errorf("org validate failed; refusing deploy"))
		}
		bundleID, checksum = result.BundleID, result.Checksum
		fmt.Fprintf(os.Stderr, "validated checksum=%s bundleId=%s\n", checksum, bundleID)
		_, man, err := customerrepo.PackFromDir(o.dir, packOpts(o.paths, o.manifest))
		if err == nil {
			packedMan = man
		}
	}
	if suite == "" && packedMan != nil && len(packedMan.RequiredTestSuites) > 0 {
		suite = packedMan.RequiredTestSuites[0]
	}

	body, status, err := orgPOST(o.auth, "/deploy/v1/promotions", map[string]any{
		"bundleId": bundleID,
		"dryRun":   dryRun,
	})
	if err != nil {
		fatal(err)
	}
	if status == http.StatusAccepted {
		body, err = awaitOrgWork(o.auth, body)
		if err != nil {
			fatal(err)
		}
		status = http.StatusCreated
	}
	_, _ = os.Stdout.Write(body)
	_, _ = os.Stdout.Write([]byte("\n"))
	if status >= 300 {
		os.Exit(1)
	}
	var applied deploy.PromoteBundleResult
	if err := json.Unmarshal(body, &applied); err == nil && applied.Promotion != nil && applied.Promotion.Status == "failed" {
		os.Exit(1)
	}

	if suite == "" {
		return
	}
	if dryRun {
		fmt.Fprintf(os.Stderr, "note: skipping --suite %s on dry-run (run after a real apply)\n", suite)
		return
	}
	suiteBody, suiteStatus, suiteErr := orgPOST(o.auth, "/deploy/v1/tests/runs", map[string]any{
		"suiteApiName": suite,
	})
	if suiteErr != nil {
		fmt.Fprintf(os.Stderr, "pack applied; suite %s failed: %v\n", suite, suiteErr)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "suite %s: %s\n", suite, truncate(string(suiteBody), 200))
	if suiteStatus >= 300 {
		fmt.Fprintf(os.Stderr, "pack applied; suite %s failed: HTTP %d\n", suite, suiteStatus)
		os.Exit(1)
	}
}

func cmdOrgRetrieve(args []string) {
	var force bool
	var baselineOnly bool
	o, _ := parseOrgFlags("org retrieve", args, func(fs *flag.FlagSet) {
		fs.BoolVar(&force, "force", false, "overwrite even if working tree is dirty")
		fs.BoolVar(&baselineOnly, "baseline-only", false, "refresh .one/baseline only (leave customer YAML)")
	})
	o.resolve()
	dir := absDir(o.dir)
	if !force {
		if dirty, err := gitDirty(dir); err != nil {
			fatal(err)
		} else if dirty {
			fatal(fmt.Errorf("working tree dirty under %s — commit/stash or pass --force", dir))
		}
	}
	raw, status, err := orgGET(o.auth, "/deploy/v1/packages/export")
	if err != nil {
		fatal(err)
	}
	if status >= 300 {
		fatal(fmt.Errorf("export: HTTP %d: %s", status, truncate(string(raw), 400)))
	}
	if baselineOnly {
		if err := customerrepo.ExtractBaselineFromZip(bytes.NewReader(raw), int64(len(raw)), dir); err != nil {
			fatal(err)
		}
		fmt.Printf("refreshed .one/baseline under %s\n", dir)
		return
	}
	if err := customerrepo.ExtractZipToDir(bytes.NewReader(raw), int64(len(raw)), dir); err != nil {
		fatal(err)
	}
	fmt.Printf("retrieved install export into %s\n", dir)
}

func orgValidate(o *orgFlags) *deploy.ValidateLocalResult {
	art, _, err := customerrepo.PackFromDir(o.dir, packOpts(o.paths, o.manifest))
	if err != nil {
		fatal(err)
	}
	label := fmt.Sprintf("cli-validate-%d", time.Now().Unix())
	body, status, err := orgPOST(o.auth, "/deploy/v1/packages/validate-local?label="+label, map[string]any{
		"label":    label,
		"artifact": art,
	})
	if err != nil {
		fatal(err)
	}
	if status == http.StatusAccepted {
		body, err = awaitOrgWork(o.auth, body)
		if err != nil {
			fatal(err)
		}
		status = http.StatusOK
	}
	if status >= 300 {
		fatal(fmt.Errorf("validate-local: HTTP %d: %s", status, body))
	}
	var result deploy.ValidateLocalResult
	if err := json.Unmarshal(body, &result); err != nil {
		fatal(fmt.Errorf("decode validate-local: %w (%s)", err, body))
	}
	return &result
}

const orgWorkPollTimeout = 6 * time.Minute

func awaitOrgWork(auth *resolvedOrg, accepted []byte) ([]byte, error) {
	var handle struct {
		JobID  string `json:"jobId"`
		Poll   string `json:"poll"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(accepted, &handle); err != nil || handle.JobID == "" {
		return nil, fmt.Errorf("decode 202 handle: %w (%s)", err, accepted)
	}
	path := handle.Poll
	if path == "" {
		path = "/deploy/v1/work/" + handle.JobID
	}
	deadline := time.Now().Add(orgWorkPollTimeout)
	for time.Now().Before(deadline) {
		body, status, err := orgGET(auth, path)
		if err != nil {
			return nil, err
		}
		if status >= 300 {
			return nil, fmt.Errorf("poll %s: HTTP %d: %s", path, status, body)
		}
		var work struct {
			Status    string          `json:"status"`
			LastError *string         `json:"lastError"`
			Result    json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(body, &work); err != nil {
			return nil, fmt.Errorf("decode work: %w (%s)", err, body)
		}
		switch work.Status {
		case "completed":
			if len(work.Result) == 0 || string(work.Result) == "null" {
				return nil, fmt.Errorf("deploy work %s completed without a result", handle.JobID)
			}
			return work.Result, nil
		case "failed":
			msg := "deploy work failed"
			if work.LastError != nil && *work.LastError != "" {
				msg = *work.LastError
			}
			return nil, fmt.Errorf("%s", msg)
		}
		time.Sleep(250 * time.Millisecond)
	}
	return nil, fmt.Errorf("timed out waiting for deploy work %s", handle.JobID)
}

func orgPOST(auth *resolvedOrg, path string, payload any) ([]byte, int, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(auth.BaseURL, "/")+path, &buf)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+auth.bearer())
	req.Header.Set("Content-Type", "application/json")
	if auth.ApiRevisionPin > 0 {
		req.Header.Set("One-API-Revision", fmt.Sprintf("%d", auth.ApiRevisionPin))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	return b, resp.StatusCode, err
}

func orgGET(auth *resolvedOrg, path string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(auth.BaseURL, "/")+path, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+auth.bearer())
	if auth.ApiRevisionPin > 0 {
		req.Header.Set("One-API-Revision", fmt.Sprintf("%d", auth.ApiRevisionPin))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	return b, resp.StatusCode, err
}

func gitDirty(dir string) (bool, error) {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}
