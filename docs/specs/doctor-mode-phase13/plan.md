# Doctor Mode Phase 13 — Implementation Plan

**Spec:** `docs/specs/doctor-mode-phase13/spec.md`
**Gap analysis:** `docs/specs/doctor-mode-phase13/gap-analysis.md`
**Matrix:** `docs/specs/doctor-mode-phase13/ux-flow-matrix.md`
**Status:** Draft

---

## PR Sequence

| PR | Focus | Matrix rows |
|---|---|---|
| 13a | Anchor fix — stuck on ProviderReady (FR-1, FR-2, FR-3) | DM-P40, DM-P41, DM-P45, DM-P67, DM-P68 |
| 13b | Re-entrancy + Esc preservation (FR-4, FR-5) | DM-P42, DM-P43, DM-P44, DM-P46, DM-P47, DM-P48 |
| 13c | Rescan persistence + help overlay (FR-6, FR-7) | DM-P49, DM-P50, DM-P54, DM-P55 |
| 13d | Unmapped keys + quit-from-anywhere + unmapped digits (FR-9, FR-10) | DM-P51, DM-P52, DM-P53, DM-P62 |
| 13e | Batch apply E2E (FR-8) | DM-P56, DM-P57, DM-P58, DM-P59 |
| 13f | ConflictResolve edges + apply error recovery | DM-P60, DM-P61, DM-P63, DM-P64, DM-P65, DM-P66, DM-P69 |

PR 13a is the user-impacting fix and must land first. The remaining PRs are additive and can land in any order once 13a is in.

---

## CodeGraph-Verified Impact

`mcp__codegraph__codegraph_impact` on `ComputeReadiness` (depth 2) returns 4 callers:
- `pkg/tui/dashboard_readiness.go:33` — the definition itself
- `pkg/tui/dashboard.go:203` — `readinessCmd` (the dashboard call site)
- `pkg/tui/dashboard_test.go:343` — `TestComputeReadiness_AllFiveStates`
- `pkg/tui/dashboard_golden_test.go:31` — `TestGoldenScreenProviderReady`

`renderProviderReady` calls only `actionBarProviderReady` and `conflictClientsInReport`. The blast radius for PR 13a is contained to `pkg/tui/dashboard_readiness.go`, `dashboard.go` (the cursor handlers), and `dashboard_view.go` (the renderer and action bar).

---

## PR 13a — Anchor Fix Detail

### Files

| File | Change |
|---|---|
| `pkg/tui/dashboard_readiness.go` | Add `RenderedProviderIndices(items []ProviderReadinessItem, hasConflicts bool) []int`. Pure function. |
| `pkg/tui/dashboard.go` | `handleKeyProviderReady`: replace bare cursor increments with bounded-by-rendered movement; on `enter` and `v`, if `len(rendered)==0`, treat `enter` as `r` and `v` as no-op with a visible status. |
| `pkg/tui/dashboard_view.go` | `renderProviderReady`: when `len(rendered)==0`, append the explanatory line "No providers can be selected until the conflicts above are resolved." `actionBarProviderReady`: take a new `hasSelectable bool` parameter; drop `[Enter] select provider` and `[v] live validate` when false. |
| `pkg/tui/dashboard_flow_matrix_test.go` | Add `TestDashboardFlowMatrix_StuckProviderReadyConflictHidesAll` (DM-P40), `TestDashboardFlowMatrix_ProviderCursorSkipsHiddenRows` (DM-P41), `TestDashboardFlowMatrix_EnterOnHiddenProviderRowSafe` (DM-P67), `TestDashboardFlowMatrix_LiveValidateBlockedDuringConflict` (DM-P68). |
| `pkg/tui/dashboard_view_test.go` (new) | `TestActionBar_AdvertisedKeysAreActionable` (DM-P45) — table-driven across screen states. |
| `pkg/tui/dashboard_golden_test.go` | Re-record `TestGoldenScreenProviderReady` with the new dead-end branch. |
| `docs/specs/doctor-mode-phase13/ux-flow-matrix.md` | Flip DM-P40, DM-P41, DM-P45, DM-P67, DM-P68 status to `Locked` once tests pass. |

### Pseudocode

```go
// dashboard_readiness.go
func RenderedProviderIndices(items []ProviderReadinessItem, hasConflicts bool) []int {
    out := make([]int, 0, len(items))
    for i, it := range items {
        if hasConflicts && it.State == ProviderStateConflictBlocked {
            continue
        }
        out = append(out, i)
    }
    return out
}

// dashboard.go handleKeyProviderReady refactor
rendered := RenderedProviderIndices(m.readiness, hasConflictClient(m.report))
hasSelectable := len(rendered) > 0

switch key {
case "esc":
    m.screen = screenDoctor
case "r":
    if hasConflictClient(m.report) {
        m.screen = screenTargetSelect
        m.planErr = nil
    }
case "up", "k":
    if hasSelectable { m.providerCursor = prevRenderedIndex(rendered, m.providerCursor) }
case "down", "j":
    if hasSelectable { m.providerCursor = nextRenderedIndex(rendered, m.providerCursor) }
case "v":
    if !hasSelectable { return m, nil } // no-op, footer should not advertise [v]
    if !m.validating {
        m.selectedProv = m.providerCursor
        m.validating = true
        return m, m.liveValidationCmd()
    }
case "enter":
    if !hasSelectable {
        // Synonym for r — anchor fix.
        m.screen = screenTargetSelect
        m.planErr = nil
        return m, nil
    }
    if !m.validating {
        m.selectedProv = m.providerCursor
        m.validating = true
        return m, m.offlineValidationCmd()
    }
}
```

### Test recipe

```go
// DM-P40
func TestDashboardFlowMatrix_StuckProviderReadyConflictHidesAll(t *testing.T) {
    requireUXMatrix(t)
    scanner, mgr, profiles := happyFlowSetup(t)
    scanner.Report.Clients = append(scanner.Report.Clients, doctor.ClientFinding{
        ID: "antigravity", Name: "Antigravity IDE", Confidence: doctor.ConfidenceConflict,
        Candidates: []doctor.CandidateFinding{
            {Label: "a", Path: "/h/.gemini/config/mcp_config.json", Exists: true, ParseOK: true},
            {Label: "b", Path: "/h/.gemini/antigravity/mcp_config.json", Exists: true, ParseOK: true},
        },
    })
    m := NewDashboardModel(scanner, mgr, profiles)
    tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

    waitForText(t, tm, "System Status")
    tm.Send(keyMsg("p"))
    waitForText(t, tm, "Provider Readiness")

    // Anchor assertion: footer must not advertise [Enter] select provider.
    waitForText(t, tm, "[r] resolve conflicts")

    tm.Send(keyMsg("enter")) // synonym for r
    waitForText(t, tm, "Select Targets")

    tm.Send(keyMsg("q"))
    tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

    if mgr.ValidateCalls != 0 {
        t.Fatalf("DM-P40: validation must not run when no provider is selectable, got %d", mgr.ValidateCalls)
    }
}
```

---

## Verification Per PR

Each PR must, in one commit:
1. Update the matrix row to `Locked`.
2. Pass `NO_COLOR=1 TERM=xterm-256color go test ./pkg/tui`.
3. Pass `make ux-fake-prod` with `issues.json` strictly decreasing or empty.
4. Update goldens where the renderer changed.

---

## Risks

| Risk | Mitigation |
|---|---|
| Changing the action bar breaks Phase 12 golden tests | Re-record goldens in the same commit as the action-bar change |
| `Enter` becoming a synonym for `r` surprises users who expect strict no-op | Footer drops `[Enter] select provider` when no selectable provider — Enter is documented as "next forward action" |
| Hidden-index cursor refactor introduces an off-by-one when transitioning between has-conflicts/no-conflicts | Unit test cursor-clamp in `TestDashboard_ProviderCursorClampOnReadinessChange` |
| Help overlay screen-aware signature touches many call sites | Centralize via `renderHelpOverlay(m.screen)` — single call site in `View()` |

---

## Out-Of-Band Notes

- The fix in PR 13a does **not** address UX-04 (readiness using unresolved conflicts). That fix changes `ComputeReadiness` semantics and remains in the Phase 12 backlog. PR 13a is the smallest change that closes the user-reported dead-end without re-architecting readiness.
- Phase 13 deliberately keeps the matrix list flat. If a row's behavior depends on a fix that hasn't landed, the row stays `Blocked` and is locked only after the dependency PR ships.
