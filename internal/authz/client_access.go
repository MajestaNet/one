package authz

import (
	"fmt"
	"strings"

	"github.com/MajestaNet/ide/internal/edge"
)

// ControlIDEAzp is the managed Connected App apiName for the optional Control IDE desktop client.
const ControlIDEAzp = "one.controlIde"

// InstallAzp is the generic install session client (password without client_id, install claim, token-exchange default).
// It is a JWT azp string, not a Connected App row.
const InstallAzp = "one.install"

// BootstrapAzp marks JWTs minted from env API_KEYS break-glass credentials.
const BootstrapAzp = "one.bootstrap"

// ErrClientAccessDenied is returned when clientAccessMode rejects the mint/use.
var ErrClientAccessDenied = fmt.Errorf("%w: client access mode", ErrForbidden)

// AllowClientAccess reports whether the given azp / auth path is permitted.
// breakglassAPIKey: env API key bootstrap always allowed (ops).
func AllowClientAccess(mode edge.ClientAccessMode, azp string, grantType string, breakglassAPIKey bool) error {
	if breakglassAPIKey {
		return nil
	}
	m := mode
	if m == "" {
		m = edge.ClientAccessOpen
	}
	switch m {
	case edge.ClientAccessOpen:
		return nil
	case edge.ClientAccessRegistered:
		return nil
	default:
		return fmt.Errorf("%w: unknown clientAccessMode %q", ErrClientAccessDenied, m)
	}
}

// AllowBearerAzp enforces clientAccessMode against a resolved JWT azp.
func AllowBearerAzp(mode edge.ClientAccessMode, azp string, authMethod string, isAPIKey bool) error {
	if isAPIKey || authMethod == "api_key" {
		return nil // bootstrap / break-glass opaque keys
	}
	m := mode
	if m == "" {
		m = edge.ClientAccessOpen
	}
	azp = strings.TrimSpace(azp)
	switch m {
	case edge.ClientAccessOpen:
		return nil
	case edge.ClientAccessRegistered:
		if azp == "" {
			return fmt.Errorf("%w: azp required when clientAccessMode=registered_clients", ErrClientAccessDenied)
		}
		return nil
	default:
		return fmt.Errorf("%w: unknown clientAccessMode %q", ErrClientAccessDenied, m)
	}
}
