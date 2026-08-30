# BP-036: Canonical field types

- **Severity:** High
- **Status:** Mitigated
- **Area:** `internal/metadata`, `internal/dataengine`

## Outcome

Metadata allowlist + `GET /metadata/v1/field-types`. Create rejects unknown/alias types. DataEngine coerce/cast follows the registry ([ADR-017](../docs/adr/017-canonical-field-types.md)).

## Do not reopen

Non-goals remain: `multipicklist`, formula, rollup, encrypted-at-rest, file/blob, in-place type changes after create. Files/blobs are [BP-045](./BP-045-files-content-storage.md).
