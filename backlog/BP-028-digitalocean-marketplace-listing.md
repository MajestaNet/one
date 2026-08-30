# BP-028: DigitalOcean Marketplace listings (deferred)

- **Severity:** Medium
- **Status:** Open (deferred — blocked on vendor readiness)
- **Area:** `deploy/digitalocean/`, Vendor Portal
- **Plan:** [do-app-platform-deploy-api-build-plan.md](../docs/architecture/do-app-platform-deploy-api-build-plan.md) Wave A (packaging) · strategy [digitalocean-distribution-build-plan.md](../docs/architecture/digitalocean-distribution-build-plan.md)
- **Depends on:** [BP-029](./BP-029-app-platform-install.md) App Spec; [BP-011](./BP-011-container-marketplace-fargate.md) Helm path; DigitalOcean **Vendor Portal** account + tax/banking

## Problem

We want Marketplace / one-click style distribution on DigitalOcean, but MajestaNet does **not** yet have a vendor account and is **not ready to publish**. Portal work must not block App Spec packaging ([BP-029](./BP-029-app-platform-install.md)) or Deploy API day-2 work ([BP-030](./BP-030-deploy-api-digitalocean-apps.md)).

## Strategy when unblocked

1. **Primary:** App Platform–oriented listing or Deploy-to-DigitalOcean flow (lowest friction — matches default customer path; uses Wave A App Spec)
2. **Also maintain (backlog track):** Kubernetes 1-Click (Helm) for network / cluster-oriented customers
3. Both pin the same GHCR digests as `v*` releases

## Blockers (must clear before scheduling)

1. DigitalOcean Marketplace **vendor application** approved + Vendor Portal access  
2. Tax / banking forms if required  
3. App Spec ([BP-029](./BP-029-app-platform-install.md)) 1-Click-ready + Helm chart smoke-ready  
4. Product decision that we are ready to support Marketplace installers  

## Scope (when unblocked — do not start now)

1. Vendor Portal listing copy (App Platform first)  
2. Optional Kubernetes 1-Click stack under `deploy/digitalocean/` (**backlog** relative to App Platform listing)  
3. Listing docs: Managed Postgres, firewall / VPC checklist, secrets  
4. Pin digests (no `:latest`); install-test on a real DO team. Existing apps upgrade by **republished listing digest** or customer/Ops roll to the newest `v*` digest — customer is DO admin; infra bill stays on their DO invoice. Listing is **free OSS backend**, not Control IDE seats ([BP-062](../docs/adr/030-install-agent-runtime.md)).  

## Explicit non-goals (while deferred)

- Vendor Portal work from product CI  
- Droplet 1-Click or AMI  
- Blocking App Spec / Deploy API waves on a live Marketplace URL  
- Implementing [BP-027](../docs/adr/030-install-agent-runtime.md) as a substitute for listing  
- Starting K8s 1-Click engineering before App Platform listing path is clear  

## Related

- [BP-029](./BP-029-app-platform-install.md) · [BP-030](./BP-030-deploy-api-digitalocean-apps.md) · [BP-011](./BP-011-container-marketplace-fargate.md) · [BP-027](../docs/adr/030-install-agent-runtime.md)  
- [do-app-platform-deploy-api-build-plan.md](../docs/architecture/do-app-platform-deploy-api-build-plan.md)  
- [DigitalOcean Marketplace vendor guidelines](https://marketplace.digitalocean.com/vendors/guidelines-resources)
