# ADR-005: Go runtime for Platform API and Worker

## Status

Accepted — **done** (Phase 6 cutover; Node/TS/Python sidecar/OpenAPI purged)

## Context

Majesta One ships as a **dedicated install container** on ECS Fargate (BP-011). Subscribers pull runnable images, so TypeScript/Node sources and JS bundles in the image are easy to inspect. We also want a long-term runtime that favors small static binaries, fast startup, and straightforward Fargate ops.

## Decision

1. **Platform runtime language is Go** for `api` and `worker`.
2. **Node/TypeScript is removed** from the repository (no runtime, SDK, or Vitest monorepo).
3. **Python agent sidecar is removed** (agents may return later via separate plans; Client `/agents/*` routes remain as platform surface).
4. **OpenAPI stubs are removed** (contracts documented in code/docs; codegen later under a separate plan).
5. **External HTTP contracts stay stable**: `/client/v1`, `/metadata/v1`, `/deploy/v1`, env vars (`API_KEYS`, `OIDC_*`, install identity; prefer `APP_ENV` over legacy `NODE_ENV`), Postgres kernel + JSONB model.
6. Kernel SQL lives in `migrations/` and is applied by Go migrate on boot / `cmd/migrate`.

## Consequences

- Images are static Go binaries (distroless, stripped).
- Tests are `go test` (+ Postgres when `DATABASE_URL` is set).
- `docs/tech-stack.md` and Marketplace/Fargate docs name Go as the only platform runtime.

## Phased delivery

| Phase | Scope | Status |
|---|---|---|
| 1 | `cmd/api` skeleton, config, AuthN, Dockerfile, CI | Done |
| 2 | DB migrate/boot + kernel access (`pgx`) | Done |
| 3 | Authz object perms + metadata describe/read | Done |
| 4 | Data-engine CRUD/query | Done |
| 5 | Deploy-engine + worker | Done |
| 6 | Fargate cutover; retire Node runtime | Done |
| 7 | Purge Node/TS/sidecar/OpenAPI; Go-only CI/docs/hardening | Done |

## Related

- [tech-stack.md](../tech-stack.md)
- [sdk/aws/docs/aws-fargate.md](../../sdk/aws/docs/aws-fargate.md)
- [BP-011](../../backlog/BP-011-container-marketplace-fargate.md)
- [BP-012](./005-go-runtime.md)
- [ADR-001](./001-dedicated-install.md)
- [ADR-004](./004-three-api-families.md)
