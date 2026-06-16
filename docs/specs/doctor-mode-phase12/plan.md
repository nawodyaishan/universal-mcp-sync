# Doctor Mode Phase 12 — Technical Implementation Plan

**Status:** Draft  
**Spec:** `docs/specs/doctor-mode-phase12/spec.md`  
**Last updated:** 2026-05-23  
**PR groupings:** 12a (conflict resolution model + view), 12b (E2E flow tests), 12c (candidate-level target rows)

---

## Summary

Phase 12 adds two orthogonal improvements to `pkg/tui`:

1. **Conflict resolution** — `screenConflictResolve` (screen 5), conflict-cursor navigation in `screenTargetSelect`, `ConflictResolution` type, and updated `eligibleClientIDs`/`buildAppSelection`.

2. **Playwright-style E2E teatest** — `dashboard_flow_test.go` with `TestDashboardFlow_*` covering the full 5→6-screen flow (including conflict resolution) via actual key inputs, `waitForText` assertions, and `tm.FinalModel(t)` state verification.

3. **Candidate-level target rows** — replace client-level target selection with concrete candidate rows carrying path, scope, file kind, git-warning, and conflict-resolution metadata.

---

## Inputs Reviewed

- `docs/specs/doctor-mode-phase12/spec.md` (all FRs, ACs)
- `pkg/tui/dashboard.go` — `DashboardModel`, `screenTargetSelect` handler, `eligibleClientIDs`, `buildAppSelection`
- `pkg/tui/dashboard_view.go` — `renderTargetSelect`, `skippedClients`, `skipReason`
- `pkg/tui/dashboard_test.go` — `FakeDashboardManager`, `FakeScanner`, existing unit tests
- `pkg/tui/test_helpers_test.go` — `waitForText`, `waitForAll`
- `pkg/doctor/types.go` — `ClientFinding`, `CandidateFinding` (full field list available)
- `docs/research/TUI-Testing-with-Go.md` — Playwright pattern: `NewTestModel` → `WaitFor` → `Send` → `FinalModel`

---

## Assumptions

1. PR 12a/12b are TUI-only. PR 12c may change the planning boundary so selected concrete target rows reach `pkg/app`.
2. `eligibleClientIDs` and `buildAppSelection` are the only two functions that filter clients — updating both is sufficient to make resolved conflicts plannable.
3. The conflict overlay shows the first two candidates with `Exists == true || IsSymlink == true` from `ClientFinding.Candidates`. If neither has two such candidates, it shows what's available (minimum 1).
4. Resolved conflict clients are added to `selectedClients` automatically when resolved — pre-selected, same as `defaultSelectedClients` does for regular clients.
5. `resolvedConflicts` persists across rescans because it is model state; after a rescan `defaultSelectedClients` is called again but `resolvedConflicts` is merged in by a new `applyResolutions` helper.
6. The cursor model for `screenTargetSelect` extends: `conflictCursor int` tracks position within the conflict section; `clientInConflictSection bool` tracks which section the cursor is in.
7. Flow tests use `FakeDashboardManager` with `HomeDir()` returning `t.TempDir()`, allowing `PlanStore.Save` to succeed with a real temp file.
8. The `FakeDashboardManager.Plan` must have non-zero `PlanID`, `ProviderID`, `CreatedAt`, `ExpiresAt` for `PlanStore.Save` to serialize correctly.
9. Workspace/project support is not safe to implement as another boolean flag. It requires target-row identity: path, scope, file kind, and git warning.

---

## Architecture Approach

### PR 12a — Conflict resolution

**State machine extension:**
```
screenTargetSelect (existing)
  → [r / Enter on conflict client] → screenConflictResolve (new, screen 5)

screenConflictResolve
  → [1] resolve with candidate 0 → screenTargetSelect (client now eligible)
  → [2] resolve with candidate 1 → screenTargetSelect (client now eligible)
  → [s] skip → screenTargetSelect (client not eligible, not conflict)
  → [Esc] cancel → screenTargetSelect (no change)
```

**Cursor model in `screenTargetSelect`:**
The existing `clientCursor` indexes into `eligibleClientIDs(report)`. In Phase 12, a second cursor `conflictCursor` indexes into `skippedClients(report)` filtered to conflict-only clients. A bool `inConflictSection` tracks which section has focus. `↑`/`↓` navigate within-section; when cursor reaches the boundary between sections, focus transfers.

**Simplified approach** (avoids complex dual-cursor): merge eligible and conflict clients into a single ordered slice `allTargets []targetEntry` where each entry knows whether it is eligible or conflict. `clientCursor` indexes this unified slice. This removes the need for `inConflictSection`.

```go
type targetEntry struct {
    clientID manifest.ClientID
    name     string
    isConflict bool
}
```

`handleKeyTargetSelect` checks `allTargets[m.clientCursor].isConflict` to decide whether `r`/`Enter` opens the overlay or toggles selection.

### PR 12b — E2E flow tests

`dashboard_flow_test.go` uses the existing `FakeDashboardManager`, `FakeScanner`, `waitForText`, and `waitForAll` helpers. Tests call `teatest.NewTestModel` with `WithInitialTermSize(80, 24)`, send key messages via `tm.Send(tea.KeyMsg{...})`, assert visible text, then call `tm.FinalModel(t)` after `tm.WaitFinished`.

The fake manager's `HomeDir()` returns `t.TempDir()` so `planCmd()` can write a real plan file to disk.

### PR 12c — Candidate-level target selection

Problem: `screenTargetSelect` currently stores `selectedClients map[manifest.ClientID]bool`. That loses candidate path, scope, and file-kind identity. It makes workspace toggle cosmetic and makes multi-file clients plan more than the row the user thought they selected.

Approach:

1. Introduce a `targetRow` model for concrete targets.
2. Build rows from `doctor.ClientFinding.Candidates`.
3. Hide `ScopeProject` and `ScopeWorkspace` rows unless `includeWorkspace` is true.
4. Track selection by stable row ID, not only client ID.
5. Convert selected rows into a narrowed planning input that can produce one operation per selected path.

Sketch:

```go
type targetRow struct {
    id         string
    clientID   manifest.ClientID
    name       string
    label      string
    path       string
    scope      manifest.ScopeKind
    kind       config.FileKind
    creatable  bool
    gitWarning bool
    conflict   bool
}
```

Planning should accept concrete target files instead of broad app IDs. A transitional implementation may convert selected rows into narrowed `config.AppConfig` values before calling the app manager, but the TUI must not advertise per-file control unless the plan is also per-file.

---

## Affected Modules

| Module | Change | PR |
|---|---|---|
| `pkg/tui/dashboard.go` | `ConflictResolution` type; `resolveTarget`, `resolvedConflicts`, `conflictCursor` fields; `screenConflictResolve` const; `targetEntry` type; refactor `handleKeyTargetSelect`; `resolveConflict`, `applyResolutions` helpers | 12a |
| `pkg/tui/dashboard_view.go` | `renderTargetSelect` refactored to unified list; `renderConflictResolve` new method | 12a |
| `pkg/tui/dashboard_test.go` | New unit tests for conflict resolution state transitions | 12a |
| `pkg/tui/dashboard_golden_test.go` | `TestGoldenScreenConflictResolve` | 12a |
| `pkg/tui/dashboard_flow_test.go` | New file: 7 flow tests | 12b |
| `pkg/tui/dashboard.go` | Target row model; workspace filtering; selected target-row IDs; concrete planning conversion | 12c |
| `pkg/tui/dashboard_view.go` | Target rows show label, scope, and path hints; workspace risk copy | 12c |
| `pkg/app/app.go` | Planning API accepts narrowed concrete target files or an equivalent target override map | 12c |
| `pkg/tui/dashboard_flow_matrix_test.go` | DM-P14, DM-P32, DM-P33, DM-P34, DM-P35 | 12c |

---

## API and Contract Changes

### New constants (pkg/tui/dashboard.go)

```go
screenConflictResolve dashboardScreen = 5
```

### New types (pkg/tui/dashboard.go)

```go
type ConflictResolution struct {
    ChosenPath  string // candidate path; empty if Skipped
    ChosenLabel string // candidate label; empty if Skipped
    Skipped     bool
}

type targetEntry struct {
    clientID   manifest.ClientID
    name       string
    isConflict bool
}
```

### New `DashboardModel` fields

```go
conflictCursor    int
resolveTarget     *doctor.ClientFinding
resolvedConflicts map[manifest.ClientID]ConflictResolution
```

### Modified functions

```go
// New: returns unified list for screenTargetSelect cursor
func allTargetEntries(report doctor.Report, resolved map[manifest.ClientID]ConflictResolution) []targetEntry

// Updated: includes resolved (non-skipped) conflicts
func eligibleClientIDs(report doctor.Report, resolved map[manifest.ClientID]ConflictResolution) []manifest.ClientID

// Updated: includes resolved clients in selection map
func (m DashboardModel) buildAppSelection() map[config.AppID]bool

// New: re-applies resolved conflicts after rescan
func (m DashboardModel) applyResolutions() DashboardModel

// New: returns conflict-only findings excluding resolved ones
func conflictClientsForDisplay(report doctor.Report, resolved map[manifest.ClientID]ConflictResolution) []doctor.ClientFinding
```

---

## Data Model Changes

All changes are in-memory model state only. No serialisation, no file system writes.

---

## Dependency Changes

None. All changes are within `pkg/tui` and its existing test dependencies (`teatest`, `golden`).

---

## Security Impact

Conflict resolution never writes to disk. The chosen candidate path is stored in model memory and consumed by `buildAppSelection`. Raw credentials are never involved.

---

## Failure Modes

| Failure | Handling |
|---|---|
| Conflict client has 0 existing candidates | Overlay shows "No accessible candidates found" with only `[s] skip` and `[Esc] cancel` |
| `resolveTarget` is nil when `screenConflictResolve` renders | Defensive guard: render error message, `Esc` returns to target select |
| Conflict resolution races with rescan | `applyResolutions()` re-applies stored resolutions after each `scanResultMsg` |
| `FakeDashboardManager.PlanErr` set | `planCreatedMsg` carries error; flow test asserts error visible |

---

## Rollback and Recovery

All changes are additive to `pkg/tui` only. If conflict resolution regresses:
- `resolvedConflicts` is a new field (nil-safe map).
- `screenConflictResolve` is only reachable via new key presses on conflict clients.
- All existing Phase 7/8/11 tests continue to pass without modification.

---

## Numbered Implementation Tasks

---

### Task 1 — New types, constants, and model fields (12a)

**File:** `pkg/tui/dashboard.go`  
**Depends on:** nothing  
**Risk:** Low

Add to `dashboard.go`:

```go
screenConflictResolve dashboardScreen = 5

type ConflictResolution struct {
    ChosenPath  string
    ChosenLabel string
    Skipped     bool
}

type targetEntry struct {
    clientID   manifest.ClientID
    name       string
    isConflict bool
}
```

Add to `DashboardModel` (after `includeWorkspace`):

```go
conflictCursor    int
resolveTarget     *doctor.ClientFinding
resolvedConflicts map[manifest.ClientID]ConflictResolution
```

**Acceptance:** `go build ./pkg/tui` passes.

---

### Task 2 — `allTargetEntries`, updated `eligibleClientIDs`, `buildAppSelection`, `applyResolutions` (12a)

**File:** `pkg/tui/dashboard.go`  
**Depends on:** Task 1  
**Risk:** Low

**`allTargetEntries(report doctor.Report, resolved map[manifest.ClientID]ConflictResolution) []targetEntry`**  
Returns eligible clients first (type `isConflict: false`), then unresolved-conflict clients (type `isConflict: true`), then skipped-conflict clients are omitted entirely.

```go
func allTargetEntries(report doctor.Report, resolved map[manifest.ClientID]ConflictResolution) []targetEntry {
    var entries []targetEntry
    // Eligible (high/medium, installed/path present, not conflict, not resolved-skipped)
    for _, c := range report.Clients {
        if c.Confidence == doctor.ConfidenceConflict || c.Confidence == doctor.ConfidenceLow {
            continue
        }
        if !c.Installed && c.EffectivePath == "" {
            continue
        }
        entries = append(entries, targetEntry{clientID: manifest.ClientID(c.ID), name: c.Name, isConflict: false})
    }
    // Resolved conflicts (non-skipped) as eligible
    for _, c := range report.Clients {
        if c.Confidence != doctor.ConfidenceConflict {
            continue
        }
        if r, ok := resolved[manifest.ClientID(c.ID)]; ok && !r.Skipped {
            entries = append(entries, targetEntry{clientID: manifest.ClientID(c.ID), name: c.Name, isConflict: false})
        }
    }
    // Unresolved conflicts
    for _, c := range report.Clients {
        if c.Confidence != doctor.ConfidenceConflict {
            continue
        }
        if _, ok := resolved[manifest.ClientID(c.ID)]; !ok {
            entries = append(entries, targetEntry{clientID: manifest.ClientID(c.ID), name: c.Name, isConflict: true})
        }
    }
    return entries
}
```

**Update `eligibleClientIDs`**: change signature to accept `resolved map[manifest.ClientID]ConflictResolution`; include resolved non-skipped conflicts. Update all callers to pass `m.resolvedConflicts`.

**Update `buildAppSelection`**: use `allTargetEntries` to determine which clients are in `selectedClients`.

**`applyResolutions() DashboardModel`**:  
After a rescan (`scanResultMsg`), re-populate `selectedClients` with resolved non-skipped conflicts:

```go
func (m DashboardModel) applyResolutions() DashboardModel {
    for id, r := range m.resolvedConflicts {
        if !r.Skipped {
            if m.selectedClients == nil {
                m.selectedClients = make(map[manifest.ClientID]bool)
            }
            m.selectedClients[id] = true
        }
    }
    return m
}
```

Called at end of `providerReadinessMsg` handler.

**Acceptance:** `go test ./pkg/tui -run TestDashboard` passes.

---

### Task 3 — `screenTargetSelect` key handler extended (12a)

**File:** `pkg/tui/dashboard.go`  
**Depends on:** Task 2  
**Risk:** Low–Medium

Replace `handleKeyTargetSelect` to use `allTargetEntries`:

```go
func (m DashboardModel) handleKeyTargetSelect(key string) (tea.Model, tea.Cmd) {
    entries := allTargetEntries(m.report, m.resolvedConflicts)
    switch key {
    case "esc":
        m.screen = screenProviderReady
    case "up", "k":
        if m.clientCursor > 0 { m.clientCursor-- }
    case "down", "j":
        if m.clientCursor < len(entries)-1 { m.clientCursor++ }
    case " ":
        if m.clientCursor < len(entries) && !entries[m.clientCursor].isConflict {
            id := entries[m.clientCursor].clientID
            if m.selectedClients == nil { m.selectedClients = make(map[manifest.ClientID]bool) }
            m.selectedClients[id] = !m.selectedClients[id]
        }
    case "r", "enter":
        if m.clientCursor < len(entries) {
            entry := entries[m.clientCursor]
            if entry.isConflict {
                // Open conflict resolve overlay
                for _, c := range m.report.Clients {
                    if manifest.ClientID(c.ID) == entry.clientID {
                        m.resolveTarget = &c
                        break
                    }
                }
                m.screen = screenConflictResolve
            } else if key == "enter" && len(m.selectedClients) > 0 {
                m.planning = true
                m.planErr = nil
                return m, m.planCmd()
            }
        }
    case "i":
        m.includeWorkspace = !m.includeWorkspace
    }
    return m, nil
}
```

Add `screenConflictResolve` handler:

```go
func (m DashboardModel) handleKeyConflictResolve(key string) (tea.Model, tea.Cmd) {
    if m.resolveTarget == nil {
        m.screen = screenTargetSelect
        return m, nil
    }
    candidates := conflictCandidatesForDisplay(*m.resolveTarget)
    switch key {
    case "esc":
        m.screen = screenTargetSelect
        m.resolveTarget = nil
    case "s":
        m.resolvedConflicts = setResolution(m.resolvedConflicts,
            manifest.ClientID(m.resolveTarget.ID), ConflictResolution{Skipped: true})
        m.screen = screenTargetSelect
        m.resolveTarget = nil
    case "1":
        if len(candidates) >= 1 {
            m.resolvedConflicts = setResolution(m.resolvedConflicts,
                manifest.ClientID(m.resolveTarget.ID),
                ConflictResolution{ChosenPath: candidates[0].Path, ChosenLabel: candidates[0].Label})
            if m.selectedClients == nil { m.selectedClients = make(map[manifest.ClientID]bool) }
            m.selectedClients[manifest.ClientID(m.resolveTarget.ID)] = true
            m.screen = screenTargetSelect
            m.resolveTarget = nil
        }
    case "2":
        if len(candidates) >= 2 {
            m.resolvedConflicts = setResolution(m.resolvedConflicts,
                manifest.ClientID(m.resolveTarget.ID),
                ConflictResolution{ChosenPath: candidates[1].Path, ChosenLabel: candidates[1].Label})
            if m.selectedClients == nil { m.selectedClients = make(map[manifest.ClientID]bool) }
            m.selectedClients[manifest.ClientID(m.resolveTarget.ID)] = true
            m.screen = screenTargetSelect
            m.resolveTarget = nil
        }
    }
    return m, nil
}
```

Helper:

```go
// conflictCandidatesForDisplay returns the first 2 candidates that exist or are symlinks.
func conflictCandidatesForDisplay(c doctor.ClientFinding) []doctor.CandidateFinding {
    var out []doctor.CandidateFinding
    for _, cand := range c.Candidates {
        if cand.Exists || cand.IsSymlink {
            out = append(out, cand)
            if len(out) == 2 { break }
        }
    }
    return out
}

func setResolution(m map[manifest.ClientID]ConflictResolution, id manifest.ClientID, r ConflictResolution) map[manifest.ClientID]ConflictResolution {
    if m == nil { m = make(map[manifest.ClientID]ConflictResolution) }
    m[id] = r
    return m
}
```

Wire `screenConflictResolve` into `handleKey`:

```go
case screenConflictResolve:
    return m.handleKeyConflictResolve(key)
```

**Acceptance:** `go test ./pkg/tui -run TestDashboard` passes.

---

### Task 4 — `renderTargetSelect` and `renderConflictResolve` (12a)

**File:** `pkg/tui/dashboard_view.go`  
**Depends on:** Tasks 2, 3  
**Risk:** Low

**Rewrite `renderTargetSelect`** to use `allTargetEntries`:

```
> [x] Antigravity CLI
  [x] Codex CLI
  [ ] VS Code

Conflict clients (press r to resolve):
? Antigravity IDE — conflict — resolve before planning
```

Cursor (`>`) and checkbox (`[x]`/`[ ]`) shown only for non-conflict entries. Conflict entries show `?` and their name. When cursor lands on a conflict entry, the action bar shows `[r] resolve` instead of `[Space] toggle`.

**New `renderConflictResolve`**:

```go
func (m DashboardModel) renderConflictResolve() string {
    var b strings.Builder
    if m.resolveTarget == nil {
        return "No conflict target selected.\n"
    }
    c := *m.resolveTarget
    fmt.Fprintf(&b, "Resolve Conflict: %s\n", c.Name)
    b.WriteString(strings.Repeat("=", 44) + "\n\n")

    candidates := conflictCandidatesForDisplay(c)
    for i, cand := range candidates {
        fmt.Fprintf(&b, "[%d] %s", i+1, cand.Label)
        if cand.Deprecated { b.WriteString("  (deprecated)") }
        b.WriteString("\n")
        fmt.Fprintf(&b, "    Path:      %s\n", cand.Path)
        if cand.IsSymlink && cand.Resolved != "" {
            fmt.Fprintf(&b, "    Symlink:   → %s\n", cand.Resolved)
        }
        if cand.ParseOK { b.WriteString("    Parse:     ok\n")
        } else if cand.ParseError != "" {
            fmt.Fprintf(&b, "    Parse:     error: %s\n", redact.Key(cand.ParseError))
        }
        if len(cand.Providers) > 0 {
            fmt.Fprintf(&b, "    Providers: %s\n", strings.Join(cand.Providers, ", "))
        } else {
            b.WriteString("    Providers: (none)\n")
        }
        b.WriteString("\n")
    }
    if len(candidates) == 0 {
        b.WriteString("No accessible candidates found.\n\n")
    }
    b.WriteString("[s] skip client  [Esc] cancel")
    if len(candidates) >= 1 { b.WriteString("  [1] use this") }
    if len(candidates) >= 2 { b.WriteString("  [2] use this") }
    return b.String()
}
```

Wire into `View()`:

```go
case screenConflictResolve:
    return renderShell(m.renderConflictResolve(), stageSetup, m.width)
```

**Acceptance:** `go test ./pkg/tui` passes; golden regenerated.

---

### Task 5 — Unit tests for conflict resolution (12a)

**File:** `pkg/tui/dashboard_test.go`  
**Depends on:** Tasks 3, 4  
**Risk:** Low

New tests:
- `TestConflictClient_CursorReachesConflict` — send scan with conflict client; press `p`, Enter (validate), wait for screenTargetSelect; press `↓` until cursor is on conflict entry; assert `allTargetEntries[m.clientCursor].isConflict == true`.
- `TestConflictClient_ROpensOverlay` — from above, press `r`; assert `m.screen == screenConflictResolve`, `m.resolveTarget != nil`.
- `TestConflictResolve_1MovesToEligible` — inject `screenConflictResolve` state; press `1`; assert `m.resolvedConflicts[id].ChosenPath != ""`, `m.selectedClients[id] == true`, `m.screen == screenTargetSelect`.
- `TestConflictResolve_2UsesSecondCandidate` — press `2`; assert second candidate path stored.
- `TestConflictResolve_SSkipsClient` — press `s`; assert `Skipped == true`, client NOT in `selectedClients`.
- `TestConflictResolve_EscCancels` — press `Esc`; assert `m.resolvedConflicts` unchanged, `m.screen == screenTargetSelect`.
- `TestAllTargetEntries_IncludesResolvedConflict` — pure function test; resolved non-skipped conflict appears in entries with `isConflict: false`.
- `TestAllTargetEntries_ExcludesSkippedConflict` — skipped conflict absent from all entries.

**Acceptance:** `go test ./pkg/tui -run TestConflict -v` all pass.

---

### Task 6 — Golden test for `screenConflictResolve` (12a)

**File:** `pkg/tui/dashboard_golden_test.go`  
**Depends on:** Task 4  
**Risk:** Low

```go
func TestGoldenScreenConflictResolve(t *testing.T) {
    scanner := &FakeScanner{Report: doctor.Report{Platform: "darwin"}}
    m := NewDashboardModel(scanner, nil, nil)
    m.scanning = false
    m.screen = screenConflictResolve
    m.resolveTarget = &doctor.ClientFinding{
        ID:   "antigravity",
        Name: "Antigravity IDE",
        Candidates: []doctor.CandidateFinding{
            {Label: "repo-current", Path: "/home/.gemini/config/mcp_config.json",
                Exists: true, ParseOK: true, Providers: []string{"exa"}},
            {Label: "alternate-symlink", Path: "/home/.gemini/antigravity/mcp_config.json",
                Exists: true, IsSymlink: true, Resolved: "/home/.gemini/antigravity-data/mcp_config.json",
                ParseOK: true},
        },
    }
    m = injectWidth(m)
    golden.RequireEqual(t, []byte(m.View()))
}
```

Generate: `NO_COLOR=1 go test ./pkg/tui -run TestGoldenScreenConflictResolve -update`

**Acceptance:** Golden file committed; test passes without `-update`.

---

### Task 7 — E2E Playwright-style flow tests (12b)

**File:** `pkg/tui/dashboard_flow_test.go` (new)  
**Depends on:** Tasks 1–6  
**Risk:** Low

```go
package tui
```

Uses: `FakeDashboardManager`, `FakeScanner`, `waitForText`, `waitForAll`, `teatest`.

**`TestDashboardFlow_HappyPath`:**

```go
func TestDashboardFlow_HappyPath(t *testing.T) {
    scanner, mgr, profiles := happyFlowSetup(t)
    m := NewDashboardModel(scanner, mgr, profiles)
    tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

    waitForText(t, tm, "System Status")                    // 1. doctor screen
    tm.Send(keyMsg("p"))
    waitForText(t, tm, "Provider Readiness")               // 2. provider ready
    tm.Send(keyMsg("enter"))
    waitForText(t, tm, "Select Targets")                   // 3. target select
    tm.Send(keyMsg("enter"))
    waitForText(t, tm, "Plan Preview")                     // 4. plan preview
    tm.Send(keyMsg("y"))
    waitForText(t, tm, "Apply Result")                     // 5. apply result

    tm.Send(keyMsg("q"))
    tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

    final := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second))
    got := final.(DashboardModel)
    if got.screen != screenApplyResult {
        t.Errorf("expected screenApplyResult, got %d", got.screen)
    }
    if got.applyResult == nil {
        t.Error("expected applyResult non-nil after happy path")
    }
}
```

Helper `happyFlowSetup`:

```go
func happyFlowSetup(t *testing.T) (*FakeScanner, *FakeDashboardManager, []provider.CredentialProfile) {
    t.Helper()
    planID, _ := app.NewPlanID()
    now := time.Now().UTC()
    mgr := &FakeDashboardManager{
        Plan: app.SavedPlan{
            PlanID: planID, ProviderID: "exa",
            CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour),
            SchemaVersion: 2,
        },
        Result: app.ApplyResult{
            UpdatedTargets: []string{"/home/.gemini/antigravity-cli/mcp_config.json"},
        },
        home: t.TempDir(),
    }
    scanner := &FakeScanner{Report: doctor.Report{
        Platform: "darwin",
        Clients: []doctor.ClientFinding{{
            ID: "antigravity-cli", Name: "Antigravity CLI",
            Confidence: doctor.ConfidenceHigh, Installed: true,
            EffectivePath: "/home/.gemini/antigravity-cli/mcp_config.json",
        }},
    }}
    profiles := []provider.CredentialProfile{{
        ProviderID: "exa",
        Values:     map[string]string{"EXA_API_KEY": "exa-test-key-placeholder"},
        Label:      "test",
    }}
    return scanner, mgr, profiles
}
```

**`TestDashboardFlow_ValidationFails`:**
Set `mgr.ValidErr = errors.New("invalid key format")`. Walk to provider ready. Press `Enter`. Wait for error text. Assert screen stays on `screenProviderReady` via `FinalModel`.

**`TestDashboardFlow_PlanFails`:**
Set `mgr.PlanErr = errors.New("plan error")`. Walk to target select. Press `Enter`. Wait for error text visible. Assert screen stays `screenTargetSelect` via `FinalModel`.

**`TestDashboardFlow_ApplyFails`:**
Set `mgr.ApplyErr = errors.New("apply error")`. Walk to plan preview. Press `y`. Wait for `"Apply Result"`. Assert error text visible and `screen == screenApplyResult` via `FinalModel`.

**`TestDashboardFlow_EscNavigation`:**
Press `p` → wait "Provider Readiness" → Esc → wait "System Status" → press `p` → wait "Provider Readiness" → Enter → wait "Select Targets" → Esc → wait "Provider Readiness".

**`TestDashboardFlow_NoRawCredential`:**
`happyFlowSetup` with `EXA_API_KEY: "11111111-1111-1111-1111-111111111111"`. Run happy path. At each `waitForText`, assert UUID not in the waited output. After `WaitFinished`, collect all output seen by `tm.Output()` — confirm no UUID. (Use a custom `assertOutputNeverContains` helper that records all bytes from `tm.Output()` before quitting.)

**`TestDashboardFlow_ConflictResolution`:**
Scanner report includes one conflict client (`ConfidenceConflict`, two candidates). Walk to target select. Navigate down to conflict entry (`↓` until cursor on conflict). Press `r`. Wait for "Resolve Conflict". Press `1`. Wait for "Select Targets". Assert resolved client now in eligible section (visible without `?`). Press `Enter`. Walk to plan preview. Assert plan includes resolved client. Press `n`. Press `Esc`. Quit.

**Acceptance:** `go test ./pkg/tui -run TestDashboardFlow -v` all pass; no `time.Sleep`.

---

## Task Dependency Graph

```
PR 12a — Conflict resolution:
  Task 1  (types + constants + fields)   independent
  Task 2  (helpers + updated functions)  depends on Task 1
  Task 3  (key handlers)                 depends on Task 2
  Task 4  (view renders)                 depends on Tasks 2, 3
  Task 5  (unit tests)                   depends on Tasks 3, 4
  Task 6  (golden test)                  depends on Task 4

PR 12b — E2E flow tests:
  Task 7  (flow tests)                   depends on Tasks 1–6 (all 12a done)

PR 12c — Candidate-level target rows:
  Task 8  (target-row model)              depends on Tasks 1–7
  Task 9  (target-row rendering)          depends on Task 8
  Task 10 (concrete planning input)       depends on Task 8
  Task 11 (workspace/multifile matrix)    depends on Tasks 8–10
```

---

## Verification Commands

```bash
# PR 12a:
go test ./pkg/tui -run "TestConflict|TestAllTarget" -v
NO_COLOR=1 go test ./pkg/tui -run "TestGolden" -update
go test ./pkg/tui

# PR 12b:
go test ./pkg/tui -run "TestDashboardFlow" -v

# PR 12c:
USYNC_UX_MATRIX=1 go test ./pkg/tui -run "TestDashboardFlowMatrix_(Workspace|Multi|Resolved)" -v
go test ./pkg/app ./pkg/tui

# Full suite:
NO_COLOR=1 TERM=xterm-256color go test ./...
```

---

## Risks and Mitigations

| Risk | Severity | Mitigation |
|---|---|---|
| `allTargetEntries` breaks Phase 7/8 tests via changed `eligibleClientIDs` signature | Medium | Update all callers of `eligibleClientIDs` in Task 2; existing tests use `nil` resolved map which gives same result as before |
| teatest timing flakiness on slow CI | Low | `WithDuration(3*time.Second)` gives ample headroom; `WithCheckInterval(25ms)` is fast |
| `FakeDashboardManager.Plan` serialisation failure in `PlanStore.Save` | Low | Ensure `SchemaVersion: 2`, `PlanID`, `ProviderID`, `CreatedAt`, `ExpiresAt` all set in `happyFlowSetup` |
| `resolveTarget` pointer aliased across rescans | Low | Store copy (`c := finding; m.resolveTarget = &c`) not slice element pointer |
| Candidate rows drift from app manager files | High | Build planning input from row path/scope/kind, not by re-detecting all client files |
| Workspace targets accidentally committed | High | Show git warning in target row and plan preview; require explicit checked row |

---

## Architecture Approval Status

**Not yet approved.** Requires sign-off on:
1. Unified `targetEntry` slice replacing separate eligible/conflict cursor model.
2. `resolvedConflicts` persisting across rescans via `applyResolutions`.
3. Resolved conflict clients auto-selected in `selectedClients`.
4. `FinalModel(t)` assertions in flow tests (guide §12 pattern — reviewer confirms approach is correct for v1.3.10).
5. PR 12c planning boundary: either add concrete target input to `pkg/app`, or construct narrowed `AppConfig` values before planning.
