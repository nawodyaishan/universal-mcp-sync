# Doctor Mode Phase 13 — Tasks

**Spec:** `docs/specs/doctor-mode-phase13/spec.md`
**Plan:** `docs/specs/doctor-mode-phase13/plan.md`
**Matrix:** `docs/specs/doctor-mode-phase13/ux-flow-matrix.md`
**Verification:** `NO_COLOR=1 TERM=xterm-256color go test ./pkg/tui` and `make ux-fake-prod`.

---

## PR 13a — Stuck-on-ProviderReady Anchor Fix

### Task 1 — `RenderedProviderIndices` helper
**File:** `pkg/tui/dashboard_readiness.go`
**Status:** TODO
- [ ] Add pure function `RenderedProviderIndices(items []ProviderReadinessItem, hasConflicts bool) []int`.
- [ ] Unit test `TestRenderedProviderIndices_FiltersConflictBlocked` covering both conflict + no-conflict cases.

### Task 2 — Action bar takes `hasSelectable`
**File:** `pkg/tui/dashboard_view.go`
**Status:** TODO
- [ ] Change signature to `actionBarProviderReady(hasConflicts, hasSelectable bool) string`.
- [ ] Drop `[Enter] select provider` and `[v] live validate` when `!hasSelectable`.
- [ ] Add explanatory line "No providers can be selected until the conflicts above are resolved." when `!hasSelectable && hasConflicts`.
- [ ] Re-record `pkg/tui/testdata/TestGoldenScreenProviderReady.golden`.

### Task 3 — Cursor + Enter + `v` route through `RenderedProviderIndices`
**File:** `pkg/tui/dashboard.go`
**Status:** TODO
- [ ] In `handleKeyProviderReady`, compute `rendered := RenderedProviderIndices(m.readiness, hasConflictClient(m.report))`.
- [ ] `up`/`k`/`down`/`j` move the cursor only across `rendered` indices.
- [ ] When `len(rendered) == 0`, treat `Enter` as a synonym for `r` (route to conflict resolve).
- [ ] When `len(rendered) == 0`, `v` is a no-op.
- [ ] Clamp `m.providerCursor` to the nearest rendered index after a state change.

### Task 4 — DM-P40 anchor test
**File:** `pkg/tui/dashboard_flow_matrix_test.go`
**Status:** TODO
- [ ] `TestDashboardFlowMatrix_StuckProviderReadyConflictHidesAll` walks `p`, then asserts the footer does NOT advertise `[Enter] select provider` and pressing `enter` routes to TargetSelect, with `mgr.ValidateCalls == 0`.

### Task 5 — DM-P41 cursor cannot land on hidden row
**File:** `pkg/tui/dashboard_test.go`
**Status:** TODO
- [ ] Unit test sets up readiness with 3 items where indices 0 and 2 are hidden. Pressing `down` from cursor at index 1 keeps cursor at index 1; pressing `up` keeps it. After re-state with all visible, cursor moves normally.

### Task 6 — DM-P67 / DM-P68 hidden-row safety
**File:** `pkg/tui/dashboard_flow_matrix_test.go`
**Status:** TODO
- [ ] `TestDashboardFlowMatrix_EnterOnHiddenProviderRowSafe` — Enter from the empty-rendered state never calls `mgr.Validate`.
- [ ] `TestDashboardFlowMatrix_LiveValidateBlockedDuringConflict` — `v` from the empty-rendered state never calls `mgr.Validate(live=true)`.

### Task 7 — DM-P45 action-bar advertised-keys table test
**File:** `pkg/tui/dashboard_view_test.go` (new)
**Status:** TODO
- [ ] Table over `(screen, state)` → footer string; for each row, parse the `[key]` tokens and assert that pressing each produces an observable change (either state mutation OR command emission OR explicit ignored-with-feedback).

### Task 8 — Matrix lock
**File:** `docs/specs/doctor-mode-phase13/ux-flow-matrix.md`
**Status:** TODO
- [ ] Flip DM-P40, DM-P41, DM-P45, DM-P67, DM-P68 from `Proposed` to `Locked`.

### PR 13a Completion Checklist
- [ ] `NO_COLOR=1 TERM=xterm-256color go test ./pkg/tui` passes.
- [ ] `make ux-fake-prod` produces empty `artifacts/ux-fake-prod/issues.json`.
- [ ] Phase 11 and Phase 12 tests unchanged and passing.
- [ ] Goldens re-recorded and committed.
- [ ] Single commit per the bug-hunt protocol (code + matrix + golden together).

---

## PR 13b — Re-entrancy + Esc Preservation

### Task 9 — Visible feedback during in-flight ops
**Files:** `pkg/tui/dashboard.go`, `pkg/tui/dashboard_view.go`
**Status:** TODO
- [ ] Action bars show `Validating…` / `Building plan…` / `Applying…` / `Rescanning…` overlay text while the relevant flag is true.
- [ ] Forward keys (`Enter`, `y`, `r`) advertised in the footer are dropped while the flag is true.

### Task 10 — DM-P42/P43/P44 double-press tests
**File:** `pkg/tui/dashboard_flow_matrix_test.go`
**Status:** TODO
- [ ] Three teatest cases that send the relevant key twice and assert call counts.

### Task 11 — Esc preservation
**File:** `pkg/tui/dashboard.go`
**Status:** TODO
- [ ] Confirm `handleKeyProviderReady("esc")`, `handleKeyTargetSelect("esc")`, `handleKeyConflictResolve("esc")`, `handleKeyPlanPreview("esc"/"n")` do not clear preserved state per spec FR-5 matrix.
- [ ] Add `validErr=nil` + `validating=false` on the Doctor-bound Esc.
- [ ] Add `planErr=nil` on the ProviderReady-bound Esc.

### Task 12 — DM-P46/P47/P48 Esc tests
**File:** `pkg/tui/dashboard_flow_matrix_test.go`
**Status:** TODO
- [ ] Three teatest cases asserting the preserved fields per spec FR-5.

---

## PR 13c — Rescan Persistence + Help Overlay

### Task 13 — Rescan persistence
**File:** `pkg/tui/dashboard.go`
**Status:** TODO
- [ ] In the rescan path (`r` on ApplyResult → `scanCmd` → `scanResultMsg`), preserve `resolvedConflicts`, `includeWorkspace`, clamp `providerCursor`/`clientCursor`, rebuild `selectedTargets` from defaults then re-apply resolutions.

### Task 14 — Help overlay screen-aware
**Files:** `pkg/tui/dashboard.go`, `pkg/tui/dashboard_view.go`
**Status:** TODO
- [ ] Change signature to `renderHelpOverlay(screen dashboardScreen) string`.
- [ ] Render keymap for the active screen.

### Task 15 — DM-P49/P50/P54/P55 tests
**Files:** `pkg/tui/dashboard_flow_matrix_test.go`, `pkg/tui/dashboard_test.go`, `pkg/tui/dashboard_golden_test.go`
**Status:** TODO

---

## PR 13d — Unmapped Keys + Quit From Anywhere

### Task 16 — DM-P51/P52/P53/P62 table tests
**File:** `pkg/tui/dashboard_test.go`
**Status:** TODO
- [ ] Table over screens × unmapped keys → assert no model change.
- [ ] Table over screens × `q` → `tea.Quit`.
- [ ] Table over screens × `ctrl+c` → `tea.Quit`.
- [ ] Table over ConflictResolve × `0`,`3`,`x`,`F1` → no model change.

---

## PR 13e — Batch Apply E2E

### Task 17 — DM-P56 three-target batch
**File:** `pkg/tui/dashboard_flow_matrix_test.go`
**Status:** TODO
- [ ] `happyFlowSetup` extended (or new helper) to seed 3 eligible clients. teatest walks `p`, `Enter`, `Enter`, `y`; asserts plan contains all three and `mgr.ApplyCalls == 1`.

### Task 18 — DM-P57 skip-on-identical
**File:** `pkg/tui/dashboard_fake_prod_matrix_test.go`
**Status:** TODO
- [ ] Docker fake-prod scenario: apply twice; second apply result shows `Unchanged (N)`.

### Task 19 — DM-P58 sequential providers
**File:** `pkg/tui/dashboard_flow_matrix_test.go`
**Status:** TODO

### Task 20 — DM-P59 plan preview at 5+ targets
**Files:** `pkg/tui/dashboard_golden_test.go`, `pkg/tui/testdata/`
**Status:** TODO

---

## PR 13f — ConflictResolve Edges + Apply Error Recovery

### Task 21 — Single-candidate conflict (DM-P60)
**File:** `pkg/tui/dashboard_test.go`
**Status:** TODO

### Task 22 — Three-candidate conflict shows two (DM-P61)
**File:** `pkg/tui/dashboard_test.go`
**Status:** TODO

### Task 23 — Sequential conflicts in one session (DM-P63)
**File:** `pkg/tui/dashboard_flow_matrix_test.go`
**Status:** TODO

### Task 24 — Apply error offers recovery (DM-P64)
**File:** `pkg/tui/dashboard_view.go` + `pkg/tui/dashboard_flow_matrix_test.go`
**Status:** TODO

### Task 25 — Esc on ApplyResult documented (DM-P65)
**File:** `pkg/tui/dashboard_flow_matrix_test.go`
**Status:** TODO

### Task 26 — Wizard route on scan error (DM-P66)
**File:** `pkg/tui/dashboard_flow_matrix_test.go`
**Status:** TODO

### Task 27 — Footer recovery after plan error (DM-P69)
**File:** `pkg/tui/dashboard_view.go` + `pkg/tui/dashboard_view_test.go`
**Status:** TODO

---

## Completion Gates

Phase 13 is done when:
- [ ] All DM-P40..DM-P69 rows in `ux-flow-matrix.md` are `Locked`.
- [ ] `artifacts/ux-fake-prod/issues.json` is empty after `make ux-fake-prod` and remains empty across two consecutive CI runs.
- [ ] `pkg/tui/dashboard_view_test.go` exists and the table test passes — no advertised footer key is a silent no-op.
- [ ] Phase 11 and Phase 12 tests pass unchanged.
