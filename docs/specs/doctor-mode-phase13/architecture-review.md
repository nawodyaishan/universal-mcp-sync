# usync — Architecture & Code-Quality Review

**Reviewer perspective:** senior Go engineer + software architect, external read.
**Last updated:** 2026-05-23
**Codebase snapshot:** main branch, Phase 11/12/13a landed; Phase 13 matrix in progress.
**Scope:** structural risks affecting long-term scalability and QA confidence. Not a style audit; not a security audit.

---

## TL;DR

`usync` has shipped a working product across 13 doctor-mode phases. The hot path (`doctor → plan → apply`) is well-tested, well-redacted, and atomic on the filesystem. The deferred bill is structural:

1. **`pkg/tui/dashboard.go` is a 1074-line god file** with a 28-field `DashboardModel` and a 9-method `DashboardManager` interface that has three overloaded `PrepareProvider` variants. Every new screen risks a regression because everything lives in one switch.
2. **The `DashboardManager` interface violates ISP** and forces the TUI to know plan/apply/preflight/validate semantics. Each new transport (CLI, programmatic SDK, MCP server mode) re-derives the same logic.
3. **Cross-package coupling is acceptable** (`go test ./...` confirms `pkg/doctor` has an `import_guard_test.go`), but layering is informal — `pkg/tui` reaches into `pkg/app`, `pkg/manifest`, `pkg/config`, `pkg/provider`, `pkg/doctor`, `pkg/validate`. A ports/adapters layer would let alternative front-ends share the core.
4. **`context.Background()` is created inside command closures** (`pkg/tui/dashboard.go:198,223,239`) instead of being plumbed from the request boundary. Cancellation and timeouts cannot be added later without rewriting the closures.
5. **Error returns are partially wrapped.** `grep` over `pkg/app` and `pkg/tui` shows ~99 error returns, only ~39% wrapped with `%w`. Failure messages are usable but root causes get truncated.
6. **The test harness has three overlapping runners** (`go test`, `USYNC_UX_MATRIX=1`, `make ux-fake-prod`) with skip-on-env-flag gating. The "authoritative" run only happens in Docker, which is friction for fast iteration and means CI must run Docker.
7. **Global registries** (`provider.DefaultRegistry()`, `manifest.AllProviders()`) are convenient but defeat dependency injection — adding a provider plugin or testing with a subset requires monkey-patching or new constructors.

None of these are emergencies. They become emergencies when a fifth dashboard screen, a sixth integration target, or a multi-tenant scenario lands.

---

## Methodology

- Read every file under `pkg/tui/`, the `DashboardManager` interface, the `app.Manager`/`app.SavedPlan` surface, and the top-level entry in `cmd/usync/main.go`.
- Ran CodeGraph `impact`/`callees` queries on the highest-fan-out symbols: `ComputeReadiness`, `renderProviderReady`, `PrepareProvider`, `DashboardModel`.
- Counted: file LoC, error-wrapping ratio, interface methods, struct fields, `context.Background()` call sites.
- Compared against Go community guidance ([go-proverbs](https://go-proverbs.github.io/), [Effective Go interface design](https://go.dev/doc/effective_go#interface-design), [the standard project layout discussion](https://github.com/golang-standards/project-layout)) and against the Phase 12 audit (`docs/specs/doctor-mode-phase12/user-flow-audit.md`) which is the closest internal precedent.

---

## Findings

Each finding is graded `Critical / Major / Minor` against long-term scalability or QA. Quick-win fixes are flagged.

### A1 — God file: `pkg/tui/dashboard.go` (1074 LoC)

**Severity:** Major

`pkg/tui/dashboard.go` mixes:

- Two interfaces (`DashboardScanner`, `DashboardManager`).
- 8 typed messages (`scanResultMsg`, `providerReadinessMsg`, `validationResultMsg`, `planCreatedMsg`, `preflightResultMsg`, `dashApplyResultMsg`, plus implicit `tea.KeyMsg` handling).
- The `DashboardModel` struct (28 fields including `RouteToWizard`, `showHelp`, `width`, plus three booleans per long-running op).
- Six `handleKey<Screen>` functions.
- Five `*Cmd` constructors (`scanCmd`, `readinessCmd`, `offlineValidationCmd`, `liveValidationCmd`, `applyCmd`, `planCmd`).
- The `Update`/`Init` dispatch and the `dashboardScreen` enum.
- The conflict-resolution overlay state.

Any future screen (e.g. credential setup, batch progress, doctor-only mode) compounds the cyclomatic complexity. Tests in `pkg/tui/dashboard_test.go` already reach 1000+ lines.

**Solution:**

1. Split into a screen package each: `pkg/tui/dashboard/doctor`, `.../provider`, `.../target`, `.../conflict`, `.../preview`, `.../result`. Each owns: state struct, key handler, renderer, message types.
2. Promote a `dashboard.Model` parent that composes children via `tea.Cmd` wiring — Bubble Tea is designed for this (charm calls it the "elm-architecture-of-models" pattern).
3. Move all shared helpers (`waitForText`, `keyMsg`, `RenderedProviderIndices`) to `pkg/tui/internal/uxkit`.
4. Forbid further growth of `dashboard.go` via a lint rule (e.g. `golangci-lint funlen` or a custom CI grep on LoC).

**Effort:** 1–2 weeks. High blast radius — must land all at once or behind a feature flag.

---

### A2 — Interface bloat: `DashboardManager` (9 methods, 3 overloaded `PrepareProvider`)

**Severity:** Major

```go
type DashboardManager interface {
    PrepareProvider(...)                          // app IDs only
    PrepareProviderWithTargetPaths(...)           // app IDs + path overrides
    PrepareProviderWithTargetFiles(...)           // app IDs + file overrides (Phase 12c)
    BuildSavedPlan(...)
    PreflightSavedPlan(...)
    ApplySavedPlan(...)
    Validate(...)
    HomeDir() string
}
```

Issues:

- **Interface Segregation Principle:** the TUI consumes all methods, but a CLI or programmatic SDK only needs `BuildSavedPlan`+`ApplySavedPlan`. A future MCP-server mode might need only `Validate`. The interface is shaped by the most demanding caller.
- **API evolution scar tissue:** three `PrepareProvider*` overloads are the visible cost of three migrations (path-only → path overrides → file overrides). Each one was added without retiring the previous.
- **`HomeDir() string` is leaking infrastructure** — the manager shouldn't be the source of truth for the user's home directory; the entry point should resolve it once and inject it where needed.

**Solution:**

1. Define small role interfaces (`PlanBuilder`, `PlanApplier`, `Validator`) and have `DashboardModel` depend on the union via a struct (not one fat interface).
2. Collapse `PrepareProvider*` into a single method that takes a `PlanRequest` struct; deprecate the overloads behind a vet tag for one release.
3. Remove `HomeDir()` from the manager; pass home as a value in `NewDashboardModel`.
4. Add the role interfaces to a `pkg/app/ports` subpackage to make the dependency direction explicit (TUI → ports → app).

**Effort:** 3–4 days for the refactor; ~1 day for the call-site sweep.

---

### A3 — God struct: `DashboardModel` (28 fields)

**Severity:** Major

Fields cover scanning state, manager wiring, screen enum, six in-flight booleans (`scanning`, `validating`, `planning`, `preflighting`, `applying`, `computingReady`), two cursors (`providerCursor`, `clientCursor`), three result/error pairs (`validReport`/`validErr`, `currentPlan`/`planErr`, `applyResult`/`applyErr`), help state, width, two selection maps, and the Phase 12 conflict overlay state.

Risks:

- Mutation in `Update(msg)` requires reasoning over 28 fields. The Phase 13 anchor bug existed precisely because cursor state could go out of sync with rendered state.
- Equality / snapshot testing is impossible — golden tests have to compare strings.
- Goroutine safety is non-obvious: `DashboardModel` is passed by value through the Bubble Tea reducer, but pointer fields (maps, pointers to plan/preflight/result) share aliasing.

**Solution:**

1. Group fields into named sub-structs: `ScanState`, `ProviderState`, `TargetState`, `ConflictState`, `PlanState`, `ApplyState`, `UIState`. Each becomes ~3–5 fields.
2. Make each sub-struct an interface for the screen package it serves (after A1). The parent `Model` composes them.
3. Replace the six boolean flags with a single `Phase` enum per workflow (`PhaseIdle | PhaseScanning | PhaseValidating | ...`). Mutually exclusive states should not be independent booleans.
4. Add invariants like `m.Validate()` returning an error if the state machine is in an impossible combination (e.g. `validating && applying`).

**Effort:** 2–3 days; high readability payoff.

---

### A4 — `context.Background()` inside command closures (cancellation leak)

**Severity:** Major (test reliability + future-proofing)

`pkg/tui/dashboard.go:198,223,239`, `pkg/app/app.go:919`, and 5 doctor-test sites construct `context.Background()` *inside* the closure that becomes a `tea.Cmd`. Consequences:

- The Bubble Tea program cannot cancel an in-flight `Validate` or `Scan` when the user quits — the closure runs to completion. Today this is invisible because validation is fast; tomorrow with `pkg/validate/live.go` doing real HTTP calls it will hold the process open after `tea.Quit`.
- Tests cannot inject deadlines.
- `tea.Cmd` already provides cancellation via program teardown; the closure should accept a context from the caller.

**Solution:**

```go
// Today
func (m DashboardModel) scanCmd() tea.Cmd {
    return func() tea.Msg {
        report, err := m.scanner.Scan(context.Background())
        return scanResultMsg{report: report, err: err}
    }
}

// Recommended
func (m DashboardModel) scanCmd(ctx context.Context) tea.Cmd {
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
        defer cancel()
        report, err := m.scanner.Scan(ctx)
        return scanResultMsg{report: report, err: err}
    }
}
```

Plumb a long-lived `programCtx` from `cmd/usync/main.go` into `NewDashboardModel`, derive per-op contexts. Phase out every `context.Background()` outside `main` and `_test.go`.

**Effort:** ~half a day, mechanical.

---

### A5 — Error wrapping is inconsistent

**Severity:** Minor (today) → Major (when production debugging starts)

`grep -c "fmt.Errorf|errors.New"` against `pkg/app` and `pkg/tui` returned 99; `grep "fmt.Errorf.*%w"` returned 39. About **60% of errors lose their wrap chain**. Two failure modes:

- A user sees `Plan error: at least one credential profile is required` — but the originating `provider.MCPProvider.RequiredCredentials()` site is invisible in stack traces and audit logs.
- The audit log (Phase 11 FR-8 redaction guard) cannot tell whether two identical messages came from the same root.

**Solution:**

1. Adopt `fmt.Errorf("context: %w", err)` everywhere a returned error originated below.
2. Prefer typed errors (`errors.Is` / `errors.As`) for boundary errors that the TUI distinguishes — e.g. `var ErrMissingCredentials = errors.New(...)` instead of string matching `"at least one credential profile is required"`.
3. Add `make lint` / `go vet`-friendly check: forbid bare `errors.New` outside `*_errors.go` files.

**Effort:** ~1 day sweep; recurring discipline.

---

### A6 — Global registries defeat injection

**Severity:** Minor

`provider.DefaultRegistry()` (`pkg/tui/dashboard.go:217,233`) and `manifest.AllProviders()` (`pkg/tui/dashboard.go:207`) are package-level singletons.

- Adding a private/experimental provider requires editing the registry's `init()`.
- Tests that need a single-provider subset have to swap the global, which Go does not allow safely under `t.Parallel()`.
- Future plugin model (load providers from a config file or a Go plugin) cannot coexist with a static registry.

**Solution:**

1. Inject `Registry` into `DashboardModel` and `app.Manager`. Keep `provider.DefaultRegistry()` as the production wiring in `cmd/usync/main.go`.
2. Tests construct a registry with whatever subset they need.
3. Optional: define `Registry` as an interface so future plugin loaders satisfy it.

**Effort:** ~half a day; trivial.

---

### A7 — Three overlapping test runners

**Severity:** Major (slows iteration, masks failures)

Today:

| Runner | What it runs | When |
|---|---|---|
| `go test ./...` | unit + golden + most teatest | every dev cycle |
| `USYNC_UX_MATRIX=1 go test ./pkg/tui -run TestDashboardFlowMatrix` | matrix teatests | manual or `make ux-matrix` |
| `make ux-fake-prod` (Docker) | PTY + fake home + matrix + CLI plan + doctor JSON | "authoritative" per protocol |

Issues:

- **Skipping by default** means the matrix tests are only ever exercised in CI and on demand. A change that breaks DM-P20 will land green locally.
- **Docker dependency for the "authoritative" run** makes CI heavy (image build ≈ 10 s warm, ≈ 60 s cold) and forces every contributor onto Docker for the full bug-hunt loop.
- **Three layers of fakes** (`FakeScanner`, `FakeDashboardManager`, `fakeProdDashboardManager`) duplicate behavior; behavior drift between them is silent.

**Solution:**

1. Default the matrix to **on**; have a `-short` flag to skip rather than skip-by-default. The protocol requires the matrix to grow — make it cheap to run.
2. Promote `make ux-fake-prod` to a thin wrapper that runs the same tests with a real PTY but **no Docker rebuild on every run**. The Docker layer is there for hermeticity; cache the image in CI and reuse locally.
3. Consolidate fakes into one `pkg/tui/uxtest` package with builder helpers (`uxtest.WithConflict()`, `uxtest.WithProvider("exa")`, …). One source of truth.
4. Add a CI matrix tier: `quick` (go test only), `extended` (matrix env on), `full` (Docker harness on nightly + PR-touching-tui).

**Effort:** 2–3 days; pays back continuously.

---

### A8 — Layering is informal (no ports/adapters boundary)

**Severity:** Major (limits reuse)

The dependency graph today (read off imports):

```
cmd/usync ──► pkg/tui ──► pkg/app, pkg/manifest, pkg/config, pkg/provider, pkg/doctor, pkg/validate, pkg/verify, pkg/audit
                  │
                  └──► pkg/redact
```

`pkg/tui` knows about every internal package. This works for one front-end, but the Phase 12 audit and Phase 13 spec both call out "share target discovery between CLI and TUI" and "future thin MCP adapter". Each new front-end will re-derive the same TUI knowledge.

**Solution (ports/adapters):**

1. Define a `pkg/core` (or `pkg/usync/core`) package that owns the domain types (`Report`, `SavedPlan`, `ApplyResult`, …) and the use-case interfaces (`ScanService`, `PlanService`, `ApplyService`).
2. `pkg/app` becomes an adapter that satisfies the services using `pkg/config`, `pkg/manifest`, etc.
3. `pkg/tui` and `cmd/usync/plan_commands.go` depend only on `pkg/core` interfaces. A future MCP-server adapter does the same.
4. `pkg/doctor/import_guard_test.go` already proves the team understands import discipline — extend the same guard test to enforce the layering rules.

**Effort:** 1 week if scoped carefully; high payoff for adding CLI features or new transports.

---

### A9 — Public surface in `pkg/tui` is over-exported

**Severity:** Minor

Types like `DashboardModel`, `DashboardManager`, `ProviderReadinessItem`, `ConflictResolution`, `RenderedProviderIndices` are exported. Only `NewDashboardModel`, `Init`, `Update`, `View`, and `RouteToWizard` need to be public for `cmd/usync` to launch the program. The exported model surface is large enough that tests use it freely — which is *good for tests* and *bad for refactors*, because each rename is a public-API break.

**Solution:**

1. Make tests `package tui` (white-box) — they already do. Then unexport `DashboardModel.scanning` (already lower-case), `RenderedProviderIndices`, `ProviderReadinessItem`, etc.
2. Expose a thin `Run(opts)` entry-point that hides the model from `cmd/usync/main.go`.

**Effort:** half a day. Counts as preparation for A1.

---

### A10 — Manifest data lives in code (`pkg/manifest/clients.go` 512 LoC)

**Severity:** Minor — but a future scalability issue

Adding a client today means editing `pkg/manifest/clients.go` and `pkg/manifest/providers.go` by hand. The Phase 11 spec already added `SourceRef.Confidence` to mark which entries are from official docs vs. community reports. With 12 clients today and an obvious roadmap for more, the file is a moderate maintenance hotspot.

**Solution:**

1. Move manifest data to a YAML or TOML resource (or `embed.FS` of one file per client) so adding a client is a data change.
2. Keep the Go types but generate the slice from the resource at startup.
3. Or, more conservatively, group the existing code by client into one file per client (`pkg/manifest/clients/antigravity.go`, `…/claude_code.go`).

**Effort:** 1 day if YAML, 2 hours if file-per-client.

---

### A11 — `pkg/audit` and `pkg/redact` are well-isolated — keep them

**Severity:** Positive observation

These two packages are exactly the right size: single responsibility, no leaking abstractions, well-tested. They are a template for what other packages could look like after the recommendations above. Phase 11 FR-1 (rotation) and FR-8 (redaction regression) demonstrate that the team can lock invariants when the package is small enough to reason about.

No action required.

---

### A12 — `Makefile` and `scripts/*.sh` are an undocumented contract

**Severity:** Minor

`make ux-fake-prod` shells into `tests/ux-fake-prod/docker-run.sh`, which in turn runs `tests/ux-fake-prod/run-flow.sh` inside Docker. CI presumably depends on the same targets. None of these are documented in a single place; a contributor inherits the contract by reading code.

**Solution:**

1. Add `docs/contributing/test-runners.md` (or a CLAUDE.md section) describing the runner hierarchy.
2. Add `make help` to print the public make targets with descriptions.
3. Codify the bug-hunt loop's 4 stages as `make ux-stage1`, `make ux-stage2`, … so the protocol becomes runnable rather than narrative.

**Effort:** 2 hours.

---

## Priority Matrix

| Finding | Severity | Effort | Order |
|---|---|---|---|
| A4 context plumbing | Major | 0.5d | 1 |
| A6 inject registries | Minor | 0.5d | 2 |
| A2 collapse `PrepareProvider*` overloads | Major | 3–4d | 3 |
| A7 unify test runners + fakes | Major | 2–3d | 4 |
| A5 error-wrapping sweep | Minor→Major | 1d | 5 |
| A9 unexport TUI internals | Minor | 0.5d | 6 |
| A3 split `DashboardModel` into sub-structs | Major | 2–3d | 7 |
| A1 split `dashboard.go` per screen | Major | 1–2w | 8 |
| A8 ports/adapters layer | Major | 1w | 9 |
| A10 manifest as data | Minor | 1d | 10 |
| A12 document contracts | Minor | 2h | 11 |

The order above is "smallest dependent first." A4 and A6 unblock A2; A2 + A3 unblock A1; A1 + A8 together unlock multi-front-end work.

---

## Quality-Assurance Risks

These are the QA invariants that the current structure makes hard to enforce:

| Risk | Source finding | Mitigation |
|---|---|---|
| A new screen's key handler silently overrides a global key | A1 | Per-screen package with explicit `Keys() []Key` registration; meta-test asserts no key collides across screens |
| A new dashboard field is mutated only in one of two paths | A3 | Sub-structs with constructors; field zero-value is a meaningful state |
| Background command runs after Quit | A4 | Context-aware closures + program teardown cancellation test |
| A provider added to `pkg/provider` is invisible to the registry | A6 | Replace `init()` registration with explicit `Registry.Register` calls; assert in `TestRegistry_AllManifestProvidersRegistered` |
| Matrix scenario passes locally but fails in Docker | A7 | Promote local matrix to default-on; reuse fakes across runners |
| Plan/apply contract drifts between CLI and TUI | A8 | Both consume `pkg/core.PlanService`; one set of tests covers both |

---

## What Not To Do

The temptation after a review like this is to schedule a rewrite. Don't. The Phase 11/12 work has solid invariants (audit rotation, plan hash, conflict resolution) that depend on the *current* package shapes. Take A4, A5, A6 first — they are low-blast-radius — then evaluate whether A1/A3 are still painful before paying their cost.

Also: avoid trendy abstractions. `usync` has no need for an ORM, no need for an event bus, no need for code generation beyond what `go:embed` already provides. Each new dependency is a future Phase-N obligation.

---

## Open Questions For The Maintainer

1. Is there a roadmap for a non-TUI front-end (CLI batch, MCP-server adapter, library)? If yes, A8 is critical and should be promoted.
2. Is the bug-hunt protocol's "Docker is authoritative" rule load-bearing for CI, or is it a documentation artifact? If the latter, A7 can simplify the runners.
3. Is the `RouteToWizard` legacy escape hatch staying long-term, or will it be removed? Its presence keeps two parallel entry points alive in `cmd/usync/main.go`.
4. Phase 11 added `SourceRef.Confidence`; Phase 13 hasn't proposed any manifest changes. Is a manifest-data refactor (A10) on anyone's radar?

---

## Closing

`usync` is a well-disciplined codebase. The phase-by-phase delivery model has produced invariants that hold — the redaction guard, audit rotation, plan content hash, and conflict resolution are real wins. The findings above are not criticisms of the work that landed; they are the predictable cost of shipping fast through 13 phases without a structural retrospective.

If only one finding is acted on: **A4 (context plumbing)**. Half a day, removes a real risk, and is a prerequisite for everything else.
