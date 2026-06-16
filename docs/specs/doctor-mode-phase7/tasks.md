# Doctor Mode Phase 7 Tasks

## Task 1: Add Dashboard Scanner Interface

- **Objective:** Let dashboard tests inject deterministic doctor reports.
- **Allowed Files:** `pkg/tui/dashboard.go`, `pkg/tui/dashboard_test.go`.
- **Forbidden Files:** `pkg/doctor` internals, `pkg/config`.
- **Acceptance Criteria:**
  - `DashboardScanner` or equivalent interface exists.
  - Production scanner wraps `pkg/doctor`.
  - Production scanner enables runtime checks and carries CLI home/workspace context where available.
  - Fake scanner tests do not touch the filesystem.
  - `pkg/tui` does not parse config files.
- **Verification:** `go test ./pkg/tui`
- **Risk:** Medium
- **Status:** Done

## Task 2: Add Dashboard Model Skeleton

- **Objective:** Introduce read-only dashboard state and async scan command.
- **Allowed Files:** `pkg/tui/dashboard.go`, `pkg/tui/dashboard_test.go`.
- **Acceptance Criteria:**
  - `NewDashboardModel` exists.
  - `Init` returns a non-nil scan command.
  - First `View` shows loading/scanning state.
  - Scan command returns typed completion message.
  - `Update` handles scan success and scan error.
  - `q` and `ctrl+c` quit.
- **Verification:** `go test ./pkg/tui`
- **Risk:** Medium
- **Status:** Done

## Task 3: Render Dashboard States

- **Objective:** Make loading, loaded, empty, conflict, runtime blocker, and error states readable.
- **Allowed Files:** `pkg/tui/dashboard_view.go`, `pkg/tui/dashboard_test.go`, small helper edits in `pkg/tui/helpers.go` if needed.
- **Acceptance Criteria:**
  - Loading view renders immediately.
  - Empty report renders stable text such as `No AI clients detected`.
  - Loaded report shows client counts and configured provider IDs.
  - Conflict clients appear before ordinary clients.
  - Runtime blockers are visible.
  - Warnings are visible and redacted.
  - Narrow width test remains coherent.
- **Verification:** `go test ./pkg/tui`
- **Risk:** Medium
- **Status:** Done

## Task 4: Add Dashboard Key Actions

- **Objective:** Add read-only dashboard navigation affordances.
- **Allowed Files:** `pkg/tui/dashboard.go`, `pkg/tui/dashboard_view.go`, `pkg/tui/dashboard_test.go`.
- **Acceptance Criteria:**
  - `r` starts another scan command.
  - `?` toggles help.
  - `w` sets a final-model wizard route flag and quits, or sets a supported read-only placeholder state.
  - Dashboard code does not start a nested Bubble Tea program from inside `Update`.
  - `c`, `x`, and `m` show read-only placeholder status messages for Phase 8/10 work.
  - Placeholder actions write no files.
  - Action bar remains visible in loading, empty, loaded, and error states.
- **Verification:** `go test ./pkg/tui`
- **Risk:** Low
- **Status:** Done

## Task 5: Wire Default CLI To Dashboard

- **Objective:** Make `usync` status-first while preserving the old wizard.
- **Allowed Files:** `cmd/usync/main.go`, `cmd/usync/main_test.go`, possibly a small TUI constructor helper.
- **Acceptance Criteria:**
  - Default interactive `usync` uses `NewDashboardModel`.
  - `usync --wizard` uses `NewWizardModel`.
  - If dashboard `w` routing is implemented, `cmd/usync` inspects the final dashboard model and then runs the existing wizard program.
  - `sync --dry-run`, `sync --apply`, `plan`, `apply`, `show`, `validate`, `doctor`, and `providers` behavior remains unchanged.
  - Command tests cover default/wizard dispatch without hanging.
- **Verification:** `go test ./cmd/usync ./pkg/tui`
- **Risk:** Medium
- **Status:** Done

## Task 6: Add Minimal Teatest Coverage

- **Objective:** Prove dashboard works in a real Bubble Tea harness without making tests brittle.
- **Allowed Files:** `pkg/tui/dashboard_teatest_test.go`, optional `tests/e2e/testdata` golden only if needed.
- **Acceptance Criteria:**
  - `teatest.NewTestModel` runs dashboard with fake scanner.
  - Test waits with `teatest.WaitFor`, not `time.Sleep`.
  - Test sends quit key and waits for completion.
  - Test asserts stable visible text, not raw full ANSI output.
- **Verification:** `go test ./pkg/tui`
- **Risk:** Low
- **Status:** Done

## Task 7: Add Redaction And No-Write Guards

- **Objective:** Prevent dashboard regressions around sensitive data and read-only behavior.
- **Allowed Files:** `pkg/tui/dashboard_test.go`, optional test helper files.
- **Acceptance Criteria:**
  - Dashboard output does not include credential-bearing URLs in fake reports.
  - Dashboard fake scanner can include sensitive-looking data and rendered output remains redacted or omitted.
  - Dashboard renders manifest client/scope labels for Antigravity-style multi-surface findings instead of hardcoding one Antigravity path.
  - Dashboard tests prove no write helper is used.
  - `pkg/tui` import scan or code review confirms no `pkg/config` parsing/writing helpers are added.
- **Verification:** `go test ./pkg/tui`
- **Risk:** Medium
- **Status:** Done

## Task 8: Phase 7 Verification

- **Objective:** Confirm dashboard foundation does not regress CLI, TUI, or build quality.
- **Allowed Files:** No planned edits unless verification finds an issue.
- **Acceptance Criteria:**
  - `go test ./pkg/tui ./cmd/usync` passes.
  - `go test ./...` passes.
  - `make lint` passes.
  - `make build` passes.
  - `make test` passes.
  - No tests call real network or depend on actual installed AI clients.
- **Verification:** listed commands
- **Risk:** Low
- **Status:** Done

## Recommended PR Boundary

Keep all Phase 7 tasks in one PR only if the diff remains focused. Split if CLI routing or `teatest` introduces noisy churn:

- PR 7a: dashboard model/view/tests.
- PR 7b: default CLI routing and `teatest` coverage.
