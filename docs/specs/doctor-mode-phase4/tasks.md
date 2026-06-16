# Doctor Mode Phase 4 Implementation Tasks

## Track Summary

Add credential validation as a reusable layer. Phase 4 validates credentials before plan/apply, supports explicit live checks for providers with safe endpoints, and stores only redacted validation cache data.

## Prerequisites

- Phase 3 saved-plan apply completed.
- Approved `docs/specs/doctor-mode-phase4/spec.md`.
- Approved `docs/specs/doctor-mode-phase4/plan.md`.

## Task List

### Task 1: Add Validation Types

- **Objective:** Define provider-neutral validation data structures.
- **Source Artifacts:** `docs/specs/doctor-mode-phase4/spec.md`
- **Allowed Files:** `pkg/validate/types.go`, `pkg/validate/types_test.go`
- **Forbidden Files:** `pkg/tui`, provider implementations
- **Acceptance Criteria:**
  - Status constants exist for `ok`, `warning`, `failed`, and `skipped`.
  - Mode constants exist for `offline` and `live`.
  - `Result`, `Request`, and batch result types use JSON tags.
  - Types do not contain raw credential-specific fields beyond in-memory request values.
  - JSON round-trip test passes.
- **Verification Command:** `go test ./pkg/validate`
- **Dependencies:** None
- **Risk Level:** Low
- **Status:** Pending

### Task 2: Add Provider-Neutral Key File Parser

- **Objective:** Parse `KEY=value` credential files without leaking values.
- **Source Artifacts:** Phase 4 credential input format
- **Allowed Files:** `pkg/validate/keys_file.go`, `pkg/validate/keys_file_test.go`
- **Forbidden Files:** `pkg/exa`, `cmd/usync`
- **Acceptance Criteria:**
  - Blank lines are ignored.
  - `#` comments are ignored.
  - `KEY=value` lines are parsed.
  - Whitespace around keys and values is trimmed.
  - Malformed lines return line-numbered errors.
  - Parse errors do not include credential values.
- **Verification Command:** `go test ./pkg/validate`
- **Dependencies:** Task 1
- **Risk Level:** Medium
- **Status:** Pending

### Task 3: Implement Offline Validation

- **Objective:** Validate required credential presence and format without network calls.
- **Source Artifacts:** `pkg/provider/types.go`, provider `RequiredCredentials`
- **Allowed Files:** `pkg/validate/offline.go`, `pkg/validate/offline_test.go`
- **Forbidden Files:** provider implementations unless a provider validator bug is discovered
- **Acceptance Criteria:**
  - Exa valid/invalid examples are tested.
  - GitHub valid/invalid examples are tested.
  - Context7 valid/invalid examples are tested.
  - Tavily valid/invalid examples are tested.
  - Missing required credentials return `failed`.
  - Providers with no required credentials return a clear non-failed result.
  - Raw fake keys do not appear in messages or JSON output.
- **Verification Command:** `go test ./pkg/validate ./pkg/provider`
- **Dependencies:** Tasks 1-2
- **Risk Level:** High
- **Status:** Pending

### Task 4: Add Duplicate Credential Detection

- **Objective:** Detect repeated credential values in one validation batch.
- **Source Artifacts:** Research spec section 13.1
- **Allowed Files:** `pkg/validate/offline.go`, `pkg/validate/offline_test.go`
- **Forbidden Files:** `pkg/app`, `cmd/usync`
- **Acceptance Criteria:**
  - Duplicate raw values return `warning` in validation reports.
  - Duplicate detection compares raw values only in memory.
  - Duplicate messages use redacted labels only.
  - Test asserts raw duplicate fixture value is absent from output.
- **Verification Command:** `go test ./pkg/validate`
- **Dependencies:** Task 3
- **Risk Level:** Medium
- **Status:** Pending

### Task 5: Add Credential Cache

- **Objective:** Store live validation results privately for 24 hours.
- **Source Artifacts:** Phase 4 cache rules
- **Allowed Files:** `pkg/validate/cache.go`, `pkg/validate/cache_test.go`
- **Forbidden Files:** `pkg/audit`, `pkg/app`
- **Acceptance Criteria:**
  - Default path is `~/.usync/cache/credentials.json`.
  - Parent directory is `0700`.
  - Cache file is `0600`.
  - Cache hit inside TTL returns cached result.
  - Expired cache entry is ignored.
  - Cache key contains no raw credential value.
  - Cache JSON contains no raw credential value.
  - Corrupt cache behavior is tested.
- **Verification Command:** `go test ./pkg/validate`
- **Dependencies:** Task 1
- **Risk Level:** Medium
- **Status:** Pending

### Task 6: Implement Live GitHub Validation

- **Objective:** Validate GitHub PATs through an opt-in quota-safe endpoint.
- **Source Artifacts:** Research spec provider validation endpoints
- **Allowed Files:** `pkg/validate/live.go`, `pkg/validate/live_test.go`
- **Forbidden Files:** `pkg/provider/github.go` unless required for shared constants
- **Acceptance Criteria:**
  - HTTP client is injectable.
  - Request uses bearer auth.
  - `200` returns `ok`.
  - `401` or `403` returns `failed`.
  - Timeout returns `skipped`.
  - Network error returns `skipped`.
  - Tests use mock HTTP or local test server, not live GitHub.
  - Raw token is absent from result output.
- **Verification Command:** `go test ./pkg/validate`
- **Dependencies:** Tasks 1 and 5
- **Risk Level:** High
- **Status:** Pending

### Task 7: Implement Live Tavily Validation

- **Objective:** Validate Tavily keys through an opt-in quota-safe usage endpoint.
- **Source Artifacts:** Research spec provider validation endpoints
- **Allowed Files:** `pkg/validate/live.go`, `pkg/validate/live_test.go`
- **Forbidden Files:** `pkg/provider/tavily.go` unless required for shared constants
- **Acceptance Criteria:**
  - HTTP client is injectable.
  - Request uses the correct Tavily auth shape.
  - `200` returns `ok`.
  - `401` or `403` returns `failed`.
  - `429` returns `skipped` or `warning` without failing offline-valid credentials.
  - Timeout returns `skipped`.
  - Tests use mock HTTP or local test server, not live Tavily.
  - Raw token is absent from result output.
- **Verification Command:** `go test ./pkg/validate`
- **Dependencies:** Tasks 1 and 5
- **Risk Level:** High
- **Status:** Pending

### Task 8: Add Validation Formatting

- **Objective:** Render validation reports for humans and JSON consumers.
- **Source Artifacts:** Phase 4 CLI behavior
- **Allowed Files:** `pkg/validate/format.go`, `pkg/validate/format_test.go`
- **Forbidden Files:** `pkg/tui`
- **Acceptance Criteria:**
  - Human output shows provider, key labels, statuses, and messages.
  - JSON output is stable.
  - Unsupported live validators show `skipped`.
  - Output contains no raw Exa, GitHub, Context7, or Tavily fixture credentials.
- **Verification Command:** `go test ./pkg/validate`
- **Dependencies:** Tasks 1 and 3
- **Risk Level:** Medium
- **Status:** Pending

### Task 9: Add `usync validate`

- **Objective:** Expose credential validation through CLI.
- **Source Artifacts:** `cmd/usync/main.go`, Phase 4 spec
- **Allowed Files:** `cmd/usync/main.go`, `cmd/usync/validate_command.go`, `cmd/usync/main_test.go`
- **Forbidden Files:** `pkg/tui`
- **Acceptance Criteria:**
  - `usync validate` requires `--provider`.
  - Unknown provider fails clearly.
  - `--keys-file` parses provider-neutral key files.
  - Offline valid credentials exit `0`.
  - Offline invalid credentials exit `1`.
  - `--json` emits valid JSON only.
  - `--live` invokes live validation and cache.
  - Live timeout/network skip does not exit `1` when offline validation passed.
  - Raw credentials do not appear in stdout or stderr.
- **Verification Command:** `go test ./cmd/usync ./pkg/validate`
- **Dependencies:** Tasks 2, 3, 5, 6, 7, and 8
- **Risk Level:** High
- **Status:** Pending

### Task 10: Integrate Offline Validation Into `usync plan`

- **Objective:** Fail malformed credentials before saved plan creation.
- **Source Artifacts:** `cmd/usync/plan_commands.go`
- **Allowed Files:** `cmd/usync/plan_commands.go`, `cmd/usync/main_test.go`
- **Forbidden Files:** `pkg/app` unless a small helper is cleaner
- **Acceptance Criteria:**
  - `usync plan --provider exa` rejects malformed Exa keys before writing a plan.
  - Plan validation output is redacted.
  - Existing valid plan tests still pass.
  - No live validation runs from `plan`.
- **Verification Command:** `go test ./cmd/usync`
- **Dependencies:** Task 9
- **Risk Level:** Medium
- **Status:** Pending

### Task 11: Integrate Offline Validation Into `usync apply --plan`

- **Objective:** Fail malformed apply-time credentials before mutation.
- **Source Artifacts:** `cmd/usync/apply_command.go`, `pkg/app/plan_apply.go`
- **Allowed Files:** `cmd/usync/apply_command.go`, `cmd/usync/main_test.go`
- **Forbidden Files:** `pkg/config`, `pkg/tui`
- **Acceptance Criteria:**
  - `usync apply --plan` rejects malformed apply-time Exa key before writing.
  - Tests prove target file is unchanged on validation failure.
  - Existing successful apply-plan tests still pass.
  - No live validation runs from `apply`.
- **Verification Command:** `go test ./cmd/usync ./pkg/app`
- **Dependencies:** Task 9
- **Risk Level:** High
- **Status:** Pending

### Task 12: Full Phase 4 Verification

- **Objective:** Confirm validation work does not regress existing plan/apply behavior.
- **Source Artifacts:** All Phase 4 tasks
- **Allowed Files:** No new edits unless tests identify an issue
- **Acceptance Criteria:**
  - `go test ./pkg/provider ./pkg/validate ./pkg/app ./cmd/usync` passes.
  - `go test ./...` passes.
  - `make test` passes.
  - No real network calls occur in tests.
  - No raw credential fixture values appear in validation output, cache files, saved plans, or audit logs.
- **Verification Command:** `go test ./...` and `make test`
- **Dependencies:** Tasks 1-11
- **Risk Level:** Low
- **Status:** Pending

## Dependency Order

Tasks 1-4 establish offline validation. Task 5 adds cache. Tasks 6-8 add live validation and formatting. Task 9 exposes CLI validation. Tasks 10-11 wire validation into plan/apply. Task 12 closes the phase.

## Parallel-Safe Groups

- Task 2 can run after Task 1.
- Task 5 can run after Task 1.
- Tasks 6 and 7 can run after Task 5.
- Task 8 can run after Task 3.
- Tasks 10 and 11 can run after Task 9.

## Implementation Start Gate

Do not start coding Phase 4 until `spec.md`, `plan.md`, and `tasks.md` are accepted.
