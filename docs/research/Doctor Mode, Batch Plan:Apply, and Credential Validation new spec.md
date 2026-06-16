# `usync` Technical Specification
## Doctor Mode, Batch Plan/Apply, and Credential Validation

**Version:** 1.0  
**Status:** Decision-Ready  
**Synthesized from:** Deep Research Report (May 21, 2026), Architecture Decision Plan, Doctor Mode Research Draft  
**Project:** [`nawodyaishan/universal-mcp-sync`](https://github.com/nawodyaishan/universal-mcp-sync)

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Scope and Hard Constraints](#2-scope-and-hard-constraints)
3. [Current State Analysis](#3-current-state-analysis)
4. [Client Configuration Reference](#4-client-configuration-reference)
5. [Provider Reference](#5-provider-reference)
6. [Final Architecture Decisions](#6-final-architecture-decisions)
7. [Package Structure](#7-package-structure)
8. [Data Models](#8-data-models)
9. [CLI Command Specification](#9-cli-command-specification)
10. [TUI Screen Specification](#10-tui-screen-specification)
11. [Plan File Specification](#11-plan-file-specification)
12. [Safety Model](#12-safety-model)
13. [Credential Validation Strategy](#13-credential-validation-strategy)
14. [Batch Apply Strategy](#14-batch-apply-strategy)
15. [Migration Handling](#15-migration-handling)
16. [Implementation Phases](#16-implementation-phases)
17. [First 3 PRs — Full Acceptance Criteria](#17-first-3-prs--full-acceptance-criteria)
18. [MCP Mode Verdict](#18-mcp-mode-verdict)
19. [Risks and Open Questions](#19-risks-and-open-questions)
20. [Deferred Work](#20-deferred-work)

---

## 1. Executive Summary

`usync` becomes a **local-first MCP configuration control plane** with a `doctor → validate → plan → apply` workflow inspired by:

- **Homebrew `brew doctor`** — silent on healthy, structured output on issues, never auto-fixes
- **Terraform `plan -out` / `apply`** — saved binary plan file consumed by apply without re-planning; `-detailed-exitcode` semantics; `show -json` for CI
- **`npm audit` / `npm audit fix`** — hard separation between report and write
- **`gh auth status`** — credential readiness as first-class output, not an error state

**Three major additions, in shipping order:**

| Priority | Feature | Time-Box |
|---|---|---|
| 🔴 URGENT | Antigravity/Gemini CLI migration (June 18, 2026 sunset) | Must ship before June 10 |
| 🟠 HIGH | `usync doctor` + `pkg/manifest` (read-only scan) | Phase 1, ~2 weeks |
| 🟡 HIGH | `usync plan` + `usync apply` (saved plan file, batch) | Phase 2–3, ~4 weeks |

**What is rejected:**
- ❌ State file (client config files are the source of truth; state is over-engineering)
- ❌ MCP gateway/proxy mode (runtime data plane, different product)
- ❌ Custom secret store (read env vars; never store secrets)
- ❌ Team/remote state (local tool, local scope)
- ❌ Dynamic registry consumption (MCP Registry is publisher-side; clients haven't adopted consumer-side yet)

**What is deferred:**
- Optional `usync mcp serve` (read-only agent-assisted setup) — after Phase 3 is stable

---

## 2. Scope and Hard Constraints

### In Scope for This Spec

- `usync doctor` command (read-only system scan)
- `usync plan` command (saved plan file generation)
- `usync apply` command (plan-file-driven batch apply)
- `usync validate` command (offline + optional live credential checking)
- `usync show` command (inspect a saved plan)
- `usync migrate` subcommand (Gemini CLI → Antigravity migration)
- TUI dashboard rewrite (doctor-first, not provider-picker-first)
- `pkg/manifest` package (client metadata, candidate paths)
- `pkg/doctor` package (read-only scanner)
- Plan file format and schema
- Audit log format

### Out of Scope for This Spec

- MCP Registry integration
- `usync mcp serve` (MCP server mode)
- Windows support (macOS and Linux only, matching current README)
- Remote/team state
- Plugin/extension system
- Per-server policy DSL

### Hard Constraints (Non-Negotiable)

1. `pkg/doctor` never writes. Enforced by dependency rule.
2. Plan files never contain credential values.
3. No stdout from library functions (`pkg/config`, `pkg/provider`, etc.).
4. Apply never runs without an explicit plan file or `--dry-run` flag.
5. Antigravity `mcp_config.json` is a symlink — detect with `lstat`, write to resolved target only after user confirmation.
6. `0600` permissions on all files that ever held or could hold credential-adjacent data.
7. All user-facing output is redacted by default.

---

## 3. Current State Analysis

### Codebase Strengths (Preserve These)

| Component | What It Does Right |
|---|---|
| `pkg/provider.MCPProvider` | Clean interface: credentials → MCPConfig. Zero coupling to clients. |
| `pkg/client.Matrix` + `Adapt` | Transport capability and remote→stdio bridging in one place |
| `pkg/app.Manager.Apply` | Preflight, backup, write, rollback on failure, verify — all in sequence |
| `pkg/config/files.go` | Atomic writes via temp+rename, `0600` enforcement, private parent dirs |
| `pkg/app.validatePathWithinHome` | Boundary check prevents writes outside home dir |
| `pkg/redact` | Covers keys, URLs, headers, args |
| `pkg/verify` | Provider-aware file verification post-apply |
| `tests/e2e/` + golden fixtures | Full scenario coverage across all 12 clients |

### Codebase Gaps (Fix These)

| Gap | Location | Fix |
|---|---|---|
| `fmt.Printf("DEBUG: ...")` in library functions | `pkg/config/json_update.go` | Remove. Route through `Manager.Logger`. **Blocker for Phase 1.** |
| `chooseExistingPath` collapses candidates | `pkg/config/paths.go` | Replace with candidate enumeration in `pkg/doctor` |
| `mapAllSelected` auto-selects every app | `pkg/app/app.go` (non-interactive path) | Require explicit `--targets` or `--all-detected` after doctor |
| Static path table | `pkg/config/paths.go` | Move to `pkg/manifest`; `DetectAppConfigsForOS` becomes a wrapper |
| No concurrent write safety | `pkg/config/files.go` | Add `WriteWithLock` using O_CREATE\|O_EXCL on `.lock` sibling |
| No plan file | — | Add in Phase 2 |
| No doctor | — | Add in Phase 1 |
| No project-scope modeling | — | Add `scope` field to `ConfigCandidate` in `pkg/manifest` |

---

## 4. Client Configuration Reference

Verified as of May 21, 2026. This table is the source of truth for `pkg/manifest`.

### 4.1 Full Client Table

| Client | AppID | macOS Path | Linux Path | Format | Root Key | Scope Levels | Manager |
|---|---|---|---|---|---|---|---|
| **Claude Desktop** | `claude-desktop` | `~/Library/Application Support/Claude/claude_desktop_config.json` | `~/.config/claude-desktop/claude_desktop_config.json` | JSON | `mcpServers` | user | file |
| **Claude Code** | `claude-code` | `~/.claude.json` (user); `.mcp.json` (project) | same | JSON | `mcpServers` | local, project, user | **CLI** (`claude mcp add-json`) |
| **Cursor** | `cursor` | `~/.cursor/mcp.json` (global); `.cursor/mcp.json` (project) | same | JSON | `mcpServers` | project > global | file |
| **VS Code** | `vscode` | user profile via `MCP: Open User Configuration`; workspace `.vscode/mcp.json` | `~/.config/Code/User/mcp.json` (user) | JSON | **`servers`** ⚠️ | workspace, user | file |
| **Windsurf** | `windsurf` | `~/.codeium/windsurf/mcp_config.json` | `~/.codeium/mcp_config.json` | JSON | `mcpServers` | user | file |
| **Zed** | `zed` | `~/.config/zed/settings.json` | `~/.config/zed/settings.json` | JSON | **`context_servers`** ⚠️ | user, project | file |
| **Roo Code** | `roo-code` | `~/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings/mcp_settings.json` | `~/.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/mcp_settings.json` | JSON | `mcpServers` | global, project, VSCode | file |
| **OpenCode** | `opencode` | `~/.opencode.json` | `~/.config/opencode/opencode.json` | **JSONC** | `mcp` ⚠️ | remote-org, global, project | file |
| **Kiro** | `kiro` | `~/.kiro/settings/mcp.json` | `~/.kiro/settings/mcp.json` | JSON | `mcpServers` | agent > workspace > user | file |
| **Gemini CLI** | `gemini-cli` | `~/.gemini/settings.json` | `~/.gemini/settings.json` | JSON | `mcpServers` | user, project | file (+ `gemini mcp add`) |
| **Antigravity** | `antigravity` | `~/.gemini/antigravity/mcp_config.json` (**symlink**) | same | JSON | `mcpServers` | user | file (+ IDE UI) |
| **Codex CLI** | `codex-cli` | `~/.codex/config.toml` | `~/.codex/config.toml` | **TOML** ⚠️ | `[mcp_servers.<name>]` | user, project | file (+ `codex mcp add`) |

### 4.2 Critical Differences by Client

**VS Code** uses `servers` not `mcpServers`. Supports `inputs` for secure vars. Workspace `.vscode/mcp.json` is often committed to git — flag this in doctor.

**Zed** uses `context_servers` not `mcpServers`. Manual entries require `"source": "custom"`. Native HTTP not yet shipped — HTTP providers need `mcp-remote` bridge.

**OpenCode** uses `mcp` key with `type: "local"` or `"remote"` (not transport field names like other clients). `command` is an array. **Security warning:** stdio command executes on first load of project config — usync must warn when writing project-scope OpenCode config.

**Kiro** uses deep merge (not replace) across scope levels: agent overrides workspace overrides user. Writing to user scope may be effectively invisible if workspace config conflicts.

**Codex CLI** uses TOML with `mcp_servers` (underscore, not camelCase). A TOML syntax error breaks both Codex CLI and the VS Code Codex extension simultaneously.

**Antigravity** `mcp_config.json` is typically a symlink. Use `lstat` not `stat`. Write to resolved target only after confirming with user. Never `os.Remove` the symlink.

**Gemini CLI** sunsets June 18, 2026 for AI Pro / Ultra / free-tier. Doctor must show the sunset warning and migration path on every run until July 15, 2026.

### 4.3 Legacy Path Table

| Client | Legacy Path | Current Path | Confidence | Action |
|---|---|---|---|---|
| Antigravity (old) | `~/.gemini/config/mcp_config.json` | `~/.gemini/antigravity/mcp_config.json` | Legacy=low, Current=high | Show migration hint |
| Antigravity (pre-I/O) | `~/.gemini/antigravity/mcp_config.json` | `~/.gemini/antigravity/mcp_config.json` (**symlink**) | High | Detect symlink |
| Windsurf (old) | `~/.codeium/mcp_config.json` | `~/.codeium/windsurf/mcp_config.json` | Legacy=medium | Show migration hint |
| Gemini CLI | `~/.gemini/settings.json` | Sunset → Antigravity | High (expiry warning) | Sunset warning |
| OpenCode (old) | `~/.opencode.json` | `~/.config/opencode/opencode.json` | Legacy=low | Show migration hint |

### 4.4 Precedence Rules Per Client

```
Claude Code:   local > project > user
Cursor:        project > global
VS Code:       workspace > user
Kiro:          agent > workspace > user (deep merge)
Roo Code:      project > VSCode-auto > global
OpenCode:      remote-org > global > project > inline
Codex:         CLI/profile > trusted-project > user > system
```

Doctor must walk this chain and report the *effective* path, not just enumerate files.

---

## 5. Provider Reference

### 5.1 Full Provider Table

| Provider | ID | Transport | Auth Shape | Credential Key | Key Prefix | Get Key URL | Runtime |
|---|---|---|---|---|---|---|---|
| **Exa** | `exa` | HTTP or stdio npx | API key in URL query (`?exaApiKey=`) or `x-api-key` header | `EXA_API_KEY` | UUID (no prefix) | https://dashboard.exa.ai/api-keys | — |
| **GitHub** | `github` | stdio via npx OR remote `https://api.githubcopilot.com/mcp/` | `Authorization: Bearer <PAT>` | `GITHUB_PERSONAL_ACCESS_TOKEN` | `ghp_` or `github_pat_` | https://github.com/settings/tokens | — |
| **Context7** | `context7` | Streamable HTTP | `CONTEXT7_API_KEY` header | `CONTEXT7_API_KEY` | `ctx7sk-` | https://context7.com/dashboard | — |
| **Tavily** | `tavily` | stdio via npx | `TAVILY_API_KEY` env | `TAVILY_API_KEY` | `tvly-` | https://app.tavily.com/home | Node.js + npx |
| **Playwright** | `playwright` | stdio via npx | None | — | — | — | **Node.js 18+**, npx |
| **Kubernetes** | `kubernetes` | stdio via npx | None (uses kubeconfig/RBAC) | — | — | — | Node.js + npx |
| **Terraform** | `terraform` | stdio via Docker | None (public registry); `TFE_TOKEN` optional | `TFE_TOKEN` (optional) | — | https://app.terraform.io/app/settings/tokens | **Docker running** |

### 5.2 Credential Validation Endpoints

| Provider | Offline Check | Live Endpoint | Cost | Notes |
|---|---|---|---|---|
| Exa | UUID length + character set | None documented | N/A | No free endpoint. **Do not ship live validation.** |
| GitHub | Prefix `ghp_` or `github_pat_` | `GET https://api.github.com/user` | Free (5000 req/hr) | Returns 200 with account info, 401 on bad key |
| Context7 | Prefix `ctx7sk-` | None confirmed safe | N/A | Defer live validation |
| Tavily | Prefix `tvly-` + length | `GET https://api.tavily.com/usage` | **Free, quota-safe** | Returns usage stats, 401 on bad key |
| Playwright | n/a | `node --version`, `npx --version` | Free | Runtime check |
| Kubernetes | n/a | `kubectl version --client` | Free | Runtime check |
| Terraform | n/a | `docker info` | Free | Runtime check |

### 5.3 Runtime Requirements

```go
var RuntimeRequirements = []RuntimeRequirement{
    {Name: "node",   Command: "node",   Args: []string{"--version"}, RequiredFor: []string{"tavily","playwright","kubernetes","github"}, InstallURL: "https://nodejs.org"},
    {Name: "npx",    Command: "npx",    Args: []string{"--version"}, RequiredFor: []string{"tavily","playwright","kubernetes","github"}, InstallURL: "https://docs.npmjs.com/downloading-and-installing-node-js-and-npm"},
    {Name: "docker", Command: "docker", Args: []string{"info"},      RequiredFor: []string{"terraform"},                                InstallURL: "https://docs.docker.com/desktop/"},
}
```

---

## 6. Final Architecture Decisions

### Decision Table

| Question | Decision | Rationale |
|---|---|---|
| State file? | **No** | Config files are the source of truth. State is over-engineering. |
| Plan file? | **Yes** — binary (gob) + JSON sidecar | Terraform pattern: apply executes precisely the plan, no re-planning |
| Lock file? | **Narrow: write lock only** | `<config>.lock` via O_CREATE\|O_EXCL prevents concurrent writes |
| Manifests in which package? | **`pkg/manifest`** (new) | Pure data, importable by any layer |
| Doctor package? | **`pkg/doctor`** (new) | Read-only scanner, no writer deps |
| Doctor UX model? | **`brew doctor`** — silent success, structured warnings | Matches developer expectations |
| Plan UX model? | **`terraform plan -out`** | Saved binary plan consumed by apply; `-detailed-exitcode` |
| Credential UX model? | **`gh auth status`** | Readiness as structured output, not an error |
| Batch selection? | **Hybrid, target-first default** | Matches how users think about "set up client X" |
| TUI entry point? | **Doctor dashboard first** | Replaces provider picker on first launch |
| CLI-managed clients? | **CLI adapter, file fallback** | `claude mcp add-json` / `codex mcp add` preferred; raw file as opt-in |
| Antigravity symlink? | **lstat + confirmed write to resolved target** | Overwriting symlink breaks active MCP connections |
| MCP gateway? | **Deferred indefinitely** | Different product, different risk profile |
| MCP server mode? | **Phase 6, read-only tools only** | Only after local API surface is stable |
| Audit log? | **Yes** — JSONL at `~/.usync/audit.log` | Append-only, `0600`, no secrets |
| Project-scope writes? | **Opt-in (`--include-workspace`)** | Default targets user-level configs only |
| Auto-select all clients? | **No** — requires explicit target or `--all-detected` | `--all-detected` selects confidence `high`+`medium` only |

---

## 7. Package Structure

```
cmd/
  usync/
    main.go               # CLI entry: doctor, plan, apply, validate, show, migrate
    main_test.go

cmd/
  usync-mcp/              # Phase 6 only — optional MCP server adapter
    main.go

pkg/
  manifest/               # NEW — pure static data, no imports from internal pkgs
    manifests.go          # All 12 ClientManifest declarations
    candidates.go         # ConfigCandidate helpers (path expansion, platform filter)
    providers.go          # ProviderMeta for all 7 providers
    runtimes.go           # RuntimeRequirement table
    manifests_test.go

  doctor/                 # NEW — read-only scanner
    doctor.go             # Doctor struct, Scan() → DoctorReport
    client_scan.go        # Per-client finding generation
    runtime_check.go      # Node/npx/Docker/CLI availability
    provider_readiness.go # ProviderReadiness computation
    report.go             # DoctorReport formatting (human + JSON)
    doctor_test.go

  app/
    app.go                # Manager — unchanged PrepareProvider / Apply
    plan.go               # NEW: PlanFromDoctorSelections, LoadPlan, SavePlan
    app_test.go
    qa_scenarios_test.go

  config/
    paths.go              # DetectAppConfigsForOS → thin wrapper over pkg/manifest
    files.go              # WriteWithLock (NEW), WriteWithBackup (existing)
    json_update.go        # Remove fmt.Printf DEBUG calls (Phase 0 blocker)
    toml_update.go

  provider/
    types.go              # MCPProvider interface + optional OfflineValidator, LiveValidator
    registry.go
    exa.go, github.go, context7.go, tavily.go,
    playwright.go, kubernetes.go, terraform.go

  client/
    capabilities.go       # Matrix (source from pkg/manifest, not duplicated)
    adapter.go

  redact/
    redact.go             # Existing + RedactPlanJSON()

  verify/
    verify.go

  tui/
    model.go              # Entry point — now starts DashboardModel
    dashboard.go          # NEW: DashboardModel (doctor scan on startup)
    conflict.go           # NEW: ConflictResolutionModel
    setup_form.go         # Existing credential entry (now step 2, not step 1)
    preview.go
    results.go

  audit/                  # NEW — append-only JSONL audit log
    audit.go

docs/
  specs/
    doctor-mode.md        # This spec document
  research/
    doctor-mode-batch-mcp-setup-research.md  # (already present)
```

### Dependency Rules

```
pkg/manifest   ← no internal imports
pkg/doctor     ← pkg/manifest, pkg/redact (for output)
pkg/app        ← pkg/manifest, pkg/doctor (DoctorReport), pkg/config, pkg/client, pkg/provider, pkg/redact, pkg/verify, pkg/audit
pkg/tui        ← pkg/app, pkg/doctor, pkg/manifest, pkg/provider, pkg/config
cmd/usync      ← pkg/app, pkg/doctor, pkg/tui, pkg/audit
```

`pkg/doctor` must never import `pkg/app` or `pkg/tui`. Enforced by Go module tooling (`go vet` + a CI import-cycle check).

---

## 8. Data Models

### 8.1 `pkg/manifest` Types

```go
// ClientManifest is the single source of truth for a client's config locations.
// Pure data: no filesystem calls, no imports from other internal packages.
type ClientManifest struct {
    AppID        config.AppID
    Name         string
    Platforms    []string // "darwin", "linux"
    Candidates   []ConfigCandidate
    Capabilities client.Capability
    Manager      ManagerKind // ManagerFile | ManagerCLI
    CLIName      string      // "claude", "codex", "gemini" — for PATH check
    DocsURL      string
    Sources      []SourceRef // verified documentation sources with date
}

type ConfigCandidate struct {
    Label       string      // "user-global", "project", "legacy-windsurf"
    PathTmpl    string      // "{{.Home}}/.cursor/mcp.json" — template expanded at scan time
    Scope       ScopeKind   // ScopeUser | ScopeProject | ScopeLegacy | ScopeManaged
    Kind        config.FileKind  // FileKindJSON | FileKindJSONC | FileKindTOML
    RootKey     string      // "mcpServers", "servers", "context_servers", "mcp"
    URLField    string      // "url", "serverUrl", "httpUrl" — client-specific field name
    Creatable   bool
    Confidence  string      // "high" | "medium" | "low" — when file matches expected location
    Precedence  int         // lower = higher priority in conflict resolution
    Deprecated  bool
    ReplacedBy  string      // label of the replacing candidate
    IsSymlink   bool        // true for Antigravity — use lstat, not stat
    GitWarning  bool        // true for .vscode/mcp.json, .cursor/mcp.json — may be committed
}

type ScopeKind  string
type ManagerKind string

const (
    ScopeUser    ScopeKind = "user"
    ScopeProject ScopeKind = "project"
    ScopeLegacy  ScopeKind = "legacy"
    ScopeManaged ScopeKind = "managed"

    ManagerFile ManagerKind = "file"
    ManagerCLI  ManagerKind = "cli"
)

type SourceRef struct {
    URL       string
    Title     string
    VerifiedAt string // ISO date of last verification
}

type RuntimeRequirement struct {
    Name        string
    Command     string
    Args        []string
    RequiredFor []string // provider IDs
    InstallURL  string
}

// CredentialAcquisition is embedded in provider metadata, not in TUI code.
type CredentialAcquisition struct {
    CredentialKey  string
    EnvVar         string
    Required       bool
    FormatHint     string   // "UUID format", "starts with ctx7sk-"
    OfflineRegex   string   // compiled at runtime for prefix/length check
    GetURL         string   // direct key creation URL
    DocsURL        string
    LiveValidation *LiveValidationSpec
}

type LiveValidationSpec struct {
    Method      string // "GET"
    URL         string // "https://api.tavily.com/usage"
    AuthHeader  string // "Authorization: Bearer {key}"
    QuotaSafe   bool   // if true, call does not consume search quota
    QuotaNote   string // "Counted against your monthly usage limit"
}
```

### 8.2 `pkg/doctor` Types

```go
type DoctorReport struct {
    GeneratedAt    time.Time
    UsyncVersion   string
    Platform       string
    Clients        []ClientFinding
    Runtimes       []RuntimeFinding
    Warnings       []string // global warnings (e.g., "Gemini CLI sunsets June 18, 2026")
}

type ClientFinding struct {
    AppID          config.AppID
    Name           string
    Installed      bool         // binary or config found
    CLIAvailable   bool         // CLI binary on PATH
    Candidates     []CandidateFinding
    EffectivePath  string       // resolved after precedence walk
    Confidence     Confidence   // High | Medium | Low | Conflict
    HasMigration   *MigrationHint
    Issues         []string
    Warnings       []string     // e.g., "GitWarning: .vscode/mcp.json may be committed to git"
}

type CandidateFinding struct {
    Candidate      manifest.ConfigCandidate
    ExpandedPath   string
    IsSymlink      bool
    ResolvedPath   string       // lstat resolved path if symlink
    Exists         bool
    ParseOK        bool
    ParseError     string
    Writable       bool
    RootKeyOK      bool         // root key has correct type (object)
    RootKeyType    string       // actual JSON type if wrong ("array", "string", etc.)
    Providers      []string     // provider IDs already configured in this file
    ActiveHint     bool         // heuristic: modified most recently among candidates
    Reason         string
}

type RuntimeFinding struct {
    Name        string
    Available   bool
    Version     string
    Path        string
    RequiredFor []string
    Error       string
}

type MigrationHint struct {
    FromLabel  string
    FromPath   string
    ToLabel    string
    ToPath     string
    Reason     string
    DocsURL    string
    Deadline   string // "June 18, 2026" for Gemini CLI sunset
}

type Confidence string
const (
    ConfidenceHigh     Confidence = "high"
    ConfidenceMedium   Confidence = "medium"
    ConfidenceLow      Confidence = "low"
    ConfidenceConflict Confidence = "conflict"
)

// ProviderReadiness is computed from a DoctorReport + known credentials.
type ProviderReadiness struct {
    ProviderID       string
    Name             string
    CredentialState  CredentialState
    RuntimeState     RuntimeState
    ExistingTargets  []config.AppID  // already configured
    BatchableTargets []config.AppID  // can be applied now
    BlockedTargets   []BlockedTarget
    KeyLinks         []manifest.CredentialAcquisition
}

type CredentialState string
const (
    CredStateNoneRequired  CredentialState = "none_required"
    CredStateMissing       CredentialState = "missing"
    CredStateProvided      CredentialState = "provided_unverified"
    CredStateVerifiedLive  CredentialState = "verified_live"
    CredStateVerifiedFail  CredentialState = "verified_failed"
)

type RuntimeState string
const (
    RuntimeReady   RuntimeState = "ready"
    RuntimeMissing RuntimeState = "missing"
    RuntimeFailed  RuntimeState = "failed"
)

type BlockedTarget struct {
    AppID  config.AppID
    Reason string
    FixURL string
}
```

### 8.3 Plan File Types (`pkg/app`)

```go
// ExecutionPlanV2 is the on-disk saved plan. Saved as gob binary + JSON sidecar.
// Plan files: 0600, written to $XDG_CACHE_HOME/usync/plans/
// Apply refuses plans older than 24h without --force-stale.
type ExecutionPlanV2 struct {
    SchemaVersion  int       // current: 2
    PlanID         string    // UUID4
    CreatedAt      time.Time
    ExpiresAt      time.Time // CreatedAt + 24h
    UsyncVersion   string
    ProviderID     string
    CredentialRefs []CredentialRef // env var names only — never values
    Operations     []PlanOperation
    Warnings       []string
    DoctorSummary  DoctorSummary
}

type CredentialRef struct {
    Key    string // "EXA_API_KEY"
    Label  string // "exa_****abcd" — redacted display
    EnvVar string
}

type PlanOperation struct {
    TargetID     config.AppID
    TargetName   string
    Action       PlanAction   // ActionCreate | ActionUpdate | ActionSkip | ActionConflict
    FilePath     string       // absolute expanded path
    BackupPath   string       // "<file>.bak-usync-<timestamp>"
    CurrentSHA   string       // SHA-256 of current file contents at plan time
    Transport    string       // "http" | "stdio" | "sse"
    Manager      ManagerKind  // "file" | "cli"
    CLICommand   []string     // populated when Manager == ManagerCLI
    Redacted     string       // human-readable redacted description
    IsSymlink    bool
    ResolvedPath string       // lstat resolved path if symlink
    Warnings     []string
}

type PlanAction string
const (
    ActionCreate   PlanAction = "create"
    ActionUpdate   PlanAction = "update"
    ActionSkip     PlanAction = "skip"
    ActionConflict PlanAction = "conflict"
)

type DoctorSummary struct {
    ClientsDetected int
    ClientsReady    int
    Conflicts       int
    Warnings        []string
}
```

### 8.4 Audit Log Types (`pkg/audit`)

```go
// AuditEntry is a single JSONL line in ~/.usync/audit.log
// File: 0600, append-only, no secret values, no config content.
type AuditEntry struct {
    Timestamp   time.Time `json:"ts"`
    Command     string    `json:"cmd"`     // "plan", "apply", "doctor", "validate"
    PlanID      string    `json:"plan_id,omitempty"`
    Targets     []string  `json:"targets,omitempty"`
    FilesTouched []string `json:"files,omitempty"`
    ExitCode    int       `json:"exit_code"`
    Error       string    `json:"error,omitempty"`
}
```

### 8.5 Optional Validator Interfaces (`pkg/provider`)

Implemented via Go interface assertion — not required by all providers.

```go
// OfflineValidator may be implemented by any MCPProvider.
// Runs without network access.
type OfflineValidator interface {
    ValidateOffline(values map[string]string) []ValidationResult
}

// LiveValidator may be implemented by providers with a quota-safe validation endpoint.
// Must be called with explicit user consent only.
type LiveValidator interface {
    ValidateLive(ctx context.Context, values map[string]string) []ValidationResult
}

type ValidationResult struct {
    Key       string
    Status    ValidationStatus // ok | warning | failed | skipped
    Message   string          // never contains credential values
    QuotaCost bool            // if true, UI shows quota warning before calling
}

type ValidationStatus string
const (
    ValidationOK      ValidationStatus = "ok"
    ValidationWarning ValidationStatus = "warning"
    ValidationFailed  ValidationStatus = "failed"
    ValidationSkipped ValidationStatus = "skipped"
)
```

---

## 9. CLI Command Specification

### 9.1 Complete Command Tree

```
usync
  doctor              Read-only system scan
    --json            Machine-readable JSON output
    --clients <ids>   Limit to specific clients
    --verbose         Show all candidates, not just active
  
  plan                Generate a saved plan file
    --provider <id>   Required: exa | github | context7 | tavily | playwright | kubernetes | terraform
    --targets <ids>   Comma-separated AppIDs, or --all-detected
    --all-detected    Target all clients with confidence high/medium
    --include-workspace  Include project-scope files (opt-in)
    --keys-file <path>   Load credentials from file (one per line KEY=value)
    --out <path>      Override plan output path (default: $XDG_CACHE_HOME/usync/plans/...)
    --detailed-exitcode  0=no-changes, 1=error, 2=changes-pending (for CI)
  
  apply               Apply a saved plan file
    --plan <path>     Required: path to plan file (.bin or .json sidecar)
    --dry-run         Preview without writing (re-reads current state)
    --auto-approve    Skip interactive confirmation (CI use)
    --force-stale     Apply even if plan is older than 24h
  
  show                Inspect a saved plan
    --json            Machine-readable JSON
    <plan-path>       Plan file to inspect
  
  validate            Validate credentials
    --provider <id>   Provider to validate for
    --keys-file <path>
    --live            Run live validation (opt-in, may cost quota)
  
  migrate             Migration helpers
    gemini-to-antigravity   Preview and apply Gemini CLI → Antigravity migration
      --dry-run
      --apply
  
  plan list           List saved plans
  plan clean          Delete plans older than 7 days
  
  providers           List all providers with readiness state
    --missing-credentials   Filter to providers needing keys
  
  # Legacy commands — preserved for backward compatibility
  sync                (existing, unchanged behavior)
    --keys-file <path>
    --dry-run
    --apply
```

### 9.2 Exit Code Contract

| Code | Meaning |
|---|---|
| 0 | Success, no changes (doctor: no issues) |
| 1 | Error (parse failure, apply failure, invalid flags) |
| 2 | Changes pending (plan: changes needed; doctor: warnings found) |

With `--detailed-exitcode`: 0 = no changes, 1 = error, 2 = changes pending. Without `--detailed-exitcode`: 0 = success, 1 = error.

### 9.3 Key Flag Decisions

**`--apply` without `--plan` is an error.** Removed the old shorthand of `usync apply` implying everything. Users must provide `--plan <path>` or use `usync sync --apply` (legacy path, preserved).

**`--all-detected` selects confidence `high` + `medium` only.** Clients with `conflict` or `low` confidence are skipped and reported in plan warnings. Pass `--include-conflicts` to include them with interactive resolution.

**`--targets` uses AppIDs, not display names.** Tab-completion lists available IDs from `pkg/manifest`.

---

## 10. TUI Screen Specification

### 10.1 Screen Flow

```
┌──────────────────────────────────────────────────────────────────┐
│  SCREEN 1: Dashboard (doctor scan runs on startup, async)        │
│  ↓ [select action]                                               │
├──────────────────────────────────────────────────────────────────┤
│  SCREEN 1a: Conflict Resolution (if any client = conflict)       │
│  ↓ [resolve each conflict]                                       │
├──────────────────────────────────────────────────────────────────┤
│  SCREEN 2: Provider + Credential Entry                           │
│  ↓ [select provider, enter keys]                                 │
├──────────────────────────────────────────────────────────────────┤
│  SCREEN 3: Plan Preview (redacted)                               │
│  ↓ [confirm]                                                     │
├──────────────────────────────────────────────────────────────────┤
│  SCREEN 4: Apply + Verify + Next Actions                         │
└──────────────────────────────────────────────────────────────────┘
```

The old wizard entry (provider picker first) remains available as `usync --wizard` during the transition period.

### 10.2 Screen 1 — Dashboard

Doctor scan runs concurrently on startup. Initial render shows a spinner. On scan complete, replaces with:

```
usync — MCP Configuration Manager

System scan complete (0.8s)

Clients                                    5 detected, 4 ready, 1 conflict
Existing MCP servers                       exa: Codex | context7: Windsurf
Provider readiness                         2 ready now, 1 ready with keys, 3 need keys

─────────────────────────────────────────────────────────────────────
CLIENT               STATUS          PATH                          MCPs
Codex CLI          ✓ installed       ~/.codex/config.toml          exa
VS Code            ✓ config found    ~/.vscode/mcp.json            —
Windsurf           ✓ config found    ~/.codeium/windsurf/…         context7
Antigravity        ⚠ path conflict   current + legacy paths        —
Claude Code        - cli missing     not managed                   —

─────────────────────────────────────────────────────────────────────
⚠ Gemini CLI sunsets June 18, 2026. Migrate to Antigravity CLI.
  Run: usync migrate gemini-to-antigravity --dry-run

Ready now:    Playwright  Kubernetes
With keys:    Exa (key supplied)
Need keys:    GitHub  Tavily  Context7
Blocked:      Terraform (Docker not running)

[Configure providers]  [Resolve Antigravity conflict]  [Migrate Gemini]  [q: Exit]
```

**Design rules for Screen 1:**
- Spinner renders immediately on launch (no blank screen)
- Doctor scan has a 2-second timeout; if it takes longer, show partial results
- No welcome message, no product description on this screen
- The word "usync" appears exactly once, in the header
- Status icons: ✓ ready, ⚠ warning/conflict, - not detected, ✗ error

### 10.3 Screen 1a — Conflict Resolution

Only shown if any client has `conflict` confidence.

```
Conflict: Antigravity IDE

Two config paths found on this machine:

  [current]  ~/.gemini/antigravity/mcp_config.json  (symlink → ~/.gemini/antigravity-config/mcp_config.json)
             modified: 2026-05-19  contains: exa, playwright
  [legacy]   ~/.gemini/config/mcp_config.json
             modified: 2026-04-01  contains: exa

Recommendation: Write to [current]. The legacy path was from a pre-I/O 2026 version.

Note: current path is a SYMLINK. usync will write to the resolved target:
      ~/.gemini/antigravity-config/mcp_config.json

(o) Use current path (recommended)
( ) Use legacy path
( ) Skip Antigravity for now

[Continue]
```

### 10.4 Screen 2 — Provider + Credential Entry

```
Configure providers

Ready now (no keys required)
  [x] Playwright    needs Node.js/npx ✓
  [ ] Kubernetes    needs Node.js/npx ✓, kubeconfig ✓

Ready with supplied credentials
  [x] Exa           EXA_API_KEY ✓ (provided_unverified)

Needs credentials
  [ ] GitHub        GITHUB_PERSONAL_ACCESS_TOKEN
                    Get key: https://github.com/settings/tokens
  [ ] Tavily        TAVILY_API_KEY
                    Get key: https://app.tavily.com/home
  [ ] Context7      CONTEXT7_API_KEY
                    Get key: https://context7.com/dashboard

Enter credentials for selected providers:

EXA_API_KEY: [already loaded from env — not shown]

[Validate credentials (offline)]  [Continue to preview]  [Back]
```

### 10.5 Screen 3 — Plan Preview

```
Plan preview

Provider: Exa
Targets:  3 files

  Codex CLI         ~/.codex/config.toml        UPDATE  (exa already present — will refresh URL)
  VS Code           ~/.vscode/mcp.json           CREATE  (git warning: file may be committed)
  Windsurf          ~/.codeium/windsurf/…        UPDATE

Credential: exa_****abcd
Transport:  HTTP → https://mcp.exa.ai/mcp?exaApiKey=****abcd
Backups:    3 files, same directory, timestamp suffix

Skipped:
  Antigravity:      Conflict not resolved — run usync migrate gemini-to-antigravity first
  Claude Code:      CLI (claude) not found on PATH

⚠ VS Code: .vscode/mcp.json may be committed to git. Credentials will be in the file.
  Consider using VS Code's `inputs` feature for secure variable injection.

[Save plan and apply]  [Save plan only]  [Back]
```

### 10.6 Screen 4 — Apply + Results

```
Applying plan  usync-plan-20260521-a4f1

Phase 1: File operations

  ✓ backed up  ~/.codex/config.toml → .bak-usync-20260521-143201
  ✓ wrote      ~/.codex/config.toml
  ✓ backed up  ~/.vscode/mcp.json → .bak-usync-20260521-143201
  ✓ wrote      ~/.vscode/mcp.json
  ✓ backed up  ~/.codeium/windsurf/mcp_config.json → .bak-usync-20260521-143201
  ✓ wrote      ~/.codeium/windsurf/mcp_config.json

Phase 2: CLI operations

  - no CLI operations in this plan

Verification

  ✓ Codex CLI       exa present, TOML valid
  ✓ VS Code         exa present, JSON valid
  ✓ Windsurf        exa present, JSON valid

Next steps
  Restart Codex CLI for changes to take effect.
  Reload VS Code window (Cmd+Shift+P → "Reload Window").
  Windsurf picks up changes on next Cascade session.

Plan file: ~/Library/Caches/usync/plans/usync-plan-20260521-a4f1.bin
Audit log: ~/.usync/audit.log
```

---

## 11. Plan File Specification

### 11.1 Storage Location

```
$XDG_CACHE_HOME/usync/plans/              # Default
~/Library/Caches/usync/plans/            # macOS fallback
~/.cache/usync/plans/                    # Linux fallback
$USYNC_PLAN_DIR/                          # Override via env var
```

### 11.2 File Naming

```
usync-plan-<YYYYMMDD>-<random4hex>.bin   # Binary (gob-encoded)
usync-plan-<YYYYMMDD>-<random4hex>.json  # JSON sidecar (human-readable, redacted)
```

### 11.3 Plan File Contents

**Binary plan (`.bin`):** gob-encoded `ExecutionPlanV2`. Contains `CurrentSHA` for each target file. Apply verifies SHA before writing.

**JSON sidecar (`.json`):** Human-readable version. No credential values. Produced by `usync show <plan.bin>` or `usync show --json <plan.bin>`. Same schema as `ExecutionPlanV2` with `CredentialRefs[].Value` always `""`.

### 11.4 Plan Validity Rules

| Rule | Behavior on Violation |
|---|---|
| Plan older than 24h | Warn and require `--force-stale` |
| Target file SHA changed since plan | Error, refuse apply, show diff |
| Plan `SchemaVersion` mismatch | Error: "plan was created by a different version of usync. Re-run `usync plan`." |
| Plan file permissions not `0600` | Warn and re-chmod before reading |
| Plan file path outside home dir | Error: "plan file must be inside home directory" |

### 11.5 `usync show` JSON Schema

Stable schema for CI consumers:

```json
{
  "schema_version": 2,
  "plan_id": "a4f1c8e2-...",
  "created_at": "2026-05-21T14:30:00Z",
  "expires_at": "2026-05-22T14:30:00Z",
  "usync_version": "1.3.0",
  "provider_id": "exa",
  "credential_refs": [
    { "key": "EXA_API_KEY", "label": "exa_****abcd", "env_var": "EXA_API_KEY" }
  ],
  "operations": [
    {
      "target_id": "codex-cli",
      "target_name": "Codex CLI",
      "action": "update",
      "file_path": "/Users/nawodya/.codex/config.toml",
      "backup_path": "/Users/nawodya/.codex/config.toml.bak-usync-20260521-143201",
      "current_sha": "sha256:abc123...",
      "transport": "stdio",
      "manager": "file",
      "redacted": "Codex CLI: update exa [stdio, npx, key=exa_****abcd]",
      "is_symlink": false,
      "warnings": []
    }
  ],
  "warnings": ["VS Code: .vscode/mcp.json may be committed to git"],
  "doctor_summary": {
    "clients_detected": 5,
    "clients_ready": 4,
    "conflicts": 1,
    "warnings": ["Antigravity: conflict — two paths found"]
  }
}
```

---

## 12. Safety Model

### 12.1 Write Safety Chain

```
plan generation          →  read all targets, compute SHA-256 of each
plan save                →  write to $XDG_CACHE_HOME (0600), never to config dirs
apply start              →  re-verify SHA for every target; abort if any changed
backup                   →  <file>.bak-usync-<YYYYMMDD>-<HHMMSS> in same dir
write                    →  write to .tmp sibling, fsync, rename(2)
lock                     →  acquire <file>.lock (O_CREATE|O_EXCL) before backup+write
rollback                 →  if write N fails, restore backups 1..(N-1) from .bak files
lock release             →  defer unlock in all paths including panic recovery
verify                   →  re-read and parse each written file, check provider entry present
audit log                →  append entry to ~/.usync/audit.log (0600) after every apply
```

### 12.2 Concurrent Write Safety

`WriteWithLock` acquires `<config-file>.lock` via `os.OpenFile(path, os.O_CREATE|os.O_EXCL, 0600)`. If the lock file already exists (another usync instance running), wait up to 5 seconds with exponential backoff, then fail with a clear error: `"config file is locked by another usync process. If this is stale, delete <path>.lock"`.

### 12.3 Symlink Safety (Antigravity)

```go
func resolveForWrite(path string) (string, bool, error) {
    info, err := os.Lstat(path)          // lstat, not stat
    if err != nil { return path, false, nil } // file doesn't exist yet
    if info.Mode()&os.ModeSymlink != 0 {
        resolved, err := filepath.EvalSymlinks(path)
        if err != nil { return "", true, fmt.Errorf("symlink %s: %w", path, err) }
        if !withinHome(resolved) {
            return "", true, fmt.Errorf("symlink target %s is outside home dir", resolved)
        }
        return resolved, true, nil
    }
    return path, false, nil
}
```

Never `os.Remove` a symlink. Write to the resolved target only. Require confirmation in TUI. Show symlink status in plan preview.

### 12.4 Approval Gates

The following require explicit confirmation (in TUI: separate prompt; in CLI: `--auto-approve` bypasses):

| Scenario | Gate |
|---|---|
| First-ever write to a target | "About to create: `<path>`. Proceed?" |
| Overwriting a non-usync-managed key | "This file has entries not managed by usync. They will be preserved." |
| Writing to a project-scoped file that may be in git | "Warning: this file may be committed to git. Credentials will be visible." |
| Writing through a symlink | "This path is a symlink → `<resolved>`. Write to resolved target?" |
| Applying a plan older than 24h | "Plan was created `<N>h` ago. File state may have changed. Continue?" |

### 12.5 Redaction Rules

| Context | Rule |
|---|---|
| Plan preview output | Show `exa_****abcd` — last 4 chars only |
| URL with key in query | `https://mcp.exa.ai/mcp?exaApiKey=****abcd` |
| Auth headers | `Authorization: Bearer ctx7sk-****` |
| Env var values | `EXA_API_KEY=****` |
| TOML env values | `[env] EXA_API_KEY = "****"` |
| Log files | No credential values, ever |
| Plan JSON sidecar | No credential values, ever |
| Backup files | Full content (backup preserves the original) |
| `--show-secrets` flag | Shows last 8 chars; never the full value |

`RedactPlanJSON()` in `pkg/redact` must cover: `url` fields with query params, `headers` objects, `env` maps, `args` arrays with key-like strings.

### 12.6 File Permissions

| Path | Permissions | Notes |
|---|---|---|
| Config files written | `0600` | Private read/write for owner only |
| Config directories (if created) | `0700` | Private for owner |
| Plan binary files | `0600` | Even though no secrets: defense in depth |
| Plan JSON sidecar | `0600` | — |
| Backup files | `0600` | Same as original |
| Audit log | `0600` | — |
| Lock files | `0600` | Deleted after write |

### 12.7 Audit Log Format

Append-only JSONL at `~/.usync/audit.log`. Each line is one `AuditEntry`. Rotated when file exceeds 5MB (rename to `audit.log.1`, start fresh). No credential values logged.

```jsonl
{"ts":"2026-05-21T14:30:00Z","cmd":"plan","plan_id":"a4f1...","targets":["codex-cli","vscode","windsurf"],"exit_code":0}
{"ts":"2026-05-21T14:32:10Z","cmd":"apply","plan_id":"a4f1...","files":["/Users/nawodya/.codex/config.toml",...],"exit_code":0}
```

---

## 13. Credential Validation Strategy

### 13.1 Three-Phase Validation

```
Phase 1 (always): Format check
  - required field present?
  - known prefix? (ctx7sk-, tvly-, ghp_, github_pat_)
  - minimum length?
  - obviously malformed? (contains whitespace, too short, all zeros)

Phase 2 (automatic for multi-key providers): Duplication check
  - duplicate keys in a keys file?
  - same key for two different profiles?

Phase 3 (opt-in only): Live endpoint check
  - Tavily: GET /usage (Bearer auth, quota-safe)
  - GitHub: GET https://api.github.com/user (PAT auth, quota-safe)
  - All others: not available — show offline status only
```

### 13.2 Live Validation Cache

Cache live validation results at `~/.usync/cache/credentials.json` (`0600`) for 24 hours. Cache key is `SHA-256(provider_id + key_prefix_4_chars + key_last_4_chars)` — never the full key.

```go
type CredentialCache struct {
    Entries map[string]CacheEntry `json:"entries"`
}

type CacheEntry struct {
    Status     ValidationStatus `json:"status"`
    CachedAt   time.Time        `json:"cached_at"`
    ExpiresAt  time.Time        `json:"expires_at"`
    ProviderID string           `json:"provider_id"`
}
```

### 13.3 Provider-Specific Offline Regexes

```go
var OfflineRegexes = map[string]string{
    "exa":      `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
    "context7": `^ctx7sk-[A-Za-z0-9]{20,}$`,
    "tavily":   `^tvly-[A-Za-z0-9]{20,}$`,
    "github":   `^(ghp_[A-Za-z0-9]{36}|github_pat_[A-Za-z0-9_]{80,})$`,
}
```

### 13.4 UI Representation of Credential States

```
✓ EXA_API_KEY          provided_unverified    UUID format ✓
✓ TAVILY_API_KEY       verified_live          GET /usage returned 200 (cached 2h ago)
✓ GITHUB_PERSONAL...   verified_live          GET /user returned 200 (cached 1h ago)
  CONTEXT7_API_KEY     missing                get key: https://context7.com/dashboard
✓ (Playwright)         none_required          —
✗ (Terraform)          runtime_missing        Docker not running: https://docs.docker.com/desktop/
```

---

## 14. Batch Apply Strategy

### 14.1 Selection Model

**Default: target-first.** Each client is a row. The user selects which clients to update with a given provider.

**`--by-provider` flag:** Provider-first view. Lists all providers, lets user select providers, then shows aggregated targets.

**Hybrid TUI:** Dashboard groups by provider readiness. Provider selection → target selection → plan preview. This is provider-first for readiness reporting and target-first for apply ordering.

### 14.2 Apply Ordering

Within a single plan, operations execute in this order:

1. **Preflight all targets:** SHA verification, writability check, symlink detection
2. **Acquire all locks** (or fail fast if any lock is held)
3. **Write all file-backed targets** (backup → write → verify), rolling back on first failure
4. **Release all locks**
5. **Run CLI-managed targets** (separate phase, reported separately, not rolled back with files)
6. **Verify all targets**
7. **Write audit log entry**

File writes are per-target atomic. The batch is as-a-group transactional: if target 3 of 5 fails, targets 1–2 are rolled back to their backups.

### 14.3 CLI-Managed Target Separation

Claude Code (`claude mcp add-json`) and Codex CLI (`codex mcp add`) are CLI-managed. They run after all file writes succeed. They are:
- Reported separately in apply output ("Phase 2: CLI operations")
- Not rolled back if they fail (file writes are already verified)
- Labeled `[cli]` in plan preview
- Skipped with a warning if the CLI binary is not on PATH

### 14.4 Idempotency

Before writing a target, compute the intended config content and compare it to the existing content (after parsing and canonicalization). If identical: `action = skip`. Report it. Do not re-write.

`_usync` metadata block in JSON configs marks usync-managed entries:

```json
{
  "mcpServers": {
    "exa": {
      "type": "http",
      "url": "https://mcp.exa.ai/mcp?exaApiKey=...",
      "_usync": { "managedBy": "usync", "at": "2026-05-21T14:32:10Z", "planID": "a4f1..." }
    }
  }
}
```

Subsequent applies check `_usync.managedBy == "usync"` before overwriting. Entries without `_usync` marker are treated as user-managed and require confirmation before overwrite.

---

## 15. Migration Handling

### 15.1 Antigravity / Gemini CLI Migration (URGENT — before June 10, 2026)

**Timeline:**
- May 18–19, 2026: Antigravity 2.0 rebrand at Google I/O
- May 21, 2026: Antigravity CLI (Go binary) released; `~/.gemini/antigravity/mcp_config.json` is canonical
- June 18, 2026: Gemini CLI sunsets for AI Pro / Ultra / free-tier users

**Doctor behavior:**
- If `~/.gemini/settings.json` exists → show `[SUNSET WARNING]` until July 15, 2026
- If `~/.gemini/antigravity/mcp_config.json` exists and is symlink → report `is_symlink: true`, resolved path
- If BOTH `~/.gemini/settings.json` AND `~/.gemini/antigravity/mcp_config.json` exist → show migration card

**`usync migrate gemini-to-antigravity`:**

```
usync migrate gemini-to-antigravity --dry-run

Migration preview: Gemini CLI → Antigravity CLI

Source:  ~/.gemini/settings.json
         Contains: exa, playwright, context7

Target:  ~/.gemini/antigravity/mcp_config.json
         (symlink → ~/.gemini/antigravity-config/mcp_config.json)
         Currently: empty

Actions:
  copy  exa entry        → mcp_config.json
  copy  playwright entry → mcp_config.json
  copy  context7 entry   → mcp_config.json
  skip  settings.json    (Gemini CLI sunsets June 18; do not delete yet)

Backup: ~/.gemini/antigravity-config/mcp_config.json.bak-usync-20260521-143201

Run without --dry-run to apply.
```

**Symlink handling during migrate:**

```go
// pkg/config/migrate.go
func MigrateGeminiToAntigravity(home string, dryRun bool) error {
    src := filepath.Join(home, ".gemini", "settings.json")
    dst := filepath.Join(home, ".gemini", "antigravity", "mcp_config.json")
    
    // Detect symlink
    info, err := os.Lstat(dst)
    isSymlink := err == nil && info.Mode()&os.ModeSymlink != 0
    
    var writePath string
    if isSymlink {
        writePath, err = filepath.EvalSymlinks(dst)
        if err != nil { return fmt.Errorf("cannot resolve symlink %s: %w", dst, err) }
        if !withinHome(writePath, home) {
            return fmt.Errorf("symlink target %s is outside home dir", writePath)
        }
    } else {
        writePath = dst
    }
    
    // Read source servers
    srcConfig, err := readGeminiSettings(src)
    // ...merge into dst without removing src...
    // Never os.Remove(dst) — it's a symlink
}
```

### 15.2 Other Legacy Path Migrations

| Client | Migration | Trigger |
|---|---|---|
| Windsurf | `~/.codeium/mcp_config.json` → `~/.codeium/windsurf/mcp_config.json` | Doctor finds both present |
| OpenCode | `~/.opencode.json` → `~/.config/opencode/opencode.json` | Doctor finds only old path |

These are reported as `MigrationHint` in doctor output. No automatic migration — user decides which path to write to in the plan step.

---

## 16. Implementation Phases

### Phase 0 — Output Hygiene + Write Lock (1 PR, 2 days)

**Why first:** Correctness blocker. Library functions emitting to stdout breaks doctor output clean detection in all subsequent phases. Concurrent write safety is a correctness issue that must predate any new write paths.

**Scope:**
- Remove all `fmt.Printf("DEBUG: ...")` from `pkg/config/json_update.go`
- Add `pkg/audit` package (basic JSONL writer, no reads yet)
- Add `WriteWithLock` to `pkg/config/files.go` using `O_CREATE|O_EXCL` on `.lock` sibling
- Update all callers of `WriteWithBackup` to use `WriteWithLock`
- Add golden test: `make dry-run KEYS_FILE=... 2>&1 | grep -i "debug"` returns nothing
- Add unit test: two goroutines trying to write same file simultaneously — second waits or fails cleanly

**Acceptance criteria:**
- `go test ./...` passes
- `make dry-run` stdout contains no "DEBUG" substring
- No `fmt.Printf` or `fmt.Fprintf(os.Stdout, ...)` in any `pkg/` file (enforced by `grep` in CI)
- `WriteWithLock` acquires lock, writes, releases, even on write error (defer unlock)

---

### Phase 1 — Client Manifests + Read-Only Doctor (2 PRs, 5–7 days)

**PR 1a: `pkg/manifest`**

**Scope:**
- New package `pkg/manifest/` with all types from §8.1
- All 12 client manifests with macOS + Linux path templates, legacy candidates, scope labels
- Antigravity manifest: `IsSymlink: true`, legacy candidate for `~/.gemini/config/mcp_config.json`
- Windsurf manifest: legacy candidate for `~/.codeium/mcp_config.json`
- Gemini CLI manifest: sunset warning flag, deadline `"June 18, 2026"`
- `manifest.ExpandPath(tmpl, home string) string` — expands `{{.Home}}` token
- `manifest.ForPlatform(manifests []ClientManifest, platform string) []ClientManifest`
- No imports from other internal packages

**Acceptance criteria:**
- All 12 AppIDs have manifests
- All deprecated candidates have `ReplacedBy` set
- macOS and Linux variants verified for every platform-divergent client
- Test: no two candidates in a manifest share the same expanded path on macOS
- Test: `manifest.ExpandPath` handles `{{.Home}}`, spaces in path (macOS Application Support), tilde expansion

---

**PR 1b: `pkg/doctor` + `usync doctor` CLI**

**Scope:**
- `pkg/doctor.Doctor.Scan(ctx context.Context) (DoctorReport, error)`
- Candidate scan: `lstat` for symlinks; parse JSON/JSONC/TOML to check root key type; report per-candidate `ParseOK`, `RootKeyOK`, `Writable`, `Providers`
- JSONC support: strip `// comments` before parsing (use a simple line stripper; no full JSONC parser dependency)
- Provider presence detection: check if known provider IDs appear in the MCP server block of each parsed config
- Confidence scoring per client (see §8.2 `Confidence` constants)
- Runtime checks: `node --version`, `npx --version`, `docker info`, `claude`, `codex`, `gemini`, `antigravity`
- Migration hint generation: Gemini CLI sunset, Antigravity symlink, Windsurf legacy path
- `usync doctor` command: human-readable table + status icons (✓ ⚠ - ✗)
- `usync doctor --json` command: deterministic JSON output, golden-tested against fixture home directory
- Exit code: 0 = no issues, 2 = warnings

**Fixture home directories** (`pkg/doctor/testdata/homes/`):
- `macos_healthy/` — all 12 clients present, no conflicts
- `macos_antigravity_conflict/` — both `~/.gemini/config/mcp_config.json` and `~/.gemini/antigravity/mcp_config.json`
- `macos_gemini_sunset/` — Gemini CLI present, no Antigravity
- `macos_malformed_codex/` — `config.toml` with invalid TOML
- `macos_partial/` — only Codex + VS Code
- `linux_healthy/` — Linux path variants

**Acceptance criteria:**
- Doctor scan completes in < 2 seconds on any of the fixture homes
- `usync doctor --json` output is byte-identical across two runs on the same fixture (no timestamps in output)
- Doctor with `macos_antigravity_conflict` fixture reports `confidence: "conflict"` for Antigravity
- Doctor never writes any file (verified by integration test mounting fixture dir as read-only)
- Doctor with malformed TOML reports `parse_ok: false` and does not panic
- Gemini CLI fixture shows `[SUNSET WARNING]` in output
- `pkg/doctor` does not import `pkg/app` or `pkg/tui` (enforced by a Go import check in CI)
- Coverage: `pkg/doctor` ≥ 70%

---

### Phase 2 — `usync plan` + Plan File (2–3 PRs, 7–10 days)

**Scope:**
- Add `ExecutionPlanV2` type to `pkg/app/plan.go`
- `Manager.PlanFromDoctorSelections(report DoctorReport, selections []CandidateSelection, prov MCPProvider, profiles []CredentialProfile) (*ExecutionPlanV2, error)`
- `Manager.SavePlan(plan *ExecutionPlanV2, outPath string) error` — writes `.bin` (gob) + `.json` sidecar, both `0600`
- `Manager.LoadPlan(path string) (*ExecutionPlanV2, error)` — validates schema version, expiry, file-level SHAs
- `usync plan` CLI command with all flags from §9.1
- `usync show` CLI command: loads plan, prints human-readable or `--json` output
- `usync plan list` and `usync plan clean` subcommands
- Plan cache dir: `$XDG_CACHE_HOME/usync/plans/` or `$USYNC_PLAN_DIR`
- `-detailed-exitcode` semantics (0/1/2)
- Update `mapAllSelected` behavior: if `--all-detected` flag, select only `high`+`medium` confidence candidates from last doctor scan result; otherwise require explicit `--targets`

**`CandidateSelection` type:**
```go
type CandidateSelection struct {
    AppID         config.AppID
    CandidatePath string  // expanded path chosen by user from doctor report
    IsSymlink     bool
    ResolvedPath  string
}
```

**Acceptance criteria:**
- `usync plan --provider exa --targets codex-cli,vscode --keys-file ./keys.txt` writes valid `.bin` + `.json` to plan cache dir
- Plan `.json` sidecar contains no credential values
- `usync show <plan.bin> --json` output matches the stable schema from §11.5
- `usync plan` with no `--targets` and no `--all-detected` returns exit code 1 with helpful error
- `--all-detected` skips confidence `conflict` and `low` clients and lists them in plan warnings
- All existing `usync sync --dry-run` and `usync sync --apply` golden tests pass unchanged
- SHA verification test: modify a target file between `plan` and `apply`; verify apply exits 1 with checksum mismatch error

---

### Phase 3 — `usync apply` Batch (2 PRs, 5–7 days)

**Scope:**
- `Manager.Apply(plan *ExecutionPlanV2) (ApplyResult, error)` — new overload accepting `ExecutionPlanV2`
- Phase 1 (file ops): preflight all → acquire all locks → backup+write each → release all locks → rollback on failure
- Phase 2 (CLI ops): run CLI-managed targets after Phase 1 success
- Per-target `_usync` metadata block injection (§14.4)
- `--auto-approve` flag for CI (bypass interactive confirmation)
- Interactive confirmation in TUI and CLI (§12.4 approval gates)
- Audit log entry after each apply
- `usync apply --plan <path> --dry-run` re-reads current state, re-verifies SHAs, prints preview — no writes
- `pkg/audit` write after apply (Phase 0 created the package; this phase adds the apply call)

**CLI-managed adapter:**
```go
// pkg/app/adapter_cli.go
type CLIAdapter interface {
    Name()  string
    Apply(op PlanOperation) error
    Rollback(op PlanOperation) error  // best-effort for CLI ops
}

type ClaudeCodeAdapter struct{}
func (a *ClaudeCodeAdapter) Apply(op PlanOperation) error {
    // runs: claude mcp add-json <server-name> <json-blob>
}

type CodexCLIAdapter struct{}
func (a *CodexCLIAdapter) Apply(op PlanOperation) error {
    // runs: codex mcp add <args>
}
```

**Acceptance criteria:**
- All existing `qa_scenarios_test.go` golden tests pass
- New test: apply 3-target plan, inject write failure on target 2; targets 1's backup is restored; target 3 was never written
- New test: plan with Antigravity symlink target; apply writes to resolved path only; symlink is not removed or replaced
- New test: `--auto-approve` skips all interactive gates and completes without prompts
- Audit log test: verify JSONL entry written after apply with correct fields and no credential values
- `_usync` metadata test: second apply of same plan sees `action: skip` for already-correct entries
- CLI adapter test: `claude mcp add-json` is called with redacted logging; raw key never appears in command log

---

### Phase 4 — Credential Validation (1 PR, 3 days)

**Scope:**
- `OfflineValidator` and `LiveValidator` interface types in `pkg/provider/types.go`
- Implement `ValidateOffline` for: Exa, Context7, Tavily, GitHub (offline regex checks from §13.3)
- Implement `ValidateLive` for: Tavily (`GET /usage`, timeout 5s), GitHub (`GET /user`, timeout 5s)
- `ValidateLive` with 5s `context.WithTimeout`; on timeout → `ValidationSkipped` with message "validation timed out"
- Cache live results 24h at `~/.usync/cache/credentials.json` (`0600`)
- `usync validate` CLI command with `--live` opt-in flag
- TUI: offline validation runs automatically after credential entry; "Validate online" button for live validation
- `QuotaCost: true` entries show warning before live call: "This may count against your monthly quota. Continue?"

**Acceptance criteria:**
- `ValidateOffline` for all 4 providers is unit-tested with known-good and known-bad inputs
- `ValidateLive` for Tavily uses a mock HTTP client in tests; no real network calls in CI
- `ValidateLive` for GitHub uses a mock; no real network calls in CI
- Live validation with expired cache re-fetches; within 24h cache returns cached result
- Timeout test: server that delays > 5s → `ValidationSkipped` result, no panic
- `--live` flag without network connection → `ValidationSkipped` with network error message, not exit code 1

---

### Phase 5 — TUI Doctor Dashboard + Migration UX (2 PRs, 5–7 days)

**Scope (PR 5a: Dashboard)**
- Replace TUI entry point with `DashboardModel` that runs `Doctor.Scan()` concurrently on startup
- Spinner renders immediately; results fill in as they arrive
- Dashboard renders the screen from §10.2
- `ConflictResolutionModel` for clients with `conflict` confidence (§10.3)
- Old wizard entry preserved as `usync --wizard` flag
- Doctor scan timeout: if > 3 seconds, show partial results with `[scanning...]` placeholders

**Scope (PR 5b: Migration UX)**
- `usync migrate gemini-to-antigravity` command with `--dry-run` / `--apply`
- Migration preview card in doctor output for Gemini CLI sunset
- Sunset warning banner in dashboard until July 15, 2026 (controlled by a constant date in `pkg/manifest`)
- TUI migration card with direct action button

**Acceptance criteria:**
- TUI starts within 500ms of launch (first render with spinner) — measured in benchmark test
- `usync` with zero AI clients installed shows "nothing detected" state with install links, not a panic or empty screen
- Antigravity conflict shown in dashboard before any provider selection
- `usync migrate gemini-to-antigravity --dry-run` on Gemini CLI fixture shows correct preview
- `usync migrate gemini-to-antigravity --apply` writes to resolved symlink target, does not remove symlink
- Sunset warning shown when `~/.gemini/settings.json` exists and date is before July 15, 2026
- All existing TUI golden tests pass

---

### Phase 6 — Optional MCP Server Mode (deferred, no timeline)

**Do not start until Phases 1–5 are shipped and the local API surface has been stable for 30+ days.**

**Scope (when ready):**
- `cmd/usync-mcp/main.go` with `usync mcp serve` entrypoint
- Read-only tools only: `usync_doctor`, `usync_list_clients`, `usync_plan_preview`, `usync_validate_credentials`
- Thin adapter: calls same `pkg/doctor.Doctor.Scan()` and `pkg/app.Manager.PlanFromDoctorSelections()` as CLI
- No `apply` tool exposed from MCP server — agents can advise but not mutate
- MCP server stdio transport only (no HTTP server in this phase)

---

## 17. First 3 PRs — Full Acceptance Criteria

### PR 1 — Output Hygiene + `WriteWithLock`

**Branch:** `fix/output-hygiene-and-write-lock`  
**Files changed:** `pkg/config/json_update.go`, `pkg/config/files.go`, `pkg/audit/audit.go` (new), tests  
**Estimated time:** 2 days

**Changes:**
1. Remove every `fmt.Printf("DEBUG: ...")` line from `pkg/config/json_update.go`
2. Add `WriteWithLock(path string, content []byte, perm fs.FileMode) error` to `pkg/config/files.go`:
   - Acquire `<path>.lock` via `os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL, 0600)`
   - On lock exists: retry 5× with 100ms backoff; fail with `ErrFileLocked`
   - `defer os.Remove(lockPath)` — released even on panic
   - Existing `WriteWithBackup` calls updated to use `WriteWithLock`
3. Create `pkg/audit/audit.go` with `Append(entry AuditEntry) error`:
   - Opens `~/.usync/audit.log` with `O_APPEND|O_CREATE`, `0600`
   - Writes a single JSONL line
   - Rotates at 5MB (rename to `.1`, create fresh)

**Acceptance criteria:**
- `go test ./...` passes with no regressions
- `make dry-run KEYS_FILE=./test-keys.txt 2>&1 | grep -ci "debug"` returns `0`
- `grep -rn 'fmt\.Printf' pkg/` returns only test files (none in production code)
- `TestWriteLock_Concurrent`: two goroutines write to the same path; second waits (up to 5s) or returns `ErrFileLocked`; neither produces a corrupted file
- `TestWriteLock_ReleasedOnError`: write fails after lock acquired; `.lock` file is removed
- `TestAuditAppend`: appends 3 entries; file is valid JSONL; no credential values in output
- `TestAuditRotation`: write entries until file > 5MB; verify rotation to `.1`; verify new file starts empty

---

### PR 2 — `pkg/manifest` (Client + Provider Metadata)

**Branch:** `feat/client-manifests`  
**Files changed:** `pkg/manifest/` (new package, ~500 lines), tests  
**Estimated time:** 3 days

**Changes:**
1. Create `pkg/manifest/types.go` with `ClientManifest`, `ConfigCandidate`, `RuntimeRequirement`, `CredentialAcquisition`, `LiveValidationSpec`, `SourceRef`
2. Create `pkg/manifest/manifests.go` with all 12 `ClientManifest` declarations
3. Antigravity manifest must have:
   - `IsSymlink: true` on current candidate
   - Legacy candidate: `~/.gemini/config/mcp_config.json` (deprecated, `ReplacedBy: "antigravity-current"`)
   - Sunset/warning candidate: `~/.gemini/settings.json` (Gemini CLI, `Deprecated: true`, `Deadline: "2026-06-18"`)
4. Windsurf manifest: legacy `~/.codeium/mcp_config.json` candidate with `Deprecated: true`
5. Codex manifest: TOML kind, `mcp_servers` root key (underscore), user + project scope
6. VS Code manifest: `servers` root key (not `mcpServers`); workspace candidate with `GitWarning: true`
7. Zed manifest: `context_servers` root key; `source: custom` requirement noted in `Notes` field
8. OpenCode manifest: JSONC kind; `mcp` root key; `SecurityNote: "stdio commands execute on load"`
9. Kiro manifest: deep-merge precedence model documented
10. Create `pkg/manifest/providers.go` with `ProviderMeta` for all 7 providers (including credential acquisition metadata)
11. Create `pkg/manifest/runtimes.go` with `RuntimeRequirements` table
12. Create `pkg/manifest/manifests_test.go`

**Acceptance criteria:**
- All 12 `config.AppID` values have a manifest
- No two candidates within the same manifest share the same expanded path on macOS
- No two candidates within the same manifest share the same expanded path on Linux
- All deprecated candidates have `ReplacedBy` set (non-empty string)
- Antigravity manifest has exactly 2 candidates (legacy + current); current has `IsSymlink: true`
- Windsurf manifest has exactly 2 candidates; legacy has `Deprecated: true`
- All provider metas have non-empty `GetURL` (except Playwright and Kubernetes — no key required)
- Tavily and GitHub have `LiveValidation` spec set; all others have `LiveValidation: nil`
- `pkg/manifest` has zero imports from any other internal package (`go vet ./pkg/manifest/...` + grep for internal imports)
- Test: `ExpandPath("{{.Home}}/.codex/config.toml", "/Users/nawodya")` returns `/Users/nawodya/.codex/config.toml`
- Test: `ExpandPath` handles spaces (macOS `Library/Application Support`)

---

### PR 3 — `pkg/doctor` + `usync doctor` CLI

**Branch:** `feat/doctor-scan`  
**Files changed:** `pkg/doctor/` (new package, ~600 lines), `cmd/usync/main.go` (new `doctor` subcommand), `pkg/doctor/testdata/homes/` (fixture dirs), tests  
**Estimated time:** 4 days

**Changes:**
1. `pkg/doctor/doctor.go`: `Doctor` struct, `New(home string, manifests []manifest.ClientManifest, providers []provider.MCPProvider) *Doctor`, `Scan(ctx context.Context) (DoctorReport, error)`
2. `pkg/doctor/client_scan.go`: per-candidate file stat, lstat, parse, root key check, provider presence detection
3. JSONC support: `stripJSONComments(input []byte) []byte` — strips `// line comments` only (sufficient for current OpenCode format)
4. `pkg/doctor/runtime_check.go`: `CheckRuntime(req manifest.RuntimeRequirement) RuntimeFinding` via `exec.LookPath` + `exec.CommandContext` with 3s timeout
5. `pkg/doctor/provider_readiness.go`: `ComputeReadiness(report DoctorReport, providers []provider.MCPProvider, creds map[string]string) []ProviderReadiness`
6. `pkg/doctor/report.go`: `FormatHuman(report DoctorReport) string` and `FormatJSON(report DoctorReport) ([]byte, error)`
7. `cmd/usync/main.go`: add `doctor` and `doctor --json` subcommands
8. Fixture home directories in `pkg/doctor/testdata/homes/`:
   - `macos_healthy/`: all 12 clients, valid configs, no conflicts
   - `macos_antigravity_conflict/`: both Antigravity paths present
   - `macos_gemini_sunset/`: only Gemini CLI
   - `macos_malformed_codex/`: invalid TOML in codex config
   - `macos_partial/`: only Codex + VS Code
   - `linux_healthy/`: Linux paths
9. Golden JSON test: `TestDoctorJSON_MacOSHealthy`, etc. — compare against committed golden files

**Acceptance criteria:**
- `usync doctor` exits 0 on `macos_healthy` fixture, prints client table
- `usync doctor` exits 2 on any fixture with warnings
- `usync doctor --json` output for each fixture is byte-identical to committed golden file
- Antigravity conflict fixture: `confidence` is `"conflict"`, `has_migration` is non-null
- Malformed TOML fixture: Codex CLI reports `parse_ok: false`, does not panic, other clients not affected
- Gemini CLI sunset fixture: global `warnings` array contains the sunset message
- Doctor scan on `macos_healthy` completes in < 2 seconds (benchmark test, `go test -bench=BenchmarkScan`)
- Doctor writes nothing: `macos_healthy` fixture dir has identical `mtime` before and after scan (integration test)
- `pkg/doctor` import check: no import of `pkg/app` or `pkg/tui` (grep + `go mod graph` check in CI)
- Coverage: `pkg/doctor` ≥ 70% statement coverage (`make coverage-check` passes)

---

## 18. MCP Mode Verdict

**Decision: Defer to Phase 6. Ship read-only tools only when local APIs are stable.**

**Reasoning:**
1. The local API surface (`pkg/doctor`, `pkg/app.PlanFromDoctorSelections`, `pkg/app.LoadPlan/Apply`) must stabilize before freezing it behind an MCP interface. An unstable MCP interface is worse than none.
2. Mutating operations from an agent context widen the blast radius for prompt injection. Claude Code can call `usync apply` to self-modify MCP configs — that's a privilege escalation path that needs careful design.
3. Existing tooling (`claude mcp serve`, `codex mcp`, native `usync` CLI) already covers the agentic use case adequately for the target audience.

**When Phase 6 does ship:**
- `cmd/usync-mcp/` as a separate binary
- Read-only tools: `usync_doctor`, `usync_list_clients`, `usync_list_providers`, `usync_plan_preview`, `usync_validate_credentials`
- No `usync_apply` tool — agents may suggest but not execute writes
- Thin adapter over the same `pkg/doctor` and `pkg/app` functions used by CLI
- No new config engine, no second code path

---

## 19. Risks and Open Questions

### Active Risks

| Risk | Probability | Impact | Mitigation |
|---|---|---|---|
| Antigravity symlink changes again before June 10 | Medium | High | Use `lstat` at runtime, not cached manifest; doctor re-reads every time |
| Gemini CLI sunset delayed past June 18 | Low | Low | Sunset date is a constant in `pkg/manifest`; easy to update |
| RFC #2219 merges a canonical schema in 2026 | Medium | High (positive) | Adapter architecture absorbs it; one new adapter replaces N client-specific writers |
| OpenCode project-config command injection | Low | High | Warn loudly when reading project-scope OpenCode config; never auto-merge from untrusted dirs |
| VS Code user profile path is non-deterministic on Linux | Low | Medium | Detect via `XDG_CONFIG_HOME` env; add as candidate with `Confidence: medium` |
| Codex TOML syntax error breaks both Codex CLI and VS Code Codex extension | High (user error) | Medium | Doctor validates TOML before apply; apply aborts on parse error |

### Open Questions (Decisions Needed Before Phase 2)

| Question | Options | Recommendation |
|---|---|---|
| Project-scope files in `--all-detected`? | Include / Exclude / Opt-in | **Opt-in via `--include-workspace`** |
| `USYNC_HOME` test override? | Yes / No | **Yes** — required for hermetic CI |
| Plan cache cleanup TTL? | 7 days / 30 days / never | **7 days** via `usync plan clean` |
| `_usync` metadata in Zed `context_servers`? | Yes / No | **Yes** — but nested under the server entry, not at root |
| Codex TOML managed marker? | `[mcp_servers.exa._usync]` table? | **Skip** — TOML table names are strict; use a comment `# managed-by=usync` |
| Live validation cache key? | SHA-256 of full key / prefix+suffix | **Prefix 4 + suffix 4 + provider ID** — never full key |

### Weak Assumptions to Validate

- **Antigravity `mcp_config.json` is always a symlink.** Verify on a fresh Antigravity install before Phase 5 ships. If it's not always a symlink, `lstat` still works correctly (just reports `is_symlink: false`).
- **Context7 `ctx7sk-` prefix is stable.** Issue #1309 in the Context7 repo confirms this error message as of 2026, but it's not formally documented. Add a comment and re-verify when updating the manifest.
- **Codex `codex mcp add` is the canonical CLI manager.** The OpenAI Codex docs describe it but behavior in edge cases (existing entries, conflicting names) is not fully documented. Default to file-backed for Codex until CLI adapter is tested.

---

## 20. Deferred Work

The following is explicitly out of scope for Phases 1–5 and should not be added without a new decision:

| Feature | Why Deferred |
|---|---|
| Terraform state file | Client config files are the source of truth; state is over-engineering |
| MCP gateway/proxy | Runtime data-plane product; different problem scope |
| Custom secret store | Adds attack surface; `${VAR}` env refs are sufficient |
| Team/remote plan state | Local tool; remote state adds locking and auth complexity |
| MCP Registry consumption | Publisher-side only; no client has adopted consumer-side yet |
| Windows support | Current README scopes to macOS + Linux; adds ~20% test surface |
| Per-server policy DSL | Not requested; add only if enterprise use cases emerge |
| `hashicorp/go-plugin` polyglot plugins | Premature; static manifests cover 95% of cases |
| Dynamic registry discovery | Stable static manifests + manual updates are safer and faster to ship |
| Write-capable MCP server tools | Security risk until agent trust model is clearer |

---

*End of Technical Specification v1.0*  
*Next review: after Phase 1 PRs are merged or June 10, 2026, whichever is sooner.*
