package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/MajestaNet/ide/internal/compat"
	"github.com/MajestaNet/ide/internal/version"
)

const (
	cliPreferredApiRevision   = 1
	cliMinApiRevision         = 1
	cliTargetProductVersion   = "0.1.0"
	cliSupportedProductMinors = 2
)

func compatExit(code int, format string, args ...any) {
	fmt.Fprintf(os.Stderr, "compat: "+format+"\n", args...)
	os.Exit(code)
}

type versionProbe struct {
	ProductVersion string                   `json:"productVersion"`
	ApiRevision    compat.APIRevisionWindow `json:"apiRevision"`
}

func probeInstallVersion(baseURL string) (*versionProbe, error) {
	res, err := http.Get(strings.TrimRight(baseURL, "/") + "/version")
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("GET /version: HTTP %d: %s", res.StatusCode, string(b))
	}
	var probe versionProbe
	if err := json.Unmarshal(b, &probe); err != nil {
		return nil, fmt.Errorf("decode /version: %w", err)
	}
	window, err := compat.NormalizeWindow(probe.ApiRevision.Min, probe.ApiRevision.Current)
	if err != nil {
		return nil, fmt.Errorf("install apiRevision invalid: %w", err)
	}
	probe.ApiRevision = window
	return &probe, nil
}

func negotiateCliPin(baseURL string, forceCompat bool, explicitPin int) (int, *versionProbe, error) {
	probe, err := probeInstallVersion(baseURL)
	if err != nil {
		return 0, nil, err
	}
	window := probe.ApiRevision
	pin := explicitPin
	if pin > 0 {
		if !compat.PinInWindow(pin, window) {
			if forceCompat {
				return pin, probe, nil
			}
			return 0, probe, fmt.Errorf("pin %d outside install window [%d,%d]", pin, window.Min, window.Current)
		}
		return pin, probe, nil
	}
	selected, code, err := compat.SelectClientPin(cliMinApiRevision, cliPreferredApiRevision, window)
	if err != nil {
		if forceCompat {
			return window.Min, probe, nil
		}
		return 0, probe, fmt.Errorf("%s: %w", code, err)
	}
	status, hardCode := compat.EvaluateRevisionHard(selected, window, false)
	if status == "block" {
		if forceCompat {
			return selected, probe, nil
		}
		return 0, probe, fmt.Errorf("%s: revision pin %d rejected", hardCode, selected)
	}
	prodStatus, prodCode := compat.ProductTestedAgainst(probe.ProductVersion, cliTargetProductVersion, cliSupportedProductMinors)
	if prodStatus == "warn" && prodCode != "" {
		fmt.Fprintf(os.Stderr, "warning: product %s outside tested window (%s)\n", probe.ProductVersion, prodCode)
	}
	return selected, probe, nil
}

func printCliCompatVersion() {
	fmt.Printf(
		"one %s | preferredApiRevision=%d minApiRevision=%d targetProduct=%s testedMinors=%d\n",
		version.Version,
		cliPreferredApiRevision,
		cliMinApiRevision,
		cliTargetProductVersion,
		cliSupportedProductMinors,
	)
}
