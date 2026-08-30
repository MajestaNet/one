# BP-031 — Customer repo initialize + IDE sync

**Severity:** Medium  
**Status:** Partially mitigated  
**Area:** `internal/deploy`, `internal/customerrepo`, `internal/httpapi` (Deploy); `tools/control-ide` (Control IDE)

## Problem

Customization docs (ADR-012) described an auto-seeded CodeCommit `one/v1` repo and IDE clone/sync, but:

1. Terraform created an **empty** remote — no baseline from prod
2. IDE only offered **Initialize from sample** (local scaffold)
3. No **pull** when a local folder already had the repo
4. Managed object/field definitions were absent from the reviewable tree
5. Sharing metadata existed on `BundleArtifact` but was **dropped** by pack/unpack

## Direction

- `POST /deploy/v1/packages/initialize-repo` seeds remote `main` from the calling install (typically prod) via **go-git**
- Export/unpack includes `.one/baseline` (managed, read-only) + sharing folders
- Control IDE: Initialize remote → Sync local (clone or pull `main`)
- Pack/promote never includes baseline

## Mitigation (this change)

| Slice | Status |
|---|---|
| `.one/baseline` + sharing folders in `customerrepo` | Done |
| Export snapshot includes managed baseline | Done |
| `POST /packages/initialize-repo` + `packageInitializeRepo` capability | Done |
| go-git `GitRemote` + MemoryRemote tests | Done |
| IDE Initialize remote / Sync / git:pull / import export | Done |
| Baseline write rejection + skip managed dual-write | Done |
| Docs (ADR-012 amendment, one-repo, build plan) | Done |
| DO / GitHub / GitLab credential UX polish | Remainder — see also [BP-048](./BP-048-one-cli.md) CLI auth (file-backed aliases); IDE host UX still open |
| CodeCommit SigV4 without HTTPS git credentials | Remainder |

## Related

- [customer-repo-init-build-plan.md](../docs/architecture/customer-repo-init-build-plan.md)
- [ADR-012](../docs/adr/012-customer-repo-and-control-ide.md)
- [customer-repo.md](../docs/customer-repo.md)
- [BP-023](./BP-048-one-cli.md)
- [BP-048](./BP-048-one-cli.md)
