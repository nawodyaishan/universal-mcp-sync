# Doctor Mode Phase 3 Implementation Plan

## Summary

Phase 3 adds saved-plan apply. It should ship as two or three PRs:

- **PR 3a:** Schema v2 and apply preflight/dry-run.
- **PR 3b:** File-backed apply with rollback and verification.
- **PR 3c:** CLI-managed operations and audit logging.

The first implementation should favor correctness over breadth. File-backed apply should become solid before CLI adapters or audit integration expand the surface.

## Inputs Reviewed

- `docs/research/Doctor Mode, Batch Plan:Apply, and Credential Validation new spec.md`
- `docs/specs/doctor-mode-phase2/spec.md`
- `docs/specs/doctor-mode-phase2/plan.md`
- `docs/specs/doctor-mode-phase2/tasks.md`
- `pkg/app/app.go`
- `pkg/app/plan_v2.go`
- `pkg/app/plan_store.go`
- `cmd/usync/plan_commands.go`
- `pkg/config/files.go`

## Key Design Corrections From The Research Spec

The research spec assumes a saved plan already contains enough information to apply directly. The current Phase 2 JSON plan is redacted and intentionally does not contain provider URLs, headers, env values, or raw CLI arguments. That is correct for security, but Phase 3 needs a small schema v2 expansion.

Apply should reconstruct intended config content at apply time using:

- saved target metadata
- provider ID
- file kind/mutation kind
- credential refs
- credentials supplied at apply time
- existing provider and client adaptation code

It should not persist raw provider config in the plan.

## Architecture Approach

### 1. Schema v2

Update saved plan constants and types:

```go
const SavedPlanSchemaVersion = 2

type PlanOperation struct {
    TargetID      string `json:"target_id"`
    TargetName    string `json:"target_name"`
    Action        string `json:"action"`
    ProviderID    string `json:"provider_id"`
    CredentialRef string `json:"credential_ref"`
    FileKind      string `json:"file_kind,omitempty"`
    FilePath      string `json:"file_path,omitempty"`
    BackupPath    string `json:"backup_path,omitempty"`
    CurrentSHA    string `json:"current_sha,omitempty"`
    Transport     string `json:"transport"`
    Manager       string `json:"manager"`
    CLICommand    []string `json:"cli_command,omitempty"`
    Redacted      string `json:"redacted"`
    IsSymlink     bool `json:"is_symlink"`
    ResolvedPath  string `json:"resolved_path,omitempty"`
    WillCreate    bool `json:"will_create"`
    Warnings      []string `json:"warnings,omitempty"`
}
```

Schema v2 plan creation should remain redacted. `CredentialRef` should be an env/key name or stable label, not a value.

### 2. Apply API

Add saved-plan apply APIs without replacing legacy apply:

```go
type SavedPlanApplyOptions struct {
    Credentials map[string]string
    AutoApprove bool
    DryRun bool
    ForceStale bool
    Approver Approver
}

type Approver interface {
    Confirm(prompt ApprovalPrompt) (bool, error)
}

func (m *Manager) ApplySavedPlan(plan SavedPlan, opts SavedPlanApplyOptions) (ApplyResult, error)
func (m *Manager) PreflightSavedPlan(plan SavedPlan, opts SavedPlanApplyOptions) (SavedPlanPreflight, error)
```

Use a default CLI approver in `cmd/usync` only. `pkg/app` should accept an approver interface so tests can auto-approve or deny without stdin prompts.

### 3. Preflight

Preflight should run before any write or CLI command:

1. Validate schema version.
2. Validate expiry unless `ForceStale`.
3. Validate credentials are present for every credential ref.
4. Validate target paths are inside home.
5. Recompute SHA for each file-backed operation.
6. Detect symlink changes with `Lstat`.
7. Check file/directory writability.
8. Collect approval prompts.

If any critical preflight fails, apply returns before writes.

`--dry-run` should run this same preflight and print the result.

### 4. Reconstructing File Operations

Do not duplicate JSON/TOML mutation code. Convert saved plan operations into existing `Operation` values and reuse:

- `Manager.prepareFileOperation`
- `Manager.WriteConfig`
- `config.RollbackWrite`
- `verify.VerifyProviderFile`

The conversion needs:

- `TargetID` -> `config.AppID`
- `FileKind` -> `config.FileKind`
- `ProviderID` -> `provider.MCPProvider`
- credentials -> `provider.MCPConfig`
- `client.Adapt` for target-specific transport shape

If a saved operation cannot be converted, fail preflight.

### 5. File Apply Ordering

File-backed operations:

1. Prepare all intended contents.
2. Write sequentially using existing `WriteConfig`.
3. Collect backups.
4. On first failure, rollback prior write outcomes in reverse order.
5. Verify written files.

This matches the current legacy `Manager.Apply` behavior and should reuse the same helper logic where possible.

Do not attempt all-lock acquisition in the first Phase 3 PR unless `pkg/config` exposes a safe batch lock API. Current `WriteWithBackup` already has per-file locks from Phase 0.

### 6. CLI-Managed Operations

Delay full CLI adapters until file apply is stable, but plan for:

- Claude Code via `claude mcp remove/add` or `add-json`.
- Codex CLI via `codex mcp add`, only after behavior is tested.

Rules:

- Run CLI operations after file-backed operations verify.
- Missing CLI is a warning or apply error depending on whether the plan operation action is required.
- Do not attempt full rollback for CLI-managed operations in Phase 3.
- Redact all command output.

### 7. Audit Logging

Add `pkg/audit` only when apply behavior is stable:

```go
type Entry struct {
    Timestamp    time.Time `json:"ts"`
    Command      string    `json:"cmd"`
    PlanID       string    `json:"plan_id,omitempty"`
    Targets      []string  `json:"targets,omitempty"`
    FilesTouched []string  `json:"files,omitempty"`
    ExitCode     int       `json:"exit_code"`
    Error        string    `json:"error,omitempty"`
}
```

Path:

```text
~/.usync/audit.log
```

Permissions:

- parent directory `0700`
- audit log `0600`

Audit failures after successful writes should become warnings, not rollback triggers.

### 8. CLI Integration

Add:

```text
usync apply --plan ./plan.json --keys-file ./keys.txt
usync apply --plan ./plan.json --dry-run --keys-file ./keys.txt
usync apply --plan ./plan.json --auto-approve --keys-file ./keys.txt
usync apply --plan ./plan.json --force-stale --keys-file ./keys.txt
```

Keep legacy behavior:

```text
usync --apply --keys-file ./keys.txt
usync sync --apply --keys-file ./keys.txt
```

The subcommand `apply` is new and distinct from legacy `--apply`.

### 9. Dependency Rules

- `pkg/app` may call `pkg/config`, `pkg/client`, `pkg/provider`, `pkg/redact`, `pkg/verify`, and optional `pkg/audit`.
- `pkg/audit` must not import `pkg/app`.
- `cmd/usync` owns CLI flags, credential loading, and interactive approval prompts.
- `pkg/tui` remains unchanged in Phase 3.

## Affected Modules

- `pkg/app/plan_v2.go`
- `pkg/app/plan_apply.go` new
- `pkg/app/plan_apply_test.go` new
- `pkg/app/plan_store.go`
- `cmd/usync/main.go`
- `cmd/usync/plan_commands.go`
- `cmd/usync/main_test.go`
- optional: `pkg/audit/audit.go`
- optional: `pkg/audit/audit_test.go`
- `docs/specs/doctor-mode-phase3/` planning docs

## Dependency Changes

No external dependencies are required.

## Testing Strategy

App tests:

- preflight rejects schema v1 for apply
- preflight rejects expired plan without `ForceStale`
- preflight accepts expired plan with `ForceStale`
- preflight rejects checksum mismatch
- preflight rejects missing credentials
- dry-run performs no writes
- file apply writes expected config and verifies
- synthetic second write failure rolls back first write
- symlink write uses resolved target and preserves symlink
- CLI operations are deferred until after file verification

CLI tests:

- `usync apply --plan` requires plan path
- `usync apply --plan --dry-run` writes nothing
- `usync apply --plan --auto-approve` succeeds for file-backed fixtures
- legacy `sync --apply` remains unchanged

Audit tests, if included:

- writes valid JSONL
- permissions are `0600`
- no raw credentials appear
- audit failure becomes warning after successful apply

## Risks And Mitigations

- **Risk:** Schema v1 plans from Phase 2 cannot apply.
  **Mitigation:** Fail clearly with "re-run usync plan"; do not try unsafe auto-upgrade.

- **Risk:** Credentials at apply time differ from credentials at plan time.
  **Mitigation:** Credential refs identify required keys; Phase 4 validation can strengthen this later.

- **Risk:** Saved-plan apply duplicates legacy apply.
  **Mitigation:** Convert saved operations into existing `Operation` values and reuse `prepareFileOperation`, `WriteConfig`, rollback, and verify helpers.

- **Risk:** Approval gates block automation.
  **Mitigation:** Require `--auto-approve` for CI and tests.

- **Risk:** CLI-managed operations are hard to rollback.
  **Mitigation:** Run them after file verification and report separately; do not claim transactionality for CLI ops.

## Human Architecture Approval Status

Pending approval to implement.
