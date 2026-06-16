# Doctor Mode Phase 8: Dashboard Provider Readiness And Saved-Plan Flow

**Status:** Draft — awaiting architecture review  
**Last updated:** 2026-05-23  
**Builds on:** Phase 7 (`docs/specs/doctor-mode-phase7/spec.md`) — DashboardModel, DashboardScanner interface  
**Source:** `docs/specs/doctor-mode-remaining-implementation-plan.md` § Phase 8  
**Research audit:** BubbleTea v1.3.10 (pkg.go.dev), Claude Code MCP docs (code.claude.com), VS Code MCP config reference (code.visualstudio.com), May 2026

---

## Problem Statement

Phase 7 delivered a read-only doctor dashboard. Users can see what is installed and what is conflicted, but they cannot act on it. The default path still requires dropping to the CLI (`usync plan … && usync apply --plan …`) to configure any AI client. Phase 8 closes that gap: the dashboard becomes a full `doctor → validate → plan → apply` path in the TUI, using the existing saved-plan APIs, without adding any new CLI flags.

Phase 8 must not implement credential entry, credential storage, or any new file-mutation logic. Every write must go through `app.BuildSavedPlan` and `app.ApplySavedPlan`.

---

## Goals

- Show provider readiness state derived from: doctor report, manifest provider metadata, runtime checks, and supplied credentials.
- Group providers as `no-key-needed`, `ready`, `missing-credentials`, `runtime-missing`, `conflict-blocked`.
- Surface get-key URLs from `manifest.ProviderMeta.Credentials[n].GetURL` for missing-credentials providers.
- Integrate offline credential validation before allowing plan creation.
- Add an explicit, opt-in live validation action with rate-limit awareness; no automatic network calls.
- Add dashboard target/provider selection using doctor candidates (not `pkg/config` static detection).
- Save a plan via `app.BuildSavedPlan` before rendering a preview; never use legacy in-memory apply.
- Render plan preview: client list, warnings, skipped clients, approval prompts, and redacted credential refs.
- Apply through `app.ApplySavedPlan`; preserve create/symlink/stale/workspace gates.
- Use target-native secret indirection where the target format supports it:
  - VS Code: emit `${input:varName}` in `env` rather than a bare credential value.
  - Claude Code `.mcp.json`: emit `${VAR:-default}` expansion rather than a bare credential value.

---

## Non-Goals

- Credential entry UI (typing API keys inside the TUI).
- Credential storage, keychain, or secret rotation.
- Provider credential validation via any automatic or background network call.
- Parsing or writing client config files directly in `pkg/tui`.
- Removing or rewriting the legacy wizard.
- Migration UX (Phase 10).
- Conflict resolution beyond what Phase 7 already renders.
- MCP Server Mode.
- Windows support.

---

## Users

- Users who saw the Phase 7 dashboard and want to proceed to configure detected clients.
- Users supplying API keys via `--keys` or `--keys-file` and wanting a TUI apply flow.
- Users with runtime blockers who need to understand what is missing before choosing a provider.
- Existing wizard users: the wizard flow is preserved unchanged behind `--wizard`.

---

## Functional Requirements

### Provider Readiness (FR-1 – FR-6)

**FR-1:** The dashboard derives provider readiness from four sources, all available without network I/O:
- `manifest.AllProviders()` — static metadata: required credentials, runtime IDs, get-key URLs.
- `doctor.Report.Runtimes` — installed/missing runtime findings.
- `doctor.Report.Clients` — confidence and conflict findings.
- Supplied credential profiles (passed in at TUI start; same source as `--keys`/`--keys-file`).

**FR-2:** Providers are classified into exactly five states:

| State | Condition |
|---|---|
| `no-key-needed` | `ProviderMeta.Credentials` is empty (e.g. Playwright, Kubernetes, Terraform without token) |
| `ready` | credentials supplied and pass offline validation; no blocking runtime missing |
| `missing-credentials` | `Credentials` non-empty and none supplied |
| `runtime-missing` | required runtime (`ProviderMeta.RuntimeIDs`) not found in `doctor.Report.Runtimes` |
| `conflict-blocked` | any high-confidence client has `Confidence == ConfidenceConflict` |

A provider can be in multiple states (e.g. `runtime-missing` AND `missing-credentials`); render the highest-severity one.

**FR-3:** For `missing-credentials` providers, display the `GetURL` from `manifest.CredentialAcquisition.GetURL` so the user knows where to obtain a key.

**FR-4:** Provider readiness is computed in a `tea.Cmd` that returns a typed `providerReadinessMsg`; it must not run in `Init` or `View`.

**FR-5:** The readiness view must not render raw credential values. Use `redact.Key()` for any partial display; prefer label-only references.

**FR-6:** Provider selection (which provider to use for the plan) is a TUI state field; the default is the first `ready` or `no-key-needed` provider, with keyboard navigation to change it.

### Validation (FR-7 – FR-10)

**FR-7:** Offline validation runs via `validate.Service.ValidateProfiles(ctx, prov, profiles, false)` inside a `tea.Cmd` before plan creation is allowed. A failed offline validation blocks plan creation and displays the redacted error.

**FR-8:** Live validation is opt-in, triggered only by an explicit key action (e.g. `v`). It calls `validate.Service.ValidateProfiles(ctx, prov, profiles, true)`. The TUI must display a spinner while live validation is in progress.

**FR-9:** The existing 24-hour validation cache (`validationCacheTTL = 24 * time.Hour` in `pkg/validate/cache.go`) is respected automatically by `validate.Service`. The TUI must not duplicate this logic. A repeated live validation press within the cache window must not trigger a new network request (the service handles this).

**FR-10:** Tavily `/usage` endpoint has a community-observed rate limit of 10 requests per 10 minutes. The TUI must not expose a repeated-trigger mechanism for Tavily live validation; once live validation is in progress, the key action is disabled until a result arrives.

### Target Selection (FR-11 – FR-13)

**FR-11:** Target clients are derived from `doctor.Report.Clients` filtered to `Confidence == ConfidenceHigh || ConfidenceMedium` and `Installed == true || EffectivePath != ""`. Conflict and low-confidence clients are excluded from planning with a visible warning.

**FR-12:** Workspace/project-scoped targets (`.mcp.json`, `.agents/mcp_config.json`, etc.) are excluded unless the user explicitly enables them via a TUI toggle (maps to `--include-workspace` semantics from Phase 6).

**FR-13:** Target selection state is stored in the dashboard model as a `map[manifest.ClientID]bool`; keyboard navigation allows toggling clients in/out.

### Plan Creation (FR-14 – FR-16)

**FR-14:** Plan creation runs inside a `tea.Cmd` that calls:
1. `app.Manager.PrepareProvider(prov, profiles, selected, assignments)` → `ExecutionPlan`
2. `app.Manager.BuildSavedPlan(executionPlan, SavedPlanOptions{…})` → `SavedPlan`
3. `app.PlanStore.Save(savedPlan, "")` → disk path

A typed `planCreatedMsg{plan SavedPlan, path string, err error}` is returned to `Update`.

**FR-15:** The plan preview screen renders:
- Target client list with path and scope label.
- Redacted credential refs (label + last-4 of key, never full value).
- Warnings from `savedPlan.Warnings`.
- Skipped clients with reason.
- Approval prompts from `app.PreflightSavedPlan` (create-new-file, symlink, workspace/project gates).

**FR-16:** The plan preview is read-only until the user explicitly confirms apply (e.g. `y` or `Enter`). A confirmation prompt is rendered inline; no nested Bubble Tea program is launched.

### Secret Indirection (FR-17 – FR-18)

**FR-17 — VS Code:** When building a plan operation targeting `AppVSCode` (or any client whose candidate has `MutationKind == MutationNamedServer`) and the provider config type is HTTP, `BuildSavedPlan` must emit the VS Code `inputs` block structure and use `${input:PROVIDER_KEY_ID}` as the env value rather than the raw key. This requires a new `SavedPlanOptions.UseInputVariables bool` flag and corresponding logic in `BuildSavedPlan` / `buildPlanOperation`.

VS Code `inputs` format (confirmed from official docs, May 2026):
```json
{
  "inputs": [{ "type": "promptString", "id": "exa-api-key", "description": "Exa API Key", "password": true }],
  "servers": {
    "exa": { "type": "http", "url": "...", "env": { "EXA_API_KEY": "${input:exa-api-key}" } }
  }
}
```

**FR-18 — Claude Code `.mcp.json`:** When building a plan operation for `AppClaudeCode` targeting `ScopeProject` (`.mcp.json`), emit `${EXA_API_KEY:-}` (or `${VARNAME:-default}`) in the `env` field rather than the raw value, using `${VAR:-default}` syntax confirmed in Claude Code docs (May 2026). This applies only to project-scope operations; user-scope `~/.claude.json` entries use a CLI mutation and are not affected.

### Apply (FR-19 – FR-20)

**FR-19:** Apply runs inside a `tea.Cmd` that calls `app.Manager.ApplySavedPlan(plan, SavedPlanApplyOptions{Confirm: autoApproveNonGated})`. Approval prompts for workspace/project-scoped targets must surface in the TUI before apply begins; the user must explicitly acknowledge each.

**FR-20:** After apply, the dashboard re-runs the doctor scan (`scanCmd`) to refresh the readiness view. A typed `applyResultMsg{result ApplyResult, err error}` is returned from the apply `tea.Cmd`.

---

## UX Requirements

### Screen Flow

```
[Loading / Scanning]
       ↓
[Doctor Dashboard — Phase 7 base]
  ↓ press [p] or [Enter]
[Provider Readiness View]
  ↓ choose provider, press [Enter]
[Offline Validation → spinner]
  ↓ pass
[Target Selection]
  ↓ toggle clients, press [Enter]
[Plan Creation → spinner]
  ↓
[Plan Preview + Approval Prompts]
  ↓ press [y] / [Enter]
[Apply → spinner]
  ↓
[Apply Result / Rescan]
```

### Key Bindings (additive to Phase 7)

| Key | Context | Action |
|---|---|---|
| `p` | Dashboard loaded | Enter provider readiness view |
| `↑`/`↓` | Provider/client lists | Navigate |
| `Space` | Target selection | Toggle client |
| `v` | Provider readiness | Trigger live validation (opt-in) |
| `Enter` | Provider/target selection | Confirm and advance |
| `y` | Plan preview | Confirm apply |
| `n` | Plan preview | Cancel, return to target selection |
| `Esc` | Any sub-screen | Return to previous screen |
| `r` | Any screen | Rescan (Phase 7) |
| `?` | Any screen | Toggle help |
| `q`/`ctrl+c` | Any screen | Quit |

### Visual Hierarchy

1. **Provider readiness summary** — counts by state, with `ready` count most prominent.
2. **Missing runtimes** — blockers listed first with install URL.
3. **Provider list** — grouped by state; `ready` and `no-key-needed` first.
4. **Get-key URLs** — shown inline for `missing-credentials`, not as hyperlinks (terminal compat).
5. **Target client list** — checkboxes, with confidence and scope label.
6. **Plan preview** — dense text; matches existing `app.FormatSavedPlan` style.
7. **Apply result** — pass/fail per file; matches `app.FormatApplyResult` style.

### Redaction Rules (strict)

- Raw credential values must never appear in `View()` output or test assertions.
- Credential refs are rendered as `<label> (<key-id>)` where key-id is `redact.Key(value)`.
- VS Code `${input:…}` and Claude Code `${VAR:-default}` literals are safe to render (they are references, not values).
- Live validation error messages pass through `redact.Text()` before display.

---

## Data Model Requirements

### New TUI State Fields (extend `DashboardModel`)

```go
// Phase 8 additions to DashboardModel in pkg/tui/dashboard.go

type dashboardScreen int

const (
    screenDoctor        dashboardScreen = iota // Phase 7 base
    screenProviderReady                        // new
    screenTargetSelect                         // new
    screenPlanPreview                          // new
    screenApplyResult                          // new
)

// Fields to add to DashboardModel:
//   screen          dashboardScreen
//   providerMetas   []manifest.ProviderMeta
//   readiness       []ProviderReadinessItem  // computed
//   selectedProv    manifest.ProviderID
//   profiles        []provider.CredentialProfile
//   validReport     *validate.Report
//   validating      bool
//   selectedClients map[manifest.ClientID]bool
//   currentPlan     *app.SavedPlan
//   planPath        string
//   planPreflight   *app.SavedPlanPreflight
//   applyResult     *app.ApplyResult
//   applyErr        error
//   applying        bool
//   planning        bool
```

```go
// New type in pkg/tui:
type ProviderReadinessItem struct {
    Meta    manifest.ProviderMeta
    State   ProviderState
    Reasons []string // human-readable blockers
}

type ProviderState string

const (
    ProviderStateNoKeyNeeded       ProviderState = "no-key-needed"
    ProviderStateReady             ProviderState = "ready"
    ProviderStateMissingCredential ProviderState = "missing-credentials"
    ProviderStateRuntimeMissing    ProviderState = "runtime-missing"
    ProviderStateConflictBlocked   ProviderState = "conflict-blocked"
)
```

### New Typed Messages

```go
type providerReadinessMsg struct {
    items []ProviderReadinessItem
    err   error
}

type validationResultMsg struct {
    report validate.Report
    err    error
}

type planCreatedMsg struct {
    plan app.SavedPlan
    path string
    err  error
}

type applyResultMsg struct {
    result app.ApplyResult
    err    error
}
```

### DashboardManager Interface

Phase 8 requires plan and apply operations. Introduce a `DashboardManager` interface in `pkg/tui` so unit tests do not need a real `app.Manager`:

```go
type DashboardManager interface {
    PrepareProvider(prov provider.MCPProvider, profiles []provider.CredentialProfile,
        selected map[config.AppID]bool, assignments map[config.AppID]int) (app.ExecutionPlan, error)
    BuildSavedPlan(plan app.ExecutionPlan, opts app.SavedPlanOptions) (app.SavedPlan, error)
    PreflightSavedPlan(plan app.SavedPlan, opts app.SavedPlanApplyOptions) (app.SavedPlanPreflight, error)
    ApplySavedPlan(plan app.SavedPlan, opts app.SavedPlanApplyOptions) (app.ApplyResult, error)
    HomeDir() string
}
```

Production wiring: `cmd/usync/main.go` passes a `*app.Manager` adapter.  
Test wiring: `FakeDashboardManager` returning canned plans.

### pkg/app Changes

Two targeted additions only:

1. **`SavedPlanOptions.UseInputVariables bool`** — when true, `buildPlanOperation` emits VS Code `${input:…}` env values and adds an `inputs` block to the operation metadata instead of writing raw credential strings.
2. **`SavedPlanOptions.UseEnvExpansion bool`** — when true, project-scope Claude Code operations use `${VARNAME:-}` in the `env` block.

These fields are opt-in and default to `false`; all existing CLI paths and tests are unaffected.

---

## Technical Requirements

- **Bubble Tea:** v1.3.10 (pinned in `go.mod`). Use `tea.Batch` for concurrent scan + readiness computation. Use `tea.Sequence` only if ordering is required. `View()` must remain side-effect free and non-blocking.
- **No new TUI sub-programs.** Screen transitions are state machine changes within `DashboardModel.Update`; no nested `tea.Program.Run()` calls.
- **No `pkg/config` parsing in `pkg/tui`.** All filesystem data flows through `DashboardScanner` or `DashboardManager`.
- **No raw credentials in `View()`.** All credential values pass through `redact.Key()` before rendering.
- **Validation cache is pkg/validate's responsibility.** The TUI must not cache validation results independently.
- **Plan store usage.** Every plan must be saved to disk via `app.PlanStore.Save` before preview is shown.
- **`pkg/tui` import guard.** Must not import `pkg/config` json-update helpers or `pkg/app` directly. Interface types are in `pkg/tui`; concrete types are injected via interfaces from `cmd/usync`.

---

## File Layout

```
pkg/tui/
  dashboard.go          — extend DashboardModel; add screen state, manager interface, typed msgs
  dashboard_view.go     — extend View(); add providerReady, targetSelect, planPreview, applyResult renders
  dashboard_readiness.go — ProviderReadinessItem, ProviderState, readiness computation helper (pure function)
  dashboard_test.go     — extend unit tests for new states and key actions
  dashboard_teatest_test.go — extend teatest flow through plan creation with FakeDashboardManager

pkg/app/
  plan_v2.go            — add UseInputVariables, UseEnvExpansion to SavedPlanOptions; update buildPlanOperation
  plan_v2_test.go       — tests for input variable and env expansion emission

cmd/usync/
  main.go               — wire manager adapter and initial profiles to NewDashboardModel
```

---

## Testing Requirements

### Unit Tests (`pkg/tui`)

- Readiness computation: pure function tested with fake doctor reports covering all five provider states.
- Key `p` advances to `screenProviderReady`.
- `Esc` returns from sub-screen.
- `v` in readiness screen triggers live validation cmd; second `v` while validating is a no-op.
- `providerReadinessMsg` with error renders error state without panic.
- `planCreatedMsg` with valid plan advances to `screenPlanPreview`.
- `planCreatedMsg` with error renders plan-error state.
- `applyResultMsg` success re-triggers scan cmd.
- No raw credential in `View()` for any state (redaction regression test).
- Workspace-scoped client excluded from default target selection.

### Unit Tests (`pkg/app`)

- `BuildSavedPlan` with `UseInputVariables: true` emits `${input:…}` in VS Code env and includes `inputs` metadata.
- `BuildSavedPlan` with `UseEnvExpansion: true` emits `${EXA_API_KEY:-}` in project-scope Claude Code operation.
- Existing `BuildSavedPlan` tests with `UseInputVariables: false` are unaffected.

### Teatest (`pkg/tui`)

- Full flow: scan → provider readiness → target selection → plan creation (fake manager) → plan preview → apply → rescan.
- `teatest.WaitFor` for each screen transition; no `time.Sleep`.
- Assert stable visible text fragments (not raw ANSI).
- No credential values in any captured output.

### Acceptance Commands

```bash
go test ./pkg/tui ./pkg/validate ./pkg/app
go test ./...
make lint
make build
```

---

## Acceptance Criteria

| # | Criterion |
|---|---|
| AC-1 | Malformed credentials block plan creation in TUI; error is redacted. |
| AC-2 | Live validation is only triggered by explicit user action; never automatic. |
| AC-3 | Repeated live validation press within 24-hour cache window does not issue a network request. |
| AC-4 | Dashboard preview creates a saved plan on disk; legacy in-memory apply is not used. |
| AC-5 | Dashboard apply calls `app.ApplySavedPlan`; result is shown per-file. |
| AC-6 | Raw credentials do not appear in rendered strings or test assertions. |
| AC-7 | VS Code operations with `UseInputVariables: true` contain `${input:…}` literals and an `inputs` block. |
| AC-8 | Project-scope Claude Code operations with `UseEnvExpansion: true` contain `${VARNAME:-}` in env. |
| AC-9 | Conflict and low-confidence clients are excluded from target selection with visible warning. |
| AC-10 | Workspace/project-scoped targets excluded unless user explicitly toggles them. |
| AC-11 | `go test ./pkg/tui ./pkg/validate ./pkg/app` passes. |
| AC-12 | All Phase 7 tests continue to pass unchanged. |

---

## Open Questions

| # | Question | Impact | Owner |
|---|---|---|---|
| OQ-1 | Should `UseInputVariables` default to `true` for VS Code in the TUI, or require user opt-in? Setting it silently changes written files; users who manage VS Code secrets differently may object. | Affects FR-17 and AC-7 | Needs product decision |
| OQ-2 | Tavily rate limit of 10 req/10 min is from the original research doc, not confirmed in official Tavily docs during this audit. If the limit is higher or lower, the UI debounce in FR-10 may need adjustment. | Affects FR-10 UX | Verify against Tavily API changelog |
| OQ-3 | `DashboardManager` interface introduces a `pkg/tui` → `pkg/app` type dependency via interface parameters (`app.ExecutionPlan`, `app.SavedPlan`). Consider whether a leaner adapter in `cmd/usync` passes only serializable types to `pkg/tui`. | Affects package isolation | Architecture review |
| OQ-4 | The `app.Manager.HomeDir` field is used for `PlanStore` path. Should `DashboardManager.HomeDir()` be replaced with injecting a `PlanStore` directly? | Affects testability | Architecture review |

---

## References

- Phase 7 spec: `docs/specs/doctor-mode-phase7/spec.md`
- Remaining plan: `docs/specs/doctor-mode-remaining-implementation-plan.md` § Phase 8
- Bubble Tea v1.3.10: https://pkg.go.dev/github.com/charmbracelet/bubbletea
- VS Code MCP input variables: https://code.visualstudio.com/docs/copilot/reference/mcp-configuration (verified May 2026)
- Claude Code MCP scopes and `${VAR:-default}` expansion: https://code.claude.com/docs/en/mcp (verified May 2026)
- Claude Code scope names (`local`, `project`, `user`): confirmed in official docs — `local` is default (was `project`), `project` is shared via `.mcp.json` (new), `user` is cross-project (was `global`)
- `pkg/validate/cache.go`: `validationCacheTTL = 24 * time.Hour`
- `pkg/validate/types.go`: `StatusOK`, `StatusFailed`, `ModeOffline`, `ModeLive`
- `pkg/app/plan_apply.go`: `PreflightSavedPlan`, `ApplySavedPlan`, `SavedPlanPreflight`, `ApprovalPrompt`
- `pkg/app/plan_v2.go`: `BuildSavedPlan`, `SavedPlanOptions`, `PlanOperation`, `CredentialRef`
- `pkg/manifest/providers.go`: `AllProviders()`, `ProviderMeta`, `CredentialAcquisition`, `LiveValidationSpec`, `QuotaSafe`
