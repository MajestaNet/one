# Sample: GitHub Actions — customer DX validate / deploy

Copy into a **customer** Git repo (not the Majesta One product monorepo). Requires a Majesta One JWT or API key with `deploy` + `deploy.promote` on the target install.

```yaml
# .github/workflows/one-dx.yml
name: one-dx
on:
  pull_request:
  push:
    branches: [main]
    tags: ["v*"]

jobs:
  validate-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install one
        env:
          # Majesta One product release tag that publishes the CLI asset, e.g. v0.2.0
          ONE_PRODUCT_TAG: ${{ vars.ONE_PRODUCT_TAG }}
        run: |
          TAG="${ONE_PRODUCT_TAG:?set vars.ONE_PRODUCT_TAG to a Majesta One v* release}"
          curl -fsSL -o one \
            "https://github.com/MajestaNet/ide/releases/download/${TAG}/one"
          chmod +x one
          sudo mv one /usr/local/bin/
          one version
      - name: Org validate (test)
        env:
          ONE_TOKEN: ${{ secrets.ONE_TEST_JWT }}
          ONE_BASE_URL: ${{ vars.ONE_TEST_URL }}
        run: one org validate -dir .

  deploy-staging:
    if: github.ref == 'refs/heads/main'
    needs: validate-test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install one
        env:
          ONE_PRODUCT_TAG: ${{ vars.ONE_PRODUCT_TAG }}
        run: |
          curl -fsSL -o one \
            "https://github.com/MajestaNet/ide/releases/download/${ONE_PRODUCT_TAG}/one"
          chmod +x one && sudo mv one /usr/local/bin/
      - name: Org deploy (staging)
        env:
          ONE_TOKEN: ${{ secrets.ONE_STAGING_JWT }}
          ONE_BASE_URL: ${{ vars.ONE_STAGING_URL }}
        run: one org deploy -dir .

  deploy-prod:
    if: startsWith(github.ref, 'refs/tags/v')
    needs: validate-test
    runs-on: ubuntu-latest
    environment: production
    steps:
      - uses: actions/checkout@v4
      - name: Install one
        env:
          ONE_PRODUCT_TAG: ${{ vars.ONE_PRODUCT_TAG }}
        run: |
          curl -fsSL -o one \
            "https://github.com/MajestaNet/ide/releases/download/${ONE_PRODUCT_TAG}/one"
          chmod +x one && sudo mv one /usr/local/bin/
      - name: Org deploy (prod)
        env:
          ONE_TOKEN: ${{ secrets.ONE_PROD_JWT }}
          ONE_BASE_URL: ${{ vars.ONE_PROD_URL }}
        run: one org deploy -dir .
```

**Notes**

- Prefer release asset `one` from product `v*` tags ([BP-048](../../../backlog/BP-048-one-cli.md)). That name is linux/amd64 (same bits as `one-linux-amd64`); Mac/Windows humans download `one-darwin-amd64`, `one-darwin-arm64`, or `one-windows-amd64.exe`. Override with `ONE_CLI_URL` only for air-gap mirrors.
- Same Git SHA is validated/deployed per env — never peer-push bundles install→install.
- `--skip-validate` is break-glass only; do not use in CI.
- See [customer-developer-workflow.md](../../customer-developer-workflow.md) · [one-cli-build-plan.md](../../architecture/one-cli-build-plan.md).
