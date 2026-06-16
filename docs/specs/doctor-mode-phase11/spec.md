# Doctor Mode Phase 11: Hardening, QA, and Compatibility Cleanup

**Type:** Hardening / Refactor spec  
**Status:** Draft — awaiting approval  
**Last updated:** 2026-05-23  
**Source:** `docs/specs/doctor-mode-remaining-implementation-plan.md` § Phase 11  
**Scope note:** Gemini CLI and `pkg/migrate` have been removed. All migration-related items are excluded. Antigravity CLI and Antigravity IDE are the only Google-ecosystem clients.

---

## Problem Statement

After Phases 6–8 the core `doctor → validate → plan → apply` pipeline is functional, but several correctness, safety, and testability gaps remain:

1. **Audit logs grow without bound.** Long-running installations will accumulate unbounded `~/.usync/audit.log` files. There is no rotation.
2. **Apply always writes, even when nothing changed.** A no-op apply (same credential, same provider, same targets) rewrites every config file, invalidates file mtimes, and creates backup files unnecessarily.
3. **Saved plans have no integrity signal.** If the provider URL format or credential shape changes between plan creation and apply, the apply silently uses the stale plan. There is no way to detect drift.
4. **Config files have no provenance marker.** There is no indicator that a given MCP server entry is managed by usync, making manual edits and audits harder.
5. **The Codex CLI path is file-only.** `codex mcp add` is the official user-scope CLI entry point; usync only writes to TOML directly, bypassing Codex's own trust model for user-scope entries.
6. **Test quality has gaps.** TUI view tests have no golden baseline, no stable color profile, and no reusable wait helpers. Redaction coverage is partial. There are no end-to-end shell tests covering the full `doctor → plan → apply` pipeline.
7. **VS Code sandbox field safety is untested.** `UpdateNamedServerJSON` should preserve `sandboxEnabled` and `sandbox` fields; this is unverified by any test.
8. **The doctor has no awareness of Claude Code enterprise policy.** When an org deploys `managed-mcp.json` or `managed-settings.json`, usync changes may be silently overridden. Users are not warned.
9. **Manifest source confidence is undocumented.** Some client paths are empirically observed, not from official docs. Implementers cannot distinguish authoritative from inferred paths.

---

## Goals

- Bound audit log disk usage at 5 MB with a single-rotation backup.
- Skip file writes when the proposed content is byte-identical to the existing file.
- Add a plan content hash that `PreflightSavedPlan` can verify has not drifted.
- Add `_usync` provenance markers to JSON MCP config entries managed by usync; add a `# managed-by=usync` comment to Codex TOML entries.
- Add a `codex mcp add` CLI path for user-scope Codex entries.
- Harden the TUI test suite: golden view baselines, stable color profile, reusable helpers, NO\_COLOR CI.
- Provide a comprehensive redaction regression suite covering all formatted output surfaces.
- Add end-to-end shell tests for `doctor → plan → apply`, `--all-detected`, and dashboard launch.
- Add a VS Code sandbox field preservation regression test.
- Add a Claude Code managed-settings warning to the doctor report.
- Add a `SourceConfidence` field to `manifest.SourceRef` to distinguish official from empirical path evidence.
- Produce a reference-verification checklist for high-change client docs.

---

## Non-Goals

- Gemini CLI support (removed).
- Migration UX (Phases 9/10 skipped).
- Multi-file audit log rotation (more than one `.1` backup).
- Automatic credential re-validation at apply time.
- Windows support.
- MCP Server Mode.
- Per-project Codex CLI support (trust-gated by Codex; excluded).
- Writing or reading `/etc/claude-code/` files (usync never touches managed config).
- Structured log format changes to the audit log body.

---

## Users / Actors

| Actor | Concern addressed |
|---|---|
| End user running `usync apply` repeatedly | Skip-on-identical prevents redundant backup creation |
| End user whose audit log fills disk | Log rotation bounds growth |
| Operator reviewing audit logs | Provenance markers clarify which entries usync manages |
| Security reviewer | Redaction regression tests confirm no credentials leak |
| Developer extending usync | Golden TUI tests catch view regressions; source-confidence field clarifies path reliability |
| Enterprise Claude Code user | Doctor warns when managed-settings override is in effect |

---

## Functional Requirements

### FR-1 — Audit log rotation at 5 MB

When `Writer.Append` is called and the existing `audit.log` file is ≥ 5 MB, usync renames `audit.log` to `audit.log.1` (overwriting any previous `.1`) before creating a new `audit.log` and appending the entry. The rename is best-effort; a rename failure must not prevent the append.

**Measurable boundary:** 5 × 1024 × 1024 bytes (5,242,880 bytes).  
**Backup count:** exactly one (`.1`). No chain rotation.  
**New-file permissions:** same `0600` as the current file.  
**Concurrent safety:** single-writer assumption (one `usync` process at a time); no additional locking required.

---

### FR-2 — Skip-on-identical apply

When `prepareFileOperation` computes the proposed write content and `bytes.Equal(existingContent, proposedContent)` is true, the file write is skipped. The file path is recorded in `ApplyResult.SkippedTargets []string` and rendered in `FormatApplyResult` under an "Unchanged" section. No backup file is created for skipped targets.

This applies to both `Apply` (legacy execution plan) and `ApplySavedPlan` (saved plan path). CLI operations (`FileKindClaudeCodeCLI`, Codex CLI add) are not subject to skip-on-identical.

---

### FR-3 — Plan content integrity hash

`BuildSavedPlan` computes a SHA-256 hash of the concatenation of all `PlanOperation.Redacted` strings (in order) and stores it as `SavedPlan.ContentHash string`. The hash is hex-encoded with a `"sha256:"` prefix.

`PreflightSavedPlan` recomputes the hash from `plan.Operations` and returns an error if it does not match `plan.ContentHash`, with message: `"saved plan content hash mismatch: plan may have been modified or provider configuration has drifted"`.

When `ContentHash` is empty (plans created before Phase 11), `PreflightSavedPlan` skips the check (backward-compatible).

---

### FR-4 — `_usync` provenance markers in JSON configs

When usync writes a JSON MCP server entry (`FileKindMCPServers`, `FileKindBareMCPServers`, `FileKindNamedServer`), it adds a `"_usync"` object to that entry containing:

```json
{
  "_usync": {
    "managedBy": "usync",
    "at": "<RFC3339 UTC timestamp>",
    "planID": "<plan-id or empty string>"
  }
}
```

The marker is nested **inside** the server entry object (alongside `url`, `command`, `headers`, etc.), not at the config root.

**Exclusions:** `AppAntigravityCLI` and `AppAntigravity` targets do not receive the marker (serverUrl format, no stable metadata slot).

The marker is written by passing it as the `extra map[string]any` argument already present in `UpdateMCPServersJSON`, `UpdateBareMCPServersJSON`, and `UpdateNamedServerJSON`. For the legacy `Apply` path where `planID` is unavailable, `planID` is the empty string and the `_usync` object is still written (with `"planID": ""`).

**Idempotency:** If a `_usync` key already exists in the entry, it is overwritten with fresh values.

---

### FR-5 — `# managed-by=usync` comment in Codex TOML

When `UpdateCodexTOML` writes or updates an `[mcp_servers.<name>]` block, it prepends a `# managed-by=usync` comment line directly above the section header. The comment is written exactly once; if it already precedes the header, it is not duplicated.

```toml
# managed-by=usync
[mcp_servers.exa]
url = "https://mcp.exa.ai/mcp?..."
```

---

### FR-6 — Codex CLI adapter (`codex mcp add`, user scope only)

For Codex CLI targets where a CLI-managed apply is preferred over direct TOML write, usync may invoke:

- **Stdio provider:** `codex mcp add <name> -- <command> [args...]`
- **HTTP provider:** `codex mcp add --url <url> --bearer-token-env-var <envvar> <name>`

This path is gated by a new `FileKindCodexCLIAdd` kind. It applies **only** to user-scope (`~/.codex/config.toml`) targets. Per-project `.codex/config.toml` targets remain file-only (trust-gated by Codex).

After CLI apply, usync verifies via `codex mcp get <name>` using the existing `verify.VerifyOptionalCLI` pattern.

If `codex` is not on PATH, the operation is skipped with a warning (same pattern as `claude` CLI unavailability).

---

### FR-7 — No-stdout-from-library CI guard

A CI test (or build-time check) asserts that no `.go` file under `pkg/` or `cmd/` (excluding `_test.go` files) contains a direct call to `fmt.Print`, `fmt.Println`, `fmt.Printf`, or `os.Stdout.Write`. This invariant is already true; the test documents and protects it.

---

### FR-8 — Redaction regression suite

A dedicated test file asserts that no raw credential pattern appears in any formatted output surface. Patterns tested:

- UUID: `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}` (case-insensitive)
- Context7 key: `ctx7sk[-_][A-Za-z0-9]{8,}`
- Tavily key: `tvly-[A-Za-z0-9]{8,}`
- GitHub PAT: `ghp_[A-Za-z0-9]{36}`

Surfaces tested (each as a table row):
1. `app.FormatSavedPlan` output
2. `app.FormatApplyResult` output
3. `app.FormatPlan` output
4. Audit entry JSON (marshalled `audit.Entry`)
5. TUI `DashboardModel.View()` with credentials in profiles (all 5 screens)
6. Doctor `FormatReport` with credential-bearing URLs in configured providers
7. Validate `FormatReport` output

Each surface is tested with at least one fake credential per pattern.

---

### FR-9 — TUI test hardening

**FR-9a — Color profile init:** A `pkg/tui/test_setup_test.go` file contains an `init()` function that calls `lipgloss.SetColorProfile(termenv.Ascii)`. This ensures all TUI tests produce stable, color-free output.

**FR-9b — Wait helpers:** A `pkg/tui/test_helpers_test.go` file exposes `waitForText(t, tm, text)` and `waitForAll(t, tm, values...)` helper functions using `teatest.WaitFor` with 2-second duration and 25 ms check interval. All teatest usages in the package use these helpers instead of inline `WaitFor` calls.

**FR-9c — Golden view tests:** A `pkg/tui/dashboard_golden_test.go` file contains one golden test per dashboard screen (`screenDoctor`, `screenProviderReady`, `screenTargetSelect`, `screenPlanPreview`, `screenApplyResult`). Each test sets `m.width = 80` via a `tea.WindowSizeMsg`, then calls `golden.RequireEqual(t, []byte(m.View()))`. Golden files are generated with `-update` and committed.

**FR-9d — `.gitattributes`:** The line `*.golden -text` is added to the repo `.gitattributes` to prevent LF/CRLF corruption on Windows CI.

**FR-9e — CI environment:** `make test` sets `NO_COLOR=1` and `TERM=xterm-256color` before running `go test ./...`. These variables are also documented in any CI workflow file in use.

---

### FR-10 — VS Code `sandboxEnabled` / `sandbox` preservation regression test

A test in `pkg/config/json_update_test.go` provides a VS Code MCP config containing both `sandboxEnabled: true` and a `sandbox` object alongside an existing server entry. After calling `UpdateNamedServerJSON`, the test asserts both `sandboxEnabled` and `sandbox` are present and unchanged in the output. (The field preservation is already implemented via `ensureObject`; this test documents and guards it.)

---

### FR-11 — Claude Code managed-settings compatibility warning

When `doctor.Scan` processes the `ClientClaudeCode` manifest entry, it additionally checks for the presence of any path in `options.ManagedSettingsPaths` (defaulting to `["/etc/claude-code/managed-settings.json", "/etc/claude-code/managed-mcp.json"]`). If any path exists on the filesystem, a `ClientWarning` with code `"managed-settings"` is appended to the `ClientFinding` for `ClientClaudeCode`.

Warning message: `"Claude Code managed configuration detected; usync changes to user/local scope may be overridden by org policy."` The path(s) found are included in the message.

`doctor.Options` gains `ManagedSettingsPaths []string` as an override for testing (empty → use defaults).

---

### FR-12 — Source-confidence field in manifest `SourceRef`

`manifest.SourceRef` gains a `Confidence string` field with values `"official"` (from a primary source such as official docs or source code), `"empirical"` (from community reports, issue trackers, or observed behaviour), or `""` (unset, treated as unknown).

All existing `SourceRef` entries in `pkg/manifest/clients.go` and `pkg/manifest/providers.go` are annotated with the appropriate confidence value. Entries whose paths were verified from official docs are `"official"`; entries whose paths come from issue reports or community observation (e.g. Antigravity CLI issue #60 path) are `"empirical"`.

The doctor report does not surface this field to end users; it is for developer reference only. A manifest test asserts that no `SourceRef` has an empty `Confidence` field.

---

### FR-13 — Reference-verification checklist

A file `docs/specs/doctor-mode-phase11/reference-checklist.md` is created listing, for each high-change client, the key claims usync makes about its config format, and a verification status (last-checked date, source URL, pass/fail). Clients covered:

- VS Code (MCP config schema, `sandboxEnabled`, `inputs`, `chat.mcp.discovery.enabled`)
- Claude Code (scope names, `${VAR:-default}` expansion, managed-mcp.json paths)
- Codex CLI (`bearer_token_env_var`, user-scope vs project-scope distinction, `mcp_oauth_credentials_store`)
- Antigravity CLI (canonical path, `serverUrl` field, `agy` binary, issue #60 fallback path)
- Antigravity IDE (canonical path, `serverUrl` field)
- Zed (`context_servers` root, OAuth flow)

This is a documentation artifact, not a runtime check.

---

## UX Requirements

**UX-1:** `FormatApplyResult` shows an "Unchanged (N)" section listing skipped targets when `ApplyResult.SkippedTargets` is non-empty. The section appears after "Updated" and before "Verification".

**UX-2:** `FormatSavedPlan` shows `Content hash: sha256:<...>` as a single line after plan metadata, when `ContentHash` is non-empty. It does not display if empty (backward compat).

**UX-3:** The Claude Code managed-settings warning appears in `FormatReport` output alongside other client warnings, using the same `!` prefix style.

**UX-4:** Golden TUI views are generated with `NO_COLOR=1`; they must not contain ANSI escape sequences.

---

## Data Model Requirements

### `audit.Writer` (pkg/audit/audit.go)

```go
const maxAuditLogBytes int64 = 5 * 1024 * 1024  // 5 MB

// maybeRotate renames audit.log → audit.log.1 when size ≥ threshold.
// Best-effort: silently ignores rename errors.
func (w Writer) maybeRotate()
```

`Append` calls `w.maybeRotate()` as its first action.

### `app.ApplyResult` (pkg/app/app.go)

```go
type ApplyResult struct {
    Plan            ExecutionPlan
    Warnings        []string
    BackupPaths     []string
    Verification    []verify.Result
    UpdatedTargets  []string
    SkippedTargets  []string   // NEW — files skipped because content is identical
    RolledBack      []string
    RollbackFailed  []string
}
```

### `app.preparedWrite` (pkg/app/app.go, unexported)

```go
type preparedWrite struct {
    op      Operation
    content []byte
    skipped bool   // NEW — true when bytes.Equal(existing, proposed)
}
```

### `app.SavedPlan` (pkg/app/plan_v2.go)

```go
type SavedPlan struct {
    // ... existing fields ...
    ContentHash   string          `json:"content_hash,omitempty"` // NEW — "sha256:<hex>"
}
```

### `config.UsyncMeta` (pkg/config/json_update.go)

```go
// UsyncMeta returns the _usync annotation map to embed in a server entry.
// Returns nil when planID is empty AND the call site opts for silent omission.
// In practice planID may be "" for the legacy Apply path; the marker is still written.
func UsyncMeta(planID, at string) map[string]any
```

### `manifest.SourceRef` (pkg/manifest/types.go)

```go
type SourceRef struct {
    URL        string
    Title      string
    VerifiedAt string
    Confidence string  // NEW — "official" | "empirical" | ""
}
```

### `doctor.Options` (pkg/doctor/doctor.go)

```go
type Options struct {
    // ... existing fields ...
    ManagedSettingsPaths []string  // NEW — override default /etc/claude-code/* paths for testing
}
```

---

## Technical Requirements

- **No new external dependencies.** `charmbracelet/x/exp/golden` and `muesli/termenv` move from indirect to direct; no version change.
- **`prepareFileOperation` must remain pure with respect to `skipped`.** It must not emit log entries for skipped writes; callers handle logging.
- **`ContentHash` computed over `Redacted` strings.** These are already redacted at plan creation time; no raw credentials enter the hash input.
- **`_usync` marker written via existing `extra` parameter.** No new function signature change to `UpdateMCPServersJSON`; callers pass the marker map as `extra`. The marker is merged at the per-server-entry level.
- **`planID` threading.** For the saved-plan apply path (`ApplySavedPlan` → `buildOperationFromSavedPlan`), the `planID` is available from `SavedPlan.PlanID`. It must be threaded to `prepareFileOperation` without changing the function signature beyond adding a `planID string` parameter.
- **Codex CLI add is optional.** `FileKindCodexCLIAdd` is only set when the manifest candidate has `MutationKind == MutationCodexCLI` (new constant). Existing `FileKindCodexTOML` (file write) remains the default for the current Codex manifest entry.
- **Golden tests use fixed terminal width 80.** Set via injecting `tea.WindowSizeMsg{Width: 80, Height: 24}` into `Update` before calling `View()`.
- **`go test ./...` must pass without `-update` after golden files are committed.**

---

## Testing Requirements

| Requirement | Layer | File |
|---|---|---|
| Rotation triggers at ≥ 5 MB; `.1` backup created | Unit | `pkg/audit/audit_test.go` |
| Rotation failure does not prevent append | Unit | `pkg/audit/audit_test.go` |
| Skip-on-identical: `preparedWrite.skipped == true` when bytes equal | Unit | `pkg/app/app_test.go` |
| Skip-on-identical: `ApplyResult.SkippedTargets` populated | Unit | `pkg/app/app_test.go` |
| Skip-on-identical: no backup file created for skipped target | Unit | `pkg/app/app_test.go` |
| `ContentHash` non-empty after `BuildSavedPlan` | Unit | `pkg/app/plan_v2_test.go` |
| `ContentHash` mismatch → `PreflightSavedPlan` returns error | Unit | `pkg/app/plan_apply_test.go` |
| Empty `ContentHash` → `PreflightSavedPlan` skips check | Unit | `pkg/app/plan_apply_test.go` |
| `_usync` marker present in MCPServers output | Unit | `pkg/config/json_update_test.go` |
| `_usync` marker absent for Antigravity targets | Unit | `pkg/app/app_test.go` |
| `_usync` overwritten on second apply (idempotent) | Unit | `pkg/config/json_update_test.go` |
| Codex TOML has `# managed-by=usync` | Unit | `pkg/config/toml_update_test.go` |
| Codex TOML comment not duplicated on second apply | Unit | `pkg/config/toml_update_test.go` |
| `codex mcp add` args correct for stdio | Unit | `pkg/app/app_test.go` |
| `codex mcp add` args correct for HTTP | Unit | `pkg/app/app_test.go` |
| `codex mcp add` skipped gracefully when codex not on PATH | Unit | `pkg/app/app_test.go` |
| VS Code `sandboxEnabled` + `sandbox` preserved | Unit | `pkg/config/json_update_test.go` |
| Managed-settings warning in doctor report | Unit | `pkg/doctor/doctor_test.go` |
| `SourceRef.Confidence` non-empty for all manifest entries | Unit | `pkg/manifest/manifest_test.go` |
| No-stdout-from-library invariant holds | Unit | `pkg/app/app_test.go` or dedicated |
| Redaction: all 7 surfaces × all 4 patterns | Unit | `pkg/tui/redaction_regression_test.go` |
| TUI golden: all 5 screens produce stable output | Golden | `pkg/tui/dashboard_golden_test.go` |
| TUI waitForText/waitForAll helpers used by teatest | Integration | `pkg/tui/dashboard_teatest_test.go` |
| e2e: `usync doctor --json` succeeds | Binary | `tests/e2e/e2e_test.go` |
| e2e: `usync plan --all-detected …` succeeds | Binary | `tests/e2e/e2e_test.go` |
| e2e: `usync plan … && usync apply --plan …` succeeds | Binary | `tests/e2e/e2e_test.go` |
| e2e: `usync` (default) launches dashboard and accepts `q` | Binary | `tests/e2e/e2e_test.go` |

---

## Acceptance Criteria

| # | Criterion |
|---|---|
| AC-1 | `go test ./...` passes. |
| AC-2 | `make lint` passes. |
| AC-3 | `make build` passes. |
| AC-4 | `NO_COLOR=1 TERM=xterm-256color go test ./...` passes (CI simulation). |
| AC-5 | After writing entries totalling ≥ 5 MB, `audit.log.1` exists and `audit.log` is a fresh file. |
| AC-6 | A second identical apply produces zero entries in `UpdatedTargets` and at least one in `SkippedTargets`. |
| AC-7 | `PreflightSavedPlan` returns error when `ContentHash` is mutated before apply. |
| AC-8 | JSON MCP config files written by usync (non-Antigravity) contain a `_usync` key inside each server entry. |
| AC-9 | Codex TOML files written by usync contain `# managed-by=usync` exactly once per server block. |
| AC-10 | `FormatApplyResult` output contains "Unchanged" section when files are skipped. |
| AC-11 | No UUID, ctx7sk, tvly, or ghp_ literal appears in any formatted output surface in the redaction regression suite. |
| AC-12 | All 5 TUI screen golden tests pass without `-update` on a second run. |
| AC-13 | Doctor report for a Claude Code installation with `/etc/claude-code/managed-settings.json` present contains a `managed-settings` warning. |
| AC-14 | All `SourceRef` entries in the manifest have a non-empty `Confidence` field. |
| AC-15 | No tests perform real live-validation network calls. |
| AC-16 | No generated plan, output, audit log, or test failure string contains raw credentials. |

---

## Edge Cases

| Edge case | Expected behaviour |
|---|---|
| Audit log does not exist yet | `maybeRotate` is a no-op; `Append` creates the file |
| Audit log is exactly 5 MB | Rotate (≥ 5 MB threshold is inclusive) |
| `audit.log.1` already exists at rotation time | Overwritten by `os.Rename` |
| `os.Rename` fails at rotation | Append continues to the existing (oversized) file; error ignored |
| Proposed write is empty (`updated == nil`) | Skip-on-identical does not apply; write proceeds normally |
| `ContentHash` field absent in old plan file | `PreflightSavedPlan` skips integrity check entirely |
| `_usync` extra map conflicts with existing `_usync` key | Overwritten; write is idempotent |
| `codex` binary not on PATH | Skip with warning; no error exit |
| Managed-settings path is unreadable (permission denied) | `os.Stat` returns error; warning suppressed; no crash |
| Terminal width < 40 in golden test | Fixed width 80 is injected via `WindowSizeMsg`; no truncation |
| `SourceRef.Confidence` left empty in a new manifest entry | Manifest test fails, forcing the developer to annotate it |

---

## Data Sensitivity and Compliance Notes

- `_usync` markers contain only `planID`, ISO timestamp, and the literal string `"usync"`. No credentials, no PII.
- `ContentHash` is computed over redacted strings; no raw credential enters the hash input.
- Audit log rotation must not lose any appended entries; rename is atomic on POSIX. On macOS/Linux this is guaranteed; on Windows it is not supported (Windows excluded from scope).
- The managed-settings warning message must not include the content of the managed config file; only the detected path is included.

---

## Assumptions

1. A single `usync` process runs at a time; concurrent audit log appends are not a concern for rotation.
2. `os.Rename` is atomic on the target platforms (macOS, Linux).
3. `go.mod` already contains `charmbracelet/x/exp/golden` and `muesli/termenv` as indirect dependencies; promoting to direct requires only adding imports, not version changes.
4. `UpdateNamedServerJSON` already preserves unknown JSON keys via `ensureObject`; the `sandboxEnabled` preservation test is a regression guard, not a fix.
5. The `_usync` key is safe to add to VS Code, Cursor, Zed, RooCode, OpenCode, Kiro, Claude Desktop, and Codex TOML entries. Antigravity is excluded (FR-4).
6. `doctor.Options.ManagedSettingsPaths` default paths (`/etc/claude-code/managed-settings.json`, `/etc/claude-code/managed-mcp.json`) are correct per Claude Code docs as of May 2026.
7. `SourceRef.Confidence` values `"official"` and `"empirical"` are sufficient for Phase 11; a richer taxonomy is deferred.

---

## Open Questions

All open questions are resolved or explicitly deferred:

| OQ | Status |
|---|---|
| Should `_usync` go at config root or per-entry? | Resolved — per server entry, to support multi-server files. |
| Should audit rotation use a rolling chain (`.1`, `.2`, …)? | Resolved — single `.1` backup is sufficient; simplicity wins. |
| Should `ContentHash` cover raw URLs (before redaction)? | Resolved — computed over `Redacted` strings only; no raw credentials. |
| Should Codex CLI add be the default path or opt-in? | Resolved — opt-in via `FileKindCodexCLIAdd`; existing TOML write remains default. |
| Should `SourceRef.Confidence` be surfaced to end users? | Resolved — developer reference only; not in formatted output. |
| Should managed-settings warning block apply? | Resolved — warning only; apply proceeds. Blocking would be too disruptive for enterprise users. |

---

## Human Approval Status

**Not yet approved.** Requires explicit sign-off on:
1. `_usync` marker format and per-entry placement.
2. Single-backup audit rotation strategy.
3. `ContentHash` scoped to `Redacted` strings (not raw URLs).
4. `FileKindCodexCLIAdd` as opt-in (Codex TOML write remains default).
