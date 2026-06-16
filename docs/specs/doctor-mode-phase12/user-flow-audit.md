# Doctor Mode Phase 12 UX Flow Audit

**Date:** 2026-05-23  
**Role:** UI/UX engineering audit  
**Scope:** Phase 12 dashboard conflict resolution and Playwright-style TUI flow tests  
**Primary spec:** `docs/specs/doctor-mode-phase12/spec.md`  
**Code scanned:** `pkg/tui/dashboard.go`, `pkg/tui/dashboard_view.go`, `pkg/tui/dashboard_flow_test.go`, `pkg/tui/dashboard_test.go`, `pkg/tui/dashboard_readiness.go`, `cmd/usync/main.go`, `cmd/usync/plan_commands.go`

## Core Product Value

`usync` should turn scattered MCP setup across many AI clients into one trusted local workflow:

1. Show the user what MCP clients and config files exist.
2. Explain what is ready, blocked, missing, or risky.
3. Let the user choose one provider and exact target configs.
4. Preview the changes before writing anything.
5. Apply with rollback, verification, and no credential leaks.

Phase 12 adds value only if it preserves that trust. Conflict resolution is not just another screen; it is the moment where `usync` tells the user, "I found multiple possible config files. Choose the one you mean, and I will use that exact file in the plan."

That promise is currently only partially true.

## Research Alignment

This report follows the earlier research in `docs/research/doctor-mode-batch-mcp-setup-research.md`, `docs/research/Doctor Mode, Batch Plan:Apply, and Credential Validation new spec.md`, and `docs/research/TUI-Testing-with-Go.md`.

Carry-forward decisions:

- Keep `usync` as a local-first setup and doctor tool; do not make MCP gateway mode the primary product.
- Preserve the researched workflow: `doctor -> validate -> target selection -> preview -> apply -> verify`.
- Start with system state detection, not credential entry.
- Enumerate config candidates instead of collapsing paths too early.
- Connect selected doctor candidates to planning so the selected path is the planned path.
- Treat migration as hints and explicit user choices, never silent file moves.
- Keep credential validation separate from planning/apply; live validation remains opt-in.
- Keep file apply and CLI-managed apply phases explicit because rollback guarantees differ.
- Use stable doctor output as the shared source for CLI, TUI, and any future thin MCP adapter.
- Use Playwright-style `teatest` flows for the main user journeys, with unit/golden tests for pure logic and views.

## UX Reference Principles

The audit uses these externally referenced CLI/TUI principles, found with Exa MCP:

- [Command Line Interface Guidelines](https://clig.dev/) emphasizes human-first design, visible state, useful errors, brief success output, and confirmation before risky actions.
- [Microsoft command-line design guidance](https://learn.microsoft.com/en-us/dotnet/standard/commandline/design-guidance) emphasizes predictable command structure, explicit interactivity, and stable conventions because CLIs become scripted workflows.
- [Nix CLI guidelines](https://nix.dev/manual/nix/2.30/development/cli-guideline) emphasize built-in help, clear feedback after each action, careful progress/status output, and designing the happy path first.
- [Charm Bubble Tea testing guidance](https://charmbracelet-bubbletea-43.mintlify.app/guides/testing) and [Charm teatest guidance](https://charm.land/blog/teatest/) support testing real key-driven flows with fixed terminal size, `WaitFor`, `Send`, and final model assertions.

Design translation for `usync`:

- The TUI should always show the current state and next safe action.
- A selected checkbox must mean "this will be planned."
- A resolved conflict must mean "this exact path will be planned."
- A workspace toggle must change the target list, not only the action bar text.
- Sad paths should tell the user what happened and where they still are.
- Tests should follow the same keystrokes a real user would press.

## Product Journey

The target user journey is:

```text
Doctor scan
  -> System/client/MCP inventory
  -> Provider readiness
  -> Credential guidance if needed
  -> Credential validation
  -> Target selection
  -> Conflict resolution if needed
  -> Saved plan preview
  -> Apply
  -> Verification and rescan
```

The user is not trying to learn `mcpServers`, `serverUrl`, TOML, or symlink details. They are trying to answer:

- Which clients did `usync` find?
- Which provider can I install now?
- Which files will be touched?
- Are secrets hidden?
- Can I trust the preview?
- Did the exact config I chose get applied?

## User Action Map

This table lists every Phase 12 dashboard action that matters to the product experience.

| ID | Screen | User action | User expectation | Current code path |
|---|---|---|---|---|
| U-01 | Launch | Open `usync` | Start with a scan, not a blank wizard | `NewDashboardModel`, `Init`, `scanCmd` |
| U-02 | Doctor | Scan succeeds | See system status, clients, warnings, providers already configured | `Update(scanResultMsg)`, `renderReport` |
| U-03 | Doctor | Scan fails | See a recoverable error and rescan option | `View` error branch, `handleKeyDoctor("r")` |
| U-04 | Doctor | `p` or `Enter` | Move into provider readiness | `handleKeyDoctor`, `readinessCmd` |
| U-05 | Doctor | `w` | Leave dashboard for legacy wizard | `RouteToWizard`, `cmd/usync/main.go` |
| U-06 | Provider readiness | `up/down/k/j` | Move among visible provider choices | `handleKeyProviderReady` |
| U-07 | Provider readiness | `Enter` | Validate selected provider offline and continue if safe | `offlineValidationCmd`, `Update(validationResultMsg)` |
| U-08 | Provider readiness | `v` | Run live validation without changing screens unexpectedly | `liveValidationCmd` |
| U-09 | Provider readiness | validation fails | Stay here, show the error, keep next action clear | `Update(validationResultMsg)`, `renderProviderReady` |
| U-10 | Provider readiness with conflicts | `r` | Go resolve target conflicts | `handleKeyProviderReady` |
| U-11 | Target select | `up/down/k/j` | Cursor reaches every selectable or resolvable row | `handleKeyTargetSelect`, `allTargetEntries` |
| U-12 | Target select on normal row | `Space` | Checkbox toggles and affects the plan | `handleKeyTargetSelect`, `buildAppSelection` |
| U-13 | Target select | `i` | Workspace/project targets appear or disappear | `handleKeyTargetSelect` |
| U-14 | Target select on normal row | `Enter` | Build a plan for selected targets only | `planCmd`, `PrepareProvider`, `BuildSavedPlan` |
| U-15 | Target select on conflict row | `r` or `Enter` | Open a choice screen for the conflicting paths | `handleKeyTargetSelect`, `screenConflictResolve` |
| U-16 | Conflict resolve | `1` or `2` | Choose that exact path, return to target select, preselect client | `handleKeyConflictResolve`, `resolvedConflicts` |
| U-17 | Conflict resolve | `s` | Skip that client for this session | `handleKeyConflictResolve` |
| U-18 | Conflict resolve | `Esc` | Cancel without changing state | `handleKeyConflictResolve` |
| U-19 | Plan preview | `n` or `Esc` | Return to target selection | `handleKeyPlanPreview` |
| U-20 | Plan preview | `y` or `Enter` | Apply exactly the previewed plan | `applyCmd`, `ApplySavedPlan` |
| U-21 | Apply result | render | Show result, verification, and rescan status | `Update(dashApplyResultMsg)`, `renderApplyResult` |
| U-22 | Any screen | `?` | See help without losing state | `handleKey` |
| U-23 | Any screen | `q` or `ctrl+c` | Quit safely | `handleKey` |

## UX State Model

The dashboard should make these states explicit:

| State | What the user should understand | Required UI signal |
|---|---|---|
| Scanning | `usync` is inspecting local clients | Progress text and no destructive action |
| Client inventory | Which supported clients and config candidates exist | Client rows with status, path, scope, and confidence |
| Existing MCP inventory | Which providers are already configured and whether entries look valid | Provider presence grouped by client, with malformed/legacy hints |
| Ready | A provider can be planned now | Provider row visible and selectable |
| Missing credential | Provider needs input before planning | Reason and key source hint |
| Missing-key journey | The user can still continue with no-key providers or return later | Provider grouping and credential links without dead-ending |
| Runtime missing | Provider needs local runtime such as Docker or npx | Runtime name and remediation |
| Conflict unresolved | One client has multiple config candidates | Conflict banner plus target-level resolution |
| Conflict resolved | A specific candidate path is chosen | Chosen path shown in target row or plan preview |
| Migration hint | A legacy/current path relationship exists | Suggestion only; no migration without explicit action |
| Target selected | The target will be planned | Checked row and plan includes it |
| Target deselected | The target will not be planned | Unchecked row and plan excludes it |
| Workspace included | Project/workspace configs are eligible | Workspace rows appear with scope/risk signal |
| Credentials missing | Required provider credentials are not available | Plan is blocked before target selection, with a route to setup or credential import |
| Plan ready | User can inspect exact changes | Preview with paths, manager, transport, warnings |
| Applying | Writes or CLI operations are running | Progress text |
| Applied | User can verify outcome | Updated/skipped/failed/verification sections |
| Output hygiene | The UI is intentionally redacted and quiet | No raw credentials, no debug stdout, clear stderr/stdout boundaries |

## Current Implementation Scan

### Dashboard Entry

`cmd/usync/main.go` launches the dashboard by default. It builds:

- `ProductionScanner` from home and workspace.
- `dashboardManagerAdapter` over `app.Manager` and `validate.Service`.
- Initial Exa credential profiles only when `--keys` or `--keys-file` is provided.

UX implication: the dashboard is now the front door, but it does not yet collect credentials. Providers with required credentials are only ready if credentials came from CLI flags.

Research implication: a first run should still be useful without keys. The dashboard should show inventory, no-key providers, credential links, and next actions instead of becoming a provider picker with missing credentials.

### Provider Readiness

`pkg/tui/dashboard_readiness.go` computes readiness. It currently marks every provider `conflict-blocked` when any doctor client has `ConfidenceConflict`.

UX implication: the user sees conflict as a global blocker, but Phase 12 resolution state is local to the dashboard and is not fed back into readiness. A resolved conflict can still look blocked when the user returns to provider readiness.

Research implication: readiness should be grouped as ready now, ready with supplied credentials, needs credentials, and blocked by runtime/conflict. It should not hide visible provider rows while the cursor can still move over hidden items.

### Target Selection

`pkg/tui/dashboard.go` uses:

- `allTargetEntries(report, resolvedConflicts)`
- `selectedClients map[manifest.ClientID]bool`
- `buildAppSelection() map[config.AppID]bool`

UX implication: the UI is client-level, while the actual user problem is file/candidate-level. This is the root cause of workspace and conflict path issues.

Research implication: the TUI should select concrete doctor candidates, then derive app config targets from those selections. This is the same direction as CLI `plan --all-detected` and future `doctor --json`.

### Conflict Resolution

`handleKeyConflictResolve` stores:

```go
ConflictResolution{
    ChosenPath: candidates[n].Path,
    ChosenLabel: candidates[n].Label,
}
```

UX implication: the TUI remembers what the user chose, but `planCmd` does not pass that choice into the app layer.

### Plan And Apply

`planCmd` calls:

```go
mgr.PrepareProvider(prov, profiles, selected, app.DefaultAssignments(...))
```

The selected object only contains app IDs. It does not contain the selected candidate path, candidate label, scope, or workspace flag.

UX implication: plan preview may not represent the exact path the user chose in target selection.

Research implication: apply must consume a saved, redacted plan built from explicit selections. Re-planning or falling back to static app detection breaks the researched Terraform-style plan/apply model.

### Missing Credential Profiles Dead End

When the dashboard starts without `--keys` or `--keys-file`, `profiles` is empty. The user can still reach target selection and press `Enter`. `app.Manager.PrepareProvider` then returns `at least one credential profile is required`, and `renderTargetSelect` shows it as a generic plan error.

Observed screen from the May 23, 2026 screenshot:

```text
Select Targets
====================

Plan error: at least one credential profile is required

[x] Claude Code
...

[up/down] navigate  [Space] toggle  [i] workspace(off)  [Enter] plan  [Esc] back  [q] quit
```

Important issue: the footer still advertises `[Enter] plan`, so the user can repeat the same failing action. There is no visible route to add credentials, open setup, or choose a no-key provider.

UX fix:

- Treat missing required credentials as a readiness/setup state, not a late plan error.
- Disable plan creation for credential-required providers until at least one profile exists.
- Show one recovery action: `[w] setup credentials` or `[Esc] provider readiness`, depending on the chosen product direction.

## Code-Accurate Flow Findings

These findings are based on the current code paths, not intended UX.

### Provider Readiness With Conflicts

Implemented behavior:

1. `renderProviderReady` shows a conflict banner and lists each client whose `doctor.ClientFinding.Confidence == ConfidenceConflict`.
2. The `[r]` key on provider readiness jumps directly to `screenTargetSelect`.
3. `actionBarProviderReady(true)` replaces the default footer with `[r] resolve conflicts`.
4. `renderProviderReady` hides rows whose state is `conflict-blocked` when the banner is visible.

Current screen shape when a conflict exists:

```text
Provider Readiness
====================

! Conflicts detected - resolve before planning:
  * Antigravity IDE

  Press [r] to go to conflict resolution.

[r] resolve conflicts  [up/down] navigate  [Enter] select provider  [Esc] back  [q] quit
```

Important issue: this screen does **not** show `[ready] Exa` or `[ready] Context7` rows during a conflict. `ComputeReadiness` marks every provider `conflict-blocked` whenever any client conflicts, then the renderer hides those rows. The cursor can still move over hidden readiness items.

UX fix:

- Keep provider credential/runtime readiness separate from unresolved target conflicts.
- Show ready providers even when target conflicts exist, while clearly saying planning is blocked until conflicts are resolved.
- Disable or reroute `Enter` while unresolved conflicts exist, instead of hiding all provider rows.

### Provider Readiness To Conflict Resolution

Implemented flow:

```text
Provider Readiness
  press r
Select Targets
  navigate to ? conflict client
  press r or Enter
Resolve Conflict
  press 1, 2, s, or Esc
```

This is a good direction: users do not need to validate before resolving path conflicts. The missing piece is that resolving the conflict must feed a concrete candidate path into planning.

### Target Select Checkboxes

Implemented behavior:

1. `Space` toggles `m.selectedClients[id] = !m.selectedClients[id]`.
2. `renderTargetSelect` shows `[x]` only when the value is true.
3. `buildAppSelection` ranges over map keys and ignores false values.

Important issue: a row can display `[ ]` while still being included in the plan.

UX fix:

- Treat `selectedClients` as a set: delete keys when unchecked.
- Make `buildAppSelection` include only true values.
- Use selected target count, not map length, to decide whether `Enter` may plan.

### Conflict Overlay

Implemented behavior:

1. Conflict rows render under `Conflict clients (press r to resolve):`.
2. On a conflict row, `r` or `Enter` opens `screenConflictResolve`.
3. `1` or `2` stores `ChosenPath` and `ChosenLabel`, selects the client, then returns to target select.
4. `s` stores `Skipped: true`.
5. `Esc` cancels without changing resolution state.

Important issue: `ChosenPath` is stored in TUI state only. `planCmd` passes only selected app IDs to `PrepareProvider`, so the chosen path does not reach planning.

UX fix:

- Plan from selected concrete doctor candidates, not only app IDs.
- The plan preview must show the exact chosen path.

## Path-By-Path Issue Trace

This is the source list for implementation fixes. Each item traces a user path to the code behavior and issue.

| ID | User path | Current code behavior | Issue | Fix / test |
|---|---|---|---|---|
| P-01 | Launch -> doctor scan succeeds | `scanResultMsg` stores report and renders `System Status` | OK baseline | Keep teatest happy path |
| P-02 | Launch -> doctor scan fails | `View` shows `Error scanning clients: ...`; `p` is blocked because `m.err != nil` | Error copy has no next-step guidance except footer rescan | Add recovery text; test scan-error view includes rescan action |
| P-03 | Doctor -> `p` with nil manager | Guard keeps user on doctor screen | Silent no-op; user cannot know why providers do nothing | Show disabled provider action or status message; test nil-manager path |
| P-04 | Doctor -> `w` | Sets `RouteToWizard` and quits dashboard | OK but legacy escape should remain secondary | Keep route test if wizard remains |
| P-05 | Provider readiness with no conflicts | Renders ready/no-key-needed first, then blocked/missing | OK baseline | Keep existing UX and footer |
| P-06 | Provider readiness with conflicts | `ComputeReadiness` changes all providers to `conflict-blocked`; renderer hides all conflict-blocked rows | Banner is visible, but provider rows disappear; cursor still moves over hidden rows | Separate provider readiness from target conflict state; test rows remain visible |
| P-07 | Provider readiness with conflicts -> `Enter` | Even though footer says resolve conflicts, `Enter` runs offline validation on hidden selected provider | User can bypass the "resolve before planning" guidance and land in target select | While unresolved conflicts exist, `Enter` should reroute to target select or show inline guidance; test `Enter` behavior |
| P-08 | Provider readiness with conflicts -> `v` | Live validation can run against hidden provider row | Hidden action causes surprise and may call network despite conflict-first UX | Disable/reroute live validation while conflicts unresolved; test no live call |
| P-09 | Provider readiness -> `r` when conflicts exist | Jumps to `screenTargetSelect` | Good direction | Keep and test full `r -> conflict row -> r` flow |
| P-10 | Provider readiness -> `r` when no conflicts | Key still jumps to target select, though footer does not advertise it | Hidden shortcut bypasses validation and provider selection | Gate `r` behind unresolved conflicts; test no-conflict `r` no-op |
| P-11 | Target select initial state | `selectedClients` built from high/medium installed clients | OK for user-scope clients | Extend to candidate-level targets |
| P-12 | Target select -> `Space` on checked row | Sets map value to false; UI shows `[ ]` | `buildAppSelection` ignores false values, so unchecked target is still planned | Delete key or check boolean; test deselect excludes plan |
| P-13 | Target select -> all rows unchecked -> `Enter` | Guard uses `len(m.selectedClients) > 0`, so false entries allow planning | Empty/no-op or unintended plan possible | Count selected eligible targets; show "Select at least one target" |
| P-14 | Target select -> `i` workspace toggle | Only flips `includeWorkspace`; target list unchanged | Cosmetic control; workspace/project targets are not added/removed | Build target entries from candidates and scope; test workspace rows |
| P-15 | Target select -> cursor down to conflict row | Unified `clientCursor` can reach conflict entries | OK baseline | Add teatest cursor-to-conflict flow |
| P-16 | Conflict row -> `r` or `Enter` | Copies matching `ClientFinding` into `resolveTarget` and opens overlay | OK baseline | Keep unit and teatest coverage |
| P-17 | Conflict overlay render | Shows label, path, symlink, parse, providers | Missing client ID, exists status, explicit deprecated/replaced-by status | Complete FR-10 decision evidence; update golden |
| P-18 | Conflict overlay with nil target | `renderConflictResolve` returns `renderShell`; `View` wraps again | Double shell wrapper | Return content only; add render test |
| P-19 | Conflict overlay -> `1` / `2` | Stores `ChosenPath`, selects client, returns to target select | Chosen path is not passed into planning | Plan from concrete selected candidates; test chosen path reaches fake manager |
| P-20 | Conflict overlay -> `s` | Stores skipped resolution, returns to target select | Skip is only respected by TUI entries; planning exclusion is not proven by flow tests | Add skip-then-plan teatest |
| P-21 | Conflict overlay -> `Esc` | Clears `resolveTarget`, leaves resolutions unchanged | OK baseline | Add teatest Esc cancel flow |
| P-22 | Resolved conflict -> target select | Resolved client moves to eligible section and is auto-selected | UI does not show chosen candidate/path, so user cannot verify choice before plan | Show chosen label/path in row; test rendered target row |
| P-23 | Resolved conflict -> `Esc` back to provider readiness | Provider readiness still computes from raw doctor conflict | Stale conflict banner/blocking after user resolved it | Use unresolved conflicts only; test banner clears |
| P-24 | Target select -> plan | `planCmd` passes app IDs only to `PrepareProvider` | Loses candidate path, workspace scope, and conflict resolution | Add concrete-target planning API or narrowed `AppConfig` |
| P-25 | Plan created -> preflight fails | Sets `planErr`, remains on target select | Error shown, but current flow tests assert only "not plan preview" | Assert exact screen and recovery action |
| P-26 | Plan preview -> `n` / `Esc` | Returns to target select | OK baseline | Add teatest for preview escape |
| P-27 | Plan preview -> `y` | Calls `ApplySavedPlan` with `AutoApprove: true` | Approval prompts displayed in preview but apply auto-approves from TUI | Decide if TUI confirmation is sufficient; test approval prompt flow |
| P-28 | Apply result success/error | Always starts rescan after result | User sees `Rescanning...`; resolved conflicts reapply only later on provider readiness | Apply resolutions after scan or fix comment; test rescan invariant |
| P-29 | Help `?` | Global help replaces current screen | OK if help includes current escape path | Ensure help names phase-specific actions; add view golden if needed |
| P-30 | Quit `q` / `ctrl+c` | Global quit | OK | Keep basic quit tests |
| P-31 | Target select -> `Enter` with zero credential profiles | `planCmd` calls `PrepareProvider`; app layer returns `at least one credential profile is required`; screen stays target select | Dead end: footer still says `[Enter] plan`, but repeating Enter cannot work and no credential setup action is offered | Block earlier in provider readiness or target select; route to setup/import; test zero-profile plan is not attempted |

## Critical UX Issues

### UX-01: Unchecking a target does not remove it from the plan

**Severity:** Critical  
**Flows:** U-12 -> U-14  
**Code:** `handleKeyTargetSelect`, `buildAppSelection`

The TUI shows `[ ]`, but `buildAppSelection` ranges over map keys and ignores the boolean value. A false entry is still included.

Why this breaks product trust:

- The visible checkbox is the user's contract with the plan.
- If unchecked still means planned, preview/apply cannot be trusted.

Sustainable fix:

- Treat `selectedClients` as a set.
- On deselect, delete the key.
- In `buildAppSelection`, include only `selected == true`.
- Use the computed selected app map for the `Enter` guard.

Tests:

- `TestBuildAppSelection_IgnoresFalseSelection`
- `TestDashboardFlow_DeselectOnlyTargetBlocksPlan`
- `TestDashboardFlow_DeselectOneOfTwoTargetsPlansOnlyCheckedTarget`

### UX-02: Conflict path choice is not honored by planning

**Severity:** Critical  
**Flows:** U-15 -> U-16 -> U-14  
**Code:** `handleKeyConflictResolve`, `planCmd`, `DashboardManager.PrepareProvider`

The user chooses candidate `1` or `2`, but the planning API receives only the app ID.

Why this breaks product trust:

- Conflict resolution is only useful if the exact chosen path reaches the plan.
- The preview can silently target a default or legacy file instead of the selected file.

Sustainable fix:

- Promote target selection from client-level to candidate-level.
- Add a shared target conversion layer that turns doctor candidates into `config.AppConfig` / `TargetFile`.
- Reuse it from both CLI `plan --all-detected` and TUI planning.
- Pass selected concrete targets into planning, either by a new dashboard manager method or by narrowing `Manager.Apps` before calling `PrepareProvider`.

Recommended shape:

```go
type DashboardTarget struct {
    ClientID       manifest.ClientID
    ClientName     string
    CandidateLabel string
    Path           string
    Scope          manifest.ScopeKind
    Kind           config.FileKind
    GitWarning     bool
    IsConflict     bool
}
```

Tests:

- `TestDashboardFlow_ConflictResolutionPlansChosenPath`
- `TestDashboardFlow_ConflictResolutionSecondCandidateUsesSecondPath`
- `TestDashboardFlow_ConflictSkipExcludesClientFromPlan`

### UX-03: Workspace toggle is cosmetic

**Severity:** High  
**Flows:** U-13 -> U-14  
**Code:** `handleKeyTargetSelect`, `defaultSelectedClients`, `allTargetEntries`, `config.DetectAppConfigs`

Pressing `i` changes `workspace(off/on)` in the footer, but it does not change entries or planning.

Why this breaks product value:

- Project/workspace config is risky because it may be shared in source control.
- The UI offers control but does not actually change the system behavior.

Sustainable fix:

- Target entries must represent concrete candidates, not only clients.
- Include project/workspace candidates only when `includeWorkspace` is true.
- Show scope in the row, for example `VS Code workspace`.
- Require approval prompt in plan preview for project/workspace targets.

Tests:

- `TestDashboardFlow_WorkspaceToggleAddsWorkspaceTarget`
- `TestDashboardFlow_WorkspaceOffExcludesWorkspaceTarget`
- `TestDashboardFlow_WorkspaceTargetShowsApprovalPrompt`

### UX-04: Resolved conflicts still look globally blocked

**Severity:** High  
**Flows:** U-10 -> U-16 -> U-19 or U-11 -> U-04  
**Code:** `ComputeReadiness`, `renderProviderReady`

Readiness looks only at `doctor.Report`, not `resolvedConflicts`.

Why this hurts UX:

- The user resolves the problem, then sees the same warning again.
- It makes the workflow feel circular and unreliable.

Sustainable fix:

- Compute unresolved conflicts:

```go
func unresolvedConflictClients(report doctor.Report, resolved map[manifest.ClientID]ConflictResolution) []doctor.ClientFinding
```

- Use unresolved conflicts in the banner and target section.
- Either recompute readiness with resolved conflict state or separate provider readiness from target readiness.

Tests:

- `TestDashboardFlow_ResolvedConflictClearsProviderBanner`
- `TestProviderReadiness_ResolvedConflictNoLongerBlocks`

### UX-05: Conflict overlay hides information required for a confident choice

**Severity:** Medium  
**Flows:** U-15  
**Code:** `renderConflictResolve`

The spec requires client ID, exists status, deprecated status, replacement label, symlink target, parse status, and providers. The current render shows only part of that.

Why this matters:

- The user is choosing between files. They need enough file evidence to decide.
- The safest default is not always obvious when legacy, symlink, and current paths coexist.

Sustainable fix:

- Render every FR-10 field.
- Make deprecated state explicit: `Deprecated: yes -> mcp-config` or `Deprecated: no`.
- Show `Exists: yes/no`.
- Show `Client: Antigravity IDE (antigravity)`.

Tests:

- Update `TestGoldenScreenConflictResolve`.
- Add `TestRenderConflictResolve_ShowsDecisionEvidence`.

### UX-06: Existing flow tests do not fully exercise the conflict product promise

**Severity:** High  
**Code:** `pkg/tui/dashboard_flow_test.go`

`TestDashboardFlow_ConflictInTargetSelect` stops after the conflict section appears. It does not:

- Move cursor to the conflict row.
- Press `r`.
- Choose `1` or `2`.
- Continue to plan preview.
- Assert selected path.

Why this matters:

- The most valuable Phase 12 flow is not covered by a user-like test.

Sustainable fix:

- Replace "conflict visible" as the main E2E with real conflict-resolution flows.
- Keep a smaller unit/golden test for visual rendering.

Tests:

- `TestDashboardFlow_ConflictChooseFirstThenPlan`
- `TestDashboardFlow_ConflictChooseSecondThenPlan`
- `TestDashboardFlow_ConflictSkipThenPlan`
- `TestDashboardFlow_ConflictEscCancel`

### UX-07: Sad path tests assert weaker states than the UI contract

**Severity:** Medium  
**Code:** `pkg/tui/dashboard_flow_test.go`

Current examples:

- Validation failure checks "not target select" instead of exactly provider readiness.
- Plan failure checks "not plan preview" instead of exactly target select.
- No-raw-credential test stops at target select despite claiming full happy path.

Why this matters:

- UX bugs often land in the "wrong but not forbidden" screen.
- A user flow test should assert the next stable screen and the visible recovery action.

Sustainable fix:

- Assert exact screen in `FinalModel`.
- Assert recovery actions are visible.
- Carry credential redaction checks through plan preview and apply result.

Tests:

- Update existing tests with exact screen assertions.
- `TestDashboardFlow_NoRawCredentialThroughPreviewAndApply`
- `TestDashboardFlow_EscFromPlanPreviewReturnsTargetSelect`

### UX-08: Empty plan is possible after deselection

**Severity:** Medium  
**Flows:** U-12 -> U-14  
**Code:** `handleKeyTargetSelect` guard uses `len(m.selectedClients) > 0`

Because false map entries still count, the user can create a no-op plan after turning every checkbox off.

Sustainable fix:

- Count selected eligible targets, not map length.
- If none are selected, show an inline message: `Select at least one target to continue.`
- Do not call `PrepareProvider`.

Tests:

- `TestDashboardFlow_NoSelectedTargetsShowsInlineError`
- `TestTargetSelect_NoSelectedTargetsDoesNotPrepare`

### UX-09: The nil conflict target fallback double-wraps shell UI

**Severity:** Low  
**Code:** `renderConflictResolve`

`renderConflictResolve` returns `renderShell(...)` when `resolveTarget == nil`, and `View` wraps the returned content again.

Sustainable fix:

- Render methods should return content only.
- Let `View` be the only shell wrapper.

Test:

- `TestRenderConflictResolve_NilTargetNotDoubleWrapped`

### UX-10: `conflictCursor` is unused

**Severity:** Low  
**Code:** `DashboardModel.conflictCursor`

The implementation chose the better unified `clientCursor` approach, but the unused field remains.

Sustainable fix:

- Remove `conflictCursor`.
- Update Phase 12 docs to say unified `targetEntry` cursor is the accepted design.

### UX-11: Missing credentials have an in-flow recovery path

**Severity:** Critical  
**Status:** Pass, closed by Phase 14 scoped PR 14e  
**Flows:** U-06 -> U-14, P-31, DM-P70..DM-P75, DM-P77  
**Code:** `NewDashboardModel`, `pkg/tui/credential_entry.go`, `pkg/tui/credential_entry_view.go`, `pkg/tui/footer_guidance.go`, `renderTargetSelect`

The dashboard can still enter target selection with no credential profiles, but the missing-credential state is no longer an unrecoverable plan error. Provider Readiness and Target Select both advertise `[k] add credentials` when a selected provider requires credentials, and submission stores a redacted in-memory profile for the current session.

Closed by Phase 14 PR 14e — see `docs/specs/doctor-mode-phase14/spec.md` §FR-9.

Locked behavior:

- Provider readiness shows credential-required providers as missing credentials and offers `[k] add credentials`.
- Target Select guards planning when the selected provider has no profile, avoids calling `PrepareProvider`, and offers `[k]`.
- The credential entry overlay validates required fields through the existing offline validation path, masks values in the view, supports paste, and recomputes readiness after submit.
- No-key providers remain plannable.

Tests:

- `TestDashboardFlowMatrix_CredentialDeadEndOffersRecovery`
- `TestCredentialEntry_KFromProviderReadyOpensOverlay`
- `TestCredentialEntry_EnterValidatesRequired`
- `TestCredentialEntry_SubmitAddsProfileAndMasksView`
- `TestCredentialEntry_EscRestoresPriorScreenUnchanged`
- `TestCredentialEntry_TabCyclesFields`
- `TestFooterGuidanceRows`

## UX Fix Strategy

### Step 1: Restore checkbox trust

Fix selection semantics first. This is the highest trust issue and the smallest change.

Acceptance:

- Checked rows are planned.
- Unchecked rows are not planned.
- No selected targets means no plan command starts.

### Step 2: Move from client rows to target rows

Represent rows as concrete file targets:

```text
[x] Antigravity CLI user      ~/.gemini/antigravity-cli/mcp_config.json
[ ] VS Code workspace         ./project/.vscode/mcp.json
?   Antigravity IDE conflict  2 candidate files
```

This improves:

- Conflict resolution.
- Workspace scope.
- Plan correctness.
- Preview confidence.

### Step 3: Share target discovery between CLI and TUI

Move target conversion logic out of `cmd/usync/plan_commands.go` into shared code. The CLI and TUI should not disagree about which path is effective.

Suggested package:

```text
pkg/discovery
  target.go
  summary.go
```

Responsibilities:

- Convert `doctor.ClientFinding` and `doctor.CandidateFinding` into `config.AppConfig`.
- Preserve candidate label, scope, path, git warning, symlink metadata.
- Summarize doctor report for saved plans.

### Step 4: Make conflict resolution update readiness

Use unresolved conflicts, not raw conflicts, for user-facing blockers. After a user resolves or skips a conflict, the interface should stop showing that conflict as blocking.

### Step 5: Complete decision evidence in the overlay

Conflict resolution should help the user choose, not just expose a menu. Render:

```text
Resolve Conflict: Antigravity IDE (antigravity)

[1] repo-current
    Path:       ~/.gemini/config/mcp_config.json
    Exists:     yes
    Deprecated: no
    Symlink:    no
    Parse:      ok
    Providers:  exa, context7

[2] alternate-symlink
    Path:       ~/.gemini/antigravity/mcp_config.json
    Exists:     yes
    Deprecated: yes -> mcp-config
    Symlink:    /private/...
    Parse:      ok
    Providers:  (none)
```

### Step 6: Add credential recovery before planning

Credential-required providers should not reach plan creation with zero profiles. The UI should either collect/import credentials or send the user back to provider readiness with a specific recovery action.

Acceptance:

- No call to `PrepareProvider` when required profiles are missing.
- The screen shows one clear credential recovery action.
- No-key providers remain usable.

### Step 7: Test the product promise with real keystrokes

Keep unit tests for pure logic. Use `teatest` for every cross-screen promise:

- User sees state.
- User presses key.
- UI changes to expected next state.
- Final model confirms exact state and selected data.

## Playwright-Style Test Plan

These tests should be the core Phase 12 UX protection suite.

| Test | Keystrokes | Product promise proved |
|---|---|---|
| `TestDashboardFlow_HappyPath` | `p`, `Enter`, `Enter`, `y`, `q` | A ready user can scan, validate, preview, apply, and see result |
| `TestDashboardFlow_ValidationFails` | `p`, `Enter`, `q` | Bad credentials keep user on provider readiness with a clear error |
| `TestDashboardFlow_PlanFails` | `p`, `Enter`, `Enter`, `q` | Plan errors keep user on target selection with recovery path |
| `TestDashboardFlow_ApplyFails` | `p`, `Enter`, `Enter`, `y`, `q` | Apply failures land on apply result with visible error |
| `TestDashboardFlow_DeselectOnlyTargetBlocksPlan` | `p`, `Enter`, `Space`, `Enter`, `q` | An unchecked only target cannot be planned |
| `TestDashboardFlow_DeselectOneOfTwoTargetsPlansOnlyCheckedTarget` | `p`, `Enter`, `Space`, cursor, `Enter`, `q` | Checkbox state maps exactly to selected plan targets |
| `TestDashboardFlow_ConflictChooseFirstThenPlan` | `p`, `r`, cursor to conflict, `r`, `1`, `Enter`, `q` | Choosing candidate 1 moves client to eligible and plans candidate 1 path |
| `TestDashboardFlow_ConflictChooseSecondThenPlan` | `p`, `r`, cursor to conflict, `r`, `2`, `Enter`, `q` | Choosing candidate 2 plans candidate 2 path |
| `TestDashboardFlow_ConflictSkipThenPlan` | `p`, `r`, cursor to conflict, `r`, `s`, `Enter`, `q` | Skipped conflict is not planned |
| `TestDashboardFlow_ConflictEscCancel` | `p`, `r`, cursor to conflict, `r`, `Esc`, `q` | Cancel preserves unresolved state |
| `TestDashboardFlow_WorkspaceToggleAddsTarget` | `p`, `Enter`, `i`, `Enter`, `q` | Workspace toggle changes target eligibility and plan scope |
| `TestDashboardFlow_MissingCredentialsBlocksBeforePlan` | no profiles, `p`, choose credential-required provider, `Enter` | Missing credentials show recovery action and do not call plan |
| `TestDashboardFlow_EscNavigation` | `p`, `Esc`, `p`, `Enter`, `Esc`, `Enter`, `Enter`, `Esc`, `q` | Back navigation is predictable across screens |
| `TestDashboardFlow_NoRawCredentialThroughPreviewAndApply` | happy path with UUID | No raw credential appears across the full visible flow |
| `TestDashboardFlow_ResolvedConflictClearsProviderBanner` | `p`, `r`, resolve conflict, `Esc` | Resolved conflicts stop looking blocked |

Test implementation guidelines:

- Use `teatest.NewTestModel` with fixed terminal size.
- Use condition waits, never `time.Sleep`.
- Prefer final model assertions for exact state.
- Record manager calls in the fake manager so tests can assert selected app IDs and selected file paths.
- Avoid asserting raw terminal output after it has been consumed; use final model `View()` or explicit captured checkpoints when testing redaction.

## Unit Test Plan

Add these pure or near-pure tests:

| Test | Purpose |
|---|---|
| `TestBuildAppSelection_IgnoresFalseSelection` | Prevent checkbox regression |
| `TestTargetToggleDeletesDeselectedKey` | Keep selected map as a set |
| `TestTargetSelect_NoSelectedTargetsDoesNotPrepare` | Avoid empty no-op plan |
| `TestAllTargetEntries_WorkspaceFiltering` | Make workspace toggle real |
| `TestConflictResolution_StoresCandidatePath` | Keep resolution data intact |
| `TestConflictResolution_PathReachesPlanningTarget` | Verify chosen path is consumed |
| `TestRenderConflictResolve_ShowsDecisionEvidence` | Ensure overlay supports confident choice |
| `TestRenderConflictResolve_NilTargetContentOnly` | Avoid double shell wrapping |
| `TestProviderReadiness_UsesUnresolvedConflicts` | Prevent stale blocked state |
| `TestTargetSelect_MissingCredentialsDoesNotPrepare` | Keep zero-profile plan attempts out of the app layer |
| `TestProviderReadiness_MissingCredentialsShowsSetupAction` | Make the credential recovery path visible before target selection |

## Updated Acceptance Status

| Acceptance criterion | Status | UX judgment |
|---|---|---|
| Happy path flow test exists | Partial | It passes, but should include redaction through apply |
| Validation failure flow | Partial | Visible error exists; exact screen assertion should be strengthened |
| Plan failure flow | Partial | Visible error exists; exact screen assertion should be strengthened |
| Apply failure flow | Pass | Good state and error assertion |
| No raw credential | Partial | Does not cover full preview/apply journey |
| Conflict row visible | Partial | Visible, but user-like navigation is under-tested |
| Conflict overlay opens | Partial | Unit tested, not tested through real teatest flow |
| Candidate choice resolves conflict | Partial | State is stored, but path is not consumed by planning |
| Skip excludes conflict | Partial | Unit tested, not proved through planning |
| Esc cancels conflict | Partial | Unit tested, not user-flow tested |
| Resolved client included in plan | Failing/Not proven | App ID can be included, but chosen path is lost |
| Workspace toggle | Failing | UI text changes only |
| Checkbox trust | Failing | Deselected targets can still be planned |
| Missing credentials | Failing | User can reach target select, press Enter, and get a repeatable dead-end plan error |

## Recommended Work Order

1. Fix missing-credential gating so credential-required providers cannot reach a dead-end plan error.
2. Fix checkbox semantics and no-selected-target handling.
3. Add failing tests for conflict chosen path reaching the planning layer.
4. Introduce candidate-level dashboard targets.
5. Share target discovery/conversion between CLI and TUI.
6. Make workspace toggle alter target entries and plan scope.
7. Make provider readiness depend on unresolved conflicts.
8. Complete conflict overlay decision evidence.
9. Replace the current conflict visibility teatest with full conflict resolution teatests.
10. Strengthen sad-path and redaction tests.
11. Add a compact existing-MCP inventory surface to the doctor/dashboard flow.
12. Keep credential guidance and no-key provider paths available when keys are missing.
13. Preserve output hygiene: no raw credentials, no library debug stdout, redacted errors.
14. Update `docs/specs/doctor-mode-phase12/tasks.md` to reflect completed versus remaining work.

## UX Definition Of Done

Phase 12 is complete when a user can:

1. See a conflict.
2. Understand why it matters.
3. Choose or skip a concrete path.
4. See that choice reflected in target selection.
5. See the same concrete path in the plan.
6. Apply only selected targets.
7. Return to provider readiness without stale conflict messaging.
8. Complete the whole flow without raw credentials appearing in the UI.
