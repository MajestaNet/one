# BP-003: Enterprise AuthZ (FLS, capabilities, sharing)

- **Severity:** High
- **Status:** Mitigated
- **Area:** `internal/authz`, `internal/dataengine`, Metadata / Deploy routes

## Outcome

Object permission sets, deny-by-default FLS, Metadata/Deploy system capabilities, and ADR-016 record sharing are in product. Humans, services, and agents share that path — do not invent agent-only AuthZ.

## Do not reopen

User custom fields / SCIM schema / JIT maps belong on [BP-058](./BP-058-user-identity-extension.md) (mitigated) and [BP-017](./BP-017-identity-directory-productionization.md). AuthN is [BP-013](./BP-013-jwt-unified-principals.md) / [ADR-006](../docs/adr/006-jwt-auth.md).

## Related

- [customization-authz.md](../docs/architecture/customization-authz.md) · [record-sharing.md](../docs/architecture/record-sharing.md) · [ADR-016](../docs/adr/016-record-sharing.md)
