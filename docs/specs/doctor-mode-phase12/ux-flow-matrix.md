# Doctor Mode Phase 12 UX Flow Matrix

**Source protocol:** `docs/specs/doctor-mode-phase12/ux-bug-hunt-protocol.md`  
**Purpose:** compact source of truth for Docker-staged product-flow cases, expected behavior, and automated coverage.  
**Status:** active matrix; expand from screenshots and `artifacts/ux-fake-prod/matrix.json` failures.

The authoritative collection run is:

```text
make ux-fake-prod
```

Local `USYNC_UX_MATRIX=1` runs are allowed only as a fast developer shortcut. Issues should enter the audit from Docker-collected artifacts.

## Case Index

| ID | Case | Preconditions | Keys | Expected | Automation | Status |
|---|---|---|---|---|---|---|
| DM-P31 | Missing credentials at target planning | credentials none, Exa selected, targets detected | `p`, `r`, `Enter` | Plan command is not called; user sees credential recovery action | `TestDashboardFakeProdMatrix_MissingCredentialsBlocksBeforePlan` | Locked |
| DM-P12 | Deselect only target before planning | credentials valid, one target, no conflicts | `p`, `Enter`, `Space`, `Enter` | Plan command is not called; user sees select-target recovery action | `TestDashboardFlowMatrix_DeselectOnlyTargetBlocksPlan` | Locked |
| DM-P10 | Hidden `r` without conflicts | no conflicts, provider readiness visible | `p`, `r` | Stays on provider readiness because `r` is not advertised | `TestDashboardFlowMatrix_RWithoutConflictDoesNotAdvance` | Locked |
| DM-P05 | No-key provider can plan without credentials | credentials none, Playwright selected, no conflicts | `p`, `Enter`, `Enter` | Planning succeeds without credential profiles | `TestDashboardFlowMatrix_NoKeyProviderCanPlanWithoutCredentials` | Locked |
| DM-P19A | Conflict candidate 1 reaches plan | credentials valid, unresolved conflict | `p`, `r`, conflict row, `r`, `1`, `Enter` | Candidate 1 path reaches planning target | `TestDashboardFlowMatrix_ConflictChosenPathReachesPlan` | Locked |
| DM-P19B | Conflict candidate 2 reaches plan | credentials valid, unresolved conflict | `p`, `r`, conflict row, `r`, `2`, `Enter` | Candidate 2 path reaches planning target | `TestDashboardFlowMatrix_ConflictSecondPathReachesPlan` | Locked |
| DM-P20 | Skipped conflict is excluded | credentials valid, unresolved conflict | `p`, `r`, conflict row, `r`, `s`, `Enter` | Skipped client is not planned | `TestDashboardFlowMatrix_SkippedConflictIsExcluded` | Locked |
| DM-P14 | Workspace toggle changes targets | workspace target exists | `p`, `Enter`, `i` | Target list changes and plan scope includes workspace | `TestDashboardFlowMatrix_WorkspaceToggleChangesTargets` | Blocked |
| DM-P32 | Workspace off excludes project targets | user and workspace candidates exist | `p`, `Enter`, `Enter` | Plan includes only user/global targets | Planned | Blocked |
| DM-P33 | Workspace target shows scope warning | workspace target selected | `p`, `Enter`, `i`, `Enter` | Preview identifies project/workspace scope and git risk | Planned | Blocked |
| DM-P34 | Multi-file client can select one candidate | app has multiple user candidates | `p`, `Enter`, target row, `Space`, `Enter` | Plan includes only checked candidate paths | Planned | Blocked |
| DM-P35 | Conflict resolution preserves row identity | conflict resolved to candidate 2 | `p`, `r`, conflict row, `r`, `2` | Target row shows chosen label/path, not only client name | Planned | Blocked |
| DM-P15 | No raw credential full journey | valid credential, happy path | `p`, `Enter`, `Enter`, `y` | Raw credential absent from all captured output | `TestDashboardFlow_NoRawCredential` | Locked |

## DM-P31 Missing Credentials At Target Planning

Preconditions:

- `credentials: none`
- `provider: Exa`
- `targets: one or many`
- `conflicts: optional`

Visible promise before final action:

- The screen says a credential profile is required.
- Footer does not advertise `[Enter] plan`.

Expected:

- The dashboard blocks before `PrepareProvider`.
- The screen shows a credential recovery action such as setup/import or provider readiness.
- Repeating `Enter` does not repeat the same error.

Locked behavior:

- `PrepareProvider` is not called with zero profiles for credential-required providers.
- The user can return to Provider Readiness with `Esc`.

Guarded invariants:

- `I-03`
- `I-04`
- `I-13`

## DM-P12 Deselect Only Target Before Planning

Preconditions:

- `credentials: valid`
- `targets: one`
- `conflicts: none`

Visible promise before final action:

- Target row shows `[ ]`.
- Footer does not advertise `[Enter] plan`.

Expected:

- The dashboard does not call `PrepareProvider`.
- The screen says to select at least one target.

Locked behavior:

- Unchecked targets are removed from planning state.
- Empty target selection stays on target select with recovery copy.

Guarded invariants:

- `I-06`
- `I-07`

## DM-P10 Hidden `r` Without Conflicts

Preconditions:

- `screen: provider readiness`
- `conflicts: none`

Visible promise before action:

- Footer does not advertise `r`.

Expected:

- Pressing `r` is a no-op.
- User remains on provider readiness.

Locked behavior:

- `r` is ignored unless conflicts exist.

Guarded invariants:

- `I-02`

## Locked Additional Cases

DM-P05: no-key providers such as Playwright can plan with zero credential profiles; credential-required providers still cannot.

DM-P19A/DM-P19B: resolving a conflict with candidate `1` or `2` passes the chosen candidate path into planning, not only the client ID.

DM-P20: skipping a conflict excludes that client from planning and does not pass a target path override.

DM-P15: the full captured TUI flow must not expose raw credential values.

## Candidate-Level Target Cases

DM-P14/DM-P32/DM-P33/DM-P34/DM-P35 share one required fix: target selection must use concrete candidate rows, not only client IDs.

Required target row fields:

- client ID and display name
- candidate label
- file path
- scope: user, global, project, workspace
- file kind and creatable state
- git-warning flag
- conflict resolution state, when applicable

Blocked behavior until PR 12c:

- workspace toggle is cosmetic
- project/workspace files cannot be selected deliberately
- multi-file clients plan every file for a checked client
- resolved conflicts do not show the chosen candidate as row identity

## Recording Results

For automated runs, record:

- case ID
- preconditions
- keys
- final screen
- visible text checkpoint
- manager call counts
- selected app IDs
- failure invariants

Docker fake-prod runs must export the same case IDs in `artifacts/ux-fake-prod/matrix.json` and collected issues in `artifacts/ux-fake-prod/issues.json`.
