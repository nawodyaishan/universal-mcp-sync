# Doctor Mode Phase 5: TUI Doctor Dashboard and Migration UX

## Problem Statement

The CLI now has the foundations for manifests, saved plans, saved-plan apply, and credential validation. The interactive TUI still starts in the old provider-first wizard, so new users do not first see system status, detected tools, existing MCP setup, missing credentials, runtime blockers, or migration risks.

The research spec defines Phase 5 as the point where `usync` becomes doctor-first in the TUI and gains Gemini CLI to Antigravity migration UX. The current codebase has `pkg/manifest` and the Phase 2-4 plan/apply/validate layers, but the working tree does not currently contain `pkg/doctor`. Phase 5 therefore must keep the dashboard implementation gated behind a read-only doctor scanner and must not duplicate scan logic inside `pkg/tui`.

## Goals

- Replace the default TUI entry point with a doctor dashboard once `pkg/doctor` is available.
- Preserve the current provider-first wizard behind `usync --wizard`.
- Render immediately with a spinner or loading state before scan completion.
- Show detected clients, effective config paths, existing MCP providers, conflicts, runtime blockers, and provider readiness.
- Integrate Phase 4 offline credential validation into the TUI credential step.
- Keep live credential validation explicit through a user action.
- Add a symlink-safe `usync migrate gemini-to-antigravity` CLI.
- Add TUI migration affordances for Gemini CLI sunset and Antigravity path conflicts.
- Do not mutate configs from doctor/dashboard screens.
- Keep all output redacted.

## Non-Goals

- Do not implement MCP server mode.
- Do not add a dynamic MCP registry.
- Do not add team or remote state.
- Do not rewrite the saved-plan apply engine.
- Do not make live validation automatic during TUI startup.
- Do not delete Gemini CLI config files during migration.
- Do not remove or replace Antigravity symlinks.
- Do not write project/workspace configs by default.
- Do not duplicate doctor scanning in `pkg/tui`.

## Users or Actors

- New users launching `usync` with unknown local MCP setup.
- Existing users affected by Gemini CLI sunset or Antigravity path changes.
- Users with multiple AI clients installed who need a single readiness overview.
- Users who prefer the current wizard and need a compatibility path during transition.
- Future automation or TUI components that depend on a stable doctor report.

## Dependency Gate

Phase 5 dashboard work depends on a completed read-only doctor scanner:

- `pkg/doctor` must expose scan report types and `Doctor.Scan(ctx)`.
- `usync doctor` must already be available or be completed as a prerequisite repair track.
- `pkg/tui` must consume doctor report data through exported APIs; it must not parse config files directly.

If `pkg/doctor` is still absent when implementation starts, the safe path is:

1. Complete the missing Phase 1b doctor scanner first.
2. Then implement Phase 5 dashboard and migration UX.

Migration CLI work can proceed in parallel only if it uses `pkg/manifest`, `pkg/config`, and read-only parse helpers without depending on dashboard state.

## External Audit Findings

Exa research on May 22, 2026 found three planning corrections:

- Official Google sources confirm the Gemini CLI to Antigravity CLI transition and the June 18, 2026 consumer sunset, but they do not by themselves define a stable MCP config file migration contract.
- Public issue evidence reports configuration fragmentation between Gemini CLI, Antigravity CLI, and Antigravity IDE paths. Treat those reports as empirical signals, not authoritative API contracts.
- Bubble Tea's documented model supports asynchronous I/O through `tea.Cmd` messages, so dashboard scan work should run as commands and update the model by message rather than blocking `Init` or `View`.

Phase 5 must therefore verify local paths at runtime with `pkg/doctor` and `pkg/manifest` rather than hardcoding a single Antigravity target path in TUI code.

## Functional Requirements

- **FR-1:** `usync` with no subcommand must start the doctor dashboard by default after Phase 5 lands.
- **FR-2:** `usync --wizard` must preserve the current provider-first wizard behavior.
- **FR-3:** The first TUI render must not block on filesystem or runtime scans.
- **FR-4:** Dashboard scan must run asynchronously and update the model when complete.
- **FR-5:** Dashboard must show a useful empty state when no supported clients are detected.
- **FR-6:** Dashboard must show detected clients, status, effective path, and configured MCP provider IDs.
- **FR-7:** Dashboard must show conflicts before provider selection.
- **FR-8:** Dashboard must show Gemini CLI consumer sunset warnings until July 15, 2026 when relevant, without implying enterprise users lose access.
- **FR-9:** Dashboard must show Antigravity symlink status and resolved target when detected.
- **FR-10:** Conflict resolution must let users choose current path, legacy path, or skip for the session.
- **FR-11:** Conflict resolution choices must feed target selection and saved-plan generation without mutating files directly.
- **FR-12:** Provider credential entry must reuse `pkg/validate` for offline validation.
- **FR-13:** Live validation in TUI must require explicit user action and use Phase 4 cache behavior.
- **FR-14:** Provider readiness must distinguish no-key-needed, ready-with-credentials, missing-key, runtime-missing, and conflict-blocked states.
- **FR-15:** Get-key URLs from `pkg/manifest` must be visible for missing credentials.
- **FR-16:** TUI preview must use saved-plan APIs instead of the legacy in-memory plan when running from the dashboard path.
- **FR-17:** TUI apply must call saved-plan apply APIs and preserve approval gates for creates, symlinks, and workspace/project writes.
- **FR-18:** `usync migrate gemini-to-antigravity --dry-run` must render a redacted migration preview and write nothing.
- **FR-19:** `usync migrate gemini-to-antigravity --apply` must copy MCP entries from Gemini CLI config to Antigravity config with backup and verification.
- **FR-20:** Migration apply must write to the resolved symlink target when Antigravity config is a symlink.
- **FR-21:** Migration apply must refuse symlink targets outside the configured home directory.
- **FR-22:** Migration must not delete Gemini CLI source files.
- **FR-23:** Migration output and dashboard output must not show raw credentials, credential-bearing URLs, headers, env values, or command args.
- **FR-24:** Existing CLI plan/apply/validate behavior must remain unchanged.
- **FR-25:** Existing TUI tests must either pass unchanged through `--wizard` or be deliberately updated for the new default dashboard.

## Migration Behavior

Phase 5 must distinguish Antigravity CLI and Antigravity IDE:

- Gemini CLI uses `settings.json` with an `mcpServers` block for global or project-level MCP configuration.
- Antigravity IDE has been observed using `~/.gemini/antigravity/mcp_config.json`.
- Antigravity CLI path reports include `~/.gemini/antigravity-cli/mcp_config.json` and post-migration `~/.gemini/config/mcp_config.json`.

Because public evidence is not a stable product contract, implementation must resolve the effective target from manifest + doctor findings. Migration must not assume the IDE path is also the CLI path.

Source candidates:

- `~/.gemini/settings.json`
- `.gemini/settings.json` when a workspace is explicitly supplied
- legacy Gemini MCP config candidates from `pkg/manifest`

Target candidates:

- Antigravity CLI current candidates from `pkg/manifest`
- `~/.gemini/antigravity/mcp_config.json`
- any manifest-selected Antigravity IDE current path

Dry-run output must include:

- target type: Antigravity CLI or Antigravity IDE
- source path
- target display path
- resolved target path when target is a symlink
- provider IDs that would be copied
- entries skipped because already present
- backup path preview
- warnings for parse errors, missing source, missing target parent, and symlink target outside home

Apply rules:

- Parse source and target as JSON object configs.
- Copy only MCP server entries under the expected MCP root.
- Preserve existing target entries unless the source provider entry is identical or user-approved overwrite behavior is explicitly implemented later.
- Write via the existing locked backup write path.
- Verify target parse health after write.
- Record an audit entry without secrets if audit integration is already available.

If more than one plausible current Antigravity target exists, dry-run must report a conflict and require explicit target selection before apply.

## TUI Screen Requirements

Dashboard:

- Header shows `usync` once.
- Loading state appears immediately.
- Summary includes clients detected, ready clients, conflicts, and existing MCP provider IDs.
- Main table includes client name, status, effective path, and configured MCP provider IDs.
- Provider readiness groups include ready now, with supplied keys, needs keys, and blocked.
- Keyboard actions include configure providers, resolve conflict, migrate Gemini, wizard, refresh, and quit.

Conflict resolution:

- Shows current and legacy candidates with path, symlink status, resolved target, last modified time when available, and configured provider IDs.
- Recommends the highest-confidence non-deprecated candidate unless the doctor report marks a conflict.
- Does not write anything.

Provider credential entry:

- Shows no-key providers separately.
- Shows missing credential providers with get-key URLs.
- Runs offline validation before preview.
- Provides explicit live validation action.

Preview and apply:

- Use saved-plan preview and saved-plan apply APIs.
- Show redacted operations, warnings, skipped clients, approval gates, and verification results.

## Acceptance Criteria

- `usync --wizard` opens the current wizard path.
- Default `usync` opens dashboard mode once `pkg/doctor` is available.
- First dashboard render is immediate and does not wait for scan completion.
- A no-client fixture renders a non-empty "nothing detected" state.
- Antigravity conflict appears before provider selection.
- Gemini CLI consumer sunset warning appears when source config exists and the current date is on or before July 15, 2026; wording must note that enterprise access may differ.
- TUI offline validation rejects malformed credentials before preview.
- TUI live validation is opt-in and uses mock HTTP in tests.
- `usync migrate gemini-to-antigravity --dry-run` writes nothing and emits a redacted preview.
- `usync migrate gemini-to-antigravity --apply` writes to the resolved symlink target and does not remove the symlink.
- Migration refuses symlink targets outside home.
- Existing `go test ./pkg/tui ./cmd/usync` passes.
- `go test ./...`, `make lint`, `make test`, and `make build` pass before implementation is marked complete.

## Success Criteria

- The interactive experience becomes status-first instead of credential-first.
- Users can see what is already installed and configured before choosing providers.
- Gemini to Antigravity migration is discoverable and safe.
- The TUI calls the same doctor, validate, plan, and apply APIs as the CLI.
- Phase 6 MCP server mode can reuse stable read-only APIs without depending on TUI internals.

## Edge Cases

- `pkg/doctor` scan takes longer than the timeout.
- Scan returns partial results and warnings.
- No supported clients are detected.
- Runtime commands are missing or slow.
- Antigravity target path is a broken symlink.
- Antigravity symlink target resolves outside home.
- Gemini source config is malformed.
- Gemini source has no MCP servers.
- Target already has one or more provider entries.
- Workspace paths are unset.
- Terminal window is narrow.
- User enters malformed credentials, then fixes them.
- Live validation is rate-limited.
- Saved-plan apply fails after dashboard-driven plan creation.

## Data Sensitivity and Compliance Notes

- Dashboard and migration previews must show provider IDs, not full configs.
- Raw credentials must remain in memory only.
- Config content must not be printed.
- Credential-bearing URLs must be redacted before display.
- Migration backups may contain local config data and must be `0600`.
- Audit entries must not include source config content or raw credentials.

## Assumptions

- macOS and Linux remain the supported platforms.
- `pkg/manifest` remains the metadata source for clients, candidates, credential URLs, runtimes, and Gemini sunset metadata.
- `pkg/validate` is available from Phase 4.
- Saved-plan apply is available from Phase 3.
- The old wizard can stay as a separate code path during Phase 5 rather than being deleted.

## Open Questions

- Should the old wizard be kept permanently or removed after one release?
- Should migration overwrite an existing target provider entry automatically when source and target differ? Recommendation: dry-run reports conflict and apply skips unless a future `--overwrite` flag is approved.
- Should dashboard refresh rerun runtime checks every time or reuse a short-lived in-memory result?
- Should the migration command support `--home-dir` only for tests, or expose it as a documented user flag?
- Should dashboard save plans only, or also offer one-step save-and-apply after preview? Recommendation: both, using existing approval gates.
- Should migration default to Antigravity CLI or Antigravity IDE when both are present? Recommendation: report conflict and require `--target antigravity-cli|antigravity-ide`.

## Human Approval Status

Approved to plan. Implementation approval pending.
