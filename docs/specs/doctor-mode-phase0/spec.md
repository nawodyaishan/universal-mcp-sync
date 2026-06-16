# Doctor Mode Phase 0: Output Hygiene and Write Safety

## Problem Statement

The upcoming doctor, plan, and apply workflow depends on deterministic command output and safe local config writes. The current codebase still has debug `fmt.Printf` calls inside `pkg/config/json_update.go`, which can leak implementation noise into CLI output and break future `doctor --json` or `plan --json` consumers.

The current write path already performs atomic writes, creates backups, enforces private file permissions, and rolls back on apply failure. It does not yet guard against two `usync` processes writing the same config file at the same time.

Phase 0 prepares the codebase for doctor mode without adding doctor, plan files, credential validation, migration, or TUI changes.

## Goals

- Remove unintended stdout output from library packages.
- Preserve current apply behavior and existing tests.
- Add narrow per-file write locking around config writes.
- Keep the current `WriteWithBackup` public behavior stable.
- Add focused tests for lock acquisition, lock cleanup, and concurrent writes.
- Add a lightweight audit package foundation only if it can be introduced without changing apply behavior.

## Non-Goals

- Implementing `usync doctor`.
- Implementing manifests.
- Implementing saved plan files.
- Changing legacy `usync sync --apply` behavior.
- Changing config file formats or generated MCP payloads.
- Rewriting the apply engine.
- Adding live credential validation.
- Rewriting the TUI.

## Users or Actors

- Contributors preparing the repo for doctor-mode development.
- CLI users who rely on clean `--dry-run` and `--apply` output.
- Future automation that will consume deterministic doctor/plan JSON output.

## Functional Requirements

- **FR-1:** No production code in `pkg/` may write directly to stdout using `fmt.Printf`.
- **FR-2:** `pkg/config/json_update.go` must remove debug output without changing generated JSON behavior.
- **FR-3:** `config.WriteWithBackup` must acquire a per-target lock before reading, backing up, or writing the target file.
- **FR-4:** Lock acquisition must use a sibling lock file named `<target>.lock`.
- **FR-5:** Lock acquisition must use exclusive creation semantics (`O_CREATE|O_EXCL`) so two writers cannot both hold the lock.
- **FR-6:** Lock files must be removed on success and on write error.
- **FR-7:** If a lock cannot be acquired after bounded retries, the write must fail with a clear lock error.
- **FR-8:** Atomic write, backup, rollback, `0600` file permissions, and `0700` directory permissions must remain unchanged.
- **FR-9:** Existing `app.Manager` callers must not need signature changes.
- **FR-10:** Any new audit package must not be wired into apply in this phase unless separately approved.

## Acceptance Criteria

- `go test ./...` passes.
- `make test` passes.
- `rg -n 'fmt\.Printf' pkg` returns no production-code matches.
- `make dry-run KEYS_FILE=<fixture>` output contains no `DEBUG` substring.
- A concurrent-write test proves two goroutines writing the same file do not corrupt the file.
- A lock cleanup test proves `.lock` is removed after a failed write path.
- Existing config golden fixtures remain unchanged except where a test explicitly targets lock behavior.

## Success Criteria

- Phase 1 can add `usync doctor --json` without inheriting noisy library stdout.
- The existing apply engine keeps its current behavior while gaining per-file write safety.
- The Phase 0 PR is small enough to review independently from doctor/manifest work.

## Edge Cases

- Stale lock file exists before write starts.
- Lock file parent directory does not exist yet.
- Backup write fails after lock acquisition.
- Target write fails after backup succeeds.
- Rollback still works after locked writes.
- Existing file permission is not private before write.
- Two writes happen within the same timestamp second and would otherwise generate the same backup path.

## Data Sensitivity and Compliance Notes

- No secrets should be added to logs, errors, lock files, or audit entries.
- Lock files must not contain config content.
- Backup files still contain original config data and must remain `0600`.

## Assumptions

- A sibling lock file is sufficient for local machine concurrency.
- Cross-process locking does not need OS-specific advisory locking in Phase 0.
- Stale lock recovery can be manual in Phase 0, with a clear error message.
- Audit logging can be introduced as a package foundation before being integrated into apply.

## Open Questions

- Should Phase 0 include `pkg/audit` implementation, or should audit be deferred until apply-from-plan exists?
- Should lock retry timing be configurable, or fixed for now?
- Should a stale lock older than a threshold be auto-removed, or should the user remove it manually?

## Human Approval Status

Approved to plan. Implementation approval pending.
