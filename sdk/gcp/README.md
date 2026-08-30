# Majesta One GCP community SDK (stub)

Community best-effort GCP helpers. **Not product GA.** Not a third install path — see [docs/self-host.md](../../docs/self-host.md) Path A / Path B and [sdk/README.md](../README.md).

## Expected layout

```text
sdk/gcp/
├── README.md      This file
├── identity/      Cloud Identity / IAM adapters
├── ops/           Upgrade / roll helpers
├── edge/          Cloud Armor / exposure adapters
├── deploy/        GKE / Terraform packaging
└── docs/          GCP runbooks
```

Until populated, use Path B Helm on GKE with the portable chart (`deploy/helm/one`).
