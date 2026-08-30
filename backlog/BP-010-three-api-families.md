# BP-010: Three API families

- **Severity:** High
- **Status:** Mitigated
- **Area:** `internal/httpapi`, `internal/deploy`, `internal/authz`

## Outcome

Client `/client/v1`, Metadata `/metadata/v1`, Deploy `/deploy/v1` (plus Ops `/ops/v1`). Deprecated `/v1` aliases remain for compatibility only. Managed metadata is not Deploy-promoted. Multi-env is shared `CUSTOMER_ID` + unique `INSTALL_ID`; Ship path is **repo→org** validate/deploy, not install→install promote.

## Do not reopen

Family split is done. Track signed-bundle rotation, peer discovery, and transactional apply separately. Do not treat `CUSTOMER_ID` as a SaaS `tenant_id` on business rows ([ADR-001](../docs/adr/001-dedicated-install.md)).

## Related

- [ADR-004](../docs/adr/004-three-api-families.md) · [api-families.md](../docs/api-families.md) · [multi-env-deploy.md](../docs/multi-env-deploy.md)
