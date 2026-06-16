# Doctor Mode Phase 8 — Technical Implementation Plan

**Status:** Revised — resubmitting for architecture review  
**Spec:** `docs/specs/doctor-mode-phase8/spec.md`  
**Review findings addressed:** B1 (callsite ordering), B2 (VS Code apply path), B3 (validate.Service injection), B4 (PreflightSavedPlan blocking)  
**Last updated:** 2026-05-23

---

## Summary

Phase 8 extends the Phase 7 read-only doctor dashboard into a full `doctor → validate → plan → apply` TUI path. It adds a five-state provider readiness view, opt-in live validation, doctor-derived target selection, saved-plan preview, and apply through the existing `app.ApplySavedPlan` API. Two targeted `pkg/app` additions support VS Code `${input:varName}` and Claude Code `${VAR:-default}` secret indirection; the VS Code path includes a new `pkg/config.MergeVSCodeInputs` helper that writes the `inputs` block to the VS Code config root at apply time. No new external dependencies.

---

## Inputs Reviewed

- `docs/specs/doctor-mode-phase8/spec.md` (all FR, AC, OQ)
- `docs/specs/doctor-mode-phase8/review.md` (B1–B4, N1–N4)
- `pkg/tui/dashboard.go` — Phase 7 `DashboardModel`, `DashboardScanner`
- `pkg/tui/dashboard_view.go` — Phase 7 `View()`, `renderReport()`
- `pkg/tui/model.go` — existing wizard `Model`; already imports `pkg/app` directly
- `pkg/app/plan_v2.go` — `SavedPlanOptions`, `BuildSavedPlan`, `buildPlanOperation`, `PlanOperation`
- `pkg/app/plan_apply.go` — `PreflightSavedPlan`, `ApplySavedPlan`, `Approver`, `SavedPlanApplyOptions`, `AutoApprove bool`
- `pkg/config/json_update.go` — `UpdateNamedServerJSON`, `buildConfigMap`; root-level `extra` gap confirmed
- `pkg/validate/types.go` — `Service`, `Report`, `StatusFailed`, `ModeOffline`, `ModeLive`
- `pkg/validate/cache.go` — `validationCacheTTL = 24 * time.Hour`
- `pkg/manifest/providers.go` — `AllProviders()`, `ProviderMeta`, `CredentialAcquisition`
- `pkg/provider/registry.go` — `DefaultRegistry()`, `Registry.Get(id string)` (public; accessible from `pkg/tui`)
- Bubble Tea v1.3.10 docs (Context7 / charmbracelet docs): `Cmd` is `func() Msg`; runs async in goroutine; `Update` must remain synchronous; all I/O in `Cmd` closures. Source: https://charmbracelet-bubbletea-43.mintlify.app/concepts/commands
- VS Code MCP input variable format (official docs, May 2026): `inputs` array at JSON root; `${input:id}` reference in server `env` and `headers` fields; VS Code resolves references securely at server start. Source: https://code.visualstudio.com/docs/copilot/reference/mcp-configuration

---

## Assumptions

1. `pkg/tui/model.go` already imports `pkg/app` directly; the package isolation guard is narrowed to "no direct config file JSON parsing in `pkg/tui`". `DashboardManager` uses `app` types directly.
2. `UseInputVariables` is set `true` by the TUI for all VS Code HTTP targets — a TUI policy choice, not a user prompt. CLI callers never set it; existing behaviour unchanged (resolves OQ-1).
3. Tavily rate-limit debounce: one in-flight live validation at a time (`validating == true` blocks re-trigger). Actual rate limit is not relied upon (resolves OQ-2).
4. `DashboardManager.HomeDir() string` retained for `PlanStore`; full injection deferred to Phase 11 (resolves OQ-4).
5. `NewDashboardModel` changes to a 3-arg constructor; existing Phase 7 call sites updated in **Task 4** (not Task 10) — fixes review finding B1.
6. `alwaysApprover` is replaced by `SavedPlanApplyOptions{AutoApprove: true}` — addresses review finding N1; uses the existing, tested field.
7. Provider lookup (`manifest.ProviderID` → `provider.MCPProvider`) uses `provider.DefaultRegistry().Get(id)` directly inside `pkg/tui` — no additional interface method needed (addresses review note N3).

---

## Architecture Approach

### State Machine

`DashboardModel` gains a `screen dashboardScreen` field. `Update` routes key events and messages based on current screen. `View` dispatches to per-screen helpers. No nested `tea.Program` instances. All I/O — including `PreflightSavedPlan` — runs in `tea.Cmd` closures per Bubble Tea's architecture contract.

```
screenDoctor (Phase 7 base)
  → [p / enter] when loaded and manager ≠ nil → readinessCmd() → screenProviderReady

screenProviderReady
  → [↑/↓ / k/j]  navigate providerCursor
  → [v] when !validating → liveValidationCmd()
  → [enter]       → offlineValidationCmd() → on pass → screenTargetSelect
  → [esc]         → screenDoctor

screenTargetSelect
  → [↑/↓ / k/j]  navigate clientCursor
  → [space]       toggle selectedClients
  → [i]           toggle includeWorkspace
  → [enter] any selected → planCmd() → planCreatedMsg → preflightCmd() → screenPlanPreview
  → [esc]         → screenProviderReady

screenPlanPreview
  → [y / enter]   → applyCmd()
  → [n / esc]     → screenTargetSelect

screenApplyResult
  → auto-triggers scanCmd() on enter
  → [r]           → rescan
  → [q / ctrl+c]  → quit
```

### B2 Resolution — VS Code Secret Indirection at Both Plan and Apply Time

**Problem confirmed:** `buildOperationFromSavedPlan` always calls `prov.GenerateConfig(realCredentials)` and produces a concrete `MCPConfig` with real header/env values. `config.UpdateNamedServerJSON` writes them verbatim. `VSCodeInputs` metadata on `PlanOperation` was never consulted.

**Fix — two-phase approach:**

**Phase A — Plan creation** (`buildPlanOperation` when `UseInputVariables && VS Code HTTP`):
Replace credential header values with `${input:<input-id>}` literals in the `MCPConfig` *before* the plan is stored. This means the plan stores a `Config` with `${input:…}` already substituted. At apply time, `buildOperationFromSavedPlan` resolves these from the plan without re-inserting real credentials. The `VSCodeInputs` slice on `PlanOperation` stores the corresponding `inputs` block definitions (type, id, description, password).

**Phase B — Apply time** (`prepareSavedPlan` when `len(planOp.VSCodeInputs) > 0`):
After `config.UpdateNamedServerJSON` writes the server entry (with `${input:…}` literals already in headers), call a new `config.MergeVSCodeInputs(existingContent, planOp.VSCodeInputs)` that merges the `inputs` array into the root of the VS Code JSON. This is a root-level merge, separate from the per-server write, because VS Code `inputs` lives at root scope alongside `servers`.

```
VS Code mcp.json written structure:
{
  "inputs": [{ "type": "promptString", "id": "exa-api-key", "description": "Exa API Key", "password": true }],
  "servers": {
    "exa": { "type": "http", "url": "https://...", "headers": { "Authorization": "${input:exa-api-key}" } }
  }
}
```

### B3 Resolution — validate.Service Injection

`DashboardManager` gains a `Validate` method. `*app.Manager` does not implement `validate.Service.ValidateProfiles` directly, so `cmd/usync/main.go` builds a `dashboardManagerAdapter` struct that wraps both `*app.Manager` and `validate.Service`:

```go
// In cmd/usync (package main, not exported):
type dashboardManagerAdapter struct {
    *app.Manager
    validator validate.Service
}

func (a dashboardManagerAdapter) Validate(
    ctx context.Context,
    prov provider.MCPProvider,
    profiles []provider.CredentialProfile,
    live bool,
) (validate.Report, error) {
    return a.validator.ValidateProfiles(ctx, prov, profiles, live)
}
```

`dashboardManagerAdapter` satisfies `DashboardManager` via embedding of `*app.Manager` for the plan/apply methods plus the explicit `Validate` method.

### B4 Resolution — PreflightSavedPlan as tea.Cmd

Per Bubble Tea's architecture: `Cmd` is `func() Msg`, runs async in a goroutine; `Update` must be synchronous. `PreflightSavedPlan` does file I/O (SHA reads, symlink resolution) and must run in a `Cmd`.

`planCreatedMsg` handler: sets `planning = false`, stores `currentPlan`, then returns `preflightCmd()`.  
`preflightCmd()` calls `PreflightSavedPlan`; returns `preflightResultMsg{preflight, err}`.  
`preflightResultMsg` handler: sets `planPreflight`, advances to `screenPlanPreview`.

---

## Affected Modules

| Module | Change Type | Scope |
|---|---|---|
| `pkg/tui/dashboard.go` | Extend | Screen enum, 15 new fields, 3-arg constructor, 6 new cmds, `DashboardManager` interface |
| `pkg/tui/dashboard_view.go` | Extend | 4 new screen render methods |
| `pkg/tui/dashboard_readiness.go` | New file | `ProviderState`, `ProviderReadinessItem`, pure `ComputeReadiness` |
| `pkg/tui/dashboard_test.go` | Extend | Phase 8 unit tests |
| `pkg/tui/dashboard_teatest_test.go` | Extend | Full-flow teatest |
| `pkg/app/plan_v2.go` | Extend | `VSCodeInput`, `VSCodeInputs` on `PlanOperation`; `UseInputVariables`/`UseEnvExpansion` on `SavedPlanOptions`; credential substitution in `buildPlanOperation` |
| `pkg/app/plan_apply.go` | Extend | Call `config.MergeVSCodeInputs` in `prepareSavedPlan` when `VSCodeInputs` present |
| `pkg/app/plan_v2_test.go` | Extend | Secret indirection unit tests |
| `pkg/config/json_update.go` | Extend | New `MergeVSCodeInputs(data []byte, inputs []VSCodeInput) ([]byte, error)` |
| `cmd/usync/main.go` | Extend | `dashboardManagerAdapter`; wire to `NewDashboardModel` |

---

## API and Contract Changes

### `SavedPlanOptions` (pkg/app/plan_v2.go)

```go
type SavedPlanOptions struct {
    PlanID            string
    CreatedAt         time.Time
    UsyncVersion      string
    ProviderID        string
    Credentials       []CredentialRef
    Doctor            DoctorSummary
    UseInputVariables bool // emit ${input:id} for VS Code HTTP targets (default false)
    UseEnvExpansion   bool // emit ${VAR:-} for Claude Code project-scope targets (default false)
}
```

### `VSCodeInput` and `PlanOperation` extension (pkg/app/plan_v2.go)

```go
type VSCodeInput struct {
    Type        string `json:"type"`        // always "promptString"
    ID          string `json:"id"`
    Description string `json:"description"`
    Password    bool   `json:"password"`
}

// Added to PlanOperation:
VSCodeInputs []VSCodeInput `json:"vscode_inputs,omitempty"`
```

### `config.MergeVSCodeInputs` (pkg/config/json_update.go)

```go
// MergeVSCodeInputs merges the given input definitions into the root-level
// "inputs" array of a VS Code mcp.json file, preserving existing entries.
// Entries with the same id are replaced. Safe to call with an empty inputs slice.
func MergeVSCodeInputs(data []byte, inputs []app.VSCodeInput) ([]byte, error)
```

Note: to avoid a circular import (`pkg/config` importing `pkg/app`), `VSCodeInput` is moved to a shared location or redefined as `config.VSCodeInput`. The cleanest option: define `config.VSCodeInput` in `pkg/config/json_update.go` (mirroring the `app.VSCodeInput` shape) and have `app.VSCodeInput` alias or embed it. Since the types are identical structs, `plan_apply.go` uses `config.VSCodeInput` when calling `MergeVSCodeInputs`. `app.PlanOperation.VSCodeInputs` stores `[]config.VSCodeInput`.

### `DashboardManager` interface (pkg/tui/dashboard.go)

```go
type DashboardManager interface {
    PrepareProvider(
        prov      provider.MCPProvider,
        profiles  []provider.CredentialProfile,
        selected  map[config.AppID]bool,
        assign    map[config.AppID]int,
    ) (app.ExecutionPlan, error)
    BuildSavedPlan(plan app.ExecutionPlan, opts app.SavedPlanOptions) (app.SavedPlan, error)
    PreflightSavedPlan(plan app.SavedPlan, opts app.SavedPlanApplyOptions) (app.SavedPlanPreflight, error)
    ApplySavedPlan(plan app.SavedPlan, opts app.SavedPlanApplyOptions) (app.ApplyResult, error)
    Validate(ctx context.Context, prov provider.MCPProvider, profiles []provider.CredentialProfile, live bool) (validate.Report, error)
    HomeDir() string
}
```

`*app.Manager` satisfies all methods except `Validate`. The `dashboardManagerAdapter` in `cmd/usync` adds `Validate` via composition.

### `DashboardModel` constructor (pkg/tui/dashboard.go)

```go
// Phase 8 (3-arg, nil-safe):
func NewDashboardModel(
    scanner  DashboardScanner,
    manager  DashboardManager,            // nil → Phase 8 screens unreachable
    profiles []provider.CredentialProfile, // nil → empty
) DashboardModel
```

### New typed messages (pkg/tui/dashboard.go)

```go
type providerReadinessMsg struct{ items []ProviderReadinessItem; err error }
type validationResultMsg  struct{ report validate.Report; live bool; err error }
type planCreatedMsg       struct{ plan app.SavedPlan; path string; err error }
type preflightResultMsg   struct{ preflight app.SavedPlanPreflight; err error }  // NEW (B4)
type applyResultMsg       struct{ result app.ApplyResult; err error }
```

---

## Data Model Changes

- `VSCodeInput` / `VSCodeInputs`: additive JSON on `PlanOperation`; `omitempty` keeps existing plan files valid.
- `config.VSCodeInput` is a new type mirroring `app.VSCodeInput`; no storage schema change.
- No database migrations.

---

## Dependency Changes

**None.** All required packages are in `go.mod`. `validate.Service` is already imported by `cmd/usync`. No new modules.

---

## Security Impact

- Raw credentials never stored in `DashboardModel` after offline validation; only labels and `redact.Key()` suffixes.
- VS Code: `${input:exa-api-key}` literal is written to disk. VS Code stores the actual key in its secure credential store on first server start, not in `mcp.json`. Source: VS Code MCP docs, May 2026.
- Claude Code `.mcp.json` project scope: `${EXA_API_KEY:-}` written; real key must be in environment. Not stored in the plan file.
- `AutoApprove: true` in `applyCmd` is only passed after the user explicitly pressed `y` in `screenPlanPreview` and saw all `ApprovalPrompts`.
- `MergeVSCodeInputs` reads and writes existing VS Code config; no new secret handling beyond what `UpdateNamedServerJSON` already does.

---

## Authorization Boundaries

| Boundary | Behaviour |
|---|---|
| Workspace/project-scoped targets | Excluded unless `includeWorkspace` toggled; approval prompt always shown in `screenPlanPreview` |
| Symlink targets | `PreflightSavedPlan` raises `ApprovalPrompt`; rendered in `screenPlanPreview` before apply |
| New file creation | `WillCreate == true` → approval prompt displayed; `AutoApprove: true` only after user confirms `y` |
| Conflict clients | Excluded from `selectedClients`; warning in `screenTargetSelect` |

---

## Observability Impact

None beyond existing `app.ApplySavedPlan` audit log entries.

---

## Testing Strategy

| Layer | Approach |
|---|---|
| `ComputeReadiness` | Table-driven, pure function, no I/O, five state cases |
| `DashboardModel.Update` | Direct typed-message injection; `preflightResultMsg` tested explicitly |
| `View()` | String fragment assertions; no ANSI assertions |
| `config.MergeVSCodeInputs` | Unit tests: empty, merge, dedup by id, preserve existing |
| Secret indirection | `BuildSavedPlan` with `UseInputVariables` / `UseEnvExpansion`; assert `${input:…}` in headers, no raw key |
| `prepareSavedPlan` | Unit test with `VSCodeInputs` present; assert `inputs` in written JSON |
| Full TUI flow | `teatest.WaitFor` chain through all 5 screens; `FakeDashboardManager` |
| Regression | All Phase 7 tests unchanged; updated call sites compile and pass |

---

## Failure Modes

| Failure | Handling |
|---|---|
| `scanCmd` error | Phase 7 unchanged |
| `readinessCmd` error | `readinessErr` set; error shown in `screenProviderReady` |
| Offline validation fails | Blocks advance; error shown redacted |
| Live validation network error | Treated as validation failure; `validating` cleared |
| `planCmd` error | `planErr` set; plan-error state in `screenProviderReady` / does not advance |
| `preflightCmd` error | `planErr` set; stays on `screenProviderReady`; plan file cleaned up |
| `applyCmd` error | `applyErr` set; shown in `screenApplyResult`; rescan still runs |
| `MergeVSCodeInputs` error | Surfaced in `prepareSavedPlan` as a write error; apply aborted |

---

## Rollback and Recovery

- Phase 7 code unchanged; `manager == nil` keeps Phase 7 behaviour.
- `UseInputVariables / UseEnvExpansion` default `false`; CLI callers unaffected.
- `VSCodeInputs` is `omitempty`; existing plan files remain valid.
- `config.VSCodeInput` is a new type with no storage implications.
- `--wizard` still bypasses dashboard entirely.

---

## Risks and Mitigations

| Risk | Severity | Mitigation |
|---|---|---|
| `DashboardModel` state explosion | Medium | Screen enum; isolated render helpers per screen |
| `pkg/tui` → `pkg/app` coupling | Low | Already present in `model.go`; interface keeps unit tests clean |
| `MergeVSCodeInputs` corrupts existing VS Code config | Medium | Read-parse-merge-write with existing `marshalJSON` pattern; unit tests with existing entries |
| VS Code `${input:…}` in headers breaks CLI apply | Low | `UseInputVariables` only set by TUI; CLI callers default `false` |
| `config.VSCodeInput` / `app.VSCodeInput` type duplication | Low | Both are tiny identical structs; dedup in Phase 11 if needed |

---

## Numbered Implementation Tasks

---

### Task 1 — `pkg/config`: Add `MergeVSCodeInputs`

**Files:** `pkg/config/json_update.go`  
**Depends on:** nothing  
**Risk:** Low

Add `config.VSCodeInput` struct and `MergeVSCodeInputs`:

```go
type VSCodeInput struct {
    Type        string `json:"type"`
    ID          string `json:"id"`
    Description string `json:"description"`
    Password    bool   `json:"password"`
}

// MergeVSCodeInputs merges input definitions into the root-level "inputs"
// array of VS Code mcp.json content. Entries with duplicate IDs are replaced.
// Existing non-conflicting entries are preserved.
func MergeVSCodeInputs(data []byte, inputs []VSCodeInput) ([]byte, error) {
    root, err := decodeJSONObject(data)
    if err != nil {
        return nil, err
    }
    // Load existing inputs.
    existing := map[string]VSCodeInput{}
    if raw, ok := root["inputs"].([]any); ok {
        for _, item := range raw {
            if m, ok := item.(map[string]any); ok {
                if id, ok := m["id"].(string); ok {
                    existing[id] = decodeVSCodeInput(m)
                }
            }
        }
    }
    // Merge new inputs (replace on duplicate id).
    for _, inp := range inputs {
        existing[inp.ID] = inp
    }
    // Write back as ordered slice (stable: existing order, then new).
    merged := make([]any, 0, len(existing))
    for _, inp := range existing {
        merged = append(merged, inp)
    }
    root["inputs"] = merged
    return marshalJSON(root)
}
```

**Acceptance:** `go test ./pkg/config -run TestMergeVSCodeInputs` passes; existing config tests unchanged.

---

### Task 2 — `pkg/app`: Add secret indirection to `SavedPlanOptions`, `PlanOperation`, and `buildPlanOperation`

**Files:** `pkg/app/plan_v2.go`  
**Depends on:** Task 1 (imports `config.VSCodeInput`)  
**Risk:** Low–Medium

1. Replace `app.VSCodeInput` with `config.VSCodeInput` (type alias or direct use):

```go
// Import pkg/config; use config.VSCodeInput throughout.
// Add to PlanOperation:
VSCodeInputs []config.VSCodeInput `json:"vscode_inputs,omitempty"`
```

2. Add to `SavedPlanOptions`:

```go
UseInputVariables bool
UseEnvExpansion   bool
```

3. Thread `opts` into `buildPlanOperation`:

```go
func (m *Manager) buildPlanOperation(
    op                   Operation,
    credentialRefsByLabel map[string]CredentialRef,
    opts                 SavedPlanOptions,
) (PlanOperation, error)
```

4. In `buildPlanOperation`, after setting `planOp.FilePath`, add credential substitution:

```go
// VS Code HTTP secret indirection (FR-17)
if opts.UseInputVariables &&
    op.AppID == config.AppVSCode &&
    op.Kind == config.FileKindNamedServer &&
    op.Config.Type != provider.TransportStdio {

    inputID := strings.ToLower(strings.ReplaceAll(op.ProviderID, "_", "-")) + "-api-key"
    planOp.VSCodeInputs = []config.VSCodeInput{{
        Type:        "promptString",
        ID:          inputID,
        Description: op.ProviderID + " API Key",
        Password:    true,
    }}
    // Replace credential-bearing headers with ${input:id} literals.
    // The substituted MCPConfig is what gets written to disk at apply time.
    substituted := make(map[string]string, len(op.Config.Headers))
    for k := range op.Config.Headers {
        substituted[k] = "${input:" + inputID + "}"
    }
    op.Config.Headers = substituted
    planOp.Redacted = redact.Text(fmt.Sprintf(
        "%s: %s %s [vscode-input:${input:%s}]",
        op.AppName, planOp.Action, op.ProviderID, inputID,
    ))
}

// Claude Code project-scope env expansion (FR-18)
if opts.UseEnvExpansion &&
    op.AppID == config.AppClaudeCode &&
    op.Scope == string(manifest.ScopeProject) {
    planOp.Redacted = redact.Text(fmt.Sprintf(
        "%s: %s %s [env-expansion, credential=%s]",
        op.AppName, planOp.Action, op.ProviderID, op.CredentialLabel,
    ))
}
```

**Note:** `op.Config.Headers` modification affects only this local copy inside the `Cmd` closure; the original plan profiles are not mutated.

**Acceptance:** `go test ./pkg/app` passes; existing tests unchanged.

---

### Task 3 — `pkg/app`: Call `MergeVSCodeInputs` in `prepareSavedPlan`

**Files:** `pkg/app/plan_apply.go`  
**Depends on:** Tasks 1, 2  
**Risk:** Low

In `prepareSavedPlan`, after computing `item.prepared.content` for a `FileKindNamedServer` operation:

```go
if len(planOp.VSCodeInputs) > 0 {
    merged, err := config.MergeVSCodeInputs(item.prepared.content, planOp.VSCodeInputs)
    if err != nil {
        return savedPreparedApply{}, fmt.Errorf("%s: merge VS Code inputs: %w", planOp.TargetName, err)
    }
    item.prepared.content = merged
}
```

**Acceptance:** `go test ./pkg/app -run TestPreflightSavedPlan` passes; new test `TestPrepareSavedPlan_VSCodeInputsMerged` passes.

---

### Task 4 — `pkg/app`: Tests for secret indirection and `MergeVSCodeInputs`

**Files:** `pkg/app/plan_v2_test.go`, `pkg/config/json_update_test.go`  
**Depends on:** Tasks 1–3  
**Risk:** Low

**`pkg/config`:**
- `TestMergeVSCodeInputs_Empty` — no-op on file without inputs.
- `TestMergeVSCodeInputs_Adds` — adds input to empty inputs array.
- `TestMergeVSCodeInputs_Deduplicates` — same id replaces existing entry.
- `TestMergeVSCodeInputs_PreservesOthers` — unrelated existing inputs kept.
- `TestMergeVSCodeInputs_MalformedJSON` — returns error.

**`pkg/app`:**
- `TestBuildSavedPlan_VSCodeInputVariables` — `UseInputVariables: true` on VS Code HTTP target; assert `VSCodeInputs` non-empty, `Redacted` contains `${input:`, no raw key in any string field.
- `TestBuildSavedPlan_ClaudeCodeEnvExpansion` — `UseEnvExpansion: true` project scope; `Redacted` contains `env-expansion`.
- `TestBuildSavedPlan_DefaultFlagsNoChange` — both flags `false`; `VSCodeInputs` nil.
- `TestPrepareSavedPlan_VSCodeInputsMerged` — `prepareSavedPlan` on plan with `VSCodeInputs`; written content contains `"inputs"` key at root.

**Acceptance:** `go test ./pkg/config ./pkg/app` all pass.

---

### Task 5 — `pkg/tui`: Provider readiness types and `ComputeReadiness`

**Files:** `pkg/tui/dashboard_readiness.go` (new)  
**Depends on:** nothing  
**Risk:** Low

```go
package tui

import (
    "github.com/nawodyaishan/universal-mcp-sync/pkg/doctor"
    "github.com/nawodyaishan/universal-mcp-sync/pkg/manifest"
    "github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
)

type ProviderState string

const (
    ProviderStateNoKeyNeeded       ProviderState = "no-key-needed"
    ProviderStateReady             ProviderState = "ready"
    ProviderStateMissingCredential ProviderState = "missing-credentials"
    ProviderStateRuntimeMissing    ProviderState = "runtime-missing"
    ProviderStateConflictBlocked   ProviderState = "conflict-blocked"
)

type ProviderReadinessItem struct {
    Meta    manifest.ProviderMeta
    State   ProviderState
    Reasons []string
}

// ComputeReadiness classifies providers. Pure function: no I/O.
// Severity order: conflict-blocked > runtime-missing > missing-credentials > ready > no-key-needed.
func ComputeReadiness(
    providers []manifest.ProviderMeta,
    report    doctor.Report,
    profiles  []provider.CredentialProfile,
) []ProviderReadinessItem
```

**Acceptance:** `go test ./pkg/tui -run TestComputeReadiness` passes.

---

### Task 6 — `pkg/tui`: `DashboardManager` interface, typed messages, screen enum, model extension

**Files:** `pkg/tui/dashboard.go`  
**Depends on:** Task 5  
**Risk:** Low (additive)

**6a — Interfaces and typed messages:**

```go
type DashboardManager interface {
    PrepareProvider(prov provider.MCPProvider, profiles []provider.CredentialProfile,
        selected map[config.AppID]bool, assignments map[config.AppID]int) (app.ExecutionPlan, error)
    BuildSavedPlan(plan app.ExecutionPlan, opts app.SavedPlanOptions) (app.SavedPlan, error)
    PreflightSavedPlan(plan app.SavedPlan, opts app.SavedPlanApplyOptions) (app.SavedPlanPreflight, error)
    ApplySavedPlan(plan app.SavedPlan, opts app.SavedPlanApplyOptions) (app.ApplyResult, error)
    Validate(ctx context.Context, prov provider.MCPProvider,
        profiles []provider.CredentialProfile, live bool) (validate.Report, error)
    HomeDir() string
}

type dashboardScreen int
const (
    screenDoctor        dashboardScreen = iota
    screenProviderReady
    screenTargetSelect
    screenPlanPreview
    screenApplyResult
)

// Typed messages:
type providerReadinessMsg struct{ items []ProviderReadinessItem; err error }
type validationResultMsg  struct{ report validate.Report; live bool; err error }
type planCreatedMsg       struct{ plan app.SavedPlan; path string; err error }
type preflightResultMsg   struct{ preflight app.SavedPlanPreflight; err error }
type applyResultMsg       struct{ result app.ApplyResult; err error }
```

**6b — Extend `DashboardModel`** (add after `showHelp`):

```go
    manager          DashboardManager
    profiles         []provider.CredentialProfile
    screen           dashboardScreen
    providerCursor   int
    readiness        []ProviderReadinessItem
    readinessErr     error
    computingReady   bool
    validating       bool
    validReport      *validate.Report
    validErr         error
    selectedProv     int
    clientCursor     int
    selectedClients  map[manifest.ClientID]bool
    includeWorkspace bool
    planning         bool
    currentPlan      *app.SavedPlan
    planPath         string
    preflighting     bool
    planPreflight    *app.SavedPlanPreflight
    planErr          error
    applying         bool
    applyResult      *app.ApplyResult
    applyErr         error
```

**6c — Update `NewDashboardModel` and update Phase 7 call sites (B1 fix):**

```go
func NewDashboardModel(
    scanner  DashboardScanner,
    manager  DashboardManager,
    profiles []provider.CredentialProfile,
) DashboardModel {
    return DashboardModel{scanner: scanner, scanning: true, manager: manager, profiles: profiles}
}
```

Update every Phase 7 call site in the same task:
- `pkg/tui/dashboard_test.go`: `NewDashboardModel(scanner)` → `NewDashboardModel(scanner, nil, nil)`
- `pkg/tui/dashboard_teatest_test.go`: same

**Acceptance:** `go build ./pkg/tui` passes; all Phase 7 tests compile.

---

### Task 7 — `pkg/tui`: Extend `Update` with Phase 8 commands and state transitions

**Files:** `pkg/tui/dashboard.go`  
**Depends on:** Task 6  
**Risk:** Medium (state machine)

**Command constructors** (all return `tea.Cmd`; all I/O in closure, never in `Update`):

```go
func (m DashboardModel) readinessCmd() tea.Cmd      // ComputeReadiness in closure → providerReadinessMsg
func (m DashboardModel) offlineValidationCmd() tea.Cmd // m.manager.Validate(ctx, prov, profiles, false) → validationResultMsg{live:false}
func (m DashboardModel) liveValidationCmd() tea.Cmd    // m.manager.Validate(ctx, prov, profiles, true)  → validationResultMsg{live:true}
func (m DashboardModel) planCmd() tea.Cmd              // PrepareProvider → BuildSavedPlan → PlanStore.Save → planCreatedMsg
func (m DashboardModel) preflightCmd() tea.Cmd         // PreflightSavedPlan → preflightResultMsg  (B4 fix)
func (m DashboardModel) applyCmd() tea.Cmd             // ApplySavedPlan{AutoApprove:true} → applyResultMsg  (N1 fix)
```

**Key routing additions to `Update`:**

| Screen | Key | Action |
|---|---|---|
| `screenDoctor` | `p`/`enter` (loaded, no err, manager≠nil) | `screen=screenProviderReady`, `computingReady=true`, return `readinessCmd()` |
| `screenProviderReady` | `↑`/`k`, `↓`/`j` | navigate `providerCursor` |
| `screenProviderReady` | `v` when `!validating` | `validating=true`, return `liveValidationCmd()` |
| `screenProviderReady` | `enter` | `validating=true`, return `offlineValidationCmd()` |
| `screenProviderReady` | `esc` | `screen=screenDoctor` |
| `screenTargetSelect` | `↑`/`k`, `↓`/`j` | navigate `clientCursor` |
| `screenTargetSelect` | `space` | toggle `selectedClients` |
| `screenTargetSelect` | `i` | toggle `includeWorkspace` |
| `screenTargetSelect` | `enter` (any selected) | `planning=true`, return `planCmd()` |
| `screenTargetSelect` | `esc` | `screen=screenProviderReady` |
| `screenPlanPreview` | `y`/`enter` | `applying=true`, return `applyCmd()` |
| `screenPlanPreview` | `n`/`esc` | `screen=screenTargetSelect` |
| `screenApplyResult` | `r` | rescan |
| `screenApplyResult` | `q`/`ctrl+c` | quit |

**Message handlers:**

- `providerReadinessMsg` → set `readiness`, `readinessErr`, `computingReady=false`; default `selectedProv` to first ready/no-key-needed; populate `selectedClients` from eligible report clients (high/medium confidence, installed, excluding workspace unless `includeWorkspace`)
- `validationResultMsg` → set `validReport`, `validErr`, `validating=false`; if `!live && !report.HasFailures()` → `screen=screenTargetSelect`; if live or failed → stay on `screenProviderReady`
- `planCreatedMsg` → set `currentPlan`, `planPath`, `planErr`, `planning=false`; if no error → `preflighting=true`, return `preflightCmd()` **(B4 fix)**; if error → show error
- `preflightResultMsg` → set `planPreflight`, clear `preflighting`; if no error → `screen=screenPlanPreview`; if error → set `planErr`, return to `screenProviderReady` **(B4 fix)**
- `applyResultMsg` → set `applyResult`, `applyErr`, `applying=false`; `screen=screenApplyResult`; return `scanCmd()` (rescan)

**Acceptance:** `go test ./pkg/tui -run TestDashboard` passes.

---

### Task 8 — `pkg/tui`: New screen renders in `dashboard_view.go`

**Files:** `pkg/tui/dashboard_view.go`  
**Depends on:** Task 7  
**Risk:** Low

Extend `View()`:

```go
switch m.screen {
case screenProviderReady:
    return renderShell(m.renderProviderReady(), stageSetup, m.width)
case screenTargetSelect:
    return renderShell(m.renderTargetSelect(), stageSetup, m.width)
case screenPlanPreview:
    return renderShell(m.renderPlanPreview(), stageSetup, m.width)
case screenApplyResult:
    return renderShell(m.renderApplyResult(), stageSetup, m.width)
default:
    // Phase 7 path unchanged
}
```

Four new render methods:

**`renderProviderReady`** — provider list grouped: ready/no-key-needed first; state label; get-key URL for missing-credentials. Spinner when `validating || computingReady`. Error via `redact.Text`. Action bar: `[↑↓] navigate  [v] live validate  [Enter] select  [Esc] back  [?] help  [q] quit`.

**`renderTargetSelect`** — client checkboxes `[x]`/`[ ]`; skipped clients with `!` reason; `[i]` workspace toggle label. Spinner when `planning || preflighting`. Action bar: `[↑↓] navigate  [Space] toggle  [i] workspace  [Enter] plan  [Esc] back  [q] quit`.

**`renderPlanPreview`** — `app.FormatSavedPlan(*m.currentPlan, time.Now())` truncated to `m.width`. Approval prompts as `! <message>`. Confirm prompt: `Press [y] to apply or [n] to cancel`. Spinner when `applying`. Error from `planErr` if set.

**`renderApplyResult`** — `app.FormatApplyResult(*m.applyResult)` or `applyErr.Error()`. Rescan spinner when `scanning`. Action bar: `[r] rescan  [q] quit`.

**Acceptance:** `go test ./pkg/tui` passes; no raw credential in any `View()` output.

---

### Task 9 — `pkg/tui`: Phase 8 unit tests

**Files:** `pkg/tui/dashboard_test.go`  
**Depends on:** Tasks 5, 7, 8  
**Risk:** Low

New tests:
- `TestComputeReadiness_AllFiveStates` — table-driven pure function.
- `TestDashboardModel_PAdvancesToProviderReady` — press `p` after scan → `screenProviderReady`.
- `TestDashboardModel_EscReturnsFromEachScreen` — subtests per screen.
- `TestDashboardModel_LiveValidationDebounce` — second `v` while `validating` → no second cmd.
- `TestDashboardModel_ValidationFailBlocksPlan` — `validationResultMsg{live:false, report:failing}` → screen stays on `screenProviderReady`.
- `TestDashboardModel_PlanToPreflightChain` — `planCreatedMsg{no err}` → `preflighting=true` + returns `preflightCmd()`.
- `TestDashboardModel_PreflightResultAdvances` — `preflightResultMsg{no err}` → `screenPlanPreview`.
- `TestDashboardModel_PreflightErrorShowsError` — `preflightResultMsg{err}` → `planErr` set, screen not `screenPlanPreview`.
- `TestDashboardModel_ApplyResultTriggersRescan` — `applyResultMsg` → cmd non-nil.
- `TestDashboardModel_NoRawCredentialInView` — UUID key injected via profiles; all 5 screens called; assert UUID absent from output.
- `TestDashboardModel_WorkspaceScopedClientExcluded` — workspace-scoped candidate not in default `selectedClients`.
- `TestDashboardModel_NilManagerDisablesPhase8` — `manager==nil`, press `p` → `screen==screenDoctor`.

**Acceptance:** `go test ./pkg/tui -v -run TestDashboard` all pass.

---

### Task 10 — `pkg/tui`: Teatest full-flow

**Files:** `pkg/tui/dashboard_teatest_test.go`  
**Depends on:** Tasks 7, 8, 9  
**Risk:** Low

`FakeDashboardManager` implements `DashboardManager`:
- `Validate` → returns `validate.Report{Results: []validate.Result{{Status: validate.StatusOK}}}`, nil
- `PrepareProvider` → canned `ExecutionPlan`
- `BuildSavedPlan` → canned `SavedPlan`
- `PreflightSavedPlan` → `SavedPlanPreflight{ApprovalPrompts: nil}`
- `ApplySavedPlan` → canned `ApplyResult`
- `HomeDir()` → `t.TempDir()`

Test `TestDashboardPhase8_FullFlow`:
1. `teatest.WaitFor` → `"System Status"` (doctor loaded)
2. Send `p` → `teatest.WaitFor` → `"Provider Readiness"` or `"no-key-needed"` / `"ready"`
3. Send `enter` → `teatest.WaitFor` → `"Select targets"` or client name
4. Send `enter` → `teatest.WaitFor` → `"Plan Preview"` or `"To apply"`
5. Send `y` → `teatest.WaitFor` → `"Applied"` or `"Scanning"`
6. Send `q` → `tm.WaitFinished`
7. Assert no UUID in captured output; no `time.Sleep` calls

**Acceptance:** `go test ./pkg/tui -run TestDashboardPhase8` passes.

---

### Task 11 — `cmd/usync/main.go`: Wire adapter and profiles

**Files:** `cmd/usync/main.go`  
**Depends on:** Tasks 6, 7  
**Risk:** Low

Add `dashboardManagerAdapter` in `main.go` (unexported, package main):

```go
type dashboardManagerAdapter struct {
    *app.Manager
    validator validate.Service
}

func (a dashboardManagerAdapter) Validate(
    ctx      context.Context,
    prov     provider.MCPProvider,
    profiles []provider.CredentialProfile,
    live     bool,
) (validate.Report, error) {
    return a.validator.ValidateProfiles(ctx, prov, profiles, live)
}
```

In the dashboard branch of `run()`:

```go
vs, err := validate.NewService(manager.HomeDir)
if err != nil {
    _, _ = fmt.Fprintln(stderr, err)
    return 1
}
adapter := dashboardManagerAdapter{Manager: manager, validator: vs}
scanner := tui.NewProductionScanner(homeDir, workspaceDir)
model   := tui.NewDashboardModel(scanner, adapter, initialProfiles)
```

`initialProfiles` is derived from `initialKeys`/`initialRaw` by constructing `[]provider.CredentialProfile` directly (same structure as `loadValidationProfiles` but without a provider — profiles are provider-agnostic at this point; the provider is chosen in `screenProviderReady`). If no keys supplied: `nil` profiles — all providers show `missing-credentials` (graceful degradation).

**Acceptance:** `go test ./cmd/usync` passes; `go build ./cmd/usync` succeeds.

---

### Task 12 — Regression and full suite

**Files:** No new edits (verification only)  
**Depends on:** All tasks  
**Risk:** Low

```bash
go test ./pkg/tui ./pkg/validate ./pkg/app ./pkg/config
go test ./...
make lint
make build
```

**Acceptance:** All packages pass; no new lint errors.

---

## Task Dependency Graph

```
Task 1  (config.MergeVSCodeInputs)
  └── Task 2  (app: SavedPlanOptions + buildPlanOperation credential substitution)
        └── Task 3  (app: MergeVSCodeInputs in prepareSavedPlan)
              └── Task 4  (tests: config + app secret indirection)

Task 5  (tui: readiness types + ComputeReadiness)
  └── Task 6  (tui: DashboardManager + typed msgs + model extension + B1 callsite fix)
        └── Task 7  (tui: Update state machine + all cmds + B4 preflightCmd)
              ├── Task 8  (tui: View renders)
              │     └── Task 9  (tui: unit tests)
              │           └── Task 10 (tui: teatest flow)
              └── Task 11 (cmd/usync: adapter + wiring)
                    └── Task 12 (regression + full suite)

Tasks 1–4 and Tasks 5–10 may proceed in parallel.
Task 11 requires Tasks 6 and 7.
Task 12 requires all prior tasks.
```

---

## Recommended PR Boundary

- **PR 8a:** Tasks 1–4 (`pkg/config` + `pkg/app` — self-contained, no TUI changes).
- **PR 8b:** Tasks 5–10 (`pkg/tui` Phase 8 screens, commands, tests).
- **PR 8c:** Tasks 11–12 (wiring + regression; requires 8a and 8b merged).

---

## Review Findings Addressed

| Finding | Resolution |
|---|---|
| B1 — callsite ordering | Phase 7 call sites updated in Task 6 (same task as constructor change) |
| B2 — apply path for VS Code `${input:…}` | Credential substitution at plan-creation time (`buildPlanOperation`); `MergeVSCodeInputs` at apply time (`prepareSavedPlan`) |
| B3 — validate.Service injection | `Validate` method on `DashboardManager`; `dashboardManagerAdapter` in `cmd/usync` |
| B4 — PreflightSavedPlan blocking Update | `preflightCmd()` + `preflightResultMsg`; `planCreatedMsg` launches cmd, not inline call |
| N1 — alwaysApprover | Replaced with `SavedPlanApplyOptions{AutoApprove: true}` |
| N2 — ClientID/AppID cast | Explicit `config.AppID(id)` casts documented in Task 7 |
| N3 — provider registry | `provider.DefaultRegistry().Get(id)` is public; used directly in `planCmd` |
| N4 — PlanStore coupling | Documented; deferred to Phase 11 |

---

## References

- Bubble Tea `Cmd` architecture: https://charmbracelet-bubbletea-43.mintlify.app/concepts/commands — "All I/O in Cmd closures; Update must remain synchronous." (Context7, May 2026)
- Bubble Tea commands tutorial: https://github.com/charmbracelet/bubbletea/blob/main/tutorials/commands/README.md (Context7, May 2026)
- VS Code MCP `inputs` format: https://code.visualstudio.com/docs/copilot/reference/mcp-configuration — `inputs` at JSON root; `${input:id}` in env/headers; VS Code stores value securely (Exa, May 2026)
- `pkg/validate/cache.go`: `validationCacheTTL = 24 * time.Hour`
- `pkg/app/plan_apply.go`: `AutoApprove bool` in `SavedPlanApplyOptions`; confirmed existing, tested field
- `pkg/provider/registry.go`: `DefaultRegistry().Get(id string) (MCPProvider, bool)` — public API
- `pkg/config/json_update.go`: `UpdateNamedServerJSON` writes `server["headers"]` at server scope; root-level `inputs` requires separate merge
