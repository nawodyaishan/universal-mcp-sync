# Doctor Mode Phase 7: TUI Doctor Dashboard Foundation

## Problem Statement

The CLI now has a read-only doctor scanner, manifest-backed path discovery, provider-neutral plan/apply plumbing, and a `providers` readiness command. The default interactive `usync` entry point still opens the legacy provider-first wizard, so new users do not first see what is installed, what is already configured as MCP, what is conflicted, or what is blocked by missing runtimes.

Phase 7 changes the default interactive experience to a status-first doctor dashboard while keeping the existing wizard behind `usync --wizard`. This phase is deliberately read-only. It must not implement provider credential setup, saved-plan preview/apply, conflict writes, or migration writes.

## Goals

- Add a `DashboardModel` in `pkg/tui`.
- Make default `usync` open the dashboard.
- Preserve the current provider-first flow through `usync --wizard`.
- Render immediately before doctor scan completion.
- Run doctor scanning through Bubble Tea `tea.Cmd` and typed messages.
- Show detected clients, confidence, effective paths, existing MCP providers, runtime blockers, warnings, and conflicts.
- Provide clear navigation actions for refresh, wizard, configure, resolve, migrate, and quit.
- Keep all filesystem/runtime facts flowing from `pkg/doctor`.
- Establish testable dashboard foundations for Phase 8 and Phase 10.

## Audit Notes

- This phase was re-audited against the original doctor/batch research docs and MCP research on May 23, 2026.
- The original research spec includes a fuller dashboard journey with provider readiness, get-key URLs, credential validation, saved-plan preview/apply, and migration UX. Phase 7 deliberately stops before those write-capable and credential-entry flows; Phase 8 through Phase 10 complete that journey.
- Current Antigravity documentation exposes multiple MCP surfaces, including editor raw config, Antigravity CLI config, and CLI plugin `mcp_config.json` files. Phase 7 must render manifest-provided client/scope labels and must not hardcode a single Antigravity path in `pkg/tui`.
- The codebase is pinned to Bubble Tea v1.3.10, so Phase 7 must keep the current `View() string` interface rather than adopting newer v2-style examples.

## Non-Goals

- Do not write client config files.
- Do not implement saved-plan preview/apply in the dashboard.
- Do not implement provider credential entry in the dashboard.
- Do not implement live validation in the dashboard.
- Do not implement Gemini to Antigravity migration logic.
- Do not parse MCP config files from `pkg/tui`.
- Do not remove or rewrite the legacy wizard.
- Do not add MCP Server Mode.

## Users

- New users launching `usync` with unknown local MCP state.
- Existing users with several AI tools installed who need a quick status view.
- Users with Antigravity/Gemini path conflicts who need to see blockers before setup.
- Existing wizard users who still need the old flow during transition.

## Functional Requirements

- **FR-1:** `usync` with no subcommand and without `--wizard`, `--dry-run`, or `--apply` launches the dashboard.
- **FR-2:** `usync --wizard` launches the existing provider-first wizard without dashboard state.
- **FR-3:** Dashboard first render shows a loading/scanning state immediately.
- **FR-4:** Doctor scan runs in a `tea.Cmd`; `Init` must return the command and must not call scan directly.
- **FR-5:** Scan completion is delivered to `Update` through a typed message containing either `doctor.Report` or an error.
- **FR-6:** Dashboard supports loading, loaded, empty, error, refresh, and last-known-report states. It must not require a streaming or partial-result doctor API in this phase.
- **FR-7:** Dashboard summary shows total detected clients, ready clients, conflicts, warnings, and existing MCP provider IDs.
- **FR-8:** Client list shows name, confidence/status, effective path or missing state, configured provider IDs, and important warning count.
- **FR-9:** Runtime blockers are rendered from doctor runtime findings.
- **FR-10:** Conflict clients are visually prioritized above ordinary ready/missing clients.
- **FR-11:** Empty home renders a useful non-empty state: no supported AI clients detected, with available actions.
- **FR-12:** Key handling supports `r` refresh, `w` wizard, `c` configure placeholder, `x` resolve conflict placeholder, `m` migrate placeholder, `?` help, `q`/`ctrl+c` quit.
- **FR-13:** Placeholder actions must not mutate files; they may set a status message such as "available in Phase 8".
- **FR-14:** The dashboard must handle narrow terminal widths without incoherent overlap.
- **FR-15:** Raw config contents, raw credentials, credential-bearing URLs, headers, env values, and command args must not be rendered.
- **FR-16:** `w` must either set a final-model wizard route flag and quit the dashboard, or remain a read-only placeholder. It must not launch a nested Bubble Tea program from inside `Update`.

## UX Requirements

- The first screen must feel like an operational dashboard, not a marketing or setup form.
- The main hierarchy is:
  1. system summary
  2. blockers/conflicts
  3. detected clients
  4. provider IDs already configured
  5. next actions
- Use concise labels suitable for repeated terminal use: `ready`, `conflict`, `missing`, `cli-only`, `runtime-missing`.
- Prefer dense but readable text tables over decorative cards.
- Keep the palette consistent with existing `pkg/tui` styles; do not introduce a new visual system in Phase 7.
- Keep status text stable for tests and docs.
- Long paths should be shortened with existing home-relative path behavior where possible.
- Do not rely on color alone; status text must carry meaning.

## Data Model Requirements

Add a small injectable scanner interface in `pkg/tui`, for example:

```go
type DashboardScanner interface {
	Scan(ctx context.Context) (doctor.Report, error)
}
```

The production scanner should wrap `pkg/doctor`. Tests should use fakes that return deterministic reports and errors.

The production scanner should enable runtime checks and pass the same home/workspace context used by CLI doctor flows. It may bound scan duration with context or doctor command timeouts, but `View` must remain pure and non-blocking.

The dashboard model should own only view state:

- scanner dependency
- scan status
- last report
- last error
- selected row cursor
- width/height
- help visibility
- transient status message

It must not own raw credentials or parsed config content.

## Technical Requirements

- Use `github.com/charmbracelet/bubbletea v1.3.10`, matching `go.mod`.
- Use `tea.Cmd` for scan/refresh commands.
- Use typed messages such as `dashboardScanStartedMsg`, `dashboardScanFinishedMsg`, and `dashboardOpenWizardMsg`.
- Do not block `Init`, `Update`, or `View` on filesystem/runtime scanning.
- `View` must be side-effect free.
- Keep dashboard code in new files, preferably:
  - `pkg/tui/dashboard.go`
  - `pkg/tui/dashboard_view.go`
  - `pkg/tui/dashboard_test.go`
- Keep scanner wiring in a small constructor path:
  - `NewDashboardModel(manager *app.Manager) DashboardModel`
  - optional `NewDashboardModelWithScanner(...)` for tests.
- `cmd/usync/main.go` should dispatch default interactive mode to dashboard and `--wizard` to `NewWizardModel`.

## Testing Requirements

Use the testing approach in `docs/research/TUI-Testing-with-Go.md`:

- Unit-test direct `Init`, `Update`, and `View` behavior first.
- Test async command behavior without sleeps by invoking returned commands directly where possible.
- Use fake scanners for deterministic loaded/error reports.
- Add `teatest` coverage only for one or two black-box interactive flows.
- Use `teatest.WaitFor` for async output; do not add arbitrary sleeps.
- Normalize colors or assert stable text fragments instead of brittle full ANSI output unless a golden test is intentionally added.
- Keep `View` side-effect free so final model/view assertions are reliable.

Required tests:

- First render contains scanning/loading text before command completion.
- `Init` returns a non-nil command.
- Scan command returns typed completion message.
- Loaded report renders summary, configured provider IDs, and client rows.
- Empty report renders "No AI clients detected" or equivalent stable copy.
- Conflict report renders conflict before ordinary clients.
- Error report renders partial/error state without panic.
- `r` triggers a new scan command.
- `q` and `ctrl+c` quit.
- `w` produces a route/action that `cmd/usync` can use to open wizard or exits dashboard with a wizard signal, depending on chosen implementation.
- `usync --wizard` still preserves current wizard behavior.
- Default `usync` starts dashboard mode.

## Acceptance Criteria

- Default `usync` opens dashboard mode.
- `usync --wizard` opens the legacy wizard.
- First dashboard render does not wait for doctor scan completion.
- Doctor scan work runs in `tea.Cmd` and updates state through typed messages.
- Dashboard never imports or calls config parse/write helpers.
- Dashboard renders a useful empty state.
- Dashboard renders conflicts, warnings, runtime blockers, configured provider IDs, and client status.
- Placeholder actions are clearly labeled and read-only.
- Existing wizard tests still pass.
- `go test ./pkg/tui ./cmd/usync` passes.
- `go test ./...`, `make lint`, `make build`, and `make test` pass before implementation is marked complete.

## References

- Local testing guide: `docs/research/TUI-Testing-with-Go.md`
- Bubble Tea command tutorial: https://github.com/charmbracelet/bubbletea/blob/main/tutorials/commands/README.md
- Bubble Tea command batching reference: https://github.com/charmbracelet/bubbletea/blob/main/commands.go
- Charmbracelet `teatest` package: https://pkg.go.dev/github.com/charmbracelet/x/exp/teatest
- Charmbracelet `teatest` example notes: https://github.com/charmbracelet/bubbletea/pull/352
