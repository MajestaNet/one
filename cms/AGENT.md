# CMS agent playbook

For agents that update **public overlay** in the Majesta CMS aggregator after a source-repo notify. This playbook lives in the **CMS repo**. Product-repo agents do not run it.

**Design:** [DESIGN.md](./DESIGN.md) · **Contract:** [SOURCE-CONTRACT.md](./SOURCE-CONTRACT.md)

The aggregator is a **publisher**. This playbook is the **writer**. CMS CI must not generate markdown.

## Plane fence

| May edit | Must not edit |
|---|---|
| `sites/<product>/**` overlay, sidebar, content map, `pin` (when the notify is a release tag) | Any product repo’s `cmd/`, `internal/`, `migrations/`, `deploy/` |
| This playbook and CMS design docs | `tools/control-ide` in any product repo |
| | Merge the PR; `netlify deploy --prod`; use `NETLIFY_*` |

## What to do (notify job)

1. Read the notify payload (`source`, `ref`, `sha`, `paths[]`, `kind`: `merge` \| `tag`). Skip if `skip: true`.
2. Open that product’s content map. Map `paths[]` to public pages (same idea as the One impact table in [sites/one/README.md](./sites/one/README.md)).
3. Fetch the source repo at `sha` (public clone). Read the mapped sources. Update **overlay** so operators see path, method, scope, what it does / does not.
4. If `kind` is `tag` and the product is One, set `sites/one/pin` to that tag and keep `/v/X.Y/` snapshot rules.
5. If `kind` is `merge` and the product is One, **do not** move the production pin to `main`. Overlay-only PR.
6. Do not dump mux listings, playbooks, or backlog IDs onto the public page.
7. Open a **draft** PR on the CMS repo labeled `cms-update`. Do not merge. Do not deploy.

## Tone and IA

- Audience: operators, builders, ISVs — not coding agents.
- Lead with **MCP + `one` + family HTTP** on the One site.
- Mention Control IDE as an **optional JWT client**, not the product shell.
- Footer may link “Source & contributing” to the product GitHub repo. Do not nav to playbooks or BPs.

## Loop guards

Skip (no PR) when:

1. Payload `skip` is true (`docs-only` source diff, no mapped paths, duplicate SHA).
2. The only source changes are already reflected in an open `cms-update` PR for that SHA.
3. Notify is from a previous CMS-driven documentation-only change in the source (if a source repo ever labels that — optional).

## Checklist (CMS PR)

- [ ] Every mapped page from `paths[]` is addressed or explicitly unchanged with a reason
- [ ] Only that product’s `sites/<product>/` overlay (and pin if tag) changed
- [ ] Customer tone; no agent-playbook voice
- [ ] One: pin unchanged unless `kind=tag`
- [ ] PR is **draft**, labeled `cms-update`
- [ ] No merge, no Netlify token, no product-repo code edits

## Docs-update job (paste)

```text
Update Majesta public docs overlay from a source-repo notify.

Read first: the issue/dispatch payload, AGENT.md, DESIGN.md,
sites/<product>/content-map.yaml and README.

For each mapped page, update the overlay so operators and builders see
the new customer-facing behavior. Curated markdown only.

Lead with MCP + one + family HTTP on the One site. Control IDE is optional.
Do not publish playbooks, BPs, or Control IDE internals.
Do not edit product cmd/, internal/, migrations/, deploy/.
Do not change sites/one/pin unless the notify kind is tag.

Open a draft PR labeled cms-update on the CMS repo. Do not merge.
Do not netlify deploy --prod. Do not use NETLIFY_AUTH_TOKEN.
```
