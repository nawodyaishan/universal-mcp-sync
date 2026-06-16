# Doctor Mode Phase 2 Implementation Tasks

## Track Summary

Add saved batch plans and CLI plan/show commands. This phase persists reviewable plan artifacts only; config mutation from saved plans belongs to Phase 3.

## Prerequisites

- Phase 1b completed: `pkg/doctor` and `usync doctor` are available.
- Approved `docs/specs/doctor-mode-phase2/spec.md`.
- Approved `docs/specs/doctor-mode-phase2/plan.md`.

## Task List

### Task 1: Define Saved Plan Types

- **Objective:** Add schema-versioned plan data structures.
- **Source Artifacts:** `docs/specs/doctor-mode-phase2/spec.md`
- **Allowed Files:** `pkg/app/plan_v2.go`, `pkg/app/plan_v2_test.go`
- **Forbidden Files:** `pkg/tui`, `pkg/config`, `pkg/doctor`
- **Acceptance Criteria:**
  - `SavedPlan`, `CredentialRef`, `PlanOperation`, `DoctorSummary`, and action constants exist.
  - JSON tags match the Phase 2 spec.
  - Schema version constant exists.
  - Types do not store raw credential values.
  - Round-trip JSON test passes.
- **Verification Command:** `go test ./pkg/app`
- **Dependencies:** None
- **Risk Level:** Medium
- **Status:** Pending

### Task 2: Add Plan Store

- **Objective:** Save, load, list, and clean plan files safely.
- **Source Artifacts:** `docs/specs/doctor-mode-phase2/plan.md`
- **Allowed Files:** `pkg/app/plan_store.go`, `pkg/app/plan_store_test.go`
- **Forbidden Files:** `pkg/config/files.go` unless a reusable no-backup atomic writer is explicitly approved
- **Acceptance Criteria:**
  - `DefaultPlanDir` honors `$USYNC_PLAN_DIR`, `$XDG_CACHE_HOME`, then `~/.cache/usync/plans`.
  - Save creates directories with `0700`.
  - Save writes plan files with `0600`.
  - Load validates schema version and permissions.
  - List returns plans in deterministic order.
  - Clean supports expired-only behavior.
- **Verification Command:** `go test ./pkg/app`
- **Dependencies:** Task 1
- **Risk Level:** Medium
- **Status:** Pending

### Task 3: Add Plan ID And File Naming Helpers

- **Objective:** Generate local unique plan IDs and deterministic test names.
- **Source Artifacts:** `docs/specs/doctor-mode-phase2/plan.md`
- **Allowed Files:** `pkg/app/plan_store.go`, `pkg/app/plan_store_test.go`
- **Forbidden Files:** New external dependencies unless approved
- **Acceptance Criteria:**
  - Default plan IDs use `crypto/rand`.
  - Tests can inject fixed plan IDs.
  - Default file names match `usync-plan-<YYYYMMDD>-<prefix>.json`.
  - Collisions are handled by retry or clear error.
- **Verification Command:** `go test ./pkg/app`
- **Dependencies:** Task 2
- **Risk Level:** Low
- **Status:** Pending

### Task 4: Build Saved Plans From Doctor Selections

- **Objective:** Convert doctor selections into saved plan operations.
- **Source Artifacts:** Phase 1b doctor report types, existing `pkg/app.PrepareProvider`
- **Allowed Files:** `pkg/app/plan_v2.go`, `pkg/app/plan_v2_test.go`
- **Forbidden Files:** `pkg/doctor` unless Phase 1b requires small exported helpers
- **Acceptance Criteria:**
  - Builder accepts provider, credential profiles, doctor report/selection data, fixed time, and plan ID.
  - Builder computes current SHA-256 or missing sentinel for file-backed operations.
  - Builder sets `create`, `update`, `skip`, or `conflict`.
  - Builder records warnings for conflicts, low-confidence selections, project/workspace writes, symlinks, and missing runtimes.
  - Builder reuses existing provider generation and client adaptation where possible.
  - No raw credential values enter `SavedPlan`.
- **Verification Command:** `go test ./pkg/app`
- **Dependencies:** Tasks 1-3 and Phase 1b
- **Risk Level:** High
- **Status:** Pending

### Task 5: Add Saved Plan Formatting

- **Objective:** Render saved plans for humans and stable JSON consumers.
- **Source Artifacts:** `pkg/app/FormatPlan`, Phase 2 spec
- **Allowed Files:** `pkg/app/plan_format.go`, `pkg/app/plan_format_test.go`
- **Forbidden Files:** `pkg/tui`
- **Acceptance Criteria:**
  - Human formatter shows provider, plan ID, expiry, operations, warnings, and "no config files written".
  - JSON formatter is deterministic with fixed input.
  - Formatter redacts token-like strings.
  - Tests assert raw fixture keys are absent.
- **Verification Command:** `go test ./pkg/app`
- **Dependencies:** Task 1
- **Risk Level:** Medium
- **Status:** Pending

### Task 6: Add `usync plan` CLI

- **Objective:** Expose saved plan creation through CLI.
- **Source Artifacts:** `cmd/usync/main.go`, Phase 2 plan
- **Allowed Files:** `cmd/usync/main.go`, `cmd/usync/main_test.go`
- **Forbidden Files:** `pkg/tui`
- **Acceptance Criteria:**
  - `usync plan --provider exa --targets ...` creates a saved plan.
  - `usync plan --all-detected` uses Phase 1b doctor findings and skips conflicts.
  - `usync plan --out <path>` writes to the given path.
  - `usync plan` with no provider fails clearly.
  - `usync plan` with no target selection fails clearly.
  - Existing `sync`, no-subcommand TUI, `--dry-run`, and `--apply` flows remain unchanged.
- **Verification Command:** `go test ./cmd/usync`
- **Dependencies:** Tasks 4-5
- **Risk Level:** High
- **Status:** Pending

### Task 7: Add `usync show`

- **Objective:** Inspect saved plan files without mutation.
- **Source Artifacts:** Phase 2 spec
- **Allowed Files:** `cmd/usync/main.go`, `cmd/usync/main_test.go`
- **Forbidden Files:** `pkg/tui`
- **Acceptance Criteria:**
  - `usync show <plan>` prints human output.
  - `usync show <plan> --json` prints stable JSON only.
  - Show warns on expired plans.
  - Show does not require credentials.
  - Show does not write files.
- **Verification Command:** `go test ./cmd/usync`
- **Dependencies:** Tasks 2 and 5
- **Risk Level:** Medium
- **Status:** Pending

### Task 8: Add `usync plan list` And `usync plan clean`

- **Objective:** Manage local plan cache.
- **Source Artifacts:** Phase 2 plan
- **Allowed Files:** `cmd/usync/main.go`, `cmd/usync/main_test.go`
- **Forbidden Files:** `pkg/tui`
- **Acceptance Criteria:**
  - `usync plan list` lists plan ID, provider, created time, expiry, and path.
  - `usync plan clean --expired` removes expired plans only.
  - `usync plan clean --all` removes all cache plans after explicit flag.
  - Commands operate only on plan cache directory.
- **Verification Command:** `go test ./cmd/usync`
- **Dependencies:** Task 2
- **Risk Level:** Medium
- **Status:** Pending

### Task 9: Add Redaction And Permission Regression Tests

- **Objective:** Prove saved plans are safe to persist.
- **Source Artifacts:** Phase 2 data sensitivity requirements
- **Allowed Files:** `pkg/app/*_test.go`, `cmd/usync/main_test.go`, optional `pkg/redact/redact.go`
- **Forbidden Files:** Provider implementations
- **Acceptance Criteria:**
  - Serialized plan JSON does not contain raw Exa UUID keys.
  - Serialized plan JSON does not contain raw GitHub, Context7, or Tavily token-like strings.
  - Plan files are `0600`.
  - Plan directories are `0700`.
  - Human output is redacted.
- **Verification Command:** `go test ./pkg/app ./cmd/usync`
- **Dependencies:** Tasks 2, 5, and 6
- **Risk Level:** High
- **Status:** Pending

### Task 10: Full Phase 2 Verification

- **Objective:** Confirm saved plan work does not regress existing behavior.
- **Source Artifacts:** All Phase 2 tasks
- **Allowed Files:** No new edits unless tests identify an issue
- **Acceptance Criteria:**
  - `go test ./pkg/app ./cmd/usync` passes.
  - `go test ./...` passes.
  - `make test` passes.
  - Existing legacy dry-run/apply tests pass unchanged.
  - No raw credential fixture values appear in saved plan fixtures.
- **Verification Command:** `go test ./...` and `make test`
- **Dependencies:** Tasks 1-9
- **Risk Level:** Low
- **Status:** Pending

## Dependency Order

Tasks 1-3 establish persistence. Tasks 4-5 build plan content and formatting. Tasks 6-8 expose CLI commands. Tasks 9-10 close the safety and regression gates.

## Parallel-Safe Groups

- Task 2 and Task 5 can begin after Task 1.
- Task 7 can begin after Tasks 2 and 5.
- Task 8 can begin after Task 2.
- Task 9 should run after CLI and formatter behavior exists.

## Implementation Start Gate

Do not start coding Phase 2 until `spec.md`, `plan.md`, and `tasks.md` are accepted.
