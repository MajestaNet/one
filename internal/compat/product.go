package compat

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var semverPrefixRe = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)

func parseSemverPrefix(version string) (major, minor, patch int, ok bool) {
	m := semverPrefixRe.FindStringSubmatch(strings.TrimSpace(version))
	if m == nil {
		return 0, 0, 0, false
	}
	var parts [3]int
	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return 0, 0, 0, false
		}
		parts[i] = n
	}
	return parts[0], parts[1], parts[2], true
}

// ProductTestedAgainst evaluates the soft IDE/CLI tested-against window (N minors).
// Returns verdict: ok or warn, and optional code when outside the window.
func ProductTestedAgainst(installProduct, targetProduct string, supportedMinors int) (status string, code string) {
	if supportedMinors < 1 {
		supportedMinors = 2
	}
	installMajor, installMinor, _, okInstall := parseSemverPrefix(installProduct)
	targetMajor, targetMinor, _, okTarget := parseSemverPrefix(targetProduct)
	if !okInstall || !okTarget {
		return "warn", "UNPARSEABLE_PRODUCT"
	}
	if installMajor != targetMajor {
		return "warn", "PRODUCT_OUTSIDE_TESTED"
	}
	lower := targetMinor - (supportedMinors - 1)
	if installMinor < lower || installMinor > targetMinor {
		return "warn", "PRODUCT_OUTSIDE_TESTED"
	}
	return "ok", ""
}

// SelectClientPin computes the default pin for a client manifest against an install window.
// Returns pin or block code per ADR-025 / BP-025 handshake.
func SelectClientPin(clientMin, clientPreferred int, window APIRevisionWindow) (int, string, error) {
	if clientMin < 1 || clientPreferred < 1 {
		return 0, "UNPARSEABLE_REVISION", fmt.Errorf("client revision manifest invalid")
	}
	if clientMin > window.Current {
		return 0, "INSTALL_REVISION_TOO_OLD", fmt.Errorf("install api revision %d is below client min %d", window.Current, clientMin)
	}
	preferred := clientPreferred
	if preferred > window.Current {
		preferred = window.Current
	}
	if preferred < window.Min {
		return 0, "API_REVISION_UNSUPPORTED", fmt.Errorf("install min revision %d exceeds client preferred %d", window.Min, preferred)
	}
	pin := preferred
	if pin < window.Min {
		pin = window.Min
	}
	if pin < clientMin {
		pin = clientMin
	}
	if !PinInWindow(pin, window) {
		return 0, "API_REVISION_UNSUPPORTED", fmt.Errorf("pin %d outside install window [%d,%d]", pin, window.Min, window.Current)
	}
	return pin, "", nil
}

// EvaluateRevisionHard returns compat status for a chosen pin.
func EvaluateRevisionHard(pin int, window APIRevisionWindow, overridden bool) (status, code string) {
	if overridden {
		return "overridden", ""
	}
	if !PinInWindow(pin, window) {
		return "block", "API_REVISION_UNSUPPORTED"
	}
	return "ok", ""
}
