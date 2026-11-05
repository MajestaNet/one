# Contributing

Majesta One is in **alpha (`0.1.0`)**. APIs, metadata, and packaging can still change in breaking ways. See the [README](README.md) for project status.

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE). By submitting a contribution, you agree that it is licensed under those same terms.

## Security

Do **not** open a public GitHub issue for vulnerabilities. Follow [SECURITY.md](SECURITY.md).

Confirmed test-campaign defects (SI rollout) **are** public GitHub issues titled `[campaign G-…]`. File them from [.github/ISSUE_TEMPLATE/campaign-finding.md](.github/ISSUE_TEMPLATE/campaign-finding.md). Do **not** add a new `backlog/BP-*.md` item for those; the architecture remainder list stays in [`backlog/`](backlog/README.md). Fix PRs must include `Fixes #<issue>`.

Do not commit secrets, customer data, exploit PoCs, or live advisory detail. The public architecture-risk list is [`backlog/`](backlog/README.md) ([BP-026](backlog/BP-026-oss-security-public-backlog.md)).

## Commit identity

Author and committer addresses in this repository are limited to GitHub noreply addresses and the Cursor agent address. Personal and workplace mailboxes do not belong in commit metadata or trailers.

Humans: in GitHub **Settings → Emails**, enable **Keep my email addresses private**, then set `user.email` to the `*@users.noreply.github.com` address shown there. Cursor agents commit as `Cursor Agent <cursoragent@cursor.com>`. GitHub web commits and Dependabot sign-offs use `noreply@github.com` and `support@github.com`.

`scripts/check-commit-emails.sh` checks authors, committers, and `Co-authored-by` / `Signed-off-by` / `Reviewed-by` trailers. CI runs it on every pull request and on `main`.

## Tests

- Go: `make test`
- Control IDE: `make test-ide`

Pull requests run path-filtered GitHub Actions (`.github/workflows/ci.yml`): Go lint/tests when `cmd/`, `internal/`, `migrations/`, `deploy/`, or `scripts/` change; Control IDE Vitest/build when `tools/control-ide/` changes; Gitleaks on every run. IDE-only PRs skip `go test`. See [docs/release-cicd.md](docs/release-cicd.md).
