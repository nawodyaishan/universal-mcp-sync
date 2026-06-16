# Architecture Audit Report

Date: 2026-05-30  
Repository: `github.com/nawodyaishan/universal-mcp-sync`  
Scope: current Go codebase structure, technologies in use, MCP provider scalability, CLI/TUI architecture, security-sensitive config mutation, and external architecture references gathered with Exa MCP.

## Executive Summary

The codebase has moved well beyond an Exa-only tool. It already has a provider contract, a target-client capability matrix, manifest metadata, saved plans, audit logs, rollback, validation, doctor discovery, and substantial TUI test coverage. The largest architectural risk is that the provider/client abstraction is only partially complete: provider generation, client transport support, manifest metadata, config mutation, verification, CLI flags, and TUI state all still contain hard-coded knowledge that will become expensive as the project scales past the current seven providers and twelve clients.

The top priority is security and transport correctness. Exa credentials are currently embedded in URLs, Context7 bridge credentials can become command arguments, and redaction is regex-based rather than provider-driven. Current Exa docs show header-based API key support, and MCP registry metadata explicitly models secret headers and remote transports. The code should move secrets into headers/env wherever possible, reject unsafe bridge shapes, and add provider-owned redaction classifiers.

The best long-term direction is a three-layer model:

1. **Core sync engine**: provider-neutral plan/apply/verify domain, no client-specific switch logic.
2. **Target adapters**: one adapter per client config shape, generated or configured from `pkg/manifest`.
3. **Provider sources**: built-in providers for high-trust/core cases, a generic provider backed by the official MCP registry, and optional RPC plugins for bespoke setup flows.

## Prioritized Changes

### P0 - Move Secrets Out of URLs and Command Arguments

**Current evidence**

- Exa provider returns an HTTP config whose URL contains `exaApiKey`: `pkg/provider/exa.go:57`, `pkg/exa/url.go:20`.
- Context7's bridge override places the API key in a CLI argument template: `pkg/provider/context7.go:44`.
- `client.Adapt` applies provider bridge overrides when a target cannot handle the remote transport natively: `pkg/client/adapter.go:16`.
- Redaction only covers UUID-like keys, Context7, and Tavily formats: `pkg/redact/redact.go:9`.

**Why this matters**

Credentials in URLs and command arguments are more likely to leak through config files, process listings, logs, screenshots, shell history, bridge output, or unredacted error text. This conflicts with the repository's own security policy: do not print full credentials, provider URLs containing secrets, or generated CLI arguments containing secrets.

**Recommended architecture**

- Change Exa to emit `URL: "https://mcp.exa.ai/mcp?tools=..."` plus `Headers: {"x-api-key": key}` when the target can persist headers safely.
- For clients that cannot persist headers safely, prefer stdio with `env` over URL query parameters. If neither safe header nor env transport is available, mark the target unsupported with an explicit skip reason.
- Remove bridge overrides that require secrets in args. Either use a bridge that accepts env/header configuration safely or skip the target.
- Add a provider-level redaction contract, for example:

```go
type SecretClassifier interface {
    SecretPatterns() []regexp.Regexp
    RedactConfig(MCPConfig) MCPConfig
}
```

- Redact nested slog attributes and maps, not only top-level string attributes.
- Add regression tests for every provider credential format: Exa, GitHub PATs, Context7, Tavily, Terraform/HCP tokens, URL query values, headers, env values, and bridge args.

**External reference**

Exa's current MCP docs show remote MCP at `https://mcp.exa.ai/mcp` and API key support through an `x-api-key` header. MCP registry remote-server metadata also models `headers` with `isSecret`.

### P1 - Make `pkg/manifest` the Source of Truth

**Current evidence**

- `pkg/manifest` already defines clients, providers, runtimes, source references, path templates, mutation kinds, scopes, and docs URLs: `pkg/manifest/types.go`, `pkg/manifest/clients.go`, `pkg/manifest/providers.go`.
- Parallel typed constants still exist in `pkg/config/paths.go:14`, `pkg/config/paths.go:25`, and `pkg/client/capabilities.go:28`.
- Provider registration is a hard-coded static list: `pkg/provider/registry.go:8`.
- `config.DetectAppConfigsForOS` consumes manifest clients, but still maps them through legacy `AppID`, `FileKind`, and `AppOrder`: `pkg/config/paths.go:74`.

**Risk**

Every new client or provider requires edits in multiple packages. The odds of drift are already high: manifest metadata says one thing, config mutation and capability adaptation may say another. This is the exact failure mode that provider/plugin architecture is meant to avoid.

**Recommended architecture**

- Promote manifest IDs to the canonical IDs used across provider, config, app, TUI, and CLI layers.
- Generate or derive `AppConfig`, runtime checks, docs links, credential help, and capability profiles from manifest records.
- Move `client.Matrix` into manifest or a generated `client.CapabilityIndex`.
- Keep built-in Go provider implementations, but register them through metadata:

```go
type ProviderDescriptor struct {
    Meta      manifest.ProviderMeta
    Factory   func() provider.MCPProvider
    Redactor  provider.SecretClassifier
}
```

- Add a manifest consistency test that proves every manifest provider has a provider implementation or a generic-registry fallback, and every manifest client has a target adapter.

### P1 - Extract Target Adapters From `app.Manager`

**Current evidence**

- `Manager.PrepareProviderWithTargetFiles` contains Claude Code special handling: `pkg/app/app.go:210`.
- `prepareFileOperation` switches on `FileKind` and `AppID` to select root keys, URL field names, VS Code extras, Roo Code type fields, Antigravity exceptions, OpenCode special writing, and Codex TOML: `pkg/app/app.go:676`.
- Saved-plan apply reconstructs operations by consulting `provider.DefaultRegistry()` directly: `pkg/app/plan_apply.go:327`.
- Verification repeats provider/client-specific branches: `pkg/verify/verify.go:33`.

**Risk**

The sync engine is both orchestrator and adapter. Adding clients will continue to widen switch statements and duplicate config-shape knowledge in plan, apply, verify, doctor, and tests.

**Recommended architecture**

Introduce target adapters with a narrow contract:

```go
type TargetAdapter interface {
    ID() manifest.ClientID
    CanHandle(provider.MCPConfig) CapabilityDecision
    Adapt(provider.MCPConfig) (provider.MCPConfig, error)
    Mutate(data []byte, op Operation) ([]byte, error)
    Verify(path string, op Operation) verify.Result
}
```

Then make `Manager` depend on an `AdapterRegistry`. The manager should select operations, call adapters, perform atomic writes, rollback, audit, and saved-plan checks. It should not know that Antigravity uses `serverUrl`, VS Code uses `servers`, Roo Code needs a `type` field, or Codex writes TOML.

### P1 - Standardize Canonical MCP Transport Semantics

**Current evidence**

- `provider.TransportHTTP` is labeled as legacy VS Code compatibility: `pkg/provider/types.go:9`.
- Exa provider emits `TransportHTTP`: `pkg/provider/exa.go:57`.
- Context7 emits `TransportStreamableHTTP`: `pkg/provider/context7.go:37`.
- VS Code config writer adds `"type": "http"` based on `AppID`: `pkg/app/app.go:735`.

**Risk**

The code conflates protocol transport with target config field names. `http`, `streamable-http`, `serverUrl`, and `url` are not all the same architectural concept. This will produce subtle adapter bugs as more remote MCP servers are added.

**Recommended architecture**

- Use canonical provider transports: `stdio`, `streamable-http`, and `sse`.
- Treat target-specific config names as adapter output, not provider output.
- Keep a compatibility enum only in target adapters, not in provider-generated config.
- Add tests where one provider config is rendered for all supported clients and compared to golden fixtures.

### P1 - Refactor the Dashboard TUI Into Screen Models or Controllers

**Current evidence**

- `pkg/tui/dashboard.go` is about 1,300 lines and owns scanning, readiness, validation, planning, saved-plan storage, preflight, apply, conflict resolution, credential entry routing, recording, and key handling.
- The dashboard calls `provider.DefaultRegistry()` internally in several places: `pkg/tui/dashboard.go:252`, `pkg/tui/dashboard.go:423`, `pkg/tui/dashboard.go:871`.
- The legacy wizard model defaults to Exa: `pkg/tui/model.go:53`.

**Risk**

The TUI follows Bubble Tea's model/update/view pattern, but the dashboard model is becoming a single large coordinator. That makes provider-neutral UX changes harder, and it increases the risk that key handling, side effects, and screen transitions regress together.

**Recommended architecture**

- Split dashboard into screen-level models:
  - `welcomeModel`
  - `doctorModel`
  - `providerReadyModel`
  - `credentialEntryModel`
  - `targetSelectModel`
  - `planPreviewModel`
  - `applyResultModel`
- Keep all IO in `tea.Cmd` closures and all screen transitions explicit.
- Inject `ProviderRegistry`, `AdapterRegistry`, `PlanStore`, and clock dependencies instead of calling defaults from inside the TUI.
- Keep shared rendering styles, but keep each screen's update and view logic close together.

**External reference**

Bubble Tea's core architecture is model, init, update, and view, with IO represented as commands. Lazygit's codebase guide documents the gradual split from a GUI god struct into contexts, controllers, and helpers; that is the same pressure now visible in `DashboardModel`.

### P1 - Harden Saved-Plan Integrity

**Current evidence**

- Saved plans include schema version, target details, credential refs, current SHA, symlink fields, and content hash: `pkg/app/plan_v2.go:36`.
- `ContentHash` is computed only over the `Redacted` strings: `pkg/app/plan_v2.go:141`.
- Apply validates schema version, expiration, current file SHA, symlink state, symlink target, path scope, and CLI command drift: `pkg/app/plan_apply.go:196`, `pkg/app/plan_apply.go:236`, `pkg/app/plan_apply.go:249`, `pkg/app/plan_apply.go:508`.

**Risk**

The saved-plan safety model is strong in several areas, but the integrity hash does not cover all safety-critical fields. A local plan edit that changes a path, manager, target ID, file kind, or warnings may not change the redacted summary string.

**Recommended architecture**

- Replace or supplement `ContentHash` with a canonical structural digest over:
  - schema version
  - provider ID
  - credential refs without secret values
  - target ID/name/scope
  - file path/resolved path
  - manager/file kind/action
  - transport
  - current SHA
  - symlink state
  - CLI command
  - VS Code input descriptors
- Keep redacted text in the hash only as display metadata, not as the primary integrity boundary.
- Add tampering tests for file path, target ID, file kind, manager, transport, and CLI command.

### P2 - Make the CLI Command Layer Provider-Neutral

**Current evidence**

- `cmd/usync/main.go` still exposes root flags as Exa-specific keys and calls `manager.Prepare`, which wraps `provider.NewExaProvider()`: `cmd/usync/main.go:97`, `pkg/app/app.go:357`.
- `plan`, `apply`, `validate`, and `providers` subcommands are more provider-neutral, but still use manual `flag.FlagSet` routing: `cmd/usync/plan_commands.go:31`, `cmd/usync/apply_command.go:19`, `cmd/usync/validate_command.go:15`.
- `apply` help still says `Exa API keys`: `cmd/usync/apply_command.go:31`.

**Recommended architecture**

- Deprecate the root Exa-only non-interactive path or make it a compatibility alias for `usync plan --provider exa` plus `usync apply --plan`.
- Rename user-facing flags from `keys` to `credentials` where provider-neutral behavior is intended.
- Consider Cobra once command breadth grows further. Cobra is not required today, but its command constructor pattern, `RunE`, command groups, and help generation fit the current growth curve.
- Adopt the GitHub CLI pattern: command packages expose constructors, receive a factory/dependency bundle, parse flags into an options struct, and call a run function that is easy to unit test.

### P2 - Preserve User Config Formatting Where Possible

**Current evidence**

- JSON/JSONC mutation decodes into `map[string]any` and re-marshals: `pkg/config/json_update.go:196`.
- JSON comments and trailing commas are stripped manually: `pkg/config/json_update.go:223`.
- TOML mutation is custom text mutation in `pkg/config/toml_update.go`.

**Risk**

Rewriting whole config files can remove comments and formatting. That may be acceptable for some generated MCP files, but risky for broad client support where users maintain hand-edited JSONC/TOML.

**Recommended architecture**

- Mark candidates as "owned/generated" vs "user-authored" in manifest.
- For user-authored JSONC/TOML, use syntax-tree preserving libraries or narrow patching where feasible.
- Record before/after diffs in dry-run and saved-plan previews.
- Add fixture tests with comments, ordering, trailing commas, and unrelated settings.

### P2 - Build a Generic MCP Registry Provider Before External Plugins

**Current evidence**

- `docs/architecture/scalability-research.md` already points toward registry plus RPC plugin architecture.
- Current providers are statically compiled and manually registered: `pkg/provider/registry.go:8`.

**Recommended architecture**

1. Implement a cached MCP registry client:
   - `GET /v0.1/servers`
   - `search`
   - `updated_since`
   - `version=latest`
   - cursor pagination
2. Implement `GenericProvider` that maps registry `remotes`, `packages`, variables, and secret headers into `MCPConfig` plus credential specs.
3. Keep built-in providers for high-value cases requiring custom validation, migration, or UX.
4. Add external RPC plugins only after the in-process extension contract is stable.

**External reference**

The official MCP registry supports search, incremental sync, latest-version filtering, server detail endpoints, remote server metadata, secret headers, URL template variables, and package definitions. HashiCorp `go-plugin` is a good fit later because it isolates plugin crashes, supports cross-language gRPC plugins, and has protocol versioning.

### P3 - Automate Source Freshness Checks for Manifest Metadata

**Current evidence**

- Manifest entries have source URLs and `VerifiedAt` values: `pkg/manifest/types.go:111`, `pkg/manifest/providers.go:20`, `pkg/manifest/clients.go:39`.
- Many source dates are `2026-05-21`; this audit was performed on `2026-05-30`.

**Recommended architecture**

- Add `make manifest-check` that validates:
  - every URL is reachable or explicitly marked empirical
  - source dates are not older than a chosen threshold for volatile clients
  - every candidate has confidence and source provenance
- Emit stale-source warnings in doctor mode for fast-moving clients.

## Current Codebase Structure

### Commands

- `cmd/usync`: main CLI and TUI entrypoint, root compatibility flow, provider commands, plan/apply/validate/doctor/replay commands.
- `cmd/ux-explore`: state-space exploration and UX invariant tooling.

### Core Packages

- `pkg/app`: orchestration, execution plans, saved plans, apply, rollback, audit integration, CLI operation construction.
- `pkg/provider`: provider interface, built-in provider implementations, static registry, MCP transport config.
- `pkg/client`: target-client capability matrix and transport bridge adaptation.
- `pkg/config`: target file discovery, file permissions, atomic writes, JSON/TOML mutation.
- `pkg/manifest`: declarative client/provider/runtime metadata and source provenance.
- `pkg/doctor`: client/runtime discovery, conflict detection, report formatting.
- `pkg/validate`: offline/live credential validation and cache.
- `pkg/verify`: post-apply file and CLI verification.
- `pkg/tui`: Bubble Tea dashboard, legacy wizard, credential entry, previews, results, recorder, styles, golden and matrix tests.
- `pkg/uxexplore`: state-space exploration, invariants, coverage, reports, allowlists.
- `pkg/audit`, `pkg/redact`, `pkg/version`: cross-cutting support.
- Provider helper packages: `pkg/exa`, `pkg/context7`, `pkg/tavily`.

### Tests and Tooling

- Unit tests are colocated with packages.
- E2E tests live under `tests/e2e`.
- Fake production UX tests live under `tests/ux-fake-prod`.
- Golden TUI fixtures live under `pkg/tui/testdata`.
- Makefile targets wrap tidy, verify, fmt, vet, lint, test, e2e, UX matrix, fake production, and build flows.

## Technologies in Use

- Language/runtime: Go `1.24.2`.
- TUI stack: `github.com/charmbracelet/bubbletea`, `bubbles`, `huh`, `lipgloss`, `termenv`.
- TUI testing: Charm `teatest` and `golden`.
- CLI parsing: Go standard-library `flag`, with manual subcommand dispatch.
- Logging: `log/slog`.
- Config mutation: Go `encoding/json`, custom JSONC comment/trailing-comma stripping, custom TOML writer.
- File safety: atomic temp-file rename, private `0600` file permissions, `0700` directories, backup/rollback, lock files.
- Networking: Go `net/http` for live validation.
- External command execution: `os/exec` behind `CommandRunner`.
- Build/test workflow: Makefile plus shell scripts, `go test`, `go vet`, `golangci-lint`, Docker-based fake production flow.

## Architectural Strengths

- Provider abstraction exists and is simple: `ID`, `Name`, `Description`, `RequiredCredentials`, `GenerateConfig`.
- `MCPConfig` covers remote and stdio transports, headers, env, runtime, and bridge override.
- Saved-plan flow adds TTL, schema versioning, target SHA checks, symlink checks, preflight approval prompts, and audit logging.
- Config writes use private permissions, backups, rollback, and atomic rename.
- Doctor and manifest packages already express the right long-term direction: discovery plus source-backed metadata.
- Tests are unusually strong for a TUI: unit tests, golden fixtures, e2e fixtures, fake production matrix, state-space exploration, and redaction regression tests.
- The TUI keeps IO inside `tea.Cmd` closures in several core paths, which is aligned with Bubble Tea best practice.

## External Best-Practice References

### Bubble Tea

Bubble Tea recommends the Elm-style model/update/view loop, with `Init` returning initial commands, `Update` handling messages, and `View` rendering from state. Its command tutorial is directly relevant: IO should run through `tea.Cmd` and return typed messages.

Project reference: https://github.com/charmbracelet/bubbletea  
Command tutorial: https://github.com/charmbracelet/bubbletea/blob/master/tutorials/commands/README.md

### Lazygit

Lazygit documents a mature TUI decomposition: startup in `pkg/app`, command execution in dedicated command packages, GUI contexts for view-specific state, controllers for keybindings and handlers, helpers for shared behavior, and presentation packages for rendering. It explicitly acknowledges the cost of a GUI god struct and the ongoing effort to split it.

Project reference: https://github.com/jesseduffield/lazygit/blob/master/docs/dev/Codebase_Guide.md

### K9s

K9s separates command bootstrap, app view, models, watch factories, page stacks, command history, and background watchers. The relevant lesson is not to copy its framework, but to keep live background work, command interpretation, and view/component injection as separate concerns.

Project reference: https://github.com/derailed/k9s/blob/master/internal/view/app.go

### GitHub CLI and Cobra

GitHub CLI uses a minimal `cmd/gh/main.go`, a root Cobra command, per-command packages, command constructors, dependency factories, command-specific options structs, and test injection through run functions. Cobra's own docs recommend modular command packages, `RunE`, command groups, aliases, and generated help as a CLI grows.

GitHub CLI layout: https://github.com/cli/cli/blob/trunk/docs/project-layout.md  
Cobra command guidance: https://cobra.dev/docs/how-to-guides/working-with-commands/

### HashiCorp `go-plugin`

HashiCorp's plugin model launches plugins as subprocesses and communicates through RPC or gRPC. It provides crash isolation, cross-language plugins, protocol versioning, checksum/TLS options, and a natural Go interface boundary. This is a strong fit for future community provider plugins, but should come after the in-process provider and generic registry contracts stabilize.

Project reference: https://github.com/hashicorp/go-plugin/blob/main/README.md

### MCP Registry and MCP Transport Model

The official MCP registry exposes server listing, search, cursor pagination, incremental sync, latest-version filtering, and detail endpoints. Remote server metadata supports `streamable-http`, `sse`, URL variables, secret headers, and coexistence with package-based stdio installation. MCP architecture separates data-layer JSON-RPC from transport-layer connection and auth, which supports the recommendation to keep provider transport canonical and move client-specific fields into target adapters.

Registry API: https://github.com/modelcontextprotocol/registry/blob/main/docs/reference/api/official-registry-api.md  
Remote server metadata: https://modelcontextprotocol.io/registry/remote-servers  
Architecture overview: https://modelcontextprotocol.io/docs/learn/architecture

### Exa MCP

Exa's current docs show a hosted remote MCP server URL, multiple client-specific config shapes, optional tool selection through `tools`, and API key support through `x-api-key` headers for production/rate-limit use. This should drive a change away from embedding Exa credentials in URL query parameters as the default.

Project reference: https://exa.ai/docs/reference/exa-mcp

## Target Architecture

### Layer 1: Domain Core

Responsibilities:

- Build execution plans.
- Build saved plans.
- Enforce schema, expiry, path, symlink, SHA, and approval rules.
- Execute writes through atomic writer.
- Execute external CLI operations through `CommandRunner`.
- Roll back, audit, verify, and format results.

Non-responsibilities:

- Knowing target-specific root keys.
- Knowing target-specific URL field names.
- Knowing which clients support which transports.
- Knowing provider-specific redaction formats.

### Layer 2: Provider Registry

Provider registry should support:

- Built-in provider descriptors.
- Generic MCP registry providers.
- Optional external plugins later.
- Credential specs and validation.
- Secret redaction.
- Runtime requirements.
- Config generation into canonical `MCPConfig`.

### Layer 3: Target Adapter Registry

Target adapters should support:

- Capability decision: native, bridged, unsafe, unsupported.
- Safe transport adaptation.
- Config mutation.
- Config verification.
- Client-specific approval prompts.
- Manifest-backed source provenance.

### Layer 4: UI and CLI Shells

CLI and TUI should:

- Depend on provider and target adapter registries through interfaces.
- Keep command-specific parsing separate from business logic.
- Keep TUI screens as composable Bubble Tea models.
- Use saved plans as the common path for interactive and non-interactive apply.

## Suggested Roadmap

### Phase 1: Security and Transport Cleanup

- Add provider secret redaction contract.
- Move Exa API key to `x-api-key` header or stdio env fallback.
- Remove unsafe secret-in-argument bridge patterns.
- Standardize canonical remote transport as `streamable-http` unless provider docs explicitly require otherwise.
- Add redaction tests across headers, env, args, URLs, logs, saved plans, dry-run output, and verification output.

### Phase 2: Adapter Extraction

- Define `TargetAdapter`.
- Move config mutation and verification branches out of `pkg/app` and `pkg/verify`.
- Derive adapter metadata from `pkg/manifest`.
- Add adapter golden tests covering every provider/target combination.

### Phase 3: Manifest Unification

- Make manifest IDs canonical.
- Generate or validate provider registry, client capability registry, runtime checks, and docs/source metadata from manifest.
- Add stale-source and registry consistency checks.

### Phase 4: TUI and CLI Modularization

- Split `DashboardModel` into screen models.
- Inject registries and stores.
- Deprecate Exa-specific root non-interactive path.
- Move CLI subcommands to constructor/factory pattern, with optional Cobra adoption.

### Phase 5: Registry and Plugin Scalability

- Add cached official MCP registry client.
- Implement `GenericProvider` for registry servers with `remotes` and `packages`.
- Define stable provider SDK.
- Add `go-plugin` host support only for providers that need custom local logic, OAuth, migration, or discovery.

## Bottom Line

This codebase is already on the right path: provider contracts, manifest metadata, doctor discovery, saved plans, and strong TUI testing are the right primitives. The next architectural step is to remove hard-coded client/provider behavior from the orchestration layer and tighten the secret model. After that, generic registry-backed providers and optional RPC plugins can scale the project without turning every new MCP server into a core-code edit.
