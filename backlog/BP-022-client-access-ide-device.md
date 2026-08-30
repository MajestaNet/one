# BP-022: Client access mode and registered clients

- **Severity:** High
- **Status:** Partially mitigated — Client `clientAccessMode` / `azp` / Connected Apps shipped; IDE device cert and ALB mTLS remainders **frozen** ([ADR-030](../docs/adr/030-install-agent-runtime.md))
- **Area:** `internal/authz`, `/auth/v1`, install exposure, Connected Apps

## Outcome (keep)

- `clientAccessMode` on install exposure: `open` | `registered_clients` | `ide_users`
- JWT `azp`; Connected App `allowedCidrs` merge on exposure apply
- System capabilities including `ide.*` (removal of chrome-only caps is [BP-065](./BP-065-ide-backend-coupling.md))
- Session durability after access JWT expiry is [BP-063](./BP-063-refresh-token-sessions.md), not device cert

## Frozen (do not implement)

Hardware-bound device certs, TPM attestation, and ALB mutual TLS as product chrome. Optional community AWS mTLS stays in `sdk/aws` if an operator opts in.

## Related

- [system-capabilities.md](../docs/architecture/system-capabilities.md) · [control-ide-security-audit.md](../docs/architecture/control-ide-security-audit.md) · [ADR-015](../docs/adr/015-idp-agnostic-social-login.md)
