# Doctor Mode Phase 0 Implementation Plan

## Summary

Phase 0 is a narrow preparation PR for the larger doctor-mode roadmap. It removes debug stdout from config mutation code and adds per-target write locking around the existing backup/write path. It should not alter provider generation, target detection, TUI flow, or CLI behavior.

## Inputs Reviewed

- `docs/research/Doctor Mode, Batch Plan:Apply, and Credential Validation new spec.md`
- `docs/research/doctor-mode-batch-mcp-setup-research.md`
- `pkg/config/json_update.go`
- `pkg/config/files.go`
- `pkg/config/files_test.go`
- `pkg/app/app.go`
- existing `docs/specs/*/{spec,plan,tasks}.md` SDD structure

## Architecture Approach

### 1. Output Hygiene

Remove debug statements from `pkg/config/json_update.go`:

- `UpdateMCPServersJSON` should not print provider ID, root key, URL field, or server keys.
- `marshalJSON` should not print when the root is empty.
- The helper `getKeys` should be removed if it becomes unused.

No replacement logging is needed in `pkg/config`. Higher-level diagnostics should go through `app.Manager.Logger` only when needed.

### 2. Write Locking

Keep the public write API stable:

```go
func WriteWithBackup(path string, data []byte, now time.Time) (WriteOutcome, error)
```

Add an internal lock helper:

```go
var ErrFileLocked = errors.New("config file is locked")

func acquireFileLock(path string) (func(), error)
```

`WriteWithBackup` should call `acquireFileLock(path)` after ensuring the parent directory exists and before reading existing content.

The lock path is:

```text
<target-path>.lock
```

The lock helper should:

- create the parent directory if needed
- attempt `os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)`
- close the lock file immediately after successful creation
- retry a small fixed number of times if the lock exists
- return `ErrFileLocked` wrapped with the lock path after retries
- return an unlock closure that removes the lock file

The unlock closure should be called with `defer` inside `WriteWithBackup`.

### 3. Audit Package Decision

The larger spec asks for `pkg/audit` in Phase 0. For the first implementation PR, audit should be treated as optional:

- Safe option: create `pkg/audit` with `Append(path string, entry Entry) error` and tests, but do not wire it into `app.Apply`.
- More conservative option: defer `pkg/audit` until Phase 3, because audit only becomes meaningful when plan/apply has stable plan IDs.

Recommendation: defer audit integration; optionally scaffold the package only if it does not expand the PR.

### 4. No CLI/TUI Behavior Change

The existing `cmd/usync` behavior remains unchanged:

- `usync` still opens the TUI.
- `--dry-run` still prints the current plan format.
- `--apply` still works through the legacy flow.
- `sync` alias remains unchanged.

## Affected Modules

- `pkg/config/json_update.go`
- `pkg/config/files.go`
- `pkg/config/files_test.go`
- optional: `pkg/audit/audit.go`
- optional: `pkg/audit/audit_test.go`

## Dependency Changes

No new external dependencies.

## Testing Strategy

- Unit tests in `pkg/config`.
- Existing full suite via `make test`.
- CLI output hygiene check using a fixture key file or existing test helper if available.

Suggested tests:

- `TestWriteWithBackupUsesPrivatePermissionsAndRollbackRestoresOriginal` remains valid.
- `TestWriteWithBackupConcurrentWritersDoNotCorruptFile`.
- `TestWriteWithBackupReturnsLockedErrorWhenLockPersists`.
- `TestWriteWithBackupRemovesLockAfterWriteError`.
- `TestJSONUpdateDoesNotWriteStdout` if practical with stdout capture, otherwise enforce through grep in CI/tasks.

## Risks and Mitigations

- **Risk:** Lock files can become stale after process crash.
  **Mitigation:** Phase 0 returns a clear error with the lock path. Auto-stale cleanup is deferred.

- **Risk:** Tests for concurrent writes become flaky.
  **Mitigation:** Use a deliberately held lock file for deterministic lock-failure tests; use concurrency only to prove no corruption.

- **Risk:** Introducing audit logging now expands scope.
  **Mitigation:** Keep audit unwired or defer it until apply-from-plan.

- **Risk:** Changing `WriteWithBackup` affects all apply paths.
  **Mitigation:** Preserve the function signature and all existing behavior; run full tests.

## Human Architecture Approval Status

Pending approval to implement.
