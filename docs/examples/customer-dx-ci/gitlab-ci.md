# Sample: GitLab CI — customer DX validate / deploy

Copy into a **customer** Git repo. Same model as GitHub: validate on MR against test; deploy on merge/tag per env URL.

```yaml
# .gitlab-ci.yml
stages: [validate, deploy]

variables:
  # Majesta One product release tag that publishes one (e.g. v0.2.0)
  ONE_PRODUCT_TAG: "v0.2.0"

.before_cli: &before_cli
  before_script:
    - |
      curl -fsSL -o one \
        "https://github.com/MajestaNet/ide/releases/download/${ONE_PRODUCT_TAG}/one"
      chmod +x one
      install one /usr/local/bin/
      one version

validate:test:
  stage: validate
  <<: *before_cli
  script:
    - one org validate -dir . --base-url "$ONE_TEST_URL" --token "$ONE_TEST_JWT"
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
    - if: $CI_COMMIT_TAG

deploy:staging:
  stage: deploy
  <<: *before_cli
  script:
    - one org deploy -dir . --base-url "$ONE_STAGING_URL" --token "$ONE_STAGING_JWT"
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH

deploy:prod:
  stage: deploy
  <<: *before_cli
  script:
    - one org deploy -dir . --base-url "$ONE_PROD_URL" --token "$ONE_PROD_JWT"
  rules:
    - if: $CI_COMMIT_TAG
  when: manual
```

**Notes**

- The unadorned asset `one` is linux/amd64 (same bits as `one-linux-amd64`). Mac/Windows humans download `one-darwin-amd64`, `one-darwin-arm64`, or `one-windows-amd64.exe`. Override with `ONE_CLI_URL` only for air-gap mirrors.

Peer-to-peer bundle promote is **not** part of this pipeline. See [customer-developer-workflow.md](../../customer-developer-workflow.md) · [one-cli-build-plan.md](../../architecture/one-cli-build-plan.md).
