# Doctor Mode Phase 3: Apply Saved Plan

## Problem Statement

Phase 2 introduced saved JSON plans, but those plans are review artifacts only. Users can generate and inspect a plan, but they still cannot execute that exact saved plan through the new `doctor -> plan -> apply` workflow.

Phase 3 adds `usync apply --plan <path>` and the app-layer machinery to execute saved plans safely. It must preserve the existing legacy `sync --apply` behavior while adding a new plan-file-driven apply path with stale-file checks, approval gates, rollback, and verification.

The current saved plan schema is intentionally redacted. Phase 3 must enrich it only with non-secret execution metadata needed to reconstruct writes; credentials must still be supplied at apply time and must never be stored in the plan.

## Goals

- Add plan-file-driven apply without breaking legacy `Manager.Apply(ExecutionPlan)`.
- Re-verify saved plan schema, expiry, target hashes, path safety, and symlink status before writing.
- Require credentials at apply time through env vars, `--keys`, or `--keys-file`.
- Reconstruct intended config content from provider ID, credential refs, target metadata, and saved operation metadata.
- Execute file-backed operations transactionally: preflight all, write sequentially with backups, rollback prior writes on failure, verify.
- Run CLI-managed operations after file writes succeed.
- Add `usync apply --plan <path> --dry-run`.
- Add `--auto-approve` for non-interactive apply gates.
- Write an audit log entry after apply attempts without storing secrets.
- Preserve existing `usync`, `sync --dry-run`, and legacy `sync --apply`.

## Non-Goals

- Do not add live credential validation.
- Do not rewrite the TUI.
- Do not implement full doctor dashboard UX.
- Do not implement MCP server or gateway mode.
- Do not add team/remote state.
- Do not store raw credential values in plan files, audit logs, or command output.
- Do not require saved-plan apply for the legacy `sync --apply` command yet.

## Users or Actors

- CLI users who want Terraform-like `plan -> apply` behavior.
- Automation that wants to review a saved JSON plan, then apply it later with explicit credentials.
- Future TUI flows that will call the same saved-plan apply API after preview approval.

## Functional Requirements

- **FR-1:** `usync apply --plan <path>` must load a saved plan and execute it.
- **FR-2:** `usync apply --plan <path> --dry-run` must perform all preflight checks and print a preview without writing files or running external CLIs.
- **FR-3:** Apply must reject unknown saved plan schema versions.
- **FR-4:** Apply must reject expired plans unless `--force-stale` is supplied.
- **FR-5:** Apply must re-compute SHA-256 for every file-backed operation and reject mismatches.
- **FR-6:** Apply must reject path escapes outside the configured home directory.
- **FR-7:** Apply must use `os.Lstat` for symlink checks and must not remove or replace symlinks.
- **FR-8:** Symlink writes must require explicit approval unless `--auto-approve` is supplied.
- **FR-9:** Project/workspace writes and first-time creates must require explicit approval unless `--auto-approve` is supplied.
- **FR-10:** File-backed operations must use the existing locked backup write path.
- **FR-11:** If file operation N fails, prior successful file operations must be rolled back.
- **FR-12:** CLI-managed operations must run only after all file-backed operations complete and verify.
- **FR-13:** CLI-managed failures must be reported separately and must not attempt to rollback file writes that already completed and verified.
- **FR-14:** Apply output must be redacted by default.
- **FR-15:** Audit logging must write JSONL entries with command, plan ID, targets, touched files, exit code, and redacted error.
- **FR-16:** Existing legacy apply tests must continue to pass unchanged.

## Saved Plan Schema Additions

Phase 2 `SavedPlan` schema version 1 is not enough for exact apply. Phase 3 should introduce schema version 2 with non-secret execution fields.

Add to `PlanOperation`:

- `file_kind`: existing config mutation kind, such as `mcpServers`, `namedServer`, `codexTOML`, or `claudeCodeCLI`.
- `provider_id`: provider to apply for this operation.
- `credential_ref`: credential key/env var label to resolve at apply time.
- `will_create`: whether the plan expected the target file to be missing.
- `backup_path`: expected backup path, optional preview field.

Do not add raw provider config, URL values, headers, env values, or command args containing secrets.

Schema handling:

- `apply --plan` accepts only schema version 2.
- `show` can continue reading schema version 1 and 2 for display if practical.
- `plan` should start writing schema version 2 once Phase 3 lands.

## Acceptance Criteria

- `usync apply --plan <plan> --dry-run` prints preflight status and writes nothing.
- `usync apply --plan <plan> --auto-approve --keys-file <keys>` applies file-backed operations and verifies them.
- Modifying a target file between plan and apply causes a checksum mismatch error and no writes.
- A synthetic failure on operation 2 rolls back operation 1 and never writes operation 3.
- Symlink target apply writes to the resolved target only after approval or `--auto-approve`.
- CLI-managed operations run after successful file operations.
- Audit JSONL is written after apply and contains no raw credential values.
- Existing `go test ./pkg/app ./cmd/usync` passes.
- Existing `go test ./...` and `make test` pass.

## Success Criteria

- The repo has a working saved-plan apply path that is safer than legacy immediate apply.
- Phase 4 credential validation can plug into the same credential resolution step without changing apply semantics.
- Future TUI work can call saved-plan apply APIs rather than duplicating write behavior.

## Edge Cases

- Plan file is expired.
- Plan file schema is v1.
- Target file changed after plan creation.
- Target file was deleted after plan creation.
- Target file was created after plan expected it missing.
- Target path is now a symlink but was not during plan.
- Symlink target resolves outside home.
- One target is locked by another process.
- Backup write succeeds but target write fails.
- CLI binary is missing at apply time.
- Credentials supplied at apply time do not match the saved credential refs.
- Audit directory cannot be created.

## Data Sensitivity and Compliance Notes

- Plan files and audit logs must never include raw keys.
- Apply errors and logs must pass through redaction.
- Dry-run output must be redacted exactly like real apply output.
- Backup files may contain original local config and must remain `0600`.
- Audit logs should be `0600` and parent directory `0700`.

## Assumptions

- Phase 2 saved-plan foundation is present.
- Phase 1b doctor integration may still be incomplete; Phase 3 can apply explicitly targeted saved plans first.
- Provider credential values can be supplied again at apply time.
- The existing config mutation helpers remain the implementation source for intended file content.

## Open Questions

- Should schema version 1 plans be auto-upgraded for apply, or should users re-run `usync plan`?
- Should CLI-managed operations be included in the first Phase 3 PR or delayed until file-backed apply is stable?
- Should `--force-stale` bypass only expiry or also checksum mismatch? Recommendation: expiry only.
- Should audit failure fail the whole apply or emit a warning after successful writes? Recommendation: warning after successful writes.

## Human Approval Status

Approved to plan. Implementation approval pending.
