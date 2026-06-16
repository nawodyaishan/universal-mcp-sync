# Doctor Mode Phase 5 Implementation Plan

## Summary

Phase 5 should ship as three implementation tracks, with the dashboard gated on the missing doctor scanner:

- **PR 5a:** Doctor dependency closure and TUI shell split.
- **PR 5b:** Dashboard, conflict resolution, readiness, and validation UX.
- **PR 5c:** Gemini CLI to Antigravity migration CLI and TUI card.

The safe path is to avoid putting filesystem scanning logic in `pkg/tui`. The TUI should render doctor reports, validation reports, saved plans, and apply results from the same packages used by the CLI.

## Inputs Reviewed

- `docs/research/Doctor Mode, Batch Plan:Apply, and Credential Validation new spec.md`
- `docs/specs/doctor-mode-phase0/spec.md`
- `docs/specs/doctor-mode-phase1/spec.md`
- `docs/specs/doctor-mode-phase2/spec.md`
- `docs/specs/doctor-mode-phase3/spec.md`
- `docs/specs/doctor-mode-phase4/spec.md`
- `pkg/tui/model.go`
- `pkg/tui/setup_form.go`
- `pkg/tui/preview.go`
- `pkg/tui/results.go`
- `pkg/manifest/clients.go`
- `pkg/validate/*`
- `cmd/usync/main.go`
- `cmd/usync/plan_commands.go`
- `cmd/usync/apply_command.go`

## Current Codebase Findings

- `pkg/tui` currently starts at `stageSetup` and uses a provider-first wizard.
- `cmd/usync/main.go` currently launches `tui.NewModel(...)` for the no-subcommand path.
- `pkg/manifest` exists and already models Gemini CLI, Antigravity CLI, Antigravity IDE, path-dispute warnings, and symlink hints.
- `pkg/validate` exists and can support TUI offline/live credential validation.
- Saved-plan apply exists and should be the dashboard apply path.
- `pkg/doctor` is not present in the current working tree. This is the main Phase 5 blocker.

## External Audit Notes

Exa research changed the migration risk model:

- Google's May 19, 2026 transition posts confirm Antigravity CLI availability and the June 18, 2026 consumer sunset for Gemini CLI and Gemini Code Assist IDE extensions.
- Google also says enterprise access is different, so warning text must say consumer/free/Pro/Ultra rather than treating every Gemini CLI install as dead after June 18.
- Public GitHub issue evidence points to MCP config fragmentation: Gemini CLI `~/.gemini/settings.json`, Antigravity IDE `~/.gemini/antigravity/mcp_config.json`, and Antigravity CLI reports around `~/.gemini/antigravity-cli/mcp_config.json` or `~/.gemini/config/mcp_config.json`.
- That path evidence is useful but not authoritative. Phase 5 should add a manifest reconciliation task and rely on doctor findings at runtime.
- Bubble Tea docs support the planned async dashboard shape: long-running I/O belongs in `tea.Cmd`, then returns messages into `Update`.

Sources checked:

- Google Developers Blog: https://developers.googleblog.com/an-important-update-transitioning-gemini-cli-to-antigravity-cli/
- Google Blog I/O 2026 highlights: https://blog.google/innovation-and-ai/technology/developers-tools/google-io-2026-developer-highlights/
- Gemini CLI MCP configuration documentation PR: https://github.com/google-gemini/gemini-cli/pull/6556
- Bubble Tea command tutorial: https://github.com/charmbracelet/bubbletea/blob/main/tutorials/commands/README.md
- Empirical Antigravity CLI path issue: https://github.com/google-antigravity/antigravity-cli/issues/60

## Key Design Decisions

### 1. Dashboard Uses Doctor Reports Only

`pkg/tui` must not read or parse config files. It should receive or request a doctor scan through a small interface:

```go
type DoctorScanner interface {
    Scan(ctx context.Context) (doctor.Report, error)
}
```

The concrete implementation can wrap `pkg/doctor.Doctor`. Tests can inject fake reports.

### 2. Keep The Old Wizard

The current `tui.Model` should remain available during the transition. Add a new dashboard model rather than deleting the wizard:

- `tui.NewWizardModel(...)` for the existing flow.
- `tui.NewDashboardModel(...)` for the new default flow.
- `usync --wizard` calls the wizard.
- default `usync` calls the dashboard after the dependency gate is satisfied.

This reduces regression risk and gives existing users a fallback.

### 3. Async Scan Command

Bubble Tea should start with a first render that does not require the scan result. Use a command that returns a scan message:

```go
type scanStartedMsg struct{}
type scanCompleteMsg struct {
    report doctor.Report
    err    error
    partial bool
}
```

The scan command should use a timeout. If scan exceeds the timeout, the model should display partial/loading state. If the doctor API cannot provide partial results in the first implementation, the dashboard should show "scanning..." placeholders until completion rather than blocking first paint.

### 4. Provider Readiness Is A View Model

Do not extend provider implementations for dashboard display. Build a TUI-local or app-level view model from:

- doctor report client findings
- manifest provider metadata
- validation report status
- runtime findings
- current credential inputs

Recommended package location:

- Start in `pkg/tui/dashboard_readiness.go` if it only formats UI state.
- Move to `pkg/app` later only if CLI and MCP server mode need the same readiness view.

### 5. Migration Should Be A Package, Not TUI Logic

Add a migration package or config-level helper:

```go
package migrate

type GeminiToAntigravityOptions struct {
    HomeDir string
    Now func() time.Time
    DryRun bool
    AutoApprove bool
}

func PreviewGeminiToAntigravity(opts GeminiToAntigravityOptions) (Plan, error)
func ApplyGeminiToAntigravity(opts GeminiToAntigravityOptions) (Result, error)
```

Recommended package: `pkg/migrate`.

Reasons:

- It is not a generic config write helper.
- It has domain-specific source/target semantics.
- It can depend on `pkg/manifest`, `pkg/config`, `pkg/redact`, and `pkg/audit` without polluting `pkg/config`.
- The CLI and TUI can call the same code.

The package must accept an explicit target kind when ambiguity exists:

```go
type AntigravityTargetKind string

const (
    TargetAntigravityCLI AntigravityTargetKind = "antigravity-cli"
    TargetAntigravityIDE AntigravityTargetKind = "antigravity-ide"
)
```

When both target kinds are present and no target is provided, preview should return a conflict result rather than choosing silently.

### 6. Migration Conflict Policy

For Phase 5, migration should be conservative:

- Copy source provider entries that do not exist in target.
- Skip target entries that already exist.
- Report differing duplicate provider IDs as conflicts.
- Do not overwrite target entries unless a later approved `--overwrite` flag is added.
- Report ambiguous Antigravity CLI vs IDE targets as conflicts.

This avoids silently replacing a newer Antigravity config with an older Gemini CLI entry.

### 7. Saved-Plan Path From Dashboard

Dashboard-driven setup should use saved plans:

1. Doctor scan identifies target candidates.
2. User resolves conflicts and selects providers/targets.
3. Offline validation runs.
4. TUI saves a plan through `pkg/app`.
5. Preview renders saved-plan format.
6. Apply calls saved-plan apply.

The old wizard can continue using legacy in-memory `ExecutionPlan` until it is retired.

## Affected Modules

- `cmd/usync/main.go`
- `cmd/usync/migrate_command.go` new
- `pkg/tui/model.go`
- `pkg/tui/dashboard.go` new
- `pkg/tui/dashboard_readiness.go` new
- `pkg/tui/conflict.go` new
- `pkg/tui/migration.go` new
- `pkg/tui/model_test.go`
- `pkg/migrate/gemini_antigravity.go` new
- `pkg/migrate/gemini_antigravity_test.go` new
- `pkg/manifest/clients.go` only if sunset metadata needs exported constants
- `pkg/doctor/*` if the Phase 1b dependency is not implemented yet

## Dependency Changes

No new external dependencies should be needed for Phase 5. Use existing Bubble Tea, Huh, standard library JSON handling, and current config write helpers.

## Implementation Tracks

### PR 5a: Dependency Closure And TUI Entry Split

1. Confirm `pkg/doctor` exists and has fixture-backed scan tests.
2. If absent, complete the Phase 1b doctor scanner before dashboard work.
3. Rename or wrap current `NewModel` as `NewWizardModel`.
4. Add default dispatch:
   - `usync --wizard` -> wizard
   - `usync` -> dashboard
5. Add tests proving `--wizard` preserves old flow.

Do not add migration apply in this PR.

### PR 5b: Dashboard And Validation UX

1. Add `DashboardModel` with loading, loaded, error, and empty states.
2. Add fake scanner tests so UI logic is deterministic.
3. Render client table and provider readiness groups.
4. Add conflict resolution model backed by doctor report candidates.
5. Wire provider credential entry to `pkg/validate.Offline`.
6. Add explicit live validation action using `pkg/validate.Service`.
7. Save and preview plans via saved-plan APIs.
8. Apply through saved-plan apply APIs with existing approval gates.

Do not implement migration writes in this PR unless PR 5c is already complete.

### PR 5c: Migration CLI And TUI Card

1. Reconcile manifest path metadata for Gemini CLI, Antigravity CLI, and Antigravity IDE.
2. Add `pkg/migrate` Gemini to Antigravity preview/apply.
3. Add `usync migrate gemini-to-antigravity --dry-run`.
4. Add `usync migrate gemini-to-antigravity --apply`.
5. Add `--target antigravity-cli|antigravity-ide` when more than one plausible target exists.
6. Add symlink handling with `os.Lstat` and `filepath.EvalSymlinks`.
7. Enforce resolved target within home.
8. Write target through `config.WriteWithBackup`.
9. Add dashboard migration card and action.
10. Add audit logging if apply succeeds or fails after a write attempt.

## Testing Strategy

Use fixture homes and fake scanners. Avoid relying on actual local AI tool installs.

Required test categories:

- TUI first render does not require scan completion.
- Dashboard empty state.
- Dashboard conflict state for Antigravity.
- Credential validation state transitions.
- Live validation via mock HTTP only.
- Wizard fallback through `--wizard`.
- Migration dry-run writes nothing.
- Migration apply writes through resolved symlink target.
- Migration refuses symlink target outside home.
- Migration preserves source Gemini config.
- Migration ambiguous target case requires explicit target selection.
- Output redaction for migration preview and dashboard strings.

Suggested commands:

```text
go test ./pkg/tui ./pkg/migrate ./cmd/usync
go test ./...
make lint
make test
make build
```

## Risks and Mitigations

- **Risk:** Dashboard work starts before doctor scanner exists.
  **Mitigation:** Make `pkg/doctor` an implementation gate and use fake scanner interfaces only for TUI unit tests.

- **Risk:** TUI model becomes too large.
  **Mitigation:** Keep dashboard, conflict, migration, setup, preview, and results as separate sub-models.

- **Risk:** Migration overwrites newer Antigravity config.
  **Mitigation:** Skip or conflict existing target entries; no automatic overwrite in Phase 5.

- **Risk:** Migration writes to the wrong Antigravity surface.
  **Mitigation:** Model Antigravity CLI and IDE as separate target kinds, require explicit target when both are plausible, and keep path selection driven by manifest + doctor findings.

- **Risk:** Symlink migration breaks Antigravity config.
  **Mitigation:** Use `Lstat`, write to resolved target only, refuse outside-home targets, never remove symlink.

- **Risk:** TUI tests become timing-flaky.
  **Mitigation:** Test async scan via injected commands/messages and fake scanners, not real timers.

- **Risk:** Live validation surprises users.
  **Mitigation:** Offline validation runs automatically; live validation only runs after explicit action.

## Rollout Plan

1. Land doctor dependency closure if needed.
2. Land `--wizard` fallback and dashboard skeleton.
3. Land dashboard report rendering.
4. Land validation integration.
5. Land migration CLI.
6. Land migration card/action in dashboard.
7. After one release, decide whether old wizard remains permanent or becomes a compatibility flag only.

## Human Architecture Approval Status

Pending approval to implement.
