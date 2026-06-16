# Doctor Mode Phase 13: Keymap × Screen Completeness and Batch Apply E2E

**Type:** Hardening / UX correctness spec
**Status:** Draft — awaiting approval
**Last updated:** 2026-05-23
**Builds on:** Phase 11 (test infrastructure, audit/skip-on-identical), Phase 12 (5-screen dashboard, conflict resolution, candidate-level targets)
**Anchor:** user-reported dead-end on Provider Readiness with no visible proceeding action.

---

## Problem Statement

Phase 11 and Phase 12 lock 13 matrix scenarios that exercise the highest-trust flows. They do not exhaustively cover the keymap × screen grid. The result is a class of UX defects where:

1. An advertised key on the action bar produces no visible response (silent no-op).
2. The cursor advances onto a hidden row, so the next action targets something the user cannot see.
3. State is lost across navigation (Esc, rescan) without a clear reason.
4. Batch apply with multiple targets — the core value proposition of `usync` — has only single-target automation.

The anchor incident: when any client is in `ConfidenceConflict`, `ComputeReadiness` (`pkg/tui/dashboard_readiness.go:36`) marks every provider `conflict-blocked`, and `renderProviderReady` (`pkg/tui/dashboard_view.go:172`) hides every conflict-blocked row. The user lands on Provider Readiness with only a banner and no provider list. The footer still advertises `[Enter] select provider` and `[v] live validate`. Both keys run against `m.readiness[m.providerCursor]` which points at a hidden row, producing either a silent advance or a confusing validation against an invisible provider.

---

## Goals

- Close the keymap × screen × precondition grid for every key wired into `handleKey*` in `pkg/tui/dashboard.go`.
- Make every advertised footer key produce visible feedback (response or rerouting) for the current screen state.
- Forbid hidden-row cursor states on Provider Readiness.
- Lock state-preservation contracts across Esc and rescan.
- Cover batch apply (multi-target, sequential, skip-on-identical) end-to-end.
- Document the help overlay contract (toggle, screen-aware text).

---

## Non-Goals

- Refactoring readiness to use unresolved conflicts (already filed as UX-04 / P-23 in Phase 12; remains in Phase 12 backlog).
- Persisting conflict resolution to disk across sessions (Phase 12 explicit non-goal).
- Responsive-layout audit (column width sweep) — separate "inconsistent layout" lens for Phase 14.
- Performance budget enforcement (keypress→render latency) — separate lens.
- Migration UX (Phase 9/10 cut, removed).

---

## Users / Actors

| Actor | Concern addressed |
|---|---|
| User stuck on Provider Readiness with conflict-blocked providers | Always a visible next action |
| User exploring the keymap | Every advertised key works; unmapped keys are no-ops without errors |
| User who pressed Esc and lost selections | Selections preserved across legitimate back-navigation |
| User running batch apply across many clients | One plan covers many targets; second apply surfaces "Unchanged" cleanly |
| Developer adding a key handler | Matrix forces a row before code lands |

---

## Functional Requirements

### FR-1 — No silent no-op on advertised keys

For every screen state, the action bar advertises only keys that produce visible feedback. The set of advertised keys is a function of state, not static. Specifically:
- ProviderReady with conflicts: action bar drops `[Enter] select provider` and `[v] live validate` when no rendered provider is selectable; `[r] resolve conflicts` remains the primary forward action.
- TargetSelect with `m.planning == true`: action bar replaces `[Enter] plan` with `Building plan…` indicator and disables other forward keys.
- PlanPreview with `m.applying == true`: action bar replaces `[y] apply` with `Applying…` indicator.
- ApplyResult: action bar advertises `[r] rescan` always; `[q] quit` always; never advertises `[y]` or `[Enter]`.

### FR-2 — Provider cursor cannot land on a hidden row

`m.providerCursor` always points at an index of `m.readiness` whose row will be rendered for the current state. Cursor movement (`↑`/`↓`/`k`/`j`) skips hidden indices.

Implementation note: introduce a helper `renderedProviderIndices(items, hasConflicts) []int` and use it both in the renderer and in the cursor handlers.

### FR-3 — Anchor fix: ProviderReady is never an empty dead-end

When the rendered provider list is empty (every provider is conflict-blocked and there are unresolved conflicts):
- The screen content explicitly says "No providers can be selected until the conflicts above are resolved."
- `Enter` and `v` are removed from the footer.
- `r` is the only forward key advertised; pressing `Enter` is treated as `r` (synonym), routing to conflict resolution.
- `Esc` returns to Doctor unchanged.

### FR-4 — Re-entrancy is visible

While a long-running operation is in flight (`m.scanning`, `m.validating`, `m.planning`, `m.applying`), the second press of the matching primary key (`r`, `Enter`, `Enter`, `y` respectively) is observable: either the action bar shows "(in progress)" or the screen shows an inline status. No second background command is started, and no error is generated.

### FR-5 — Esc state preservation

| Transition | Preserved | Cleared |
|---|---|---|
| ProviderReady → Esc → Doctor | report, scan result, `selectedClients`, `selectedTargets`, `resolvedConflicts`, `includeWorkspace`, `providerCursor`, `clientCursor` | `validErr`, `validating` |
| TargetSelect → Esc → ProviderReady | as above (no clears) | `planErr` |
| ConflictResolve → Esc → TargetSelect | `resolvedConflicts`, `selectedTargets`, `clientCursor` | `resolveTarget` |
| PlanPreview → Esc/n → TargetSelect | everything | `planning` flag, `planErr` |

### FR-6 — Rescan preserves user decisions

After `r` on ApplyResult triggers `scanCmd`, the model preserves:
- `resolvedConflicts` (conflicts the user already resolved this session)
- `includeWorkspace` (workspace toggle)
- `providerCursor` (mapped to nearest valid index after rescan)

The model rebuilds:
- `selectedTargets` (default selection for the new report, then re-applies `resolvedConflicts`)
- `report` (fresh scan output)

This matches the Phase 12 spec OQ: "resolved conflicts survive rescan."

### FR-7 — Help overlay is screen-aware

`?` toggles `m.showHelp`. The overlay (`renderHelpOverlay()`) renders the keymap for the **current screen** (passed as a parameter), not a static catalog. On the second `?` press the overlay closes and the prior screen renders identically (no state lost).

### FR-8 — Batch apply end-to-end

`usync`'s batch promise is asserted by automation:
- A single Apply Result lists every target written, separated by status (`Updated`, `Unchanged`, `Failed`).
- The "Unchanged" section is present and accurate when Phase 11 FR-2 skip-on-identical kicks in.
- A sequential session (apply provider A, rescan, apply provider B) is tested for state cleanliness.
- Plan preview rendering at width 80 with 5+ targets does not truncate path or scope information.

### FR-9 — Unmapped keys are no-ops with no error

Pressing keys outside the screen's vocabulary (e.g., `x`, `5`, `F1`) does not modify any model field, does not enqueue any command, and does not surface an error. A debug log entry MAY be written but no UI surface changes.

### FR-10 — Global quit works from every screen

`q` and `ctrl+c` quit cleanly from Doctor, ProviderReady, TargetSelect, ConflictResolve, PlanPreview, and ApplyResult, including while operations are in flight. No deferred goroutine leaks the process beyond `tea.Quit`.

---

## UX Requirements

**UX-1:** ProviderReady action bar with no selectable provider:

```text
[r] resolve conflicts  [Esc] back  [?] help  [q] quit
```

**UX-2:** ProviderReady action bar during validation:

```text
[Esc] back  [?] help  [q] quit
                       Validating…
```

**UX-3:** PlanPreview action bar during apply:

```text
[Esc] cancel  [?] help  [q] quit
                         Applying…
```

**UX-4:** Help overlay header includes screen name:

```text
Help — Provider Readiness
─────────────────────────
[↑↓]    move cursor
[Enter] select / validate provider
[v]     live validation
[r]     resolve conflicts (when present)
[Esc]   back to system status
[?]     close help
[q]     quit
```

**UX-5:** ApplyResult always advertises `[r] rescan` even on failure.

---

## Data Model Requirements

```go
// renderedProviderIndices returns indices of m.readiness whose rows will be rendered
// given the current state. Cursor movement and Enter/v key handlers MUST use this set.
func renderedProviderIndices(items []ProviderReadinessItem, hasConflicts bool) []int
```

No new screen, no new model field beyond what Phase 12 already declares. The fix is computed, not stored.

The help overlay gains a screen parameter:

```go
func renderHelpOverlay(screen dashboardScreen) string
```

---

## Testing Requirements

| Row | Layer | File |
|---|---|---|
| DM-P40 anchor (stuck on ProviderReady) | teatest matrix | `pkg/tui/dashboard_flow_matrix_test.go` |
| DM-P41 cursor cannot land on hidden row | unit | `pkg/tui/dashboard_test.go` |
| DM-P42–P44 double-press safety | teatest matrix | `pkg/tui/dashboard_flow_matrix_test.go` |
| DM-P45 action bar advertised keys are actionable | unit (table-driven) | `pkg/tui/dashboard_view_test.go` (new) |
| DM-P46–P48 Esc state preservation | teatest matrix | `pkg/tui/dashboard_flow_matrix_test.go` |
| DM-P49–P50 rescan persistence | teatest matrix | `pkg/tui/dashboard_flow_matrix_test.go` |
| DM-P51–P53 unmapped keys / quit-from-anywhere | unit (table-driven) | `pkg/tui/dashboard_test.go` |
| DM-P54–P55 help overlay toggle + screen-aware | unit + golden | `pkg/tui/dashboard_test.go` + `dashboard_golden_test.go` |
| DM-P56–P59 batch apply E2E | teatest matrix + docker fake-prod | `pkg/tui/dashboard_flow_matrix_test.go` + `dashboard_fake_prod_matrix_test.go` |
| DM-P60–P63 conflict overlay edge cases | unit + teatest | `pkg/tui/dashboard_test.go` + matrix |
| DM-P64–P69 misc | mixed | per matrix row pointers |

---

## Acceptance Criteria

| # | Criterion |
|---|---|
| AC-1 | `go test ./pkg/tui` passes including all DM-P40..P69 rows that are `Locked`. |
| AC-2 | `make ux-fake-prod` runs clean and `artifacts/ux-fake-prod/issues.json` strictly decreases or is empty across the iteration sequence. |
| AC-3 | A manual walk-through with `report.Clients` containing any conflict client always produces a visible forward action on Provider Readiness. |
| AC-4 | No advertised footer key produces a silent no-op in `TestActionBar_AdvertisedKeysAreActionable`. |
| AC-5 | Phase 11/12 tests continue to pass unchanged. |

---

## Open Questions

| OQ | Status |
|---|---|
| Should `Enter` on ProviderReady with no selectable rows be a synonym for `r`, or a hard no-op with status copy? | Resolved — synonym for `r`. Lower keystroke cost for the dead-end. |
| Should `Esc` on ApplyResult route back to Doctor, or stay (no-op)? | Resolved — `Esc` is a no-op on ApplyResult; the only forward keys are `r` (rescan) and `q` (quit). Documented via DM-P65. |
| Should the help overlay render the full keymap of every screen? | Resolved — screen-aware only; static catalogs become stale. |
| Should `?` toggle inside the conflict-resolve overlay? | Resolved — yes; help is screen-aware so it shows ConflictResolve's keys when active. |

---

## Approval Status

Pending approval on:
1. Anchor fix shape (synonym `Enter==r` when no selectable provider).
2. Esc state-preservation matrix.
3. Help overlay signature change (screen parameter).
