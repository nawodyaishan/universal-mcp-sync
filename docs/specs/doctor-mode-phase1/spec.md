# Doctor Mode Phase 1: Client Manifests and Read-Only Doctor

## Problem Statement

The current client detection logic is encoded in `pkg/config/paths.go` as a static table that returns one chosen path per target. That is enough for the current apply flow, but it is not enough for doctor mode.

Doctor mode needs to enumerate every known candidate path, distinguish current and legacy locations, report parse and root-key problems, detect already configured MCP providers, surface runtime prerequisites, and do all of that without writing files. Recent Antigravity path churn showed that hardcoded single-path detection is brittle and makes migration guidance difficult.

Phase 1 introduces the read-only foundation for the larger `doctor -> validate -> plan -> apply` workflow.

## Goals

- Add `pkg/manifest` as the static source of truth for supported clients, config candidates, provider credential links, and runtime requirements.
- Keep `pkg/manifest` independent from existing internal packages to avoid import cycles and premature rewiring.
- Add `pkg/doctor` as a read-only scanner over manifests and the local filesystem.
- Add `usync doctor` and `usync doctor --json`.
- Detect installed/configured supported tools, existing MCP provider entries, malformed config files, legacy/current path conflicts, symlinks, writability, and runtime availability.
- Preserve current `usync`, `sync`, `--dry-run`, and `--apply` behavior.
- Create enough stable JSON output for future plan, validation, and TUI dashboard work.

## Non-Goals

- Do not change the existing apply engine.
- Do not replace `pkg/config.DetectAppConfigsForOS` in this phase.
- Do not make the TUI doctor-first yet.
- Do not add saved plan files.
- Do not validate credentials live.
- Do not write, migrate, repair, normalize, or format any client config file.
- Do not add MCP gateway or MCP server mode.
- Do not add a dynamic external registry.

## Users or Actors

- New users who want to know which supported AI tools are installed or already configured.
- Existing users affected by config path drift or malformed local config files.
- Contributors preparing for batch plan/apply.
- Future CI or automation consuming `usync doctor --json`.

## Functional Requirements

- **FR-1:** `pkg/manifest` must define all supported client IDs currently known to the repo.
- **FR-2:** `pkg/manifest` must define current and legacy config candidates for macOS and Linux where they differ.
- **FR-3:** `pkg/manifest` must represent config format separately from existing mutation shape.
- **FR-4:** `pkg/manifest` must use no imports from `github.com/nawodyaishan/universal-mcp-sync/pkg/...`.
- **FR-5:** `pkg/manifest` must expose path expansion for `{{.Home}}` and `{{.Workspace}}`.
- **FR-6:** Deprecated candidates must include a replacement label or a documented reason when no replacement exists.
- **FR-7:** Provider metadata must include credential key names, acquisition URLs, docs URLs, offline format hints, and whether the credential is required.
- **FR-8:** Runtime metadata must include command, args, install URL, and provider/client reasons.
- **FR-9:** `pkg/doctor` must scan candidates using `os.Lstat` so symlinks can be reported without following them blindly.
- **FR-10:** `pkg/doctor` must not write files.
- **FR-11:** `pkg/doctor` must parse JSON, simple JSONC, and Codex TOML enough to report parse health and provider presence.
- **FR-12:** `pkg/doctor` must report whether the expected root key exists and is object-shaped.
- **FR-13:** `pkg/doctor` must report provider IDs already configured in each candidate file.
- **FR-14:** `pkg/doctor` must compute per-client confidence: `high`, `medium`, `low`, or `conflict`.
- **FR-15:** `pkg/doctor` must generate migration hints for Antigravity/Gemini and Windsurf legacy paths when detected.
- **FR-16:** `pkg/doctor` must run bounded runtime checks and report command availability without failing the whole scan.
- **FR-17:** `usync doctor --json` must emit deterministic JSON for fixture homes.
- **FR-18:** `usync doctor` human output must be redacted and must not print secrets, config values, or provider URLs containing credentials.
- **FR-19:** `usync doctor` exit codes must be stable: `0` for clean/no critical issues, `2` for warnings or detected issues, `1` for command failure.
- **FR-20:** Existing CLI/TUI flows must continue to pass unchanged.

## Acceptance Criteria

- `pkg/manifest` exists with tests and has no internal package imports.
- All currently supported app IDs from `pkg/config.AppOrder` are represented in manifests, including `antigravity-cli`.
- Provider metadata exists for Exa, GitHub, Context7, Tavily, Playwright, Kubernetes, and Terraform.
- Runtime metadata exists for at least `node`, `npx`, `docker`, and the CLI-managed clients already modeled by the repo.
- `pkg/doctor` exists with fixture-backed tests for healthy, partial, malformed, legacy/conflict, and symlink cases.
- `usync doctor --json --home-dir <fixture>` produces stable output across two runs.
- Doctor tests prove no files are created or modified during scan.
- `pkg/doctor` does not import `pkg/app` or `pkg/tui`.
- `go test ./pkg/manifest ./pkg/doctor ./cmd/usync` passes.
- `go test ./...` passes.
- `make test` passes before implementation is marked complete.

## Success Criteria

- Phase 2 can build batch plan selection from doctor findings without re-discovering client config locations.
- The current apply path remains stable while doctor mode becomes the new read-only discovery surface.
- Antigravity/Gemini and Windsurf path drift is visible to users before they apply changes.
- New providers can add metadata links and credential hints without adding TUI-specific branches.

## Edge Cases

- Home directory path contains spaces.
- Workspace path is unset.
- Candidate path parent directory does not exist.
- Candidate file exists but is unreadable.
- Candidate file is a symlink.
- Symlink target is missing.
- Symlink target resolves outside home.
- JSON file is empty.
- JSON file has the right root key with the wrong type.
- JSONC file contains line comments.
- TOML file has malformed syntax.
- Two current candidates exist for one client.
- Only a deprecated legacy candidate exists.
- Runtime command is missing or times out.
- Provider-like keys appear outside the MCP root and must not be counted as configured providers.

## Data Sensitivity and Compliance Notes

- Doctor output must list provider IDs, not full provider configs.
- Doctor output must not include raw API keys, URLs with secret query params, headers, or command arguments that contain secrets.
- Provider acquisition links are allowed because they are public URLs.
- Parse errors should include file path and parser context, but not raw config content.
- JSON output is intended for automation and must be redacted by default.

## Assumptions

- macOS and Linux remain the supported platforms for Phase 1.
- Runtime checks can use `exec.LookPath` plus short command execution only when needed for version output.
- JSONC support can be minimal in Phase 1: strip line comments outside quoted strings or use a small local helper, with no new dependency unless the implementation proves brittle.
- Codex TOML parse health can start with targeted section scanning if adding a TOML parser would expand dependency risk.
- Manifest source references can include URLs and verification dates, but Phase 1 does not need to re-browse during tests.

## Open Questions

- Should `pkg/manifest` include source URLs in the initial PR, or should source references stay in docs until the metadata stabilizes?
- Should doctor scan workspace/project candidates by default, or only when `--workspace` is supplied?
- Should `usync doctor` run runtime commands by default, or should version checks require `--runtimes` to keep scans fast?
- Should Antigravity canonical path be `~/.gemini/config/mcp_config.json` or `~/.gemini/antigravity/mcp_config.json` for the first implementation? This must be verified before coding that manifest entry.
- Should malformed JSONC be parsed with a dependency or a local comment stripper?

## Human Approval Status

Approved to plan. Implementation approval pending.
