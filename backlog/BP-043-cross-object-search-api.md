# BP-043: Cross-object indexed search (Client API)

- **Severity:** High
- **Status:** Mitigated
- **Area:** `internal/dataengine`, `internal/httpapi`, `internal/metadata`, `internal/worker`

## Outcome

`POST /client/v1/search` over metadata `searchable` fields with `search_document` + `pg_trgm` GIN. AuthZ + sharing apply. Do not add Elasticsearch or a global JSONB GIN.

## Do not reopen

IDE find chrome is frozen. Search remainders belong in this Client API, not a second store. Plan: [cross-object-search-build-plan.md](../docs/architecture/cross-object-search-build-plan.md).
