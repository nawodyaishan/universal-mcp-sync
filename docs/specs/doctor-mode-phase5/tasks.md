# Doctor Mode Phase 5 Implementation Tasks

## Track Summary

Make the TUI doctor-first and add safe Gemini CLI to Antigravity migration UX. This phase must reuse doctor, validate, saved-plan, apply, manifest, config, and audit APIs rather than duplicating their behavior in the TUI.

## Prerequisites

- Phase 4 credential validation completed.
- `pkg/doctor` and `usync doctor` completed from Phase 1b. If missing, complete that dependency before dashboard implementation.
- Approved `docs/specs/doctor-mode-phase5/spec.md`.
- Approved `docs/specs/doctor-mode-phase5/plan.md`.

## Task List

### Task 1: Close Doctor Dependency Gate

- **Objective:** Ensure dashboard work has a real read-only scan source.
- **Source Artifacts:** Phase 1 spec and Phase 5 dependency gate.
- **Allowed Files:** `pkg/doctor/*`, `cmd/usync/main.go`, doctor tests only if missing.
- **Forbidden Files:** `pkg/tui` dashboard implementation until this gate is satisfied.
- **Acceptance Criteria:**
  - `pkg/doctor` exists and exposes scan report types.
  - `usync doctor` exists with human and JSON output.
  - Doctor scan is read-only.
  - Doctor tests cover healthy, empty, malformed, conflict, symlink, and sunset fixture homes.
  - `pkg/doctor` does not import `pkg/app` or `pkg/tui`.
- **Verification Command:** `go test ./pkg/doctor ./cmd/usync`
- **Dependencies:** None
- **Risk Level:** High
- **Status:** Pending

### Task 2: Split Wizard And Dashboard Entry Points

- **Objective:** Preserve old wizard while making room for dashboard default.
- **Source Artifacts:** `pkg/tui/model.go`, Phase 5 plan.
- **Allowed Files:** `pkg/tui/model.go`, `cmd/usync/main.go`, `pkg/tui/model_test.go`, `cmd/usync/main_test.go`.
- **Forbidden Files:** `pkg/app`, `pkg/config`.
- **Acceptance Criteria:**
  - Current provider-first model is available as `NewWizardModel` or equivalent.
  - `usync --wizard` launches the current wizard path.
  - Default `usync` launches dashboard only after Task 1 is complete.
  - Existing wizard tests are updated without reducing coverage.
  - Legacy `sync`, `--dry-run`, `--apply`, `plan`, `apply`, `show`, and `validate` behavior is unchanged.
- **Verification Command:** `go test ./pkg/tui ./cmd/usync`
- **Dependencies:** Task 1 for default dashboard switch; wrapper can be prepared earlier.
- **Risk Level:** Medium
- **Status:** Pending

### Task 3: Add Dashboard Model Skeleton

- **Objective:** Add non-blocking TUI dashboard states.
- **Source Artifacts:** Research spec screen 10.2.
- **Allowed Files:** `pkg/tui/dashboard.go`, `pkg/tui/dashboard_test.go`, `pkg/tui/model.go`.
- **Forbidden Files:** `pkg/doctor` internals, `pkg/config`.
- **Acceptance Criteria:**
  - Dashboard has loading, loaded, empty, partial, and error states.
  - First render does not wait for scan completion.
  - Scan command is injectable for deterministic tests.
  - `ctrl+c` and `q` quit.
  - Window resize does not produce overlapping text.
- **Verification Command:** `go test ./pkg/tui`
- **Dependencies:** Task 2
- **Risk Level:** Medium
- **Status:** Pending

### Task 4: Render Doctor Summary And Client Table

- **Objective:** Show system status before provider selection.
- **Source Artifacts:** Phase 5 TUI requirements.
- **Allowed Files:** `pkg/tui/dashboard.go`, `pkg/tui/dashboard_test.go`, optional helper file.
- **Forbidden Files:** `pkg/config`, provider implementations.
- **Acceptance Criteria:**
  - Dashboard shows clients detected, ready clients, conflicts, and existing MCP provider IDs.
  - Client table shows client name, status, effective path, and MCP provider IDs.
  - No-client fixture renders a useful empty state with relevant docs/install links.
  - Output is redacted.
  - Narrow terminal rendering remains coherent.
- **Verification Command:** `go test ./pkg/tui`
- **Dependencies:** Task 3
- **Risk Level:** Medium
- **Status:** Pending

### Task 5: Add Provider Readiness View Model

- **Objective:** Group providers by actionable readiness.
- **Source Artifacts:** Phase 4 validation package, manifest provider metadata.
- **Allowed Files:** `pkg/tui/dashboard_readiness.go`, `pkg/tui/dashboard_readiness_test.go`.
- **Forbidden Files:** provider implementations, `pkg/config`.
- **Acceptance Criteria:**
  - Providers are grouped as ready now, ready with supplied credentials, needs credentials, and blocked.
  - Missing credential rows include get-key URLs from manifest metadata.
  - Runtime blockers are represented from doctor/runtime findings.
  - No raw credentials appear in readiness strings.
- **Verification Command:** `go test ./pkg/tui`
- **Dependencies:** Task 4
- **Risk Level:** Medium
- **Status:** Pending

### Task 6: Add Conflict Resolution Model

- **Objective:** Let users resolve path conflicts before setup.
- **Source Artifacts:** Research spec screen 10.3.
- **Allowed Files:** `pkg/tui/conflict.go`, `pkg/tui/conflict_test.go`, dashboard model wiring.
- **Forbidden Files:** `pkg/config` writes, migration apply code.
- **Acceptance Criteria:**
  - Antigravity conflict is shown before provider selection.
  - Current, legacy, and skip choices are supported.
  - Symlink and resolved target details are displayed when available.
  - Selection updates in-memory target selection only.
  - No files are written.
- **Verification Command:** `go test ./pkg/tui`
- **Dependencies:** Tasks 3-4
- **Risk Level:** High
- **Status:** Pending

### Task 7: Integrate Offline And Live Validation In TUI

- **Objective:** Reuse Phase 4 validation in the credential step.
- **Source Artifacts:** `pkg/validate`, Phase 5 spec.
- **Allowed Files:** `pkg/tui/setup_form.go`, `pkg/tui/dashboard.go`, validation-related TUI tests.
- **Forbidden Files:** `pkg/validate` unless a bug is discovered.
- **Acceptance Criteria:**
  - Offline validation runs before preview.
  - Malformed credentials block preview with redacted output.
  - Live validation requires explicit user action.
  - Live validation uses injected/mock HTTP in tests.
  - Cached live result behavior is reflected when available.
- **Verification Command:** `go test ./pkg/tui ./pkg/validate`
- **Dependencies:** Task 5
- **Risk Level:** High
- **Status:** Pending

### Task 8: Wire Dashboard Preview To Saved Plans

- **Objective:** Make dashboard setup use the Phase 2-3 saved-plan workflow.
- **Source Artifacts:** `pkg/app/plan_v2.go`, `pkg/app/plan_apply.go`.
- **Allowed Files:** `pkg/tui/preview.go`, dashboard setup files, `pkg/tui/*_test.go`.
- **Forbidden Files:** saved-plan schema changes unless approved separately.
- **Acceptance Criteria:**
  - Dashboard path saves a redacted plan before preview/apply.
  - Preview displays saved-plan operations and warnings.
  - Apply calls saved-plan apply APIs.
  - Approval gates for create, symlink, and workspace/project writes are preserved.
  - Legacy wizard can keep current in-memory apply behavior.
- **Verification Command:** `go test ./pkg/tui ./pkg/app`
- **Dependencies:** Tasks 6-7
- **Risk Level:** High
- **Status:** Pending

### Task 9: Reconcile Gemini And Antigravity Path Metadata

- **Objective:** Prevent migration from targeting the wrong Antigravity surface.
- **Source Artifacts:** Exa audit findings, `pkg/manifest/clients.go`, Phase 5 spec.
- **Allowed Files:** `pkg/manifest/clients.go`, `pkg/manifest/*_test.go`, `docs/specs/doctor-mode-phase5/*`.
- **Forbidden Files:** `pkg/tui`, `pkg/migrate` apply logic until path metadata is reconciled.
- **Acceptance Criteria:**
  - Gemini CLI source candidates include global `~/.gemini/settings.json` and workspace `.gemini/settings.json` when workspace scanning is enabled.
  - Antigravity CLI and Antigravity IDE are represented as distinct migration target kinds.
  - Antigravity CLI current and legacy candidates are documented separately from Antigravity IDE candidates.
  - Manifest tests prevent duplicate target labels and verify replacement/deprecation metadata.
  - Task notes include source confidence: official Google transition docs versus empirical GitHub issue reports.
- **Verification Command:** `go test ./pkg/manifest`
- **Dependencies:** Task 1
- **Risk Level:** High
- **Status:** Pending

### Task 10: Add Migration Data Model And Preview

- **Objective:** Compute Gemini CLI to Antigravity migration actions without writing.
- **Source Artifacts:** Research spec section 15.
- **Allowed Files:** `pkg/migrate/gemini_antigravity.go`, `pkg/migrate/gemini_antigravity_test.go`.
- **Forbidden Files:** `pkg/tui`, `cmd/usync` until package behavior is tested.
- **Acceptance Criteria:**
  - Preview reads Gemini source and Antigravity target candidates.
  - Preview reports source path, target path, resolved symlink target, provider IDs, skips, conflicts, and backup path.
  - Preview writes nothing.
  - Malformed source/target configs return clear redacted errors.
  - Raw credentials and credential URLs are absent from preview output.
- **Verification Command:** `go test ./pkg/migrate`
- **Dependencies:** Tasks 1 and 9. This can run independently of dashboard tasks after path metadata is reconciled.
- **Risk Level:** High
- **Status:** Pending

### Task 11: Implement Migration Apply

- **Objective:** Apply safe copy actions from Gemini CLI config to Antigravity config.
- **Source Artifacts:** Phase 5 migration behavior.
- **Allowed Files:** `pkg/migrate/gemini_antigravity.go`, `pkg/migrate/gemini_antigravity_test.go`, optional `pkg/audit` integration.
- **Forbidden Files:** `pkg/tui`, provider implementations.
- **Acceptance Criteria:**
  - Apply writes through `config.WriteWithBackup`.
  - Existing target entries are preserved.
  - Differing duplicate provider entries are skipped or reported as conflicts.
  - Symlink target is resolved with `os.Lstat` and `filepath.EvalSymlinks`.
  - Symlink target outside home is refused.
  - Symlink itself is not removed or replaced.
  - Source Gemini config is not deleted or modified.
  - Target parse health is verified after write.
- **Additional Acceptance Criteria From Audit:**
  - Ambiguous Antigravity CLI vs IDE target returns a conflict unless target kind is explicit.
  - Apply supports `--target antigravity-cli|antigravity-ide` once exposed by CLI.
- **Verification Command:** `go test ./pkg/migrate ./pkg/config`
- **Dependencies:** Task 10
- **Risk Level:** High
- **Status:** Pending

### Task 12: Add `usync migrate gemini-to-antigravity`

- **Objective:** Expose migration preview/apply through CLI.
- **Source Artifacts:** Phase 5 spec.
- **Allowed Files:** `cmd/usync/main.go`, `cmd/usync/migrate_command.go`, `cmd/usync/main_test.go`.
- **Forbidden Files:** `pkg/tui`.
- **Acceptance Criteria:**
  - `usync migrate gemini-to-antigravity --dry-run` works.
  - `usync migrate gemini-to-antigravity --apply` works.
  - Command supports `--home-dir` for fixture testing.
  - Command supports `--target antigravity-cli|antigravity-ide` when target ambiguity exists.
  - Running without `--dry-run` or `--apply` returns a helpful error.
  - Output is redacted.
  - CLI tests cover missing source, dry-run, apply, ambiguous target, explicit target, and symlink-outside-home refusal.
- **Verification Command:** `go test ./cmd/usync ./pkg/migrate`
- **Dependencies:** Tasks 10-11
- **Risk Level:** Medium
- **Status:** Pending

### Task 13: Add Dashboard Migration Card And Action

- **Objective:** Make Gemini sunset migration discoverable in the TUI.
- **Source Artifacts:** Research spec screens 10.2 and 15.
- **Allowed Files:** `pkg/tui/migration.go`, `pkg/tui/dashboard.go`, TUI tests.
- **Forbidden Files:** `pkg/migrate` unless Task 9-10 exposed insufficient API.
- **Acceptance Criteria:**
  - Dashboard shows consumer sunset warning when Gemini source exists and date is before or on July 15, 2026.
  - Warning text notes enterprise access may differ.
  - Dashboard shows migration card when source and Antigravity candidates are relevant.
  - Migration card can show dry-run preview.
  - Apply action calls `pkg/migrate` and displays result.
  - Raw credentials are absent from rendered strings.
- **Verification Command:** `go test ./pkg/tui ./pkg/migrate`
- **Dependencies:** Tasks 10-12
- **Risk Level:** Medium
- **Status:** Pending

### Task 14: Full Phase 5 Verification

- **Objective:** Confirm TUI and migration work does not regress CLI workflows.
- **Source Artifacts:** All Phase 5 tasks.
- **Allowed Files:** No new edits unless tests identify an issue.
- **Acceptance Criteria:**
  - `go test ./pkg/tui ./pkg/migrate ./cmd/usync` passes.
  - `go test ./...` passes.
  - `make lint` passes.
  - `make test` passes.
  - `make build` passes.
  - No real live-validation network calls occur in tests.
  - Migration fixtures do not leak raw credentials in output, backups beyond expected local config copies, audit logs, or test failure strings.
- **Verification Command:** `go test ./...`, `make lint`, `make test`, and `make build`
- **Dependencies:** Tasks 1-13
- **Risk Level:** Low
- **Status:** Pending

## Dependency Order

Task 1 is the implementation gate. Task 2 splits entry points. Tasks 3-8 build the dashboard path. Task 9 reconciles path metadata. Tasks 10-12 build migration CLI. Task 13 connects migration to the dashboard. Task 14 closes the phase.

## Parallel-Safe Groups

- Tasks 2 and 9 can start after Task 1 if the old wizard wrapper is isolated.
- Tasks 4 and 5 can run in parallel after Task 3.
- Tasks 6 and 7 can run in parallel after Tasks 4-5.
- Tasks 10-11 can be developed independently from dashboard rendering after Task 9.
- Task 13 waits for migration package and dashboard state.

## Implementation Start Gate

Do not start coding Phase 5 until `spec.md`, `plan.md`, and `tasks.md` are accepted. If `pkg/doctor` remains absent, complete the Phase 1b doctor dependency before dashboard implementation.
