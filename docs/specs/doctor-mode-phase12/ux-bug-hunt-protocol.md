# Doctor Mode Phase 12 UX Bug Hunt Protocol

> **Superseded by Phase 14 — see [ux-bug-hunt-protocol-v2.md](../doctor-mode-phase14/ux-bug-hunt-protocol-v2.md)**
> Kept for history. The 15 invariants (I-01..I-15) and matrix dimensions are still authoritative;
> the manual screenshot → matrix row process is superseded by the Phase 14 state-space explorer.

**Purpose:** find TUI flow bugs by running the product like a user, then confirming failures in code.  
**Primary output:** a Docker-collected issue matrix with transcripts, fake-home artifacts, and repeatable tests.  
**Scope:** Doctor dashboard, provider readiness, target selection, conflict resolution, plan preview, apply result.

## Why This Exists

Code reading finds implementation risks. It does not reliably find UX state-machine bugs.

The strongest signal comes from this order:

1. Run an exact user path.
2. Record what the screen promises.
3. Press the advertised key.
4. Compare the result against product invariants.
5. Confirm the code path only after the behavior is observed.

Screenshots are useful evidence, but they should become matrix cases and tests. Do not leave them as one-off bug reports.

## Automation Strategy

The bug hunt must run inside Docker with mocked staged scenarios. Local tests are only a developer shortcut; they are not the authoritative collection run.

Use Bubble Tea `teatest` as the Playwright-style runner inside the Docker environment. The automation should not use a browser.

The runner should:

- Build the real `usync` binary in the image.
- Create a fake production `$HOME`, workspace, PATH, runtimes, and MCP config files.
- Run Docker-only matrix tests against the real scanner and app manager when the scenario depends on filesystem discovery.
- Start `DashboardModel` with fake scanner, fake manager, and chosen credential profiles only for fast state-machine cases.
- Send real key inputs.
- Capture stable visible text after each action.
- Capture final model state with `FinalModel`.
- Record fake manager calls: validation, plan, preflight, apply.
- Assert product invariants, not only screen names.
- Export all data to `artifacts/ux-fake-prod`.

Recommended file:

```text
tests/ux-fake-prod/Dockerfile
tests/ux-fake-prod/run-flow.sh
pkg/tui/dashboard_fake_prod_matrix_test.go
pkg/tui/dashboard_flow_matrix_test.go
```

Recommended matrix files:

```text
docs/specs/doctor-mode-phase12/ux-flow-matrix.md
artifacts/ux-fake-prod/matrix.json
```

Required staged scenario outputs:

- `artifacts/ux-fake-prod/matrix.json`
- `artifacts/ux-fake-prod/issues.json`
- `artifacts/ux-fake-prod/fake-prod-matrix.txt`
- `artifacts/ux-fake-prod/teatest-matrix.txt`
- `artifacts/ux-fake-prod/doctor.json`
- `artifacts/ux-fake-prod/flows/*.ansi`
- `artifacts/ux-fake-prod/flows/*.txt`
- `artifacts/ux-fake-prod/plans/*.json`
- `artifacts/ux-fake-prod/home-after/`

## Matrix Dimensions

Keep dimensions small and intentional. Add a dimension only when it changes user behavior.

| Dimension | Values |
|---|---|
| Credentials | none, valid, invalid |
| Provider type | requires credentials, no-key provider |
| Conflicts | none, unresolved, resolved-first, resolved-second, skipped |
| Targets | none, one, many, all-unchecked, mixed-checked |
| Workspace | off, on |
| Validation | success, failure |
| Plan | success, failure |
| Preflight | clean, warnings, failure |
| Apply | success, failure |
| Rescan | unchanged, conflict remains, conflict resolved |

Do not test every Cartesian combination. Test meaningful intersections where user promises can break.

## Core Invariants

Every flow case should assert at least one invariant.

| ID | Invariant |
|---|---|
| I-01 | Every visible footer action must be valid for the current screen state. |
| I-02 | Hidden keys must not advance the product flow. |
| I-03 | Repeating the primary action must not repeat the same unrecoverable error. |
| I-04 | Credential-required providers cannot plan with zero credential profiles. |
| I-05 | No-key providers remain plannable without credentials. |
| I-06 | Visible checkbox state must equal planned target state. |
| I-07 | No selected targets means no plan command starts. |
| I-08 | Unresolved conflicts block planning or route to resolution. |
| I-09 | Resolved conflicts stop looking blocked. |
| I-10 | A chosen conflict candidate path must reach the planning layer. |
| I-11 | A skipped conflict must not be planned. |
| I-12 | Workspace toggle must change target eligibility or be removed. |
| I-13 | Every error state must show a recovery action. |
| I-14 | Esc must return to a useful previous state without data loss. |
| I-15 | No raw credential appears in captured output, errors, preview, or result. |

## Case Record Format

Use this format when adding a matrix case manually or from a screenshot.

```md
### DM-P31 Missing credentials at target planning

Preconditions:
- credentials: none
- provider: Exa
- conflicts: unresolved
- targets: many

Keys:
- p
- r
- Enter

Visible promise before action:
- Footer says `[Enter] plan`

Expected:
- Planning is blocked before `PrepareProvider`
- User sees credential recovery action

Actual:
- `Plan error: at least one credential profile is required`
- Footer still says `[Enter] plan`

Invariant failures:
- I-03
- I-04
- I-13

Code confirmation:
- `planCmd`
- `app.Manager.PrepareProvider`
- `renderTargetSelect`

Test:
- `TestDashboardFlow_MissingCredentialsBlocksBeforePlan`
```

## Flow Families

### F-01 Entry And Scan

Paths:

- launch -> scan success
- launch -> scan failure -> rescan
- launch -> wizard route
- launch -> providers route

Key risks:

- Provider route silently blocked.
- Scan error lacks recovery copy.
- Existing MCP inventory is visible but not actionable.

### F-02 Provider Readiness

Paths:

- no conflicts -> select ready provider
- no conflicts -> live validate
- no conflicts -> hidden `r`
- unresolved conflicts -> `r`
- unresolved conflicts -> `Enter`
- unresolved conflicts -> live validate
- missing credentials -> try to continue

Key risks:

- Hidden rows remain navigable.
- Conflict banner and footer disagree with key handling.
- Missing credentials are detected too late.

### F-03 Target Selection

Paths:

- selected target -> plan
- checked row -> Space -> plan
- all rows unchecked -> Enter
- no targets -> Enter
- workspace off -> on -> plan
- conflict row -> resolve

Key risks:

- Visual checkbox state does not match planned state.
- Empty/no-op plan can start.
- Workspace toggle is cosmetic.
- Footer advertises actions that cannot succeed.

### F-04 Conflict Resolution

Paths:

- conflict row -> r -> choose 1 -> plan
- conflict row -> r -> choose 2 -> plan
- conflict row -> r -> s -> plan
- conflict row -> r -> Esc
- conflict with one candidate -> render -> Esc
- conflict with zero accessible candidates -> render -> s

Key risks:

- Chosen path is not passed to planning.
- Skip is only a visual exclusion.
- Overlay lacks enough evidence to choose safely.
- Resolved conflicts still appear globally blocked.

### F-05 Plan Preview

Paths:

- clean plan -> y
- clean plan -> n
- warning preflight -> y
- preflight failure -> recovery
- generated plan includes redacted provider config

Key risks:

- Preview does not show exact chosen path.
- Approval prompts are auto-approved without a clear contract.
- Esc/cancel loses or keeps the wrong state.

### F-06 Apply Result

Paths:

- apply success -> rescan
- apply failure -> rescan or stay
- apply success with previous conflict resolution
- result -> r
- result -> q

Key risks:

- Rescan resets user decisions unexpectedly.
- Error result lacks next action.
- Raw credentials leak in result text.

## Matrix Runner Design

Represent each case as data.

```go
type flowCase struct {
    name       string
    setup      flowSetup
    keys       []string
    checkpoints []checkpoint
    invariants []invariant
}

type flowSetup struct {
    credentials credentialState
    provider    providerState
    report      doctor.Report
    manager     fakeManagerState
}

type checkpoint struct {
    afterKey int
    contains []string
    excludes []string
}
```

The fake manager should record calls.

```go
type fakeManagerCalls struct {
    validateCount int
    planCount     int
    preflightCount int
    applyCount    int
    selectedApps  []config.AppID
    selectedPaths []string
}
```

Each test should assert:

- visible screen text
- final screen enum
- error state
- selected target state
- manager calls
- redaction

## Minimum Automated Cases

Start with these. They cover the highest-risk intersections.

| Test | Matrix focus | Main invariant |
|---|---|---|
| `TestDashboardFlow_MissingCredentialsBlocksBeforePlan` | credentials none + target plan | I-03, I-04, I-13 |
| `TestDashboardFlow_DeselectOnlyTargetBlocksPlan` | one target + unchecked | I-06, I-07 |
| `TestDashboardFlow_DeselectOneOfManyPlansOnlyChecked` | many targets + mixed checked | I-06 |
| `TestDashboardFlow_ConflictEnterRoutesToResolution` | unresolved conflict + Enter | I-08 |
| `TestDashboardFlow_ConflictChooseFirstPlansChosenPath` | conflict resolved-first + plan | I-10 |
| `TestDashboardFlow_ConflictChooseSecondPlansChosenPath` | conflict resolved-second + plan | I-10 |
| `TestDashboardFlow_ConflictSkipDoesNotPlan` | conflict skipped + plan | I-11 |
| `TestDashboardFlow_ResolvedConflictClearsReadinessBlocker` | resolved conflict + back | I-09 |
| `TestDashboardFlow_WorkspaceToggleChangesTargets` | workspace off/on | I-12 |
| `TestDashboardFlow_NoRawCredentialFullJourney` | valid credentials + full flow | I-15 |

## Screenshot Triage

When a screenshot reveals a bug:

1. Name the screen.
2. Write the preconditions.
3. Copy the visible footer actions.
4. Record the exact key that failed.
5. Mark invariant failures.
6. Add a matrix case.
7. Only then inspect code and link locations.

Do not update the main audit from screenshots directly. First convert the screenshot into a reproducible matrix case.

## Pass/Fail Rules

A case fails when any of these happen:

- The advertised key does nothing without explanation.
- The advertised key repeats the same unrecoverable error.
- A hidden key advances the flow.
- The screen changes but the model state does not match.
- The model state changes but the screen does not show it.
- Planning or applying starts when prerequisites are missing.
- A selected path, scope, credential assignment, or checkbox state is lost before preview.
- Any raw credential appears in output.

## How This Feeds The Audit

The audit should only list issues that have one of:

- a passing reproduction case marked as current behavior
- a failing automated case
- a screenshot converted into a matrix case

For each issue, link:

- matrix case ID
- invariant IDs
- code confirmation
- proposed test name

This keeps the audit short and makes it a source doc for fixes rather than a long investigation log.
