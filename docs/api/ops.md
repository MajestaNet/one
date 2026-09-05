# Ops API (`/ops/v1`)

Orchestrate a **product image** upgrade on **this** install: confirm target, roll, gate on tests, roll back.

**Scope:** `ops`. Confirm and rollback also require **admin**.

**Does not:** promote customer metadata (that is [Deploy](./deploy.md)); enable managed modules (that is [Metadata](./metadata.md)); mutate business records.

There is no flat `/v1` alias. Operator narrative: [product-upgrades.md](../product-upgrades.md) · [self-host.md](../self-host.md).

| Method | Path | Admin? | What it does | What it does not |
|---|---|---|---|---|
| `GET` | `/ops/v1/upgrades/available` | | Current product version + roll target config | List Deploy bundles |
| `GET` | `/ops/v1/upgrades` | | List upgrade runs | |
| `GET` | `/ops/v1/upgrades/{id}` | | One run | |
| `POST` | `/ops/v1/upgrades` | yes | Confirm images + version; create a run | Apply a customer bundle |
| `POST` | `/ops/v1/upgrades/{id}/rollback` | yes | Roll back to the previous task definition / image | Undo a Metadata write |

After the new image boots, kernel migrations and enabled package migrate run. Gate with `/healthz` + `/readyz`, then Deploy suites `PlatformSmoke` and optional customer `PostUpgradeSmoke`.

Day-2 cloud scale / resize / provision stay on [Deploy cloud](./deploy.md#cloud-day-2-on-this-install). Ops is the image circuit, not the cloud adapter.

## Related

- [API families overview](../api-families.md) · [Deploy](./deploy.md)
- [Product upgrades](../product-upgrades.md) · [Operations](../ops.md)
