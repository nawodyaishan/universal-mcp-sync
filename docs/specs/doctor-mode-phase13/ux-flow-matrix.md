# Doctor Mode Phase 13 UX Flow Matrix

**Source protocol:** `docs/specs/doctor-mode-phase12/ux-bug-hunt-protocol.md`
**Companion gap analysis:** `docs/specs/doctor-mode-phase13/gap-analysis.md`
**Authoritative run:** `make ux-fake-prod` writes results into `artifacts/ux-fake-prod/matrix.json` and `issues.json`.

Phase 12 rows remain canonical for the scenarios they cover. Phase 13 extends the matrix with DM-P40..DM-P69 to close the (key × screen × precondition) grid.

**Origin:** Every DM-P40..DM-P69 row in this Phase 13 table is `manual`. Phase 14 explorer rows are tracked separately and should be imported from `artifacts/ux-explore/proposed-matrix-rows.md` once the full reporter lands.

---

## Status Lifecycle

| Status | Meaning |
|---|---|
| Proposed | row written, no automation yet |
| Captured | harness reproduces; behavior under triage |
| Locked | code + test + matrix row agree; CI guards regression |
| Blocked | depends on a fix landing first |

---

## Case Index — Phase 13 Additions

| ID | Family | Case | Preconditions | Keys | Expected | Invariants | Automation | Status |
|---|---|---|---|---|---|---|---|---|
| DM-P40 | F1 | Anchor — stuck on Provider Readiness with all providers conflict-blocked | unresolved conflict client present, credentials valid for at least one provider | `p`, `Enter` | Footer drops `[Enter] select provider` + `[v] live validate`; `Enter` becomes synonym for `r` and routes to Target Select; `mgr.ValidateCalls == 0` | I-01, I-02, I-13 | `TestDashboardFlowMatrix_StuckProviderReadyConflictHidesAll` | Locked |
| DM-P41 | F1 | Provider cursor cannot land on a hidden row | conflicts present, hidden conflict-blocked rows in `m.readiness` | `↑`/`↓` move via `RenderedProviderIndices` | `nextRenderedIndex`/`prevRenderedIndex` skip hidden indices; cursor never lands on a conflict-blocked row | I-01, I-06 | `TestRenderedProviderIndices_FiltersConflictBlocked` + `TestRenderedProviderIndices_CursorHelpersSkipHidden` | Locked |
| DM-P42 | F2 | Double-Enter on ProviderReady during validation | validation in flight | `p`, `Enter`, `Enter` | Second press is a no-op; UI shows `Validating...`; footer drops validation keys while in flight; no duplicate validation call | I-03 | `TestDashboardFlowMatrix_DoubleEnterDuringValidationIsSafe` | Locked |
| DM-P43 | F2 | Double-`y` on PlanPreview during apply | apply in flight | …, `y`, `y` | Second `y` is a no-op; `ApplyCalls == 1`; apply-in-flight view drops confirmation copy | I-03 | `TestDashboardFlowMatrix_DoubleYDuringApplyIsSafe` | Locked |
| DM-P44 | F2 | Double-`r` rescan on ApplyResult | scan in flight after first `r` | apply success, `r`, `r` | Second `r` is a no-op while rescanning; UI shows `Rescanning...`; footer drops `[r] rescan` until scan completes | I-03 | `TestDashboardFlowMatrix_DoubleRescanIsSafe` | Locked |
| DM-P45 | F2 | Action bar advertises an action with no effect | provider-ready with conflicts: footer says `[v] live validate` may still be present | inspect footer text on each screen state | Footer key list is computed from current state — keys with no effect are absent | I-01 | `TestActionBar_AdvertisedKeysAreActionable` (new, table-driven) | Proposed |
| DM-P46 | F3 | Esc Doctor preserves scan result | scan complete | `p`, `Esc` | Returning to Doctor shows the same report without rescanning; transient validation state is cleared | I-14 | `TestDashboardFlowMatrix_EscFromProviderReadyKeepsReport` | Locked |
| DM-P47 | F3 | Esc TargetSelect preserves selections | one target deselected, one selected | `p`, `Enter`, `Space`, `Esc`, `Enter` | Re-entering TargetSelect shows the same checkbox state | I-06, I-14 | `TestDashboardFlowMatrix_EscPreservesTargetSelections` | Locked |
| DM-P48 | F3 | Esc PlanPreview preserves selections + workspace toggle | mixed checked, includeWorkspace=on | `p`, `Enter`, `i`, `Enter`, `Esc` | Re-entering TargetSelect shows workspace rows visible + prior checkboxes | I-06, I-12, I-14 | `TestDashboardFlowMatrix_EscFromPlanPreviewKeepsSelections` | Locked |
| DM-P49 | F4 | Resolved conflict survives rescan | conflict resolved to candidate 1, apply success, harness rescans | `p`, `r`, conflict row, `r`, `1`, `Enter`, `y`, wait for rescan | After rescan the same conflict client appears as resolved-eligible (not conflict) and remains selected | I-09, I-14 | `TestDashboardFlowMatrix_ResolvedConflictSurvivesRescan` | Locked |
| DM-P50 | F4 | Selected targets cleared deliberately on rescan | apply success | `r` from ApplyResult | Selected-targets map is rebuilt from default (apply already happened) — UI does not show stale `[x]` against now-applied rows | I-06 | `TestDashboardFlowMatrix_RescanRebuildsTargetSelections` | Locked |
| DM-P51 | F5 | Unmapped key at every screen is no-op with no error toast | any screen | `x`, `z`, `5` per screen | Screen content + cursor + selected state unchanged; no error placeholder | I-01 | `TestDashboardModel_UnmappedKeysNoOpAcrossScreens` | Locked |
| DM-P52 | F5 | `q` quits from any screen | each screen | `q` | Program exits cleanly, no panic | I-14 | `TestDashboardModel_QuitKeysFromAnyScreen` | Locked |
| DM-P53 | F5 | `ctrl+c` quits from any screen | each screen | `ctrl+c` | Program exits cleanly | I-14 | `TestDashboardModel_QuitKeysFromAnyScreen` | Locked |
| DM-P54 | F6 | `?` opens help from each screen and preserves underlying state | each screen | `?`, `?` | First `?` shows help overlay; second `?` restores prior screen identically | I-01, I-14 | `TestDashboardModel_HelpOverlayTogglesWithoutLosingState` | Locked |
| DM-P55 | F6 | Help overlay text is screen-aware | provider ready vs target select | `?` after navigating | Overlay lists keys valid on the current screen, not a static superset | I-01 | `TestRenderDashboardHelpOverlay_ScreenAware` + help goldens | Locked |
| DM-P56 | F7 | Batch happy path — multiple targets one provider | credentials valid, 3 eligible clients all checked | `p`, `Enter`, `Enter`, `y` | All three targets appear in plan preview; `ApplyCalls == 1` with all three; ApplyResult lists each | I-06, I-10 | `TestDashboardFlowMatrix_BatchApplyThreeTargets` | Locked |
| DM-P57 | F7 | Batch skip-on-identical | first apply done, files unchanged on disk, second apply attempted | full batch flow twice | ApplyResult shows "Unchanged (N)" section per Phase 11 FR-2; `UpdatedTargets` empty on second pass | I-06 | `TestDashboardFakeProdMatrix_BatchSkipOnIdentical` | Locked |
| DM-P58 | F7 | Sequential session — apply Exa then Context7 | valid creds for both | `p`, choose Exa, `Enter`, `Enter`, `y`, `r`, `p`, choose Context7, `Enter`, `Enter`, `y` | Both applies succeed; second plan does not include Exa entries unintentionally | I-06 | `TestDashboardFlowMatrix_SequentialProvidersInOneSession` | Locked |
| DM-P59 | F7 | Plan preview renders 5+ targets at width 80 | 5 eligible targets | `p`, `Enter`, `Enter` | Plan preview content does not truncate critical info (path, scope); no overflow into action bar | I-01 | `TestGoldenPlanPreview_FiveTargets80Cols` | Locked |
| DM-P60 | F8 | Single-candidate conflict overlay | conflict client with exactly one accessible candidate | `p`, `r`, conflict row, `r` | Overlay renders one block; `2` is a no-op; `1`, `s`, `Esc` work | I-08 | `TestDashboardFlowMatrix_SingleCandidateConflict` (new) | Proposed |
| DM-P61 | F8 | Three-candidate conflict shows only first two | conflict client with 3+ candidates | `p`, `r`, conflict row, `r` | Overlay renders candidates 0 and 1; the 3rd is dropped per Phase 12 OQ | I-08 | `TestDashboardFlowMatrix_ThreeCandidateConflictShowsTwo` (new) | Proposed |
| DM-P62 | F8 | Unmapped digit on ConflictResolve | overlay visible | `p`, `r`, conflict row, `r`, `0`, `3` | No state change; overlay stays open | I-01 | `TestDashboardModel_ConflictResolveUnmappedDigitsNoOp` | Locked |
| DM-P63 | F8 | Sequential conflicts in one session | two conflict clients in report | `p`, `r`, conflict-1, `r`, `1`, navigate to conflict-2, `r`, `2`, `Enter` | Both resolutions stored; plan includes both chosen paths | I-08, I-10 | `TestDashboardFlowMatrix_SequentialConflicts` (new) | Proposed |
| DM-P64 | F4 | Apply error: result shows recovery key | `mgr.ApplyErr` set | …, `y` | ApplyResult shows error AND `[r] rescan` advertised as recovery | I-13 | `TestDashboardFlowMatrix_ApplyErrorOffersRecovery` (extension of existing flow test) | Proposed |
| DM-P65 | F3 | Esc on ApplyResult — what should happen? | apply complete | `Esc` | Documented decision: today this is a no-op; matrix locks "no-op" OR fix routes to Doctor | I-01, I-14 | `TestDashboardFlowMatrix_EscFromApplyResult` (new) | Proposed |
| DM-P66 | F2 | Wizard `w` from Doctor when scan errored | `m.err != nil` | `w` | Route to wizard still works (escape hatch) | I-13 | `TestDashboardFlowMatrix_WizardRouteOnScanError` (new) | Proposed |
| DM-P67 | F1 | `Enter` on ProviderReady when cursor on hidden conflict-blocked row | conflicts present, cursor=0 maps to hidden row | `p`, `Enter` | Either (a) Enter routes to conflict resolve, or (b) validation runs on a *visible* provider — never on a hidden one | I-01, I-06 | `TestDashboardFlowMatrix_EnterOnHiddenProviderRowSafe` (new) | Proposed |
| DM-P68 | F1 | `v` (live validate) when all providers conflict-blocked | conflicts present | `p`, `v` | Live validation does NOT call the network; UI explains why | I-04, I-13 | `TestDashboardFlowMatrix_LiveValidateBlockedDuringConflict` (new) | Proposed |
| DM-P69 | F2 | Footer never repeats an action that just failed | plan error visible | error repaint inspection | Footer changes to recovery copy after a failure (e.g., `[Esc] back  [?] help`), no `[Enter] plan` while a non-recoverable plan error is present | I-03, I-13 | `TestRenderTargetSelect_FooterRecoveryAfterPlanError` (new view test) | Proposed |

---

## Anchor Detail — DM-P40 Stuck On Provider Readiness

Preconditions:
- `report.Clients` contains at least one client with `Confidence == ConfidenceConflict`.
- `profiles` contains a valid credential for at least one provider (so the dead-end isn't a credential issue).
- `ComputeReadiness` therefore marks every provider `conflict-blocked`.

Reproduction keys:
1. Launch the dashboard.
2. `p` — enter Provider Readiness.

Observed today (per code reading + reproduction below):
- Banner: `! Conflicts detected — resolve before planning:`
- Provider rows: empty (all conflict-blocked rows are hidden).
- Footer: `[r] resolve conflicts  [↑↓] navigate  [Enter] select provider  [Esc] back  [q] quit`.
- Pressing `Enter`: `m.selectedProv = 0`, validation runs against `m.readiness[0]` which is a hidden conflict-blocked provider. Validation usually succeeds offline (no network call needed) and the screen advances to Target Select. The user never sees the validation occur — feels like Enter "did nothing" because the next screen looks like a sibling of the previous one.
- Pressing `v`: same behavior, against hidden row, may make a network call.
- Pressing `r`: works — routes to TargetSelect with the conflict highlighted. **This is the only correct forward key today.**

Expected behavior after fix:
- Either visible provider rows remain (preferred — separate provider readiness from target conflict, see UX-04), OR
- `Enter`/`v` reroute to conflict resolution when no rendered provider is selectable, matching `r`.
- Footer drops `[Enter] select provider` and `[v] live validate` when no provider is selectable.

Locked behavior to assert:
- `mgr.ValidateCalls == 0` when no rendered row is selectable.
- After `Enter`, either screen is `screenTargetSelect` (rerouted) or the screen remains `screenProviderReady` with a visible explanation.

Guarded invariants: I-01, I-02, I-13.

---

## Recording Results

For Phase 13 rows, automation must populate:
- `matrix.json.cases[].id == DM-P<n>`
- `issues.json.issues[]` entries when a row is `Captured` but not yet `Locked`
- `flows/DM-P<n>.txt` (and `.ansi`) when the row uses the real-TUI PTY runner

Locked rows must additionally:
- Live in `pkg/tui/dashboard_flow_matrix_test.go` or `pkg/tui/dashboard_fake_prod_matrix_test.go`.
- Reference the matrix row ID in the test name or comment.
- Pass with `NO_COLOR=1 TERM=xterm-256color go test ./pkg/tui` and inside `make ux-fake-prod`.
