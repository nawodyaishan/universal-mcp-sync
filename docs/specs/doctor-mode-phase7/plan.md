# Doctor Mode Phase 7 Implementation Plan

## Summary

Phase 7 should be implemented as a dashboard foundation PR. It changes the default TUI entry from provider-first wizard to doctor-first dashboard, but keeps the dashboard read-only. The implementation should be small enough that Phase 8 can add provider readiness, validation, saved-plan preview, and apply without rewriting this foundation.

## Inputs Reviewed

- `docs/specs/doctor-mode-remaining-implementation-plan.md`
- `docs/research/Doctor Mode, Batch Plan:Apply, and Credential Validation new spec.md`
- `docs/research/TUI-Testing-with-Go.md`
- `pkg/tui/model.go`
- `pkg/tui/helpers.go`
- `pkg/tui/model_test.go`
- `pkg/doctor/*`
- `cmd/usync/main.go`
- Bubble Tea command docs and `teatest` MCP research references

## Current Codebase Findings

- `NewWizardModel` already wraps the old provider-first flow.
- `cmd/usync/main.go` currently still launches `NewWizardModel` for default interactive mode.
- `pkg/doctor` exists and is read-only.
- `usync doctor` supports JSON/human output, client filtering, verbose output, and runtime scanning.
- `pkg/tui` currently has wizard-specific stage rendering: Setup, Assign, Preview, Results.
- Existing styles in `pkg/tui/helpers.go` are sufficient for a compact dashboard.
- `teatest` is already in `go.mod`, so no dependency is needed.

## MCP Research Notes

- Bubble Tea docs confirm `Cmd` is the correct place for I/O, including filesystem/runtime work.
- Bubble Tea renders the initial view after `Init` returns, before async command results are processed. This supports immediate loading UI.
- `tea.Batch` can be used if dashboard needs spinner ticks plus scan command, but Phase 7 can start without spinner dependency if stable text is enough.
- `teatest.WaitFor` is the right harness for async output; direct `Update/View` unit tests should cover most dashboard logic.
- Avoid arbitrary sleeps in tests. Use fake scanner commands directly or `teatest.WaitFor`.
- Exa/Tavily audit on May 23, 2026 confirmed that the current Bubble Tea upstream examples increasingly show v2 import paths and `tea.View`; this repo is still pinned to `github.com/charmbracelet/bubbletea v1.3.10`, so Phase 7 implementation must keep the existing v1 `View() string` model signature.
- Current Antigravity docs now expose multiple MCP surfaces: editor raw config, Antigravity CLI config, and CLI plugin `mcp_config.json` files. The dashboard must label findings by manifest client/scope and must not imply there is one authoritative Antigravity MCP path.

## Original Spec Audit Corrections

- The original Phase 5 spec also includes provider readiness, get-key URLs, saved-plan preview/apply, and migration UX. Phase 7 intentionally implements only the read-only dashboard foundation; those larger user-journey items remain Phase 8 through Phase 10 work.
- The dashboard should reserve layout space and stable labels for later provider readiness, but it must not attempt credential collection, live validation, saved-plan preview, or apply in Phase 7.
- "Partial/error" in Phase 7 means the dashboard can render a scan error and, on refresh, keep the last successful report if one exists. It does not require changing `pkg/doctor.Scan(ctx)` to stream partial results.
- The production dashboard scanner should construct `doctor.Options` with the current home/workspace inputs, `CheckRuntimes: true`, and a bounded command timeout. Tests should inject fake scanners and must not depend on local installed tools.

## Architecture Decision

Add a new dashboard model instead of changing the wizard model into a polymorphic model.

Reason:

- The existing wizard has state and dependencies specific to provider setup.
- Dashboard state is report/status/navigation focused.
- A separate model prevents Phase 7 from destabilizing wizard preview/apply behavior.
- Later phases can route from dashboard to wizard or new setup flows deliberately.

## Proposed Structure

New files:

- `pkg/tui/dashboard.go`
- `pkg/tui/dashboard_view.go`
- `pkg/tui/dashboard_test.go`

Possible command-line changes:

- `cmd/usync/main.go`

Optional small test helpers:

- `pkg/tui/dashboard_fixtures_test.go`

## Model Shape

Recommended types:

```go
type DashboardScanner interface {
	Scan(ctx context.Context) (doctor.Report, error)
}

type DashboardModel struct {
	scanner DashboardScanner
	state dashboardState
	report doctor.Report
	err error
	cursor int
	width int
	height int
	showHelp bool
	status string
}
```

Recommended states:

```go
const (
	dashboardLoading dashboardState = iota
	dashboardLoaded
	dashboardEmpty
	dashboardError
)
```

Recommended messages:

```go
type dashboardScanFinishedMsg struct {
	report doctor.Report
	err error
}
```

## Command Design

`DashboardModel.Init()` should:

- set state to loading in the constructor or zero state
- return a scan command
- avoid calling scanner directly

Scan command:

```go
func scanDashboardCmd(scanner DashboardScanner) tea.Cmd {
	return func() tea.Msg {
		report, err := scanner.Scan(context.Background())
		return dashboardScanFinishedMsg{report: report, err: err}
	}
}
```

If cancellation or timeout is needed, wrap it inside the production scanner, not in `View`.

## Rendering Design

Render order:

1. Header/status line
2. Summary metrics
3. Blockers/conflicts
4. Client rows
5. Existing configured provider IDs
6. Action bar

Use stable text fragments for tests:

- `Scanning local MCP setup`
- `No AI clients detected`
- `Conflicts`
- `Runtime blockers`
- `Configured providers`
- `r refresh`
- `w wizard`

Do not overinvest in complex table layout yet. Phase 7 should favor predictable text wrapping and truncation over a custom table engine.

## UX Plan

The dashboard should feel operational and compact:

- Keep one shell header.
- Use one dashboard body, not nested cards.
- Put blockers above ordinary rows.
- Keep actions always visible.
- Use consistent status labels.
- Do not show explanatory paragraphs about how the product works.

Suggested first render:

```text
Doctor Dashboard

Scanning local MCP setup...

[r refresh] [w wizard] [q quit]
```

Suggested loaded summary:

```text
Doctor Dashboard

Clients  4 detected  3 ready  1 conflict
Providers configured  exa, context7

Conflicts
- Antigravity IDE  multiple current config candidates exist

Clients
- Claude Desktop  ready  ~/Library/.../claude_desktop_config.json  context7
- Codex CLI       ready  ~/.codex/config.toml                      exa
```

## Routing Plan

Phase 7 has two acceptable routing designs. Prefer option A unless implementation shows a blocker.

Option A:

- `w` sets a `wizardRequested` flag on `DashboardModel` and returns `tea.Quit`.
- `cmd/usync/main.go` runs dashboard first.
- If the final model indicates wizard was requested, run the existing wizard program after dashboard exits.

Option B:

- `w` sets a status message only in Phase 7.
- `usync --wizard` is the supported wizard route.

Option A gives a better UX but is slightly more complex because `cmd/usync` must inspect final model state. Option B is safer if the final model plumbing becomes noisy. Either option must preserve `usync --wizard`.

## Testing Plan

Layer 1: Direct unit tests

- construct `DashboardModel` with fake scanner
- call `Init` and assert non-nil command
- call returned command and assert `dashboardScanFinishedMsg`
- feed message into `Update`
- assert state and `View` text

Layer 2: View tests

- loaded report
- empty report
- conflict report
- runtime blocker report
- narrow width report

Layer 3: Minimal `teatest`

- start dashboard with fake scanner
- wait for loading text or loaded text using `teatest.WaitFor`
- send `q`
- wait finished

Layer 4: CLI tests

- verify default `run([]string{})` enters dashboard path using a testable constructor or command-level helper
- verify `--wizard` still enters wizard path

Avoid tests that require actual local AI tools, network, or real terminal state.

## Quality Gates

- No config parsing in `pkg/tui`.
- No filesystem writes in dashboard code.
- No credentials in dashboard model fields or rendered strings.
- `View` remains side-effect free.
- All async work is in commands.
- Tests do not use real sleeps.
- Keep `go test ./pkg/tui ./cmd/usync` fast.

## Implementation Sequence

1. Add dashboard model with fake scanner tests.
2. Add dashboard view rendering for loading, empty, loaded, conflict, and error states.
3. Add refresh and quit key handling.
4. Add wizard route behavior.
5. Wire default CLI interactive path to dashboard.
6. Add CLI tests for default dashboard and `--wizard`.
7. Run targeted and full verification.

## Risks

- The current `renderShell` assumes wizard stages. Dashboard can either bypass it or add a dashboard-specific shell to avoid pretending dashboard is "Setup".
- Running a full Bubble Tea program in CLI tests can hang if the dashboard does not have an injectable quit path. Prefer unit tests and constructor-level routing tests for command dispatch.
- Snapshot tests can be brittle because terminal output contains ANSI and frame history. Prefer stable `View()` strings for most assertions.

## Final Verification

```text
go test ./pkg/tui ./cmd/usync
go test ./...
make lint
make build
make test
```
