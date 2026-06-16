# Doctor Mode Remaining Implementation Plan

**Last updated:** May 22, 2026 (Exa MCP research audit #2)
**Excludes:** Optional MCP Server Mode (`usync mcp serve`)

## Scope

This plan consolidates the leftovers from Doctor Mode Phases 0–5 plus the remaining implementation work in `docs/research/Doctor Mode, Batch Plan:Apply, and Credential Validation new spec.md`.

Excluded for now:

- Optional MCP Server Mode (`usync mcp serve`)
- MCP gateway/proxy behavior
- Dynamic MCP registry consumption
- Remote/team state
- Windows support

## Current Snapshot

Already present in the codebase:

- `pkg/manifest` with all 12 client manifests, 7 provider metas, runtime requirements, and path expansion helpers.
- `pkg/doctor` read-only scanner with candidate file scanning, confidence scoring, migration hints, runtime checks, JSON/human formatting, and `usync doctor` CLI command with `--json`, `--home-dir`, `--workspace`, `--no-runtimes` flags.
- Saved plan schema v2, plan store (save/load/list/clean), `usync plan`, `usync show`, `plan list`, and `plan clean`.
- `usync apply --plan` with stale-plan checks, SHA-256 checksum verification, create/symlink approval prompts, rollback, verification, and audit logging via `pkg/audit`.
- `pkg/validate` with offline validation, live validation support for GitHub and Tavily, key-file parsing, and a private 24h validation cache.
- Legacy wizard still available through the existing TUI model, now wrapped as `NewWizardModel`.
- `pkg/config/files.go` has `WriteWithLock` with O_CREATE|O_EXCL locking and retry.
- `pkg/audit/audit.go` has append-only JSONL writer with `0600` permissions.

Important leftovers found in the current code:

- `usync plan --all-detected` exists as a flag but returns `"--all-detected requires doctor mode and is not implemented yet"` (plan_commands.go:67).
- `usync plan` and `apply --plan` credential loading and plan creation are hardcoded to `--provider exa` only (plan_commands.go:58-61).
- `pkg/config/paths.go` still has `chooseExistingPath` used for Windsurf path selection in legacy detection; not yet fully replaced by manifest-backed discovery.
- Default `usync` still opens the provider-first wizard; there is no doctor dashboard model yet. `--wizard` flag exists but both paths call `NewWizardModel()`.
- `usync providers` command does not exist.
- Audit log rotation is not implemented — `pkg/audit/audit.go` is append-only with no size limit.
- Saved-plan apply regenerates desired content at apply time and does not persist desired-content digests.
- `_usync` metadata/idempotency markers from the research spec are not implemented.
- CLI-managed apply exists for Claude Code but Codex CLI adapter support is not complete.
- No `fmt.Printf` in any `pkg/` production code (verified clean).
- No `TODO` or `FIXME` markers in `pkg/` or `cmd/` except the `--all-detected` stub.

## MCP Research Audit (May 22, 2026, Pass 2)

This plan was reviewed with Exa MCP searches on May 22, 2026 (second pass). The following findings tighten requirements and **correct path assumptions** from the original plan:

### Critical Path Corrections

- **Antigravity CLI config path has changed.** Google's official migration guide at `antigravity.google/docs/gcli-migration` confirms:
  - Global MCP config: `~/.gemini/antigravity-cli/mcp_config.json` (NOT `~/.gemini/antigravity/mcp_config.json` as previously assumed)
  - Workspace MCP config: `.agents/mcp_config.json` (NOT `.gemini/settings.json`)
  - The `mcp_config.json` may or may not be a symlink depending on install method. Issue #60 on `google-antigravity/antigravity-cli` confirms post-migration path is `~/.gemini/config/mcp_config.json` with `~/.gemini/antigravity-cli/mcp_config.json` as a legacy symlink.
  - Binary name is `agy`, not `antigravity`. Both `gemini` and `agy` can coexist.
- **Antigravity CLI uses `serverUrl` field** (not `url` or `httpUrl`) for remote MCP servers. Local stdio servers with `command`+`args` are unchanged.
- **Manifest must be updated** to reflect the corrected `~/.gemini/antigravity-cli/mcp_config.json` path and the `.agents/mcp_config.json` workspace path. The URLField for Antigravity should be `serverUrl`.

### Client-Specific Findings

- **Google's May 19, 2026 transition post** confirms June 18, 2026 consumer cutoff. Enterprise/Workspace users keep Gemini CLI access indefinitely. All warnings must say "consumer/free/Pro/Ultra" and must not imply every enterprise user loses access.
- **Claude Code scope names changed**: `local` (default, was `project`), `project` (shared via `.mcp.json`), `user` (was `global`). Stored in `~/.claude.json` (user/local) and `.mcp.json` (project). Supports `streamable-http` as alias for `http`. Environment variable expansion in `.mcp.json` supports `${VAR}` and `${VAR:-default}` syntax. Managed MCP configuration (`managed-mcp.json` and `managed-settings.json` deployed to system paths like `/etc/claude-code/managed-settings.json`) is a new enterprise feature for org-wide policy.
- **VS Code** now supports `sandboxEnabled` and `sandbox` objects for MCP server security in `mcp.json`. The `inputs` feature (`${input:variableName}`) remains the recommended approach for secrets. Dev Containers can configure MCP via `customizations.vscode.mcp.servers`. The `chat.mcp.discovery.enabled` setting allows auto-discovery of MCP configs from other apps.
- **Codex CLI** PR #12718 to add local `mcp.json` support was rejected; per-project MCP config is only through `.codex/config.toml` (trust-gated for security). Codex config also supports `bearer_token_env_var`, `env_http_headers`, `enabled_tools`/`disabled_tools`, and `mcp_oauth_credentials_store` ("auto" | "file" | "keyring"). An open community issue (#13056) requests better per-project MCP but no resolution yet.
- **Zed** uses `context_servers` root in `~/.config/zed/settings.json`. Custom servers support both stdio (`command`/`args`/`env`) and remote (`url`/`headers`). Zed now supports OAuth flow for remote MCP servers without configured `Authorization` header. Zed is evolving toward Agent Client Protocol (ACP) for deeper agent integration beyond MCP — ACP is orthogonal to usync's MCP config scope.
- **Tavily's `/usage` endpoint** is confirmed as a valid live validation target. Rate limits remain at 10 requests per 10 minutes for the usage endpoint. The cache and TUI must respect this.

### Architecture Patterns Confirmed

- **Bubble Tea command model** remains the right fit: filesystem/runtime work belongs in `tea.Cmd`, returning typed messages into `Update`. The dashboard must not block `Init` or `View`. Bubble Tea's batching (`tea.Batch`) enables concurrent scans.
- **Terraform plan -out / apply** semantics remain the UX reference. Terraform warns that saved plans may contain sensitive material; usync stays JSON-only and redacted.
- **VS Code `inputs`** should be preferred over hardcoded credentials when writing VS Code MCP configs. This is now documented as a security best practice in the VS Code docs.

## Planning Decision

Do not reopen Phases 0–5 as large rewrites. Treat the already implemented parts as the foundation and finish the rest through focused follow-up phases:

- Phase 6: Close CLI and doctor completeness gaps, fix manifest paths.
- Phase 7: Add TUI doctor dashboard foundation.
- Phase 8: Add provider readiness, validation UX, and saved-plan TUI flow.
- Phase 11: Hardening, QA, docs, and compatibility cleanup.

This keeps MCP Server Mode out of the implementation path while still completing the local-first `doctor → validate → plan → apply` product. Conflict resolution between competing client config paths lives inside the dashboard (Phase 8); cross-product migration tooling is intentionally not in scope.

## Phase 6: CLI And Doctor Completeness

Objective: make the CLI match the non-TUI research contract, fix known path/metadata errors, and remove old discovery inconsistencies.

Tasks:

1. **Fix Antigravity CLI manifest paths.** Update `pkg/manifest/clients.go` Antigravity entries:
   - Global path: `~/.gemini/antigravity-cli/mcp_config.json` (current/canonical)
   - Legacy path: `~/.gemini/config/mcp_config.json` (post-migration fallback observed in issue #60)
   - Old legacy: `~/.gemini/antigravity/mcp_config.json` (pre-I/O 2026, deprecated)
   - Workspace path: `.agents/mcp_config.json`
   - Set URLField to `serverUrl` for Antigravity entries.
   - The `IsSymlink` flag should be `true` for the canonical path (symlink to resolved target).
   - Add `CLIName: "agy"` to the manifest (not `antigravity`).
2. Add `usync doctor --clients <ids>` and `--verbose`.
3. Add doctor fixture homes and golden JSON tests for healthy, empty, malformed, Antigravity conflict (old vs new paths), Gemini sunset, and Linux variants.
4. Make doctor report effective-path selection more explicit: current, deprecated, conflict, CLI-only, missing.
5. Populate `SavedPlan.DoctorSummary` from doctor reports when planning.
6. **Implement `usync plan --all-detected`** using doctor confidence `high` and `medium`, skipping `low` and `conflict`. Remove the "not implemented yet" stub from `plan_commands.go:67`.
7. Add `--include-workspace` to opt into project/workspace candidates (`.agents/mcp_config.json`, `.vscode/mcp.json`, `.codex/config.toml`, `.mcp.json`).
8. Add `--detailed-exitcode` for `plan`.
9. Decide and document plan-file format as JSON-only for this project, superseding the older gob-plus-sidecar language in the research doc.
10. **Add provider-neutral credential loading** to `plan` and `apply --plan` through `pkg/validate` key files. Remove the `--provider exa` hardcoding from `plan_commands.go:58-61`. Support all registered providers.
11. Preserve target scope in plan operations: user, global, local, project, workspace, managed, legacy.
12. Add project/workspace approval gates for `.mcp.json`, `.vscode/mcp.json`, `.codex/config.toml`, `.agents/mcp_config.json`, and any target likely to enter source control.
13. Prefer target-native secret indirection where supported: VS Code `${input:varName}`, Claude Code `${VAR:-default}`, Codex `env_vars` forwarding; otherwise show explicit credential-in-file warnings.
14. Replace or wrap `pkg/config.DetectAppConfigsForOS` with manifest-backed discovery and remove `chooseExistingPath` behavior from new flows.
15. Add `usync providers` with readiness from manifest, doctor runtime checks, and optional credential validation state.
16. Add CI/import guard tests for `pkg/doctor` not importing `pkg/app` or `pkg/tui`.

Acceptance:

- `usync plan --all-detected --provider exa --keys-file ...` produces a saved plan from doctor findings.
- `usync plan --all-detected --provider github --keys-file ...` also works (not exa-only).
- Conflict and low-confidence clients are skipped with plan warnings.
- Provider-neutral key files work for Exa, GitHub, Context7, Tavily, Playwright, Kubernetes, and Terraform where applicable.
- `--detailed-exitcode` returns 0 for no changes, 1 for errors, and 2 for pending changes.
- Project/workspace targets are excluded unless `--include-workspace` is set.
- No new code path depends on `chooseExistingPath`.
- Antigravity manifest paths match the official `antigravity.google/docs/gcli-migration` documentation.
- `go test ./pkg/doctor ./pkg/manifest ./pkg/app ./cmd/usync` passes.

## Phase 7: TUI Doctor Dashboard Foundation

Objective: make default `usync` status-first without changing apply semantics yet.

Tasks:

1. Add `DashboardModel` in `pkg/tui`.
2. Add a small scanner interface so dashboard tests can inject fake reports.
3. Start doctor scan through a Bubble Tea `tea.Cmd`; first render must show loading immediately.
4. Add loaded, empty, partial/error states.
5. Render a compact dashboard: detected clients, conflicts, configured providers, runtime blockers, and warnings.
6. Add key actions for refresh, wizard, configure, resolve conflict, and quit.
7. Wire default `usync` to dashboard and make `usync --wizard` actually open the current provider-first flow (fix the current no-op behavior).
8. Keep dashboard read-only in this phase.
9. Never parse client config files directly in `pkg/tui`; all filesystem findings must come from `pkg/doctor`.

Acceptance:

- `usync --wizard` opens the legacy wizard.
- Default `usync` opens dashboard mode.
- First render does not wait for a filesystem/runtime scan (spinner immediately).
- Dashboard scan work runs in `tea.Cmd` and updates state through typed messages.
- Empty fixture renders a useful non-empty state ("No AI clients detected").
- Existing wizard tests still pass.
- `go test ./pkg/tui ./cmd/usync` passes.

## Phase 8: Dashboard Provider Readiness And Saved-Plan Flow

Objective: make the dashboard a complete path to preview and apply, using the existing plan/apply APIs.

Tasks:

1. Add provider readiness view model from doctor report, manifest provider metadata, runtime checks, supplied credentials, and validation reports.
2. Group providers as no-key-needed, ready-with-credentials, missing-credentials, runtime-missing, conflict-blocked.
3. Show get-key URLs from manifest for missing credentials.
4. Integrate offline validation before preview.
5. Add explicit live validation action with cache behavior and no automatic network calls.
6. Add dashboard target/provider selection using doctor candidates rather than `pkg/config` static paths.
7. Save a plan before preview through the saved-plan APIs.
8. Render saved-plan preview, warnings, skipped clients, approval gates, and redacted credential refs.
9. Apply through `ApplySavedPlan`, preserving create, symlink, stale, and workspace/project gates.
10. Rate-limit live validation UI, especially Tavily `/usage`, to respect the official 10 requests per 10 minutes limit.
11. Use target-native secret indirection where possible: for VS Code support `${input:varName}` before writing credential-bearing values directly; for Claude Code support `${VAR:-default}` expansion syntax.

Acceptance:

- Malformed credentials block preview in TUI with redacted output.
- Live validation is opt-in and tested with mock HTTP only.
- Repeated live validation within the cache window does not make a new HTTP request.
- Dashboard preview creates a saved plan; it does not use legacy in-memory apply.
- Dashboard apply calls saved-plan apply.
- Raw credentials do not appear in rendered strings or tests.
- `go test ./pkg/tui ./pkg/validate ./pkg/app` passes.

## Phase 11: Hardening, QA, And Compatibility Cleanup

Objective: close the remaining research-spec safety and quality items after the main flows exist.

Tasks:

1. Add audit log rotation at 5MB.
2. Add desired-content digest or equivalent immutability metadata to saved plans so apply can detect provider/client generation drift.
3. Add `_usync` metadata/idempotency markers where target formats can safely store metadata. JSON targets get `"_usync": { "managedBy": "usync", "at": "...", "planID": "..." }` nested under each server entry. Codex TOML gets a `# managed-by=usync` comment. Zed `context_servers` gets the marker under each server entry.
4. Add skip-on-identical behavior so no-op applies do not rewrite files.
5. Add full Codex CLI adapter support: `codex mcp add <server-name> -- <command> <args>`, `codex mcp add --url <url> --bearer-token-env-var <var>`. Note that `codex mcp add` only supports user-scope `~/.codex/config.toml`; per-project `.codex/config.toml` is trust-gated.
6. Add all-locks-before-write behavior if the current per-file lock behavior proves insufficient for batch transaction semantics.
7. Add no-stdout-from-library CI guard.
8. Add redaction regression tests for provider readiness output, saved plans, audit entries, command args, URLs, env maps, and headers.
9. Add shell/e2e tests for `doctor → validate → plan → apply`, `--all-detected`, and dashboard entry.
10. Update user docs and README command examples.
11. Keep legacy `sync --dry-run` and `sync --apply` behavior until a later deprecation plan is accepted.
12. Add reference-verification tests or checklist entries for high-change client docs: VS Code, Claude Code, Codex, Zed, Gemini/Antigravity.
13. Add a documented source-confidence field or comment in manifest entries whose path evidence is empirical rather than official.
14. **Validate VS Code `sandboxEnabled`/`sandbox` config objects** are preserved during mutation. Never strip unknown fields from MCP configs.
15. **Validate Claude Code managed-settings compatibility**: if `managed-mcp.json` or `managed-settings.json` exists, warn that usync changes may be overridden by org policy.

Acceptance:

- `go test ./...` passes.
- `make lint` passes.
- `make test` passes.
- `make build` passes.
- `make verify` passes if available in the environment.
- No tests perform real live-validation network calls.
- No generated plan, output, audit log, or test failure string contains raw credentials.

## References Used For This Audit

### Primary Sources (Official Documentation)

- Google Developers Blog, "An important update: Transitioning Gemini CLI to Antigravity CLI": https://developers.googleblog.com/en/an-important-update-transitioning-gemini-cli-to-antigravity-cli/
- Google Antigravity CLI Migration Guide: https://antigravity.google/docs/gcli-migration
- VS Code MCP server documentation: https://code.visualstudio.com/docs/copilot/customization/mcp-servers
- VS Code MCP configuration reference: https://code.visualstudio.com/docs/copilot/reference/mcp-configuration
- Claude Code MCP documentation: https://code.claude.com/docs/en/mcp
- Claude Code settings documentation: https://code.claude.com/docs/en/configuration
- Zed MCP documentation: https://zed.dev/docs/ai/mcp
- Codex CLI MCP servers: https://openai-codex.mintlify.app/configuration/mcp-servers
- Codex config basics: https://developers.openai.com/codex/config-basic
- Codex advanced config: https://developers.openai.com/codex/config-advanced

### Secondary Sources (Community / Issue Reports)

- Antigravity CLI issue #60 (project-local MCP config bug): https://github.com/google-antigravity/antigravity-cli/issues/60
- VS Code issue #252907 (user mcp.json schema standardization): https://github.com/microsoft/vscode/issues/252907
- Claude Code issue #5037 (.mcp.json loading): https://github.com/anthropics/claude-code/issues/5037
- Codex CLI PR #12718 (local mcp.json — rejected): https://github.com/openai/codex/pull/12718
- Codex example config: https://github.com/openai/codex/blob/rust-v0.63.0/docs/example-config.md
- Local MCP with Antigravity CLI (dev.to): https://dev.to/gde/local-mcp-development-with-python-and-antigravity-cli-3ojg
- Zed ACP progress report: https://zed.dev/blog/acp-progress-report

### UX Pattern References

- Bubble Tea commands tutorial: https://github.com/charmbracelet/bubbletea/blob/main/tutorials/commands/README.md
- Terraform plan command reference: https://developer.hashicorp.com/terraform/cli/commands/plan
- Terraform apply command reference: https://developer.hashicorp.com/terraform/cli/commands/apply

## Recommended Implementation Order

1. **Phase 6 Task 1:** Fix Antigravity CLI manifest paths (URGENT — affects all downstream phases).
2. **Phase 6 Task 6:** `--all-detected` with doctor reports.
3. **Phase 6 Task 10:** Provider-neutral key loading in `plan` and `apply --plan`.
4. **Phase 7:** Dashboard skeleton and default entry switch.
5. **Phase 8:** Dashboard readiness and saved-plan flow (includes the in-dashboard conflict-resolution overlay).
6. **Phase 11:** Hardening and compatibility cleanup.

## Key Manifest Corrections Required

The following corrections MUST be applied to `pkg/manifest/clients.go` before any Phase 6+ work begins:

### Antigravity CLI

```
Current manifest path:  ~/.gemini/antigravity/mcp_config.json
Correct path:           ~/.gemini/antigravity-cli/mcp_config.json
Legacy/fallback:        ~/.gemini/config/mcp_config.json (observed post-migration)
Old legacy:             ~/.gemini/antigravity/mcp_config.json (pre-I/O 2026)
Workspace path:         .agents/mcp_config.json
URL field:              serverUrl (NOT url)
CLI binary:             agy (NOT antigravity)
```

### Antigravity IDE

```
Config location:        Managed by Antigravity IDE desktop app
MCP config:             Via IDE Settings → Customizations → Open MCP Config
Uses serverUrl field:   Yes
```

### Claude Code Scope Changes

```
Old scope names:        project (local), global (user)
New scope names:        local (default), project (shared), user (cross-project)
New feature:            managed-settings.json / managed-mcp.json (enterprise via system paths like /etc/claude-code/)
New feature:            ${VAR} and ${VAR:-default} expansion syntax in .mcp.json
Transport alias:        "streamable-http" accepted as alias for "http"
```

### Codex CLI Project Config

```
Per-project config:     .codex/config.toml (trust-gated)
Local mcp.json:         NOT supported (PR #12718 rejected)
New fields:             bearer_token_env_var, env_http_headers, enabled_tools, disabled_tools
OAuth:                  mcp_oauth_credentials_store = "auto" | "file" | "keyring"
```

### VS Code New Features

```
New config fields:      sandboxEnabled (boolean), sandbox (object with filesystem/network rules)
Dev containers:         customizations.vscode.mcp.servers
Auto-discovery:         chat.mcp.discovery.enabled (discovers MCP from other apps)
CLI install:            --add-mcp command-line option
```

## Non-Goals For This Roadmap

- No `usync mcp serve`.
- No agent-callable apply tool.
- No external registry sync.
- No remote state or team coordination.
- No broad TUI rewrite beyond the dashboard flow required here.
- No Windows support.
- No ACP (Agent Client Protocol) integration — this is Zed-specific and orthogonal to MCP config management.
