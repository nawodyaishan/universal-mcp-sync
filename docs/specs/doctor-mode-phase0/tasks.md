# Doctor Mode Phase 0 Implementation Tasks

## Track Summary

Prepare the codebase for doctor mode by removing library stdout noise and adding a narrow per-file lock around the existing config write path.

## Prerequisites

- Approved `docs/specs/doctor-mode-phase0/spec.md`
- Approved `docs/specs/doctor-mode-phase0/plan.md`

## Task List

### Task 1: Remove Config Library Debug Output

- **Objective:** Ensure config mutation helpers never write debug output to stdout.
- **Source Artifacts:** `docs/specs/doctor-mode-phase0/spec.md`
- **Allowed Files:** `pkg/config/json_update.go`, tests if needed
- **Forbidden Files:** Provider implementations, TUI, CLI flow
- **Acceptance Criteria:**
  - Remove all `fmt.Printf("DEBUG: ...")` calls from `pkg/config/json_update.go`.
  - Remove any now-unused helper such as `getKeys`.
  - Keep JSON mutation output byte-for-byte compatible with existing tests.
  - `rg -n 'fmt\.Printf' pkg` returns no production-code matches.
- **Verification Command:** `go test ./pkg/config`
- **Dependencies:** None
- **Risk Level:** Low
- **Status:** Pending

### Task 2: Add Internal File Lock Helper

- **Objective:** Add bounded exclusive sibling-lock behavior for config writes.
- **Source Artifacts:** `docs/specs/doctor-mode-phase0/plan.md`
- **Allowed Files:** `pkg/config/files.go`, `pkg/config/files_test.go`
- **Forbidden Files:** `pkg/app`, `cmd/usync`, `pkg/tui`
- **Acceptance Criteria:**
  - Add `ErrFileLocked`.
  - Add internal lock acquisition using `<path>.lock`.
  - Use `os.O_CREATE|os.O_EXCL|os.O_WRONLY` and `0600`.
  - Retry lock acquisition a small fixed number of times.
  - Return a clear wrapped lock error after retry exhaustion.
  - Remove the lock file through deferred unlock after success and after write error.
- **Verification Command:** `go test ./pkg/config`
- **Dependencies:** Task 1 can run independently
- **Risk Level:** Medium
- **Status:** Pending

### Task 3: Wrap Existing `WriteWithBackup`

- **Objective:** Apply locking without changing callers or public write behavior.
- **Source Artifacts:** `docs/specs/doctor-mode-phase0/plan.md`
- **Allowed Files:** `pkg/config/files.go`, `pkg/config/files_test.go`
- **Forbidden Files:** `pkg/app/app.go` unless a compile error requires a small signature-compatible adjustment
- **Acceptance Criteria:**
  - `WriteWithBackup` keeps the same signature.
  - Lock is acquired before reading existing target contents.
  - Existing backup, atomic write, permissions, and rollback tests still pass.
  - Existing app apply tests still pass.
- **Verification Command:** `go test ./pkg/config ./pkg/app`
- **Dependencies:** Task 2
- **Risk Level:** Medium
- **Status:** Pending

### Task 4: Add Lock Behavior Tests

- **Objective:** Prove lock behavior is deterministic and does not corrupt files.
- **Source Artifacts:** `docs/specs/doctor-mode-phase0/spec.md`
- **Allowed Files:** `pkg/config/files_test.go`
- **Forbidden Files:** Production packages outside `pkg/config`
- **Acceptance Criteria:**
  - Test persistent lock returns `ErrFileLocked`.
  - Test failed write path removes lock.
  - Test concurrent writes do not leave partial content or a stale lock.
  - Test target permissions remain `0600`.
- **Verification Command:** `go test ./pkg/config -count=20`
- **Dependencies:** Task 3
- **Risk Level:** Medium
- **Status:** Pending

### Task 5: Decide Audit Package Scope

- **Objective:** Avoid accidentally expanding Phase 0 beyond safety prep.
- **Source Artifacts:** `docs/specs/doctor-mode-phase0/plan.md`
- **Allowed Files:** `docs/specs/doctor-mode-phase0/tasks.md`; optional `pkg/audit/*` only after approval
- **Forbidden Files:** Wiring audit into `pkg/app.Apply` in Phase 0
- **Acceptance Criteria:**
  - Either explicitly defer audit to Phase 3, or add only a standalone audited JSONL writer with tests.
  - No apply behavior changes.
- **Verification Command:** `go test ./...`
- **Dependencies:** None
- **Risk Level:** Low
- **Status:** Pending

### Task 6: Run Full Verification

- **Objective:** Confirm Phase 0 has no behavioral regressions.
- **Source Artifacts:** All Phase 0 tasks
- **Allowed Files:** No new edits unless tests identify an issue
- **Acceptance Criteria:**
  - `go test ./...` passes.
  - `make test` passes.
  - No production `fmt.Printf` remains in `pkg/`.
  - No `.lock` files remain after tests.
- **Verification Command:** `go test ./...` and `make test`
- **Dependencies:** Tasks 1-4
- **Risk Level:** Low
- **Status:** Pending

## Dependency Order

Task 1 can be done immediately. Tasks 2, 3, and 4 should be done in order. Task 5 can be decided before or during implementation. Task 6 closes the phase.

## Parallel-Safe Groups

- Task 1 and Task 5 are independent.
- Tasks 2-4 should remain sequential because tests depend on the lock helper behavior.

## Implementation Start Gate

Do not start coding Phase 0 until `spec.md`, `plan.md`, and `tasks.md` are accepted.
