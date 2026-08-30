# Marketplace listing prep (do not publish yet)

Blocked on DigitalOcean Vendor Portal + tax/banking — see [BP-028](../../backlog/BP-028-digitalocean-marketplace-listing.md).
Use this checklist when publish is unblocked. Packaging artifacts live in this directory ([BP-029](../../backlog/BP-029-app-platform-install.md)).

## Listing strategy

1. **Primary:** App Platform–oriented listing / Deploy-to-DigitalOcean using `app.yaml`
2. **Optional later:** Kubernetes 1-Click (Helm) — backlog relative to App Platform

## Copy draft (placeholder)

- **Name:** Majesta One
- **Category:** Developer Tools / Business Apps (confirm in Vendor Portal)
- **Short:** Open-source dedicated install CRM backend on App Platform + Managed PostgreSQL
- **Long:** Majesta One is an API-first, metadata-driven CRM backend. This listing deploys the API and worker from public GHCR images with pinned digests. You supply Managed PostgreSQL and bootstrap secrets. Optional Control IDE connects to your install URL.
- **Keywords:** CRM, API, Postgres, App Platform, metadata, dedicated install

## Screenshots / media checklist

- [ ] App Spec / architecture diagram (API + worker + Managed Postgres)
- [ ] Control IDE Connect against App Platform HTTPS URL (optional product shot)
- [ ] Secrets checklist screenshot (no real secrets)

## Digest pin rules (required for listing)

- Never publish `:latest`
- Pin `PRODUCT_VERSION` and image digests from GitHub Release `image-digests-X.Y.Z.txt`
- Re-test install on a DO team account after each listed version bump

## Operator secrets (document in listing)

| Secret / env | Required |
|---|---|
| `DATABASE_URL` | Yes — Managed Postgres connection string |
| `API_KEYS` | Yes — bootstrap / break-glass key with `+admin` |
| `AUTH_JWT_SIGNING_KEY` | Yes |
| `INSTALL_CLAIM_TOKEN` | Yes — one-time day-0 claim (first SystemAdmin) |
| `DEPLOY_SHARE_SECRET` | Yes in production |
| `CUSTOMER_ID` / `INSTALL_ID` / `INSTALL_ROLE` | Yes |
| `PLATFORM_PUBLIC_URL` | Yes — App Platform HTTPS origin |
| `DIGITALOCEAN_API_TOKEN` | Optional — enables Deploy API day-2 cloud ops |

## Explicit non-goals while deferred

- Submitting the Vendor Portal listing from CI
- Droplet 1-Click
- Claiming live Marketplace URL in product docs
