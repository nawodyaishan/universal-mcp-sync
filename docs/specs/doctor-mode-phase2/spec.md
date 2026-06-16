# Doctor Mode Phase 2: Saved Batch Plan

## Problem Statement

The current non-interactive flow can print a redacted dry-run plan or apply immediately, but it does not produce a durable artifact that can be reviewed, shared with automation, or applied later. The upcoming doctor-first workflow needs a saved plan boundary between discovery and mutation.

Phase 2 adds a saved batch plan model and CLI planning commands. It should convert doctor findings plus explicit user target selections into a redacted, file-backed plan that records intended operations and current file hashes. It must not apply the plan yet.

## Goals

- Add a versioned saved plan model in `pkg/app`.
- Add deterministic plan JSON output with no credential values.
- Add plan save/load/show/list/clean helpers.
- Add `usync plan` and `usync show` CLI commands.
- Require explicit target selection through `--targets` or `--all-detected` for the new plan flow.
- Preserve existing `usync`, `sync`, `--dry-run`, and `--apply` behavior.
- Capture current file SHA-256 for each file-backed operation so Phase 3 apply can refuse stale plans.
- Reuse existing provider generation, client adaptation, redaction, and config safety behavior where possible.

## Non-Goals

- Do not implement `usync apply --plan`.
- Do not change legacy `sync --apply`.
- Do not remove the existing in-memory `ExecutionPlan`.
- Do not rewrite the TUI.
- Do not add live credential validation.
- Do not add audit logging integration.
- Do not add MCP server or gateway mode.
- Do not write client config files.

## Users or Actors

- CLI users who want to preview and persist a batch MCP setup plan.
- Automation that wants a stable JSON plan artifact before apply exists.
- Future Phase 3 apply logic that needs file hashes, redacted operations, and explicit selections.

## Functional Requirements

- **FR-1:** Add a saved plan type with schema version, plan ID, created/expiry timestamps, usync version, provider ID, credential refs, operations, warnings, and doctor summary.
- **FR-2:** Plan files must never contain credential values.
- **FR-3:** Plan files must be written with `0600` permissions.
- **FR-4:** The canonical Phase 2 plan file should be JSON, not gob. A binary plan format can be revisited only if JSON cannot preserve required fidelity.
- **FR-5:** Saved plan JSON must be deterministic for stable test fixtures when `Now` and plan ID generation are injected.
- **FR-6:** Each file-backed operation must include current SHA-256 of the target file contents or a sentinel for a missing file.
- **FR-7:** Each operation must include action: `create`, `update`, `skip`, or `conflict`.
- **FR-8:** Each operation must include target ID, target name, manager kind, path or CLI command summary, transport, redacted description, symlink status, and warnings.
- **FR-9:** `usync plan` must require `--provider` and either `--targets` or `--all-detected`.
- **FR-10:** `--all-detected` must select only doctor findings with acceptable confidence and must skip conflicts.
- **FR-11:** `usync plan` must support `--out`; otherwise it writes to the plan cache directory.
- **FR-12:** Plan cache directory resolution must support `$USYNC_PLAN_DIR`, then `$XDG_CACHE_HOME/usync/plans`, then `~/.cache/usync/plans`.
- **FR-13:** `usync show <plan>` must render human output by default and JSON with `--json`.
- **FR-14:** `usync plan list` and `usync plan clean` must operate only inside the plan cache directory.
- **FR-15:** Loading a plan must validate schema version, JSON shape, file permissions, and plan path safety.
- **FR-16:** Plan path safety must prevent accidentally loading plans outside the configured home or plan cache unless explicitly allowed by a future flag.
- **FR-17:** Existing CLI/TUI flows must continue to pass unchanged.

## Acceptance Criteria

- `pkg/app` has saved plan types and tests.
- `usync plan --provider exa --targets codex-cli,vscode --keys-file <fixture> --home-dir <fixture> --out <path>` writes a `0600` JSON plan.
- Plan JSON contains no full Exa, GitHub, Context7, Tavily, or token-like credential values.
- `usync show <plan> --json` returns stable JSON.
- `usync show <plan>` returns concise human-readable output.
- `usync plan` without `--targets` or `--all-detected` exits with a helpful error.
- `--all-detected` skips `low` and `conflict` findings and records skipped targets in warnings.
- Existing `sync --dry-run` and `sync --apply` behavior remains unchanged.
- `go test ./pkg/app ./cmd/usync` passes.
- `go test ./...` and `make test` pass before implementation is marked complete.

## Success Criteria

- Phase 3 can implement `usync apply --plan` without reworking plan persistence.
- Users can inspect exactly what would be changed before any write occurs.
- Plan artifacts are safe to archive in local automation logs because they are redacted and permission-restricted.

## Edge Cases

- Target file does not exist at plan time.
- Target file exists but is unreadable.
- Target path points to a symlink.
- Target path resolves outside home.
- Doctor report contains conflicts.
- Provider cannot generate config because credentials are missing or invalid.
- Multiple credentials are provided for one provider.
- Plan output path already exists.
- Plan cache directory does not exist.
- Plan cache directory is not writable.
- Plan file is stale or schema-incompatible.
- Plan file has permissive permissions.

## Data Sensitivity and Compliance Notes

- Credential values must not be persisted in plan files, sidecars, logs, errors, or test fixtures.
- Redacted credential labels are allowed.
- CLI command summaries must redact args and env values.
- Plan files still describe local paths and should be `0600`.
- Backup files are not created in Phase 2.

## Assumptions

- Phase 1b provides `pkg/doctor` and `usync doctor` output before Phase 2 implementation begins.
- `pkg/manifest` remains the metadata source for target IDs, provider links, and runtime hints.
- The existing `pkg/app.ExecutionPlan` remains the legacy in-memory apply plan.
- Saved plan v1 can store enough redacted metadata to reconstruct Phase 3 apply using existing provider generation and credential refs, without storing secrets.

## Open Questions

- Should `usync plan` accept credentials only from env vars for saved plans, or also from `--keys-file` for compatibility?
- Should plan files outside the cache directory be allowed when explicitly passed through `--out`?
- Should `plan list` show all plans or only unexpired plans by default?
- Should `show` warn on stale current SHA, or should stale checks wait until `apply --plan` in Phase 3?
- How much of Phase 2 should depend on `pkg/doctor` concrete types versus a small app-level selection DTO?

## Human Approval Status

Approved to plan. Implementation approval pending.
