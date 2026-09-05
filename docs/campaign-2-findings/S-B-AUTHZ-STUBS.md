# Campaign finding (SI rollout)

## Campaign

- Run date: 2026-09-05
- Beat id (`G-…` or `S-…`): S-B-AUTHZ-STUBS
- Scenario card (`A`–`F` or `S-A`–`S-E`): S-B
- Outcome: `pass-with-workaround`
- DX (1–5): 3
- Class: `docs-drift`
- Operator doc you used first: docs/modules/README.md (AuthZ stubs), docs/customer-connect.md Path B
- Gap-log row: [docs/customer-rollout-gap-log.md](../customer-rollout-gap-log.md)
- GitHub issue: [#35](https://github.com/MajestaNet/one/issues/35)

## What happened

Expected: after enabling `sales`, non-admin permission sets get deny stubs on Opportunity; a non-admin JWT cannot CRUD Opportunity until a PS grant; claim admin can.

Actual (acme-dev, packs enabled):

- `GET /metadata/v1/permissions/sets/Operate` (and every non-Admin system PS) shows Opportunity `canCreate/canRead/canUpdate/canDelete` all false. Admin PS is full CRUD. Matches docs/modules/README.md.
- Service principal `StandardUser` + `Operate`, `isAdmin=false`. Client POST/GET/PATCH/DELETE `/client/v1/sobjects/Opportunity` → 403 `forbidden: not allowed to {create,read,update,delete} Opportunity`.
- Claim admin JWT: create Account, then Opportunity with `AccountId` → 201; GET/PATCH 200; DELETE 204. (Bare Opportunity create without AccountId/ContactId is 400 validation, not AuthZ.)

Workaround / docs gap: `docs/customer-connect.md` Path B says

```
client_id=<principal_credential_id>
client_secret=<secret>
```

`POST /client/v1/principals/{id}/credentials` returns `id` (credential UUID) + `clientSecret`. Using that credential `id` as `client_id` → 401 `INVALID_CLIENT`. Using the **principal / user id** as `client_id` with the same `clientSecret` → 200 access token. Assign endpoints also require `userId` (not `principalId`); 400 text names the fields.

AuthZ behavior is correct. Operator docs for minting the non-admin JWT are wrong.

## Fix-it (for the implementing agent)

Playbook (one): docs/architecture/agent-authz.md
Domain agent: `authz-security`
Packages (stay in): operator docs only unless the token endpoint is intentionally credential-id and is broken — then `internal/httpapi` token handler + tests. Prefer documenting the live contract first.
Out of scope: Control IDE; changing deny-stub seed behavior

1. Align `docs/customer-connect.md` Path B (and builder-connect if it copies the same curl) with the live `grant_type=client_credentials` body: `client_id` is the principal id; `client_secret` is `clientSecret` from the credentials POST.
2. Document `POST /client/v1/roles/assign` and `POST /client/v1/permissions/assign` bodies (`userId` + `roleApiName` / `permissionSetApiName`).
3. If product intended `client_id` = credential id, that is a product-bug instead — do not “fix” docs to hide a broken mint; add a test for the intended id.

Verify:

- [ ] Docs match a live `POST /auth/v1/token` on a fresh service principal
- [ ] PR description includes `Fixes #<this-issue>`
- [ ] Gap-log **Issue registry** marks this beat `closed` and links the PR

## Related (optional)

docs/api-families.md identity table lists the paths but not the JSON/form fields.
