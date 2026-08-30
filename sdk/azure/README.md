# Majesta One Azure community SDK (stub)

Community best-effort Azure helpers. **Not product GA.** Not a third install path — see [docs/self-host.md](../../docs/self-host.md) Path A / Path B and [sdk/README.md](../README.md).

## Expected layout

```text
sdk/azure/
├── README.md      This file
├── identity/      Entra ID / managed identity adapters
├── ops/           Upgrade / roll helpers
├── edge/          Front Door / WAF exposure adapters
├── deploy/        AKS / Bicep / ARM packaging
└── docs/          Azure runbooks
```

Until populated, use Path B Helm on AKS with the portable chart (`deploy/helm/one`).
