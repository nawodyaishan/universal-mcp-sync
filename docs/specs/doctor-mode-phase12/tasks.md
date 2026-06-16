# Doctor Mode Phase 12 — Tasks

**Source plan:** `docs/specs/doctor-mode-phase12/plan.md`  
**Last updated:** 2026-05-23  
**Verification:** `NO_COLOR=1 TERM=xterm-256color go test ./...`

---

## PR 12a — Conflict Resolution

### Task 1 — Types, constants, model fields
**File:** `pkg/tui/dashboard.go`  
**Status:** TODO

- [ ] Add `screenConflictResolve dashboardScreen = 5`
- [ ] Add `ConflictResolution struct { ChosenPath, ChosenLabel string; Skipped bool }`
- [ ] Add `targetEntry struct { clientID manifest.ClientID; name string; isConflict bool }`
- [ ] Add `conflictCursor int`, `resolveTarget *doctor.ClientFinding`, `resolvedConflicts map[manifest.ClientID]ConflictResolution` to `DashboardModel`
- [ ] `go build ./pkg/tui` passes

---

### Task 2 — `allTargetEntries`, updated `eligibleClientIDs`, `buildAppSelection`, `applyResolutions`
**File:** `pkg/tui/dashboard.go`  
**Status:** TODO  
**Depends on:** Task 1

- [ ] Implement `allTargetEntries(report doctor.Report, resolved map[manifest.ClientID]ConflictResolution) []targetEntry` — eligible first (including resolved non-skipped conflicts), then unresolved conflicts
- [ ] Update `eligibleClientIDs(report doctor.Report, resolved map[manifest.ClientID]ConflictResolution) []manifest.ClientID` — include resolved non-skipped clients
- [ ] Update all callers of `eligibleClientIDs` to pass `m.resolvedConflicts`
- [ ] Update `buildAppSelection()` to use `allTargetEntries` for selection
- [ ] Implement `applyResolutions() DashboardModel` — re-adds resolved clients to `selectedClients`
- [ ] Call `m.applyResolutions()` at end of `providerReadinessMsg` handler
- [ ] Implement `conflictCandidatesForDisplay(c doctor.ClientFinding) []doctor.CandidateFinding` — first 2 that exist or are symlinks
- [ ] Implement `setResolution(m map[...], id, r) map[...]` helper
- [ ] `go test ./pkg/tui -run TestDashboard` passes

---

### Task 3 — Key handlers: `handleKeyTargetSelect` and `handleKeyConflictResolve`
**File:** `pkg/tui/dashboard.go`  
**Status:** TODO  
**Depends on:** Task 2

- [ ] Refactor `handleKeyTargetSelect` to use `allTargetEntries` for cursor navigation
- [ ] `Space` only toggles non-conflict entries
- [ ] `r` or `Enter` on a conflict entry: find `resolveTarget` from report, set `m.screen = screenConflictResolve`
- [ ] `Enter` on a non-conflict entry with selections: triggers `planCmd()` (unchanged)
- [ ] Implement `handleKeyConflictResolve(key string) (tea.Model, tea.Cmd)`:
  - `Esc` → `screenTargetSelect`, clear `resolveTarget`
  - `s` → record `Skipped: true`, return to target select
  - `1` → record `ChosenPath/Label` for candidate 0, add to `selectedClients`, return to target select
  - `2` → same for candidate 1 (no-op if `< 2` candidates)
- [ ] Wire `screenConflictResolve` into `handleKey` switch
- [ ] `go test ./pkg/tui -run TestDashboard` passes

---

### Task 4 — View renders: `renderTargetSelect` and `renderConflictResolve`
**File:** `pkg/tui/dashboard_view.go`  
**Status:** TODO  
**Depends on:** Tasks 2, 3

- [ ] Rewrite `renderTargetSelect` using `allTargetEntries`:
  - Eligible entries: cursor + checkbox `[x]`/`[ ]`
  - Conflict entries: `? <name>` (no checkbox); cursor visible on them
  - Action bar changes to `[r] resolve` when cursor on conflict entry
- [ ] Implement `renderConflictResolve()`:
  - Client name header
  - For each candidate: label, `(deprecated)` tag, path, symlink resolved, parse status, providers
  - Zero candidates: show "No accessible candidates found."
  - Action bar: `[s] skip  [Esc] cancel  [1] use this  [2] use this` (1/2 only if candidates exist)
  - Credentials in `ParseError` pass through `redact.Key()`
- [ ] Wire `screenConflictResolve` case into `View()`
- [ ] `go test ./pkg/tui` passes
- [ ] `go build ./pkg/tui` passes

---

### Task 5 — Unit tests for conflict resolution
**File:** `pkg/tui/dashboard_test.go`  
**Status:** TODO  
**Depends on:** Tasks 3, 4

- [ ] `TestConflictClient_CursorReachesConflict` — scan with conflict client; navigate; assert `allTargetEntries[cursor].isConflict`
- [ ] `TestConflictClient_ROpensOverlay` — press `r` on conflict; assert `screenConflictResolve`, `resolveTarget != nil`
- [ ] `TestConflictResolve_1MovesToEligible` — press `1`; assert `ChosenPath`, `selectedClients[id] == true`, `screenTargetSelect`
- [ ] `TestConflictResolve_2UsesSecondCandidate` — press `2`; assert second candidate path stored
- [ ] `TestConflictResolve_SSkipsClient` — press `s`; assert `Skipped == true`, NOT in `selectedClients`
- [ ] `TestConflictResolve_EscCancels` — press `Esc`; assert `resolvedConflicts` unchanged
- [ ] `TestAllTargetEntries_IncludesResolvedConflict` — pure function; resolved non-skipped → `isConflict: false`
- [ ] `TestAllTargetEntries_ExcludesSkippedConflict` — skipped → absent from entries
- [ ] `go test ./pkg/tui -run TestConflict -v` all pass

---

### Task 6 — Golden test for `screenConflictResolve`
**File:** `pkg/tui/dashboard_golden_test.go`  
**Status:** TODO  
**Depends on:** Task 4

- [ ] Add `TestGoldenScreenConflictResolve` with two-candidate fake `resolveTarget`
- [ ] `NO_COLOR=1 go test ./pkg/tui -run TestGoldenScreenConflictResolve -update` generates golden file
- [ ] Commit golden file
- [ ] `go test ./pkg/tui -run TestGoldenScreenConflictResolve` passes without `-update`

---

## PR 12b — Playwright-Style E2E Flow Tests

### Task 7 — `dashboard_flow_test.go`: all flow tests
**File:** `pkg/tui/dashboard_flow_test.go` (new)  
**Status:** TODO  
**Depends on:** Tasks 1–6

- [ ] Implement `happyFlowSetup(t)` helper → `(*FakeScanner, *FakeDashboardManager, []provider.CredentialProfile)` with valid `SavedPlan` fields
- [ ] `TestDashboardFlow_HappyPath`:
  - Walk all 5 screens with key inputs
  - `waitForText` at each screen
  - `tm.FinalModel(t)` → `screen == screenApplyResult`, `applyResult != nil`
- [ ] `TestDashboardFlow_ValidationFails`:
  - `mgr.ValidErr` set
  - Walk to provider ready → press Enter → error visible → screen stays `screenProviderReady`
- [ ] `TestDashboardFlow_PlanFails`:
  - `mgr.PlanErr` set
  - Walk to target select → press Enter → error visible → screen stays `screenTargetSelect`
- [ ] `TestDashboardFlow_ApplyFails`:
  - `mgr.ApplyErr` set
  - Walk to plan preview → press `y` → error visible → `screenApplyResult` via `FinalModel`
- [ ] `TestDashboardFlow_EscNavigation`:
  - `p` → wait "Provider Readiness" → Esc → wait "System Status"
  - `p` → Enter → wait "Select Targets" → Esc → wait "Provider Readiness"
- [ ] `TestDashboardFlow_NoRawCredential`:
  - UUID key in profiles
  - Happy path; at each `waitForText` assert UUID absent from buffer
- [ ] `TestDashboardFlow_ConflictResolution`:
  - Scanner has one conflict client
  - Walk to target select → navigate to conflict → `r` → wait "Resolve Conflict"
  - Press `1` → wait "Select Targets" → conflict entry gone; client eligible
  - Continue to plan preview, cancel with `n`
- [ ] All tests: no `time.Sleep`; all waits via `waitForText`/`waitForAll`
- [ ] `go test ./pkg/tui -run TestDashboardFlow -v` all pass

---

## Completion Checklist

- [ ] PR 12a merged: `go test ./pkg/tui` — all pass including Phase 7/8/11 regressions
- [ ] PR 12b merged: `go test ./pkg/tui -run TestDashboardFlow` — all 7 tests pass
- [ ] `NO_COLOR=1 TERM=xterm-256color go test ./...` — all 18 packages pass
- [ ] No `time.Sleep` in any new test (enforce via grep in CI)
- [ ] AC-1 through AC-14 from spec verified
- [ ] PR 12c merged: DM-P14, DM-P32, DM-P33, DM-P34, DM-P35 pass in `make ux-fake-prod`
- [ ] AC-15 through AC-17 from spec verified

---

## PR 12c — Candidate-Level Target Rows

### Task 8 — Target row model and discovery
**Files:** `pkg/tui/dashboard.go`, `pkg/tui/dashboard_test.go`  
**Status:** TODO  
**Depends on:** PR 12a/12b

- [ ] Replace client-only target entries with concrete target rows carrying client ID, candidate label, path, scope, file kind, creatable, and git-warning.
- [ ] Build rows from `doctor.ClientFinding.Candidates`.
- [ ] Hide project/workspace rows when `includeWorkspace == false`.
- [ ] Preserve conflict resolution choices as concrete target rows.
- [ ] Add unit tests for row construction, workspace filtering, and resolved conflict row identity.
- [ ] Verification: `go test ./pkg/tui -run "TestTargetRows|TestAllTargetEntries" -v`

### Task 9 — Target select UX for concrete rows
**Files:** `pkg/tui/dashboard_view.go`, `pkg/tui/dashboard_golden_test.go`  
**Status:** TODO  
**Depends on:** Task 8

- [ ] Render each row with app name, candidate label, scope, and path hint.
- [ ] Show workspace/project rows only after pressing `i`.
- [ ] Show git-warning signal for project/workspace rows.
- [ ] Update target-select golden coverage at 80-column width.
- [ ] Verification: `NO_COLOR=1 go test ./pkg/tui -run "TestGoldenScreenTargetSelect|TestDashboardFlowMatrix_Workspace" -v`

### Task 10 — Concrete planning input
**Files:** `pkg/app/app.go`, `pkg/app/app_test.go`, `pkg/tui/dashboard.go`, `pkg/tui/dashboard_test.go`  
**Status:** TODO  
**Depends on:** Task 8

- [ ] Pass selected target rows to planning without expanding back to all files for the app.
- [ ] Ensure one checked file row produces one planned file operation.
- [ ] Preserve CLI-managed target behavior for Claude Code/Codex where a row represents a CLI scope.
- [ ] Add app-level tests for narrowed planning by path, scope, and file kind.
- [ ] Verification: `go test ./pkg/app ./pkg/tui`

### Task 11 — Docker matrix coverage for candidate-level UX
**Files:** `pkg/tui/dashboard_flow_matrix_test.go`, `tests/ux-fake-prod/run-flow.sh`, `docs/specs/doctor-mode-phase12/ux-flow-matrix.md`  
**Status:** TODO  
**Depends on:** Tasks 8-10

- [ ] DM-P14: workspace toggle changes target list.
- [ ] DM-P32: workspace off excludes project/workspace targets from planning.
- [ ] DM-P33: workspace target plan preview shows scope and git warning.
- [ ] DM-P34: multi-file client can select exactly one candidate.
- [ ] DM-P35: resolved conflict row shows chosen label/path before planning.
- [ ] Verification: `make ux-fake-prod`; `artifacts/ux-fake-prod/issues.json` is empty.
