# Netlify — one CMS repo, many subdomains

**Yes:** one GitHub repository can drive many custom subdomains.  
**How:** create **one Netlify site per product**, all connected to **this** CMS repo.  
**Not:** one Netlify site with domain aliases (aliases serve the **same** deploy on every hostname).

Official model: [Monorepos](https://docs.netlify.com/build/configure-builds/monorepos/). Netlify does not spawn N sites from a single root file. You add N site entries (UI or API).

## Site per product

| Netlify site | Package directory | Production hostname |
|---|---|---|
| `majesta-one-docs` | `sites/one` | `one.majesta.net` |
| (later) company / other products | `sites/<name>` | that product’s host |

For each site:

1. Add a new Netlify site → connect the **CMS** GitHub repo (not `MajestaNet/ide`).
2. Set **package directory** to `sites/one` (etc.). Leave **base directory** unset (repo root) so shared tooling can live at root.
3. Put `netlify.toml`, `_redirects`, `_headers` in the **package directory**.
4. Attach **one** custom domain (CNAME → that site’s `*.netlify.app`, or Netlify DNS).
5. Turn **Deploy Previews** on. Production publish from **CMS** `main` on (after human merge).
6. Functions / Identity / Forms **off**.

### Example `sites/one/netlify.toml` (when scaffolded)

```toml
# Paths are relative to the base directory (repo root if base is unset).
[build]
  command = "npm ci && npm run build --workspace=sites/one"
  publish = "sites/one/dist"
  ignore = "git diff --quiet $CACHED_COMMIT_REF $COMMIT_REF -- sites/one"

[build.environment]
  NODE_VERSION = "22"
```

Adjust `command` / `publish` to match the scaffold (workspace vs `cd sites/one && npm ci && npm run build`). The important part is **`ignore`**: a change under `sites/www` must not rebuild `one.majesta.net`.

By default, any commit under the repo root can trigger **all** connected sites. Custom `ignore` is required.

## What not to use

| Mechanism | Result |
|---|---|
| Domain aliases on one site | Many hostnames, identical HTML |
| Branch subdomains (`foo--site.netlify.app`) | Previews only — not product DNS |
| One `dist/` + host-based routing | Needs edge/functions — out of scope |
| Path proxy (`majesta.net/one/` → another site) | Optional later; subdomains do not need it |
| Root `netlify.toml` shared by every site | Fights per-site package config |

Apex `majesta.net` stays its **own** Netlify site (existing company static site). Do not reuse that site for One — different publish rules (`main` vs `v*` pin).

## Production vs preview (aggregator)

```text
CMS PR     → Deploy Preview (that site only, if ignore is correct)
CMS main   → production for sites whose pin/overlay changed
ide main   → notify only (One overlay draft; pin unchanged)
ide v*     → notify kind=tag → pin bump PR → after merge, one.majesta.net
```

Do not grant coding agents `NETLIFY_AUTH_TOKEN`. The GitHub app publishes after merge. Optional CLI `netlify deploy --prod` is a human/release fallback, secrets in a CMS GitHub Environment only.

## DNS

CNAME `one.majesta.net` → the **One** Netlify hostname. Company apex stays on the existing site and links here.
