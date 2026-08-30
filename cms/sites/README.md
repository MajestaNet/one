# Sites

One directory per **Netlify site** / public hostname. Shared theme or packages may live at the CMS repo root after scaffold; each site keeps its own `content-map.yaml`, `pin`, overlay, and `netlify.toml`.

| Directory | Hostname | Source repo | Production pin |
|---|---|---|---|
| [one/](./one/) | `one.majesta.net` | `MajestaNet/ide` | `v*` (GHCR) |

Add a new product by copying `one/`’s shape (map + pin + overlay + ignore), then creating a **new** Netlify site pointed at that package directory. Do not hang a second product off domain aliases on the One site.
