# ADR-034: Node guest for customer code automations

## Status

Accepted. Supersedes the Deno guest engine in [ADR-014](./014-customer-code-automations.md) §2. No production installs exist, so the guest binary swap is a breaking change we take now.

## Context

[ADR-014](./014-customer-code-automations.md) runs customer TypeScript in a default-deny guest so domain logic stays out of the Go kernel ([ADR-005](./005-go-runtime.md)). The first engine was Deno 2.9.3: a second toolchain, a GitHub-release binary in the product image, and an operator prerequisite (`DENO_PATH`) that is not a normal open-source worker dependency.

Node is already the vendor-plane toolchain (Control IDE and `tools/one-mcp` pin Node 22) and is the runtime operators can install from ordinary OS and CI channels. Product API and worker binaries stay Go. The guest is an install-side executor only — not a platform Node sidecar and not a return of the retired Node API.

## Decision

### 1. Guest engine is Node 22

- Product images pin **Node 22.22.2** (`NODE_BIN=/usr/local/bin/node` in `deploy/Dockerfile`).
- Native `go run` / `make api` / `make worker` resolve `NODE_BIN`, then `node` on `PATH`. Minimum **22.15.0** (in-process `module.registerHooks`). Older Node fails closed (`ErrNodeTooOld`); a missing binary is `ErrNodeUnavailable`.
- Do not set `NODE_PATH` or pass `NODE_OPTIONS` into the guest. `NODE_PATH` is Node’s module search path and would widen the sandbox.
- Suites run Node in the **API** process. `execution: async` automations run Node in the **worker**. Both processes need the binary.

### 2. Default-deny stays, with Node’s controls

The host invokes:

- `--permission` with `--allow-fs-read` limited to the guest workdir (no fs write, child process, worker, WASI, or addon grant)
- `--disallow-code-generation-from-strings` (`eval` / `new Function`)
- `--experimental-strip-types` (erasable TypeScript; also the default on 22.18+)
- `--import` of an embedded policy that:
  - resolves `one:automation` to the injected SDK
  - rejects `node:` (except `node:fs` from platform bootstrap files), `npm:`, `jsr:`, `http(s):`, `data:`, and files outside the workdir
  - disables `fetch`, `WebSocket`, `EventSource`, `process.getBuiltinModule` for IO/code builtins, and `process.dlopen`

Static import ban is unchanged: only `one:automation`, no `require`, no literal dynamic import, no `eval`. The policy is the runtime backstop when a specifier is built at runtime.

Outbound HTTPS remains Go host RPC (`ctx.http` / `ctx.connector`). The guest has no network.

### 3. What does not change

- Customer source shape, `one:automation`, permission-set grants, run-as, sync rollback, async jobs, platform `invokeAction`, and the unit harness (`tests/automations/**`).
- NDJSON host protocol on stdin/stdout.
- No npm, JSR, URL, or Node builtin imports in customer source.
- Guest TypeScript is **erasable** syntax (types, interfaces, `import type`). Enums, namespaces, and parameter properties are rejected by Node’s type stripper. That is the supported language subset.
- Managed package automations ([ADR-033](./033-managed-package-automations.md)) use this same guest.

## Consequences

- `DENO_PATH` and the Deno binary are removed. Images and native shells use `NODE_BIN` or `node` on `PATH`.
- [ADR-005](./005-go-runtime.md) still holds: Go is the platform runtime. Node in the image is the guest executor, the same role Deno had.
- Operators on macOS install Node 22.22.2 (or any 22.15+) with the rest of the toolchain. They do not install Deno.
- Future RSS/CPU caps ([BP-033](../../backlog/BP-033-customer-runtime-isolation.md)) apply to the Node process (`GUEST_RSS_LIMIT_MB` when that work lands), not a Deno-named knob.

## Explicit non-goals

- npm allowlists
- Running customer code inside the Go process
- A Node or TypeScript platform API
- Full TypeScript transform (enums and namespaces)

## Related

- [ADR-014](./014-customer-code-automations.md) · [ADR-005](./005-go-runtime.md) · [ADR-033](./033-managed-package-automations.md)
- [automation-sdk.md](../automation-sdk.md) · [customer-automations-build.md](../architecture/customer-automations-build.md)
- [BP-009](../../backlog/BP-009-no-in-kernel-language.md)
