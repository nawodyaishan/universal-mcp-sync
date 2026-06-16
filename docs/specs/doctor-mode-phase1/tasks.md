# Doctor Mode Phase 1 Implementation Tasks

## Track Summary

Add client/provider/runtime manifests and a read-only doctor scanner. This phase creates discovery output only; it must not mutate MCP configs or change existing apply behavior.

## Prerequisites

- Phase 0 implementation completed.
- Approved `docs/specs/doctor-mode-phase1/spec.md`.
- Approved `docs/specs/doctor-mode-phase1/plan.md`.

## Task List

### Task 1: Finalize Manifest Data Model

- **Objective:** Define pure metadata types without internal imports.
- **Source Artifacts:** `docs/specs/doctor-mode-phase1/plan.md`
- **Allowed Files:** `pkg/manifest/types.go`, `pkg/manifest/types_test.go`
- **Forbidden Files:** `pkg/config`, `pkg/client`, `pkg/app`, `pkg/tui`
- **Acceptance Criteria:**
  - `pkg/manifest` compiles.
  - Types use manifest-local string aliases, not `config.AppID`, `config.FileKind`, or `client.Capability`.
  - Config format and mutation shape are separate fields.
  - No internal package imports exist in `pkg/manifest`.
  - Public type names and JSON tags are stable enough for doctor output reuse.
- **Verification Command:** `go test ./pkg/manifest`
- **Dependencies:** None
- **Risk Level:** Medium
- **Status:** Pending

### Task 2: Add Client Manifests

- **Objective:** Encode supported client config candidates for macOS and Linux.
- **Source Artifacts:** `docs/research/Doctor Mode, Batch Plan:Apply, and Credential Validation new spec.md`, `pkg/config/paths.go`
- **Allowed Files:** `pkg/manifest/clients.go`, `pkg/manifest/clients_test.go`
- **Forbidden Files:** `pkg/config/paths.go` unless a compile-only constant comparison requires a test helper elsewhere
- **Acceptance Criteria:**
  - Manifests cover all app IDs currently listed in `pkg/config.AppOrder`.
  - macOS and Linux path differences are represented.
  - Legacy candidates are represented for Antigravity/Gemini and Windsurf where appropriate.
  - Deprecated candidates have `ReplacedBy` or a clear deprecation reason.
  - Project/workspace candidates are marked opt-in metadata, not selected by default.
  - Antigravity disputed paths are implemented only after verification is documented in the PR.
- **Verification Command:** `go test ./pkg/manifest`
- **Dependencies:** Task 1
- **Risk Level:** High
- **Status:** Pending

### Task 3: Add Provider And Runtime Metadata

- **Objective:** Add user guidance data for credentials, key URLs, docs URLs, and runtime prerequisites.
- **Source Artifacts:** `pkg/provider/registry.go`, provider implementations, research doc provider references
- **Allowed Files:** `pkg/manifest/providers.go`, `pkg/manifest/runtimes.go`, `pkg/manifest/providers_test.go`
- **Forbidden Files:** `pkg/provider/*` unless a test helper outside manifest is needed later
- **Acceptance Criteria:**
  - Provider metadata covers Exa, GitHub, Context7, Tavily, Playwright, Kubernetes, and Terraform.
  - Credential acquisition data includes env var/key name, required flag, format hint, acquisition URL, and docs URL where applicable.
  - Runtime metadata covers `node`, `npx`, `docker`, and relevant CLI tools.
  - No secret values appear in metadata or tests.
- **Verification Command:** `go test ./pkg/manifest`
- **Dependencies:** Task 1
- **Risk Level:** Medium
- **Status:** Pending

### Task 4: Add Manifest Helpers And Invariant Tests

- **Objective:** Make manifest metadata queryable and hard to regress.
- **Source Artifacts:** `docs/specs/doctor-mode-phase1/plan.md`
- **Allowed Files:** `pkg/manifest/paths.go`, `pkg/manifest/manifest_test.go`
- **Forbidden Files:** Production packages outside `pkg/manifest`
- **Acceptance Criteria:**
  - `AllClients`, `ClientByID`, `ForPlatform`, `ExpandPath`, `AllProviders`, `ProviderByID`, and `AllRuntimeRequirements` exist.
  - `ExpandPath` supports `{{.Home}}` and `{{.Workspace}}`.
  - Tests cover home paths with spaces.
  - Tests prevent duplicate candidate labels per client.
  - Tests prevent unsupported platforms and missing candidate fields.
  - Tests verify no internal imports by scanning package files.
- **Verification Command:** `go test ./pkg/manifest`
- **Dependencies:** Tasks 1-3
- **Risk Level:** Medium
- **Status:** Pending

### Task 5: Add Doctor Report Types And Scanner Skeleton

- **Objective:** Establish the read-only doctor package and report schema.
- **Source Artifacts:** `docs/specs/doctor-mode-phase1/spec.md`
- **Allowed Files:** `pkg/doctor/types.go`, `pkg/doctor/doctor.go`, `pkg/doctor/doctor_test.go`
- **Forbidden Files:** `pkg/app`, `pkg/tui`, `pkg/config/files.go`
- **Acceptance Criteria:**
  - `doctor.New(options)` and `Doctor.Scan(ctx)` exist.
  - Report types use manifest string IDs and deterministic field ordering.
  - `Scan` enumerates platform-filtered manifests and expanded candidate paths.
  - Initial tests prove scan does not write to fixture directories.
  - `pkg/doctor` does not import `pkg/app` or `pkg/tui`.
- **Verification Command:** `go test ./pkg/doctor`
- **Dependencies:** Task 4
- **Risk Level:** Medium
- **Status:** Pending

### Task 6: Implement Candidate File Scanning

- **Objective:** Read existing candidate files and report file-level health.
- **Source Artifacts:** `docs/specs/doctor-mode-phase1/plan.md`
- **Allowed Files:** `pkg/doctor/client_scan.go`, `pkg/doctor/parse.go`, `pkg/doctor/testdata/homes/*`
- **Forbidden Files:** `pkg/config/json_update.go`, `pkg/config/toml_update.go`
- **Acceptance Criteria:**
  - Scanner uses `os.Lstat` and reports symlinks separately from regular files.
  - Existing files are parsed for JSON, JSONC, and Codex TOML health.
  - Expected root key shape is reported.
  - Provider IDs are detected only under the expected MCP root.
  - Missing candidates are reported without error.
  - Malformed files report parse errors and do not panic.
- **Verification Command:** `go test ./pkg/doctor`
- **Dependencies:** Task 5
- **Risk Level:** High
- **Status:** Pending

### Task 7: Implement Confidence, Migration Hints, And Warnings

- **Objective:** Turn raw candidate findings into useful client-level status.
- **Source Artifacts:** new spec §4, Phase 1 requirements
- **Allowed Files:** `pkg/doctor/client_scan.go`, `pkg/doctor/providers.go`, `pkg/doctor/doctor_test.go`
- **Forbidden Files:** `pkg/app`, `pkg/tui`
- **Acceptance Criteria:**
  - Client confidence is one of `high`, `medium`, `low`, or `conflict`.
  - Legacy-only configs produce migration hints.
  - Current+legacy duplicates produce conflict or warning findings.
  - Gemini CLI sunset warning is emitted when applicable.
  - Git-warning candidates such as workspace/project files are surfaced as warnings.
- **Verification Command:** `go test ./pkg/doctor`
- **Dependencies:** Task 6
- **Risk Level:** Medium
- **Status:** Pending

### Task 8: Implement Runtime Checks

- **Objective:** Report local runtime availability without making doctor brittle.
- **Source Artifacts:** `pkg/manifest/runtimes.go`
- **Allowed Files:** `pkg/doctor/runtime.go`, `pkg/doctor/runtime_test.go`
- **Forbidden Files:** Provider implementations
- **Acceptance Criteria:**
  - Runtime checks support disabled mode for deterministic tests.
  - Runtime commands use a short timeout.
  - Missing commands report unavailable instead of failing the scan.
  - Runtime test can use fake command lookup/runner injection.
  - Provider/client reasons from manifest are preserved in findings.
- **Verification Command:** `go test ./pkg/doctor`
- **Dependencies:** Tasks 3 and 5
- **Risk Level:** Medium
- **Status:** Pending

### Task 9: Add Doctor JSON And Human Formatting

- **Objective:** Provide stable machine output and concise human output.
- **Source Artifacts:** `docs/specs/doctor-mode-phase1/spec.md`
- **Allowed Files:** `pkg/doctor/report.go`, `pkg/doctor/report_test.go`
- **Forbidden Files:** `pkg/tui`
- **Acceptance Criteria:**
  - JSON output is deterministic with fixed `Now`.
  - Human output includes detected clients, issues, runtime blockers, and migration hints.
  - Output is redacted and does not include raw config values.
  - Tests prove repeated JSON render on the same report is byte-identical.
- **Verification Command:** `go test ./pkg/doctor`
- **Dependencies:** Tasks 6-8
- **Risk Level:** Medium
- **Status:** Pending

### Task 10: Add `usync doctor` CLI

- **Objective:** Expose read-only doctor scan through the CLI without changing existing flows.
- **Source Artifacts:** `cmd/usync/main.go`, `docs/specs/doctor-mode-phase1/plan.md`
- **Allowed Files:** `cmd/usync/main.go`, `cmd/usync/main_test.go`
- **Forbidden Files:** `pkg/tui`, `pkg/app/app.go`
- **Acceptance Criteria:**
  - `usync doctor` prints human-readable output.
  - `usync doctor --json` prints deterministic JSON.
  - `--home-dir`, `--workspace`, and `--no-runtimes` are supported.
  - Exit code `0`, `1`, and `2` semantics are tested.
  - Existing no-subcommand TUI behavior is unchanged.
  - Existing `sync`, `--dry-run`, and `--apply` behavior is unchanged.
- **Verification Command:** `go test ./cmd/usync`
- **Dependencies:** Task 9
- **Risk Level:** Medium
- **Status:** Pending

### Task 11: Add Full Phase 1 Verification

- **Objective:** Confirm Phase 1 planning gates are met before implementation closes.
- **Source Artifacts:** All Phase 1 tasks
- **Allowed Files:** No new edits unless tests identify an issue
- **Acceptance Criteria:**
  - `go test ./pkg/manifest ./pkg/doctor ./cmd/usync` passes.
  - `go test ./...` passes.
  - `make test` passes.
  - `rg -n 'github.com/nawodyaishan/universal-mcp-sync/pkg/' pkg/manifest` returns no matches.
  - `rg -n 'pkg/app|pkg/tui' pkg/doctor` returns no matches.
  - Doctor scan on fixture homes creates no files.
- **Verification Command:** `go test ./...` and `make test`
- **Dependencies:** Tasks 1-10
- **Risk Level:** Low
- **Status:** Pending

## Dependency Order

Tasks 1-4 form PR 1a. Tasks 5-10 form PR 1b. Task 11 closes Phase 1.

Task 3 can run in parallel with Task 2 after Task 1. Task 8 can run in parallel with Tasks 6-7 after the scanner skeleton exists.

## Parallel-Safe Groups

- Task 2 and Task 3 after Task 1.
- Task 6 and Task 8 after Task 5, if runtime runner injection is designed in Task 5.
- Task 9 can begin once report types from Task 5 stabilize.

## Implementation Start Gate

Do not start coding Phase 1 until `spec.md`, `plan.md`, and `tasks.md` are accepted.
