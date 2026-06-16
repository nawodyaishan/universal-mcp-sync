# Doctor Mode Phase 3 Implementation Tasks

## Track Summary

Add saved-plan apply. Phase 3 executes reviewed plan files safely while preserving the existing legacy apply flow.

## Prerequisites

- Phase 2 saved-plan foundation completed.
- Approved `docs/specs/doctor-mode-phase3/spec.md`.
- Approved `docs/specs/doctor-mode-phase3/plan.md`.

## Task List

### Task 1: Upgrade Saved Plan Schema For Apply

- **Objective:** Add non-secret execution metadata required by apply.
- **Source Artifacts:** `docs/specs/doctor-mode-phase3/spec.md`
- **Allowed Files:** `pkg/app/plan_v2.go`, `pkg/app/plan_v2_test.go`
- **Forbidden Files:** `pkg/tui`, provider implementations
- **Acceptance Criteria:**
  - `SavedPlanSchemaVersion` becomes `2`.
  - `PlanOperation` includes `provider_id`, `credential_ref`, `file_kind`, `backup_path`, and `will_create`.
  - Plan generation fills these fields.
  - Plan JSON still contains no credential values.
  - Schema v2 round-trip tests pass.
- **Verification Command:** `go test ./pkg/app`
- **Dependencies:** None
- **Risk Level:** Medium
- **Status:** Pending

### Task 2: Add Saved Plan Preflight

- **Objective:** Validate a saved plan before any mutation.
- **Source Artifacts:** `docs/specs/doctor-mode-phase3/plan.md`
- **Allowed Files:** `pkg/app/plan_apply.go`, `pkg/app/plan_apply_test.go`
- **Forbidden Files:** `cmd/usync`, `pkg/tui`
- **Acceptance Criteria:**
  - Preflight rejects schema mismatch.
  - Preflight rejects expired plan unless `ForceStale`.
  - Preflight rejects missing credentials.
  - Preflight recomputes target SHA and rejects mismatch.
  - Preflight checks path boundaries.
  - Preflight reports approval prompts for create, symlink, and project/workspace writes.
- **Verification Command:** `go test ./pkg/app`
- **Dependencies:** Task 1
- **Risk Level:** High
- **Status:** Pending

### Task 3: Add Saved Plan Dry Run

- **Objective:** Expose no-write preflight preview for saved plans.
- **Source Artifacts:** Phase 3 spec
- **Allowed Files:** `pkg/app/plan_apply.go`, `pkg/app/plan_format.go`, tests
- **Forbidden Files:** `pkg/config/files.go`
- **Acceptance Criteria:**
  - Dry-run executes preflight only.
  - Dry-run output lists operations and approval gates.
  - Dry-run does not call `WriteConfig`.
  - Dry-run does not run CLI commands.
- **Verification Command:** `go test ./pkg/app`
- **Dependencies:** Task 2
- **Risk Level:** Medium
- **Status:** Pending

### Task 4: Convert Saved Operations To Legacy Operations

- **Objective:** Reuse existing config mutation machinery.
- **Source Artifacts:** `pkg/app/app.go`, `pkg/app/plan_v2.go`
- **Allowed Files:** `pkg/app/plan_apply.go`, `pkg/app/plan_apply_test.go`
- **Forbidden Files:** `pkg/config/json_update.go`, `pkg/config/toml_update.go`
- **Acceptance Criteria:**
  - Saved file operations convert to `Operation`.
  - Provider config is regenerated from apply-time credentials.
  - Client-specific adaptation is applied.
  - Unsupported target IDs or file kinds fail preflight.
  - Raw credentials do not enter logs or output.
- **Verification Command:** `go test ./pkg/app`
- **Dependencies:** Tasks 1-2
- **Risk Level:** High
- **Status:** Pending

### Task 5: Implement File-Backed Apply From Plan

- **Objective:** Execute saved file operations transactionally.
- **Source Artifacts:** `pkg/app/app.go`, Phase 3 plan
- **Allowed Files:** `pkg/app/plan_apply.go`, `pkg/app/plan_apply_test.go`
- **Forbidden Files:** `pkg/tui`
- **Acceptance Criteria:**
  - File-backed operations write expected configs.
  - Existing `WriteConfig` path is used.
  - Prior writes rollback when a later write fails.
  - Verification runs after writes.
  - Locked target errors fail cleanly.
  - `ApplyResult` formatting remains redacted.
- **Verification Command:** `go test ./pkg/app`
- **Dependencies:** Task 4
- **Risk Level:** High
- **Status:** Pending

### Task 6: Add Approval Gate Interface

- **Objective:** Make risky operations explicit without tying app code to stdin.
- **Source Artifacts:** Phase 3 spec approval gates
- **Allowed Files:** `pkg/app/plan_apply.go`, tests
- **Forbidden Files:** `pkg/tui`
- **Acceptance Criteria:**
  - `Approver` interface exists.
  - `AutoApprove` bypasses prompts.
  - Denied approval aborts before writes.
  - Tests cover symlink and first-create prompts.
- **Verification Command:** `go test ./pkg/app`
- **Dependencies:** Task 2
- **Risk Level:** Medium
- **Status:** Pending

### Task 7: Add `usync apply --plan`

- **Objective:** Expose saved-plan apply through CLI.
- **Source Artifacts:** `cmd/usync/main.go`, `cmd/usync/plan_commands.go`
- **Allowed Files:** `cmd/usync/main.go`, `cmd/usync/plan_commands.go`, `cmd/usync/main_test.go`
- **Forbidden Files:** `pkg/tui`
- **Acceptance Criteria:**
  - `usync apply --plan <path> --dry-run` works.
  - `usync apply --plan <path> --auto-approve --keys-file <file>` works for file-backed targets.
  - `--force-stale` bypasses expiry only.
  - Legacy `usync --apply` and `usync sync --apply` remain unchanged.
  - CLI output is redacted.
- **Verification Command:** `go test ./cmd/usync`
- **Dependencies:** Tasks 3, 5, and 6
- **Risk Level:** High
- **Status:** Pending

### Task 8: Add CLI-Managed Operation Support

- **Objective:** Run CLI-managed saved-plan operations after file writes.
- **Source Artifacts:** research spec CLI adapter section
- **Allowed Files:** `pkg/app/plan_apply.go`, `pkg/app/adapter_cli.go`, tests
- **Forbidden Files:** `pkg/tui`
- **Acceptance Criteria:**
  - Claude Code CLI operation runs after file verification.
  - Missing CLI produces a clear redacted error or warning.
  - CLI output is redacted.
  - CLI failure does not claim file rollback.
  - Codex CLI support is either tested or explicitly deferred.
- **Verification Command:** `go test ./pkg/app`
- **Dependencies:** Task 5
- **Risk Level:** Medium
- **Status:** Pending

### Task 9: Add Audit Package And Apply Integration

- **Objective:** Record apply attempts without secrets.
- **Source Artifacts:** Phase 3 audit requirements
- **Allowed Files:** `pkg/audit/audit.go`, `pkg/audit/audit_test.go`, `pkg/app/plan_apply.go`
- **Forbidden Files:** `pkg/tui`
- **Acceptance Criteria:**
  - Audit writer appends JSONL.
  - Audit parent directory is `0700`.
  - Audit file is `0600`.
  - Audit entries include command, plan ID, targets, files touched, exit code, and redacted error.
  - Raw credential fixture values do not appear in audit logs.
  - Audit failure after successful apply becomes a warning.
- **Verification Command:** `go test ./pkg/audit ./pkg/app`
- **Dependencies:** Task 5
- **Risk Level:** Medium
- **Status:** Pending

### Task 10: Full Phase 3 Verification

- **Objective:** Confirm saved-plan apply does not regress existing flows.
- **Source Artifacts:** All Phase 3 tasks
- **Allowed Files:** No new edits unless tests identify an issue
- **Acceptance Criteria:**
  - `go test ./pkg/app ./cmd/usync` passes.
  - `go test ./...` passes.
  - `make test` passes.
  - Legacy apply and dry-run tests pass unchanged.
  - Saved-plan apply tests cover rollback, checksum mismatch, dry-run, and redaction.
- **Verification Command:** `go test ./...` and `make test`
- **Dependencies:** Tasks 1-9
- **Risk Level:** Low
- **Status:** Pending

## Dependency Order

Tasks 1-3 establish schema and preflight. Tasks 4-6 implement safe file-backed apply. Task 7 exposes CLI apply. Tasks 8-9 can follow after file-backed apply is stable. Task 10 closes the phase.

## Parallel-Safe Groups

- Task 3 can begin after Task 2.
- Task 6 can begin after Task 2.
- Task 9 audit package can be implemented standalone after the entry shape is agreed, but should not be wired until Task 5 is stable.

## Implementation Start Gate

Do not start coding Phase 3 until `spec.md`, `plan.md`, and `tasks.md` are accepted.
