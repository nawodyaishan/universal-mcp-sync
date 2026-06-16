# Doctor Mode Phase 11 — Technical Implementation Plan

**Status:** Draft — awaiting architecture approval  
**Spec:** `docs/specs/doctor-mode-phase11/spec.md`  
**Last updated:** 2026-05-23  
**PR groupings:** 11a (operational), 11b (test hardening), 11c (compatibility)

---

## Summary

Phase 11 closes 13 hardening gaps across audit safety, apply correctness, provenance tracking, test quality, and client compatibility. All changes are additive and backward-compatible. No new external dependencies beyond promoting two Charm packages from indirect to direct. No schema migrations.

---

## Inputs Reviewed

- `docs/specs/doctor-mode-phase11/spec.md` — 13 FRs, 16 ACs, data model requirements
- `pkg/audit/audit.go` — `Writer`, `Append`; no rotation logic
- `pkg/app/app.go:53–61` — `ApplyResult` struct; no `SkippedTargets`
- `pkg/app/app.go:529–562` — `prepareOperations`; calls `prepareFileOperation(op)` at line 547
- `pkg/app/app.go:563–640` — `prepareFileOperation(op Operation) (preparedWrite, error)`
- `pkg/app/plan_v2.go` — `SavedPlan`, `PlanOperation`; no `ContentHash`
- `pkg/app/plan_apply.go:164–275` — `prepareSavedPlan`; calls `prepareFileOperation(opForWrite)` at line 255
- `pkg/config/json_update.go` — `UpdateMCPServersJSON`, `UpdateBareMCPServersJSON`, `UpdateNamedServerJSON`; all accept `extra map[string]any`
- `pkg/config/toml_update.go` — `UpdateCodexTOML`, `buildCodexBlock`; no managed-by comment
- `pkg/config/paths.go` — `FileKind` constants; no `FileKindCodexCLIAdd`
- `pkg/manifest/types.go:107–110` — `SourceRef`; no `Confidence` field; `MutationKind` constants
- `pkg/doctor/doctor.go:18–36` — `Options` struct; no `ManagedSettingsPaths`
- `go.mod` — `charmbracelet/x/exp/golden` and `muesli/termenv` already indirect

---

## Assumptions

1. `os.Rename` is atomic on macOS and Linux; single-backup rotation is safe without additional locking.
2. Skip-on-identical skips writes only when `len(data) > 0 && bytes.Equal(data, updated)` — an empty existing file is never skipped.
3. `ContentHash` covers `PlanOperation.Redacted` strings only; these are already redacted at plan creation time. No raw credentials enter the hash input.
4. `_usync` marker uses the `extra` parameter already present in all JSON-writing helpers; no signature change to those helpers.
5. `prepareFileOperation` gains one additional parameter: `planID string`. Both callers (`prepareOperations` at line 547 and `prepareSavedPlan` at line 255) are updated in the same task.
6. Codex CLI adapter (`FileKindCodexCLIAdd`) is opt-in via manifest; existing `FileKindCodexTOML` (direct TOML write) remains the default for the current Codex CLI manifest entry.
7. `doctor.Options.ManagedSettingsPaths` nil-slice means use package-level defaults; zero-length slice (explicitly set) means skip the check — allowing tests to disable it cleanly.
8. `SourceRef.Confidence` values are `"official"` or `"empirical"` only; no third value for Phase 11.
9. `charmbracelet/x/exp/golden` and `muesli/termenv` require only adding import lines; `go mod tidy` promotes them from indirect to direct without version changes.

---

## Architecture Approach

All tasks follow existing patterns:
- Audit rotation: `maybeRotate()` method on `Writer`; called at the top of `Append`.
- Skip-on-identical: `preparedWrite.skipped bool` field; write loop in `Apply` and `ApplySavedPlan` honours it.
- ContentHash: SHA-256 in `BuildSavedPlan`; early-exit check in `PreflightSavedPlan`.
- `_usync` markers: composed via `config.UsyncMeta(planID, at)`, passed as `extra` to existing helpers.
- planID threading: `prepareFileOperation(op Operation, planID string)` — both callers updated.
- Codex CLI adapter: new `FileKindCodexCLIAdd` constant; new case in `prepareOperations` and `Apply`.
- Managed-settings: `d.checkManagedSettings()` called inside `scanClient` when `client.ID == manifest.ClientClaudeCode`.
- SourceRef.Confidence: additive field with test guard.

---

## Affected Modules

| Module | Change | PR |
|---|---|---|
| `pkg/audit/audit.go` | `maybeRotate()`, `maxAuditLogBytes` | 11a |
| `pkg/audit/audit_test.go` | Rotation unit tests | 11a |
| `pkg/app/app.go` | `ApplyResult.SkippedTargets`; `preparedWrite.skipped`; `prepareFileOperation(op, planID)`; `FormatApplyResult` Unchanged section; `buildCodexCLIAddArgs`; Codex CLI add case | 11a |
| `pkg/app/plan_apply.go` | `prepareFileOperation(opForWrite, plan.PlanID)`; ContentHash check in `PreflightSavedPlan`; skip-on-identical in saved-plan write loop | 11a |
| `pkg/app/plan_v2.go` | `SavedPlan.ContentHash`; `BuildSavedPlan` hash computation; `FormatSavedPlan` content-hash line | 11a |
| `pkg/config/json_update.go` | `UsyncMeta(planID, at string) map[string]any` | 11a |
| `pkg/config/paths.go` | `FileKindCodexCLIAdd FileKind` | 11a |
| `pkg/manifest/types.go` | `MutationCodexCLI MutationKind`; `SourceRef.Confidence string` | 11a + 11c |
| `pkg/config/toml_update.go` | `# managed-by=usync` comment in `UpdateCodexTOML` | 11a |
| `.gitattributes` | `*.golden -text` | 11b |
| `Makefile` | `NO_COLOR=1 TERM=xterm-256color` on test target | 11b |
| `pkg/tui/test_setup_test.go` | New: `lipgloss.SetColorProfile(termenv.Ascii)` init | 11b |
| `pkg/tui/test_helpers_test.go` | New: `waitForText`, `waitForAll` | 11b |
| `pkg/tui/dashboard_golden_test.go` | New: 5 golden screen tests | 11b |
| `pkg/tui/redaction_regression_test.go` | New: 7 surfaces × 4 patterns | 11b |
| `pkg/config/json_update_test.go` | VS Code sandbox preservation test | 11b |
| `pkg/app/app_test.go` | No-stdout guard; skip-on-identical; Codex CLI args; `_usync` Antigravity exclusion | 11a + 11b |
| `tests/e2e/e2e_test.go` | Doctor→plan→apply flow; `--all-detected`; dashboard launch | 11b |
| `pkg/doctor/doctor.go` | `Options.ManagedSettingsPaths`; `checkManagedSettings()`; `scanClient` integration | 11c |
| `pkg/manifest/clients.go` | `SourceRef.Confidence` values on all client entries | 11c |
| `pkg/manifest/providers.go` | `SourceRef.Confidence` values on all provider entries | 11c |
| `pkg/manifest/manifest_test.go` | `TestAllSourceRefsHaveConfidence` | 11c |
| `pkg/doctor/doctor_test.go` | `TestScanDetectsManagedSettings` | 11c |
| `docs/specs/doctor-mode-phase11/reference-checklist.md` | New documentation artifact | 11c |

---

## API and Contract Changes

### `pkg/audit/audit.go`

```go
const maxAuditLogBytes int64 = 5 * 1024 * 1024

// maybeRotate renames audit.log → audit.log.1 when size ≥ 5 MB. Best-effort.
func (w Writer) maybeRotate()

// Append — first line changes to: w.maybeRotate()
func (w Writer) Append(entry Entry) error
```

### `pkg/app/app.go`

```go
// ApplyResult gains one field:
SkippedTargets []string

// preparedWrite gains one field:
type preparedWrite struct {
    op      Operation
    content []byte
    skipped bool  // true when bytes.Equal(existingFileBytes, content)
}

// prepareFileOperation gains planID parameter:
func (m *Manager) prepareFileOperation(op Operation, planID string) (preparedWrite, error)

// New helper for Codex CLI adapter:
func buildCodexCLIAddArgs(providerID string, cfg provider.MCPConfig) []string
```

### `pkg/app/plan_v2.go`

```go
// SavedPlan gains one field (omitempty for backward compat):
ContentHash string `json:"content_hash,omitempty"`

// BuildSavedPlan — after Operations loop, before return:
//   h := sha256.New()
//   for _, op := range saved.Operations { io.WriteString(h, op.Redacted) }
//   saved.ContentHash = "sha256:" + hex.EncodeToString(h.Sum(nil))
```

### `pkg/app/plan_apply.go`

```go
// PreflightSavedPlan — after schema version check, before prepareSavedPlan:
//   if plan.ContentHash != "" {
//       recompute hash; return error on mismatch
//   }

// prepareSavedPlan — prepareFileOperation call at line 255 changes to:
item, err := m.prepareFileOperation(opForWrite, plan.PlanID)
```

### `pkg/config/json_update.go`

```go
// New exported helper (planID may be ""):
func UsyncMeta(planID, at string) map[string]any {
    return map[string]any{
        "_usync": map[string]any{
            "managedBy": "usync",
            "at":        at,
            "planID":    planID,
        },
    }
}
```

### `pkg/config/paths.go`

```go
// New FileKind constant:
FileKindCodexCLIAdd FileKind = "codexCLIAdd"
```

### `pkg/manifest/types.go`

```go
// New MutationKind constant:
MutationCodexCLI MutationKind = "codexCLI"

// SourceRef gains Confidence field:
type SourceRef struct {
    URL        string
    Title      string
    VerifiedAt string
    Confidence string  // "official" | "empirical"
}
```

### `pkg/doctor/doctor.go`

```go
// Options gains override for testing:
type Options struct {
    HomeDir              string
    WorkspaceDir         string
    GOOS                 string
    CheckRuntimes        bool
    Now                  func() time.Time
    ManagedSettingsPaths []string  // nil → use package defaults
}

var defaultManagedSettingsPaths = []string{
    "/etc/claude-code/managed-settings.json",
    "/etc/claude-code/managed-mcp.json",
}

// New private method on Doctor:
func (d *Doctor) checkManagedSettings() []string
```

---

## Data Model Changes

- `SavedPlan.ContentHash` — additive `omitempty` JSON field; existing plan files without it remain valid.
- `ApplyResult.SkippedTargets` — additive Go slice; not serialised.
- `preparedWrite.skipped` — unexported; no serialisation.
- `SourceRef.Confidence` — additive string field; zero value `""` triggers a manifest test failure (documentation enforcement).
- `_usync` object inside MCP server entries — unknown-key safe for all JSON-consuming MCP clients.
- `# managed-by=usync` comment in Codex TOML — TOML comments are ignored by parsers.

---

## Dependency Changes

`go mod tidy` after adding test imports promotes two packages from indirect to direct. **No version changes required.**

| Package | Old | New |
|---|---|---|
| `github.com/charmbracelet/x/exp/golden` | `// indirect` | direct (test only) |
| `github.com/muesli/termenv` | `// indirect` | direct (test only) |

---

## Security Impact

- `_usync` markers contain only `planID` (opaque string), ISO-8601 timestamp, and the literal `"usync"`. No credentials.
- `ContentHash` is SHA-256 over already-redacted strings; no raw credential enters the hash input.
- Audit rotation uses `os.Rename`; on POSIX this is atomic. The new file gets the same `0600` permissions as the old one.
- Managed-settings warning message includes only the detected file path, not the file content.

---

## Authorization Boundaries

Skip-on-identical does not bypass approval prompts. The skip is evaluated **after** `prepareFileOperation` computes content — if the content is identical, there is nothing to approve. Approval gates in `prepareSavedPlan` (symlink, create, workspace-scope prompts) are unaffected.

---

## Observability Impact

- `FormatApplyResult` gains an "Unchanged (N)" section.
- `FormatSavedPlan` gains a `Content hash: sha256:<...>` line (omitted when empty).
- `FormatReport` (doctor) surfaces managed-settings warning with `!` prefix.
- Audit `.1` backup file created on rotation; no log entry written for the rotation event.

---

## Testing Strategy

See spec §Testing Requirements. All tests use `go test ./...` with `NO_COLOR=1`. No network calls in any test. Fake service/manager pattern throughout.

---

## Failure Modes

| Failure | Handling |
|---|---|
| `os.Rename` fails at audit rotation | `maybeRotate` ignores error; `Append` continues to existing (oversized) file |
| `bytes.Equal` false positive | Cannot happen: identical bytes are identical content |
| `ContentHash` mismatch | `PreflightSavedPlan` returns descriptive error; apply blocked |
| `codex` not on PATH | `buildCodexCLIAddArgs` result passed to Runner; `LookPath` returns error; operation skipped with warning |
| `os.Stat` fails on managed-settings path (permission denied) | `checkManagedSettings` treats it as absent; no warning |
| `io.WriteString` fails writing to SHA-256 hasher | `sha256.New()` writer never returns errors; safe to ignore |

---

## Rollback and Recovery

All changes are additive and backward-compatible:
- `ContentHash` field `omitempty` — old binaries read new plans without error.
- `SkippedTargets` — not serialised; old binaries unaffected.
- `_usync` — unknown JSON keys; all MCP clients ignore them.
- `SourceRef.Confidence` — zero-value `""` is valid Go; old binaries compile.
- Audit `.1` file — old binary creates a fresh `audit.log` on next run.
- `--wizard` flag still bypasses the dashboard; Phase 8 regression is contained.

---

## Numbered Implementation Tasks

Dependencies are listed per task. Tasks within the same PR may be developed in parallel where no intra-PR dependency exists.

---

### Task 1 — Audit log rotation at 5 MB (FR-1)

**Files:** `pkg/audit/audit.go`, `pkg/audit/audit_test.go`  
**Depends on:** nothing  
**PR:** 11a | **Risk:** Low

**Changes to `pkg/audit/audit.go`:**

```go
const maxAuditLogBytes int64 = 5 * 1024 * 1024

func (w Writer) maybeRotate() {
    info, err := os.Stat(w.Path)
    if err != nil || info.Size() < maxAuditLogBytes {
        return
    }
    _ = os.Rename(w.Path, w.Path+".1")
}
```

Modify `Append`: insert `w.maybeRotate()` as the first statement, before `os.MkdirAll`.

**New tests in `pkg/audit/audit_test.go`:**
- `TestAuditLogRotatesAt5MB` — write entries in a loop until file size ≥ 5 MB; assert `audit.log.1` exists; assert new `audit.log` is smaller than threshold.
- `TestAuditLogRotationFailureDoesNotPreventAppend` — simulate rename failure by making `audit.log.1` a read-only directory; assert `Append` succeeds despite rename error.

**Spec FR:** FR-1  
**Acceptance:** `go test ./pkg/audit` passes; AC-5.  
**Verify:** `go test ./pkg/audit -run TestAuditLog -v`

---

### Task 2 — Skip-on-identical apply (FR-2)

**Files:** `pkg/app/app.go`  
**Depends on:** nothing  
**PR:** 11a | **Risk:** Low

**Step 1 — extend `preparedWrite`:**

```go
type preparedWrite struct {
    op      Operation
    content []byte
    skipped bool
}
```

**Step 2 — `prepareFileOperation` (line 563)** — add `skipped` computation after `updated` is computed, before the final return:

```go
skipped := len(data) > 0 && bytes.Equal(data, updated)
return preparedWrite{op: op, content: updated, skipped: skipped}, nil
```

Note: this task does **not** yet change the `prepareFileOperation` signature (planID threading is Task 4). The `skipped` field is set here using only local `data` and `updated`.

**Step 3 — extend `ApplyResult`:**

```go
type ApplyResult struct {
    // ... existing fields ...
    SkippedTargets []string
}
```

**Step 4 — `Apply` write loop (around line 303)** — skip write when `item.skipped`:

```go
if item.skipped {
    result.SkippedTargets = append(result.SkippedTargets, item.op.Path)
    m.logDebug("skipping identical write", "path", item.op.Path)
    continue
}
```

**Step 5 — `ApplySavedPlan` write loop in `plan_apply.go` (around the `WriteConfig` call in `ApplySavedPlan`)** — same pattern. In `ApplySavedPlan`, the file write is `m.WriteConfig(item.prepared.op.Path, item.prepared.content, m.Now())`. Check `item.prepared.skipped` before this call.

**Step 6 — `FormatApplyResult` (pkg/app/app.go line 458)** — add after the "Updated" section:

```go
if len(result.SkippedTargets) > 0 {
    fmt.Fprintf(&b, "Unchanged (%d)\n", len(result.SkippedTargets))
    for _, t := range result.SkippedTargets {
        fmt.Fprintf(&b, "- %s\n", t)
    }
}
```

**New tests in `pkg/app/app_test.go`:**
- `TestPrepareFileOperation_SkipsIdenticalContent` — mock existing file with content X; mock `updated == X`; assert `preparedWrite.skipped == true`.
- `TestApply_SkipsIdenticalFiles` — two consecutive applies; second has `SkippedTargets` non-empty; no new backup file.

**Spec FR:** FR-2, UX-1  
**Acceptance:** `go test ./pkg/app` passes; AC-6, AC-10.  
**Verify:** `go test ./pkg/app -run TestPrepare.*Identical -v && go test ./pkg/app -run TestApply.*Skip -v`

---

### Task 3 — Plan content integrity hash (FR-3)

**Files:** `pkg/app/plan_v2.go`, `pkg/app/plan_apply.go`  
**Depends on:** nothing  
**PR:** 11a | **Risk:** Low

**Step 1 — `pkg/app/plan_v2.go`: extend `SavedPlan`:**

```go
type SavedPlan struct {
    // ... existing fields ...
    ContentHash string `json:"content_hash,omitempty"`
}
```

**Step 2 — `BuildSavedPlan` (line 89)**: after `saved.Operations` is populated, before `return saved, nil`:

```go
h := sha256.New()
for _, op := range saved.Operations {
    _, _ = io.WriteString(h, op.Redacted)
}
saved.ContentHash = "sha256:" + hex.EncodeToString(h.Sum(nil))
```

Add imports: `"crypto/sha256"`, `"encoding/hex"`, `"io"` (all already present or available in module).

**Step 3 — `FormatSavedPlan` (pkg/app/plan_format.go or wherever it lives)**: add a line after the plan ID / provider metadata block:

```go
if plan.ContentHash != "" {
    fmt.Fprintf(&b, "Content hash: %s\n", plan.ContentHash)
}
```

**Step 4 — `PreflightSavedPlan` (pkg/app/plan_apply.go line 62)**: add hash check before calling `prepareSavedPlan`:

```go
func (m *Manager) PreflightSavedPlan(plan SavedPlan, opts SavedPlanApplyOptions) (SavedPlanPreflight, error) {
    if plan.ContentHash != "" {
        h := sha256.New()
        for _, op := range plan.Operations {
            _, _ = io.WriteString(h, op.Redacted)
        }
        computed := "sha256:" + hex.EncodeToString(h.Sum(nil))
        if computed != plan.ContentHash {
            return SavedPlanPreflight{}, fmt.Errorf(
                "saved plan content hash mismatch: plan may have been modified or provider configuration has drifted")
        }
    }
    prepared, err := m.prepareSavedPlan(plan, opts)
    // ...
}
```

**New tests:**
- `pkg/app/plan_v2_test.go`: `TestBuildSavedPlan_ContentHashPresent` — hash is non-empty and starts with `"sha256:"`.
- `pkg/app/plan_apply_test.go`: `TestPreflightSavedPlan_ContentHashMismatchReturnsError` — mutate one `Redacted` field after `BuildSavedPlan`; assert error.
- `pkg/app/plan_apply_test.go`: `TestPreflightSavedPlan_EmptyContentHashSkipsCheck` — zero `ContentHash`; assert no error.

**Spec FR:** FR-3, UX-2  
**Acceptance:** `go test ./pkg/app` passes; AC-7.  
**Verify:** `go test ./pkg/app -run TestBuildSavedPlan_ContentHash -v && go test ./pkg/app -run TestPreflightSavedPlan_ContentHash -v`

---

### Task 4 — `_usync` provenance markers + planID threading (FR-4)

**Files:** `pkg/config/json_update.go`, `pkg/app/app.go`, `pkg/app/plan_apply.go`  
**Depends on:** Task 2 (signature change to `prepareFileOperation`)  
**PR:** 11a | **Risk:** Medium (signature change + two callers)

**Step 1 — `pkg/config/json_update.go`: add `UsyncMeta`:**

```go
// UsyncMeta returns the _usync provenance annotation for a server entry.
// planID may be "" for the legacy Apply path.
func UsyncMeta(planID, at string) map[string]any {
    return map[string]any{
        "_usync": map[string]any{
            "managedBy": "usync",
            "at":        at,
            "planID":    planID,
        },
    }
}
```

**Step 2 — `pkg/app/app.go`: change `prepareFileOperation` signature** to accept `planID string`:

```go
func (m *Manager) prepareFileOperation(op Operation, planID string) (preparedWrite, error)
```

Inside `prepareFileOperation`, before the `switch op.Kind` block, compute:

```go
var usyncExtra map[string]any
isAntigravity := op.AppID == config.AppAntigravityCLI || op.AppID == config.AppAntigravity
if !isAntigravity {
    usyncExtra = config.UsyncMeta(planID, m.Now().UTC().Format(time.RFC3339))
}
```

Pass `usyncExtra` as the `extra` argument in each relevant case:
- `FileKindMCPServers`: merge with existing `extra` (use `maps.Clone` or manual merge; Go 1.21+ has `maps.Clone`)
- `FileKindBareMCPServers`: pass `usyncExtra` as `extra` (currently `nil`)
- `FileKindNamedServer`: merge with existing `extra`

For `FileKindCodexTOML` and `FileKindClaudeCodeCLI`, no `usyncExtra` is used (the marker is handled by `UpdateCodexTOML` via Task 5 comment injection).

**Step 3 — update caller in `prepareOperations` (line 547 of `pkg/app/app.go`)**:

```go
item, err := m.prepareFileOperation(op, "")  // legacy path: no planID
```

**Step 4 — update caller in `prepareSavedPlan` (line 255 of `pkg/app/plan_apply.go`)**:

```go
item, err := m.prepareFileOperation(opForWrite, plan.PlanID)
```

**New tests:**
- `pkg/config/json_update_test.go`: `TestUsyncMetaInMCPServersJSON` — `UsyncMeta` output written inside server entry; `_usync.managedBy == "usync"`.
- `pkg/config/json_update_test.go`: `TestUsyncMetaIdempotent` — second call overwrites `_usync`; no duplicate.
- `pkg/app/app_test.go`: `TestPrepareFileOperation_UsyncAbsentForAntigravity` — `AppAntigravityCLI` and `AppAntigravity` outputs have no `_usync` key.

**Spec FR:** FR-4, AC-8  
**Acceptance:** `go test ./pkg/config ./pkg/app` passes.  
**Verify:** `go test ./pkg/config -run TestUsync -v && go test ./pkg/app -run TestPrepareFileOperation_Usync -v`

---

### Task 5 — `# managed-by=usync` comment in Codex TOML (FR-5)

**Files:** `pkg/config/toml_update.go`, `pkg/config/toml_update_test.go`  
**Depends on:** nothing  
**PR:** 11a | **Risk:** Low

**Change to `buildCodexBlock` (line 67)**:

Prepend `"# managed-by=usync"` as the first line of the block:

```go
func buildCodexBlock(providerID string, cfg provider.MCPConfig) []string {
    block := []string{
        "# managed-by=usync",
        fmt.Sprintf("[mcp_servers.%s]", providerID),
    }
    // ... rest unchanged ...
}
```

The existing `UpdateCodexTOML` logic already replaces the section by building a fresh block — this guarantees idempotency without needing a separate dedup check.

**New tests:**
- `TestUpdateCodexTOML_HasManagedByComment` — output contains `# managed-by=usync` before the section header.
- `TestUpdateCodexTOML_CommentNotDuplicated` — call `UpdateCodexTOML` twice on the same data; assert exactly one `# managed-by=usync` in the final output.

**Spec FR:** FR-5, AC-9  
**Acceptance:** `go test ./pkg/config` passes.  
**Verify:** `go test ./pkg/config -run TestUpdateCodexTOML -v`

---

### Task 6 — Codex CLI adapter (`codex mcp add`, user scope) (FR-6)

**Files:** `pkg/config/paths.go`, `pkg/manifest/types.go`, `pkg/app/app.go`  
**Depends on:** Task 4 (planID threading is in place)  
**PR:** 11a | **Risk:** Medium (new CLI path)

**Step 1 — `pkg/config/paths.go`**: add constant:

```go
FileKindCodexCLIAdd FileKind = "codexCLIAdd"
```

**Step 2 — `pkg/manifest/types.go`**: add constant:

```go
MutationCodexCLI MutationKind = "codexCLI"
```

**Step 3 — `pkg/app/app.go`**: add `buildCodexCLIAddArgs`:

```go
// buildCodexCLIAddArgs returns the arguments for `codex mcp add`.
// Stdio: codex mcp add <name> -- <command> [args...]
// HTTP:  codex mcp add --url <url> --bearer-token-env-var <envvar> <name>
func buildCodexCLIAddArgs(providerID string, cfg provider.MCPConfig) []string {
    if cfg.Type == provider.TransportStdio {
        args := []string{"mcp", "add", providerID, "--"}
        args = append(args, cfg.Command)
        return append(args, cfg.Args...)
    }
    args := []string{"mcp", "add", "--url", cfg.URL}
    for k := range cfg.Headers {
        args = append(args, "--bearer-token-env-var", k)
        break // first header key only; Codex supports one credential per server
    }
    return append(args, providerID)
}
```

**Step 4 — `prepareOperations`**: add `FileKindCodexCLIAdd` to the CLI-op collection case:

```go
case config.FileKindCodexCLIAdd:
    m.logDebug("preparing Codex CLI add operation", "app", op.AppName)
    cliOps = append(cliOps, op)
```

**Step 5 — `Apply`**: add case alongside `FileKindClaudeCodeCLI`:

```go
case config.FileKindCodexCLIAdd:
    args := buildCodexCLIAddArgs(op.ProviderID, op.Config)
    if _, err := m.Runner.Run("codex", args...); err != nil {
        result.Warnings = append(result.Warnings,
            fmt.Sprintf("codex mcp add %s: %v", op.ProviderID, err))
    } else {
        result.UpdatedTargets = append(result.UpdatedTargets, "codex mcp add "+op.ProviderID)
        seenApps[op.AppID] = true
    }
```

**Step 6 — verification in `Apply` loop** (where CLI verification runs, alongside the codex `mcp get` already present):

```go
case config.AppCodexCLI:
    result.Verification = append(result.Verification,
        verify.VerifyOptionalCLI(m.Runner, "codex", "mcp", "get", provID))
```

(This case already exists for Codex; `FileKindCodexCLIAdd` operations are collected into `seenApps` so the verification loop picks them up.)

**New tests in `pkg/app/app_test.go`:**
- `TestBuildCodexCLIAddArgs_Stdio` — assert `["mcp", "add", "exa", "--", "npx", "-y", "@exa/mcp"]`.
- `TestBuildCodexCLIAddArgs_HTTP` — assert `["mcp", "add", "--url", "https://...", "--bearer-token-env-var", "EXA_API_KEY", "exa"]`.
- `TestApply_CodexCLIAddSkippedWhenNotOnPath` — `fakeRunner` where `codex` lookup fails; assert warning in `result.Warnings`; no error return.

**Spec FR:** FR-6  
**Acceptance:** `go test ./pkg/app` passes.  
**Verify:** `go test ./pkg/app -run TestBuildCodexCLIAdd -v && go test ./pkg/app -run TestApply_CodexCLI -v`

---

### Task 7 — No-stdout-from-library CI guard (FR-7)

**Files:** `pkg/app/app_test.go` (or `cmd/usync/main_test.go`)  
**Depends on:** nothing  
**PR:** 11b | **Risk:** Low

Add `TestNoStdoutFromLibrary`:

```go
func TestNoStdoutFromLibrary(t *testing.T) {
    dirs := []string{"../../pkg", "../../cmd"}
    forbidden := []string{"fmt.Print(", "fmt.Println(", "fmt.Printf(", "os.Stdout"}
    for _, dir := range dirs {
        err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
            if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") ||
                strings.HasSuffix(path, "_test.go") {
                return err
            }
            data, readErr := os.ReadFile(path)
            if readErr != nil {
                return readErr
            }
            for _, pattern := range forbidden {
                if bytes.Contains(data, []byte(pattern)) {
                    t.Errorf("found %q in library file %s", pattern, path)
                }
            }
            return nil
        })
        if err != nil {
            t.Fatalf("WalkDir %s: %v", dir, err)
        }
    }
}
```

**Spec FR:** FR-7, AC-2  
**Acceptance:** `go test ./pkg/app -run TestNoStdout` passes and documents the invariant.  
**Verify:** `go test ./pkg/app -run TestNoStdoutFromLibrary -v`

---

### Task 8 — Redaction regression suite (FR-8)

**Files:** `pkg/tui/redaction_regression_test.go` (new)  
**Depends on:** Task 9a (color profile init must be in place before TUI View() calls)  
**PR:** 11b | **Risk:** Low

New file `pkg/tui/redaction_regression_test.go`:

```go
package tui

import (
    "regexp"
    "testing"
    "time"
    // app, audit, doctor, validate imports
)

var credentialPatterns = []*regexp.Regexp{
    regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
    regexp.MustCompile(`ctx7sk[-_][A-Za-z0-9]{8,}`),
    regexp.MustCompile(`tvly-[A-Za-z0-9]{8,}`),
    regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`),
}

func assertNoRawCredential(t *testing.T, subject string) {
    t.Helper()
    for _, re := range credentialPatterns {
        if loc := re.FindString(subject); loc != "" {
            t.Errorf("raw credential %q found in output", loc)
        }
    }
}

func TestRedactionRegression(t *testing.T) {
    // one subtest per surface
}
```

Surfaces (7 subtests): `app.FormatSavedPlan`, `app.FormatApplyResult`, `app.FormatPlan`, `audit.Entry` JSON, `DashboardModel.View()` all 5 screens, `doctor.FormatReport`, `validate.FormatReport`. Each subtest injects at least one fake UUID credential.

**Spec FR:** FR-8, AC-11, AC-16  
**Acceptance:** `go test ./pkg/tui -run TestRedaction -v` passes.  
**Verify:** `go test ./pkg/tui -run TestRedactionRegression -v`

---

### Task 9 — TUI test hardening (FR-9)

**Files:** `pkg/tui/test_setup_test.go` (new), `pkg/tui/test_helpers_test.go` (new), `pkg/tui/dashboard_golden_test.go` (new), `.gitattributes`, `Makefile`  
**Depends on:** nothing (precedes Task 8)  
**PR:** 11b | **Risk:** Low

**9a — `pkg/tui/test_setup_test.go`** (new):

```go
package tui

import (
    "github.com/charmbracelet/lipgloss"
    "github.com/muesli/termenv"
)

func init() {
    lipgloss.SetColorProfile(termenv.Ascii)
}
```

**9b — `pkg/tui/test_helpers_test.go`** (new):

```go
package tui

import (
    "bytes"
    "testing"
    "time"
    "github.com/charmbracelet/x/exp/teatest"
)

func waitForText(t *testing.T, tm *teatest.TestModel, text string) {
    t.Helper()
    teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
        return bytes.Contains(b, []byte(text))
    }, teatest.WithDuration(2*time.Second), teatest.WithCheckInterval(25*time.Millisecond))
}

func waitForAll(t *testing.T, tm *teatest.TestModel, values ...string) {
    t.Helper()
    teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
        for _, v := range values {
            if !bytes.Contains(b, []byte(v)) {
                return false
            }
        }
        return true
    }, teatest.WithDuration(2*time.Second), teatest.WithCheckInterval(25*time.Millisecond))
}
```

Refactor all existing `teatest.WaitFor` inline calls in `dashboard_teatest_test.go` to use `waitForText` or `waitForAll`.

**9c — `pkg/tui/dashboard_golden_test.go`** (new) — one function per screen:

```go
func TestGoldenScreenDoctor(t *testing.T) {
    m := NewDashboardModel(&FakeScanner{Report: doctor.Report{Platform: "test"}}, nil, nil)
    m.scanning = false
    // inject fixed width
    next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
    m = next.(DashboardModel)
    golden.RequireEqual(t, []byte(m.View()))
}
// ... one function per screen (screenProviderReady, screenTargetSelect, screenPlanPreview, screenApplyResult)
```

Generate with: `NO_COLOR=1 go test ./pkg/tui -run TestGolden -update`

**9d — `.gitattributes`**: append `*.golden -text`

**9e — `Makefile`**: update `test` target:

```makefile
test:
	NO_COLOR=1 TERM=xterm-256color go test ./...
```

**Spec FR:** FR-9, UX-4, AC-4, AC-12  
**Acceptance:** `go test ./pkg/tui` passes; golden files committed; Task 8 can run after this.  
**Verify:** `NO_COLOR=1 go test ./pkg/tui -run TestGolden -v`

---

### Task 10 — VS Code `sandboxEnabled` / `sandbox` preservation regression test (FR-10)

**Files:** `pkg/config/json_update_test.go`  
**Depends on:** nothing  
**PR:** 11b | **Risk:** Low

Add `TestUpdateNamedServerJSON_PreservesSandboxFields`:

```go
func TestUpdateNamedServerJSON_PreservesSandboxFields(t *testing.T) {
    data := []byte(`{
      "servers": {
        "exa": {
          "type": "http",
          "url": "https://old.example/mcp",
          "sandboxEnabled": true,
          "sandbox": {"filesystem": "readonly", "network": "none"}
        }
      }
    }`)
    cfg := provider.MCPConfig{Type: provider.TransportStreamableHTTP, URL: "https://new.example/mcp"}
    updated, err := UpdateNamedServerJSON(data, "exa", "servers", "url", cfg, nil)
    if err != nil {
        t.Fatal(err)
    }
    if !bytes.Contains(updated, []byte(`"sandboxEnabled": true`)) {
        t.Errorf("sandboxEnabled stripped:\n%s", updated)
    }
    if !bytes.Contains(updated, []byte(`"sandbox"`)) {
        t.Errorf("sandbox object stripped:\n%s", updated)
    }
    if !bytes.Contains(updated, []byte(`"readonly"`)) {
        t.Errorf("sandbox content stripped:\n%s", updated)
    }
}
```

**Spec FR:** FR-10, AC-1  
**Acceptance:** `go test ./pkg/config -run TestUpdateNamedServerJSON_PreservesSandbox -v` passes.  
**Verify:** `go test ./pkg/config -run TestUpdateNamedServerJSON_PreservesSandboxFields -v`

---

### Task 11 — Claude Code managed-settings compatibility warning (FR-11)

**Files:** `pkg/doctor/doctor.go`, `pkg/doctor/doctor_test.go`  
**Depends on:** nothing  
**PR:** 11c | **Risk:** Low

**Step 1 — extend `Options` in `pkg/doctor/doctor.go`**: add after existing fields:

```go
ManagedSettingsPaths []string // nil → use defaultManagedSettingsPaths
```

**Step 2 — package-level default** (alongside other package-level vars):

```go
var defaultManagedSettingsPaths = []string{
    "/etc/claude-code/managed-settings.json",
    "/etc/claude-code/managed-mcp.json",
}
```

**Step 3 — new private method**:

```go
// checkManagedSettings returns paths from ManagedSettingsPaths that exist on disk.
func (d *Doctor) checkManagedSettings() []string {
    paths := d.options.ManagedSettingsPaths
    if paths == nil {
        paths = defaultManagedSettingsPaths
    }
    var found []string
    for _, p := range paths {
        if _, err := os.Stat(p); err == nil {
            found = append(found, p)
        }
    }
    return found
}
```

**Step 4 — call inside `scanClient`** (or in `clientHintsAndWarnings`) when `client.ID == manifest.ClientClaudeCode`:

```go
if client.ID == manifest.ClientClaudeCode {
    if detected := d.checkManagedSettings(); len(detected) > 0 {
        finding.Warnings = append(finding.Warnings,
            fmt.Sprintf("Claude Code managed configuration detected (%s); usync changes to user/local scope may be overridden by org policy.",
                strings.Join(detected, ", ")))
    }
}
```

**New test `TestScanDetectsManagedSettings` in `pkg/doctor/doctor_test.go`**:

```go
func TestScanDetectsManagedSettings(t *testing.T) {
    homeDir := t.TempDir()
    fakeManaged := filepath.Join(homeDir, "managed-settings.json")
    if err := os.WriteFile(fakeManaged, []byte("{}"), 0o600); err != nil {
        t.Fatal(err)
    }
    doc, err := New(Options{
        HomeDir:              homeDir,
        CheckRuntimes:        false,
        ManagedSettingsPaths: []string{fakeManaged},
    })
    if err != nil {
        t.Fatal(err)
    }
    report, err := doc.Scan(context.Background())
    if err != nil {
        t.Fatal(err)
    }
    cc := findClientFinding(t, report, "claude-code")
    if len(cc.Warnings) == 0 {
        t.Errorf("expected managed-settings warning, got none")
    }
    found := false
    for _, w := range cc.Warnings {
        if strings.Contains(w, "managed configuration") {
            found = true
        }
    }
    if !found {
        t.Errorf("expected 'managed configuration' in warning, got %v", cc.Warnings)
    }
}
```

**Spec FR:** FR-11, UX-3, AC-13  
**Acceptance:** `go test ./pkg/doctor` passes.  
**Verify:** `go test ./pkg/doctor -run TestScanDetectsManagedSettings -v`

---

### Task 12 — `SourceRef.Confidence` field (FR-12)

**Files:** `pkg/manifest/types.go`, `pkg/manifest/clients.go`, `pkg/manifest/providers.go`, `pkg/manifest/manifest_test.go`  
**Depends on:** nothing  
**PR:** 11c | **Risk:** Low

**Step 1 — extend `SourceRef` in `pkg/manifest/types.go`**:

```go
type SourceRef struct {
    URL        string
    Title      string
    VerifiedAt string
    Confidence string // "official" | "empirical"
}
```

**Step 2 — annotate all `SourceRef` entries in `pkg/manifest/clients.go`**:

Example pattern:
```go
{URL: "https://antigravity.google/docs/mcp", Title: "Antigravity MCP", VerifiedAt: "2026-05-21", Confidence: "official"},
{URL: "https://antigravity.google/docs/gcli-migration", Title: "Gemini CLI Migration Guide", VerifiedAt: "2026-05-23", Confidence: "official"},
// issue #60 path — empirical:
// (used in the DeprecationNote, not a SourceRef itself, but any SourceRef citing it gets "empirical")
```

Confidence assignment rules:
- Primary product documentation, official migration guides, official source repos → `"official"`
- Community posts, issue tracker reports, empirically observed paths → `"empirical"`

**Step 3 — annotate all `SourceRef` entries in `pkg/manifest/providers.go`** with the same rules.

**Step 4 — add enforcement test in `pkg/manifest/manifest_test.go`**:

```go
func TestAllSourceRefsHaveConfidence(t *testing.T) {
    for _, client := range AllClients() {
        for i, src := range client.Sources {
            if src.Confidence == "" {
                t.Errorf("client %s Sources[%d] URL=%q missing Confidence", client.ID, i, src.URL)
            }
        }
    }
    for _, prov := range AllProviders() {
        for i, src := range prov.Sources {
            if src.Confidence == "" {
                t.Errorf("provider %s Sources[%d] URL=%q missing Confidence", prov.ID, i, src.URL)
            }
        }
    }
}
```

**Spec FR:** FR-12, AC-14  
**Acceptance:** `go test ./pkg/manifest` passes; no empty Confidence fields.  
**Verify:** `go test ./pkg/manifest -run TestAllSourceRefsHaveConfidence -v`

---

### Task 13 — Reference-verification checklist (FR-13)

**Files:** `docs/specs/doctor-mode-phase11/reference-checklist.md`  
**Depends on:** nothing  
**PR:** 11c | **Risk:** None (documentation only)

Create `docs/specs/doctor-mode-phase11/reference-checklist.md` with a table per client: claim, last-verified date, source URL, confidence, pass/fail. Clients: VS Code, Claude Code, Codex CLI, Antigravity CLI, Antigravity IDE, Zed.

**Spec FR:** FR-13  
**Acceptance:** File exists and is non-empty; reviewed in PR.  
**Verify:** `test -f docs/specs/doctor-mode-phase11/reference-checklist.md`

---

## Task Dependency Graph

```
PR 11a — Operational reliability:
  Task 1  (audit rotation)            independent
  Task 2  (skip-on-identical)         independent
  Task 3  (ContentHash)               independent
  Task 4  (_usync + planID thread)    depends on Task 2 (prepareFileOperation signature)
  Task 5  (Codex TOML comment)        independent
  Task 6  (Codex CLI adapter)         depends on Task 4 (planID threading in place)

PR 11b — Test hardening:
  Task 7  (no-stdout guard)           independent
  Task 8  (redaction regression)      depends on Task 9 (color profile init must be first)
  Task 9  (TUI hardening)             independent  ← do first in 11b
  Task 10 (VS Code sandbox test)      independent

PR 11c — Compatibility:
  Task 11 (managed-settings warning)  independent
  Task 12 (SourceRef.Confidence)      independent
  Task 13 (reference checklist)       independent
```

**Intra-PR ordering:** Within PR 11a, Tasks 1–3 and 5 may be built in parallel; Task 4 waits for Task 2; Task 6 waits for Task 4. Within PR 11b, Task 9 must be committed before Task 8 begins.

---

## Recommended PR Sequence

| PR | Tasks | Merge prerequisite |
|---|---|---|
| **11a** | 1, 2, 3, 5, 4, 6 | none |
| **11b** | 9, 7, 10, 8 | 11a merged (redaction suite uses formatted output from updated Apply) |
| **11c** | 11, 12, 13 | none (independent of 11a and 11b) |

---

## Risks and Mitigations

| Risk | Severity | Mitigation |
|---|---|---|
| `prepareFileOperation` signature change breaks callers | Medium | Exactly two callers; both updated in Task 4; compiler enforces completeness |
| `_usync` extra map conflicts with existing `extra` in some callers | Low | Merge using `maps.Clone` + overwrite; idempotency test (Task 4) verifies |
| Golden test instability across environments | Medium | `lipgloss.SetColorProfile(termenv.Ascii)` in Task 9 eliminates color variance; fixed width 80 via `WindowSizeMsg` |
| `os.Rename` non-atomic on some filesystems | Low | Scope is macOS/Linux only; both guarantee atomic rename on same filesystem |
| Codex CLI adapter args incorrect for future Codex versions | Low | `buildCodexCLIAddArgs` is a pure function; easy to update; unit test documents expected shape |

---

## Verification Commands

```bash
# Full suite:
NO_COLOR=1 TERM=xterm-256color go test ./...

# Per-PR:
go test ./pkg/audit ./pkg/app ./pkg/config            # 11a
NO_COLOR=1 go test ./pkg/tui ./tests/e2e/...           # 11b
go test ./pkg/doctor ./pkg/manifest                    # 11c

# Generate golden files (once, then commit):
NO_COLOR=1 go test ./pkg/tui -run TestGolden -update

# CI simulation:
make test
make lint
make build
```

---

## Architecture Approval Status

**Not yet approved.** Requires sign-off on:

1. `prepareFileOperation(op Operation, planID string)` — adding `planID` as a parameter (two callers updated).
2. `_usync` marker format: per-server-entry object; Antigravity excluded.
3. ContentHash over `PlanOperation.Redacted` strings only (no raw URLs).
4. Single `.1` backup audit rotation.
5. `FileKindCodexCLIAdd` as opt-in (existing TOML write remains default; manifest change required to activate).
6. `Options.ManagedSettingsPaths` nil-means-default convention (empty slice skips check).
