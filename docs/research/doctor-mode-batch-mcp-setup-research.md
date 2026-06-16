# Doctor Mode And Batch MCP Setup Research

Date: 2026-05-21

## Context

This research note captures the findings from investigating recent MCP client config drift, especially the Antigravity path change handled by the last commit:

```text
b98a59c fix: update Antigravity config paths in documentation and tests
```

That commit changed Antigravity IDE config references from:

```text
~/.gemini/antigravity/mcp_config.json
```

to:

```text
~/.gemini/config/mcp_config.json
```

The change appears tied to the Google I/O 2026 Antigravity update, where Google announced Antigravity 2.0 and Antigravity CLI. The important conclusion is broader than one path: MCP client config locations and schemas are still moving quickly, and this project should treat config detection as an inventory and validation problem rather than a static table of paths.

## Main Finding

The sustainable design is:

```text
doctor -> credential validation -> batch target selection -> preview -> batch apply -> post-apply verification
```

This project should become a human-controlled MCP setup layer. It should discover local AI tools, identify the correct MCP config candidates, validate credentials separately, generate a redacted batch plan, and apply all selected targets together with backups and rollback.

## Why This Is Needed

MCP client configuration is fragmented. Different clients use different:

- config files
- root keys
- transport field names
- URL field names
- JSON/TOML formats
- user-level versus project-level scopes
- CLI-managed versus file-managed flows

A 2026 MCP RFC explicitly describes this fragmentation and gives examples of differing schemas across Claude Code, VS Code, Gemini CLI, Codex, OpenCode, and others.

Source: https://github.com/modelcontextprotocol/modelcontextprotocol/issues/2219

## Current Client Evidence

### Antigravity

Google announced Antigravity 2.0, Antigravity CLI, SDK access, and broader ecosystem changes at Google I/O 2026.

Sources:

- https://antigravity.google/blog/google-io-2026
- https://blog.google/innovation-and-ai/technology/developers-tools/google-io-2026-developer-highlights/

The public Antigravity MCP docs still describe "View raw config" and state the config file is:

```text
~/.gemini/antigravity/mcp_config.json
```

Source: https://antigravity.google/docs/mcp

However, this repo's last commit changed Antigravity IDE to:

```text
~/.gemini/config/mcp_config.json
```

Conclusion: Antigravity should be treated as a high-drift client. Doctor mode should know both the legacy/current-docs path and the new observed I/O-era path, then report confidence and conflicts.

### VS Code

VS Code MCP config is stored in `mcp.json`. It can be workspace-level:

```text
.vscode/mcp.json
```

or in the user profile. The native root key is:

```json
{
  "servers": {}
}
```

For remote servers, VS Code uses `type: "http"` or `type: "sse"` and `url`.

Source: https://code.visualstudio.com/docs/copilot/reference/mcp-configuration

### Windsurf

Current Windsurf docs say Cascade's MCP config is:

```text
~/.codeium/mcp_config.json
```

The file uses:

```json
{
  "mcpServers": {}
}
```

Remote HTTP MCP examples use `serverUrl` or `url`, plus optional `headers`.

Source: https://docs.windsurf.com/plugins/cascade/mcp

This differs from older paths such as:

```text
~/.codeium/windsurf/mcp_config.json
```

Conclusion: Windsurf also needs candidate-path detection and migration behavior.

### Kiro

Kiro CLI documents these MCP config scopes:

```text
~/.kiro/settings/mcp.json
.kiro/settings/mcp.json
```

It uses:

```json
{
  "mcpServers": {}
}
```

Kiro also supports agent-level MCP configuration and an `includeMcpJson` option that controls whether an agent includes workspace/user MCP files.

Sources:

- https://kiro.dev/docs/cli/chat/configuration/
- https://kiro.dev/docs/cli/mcp/

### OpenCode

OpenCode uses layered JSON/JSONC config. Important locations include:

```text
~/.config/opencode/opencode.json
opencode.json
```

It stores MCP servers under:

```json
{
  "mcp": {}
}
```

Local MCP servers use:

```json
{
  "type": "local",
  "command": ["npx", "-y", "server"],
  "environment": {}
}
```

Remote MCP servers use:

```json
{
  "type": "remote",
  "url": "https://example.com/mcp",
  "headers": {}
}
```

Sources:

- https://opencode.ai/docs/config/
- https://frank.dev.opencode.ai/docs/mcp-servers/

### Codex

Codex uses TOML. User-level config lives at:

```text
~/.codex/config.toml
```

Trusted project overrides can live at:

```text
.codex/config.toml
```

MCP config is under:

```toml
[mcp_servers.<name>]
```

Codex supports stdio fields such as `command`, `args`, `env`, `cwd`, and HTTP fields such as `url`, `bearer_token_env_var`, `http_headers`, and `env_http_headers`.

Source: https://developers.openai.com/codex/config-reference

### Context7 Client Examples

Context7 maintains a broad client configuration guide. It documents examples for Claude Code, Cursor, OpenCode, Codex, Antigravity, VS Code, Kiro, Roo Code, Windsurf, Gemini CLI, and others.

Important details from the Context7 docs:

- Context7 remote URL: `https://mcp.context7.com/mcp`
- API key header: `CONTEXT7_API_KEY`
- OAuth endpoint: `https://mcp.context7.com/mcp/oauth`
- OAuth is only for remote HTTP connections.

Source: https://context7.com/docs/resources/all-clients

## Feasibility In This Repo

Feasibility is high because the repo already has most of the lower-level machinery:

- `pkg/config`: app config detection and target file metadata
- `pkg/provider`: provider-neutral `MCPProvider`, `MCPConfig`, and credential specs
- `pkg/client`: client transport adaptation and capability rules
- `pkg/app`: planning, preview, apply, backups, rollback, and verification orchestration
- `pkg/verify`: file-level verification
- `pkg/redact`: secret redaction

The missing layer is a richer read-only inventory/doctor model before planning.

## Proposed Doctor Model

Add a `pkg/doctor` package that produces a read-only report.

```go
type DoctorReport struct {
    Clients     []ClientFinding
    Credentials []CredentialFinding
    Warnings    []string
}

type ClientFinding struct {
    AppID             config.AppID
    Name              string
    Installed         bool
    Evidence          []Evidence
    Candidates        []ConfigCandidate
    RecommendedTarget string
    Confidence        string // high | medium | low | conflict
    Issues            []string
}

type ConfigCandidate struct {
    Path             string
    Scope            string // user | project | legacy | managed
    Exists           bool
    ParseStatus      string // ok | missing | empty | invalid
    RootKey          string
    Writable         bool
    ContainsProvider bool
    LastModified     string
}

type Evidence struct {
    Kind   string // file | dir | cli | docs | env
    Value  string
    Detail string
}
```

Doctor mode should scan:

- known config candidate paths
- app install indicators
- CLI availability such as `claude`, `codex`, `gemini`, `antigravity`, `kiro-cli`, `opencode`
- existing config shape and root key
- file writability
- whether legacy and new paths both exist
- whether a provider is already configured
- whether a config file is malformed, empty, missing, or valid

Doctor mode should not write files.

## Candidate Path Policy

Each app should have a client adapter or manifest with candidate paths and source metadata.

Example:

```go
type ClientManifest struct {
    AppID       config.AppID
    Name        string
    DocsURL     string
    VerifiedAt  string
    Candidates  []PathCandidate
    RootKey     string
    Format      string // json | toml | jsonc
    Transports  []provider.TransportType
}

type PathCandidate struct {
    PathTemplate string
    Scope        string
    Status       string // canonical | legacy | observed | fallback
    Confidence   string
}
```

This prevents literals from being scattered across app tests, e2e fixtures, docs, and apply code.

## Batch Selection UX

Doctor should feed batch target selection.

Example terminal/TUI output:

```text
Detected MCP clients

[x] Codex CLI        ~/.codex/config.toml                 high
[x] Windsurf         ~/.codeium/mcp_config.json           high
[x] Kiro             ~/.kiro/settings/mcp.json            high
[!] Antigravity IDE  two possible paths found             conflict
[ ] VS Code          .vscode/mcp.json missing             creatable
```

The user should be able to select multiple targets and apply them as one reviewed batch. This is better than one-by-one setup because it:

- reduces repeated credential entry
- enables one full preview
- preserves a single rollback boundary for file writes
- makes config conflicts visible before writing
- matches how users think about "set up MCP on this machine"

## Batch Apply Flow

Recommended flow:

1. Run doctor scan.
2. Collect/select provider.
3. Validate credentials offline.
4. Optionally validate credentials live.
5. Select target clients in one batch.
6. Generate one redacted plan.
7. Apply all file operations with backups.
8. Run CLI operations after file preflight.
9. Verify file and CLI state.
10. Print restart/reload guidance per affected app.

File writes are already close to transactional because `Manager.Apply` prepares operations, writes with backups, and rolls back prior file writes on later failure.

CLI-managed mutations such as Claude Code are less transactional. They should be separated in reporting and ideally run after file preflight. If possible, CLI adapters should include their own remove/add rollback strategy.

## Credential Validation

Credential validation should be a separate step from config generation.

Proposed provider extension:

```go
type CredentialVerifier interface {
    ValidateOffline(values map[string]string) []CredentialCheck
    ValidateLive(ctx context.Context, values map[string]string) []CredentialCheck
}

type CredentialCheck struct {
    Key     string
    Status  string // ok | warning | failed | skipped
    Message string
}
```

### Offline Validation

Offline validation should run automatically. It checks:

- required fields present
- known key prefixes or regexes
- multi-value parsing
- duplicate credentials
- obviously malformed values

This never touches the network.

### Live Validation

Live validation should be explicit because it may:

- consume provider quota
- expose a key to the provider over the network
- fail due to network restrictions unrelated to key validity
- require provider-specific low-cost endpoints

The UI should label live validation as optional.

## Provider-Specific Credential Findings

### Exa

Exa API requests use `x-api-key`. Docs describe `401 INVALID_API_KEY`, `402` budget errors, and related auth tags.

Sources:

- https://exa.ai/docs/reference/error-codes
- https://exa.ai/docs/reference/search-api-guide-for-coding-agents
- https://exa.ai/docs/reference/quickstart

No dedicated key validation endpoint was found in this research. A live Exa check would probably need a minimal `/search` request. That should be opt-in and documented as potentially consuming quota.

### Tavily

Tavily has a `/usage` endpoint that returns API key and account usage details. It uses:

```text
GET https://api.tavily.com/usage
Authorization: Bearer <token>
```

Source: https://docs.tavily.com/documentation/api-reference/endpoint/usage

Tavily also has an enterprise `/key-info` endpoint, but docs state it is enterprise-only.

Source: https://docs.tavily.com/documentation/enterprise/key-info

### Context7

Context7 keys currently use this documented format:

```text
ctx7sk-**********************
```

Context7 docs show authenticated calls using:

```text
Authorization: Bearer YOUR_API_KEY
```

Sources:

- https://context7.com/docs/howto/api-keys
- https://context7.com/docs/security/best-practices

Live validation can use a small documentation API request, but it should be optional.

## Safety Requirements

Doctor mode and batch apply must preserve the repo's existing security posture:

- never print full keys
- never print secret-bearing URLs
- never print full headers with secrets
- never print generated CLI args containing secrets
- keep dry-run output redacted
- create backups before modifying existing config
- preserve rollback behavior
- avoid writing outside the configured home directory
- preserve existing servers in config files
- do not overwrite malformed files without backup or clear reporting

## Suggested Implementation Phases

### Phase 1: Read-Only Doctor

- Add `pkg/doctor`.
- Move path candidate knowledge into structured client manifests.
- Scan candidate paths and parse JSON/TOML enough to report status.
- Add `usync doctor`.
- Add `--json` output for automation.

### Phase 2: Planner Integration

- Let doctor findings feed target selection.
- Replace blind `mapAllSelected(manager.Apps)` behavior in non-interactive mode with explicit `--all`, `--targets`, or doctor-derived defaults.
- Add conflict handling for Antigravity and Windsurf path migrations.

### Phase 3: Credential Validation

- Add offline validation as a provider interface.
- Add optional live validation as a separate command or TUI step.
- Add provider-specific live checks for Tavily first because `/usage` is clean.
- Add Exa live check only with clear quota warning.

### Phase 4: Batch UX

- Update TUI to run doctor first.
- Show detected clients and confidence.
- Allow selecting multiple targets.
- Show one redacted preview.
- Apply as a batch.

### Phase 5: Verification And Migration

- Add per-client post-apply verification hooks.
- Add migration warnings for old/new path conflicts.
- Add source metadata to report docs URL and verification date.

## Open Questions

- Should Antigravity write only the new `~/.gemini/config/mcp_config.json`, or should doctor offer a migration from `~/.gemini/antigravity/mcp_config.json` when both exist?
- Should project-level configs be included by default, or should default batch mode target only user-level configs?
- Should CLI-managed clients be mutated through their CLIs when available, or should file writes remain the preferred stable path?
- How should managed/enterprise policy configs be detected and reported without trying to override them?
- Should live credential validation be available in CI, or only interactive/TUI flows?

## Conclusion

The feasible long-term design is not a larger hardcoded path table. It is an inventory-first MCP setup control plane:

- detect local AI tools
- inspect all known candidate config files
- report confidence and conflicts
- validate credentials separately
- generate one batch plan
- apply selected targets safely
- verify the result

That design directly addresses the class of issues exposed by the Antigravity update and makes future MCP client changes easier to absorb.

## Architecture Verdict: MCP Mode Versus Local Setup Tool

Follow-up research compared two possible directions:

1. keep this project as a VM/package-level MCP setup flow tool
2. wire this project as an MCP server mode that can call other MCPs

The recommendation is to keep `usync` primarily as a local MCP setup, doctor, and batch-apply tool. An optional MCP server mode is feasible, but it should be a thin control adapter for agent-assisted setup, not a runtime gateway that proxies arbitrary upstream MCP tools.

### Why The Setup Tool Should Stay Primary

The Antigravity issue was caused by config-location drift. The sustainable fix is a stronger local setup layer:

- detect installed AI clients
- inspect known and discovered config locations
- validate credentials separately
- generate a redacted batch plan
- let the user select targets
- apply selected config mutations together
- verify and rollback safely

That maps directly onto the repository's existing shape: provider config generation, file mutation, backup, rollback, and verification.

### What MCP Gateway Mode Would Mean

The MCP specification defines hosts, clients, and servers. A host creates MCP clients to connect to MCP servers. If `usync` becomes a server that calls other MCPs, it becomes both a downstream MCP server and an upstream MCP host/client:

```text
AI host -> usync MCP server -> usync MCP clients -> upstream MCP servers
```

That is a runtime data-plane product, not just a setup tool. It would need to handle:

- MCP server lifecycle
- MCP client lifecycle
- `tools/list` and `tools/call`
- capability negotiation
- stdio and streamable HTTP transports
- session state
- tool name conflicts and namespacing
- upstream authentication
- secret isolation
- user consent and approval flows
- timeout, logging, and rate-limit behavior
- upstream tool errors and result mapping

This is closer to a gateway or router product than to the current CLI/TUI scope.

### Comparison

| Direction | Meaning | Fit For This Repo | Risk |
| --- | --- | --- | --- |
| Local setup and doctor tool | Detect clients, locate config files, validate keys, plan, batch apply, verify | High | Low to medium |
| Optional MCP mode for setup | Expose doctor, plan, validate, apply, and verify as MCP tools | Medium to high | Medium |
| MCP that calls other MCPs | Federate and proxy arbitrary upstream MCP servers and tools | Low today | High |

### Recommended Optional MCP Tool Surface

If MCP mode is added, it should expose setup operations only:

```text
usync.doctor()
usync.list_supported_clients()
usync.plan_install(provider, targets)
usync.validate_credentials(provider)
usync.apply_plan(plan_id)
usync.verify_targets(plan_id)
```

The MCP adapter should be thin and should call the same internal APIs as the CLI/TUI. It should not introduce a second config engine.

Suggested package shape:

```text
pkg/doctor      client detection and config candidates
pkg/provider    MCP provider config generation
pkg/config      file mutation and format handling
pkg/app         planning, backup, apply, rollback, verify
cmd/usync       CLI/TUI
cmd/usync-mcp   optional MCP adapter
```

### Guardrails For MCP Mode

- `doctor` must be read-only.
- `plan_install` must return redacted diffs only.
- credential validation must be a separate workflow step.
- `apply_plan` must require explicit local approval.
- remote MCP use should refuse filesystem writes unless explicitly allowed.
- no full secrets should appear in logs, prompts, tool results, URLs, headers, or config previews.
- MCP mode should configure other MCP servers, not invoke arbitrary upstream tools on behalf of the host.

### Final Verdict

Build doctor mode and batch apply first. Add credential validation as a separate step. Add optional MCP server mode only for agent-assisted configuration. Defer "call other MCPs" gateway behavior unless the project intentionally expands into a runtime MCP gateway.

Additional sources:

- https://modelcontextprotocol.io/specification/latest
- https://modelcontextprotocol.io/docs/learn/architecture
- https://modelcontextprotocol.io/legacy/concepts/tools
- https://github.com/microsoft/mcp-gateway
- https://modelcontextprotocol.io/registry/registry-aggregators.md

## Codebase Scan: Upgrade Approaches And Safe Path

This section records the follow-up codebase scan and Exa advanced-search findings for upgrading the current implementation safely.

### Current Codebase Strengths

The existing implementation already has several pieces that should be preserved:

- `pkg/provider` defines a provider-neutral `MCPProvider`, `MCPConfig`, transport types, credential specs, and credential profiles.
- `pkg/client` centralizes client transport support and bridge behavior through `Matrix`, `CanHandle`, and `Adapt`.
- `pkg/app.Manager.PrepareProvider` already creates provider-aware operations across selected apps.
- `pkg/app.Manager.Apply` preflights file operations, writes files with backups, rolls file writes back on later file-write failure, and verifies results.
- `pkg/config/files.go` writes atomically, creates private parent directories, and enforces `0600` config files.
- `pkg/app.validatePathWithinHome` prevents planned writes from escaping the configured home directory.
- `pkg/verify` has provider-aware and generic file verification paths.
- e2e fixtures already cover many supported clients and providers.

These are the right foundations for doctor mode. The upgrade should build on them instead of replacing the apply engine.

### Current Codebase Gaps

The scan also found constraints that make a direct "just add more paths" approach brittle:

- `pkg/config/paths.go` still mixes client identity, candidate paths, OS rules, file format, and creatability in one static detector.
- `chooseExistingPath` collapses competing candidates into one path. Doctor mode needs to report all candidates, conflicts, and confidence instead.
- current non-interactive CLI behavior selects every detected app by default through `mapAllSelected`; a doctor-driven batch flow should require explicit `--all`, `--targets`, or a selected doctor plan.
- project-level configs are not modeled as a separate scope, even though Codex, Kiro, VS Code, and OpenCode all have workspace/project behavior.
- `pkg/config/json_update.go` has direct `fmt.Printf("DEBUG: ...")` calls inside library mutation functions. These should be removed before doctor/batch output is treated as stable, because library writes should not emit unredacted or unexpected stdout.
- JSON mutators use `ensureObject`, which silently replaces a non-object root key. Doctor mode should flag wrong-shape config before apply instead of correcting it silently.
- file writes have a rollback boundary, but CLI-managed changes such as Claude Code are less transactional. CLI mutations should be reported separately and ideally run after file writes.
- provider credential validation is currently mostly input-format validation. Live credential validation needs a separate interface and explicit user consent.

### External Findings From Exa Advanced Search

Exa advanced search and source fetches confirm that client config behavior is fragmented and precedence-sensitive:

- Codex stores MCP config in `~/.codex/config.toml`, also supports trusted project `.codex/config.toml`, and supports stdio plus streamable HTTP. Codex also supports environment-backed HTTP headers and tool approval settings.
- Codex config precedence includes CLI/profile overrides, trusted project config, user config, and system config. Doctor mode must report when a user-level write may be overridden by project or system config.
- VS Code stores MCP config in `mcp.json`, either workspace `.vscode/mcp.json` or user profile. The root key is `servers`, and sensitive values can be modeled with `inputs`.
- VS Code supports sandbox settings for local stdio MCP servers on macOS and Linux. Future stdio provider support should consider writing sandbox metadata where appropriate.
- Kiro supports user-level `~/.kiro/settings/mcp.json` and workspace `.kiro/settings/mcp.json`; workspace config takes precedence. It recommends environment variable references for sensitive data.
- OpenCode has a layered config model with remote org defaults, global `~/.config/opencode/opencode.json`, custom env-config paths, project `opencode.json`, inline config, and managed settings. Managed settings should be detected and treated as not user-overridable.
- Windsurf documentation and GitHub MCP installation guides point to `~/.codeium/windsurf/mcp_config.json`, with global scope and valid JSON requirements. Some guides also note Windsurf-specific limitations such as no environment-variable interpolation.
- Public Antigravity examples still show `~/.gemini/antigravity/mcp_config.json`, while this repo's latest commit moved Antigravity IDE to `~/.gemini/config/mcp_config.json`. Doctor mode should treat both as candidates and report which one appears active instead of assuming a single truth from docs.

Sources:

- https://developers.openai.com/codex/mcp
- https://developers.openai.com/codex/config-basic
- https://code.visualstudio.com/docs/copilot/reference/mcp-configuration
- https://kiro.dev/docs/mcp/configuration/
- https://dev.opencode.ai/docs/config/
- https://github.com/github/github-mcp-server/blob/main/docs/installation-guides/install-antigravity.md
- https://github.com/github/github-mcp-server/blob/refs/heads/main/docs/installation-guides/install-windsurf.md

### Upgrade Approaches Considered

#### Approach A: Minimal Doctor Wrapper

Add `usync doctor` around the current `config.DetectAppConfigsForOS` output.

Benefits:

- fastest path
- low code movement
- can reuse existing tests quickly

Limitations:

- cannot see alternate candidate paths hidden by `chooseExistingPath`
- cannot model precedence, project/user scope, managed settings, or migration conflicts well
- risks turning doctor mode into a prettier view of the same hardcoded assumptions

This is acceptable only as a short-lived bootstrap step.

#### Approach B: Manifest-Driven Doctor And Planner

Move client path knowledge into structured manifests, scan every candidate, produce a doctor report, then derive apply targets from selected findings.

This is the recommended path.

The manifest should describe:

```go
type ClientManifest struct {
    AppID       config.AppID
    Name        string
    Platforms   []string
    Candidates  []ConfigCandidate
    Capabilities client.Capability
    Docs        []SourceRef
}

type ConfigCandidate struct {
    Label       string
    Path        string
    Scope       string // user, workspace, project, managed, legacy
    Kind        config.FileKind
    RootKey     string
    URLField    string
    Creatable   bool
    Confidence  string // high, medium, low
    Precedence  int
    Deprecated  bool
    ReplacedBy  string
}
```

Doctor should produce findings without writing:

```go
type ClientFinding struct {
    AppID      config.AppID
    Name       string
    Installed  bool
    Candidates []CandidateFinding
    Warnings   []string
}

type CandidateFinding struct {
    Candidate  ConfigCandidate
    Exists     bool
    ParseOK    bool
    Writable   bool
    ActiveHint bool
    Reason     string
}
```

Then `PrepareProvider` can keep using the existing `config.AppConfig` and `TargetFile` model after the user selects concrete candidate files from the doctor report.

#### Approach C: Dynamic Registry First

Use the MCP Registry or external manifests before implementing local doctor mode.

This is not the safe next step. Registry data can help with provider discovery, but it does not solve local client precedence, machine-specific config paths, managed settings, or migration conflicts. Registry integration should wait until local doctor manifests are stable.

#### Approach D: MCP Server Mode First

Expose setup operations over MCP before changing the local CLI/TUI.

This is also not the safe next step. MCP mode should be a thin adapter after the local doctor and planner APIs are stable. Otherwise the MCP interface will freeze immature local behavior.

### Recommended Safe Upgrade Path

#### Step 0: Output Hygiene

Remove library-level debug `fmt.Printf` calls from `pkg/config/json_update.go`. Debug output should go through `Manager.Logger` with redaction, or be absent.

Acceptance:

- `make test`
- dry-run output contains only intentional plan text
- no config mutator writes directly to stdout

#### Step 1: Introduce Read-Only Manifest Layer

Add a new package, likely `pkg/doctor`, with client manifests and candidate paths. Initially, generate manifests that reproduce the current `DetectAppConfigsForOS` behavior.

Acceptance:

- manifest tests assert current macOS and Linux paths
- current `DetectAppConfigsForOS` can be implemented from manifests or compared against them
- no behavior changes in apply

#### Step 2: Enumerate Candidates Instead Of Choosing One

Replace path collapse logic with candidate reporting. For Windsurf and Antigravity, report old and new known paths with confidence and deprecation status.

Acceptance:

- doctor report can show multiple candidates for one app
- old Antigravity path and new Antigravity path can both appear
- no automatic migration happens in read-only doctor

#### Step 3: Add Parse And Shape Preflight

Doctor should parse candidate files and classify them:

- missing but creatable
- exists and parseable
- exists but malformed
- exists but root key has wrong type
- exists but not writable
- exists but overridden by higher-precedence config

Acceptance:

- malformed JSON/TOML blocks apply
- wrong-shape root keys warn before apply
- missing parent directories are reported as creatable, not silently assumed safe

#### Step 4: Add Doctor JSON Output

Add `usync doctor --json` so tests and future MCP mode can consume stable output.

Acceptance:

- JSON report contains app ID, candidate path, scope, status, confidence, warnings, and source metadata
- no secrets are included
- output is deterministic for fixture homes

#### Step 5: Connect Doctor To Planning

Add a planner input that accepts selected candidate IDs from the doctor report. Avoid selecting every app by default in non-interactive batch mode unless the user passes `--all`.

Acceptance:

- `--dry-run --targets codex-cli,windsurf` only plans selected targets
- `--apply` without explicit selection does not unexpectedly mutate every supported client
- TUI can select from doctor findings instead of raw hardcoded app list

#### Step 6: Credential Validation As A Separate Stage

Add provider interfaces for offline and optional live validation:

```go
type OfflineCredentialValidator interface {
    ValidateCredentialsOffline(profile provider.CredentialProfile) []ValidationResult
}

type LiveCredentialValidator interface {
    ValidateCredentialsLive(ctx context.Context, profile provider.CredentialProfile) []ValidationResult
}
```

Live validation must be opt-in and separate from planning/apply.

Acceptance:

- offline validation runs before plan generation
- live validation can be skipped
- validation output is redacted
- provider implementations can warn about quota-costing validation

#### Step 7: Keep File Batch Apply And CLI Apply Separate

Preserve the current file transaction behavior. Run file preflight first, file writes second, verification third. Treat external CLI mutations as a separate phase because they cannot share the same rollback guarantees.

Acceptance:

- file writes still roll back on later file-write failure
- CLI mutations are clearly labeled as non-file operations
- failed CLI mutations do not imply file rollback unless explicitly designed

#### Step 8: Add Migration Hints, Not Silent Migration

For Antigravity and other path changes, doctor should recommend migration but not silently move or delete files.

Possible statuses:

- `active-current`
- `legacy-present`
- `conflict-current-and-legacy`
- `missing-creatable`
- `managed-not-user-writable`

Acceptance:

- both Antigravity paths are visible when both exist
- plan writes only to the user-selected path
- migration actions require explicit confirmation

#### Step 9: Optional MCP Adapter After Local APIs Stabilize

Only after doctor and planner APIs are stable, add optional MCP mode as a setup adapter:

```text
usync.doctor
usync.plan_install
usync.validate_credentials
usync.apply_plan
usync.verify_targets
```

It should call the same local APIs as CLI/TUI and should not proxy arbitrary upstream MCP tools.

### Safe Path Summary

The safest upgrade sequence is:

```text
remove noisy debug output
add manifests
add read-only doctor
enumerate candidate paths
add parse/shape/permission preflight
add JSON doctor report
connect doctor selections to planner
split credential validation from apply
preserve batch file apply and rollback
add migration hints
add optional MCP adapter last
```

This path minimizes risk because each step is independently testable, keeps the existing apply engine intact, and addresses the real failure mode: config drift across clients, paths, scopes, and precedence layers.

## Improved New-User Workflow Research

This section refines the product workflow for a new user. The main design change is that onboarding should begin with system state detection, not credential entry.

Doctor mode should answer these questions before asking the user to configure anything:

- Which supported AI clients are installed or have config files?
- Which supported MCP providers are already configured?
- Which existing MCP entries are valid, stale, malformed, or in a legacy path?
- Which providers can be installed now with no credentials?
- Which providers need credentials?
- Which required local runtimes are missing, such as Node.js, npm/npx, Docker, or a provider-specific CLI?
- If credentials are missing, where should the user get them?
- If credentials are present, which targets can be updated together as one batch?

### New-User Journey

The recommended first-run journey is:

```text
1. Doctor scan
2. System status summary
3. Installed client and config inventory
4. Existing MCP inventory
5. Provider readiness check
6. Credential guidance
7. Optional credential validation
8. Batch target selection
9. Redacted preview
10. Apply selected batch
11. Post-apply verification
12. Next actions
```

The user should be able to stop after the doctor scan and still get value. A first-time run should be useful even with no API keys.

### Step 1: Doctor Scan

Doctor mode should scan the local system without writing:

- supported client config candidates
- existing config files
- parse status for JSON/TOML files
- already configured MCP server IDs
- candidate path conflicts
- legacy path presence
- file writability
- required runtimes on `PATH`
- provider credential presence only when the user explicitly supplies credentials or points to a credentials file

The scan should not search shell history, environment files, password managers, or arbitrary dotfiles for secrets. That would create privacy risk and could accidentally expose credentials in logs or reports.

### Step 2: System Status Summary

The first screen should show a compact status overview:

```text
System status

Clients
  detected: 5
  configurable now: 4
  needs attention: 1

Existing MCP servers
  configured: exa, context7
  stale or legacy: antigravity: old path also exists
  malformed configs: none

Provider readiness
  ready without keys: playwright, kubernetes, terraform
  ready with supplied keys: exa
  missing keys: github, tavily, context7
  missing runtimes: docker

Batch actions
  can update now: Codex, VS Code, Windsurf, Cursor
  blocked: Terraform provider needs Docker
```

This summary should separate facts from actions. The tool should not immediately assume that every detected client should be changed.

### Step 3: Supported Client Inventory

For each supported client, show one row with state:

```text
Client              Status          Config path                         MCP state
Codex CLI           installed       ~/.codex/config.toml                exa present
VS Code             config found    ~/.vscode/mcp.json                  no usync providers
Windsurf            config found    ~/.codeium/windsurf/mcp_config.json context7 present
Antigravity IDE     conflict        current + legacy paths              exa in legacy path
Claude Code         cli missing     ~/.claude.json not mutated          skipped
```

Recommended statuses:

- `installed`
- `config-found`
- `missing-creatable`
- `not-detected`
- `legacy-present`
- `conflict`
- `managed`
- `malformed`
- `not-writable`
- `cli-missing`

The inventory should support a `--json` output so the TUI, CLI, and future MCP mode can reuse the same report.

### Step 4: Existing MCP Inventory

Doctor mode should parse each config and list existing MCP servers without revealing secrets:

```text
Existing MCP servers

Provider    Clients                      Status
exa         Codex, Antigravity legacy     configured, migration suggested
context7    Windsurf                      configured
github      none                          not configured
tavily      none                          not configured
playwright  none                          not configured
```

For each configured provider, doctor should classify:

- `present-valid-shape`
- `present-wrong-transport`
- `present-wrong-url-field`
- `present-legacy-path`
- `present-disabled`
- `present-malformed`
- `present-secret-inline`
- `not-present`

This lets the user distinguish "not installed" from "installed but probably broken".

### Step 5: Provider Readiness

The provider list should not be a flat list. It should be grouped by readiness:

```text
Ready now
  Playwright      no key required, needs Node.js/npx
  Kubernetes      no key required, needs Node.js/npx and kubeconfig

Ready with supplied credentials
  Exa             EXA_API_KEY supplied, 4 targets available

Needs credentials
  GitHub          needs GITHUB_PERSONAL_ACCESS_TOKEN
  Tavily          needs TAVILY_API_KEY
  Context7        needs CONTEXT7_API_KEY for configured mode

Blocked by runtime
  Terraform       needs Docker running
```

This avoids forcing new users to understand every provider before they can take action.

### Step 6: Credential Guidance And Key Links

The tool should show credential acquisition links only for providers that need credentials and only after doctor confirms the provider is relevant to the user's selected workflow.

Suggested provider metadata:

```go
type CredentialAcquisition struct {
    CredentialKey string
    Required      bool
    GetURL        string
    DocsURL       string
    EnvVar        string
    FormatHint    string
    ValidationHint string
}
```

Current supported provider guidance:

| Provider | Credential | Required For Current Provider | Get Key / Token URL | Format / Notes |
| --- | --- | --- | --- | --- |
| Exa | `EXA_API_KEY` | Yes in current repo implementation | https://dashboard.exa.ai/api-keys or https://dashboard.exa.ai/login?redirect=/ | UUID-style key in current parser. Exa docs also describe hosted MCP usage with free rate limits and optional API key. |
| GitHub | `GITHUB_PERSONAL_ACCESS_TOKEN` | Yes | https://github.com/settings/personal-access-tokens/new or https://github.com/settings/tokens | Prefer fine-grained PAT when possible. Classic `repo` scope is broad and should be explained clearly. |
| Context7 | `CONTEXT7_API_KEY` | Yes in current repo implementation | https://context7.com/dashboard | Format starts with `ctx7sk`. Context7 API docs use `Authorization: Bearer CONTEXT7_API_KEY`. |
| Tavily | `TAVILY_API_KEY` | Yes | https://app.tavily.com/ | Format starts with `tvly-`. Tavily MCP supports API key in URL, Authorization header, or OAuth for compatible clients. |
| Playwright | none | No | https://playwright.dev/docs/getting-started-mcp | Needs Node.js 18+ and `npx` for current repo provider. |
| Kubernetes | none | No | Provider package docs should be linked when finalized | Needs Node.js/npx and valid Kubernetes access context. Current provider runs read-only. |
| Terraform | none for public registry default in current repo | No for current read-only/public registry mode | https://developer.hashicorp.com/terraform/mcp-server | Needs Docker. HCP Terraform/TFE features may need `TFE_TOKEN` and `TFE_ADDRESS`; destructive operations should remain disabled by default. |

Sources:

- https://exa.ai/docs/reference/quickstart
- https://exa.ai/docs/reference/exa-mcp
- https://github.com/exa-labs/exa-mcp-server/blob/main/api/mcp.ts
- https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens
- https://context7.com/docs/api-guide
- https://docs.tavily.com/documentation/quickstart
- https://docs.tavily.com/documentation/mcp
- https://docs.tavily.com/documentation/best-practices/api-key-management
- https://playwright.dev/docs/getting-started-mcp
- https://docs.npmjs.com/downloading-and-installing-node-js-and-npm
- https://docs.docker.com/desktop/
- https://developer.hashicorp.com/terraform/mcp-server
- https://developer.hashicorp.com/terraform/mcp-server/deploy
- https://developer.hashicorp.com/terraform/mcp-server/reference

### Step 7: Credential Entry Modes

Support three credential states:

```text
missing      user has not provided a value
provided     value was entered or loaded, but only offline validation ran
validated    live validation passed or user intentionally skipped live validation
```

Credential entry should allow:

- paste one key
- paste multiple keys where provider supports it, such as Exa
- load from a user-specified keys file
- skip for now and continue with no-key providers
- show key acquisition links
- run offline validation
- optionally run live validation

Live validation must be separate because it can cost quota, touch external APIs, or reveal usage patterns.

### Step 8: Runtime Readiness

Provider readiness should include local runtime checks:

| Runtime | Needed By | Check | User Guidance |
| --- | --- | --- | --- |
| Node.js | Playwright and npm-based stdio providers | `node --version` | Install Node.js 18+ where required. |
| npm/npx | Playwright, GitHub, Tavily, Context7 bridge, Kubernetes | `npx --version` | npm docs recommend installing Node.js and npm together, preferably with a version manager. |
| Docker | Terraform provider | `docker info` | Install/start Docker Desktop before applying Terraform provider. |
| Provider CLI | Claude Code direct mutation | `claude` on `PATH` | If missing, skip CLI mutation and report manual setup path. |
| Client CLIs | optional verification | `codex`, `gemini`, `antigravity` | Missing CLI should be a warning, not apply failure. |

Runtime failures should block only the affected provider/client operations.

### Step 9: Batch Actions

After doctor and credential validation, the tool should propose batch actions:

```text
Suggested batch

Install/update Exa:
  targets: Codex, VS Code, Windsurf, Cursor
  skipped: Antigravity until path conflict is resolved
  backup: 4 files
  verification: file parse + optional CLI checks

Install Playwright:
  targets: Codex, Cursor, VS Code, Windsurf
  skipped: Gemini CLI and Antigravity IDE because stdio unsupported
  prerequisite: Node.js/npx found

Install Terraform:
  blocked: Docker not running
```

The user should then choose:

```text
[ ] Apply Exa to 4 targets
[ ] Apply Playwright to 4 targets
[ ] Resolve Antigravity path conflict
[ ] Show missing-key providers
```

Batch selection should be provider-first, then target-specific. This matches how users think: "Add Exa everywhere safe" rather than "edit one file at a time."

### Step 10: Missing-Key Journey

If the user does not have keys, the workflow should not dead-end. It should produce a useful next-actions view:

```text
Missing credentials

GitHub
  needs: GITHUB_PERSONAL_ACCESS_TOKEN
  get: https://github.com/settings/personal-access-tokens/new
  note: prefer fine-grained token with minimal repository access

Tavily
  needs: TAVILY_API_KEY
  get: https://app.tavily.com/
  note: key should start with tvly-

Context7
  needs: CONTEXT7_API_KEY
  get: https://context7.com/dashboard
  note: key should start with ctx7sk

Available without keys
  Playwright can be installed now if Node.js/npx is available.
  Terraform can be installed now if Docker is available.
```

The TUI should allow the user to install no-key providers immediately, then return later with keys for the others.

### Step 11: Preview

Preview should show:

- provider
- target clients
- target files
- whether files will be created or updated
- backup paths
- detected existing server entry state
- transport per client after adaptation
- warnings and skipped targets
- redacted credential labels

Preview should not show:

- full API keys
- full Authorization headers
- secret-bearing URLs
- complete generated command arguments if they contain secrets

For secret-bearing remote URLs, display a normalized redacted form:

```text
https://mcp.tavily.com/mcp/?tavilyApiKey=tvly...abcd
https://mcp.exa.ai/mcp?exaApiKey=6ea4...7887&tools=...
```

### Step 12: Apply And Verify

Apply should keep the current file-first behavior:

```text
1. preflight every selected file operation
2. write backups
3. write all file configs
4. rollback prior file writes if a later file write fails
5. run external CLI operations as a separate phase
6. verify file state
7. run optional CLI verification
8. show restart and refresh instructions
```

Verification should report:

- provider entry exists
- config parses
- URL field is correct for the client
- transport shape is compatible with the client
- expected command/runtime exists where applicable
- key/headers are present without revealing values
- optional CLI status

### Example End-To-End New User Flow

Scenario: a user has Codex, VS Code, Windsurf, and Antigravity installed. They have an Exa key but no GitHub, Tavily, or Context7 keys. Docker is not running.

Expected journey:

```text
usync doctor

Detected:
  Codex CLI: user config found
  VS Code: workspace/user config found
  Windsurf: config found
  Antigravity IDE: current and legacy paths both present

Already configured:
  exa: Codex
  context7: Windsurf

Ready with supplied credentials:
  exa: can update Codex, VS Code, Windsurf

Needs decision:
  Antigravity: choose current path or migrate from legacy path

Missing credentials:
  github: https://github.com/settings/personal-access-tokens/new
  tavily: https://app.tavily.com/
  context7: https://context7.com/dashboard

No-key providers:
  playwright: ready if Node.js/npx available
  terraform: blocked, Docker not running
```

Then:

```text
usync plan --provider exa --targets codex-cli,vscode,windsurf
usync apply --plan <plan-id>
```

The TUI equivalent should make this a selectable batch:

```text
[x] Update Exa on Codex, VS Code, Windsurf
[ ] Resolve Antigravity migration
[ ] Install Playwright
[ ] Configure missing-key providers later
```

### Recommended CLI Shape

Add commands in this order:

```text
usync doctor
usync doctor --json
usync providers
usync providers --missing-credentials
usync validate-credentials --provider exa --keys-file ./keys.txt
usync plan --provider exa --targets codex-cli,vscode,windsurf --keys-file ./keys.txt
usync apply --plan ./usync-plan.json
```

Avoid making `--apply` infer "all clients" by default. Batch apply should be explicit:

```text
usync plan --provider exa --all-detected --keys-file ./keys.txt
usync plan --provider exa --targets codex-cli,windsurf --keys-file ./keys.txt
```

### Recommended TUI Shape

The TUI should be status-led:

```text
Dashboard
  System status
  Supported clients
  Existing MCPs
  Provider readiness
  Batch actions

Provider readiness
  Ready now
  Ready with keys
  Missing keys
  Blocked by runtime

Batch preview
  One provider or bundle at a time
  Multiple targets
  Redacted config summary
  Backups
  Warnings

Results
  Updated
  Skipped
  Verification
  Next actions
```

The first useful screen should not be a marketing or welcome page. It should be a working dashboard of the current machine state.

### Data Model Additions

Add explicit readiness and onboarding metadata:

```go
type ProviderReadiness struct {
    ProviderID       string
    Name             string
    CredentialState  string // none-required, missing, provided, validated
    RuntimeState     string // ready, missing, failed
    ExistingTargets  []config.AppID
    BatchableTargets []config.AppID
    BlockedTargets   []BlockedTarget
    KeyLinks         []CredentialAcquisition
}

type BlockedTarget struct {
    AppID  config.AppID
    Reason string
    FixURL string
}
```

Provider implementations should expose acquisition metadata without embedding product-specific URLs in the TUI:

```go
type CredentialGuideProvider interface {
    CredentialGuides() []CredentialAcquisition
}
```

Runtime checks should be provider-aware:

```go
type RuntimeRequirement struct {
    Name        string
    Command     string
    Args        []string
    RequiredFor []string
    InstallURL  string
}
```

### Final Improved Workflow Verdict

The best workflow is a diagnostic dashboard followed by explicit batch actions:

```text
doctor scan -> status dashboard -> provider readiness -> credential guidance -> optional validation -> batch plan -> redacted preview -> apply -> verify -> next actions
```

This is more sustainable than a wizard that starts with provider/key entry because it adapts to the actual machine state. It helps brand-new users, users with partial MCP setup, and users affected by path changes such as Antigravity without forcing one-by-one manual edits.
