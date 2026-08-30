# BP-032: Customer DX — validate/deploy local repo vs org

- **Severity:** Medium
- **Status:** Mitigated
- **Area:** `internal/deploy`, `cmd/one`

## Outcome

Validate-first repo→org loop: `POST /packages/validate-local`, `one org validate`, gated `org deploy`. Peer push / inbound artifact promote is not the supported path. Git host is provider-agnostic.

## Do not reopen

CLI Ship remainders live on [BP-048](./BP-048-one-cli.md). Control IDE Ship chrome is frozen ([ADR-030](../docs/adr/030-install-agent-runtime.md)).

## Related

- [customer-dx-build-plan.md](../docs/architecture/customer-dx-build-plan.md) · [customer-developer-workflow.md](../docs/customer-developer-workflow.md)
