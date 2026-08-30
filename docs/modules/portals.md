# Module: `portals`

Optional managed module: **Portal website and community surfaces**.

See [industry packages](../architecture/cdm-industry-packages.md) for the curated object set.

## Dependency

- `core` (must be installed)

## Version

`1.0.0`

## Objects

Website, WebPage, WebRole, Invitation, Forum, ForumThread, ForumPost, Blog, BlogPost, Idea, Poll

Flexible `records` storage; `ownership=managed`, `package_name=portals`.

Does **not** redefine spine apiNames (`Account`, `Contact`, `Product`, `Case`, `Opportunity`, `Campaign`, `Asset`, …).

## Enable

```http
POST /metadata/v1/packages/portals/enable
```

## Soft-disable

```http
POST /metadata/v1/packages/portals/disable
```
