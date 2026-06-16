# Doctor Mode Phase 12: Playwright-Style TUI E2E Tests and Conflict Resolution

**Type:** Feature + Test Hardening  
**Status:** Draft  
**Last updated:** 2026-05-23  
**Builds on:** Phase 8 (5-screen dashboard), Phase 11 (TUI test infrastructure)

---

## Problem Statement

Two gaps remain after Phase 11:

**Gap 1 — No Playwright-style TUI flow tests.**  
The existing teatest (`TestDashboardTeatest`) only covers the Phase 7 doctor screen. The full Phase 8 flow — pressing `p` to enter provider readiness, navigating with keys, advancing through validation → target selection → plan preview → apply → result — has no teatest coverage. Without this, any screen transition regression is only caught by unit tests that directly inject typed messages, not by realistic user-input flows.

**Gap 2 — Conflict clients are a dead end.**  
In `screenTargetSelect`, clients with `Confidence == ConfidenceConflict` appear with a `!` label and "conflict — resolve before planning". There is no action available. Users who have, for example, both `~/.gemini/config/mcp_config.json` and `~/.gemini/antigravity/mcp_config.json` present (the known Antigravity IDE path dispute) cannot proceed: they cannot skip the conflicted client, they cannot select it, they cannot choose which path to use. The entire flow is blocked.

---

## Goals

**E2E TUI tests (Playwright-style):**
- Walk all 5 dashboard screens via actual key inputs using `teatest.NewTestModel`.
- Assert visible text at each screen transition using `waitForText`/`waitForAll`.
- Assert final model state with `tm.FinalModel(t)` after quit.
- Cover the happy path (full doctor → validate → target select → plan → apply → result).
- Cover key sad paths: validation failure (stay on provider ready), plan error, apply error.
- Cover navigation: `Esc` returns to previous screen at each level.
- No `time.Sleep` anywhere; all waits are condition-based.

**Conflict resolution in `screenTargetSelect`:**
- Conflict clients are navigable — cursor can land on them.
- Pressing `r` (or `Enter`) on a conflict client opens a conflict resolution overlay (`screenConflictResolve`).
- The overlay shows both conflicting candidates with: path, exists status, deprecated flag, configured providers, symlink target (if any).
- User picks one candidate (`1`, `2`) or skips the client (`s`).
- Choosing a candidate marks the client as **resolved**: it moves from the `!` list into the eligible/selectable list with the chosen path recorded.
- Skipping keeps the client excluded from planning for this session.
- Resolved choices feed into plan creation: the chosen candidate path is used.
- `Esc` cancels without changing resolution state.

**Candidate-level target selection (PR 12c):**
- Target selection renders concrete config-file candidates, not only app/client names.
- Workspace/project candidates are hidden while workspace mode is off.
- Pressing `i` toggles workspace/project candidates into or out of the target list.
- Multi-file clients can select or deselect individual candidate files.
- Plan preview names the selected path, scope, and git warning for project/workspace files.

---

## Non-Goals

- Writing or deleting config files during conflict resolution (resolution is a path choice, not a file mutation).
- Migration between Gemini and Antigravity (removed in Phase 9/10 cut).
- Resolving runtime-missing or missing-credentials states from the conflict screen.
- Persisting conflict resolutions across sessions.
- Multi-step conflict wizards (this is intentionally a single-screen overlay).
- `strider` or tmux-based black-box terminal testing (optional, deferred).

---

## Users

| User | Benefit |
|---|---|
| Developer adding new dashboard screens | Teatest flow tests catch regressions immediately |
| User with Antigravity IDE path dispute | Can choose which path to use and continue to plan |
| User with any two-candidate conflict client | Unblocked: can skip or resolve and proceed |

---

## Functional Requirements

### E2E Teatest (FR-1 – FR-7)

**FR-1 — Happy path flow test.**  
`TestDashboardFlow_HappyPath` uses `teatest.NewTestModel` with `FakeDashboardManager` and `FakeScanner`. It walks: doctor screen → provider ready → offline validation → target select → plan creation → plan preview → apply → apply result. At each screen it waits for stable visible text using `waitForText`. After the program quits, `tm.FinalModel(t)` asserts `m.screen == screenApplyResult` and `m.applyResult != nil`.

**FR-2 — Validation failure sad path.**  
`TestDashboardFlow_ValidationFails` sends `p` to enter provider ready, then sends `Enter` to trigger offline validation. `FakeDashboardManager.ValidErr` is set to a non-nil error. The test asserts the screen stays on `screenProviderReady` and the error text is visible.

**FR-3 — Plan creation failure sad path.**  
`TestDashboardFlow_PlanFails` walks to target select, presses `Enter`. `FakeDashboardManager.PlanErr` is set. The test asserts the screen stays on `screenTargetSelect` with an error message visible.

**FR-4 — Apply failure sad path.**  
`TestDashboardFlow_ApplyFails` walks to plan preview, presses `y`. `FakeDashboardManager.ApplyErr` is set. The test asserts `screenApplyResult` is shown with an error visible. It also asserts `screen == screenApplyResult` via `FinalModel`.

**FR-5 — Esc navigation.**  
`TestDashboardFlow_EscNavigation` sends `p` (→ provider ready), `Esc` (→ doctor), `p` again (→ provider ready), `Enter` (→ target select), `Esc` (→ provider ready), `Esc` (→ doctor). Asserts screen text at each step.

**FR-6 — No raw credential in any captured output.**  
All flow tests use a profile with a fake UUID key. `tm.Output()` is never checked to contain the raw key — each output assertion uses `waitForText` for stable non-credential text only. Additionally, a final scan of the captured output asserts the UUID is absent.

**FR-7 — `FinalModel` state assertions.**  
After `tm.WaitFinished`, `tm.FinalModel(t)` is called and the returned model is type-asserted to `DashboardModel`. At minimum: `m.scanning` is false after apply result.

---

### Conflict Resolution (FR-8 – FR-15)

**FR-8 — Conflict clients are cursor-navigable.**  
In `screenTargetSelect`, the cursor (controlled by `↑`/`↓`) can land on conflict clients. Conflict clients appear in their own section below eligible clients, with `?` prefix and their name. The cursor wraps across both sections.

**FR-9 — `r` or `Enter` on a conflict client opens the overlay.**  
When `clientCursor` points at a conflict client and the user presses `r` or `Enter`, the screen transitions to `screenConflictResolve`. The conflicting client's `ClientFinding` is stored in `resolveTarget ClientFinding`. No change to other model state occurs.

**FR-10 — `screenConflictResolve` displays both candidates.**  
The overlay renders:
- Client name and ID.
- For each candidate (up to the first 2 that exist or are symlinks):
  - Label and full path.
  - Exists on disk: yes/no.
  - Deprecated: yes (with replacement label) or no.
  - Symlink: resolved target path if `IsSymlink`.
  - Parse status: `ok` or `error: <message>`.
  - Configured providers: comma-separated list or `(none)`.
- Actions: `[1] use this`, `[2] use this`, `[s] skip client`, `[Esc] cancel`.

**FR-11 — Choosing a candidate (`1` or `2`) resolves the conflict.**  
When the user presses `1` or `2`, the corresponding `CandidateFinding.Path` is stored as `resolvedConflicts[clientID] = ConflictResolution{ChosenPath, ChosenLabel}`. The model returns to `screenTargetSelect`. The client is added to `selectedClients` automatically (pre-selected). It moves from the conflict section to the eligible section.

**FR-12 — Skipping (`s`) excludes the client.**  
When the user presses `s`, `resolvedConflicts[clientID] = ConflictResolution{SkippedBy: true}` is recorded. The client is not added to `selectedClients`. It disappears from the conflict section and does not appear in the eligible section (skipped for this session). The model returns to `screenTargetSelect`.

**FR-13 — `Esc` cancels without changing state.**  
Returns to `screenTargetSelect` with no change to `resolvedConflicts` or `selectedClients`.

**FR-14 — Resolved conflicts feed into plan creation.**  
`buildAppSelection()` includes resolved (non-skipped) conflict clients. The chosen path is stored in `resolvedConflicts`; when `planCmd()` runs `PrepareProvider`, the resolved client is included by its `config.AppID`. The plan targets the chosen candidate's path (Phase 12 passes the chosen path to the manager via a new `ConflictResolutions map[manifest.ClientID]string` field on `DashboardManager.PrepareProvider` — or via a simpler approach: the selected clients map already encodes the choice). See Data Model Requirements.

**FR-15 — `Esc` from `screenConflictResolve` works even if only one candidate exists.**  
If the conflict client has fewer than 2 existing candidates, the overlay shows only what's available and disables inapplicable number keys. The screen still renders and `Esc` / `s` always work.

### Candidate-Level Target Selection (FR-16 - FR-20)

**FR-16 — Target rows represent concrete candidates.**  
Each selectable row maps to one concrete target file: client ID, candidate label, path, scope, file kind, creatable state, and git-warning flag.

**FR-17 — Workspace toggle changes target eligibility.**  
With workspace mode off, project/workspace candidates are absent. With workspace mode on, they appear as selectable rows.

**FR-18 — Multi-file clients are selectable per file.**  
Checking a client row never implicitly selects every config file for that client. The plan only includes checked target rows.

**FR-19 — Conflict resolution keeps chosen row identity.**  
After choosing a conflict candidate, target select shows the chosen label/path so the user can confirm what will be planned.

**FR-20 — Project/workspace risk is visible before apply.**  
Plan preview shows scope and git-warning risk for selected project/workspace targets.

---

## UX Requirements

**UX-1:** In `screenTargetSelect`, the conflict section heading reads: `Conflict clients (press r to resolve):`. Eligible clients appear above it; conflict clients below.

**UX-2:** The cursor indicator (`>`) navigates continuously across both sections. The cursor position in the conflict section uses a separate index (`conflictCursor int`) relative to the conflict list.

**UX-3:** In `screenConflictResolve`, each candidate is displayed as a numbered block:
```
[1] mcp-config  (canonical)
    Path:       ~/.gemini/antigravity-cli/mcp_config.json
    Exists:     yes
    Providers:  exa, context7
    Parse:      ok

[2] repo-current  (deprecated → mcp-config)
    Path:       ~/.gemini/config/mcp_config.json
    Exists:     yes
    Symlink:    → /private/var/.../resolved.json
    Providers:  (none)
    Parse:      ok
```

**UX-4:** After resolution, the formerly-conflict client appears in the eligible section with a `[✓]` or `[x]` checkbox (selected by default). The `?` entry in the conflict section is removed.

**UX-5:** In `screenTargetSelect`, the action bar gains `[r] resolve conflict` when the cursor is on a conflict client, replacing the default bar.

---

## Data Model Requirements

### New screen constant

```go
screenConflictResolve dashboardScreen = 5
```

### New types (pkg/tui/dashboard.go)

```go
type ConflictResolution struct {
    ChosenPath  string            // empty string if skipped
    ChosenLabel string            // candidate label chosen
    Skipped     bool
}
```

### New `DashboardModel` fields

```go
// Conflict resolution state
conflictCursor    int                                       // index into skippedClients(report)
resolveTarget     *doctor.ClientFinding                    // client being resolved
resolvedConflicts map[manifest.ClientID]ConflictResolution // session-scoped choices
```

### Updated `eligibleClientIDs` and `buildAppSelection`

`eligibleClientIDs` includes resolved (non-skipped) conflict clients after resolution.  
`buildAppSelection` maps `manifest.ClientID → config.AppID` for resolved clients; the chosen path is the one the manager will find in the app config (no new API change to `PrepareProvider` needed — the manager already has access to all files; resolved path is used for display and for setting `selectedClients`).

### Updated `defaultSelectedClients`

When called after conflict resolution, resolved clients are pre-selected. Session conflict state survives re-scans only if the same conflict is detected again (it will be); resolved state is re-applied after each rescan.

---

## Testing Requirements

| Requirement | Layer | File |
|---|---|---|
| Happy path full flow (5 screens) | teatest | `pkg/tui/dashboard_flow_test.go` |
| Validation failure sad path | teatest | `pkg/tui/dashboard_flow_test.go` |
| Plan error sad path | teatest | `pkg/tui/dashboard_flow_test.go` |
| Apply error sad path | teatest | `pkg/tui/dashboard_flow_test.go` |
| Esc navigation through all screens | teatest | `pkg/tui/dashboard_flow_test.go` |
| No raw credential in any flow output | teatest | `pkg/tui/dashboard_flow_test.go` |
| `FinalModel` state assertions | teatest | `pkg/tui/dashboard_flow_test.go` |
| Conflict client is cursor-navigable | unit | `pkg/tui/dashboard_test.go` |
| `r`/`Enter` on conflict opens resolve screen | unit | `pkg/tui/dashboard_test.go` |
| Choosing `1` resolves conflict, moves to eligible | unit | `pkg/tui/dashboard_test.go` |
| Choosing `2` resolves conflict (second candidate) | unit | `pkg/tui/dashboard_test.go` |
| `s` skips conflict, not added to selected | unit | `pkg/tui/dashboard_test.go` |
| `Esc` from conflict resolve cancels | unit | `pkg/tui/dashboard_test.go` |
| `renderConflictResolve` shows both candidates | unit | `pkg/tui/dashboard_test.go` |
| Resolved client appears in eligibleClientIDs | unit | `pkg/tui/dashboard_test.go` |
| Skipped client absent from eligibleClientIDs | unit | `pkg/tui/dashboard_test.go` |
| Golden: `screenConflictResolve` view | golden | `pkg/tui/dashboard_golden_test.go` |
| Conflict resolution flow in teatest | teatest | `pkg/tui/dashboard_flow_test.go` |
| Workspace toggle changes target rows | matrix teatest | `pkg/tui/dashboard_flow_matrix_test.go` |
| Workspace off excludes project targets | matrix teatest | `pkg/tui/dashboard_flow_matrix_test.go` |
| Project/workspace risk visible in preview | matrix teatest | `pkg/tui/dashboard_flow_matrix_test.go` |
| Multi-file client per-candidate selection | matrix teatest | `pkg/tui/dashboard_flow_matrix_test.go` |
| Resolved conflict row shows chosen identity | matrix teatest | `pkg/tui/dashboard_flow_matrix_test.go` |

---

## Acceptance Criteria

| # | Criterion |
|---|---|
| AC-1 | `TestDashboardFlow_HappyPath` walks all 5 screens via key inputs and passes. |
| AC-2 | `tm.FinalModel(t)` after happy path: `m.screen == screenApplyResult`, `m.applyResult != nil`. |
| AC-3 | Validation failure test: screen stays `screenProviderReady`; error text visible. |
| AC-4 | Plan error test: screen stays `screenTargetSelect`; error text visible. |
| AC-5 | Apply error test: `screenApplyResult` shown; error text visible. |
| AC-6 | No raw UUID appears in any `tm.Output()` captured during flow tests. |
| AC-7 | Conflict client appears in `screenTargetSelect` with `?` prefix and cursor can reach it. |
| AC-8 | `r` on conflict client enters `screenConflictResolve`; both candidates rendered. |
| AC-9 | `1` on conflict resolve screen moves client to eligible section, pre-selected. |
| AC-10 | `s` skips client; it disappears from conflict section and is not in eligible. |
| AC-11 | `Esc` from `screenConflictResolve` returns to `screenTargetSelect` unchanged. |
| AC-12 | Resolved conflict client is included in plan via `buildAppSelection`. |
| AC-13 | `go test ./pkg/tui` passes. |
| AC-14 | All Phase 7, 8, 11 tests continue to pass unchanged. |
| AC-15 | `DM-P14`, `DM-P32`, `DM-P33`, `DM-P34`, and `DM-P35` pass in `make ux-fake-prod`. |
| AC-16 | Workspace/project targets are not planned unless visible and checked. |
| AC-17 | A selected target row produces exactly one planned file operation unless the row explicitly represents a CLI-managed target. |

---

## Open Questions

| OQ | Resolution |
|---|---|
| Should resolved conflicts survive a rescan? | Yes — `resolvedConflicts` persists in model state; after rescan, `defaultSelectedClients` re-applies it. |
| What if a conflict has 3+ candidates? | Overlay shows first 2 that `Exist == true` or are symlinks; remainder ignored for Phase 12. |
| Should "use this" skip validation of the chosen path? | No — the doctor already scanned it; parse status is shown in the overlay. |
| Does conflict resolution affect `UseInputVariables` for VS Code? | No — it only determines which path is targeted; secret indirection is controlled by `SavedPlanOptions`. |
