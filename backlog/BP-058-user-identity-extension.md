# BP-058: Customer-extendable User + provisioning config

- **Severity:** High
- **Status:** Mitigated
- **Area:** `internal/authz`, `internal/db`, `internal/metadata`, `internal/scim`, `internal/seed`

## Outcome

Kernel User metadata object, `users.data`, SCIM UserCustom, install provisioning / JIT maps ([ADR-026](../docs/adr/026-kernel-user-metadata.md)). User stays kernel — not a flexible `records` object.

## Do not reopen

Directory bulk/filters remain on [BP-017](./BP-017-identity-directory-productionization.md). SSO/claim remainders: [BP-037](./BP-037-install-claim-customer-sso.md). Plan: [user-identity-extension-build-plan.md](../docs/architecture/user-identity-extension-build-plan.md).
