# UX Bug-Hunt Protocol v2 — State-Space Explorer

**Purpose:** find TUI flow bugs by enumerating the state space programmatically, not by waiting for a user to report one.
**Supersedes:** `docs/specs/doctor-mode-phase12/ux-bug-hunt-protocol.md` (v1) — v1 remains for history.
**Status:** Authoritative protocol from Phase 14 forward.
**Last updated:** 2026-05-23
**Companion spec:** `docs/specs/doctor-mode-phase14/spec.md`

---

## 0. Why v2

v1 worked. It found UX-01..UX-11 in Phase 12 and DM-P40..P69 in Phase 13. The cost was a human cycle per finding: screenshot → matrix row → test → fix. The product has 6 screens × 9 precondition classes × ~10 keys per screen = ~540 cells. v1 covers ~70 of those today. The rate of new findings is bounded by maintainer attention.

v2 changes the loop:

```text
v1: user reports → human writes matrix row → human writes test → fix
v2: explorer enumerates → analyzer finds anomalies → matrix rows generated → fix
```

Human attention now goes to: judging *fixes*, not finding *bugs*. Recordings (Phase 14 FR-11) become a way to feed real-user paths back into the explorer as seeds, so the explorer's coverage extends to scenarios it didn't model.

---

## 1. Core Loop

```text
┌─────────────────────────────────────────────────────────────────┐
│  1. ENUMERATE   FixtureSpec[]           ← declared scenarios     │
│       ↓                                                           │
│  2. DRIVE       Trace per fixture       ← keystrokes + snapshots │
│       ↓                                                           │
│  3. PROBE       Edges + invariants      ← advertised keys, I-*   │
│       ↓                                                           │
│  4. ANALYZE     Findings                ← graph analysis         │
│       ↓                                                           │
│  5. REPORT      findings.json + .md     ← machine + human view   │
│       ↓                                                           │
│  6. GATE        CI pass/fail            ← no finding w/o row     │
└─────────────────────────────────────────────────────────────────┘
```

Each step is a Go package under `pkg/uxexplore/`. Each step is testable on its own (per-step unit tests; integration tests for the full loop).

---

## 2. Step 1 — Enumeration

### 2.1 Fixture spec

A *fixture* is a declarative starting condition. The dimensions match v1's matrix dimensions:

| Dimension | Values | Notes |
|---|---|---|
| `Credentials` | `none`, `valid`, `invalid` | per provider |
| `Provider` | `requires-creds`, `no-key`, `runtime-missing` | selected provider's class |
| `Conflicts` | `none`, `one`, `many` | unresolved conflicts in report |
| `Targets` | `none`, `one`, `many`, `mixed-checked` | eligible target count |
| `Workspace` | `off`, `on` | `--include-workspace` |
| `ScanError` | `false`, `true` | scanner returns error |
| `ApplyError` | `false`, `true` | manager.Apply returns error |
| `PreflightWarnings` | `false`, `true` | preflight surfaces non-fatal warnings |

The full cartesian is 3·3·3·4·2·2·2·2 = **1728** cells. **Do not enumerate the full cartesian.** Prune to ~24–32 meaningful intersections per v1 protocol §"Matrix Dimensions":

```go
// pkg/uxexplore/fixtures.go (partial)
var canonicalFixtures = []FixtureSpec{
    {Name: "happy-path",             Credentials: "valid", Provider: "requires-creds", Targets: "one"},
    {Name: "no-creds-requires",      Credentials: "none",  Provider: "requires-creds", Targets: "one"},
    {Name: "no-creds-no-key",        Credentials: "none",  Provider: "no-key",         Targets: "one"},
    {Name: "conflict-then-resolve",  Credentials: "valid", Conflicts: "one",           Targets: "one"},
    {Name: "credential-anchor",      Credentials: "none",  Provider: "requires-creds", Conflicts: "one"},
    {Name: "scan-error",             ScanError: true},
    {Name: "apply-error",            Credentials: "valid", Provider: "no-key", ApplyError: true},
    // ...24+ total
}
```

### 2.2 Coverage contract

Every `(screen, precondition_class)` pair must be reachable by at least one fixture, OR be in `exclusions.yaml` with a `reason:` field.

```yaml
# pkg/uxexplore/exclusions.yaml
exclusions:
  - screen: CredentialEntry
    precondition_class: scan-error
    reason: "Credential entry is only reachable from screens that require a successful scan."
```

The coverage test (`UXE-05` in spec FR-8) enumerates the cells, checks fixtures cover them, and fails CI on uncovered cells without an exclusion.

### 2.3 Adding a new fixture

When a new screen lands or a new precondition class emerges:

1. Add the `FixtureSpec` entry.
2. Add an `exclusions.yaml` line for any cell genuinely unreachable.
3. Re-run `make ux-explore` and verify `coverage.json` is 100%.

When a new bug class emerges:

1. Add a new `precondition_class` constant.
2. Add a new `BlockReason` parser if the block is user-visible.
3. Add fixtures that trigger it.
4. The analyzer's existing detectors usually cover it; if not, extend `pkg/uxexplore/analyze.go`.

---

## 3. Step 2 — Driving

### 3.1 Fixture → model

```go
// pkg/uxexplore/driver.go
func (d *Driver) buildModel(spec FixtureSpec) DashboardModel {
    scanner := buildScanner(spec)   // FakeScanner with seeded report
    mgr     := buildManager(spec)   // FakeDashboardManager with seeded errors
    profs   := buildProfiles(spec)  // valid|invalid|empty
    m := NewDashboardModel(scanner, mgr, profs)
    return m
}
```

`buildScanner`/`buildManager`/`buildProfiles` are pure functions over `FixtureSpec` — no fixture-specific branching in the driver itself.

### 3.2 Drive to initial state

The driver:

1. Calls `m.Init()` to start the scan.
2. Processes the resulting `tea.Cmd` (synchronously, via the existing `cmd()` invocation).
3. Records the first `VisitedState` after the scan completes (or scan-error settles).
4. From there, kicks off the probe (Step 3).

The driver **does not** synthesize advanced flows. It hands control to the probe at the post-scan state. Reaching deeper states is the probe's job, via key sequences.

### 3.3 Determinism

Every random source is seeded:

- `time.Now()` calls inside Update closures are mocked via a fixed `now func() time.Time`.
- `crypto/rand` (for plan IDs) is seeded.
- Map iteration order is normalized via sorted keys before fingerprinting.

A `make ux-explore` run is byte-identical across machines. This makes the `findings.json` diff a real signal in PRs.

---

## 4. Step 3 — Probing

### 4.1 At each visited state

```go
func (p *Probe) Visit(m DashboardModel, trace *Trace) {
    fp := Fingerprint(m)
    trace.Visited = append(trace.Visited, VisitedState{...})

    // Invariant pass
    for _, inv := range invariants {
        if err := inv.Check(m); err != nil {
            trace.Errors = append(trace.Errors, ExplorerError{Inv: inv.ID, Err: err})
        }
    }

    // Action bar pass
    keys := parseActionBar(m.View())
    for _, k := range keys {
        next, _ := m.Update(keyMsg(k))
        nextFP := Fingerprint(next.(DashboardModel))
        trace.Edges = append(trace.Edges, Edge{
            From: fp, Key: k, To: nextFP,
            Caused: classify(fp, nextFP, m, next.(DashboardModel)),
        })
        if nextFP != fp {
            p.Visit(next.(DashboardModel), trace)  // recurse, with visited-set dedup
        }
    }

    // Unmapped-key pass
    for _, k := range unmappedKeys {  // ['x', '5', 'F1', ...]
        next, _ := m.Update(keyMsg(k))
        if Fingerprint(next.(DashboardModel)) != fp {
            trace.Errors = append(trace.Errors, ExplorerError{
                Kind: "unmapped-key-changed-state", Key: k,
            })
        }
    }

    // Double-press pass for in-flight booleans
    if m.scanning || m.validating || m.planning || m.applying {
        primary := primaryKeyFor(m)
        _, _ = m.Update(keyMsg(primary))
        _, _ = m.Update(keyMsg(primary))
        // assert second press didn't trigger a duplicate Cmd
    }
}
```

### 4.2 Action-bar parser

```go
var actionBarRE = regexp.MustCompile(`\[([^\]]+)\]\s+(\S+)`)

func parseActionBar(view string) []string {
    line := lastNonEmptyLine(view)  // action bar is last
    matches := actionBarRE.FindAllStringSubmatch(line, -1)
    keys := make([]string, 0, len(matches))
    for _, m := range matches {
        keys = append(keys, normalizeKey(m[1]))
    }
    return keys
}
```

`normalizeKey` maps `↑↓` → `up`/`down`, `Esc` → `esc`, `Enter` → `enter`, etc. A unit test (`UXE-03`) asserts the parser handles every action bar variant in the codebase. If a new action bar format slips past the parser, the parser fails closed (returns empty keys → the probe reports zero advertised keys → the analyzer fires `orphan state` → CI fails).

### 4.3 Cycle detection

The probe uses a visited-set on `(StateFingerprint, ViewDigest)` to avoid infinite loops. Two visits with identical fingerprint + identical view → already explored; skip.

---

## 5. Step 4 — Invariants

Invariants are pure functions of `m DashboardModel`. Each returns `nil` (pass) or `error` (failure with a diagnostic message).

### 5.1 Inherited from v1 (I-01..I-15)

These are unchanged. See `docs/specs/doctor-mode-phase12/ux-bug-hunt-protocol.md` §"Core Invariants".

### 5.2 New in v2

| ID | Invariant | Why |
|---|---|---|
| **I-16** | Cursor on rendered row | Phase 13 anchor (DM-P41). Generalized: `m.providerCursor` must be in `RenderedProviderIndices(...)`; `m.clientCursor` must point at a rendered target entry. |
| **I-17** | Progress-edge existence | Phase 14 anchor (UX-11). Every non-terminal state must have at least one outbound edge whose target has a different `Screen` OR different `PreconditionClass`. `Esc` to same-class screen does not count. Terminal states tagged explicitly in `pkg/uxexplore/terminals.go`. |

### 5.3 Adding an invariant

1. Implement `Check(m DashboardModel) error` in `pkg/uxexplore/invariants.go`.
2. Add an entry to `invariants` slice with a stable ID (`I-<n>`).
3. Add a unit test that fails-closed: construct a `DashboardModel` that violates the invariant, assert the check returns error.
4. Add a regression test that fails when the invariant is removed.

Invariants must be cheap (microseconds). Heavy checks belong in the analyzer, not the per-visit invariant pass.

---

## 6. Step 5 — Anomaly Analysis

### 6.1 Detectors

Run over the complete `Trace` after driving + probing finish.

| Detector | Algorithm | Output kind |
|---|---|---|
| **Dead-end** | For each non-terminal state S, check every outbound edge's target. If all targets have `(Screen, PreconditionClass, BlockReason) == S.fp`, S is a dead-end. | `dead-end` |
| **Silent no-op** | For each edge (S, k, S') where S == S' (fingerprint and ViewDigest both match), check if `k` was advertised in S. If yes → `silent no-op`. | `silent-noop` |
| **Hidden-cursor** | Direct from I-16 failures during probe. | `hidden-cursor` |
| **Orphan state** | State with zero outbound edges in `trace.Edges` and not in `terminals.go`. | `orphan` |
| **Repeating-error cycle** | Tarjan SCC on the edge graph; any SCC with size ≥ 2 where every visited state has `HasError == true` and identical `BlockReason` → cycle. | `error-cycle` |
| **Unadvertised reachable key** | For each screen, compare `handleKey*` cases (via go/ast) with action-bar advertised keys (parsed at runtime). Cases present in handler but never in any visited action bar → unadvertised. | `unadvertised-key` |
| **Advertised unreachable key** | Inverse: keys in any action bar that have no `handleKey*` case → fail. | `unreachable-key` |
| **Invariant violations** | Per-visit invariant failures aggregated. | `invariant-violation` |

### 6.2 Recommendations

Each finding includes a recommendation:

```go
type Finding struct {
    Kind           FindingKind
    Fixture        string
    Path           []string
    State          StateFingerprint
    Recommendation string
    MatrixID       string  // proposed DM-P<auto-id>, deterministic from finding hash
}
```

`Recommendation` is a one-line human suggestion. Examples:

- **Dead-end:** `Add a recovery key advertised in S's action bar that transitions to a different PreconditionClass.`
- **Silent no-op:** `Drop the advertised key [k] from S's action bar, or implement the handler to mutate state.`
- **Hidden-cursor:** `Clamp m.<cursor> to rendered indices in the handler for key [<k>].`

`MatrixID` is generated from `sha256(finding-canonical-form)[:8]` so the same defect re-discovered in a later run keeps its ID. New defects get new IDs.

---

## 7. Step 6 — Reporting

### 7.1 Artifacts

```text
artifacts/ux-explore/
  findings.json              # machine-readable list of Findings
  findings.md                # human-readable, grouped by family
  proposed-matrix-rows.md    # ready-to-paste stubs for ux-flow-matrix.md
  coverage.json              # { visited_cells, total_cells, gaps[] }
  graph.dot                  # state graph (visualize with graphviz)
```

### 7.2 Findings.md format

```md
# UX Explorer Findings — 2026-05-23 17:23:01 UTC

## Dead-End (1)

### DM-PE3F1A2C — Credential dead-end on TargetSelect

**Fixture:** no-creds-requires
**Path:** `p`, `Enter`

**State:**
- Screen: TargetSelect
- PreconditionClass: missing-credentials
- BlockReason: "Credential profile required for Exa AI Search before planning"

**Outbound edges:**
- `[↑↓] navigate` → same fingerprint
- `[Space] toggle` → same fingerprint
- `[i] workspace(off)` → same fingerprint
- `[Esc] back` → ProviderReady, missing-credentials, "..."  (same PreconditionClass)
- `[q] quit` → (terminal)

**Recommendation:** Add a recovery key advertised in TargetSelect's action bar that transitions to a different PreconditionClass. Per spec FR-9, this is `[k] add credentials` → `screenCredentialEntry`.

## Silent No-Op (0)

## Hidden-Cursor (0)

## Orphan (0)

## Error-Cycle (0)
```

### 7.3 Proposed-matrix-rows.md format

Each finding becomes a stub in Phase-12 case-record format:

```md
### DM-PE3F1A2C Credential dead-end on TargetSelect

Preconditions:
- credentials: none
- provider: requires-creds
- targets: one

Keys:
- p
- Enter

Visible promise before action:
- Footer says `[Esc] back  [q] quit`

Expected:
- A forward action is advertised OR an inline recovery instruction is present.

Actual:
- Footer offers `[Esc] back` only; Esc returns to ProviderReady with same PreconditionClass.

Invariant failures:
- I-13 (every error state must show a recovery action)
- I-17 (every non-terminal state must have a progress edge)

Code confirmation:
- `pkg/tui/dashboard_view.go:250` (renderTargetSelect — block message)
- `pkg/tui/dashboard_view.go:305` (actionBarTargetSelect — footer composition)

Test:
- (proposed) TestDashboardFlow_CredentialDeadEndOffersRecovery
```

The human fills in `Expected` if the stub got it wrong, edits invariants, and pastes into `ux-flow-matrix.md`.

---

## 8. CI Integration

### 8.1 Gates

`make ux-explore` runs the full loop. CI fails when:

| Condition | Failure mode |
|---|---|
| Any new `Finding` not in `ux-flow-matrix.md` or `findings-allowlist.yaml` | Merge-blocking |
| `coverage.json.gaps[]` non-empty AND not in `exclusions.yaml` | Merge-blocking |
| `artifacts/ux-explore/findings.json` byte-different from main without justification | PR comment with diff; not blocking by default |
| Run time exceeds 60s | Warning (Phase 14 budget: 30s) |

### 8.2 Allowlist

`pkg/uxexplore/findings-allowlist.yaml` holds findings the team has accepted as known issues with deferred fixes:

```yaml
allowlist:
  - matrix_id: DM-PE3F1A2C
    accepted_at: 2026-05-25
    accepted_by: nawodyaishan
    reason: Tracked in Phase 14 FR-9; fix lands in PR 14e.
    expires_at: 2026-06-15  # CI fails after this date
```

Expirations are checked daily by a CI scheduled job. An expired allowlist entry fails the next PR run.

### 8.3 PR comment

A GitHub Action (or equivalent) posts a comment summarizing:

- New findings (compared to base branch).
- Cleared findings (no longer present).
- Coverage delta.

---

## 9. Recording-as-Seed (FR-13)

A `--record`ed session is a path through the dashboard. The explorer ingests it:

```go
// pkg/uxexplore/seed.go
func (e *Explorer) RunWithSeed(seed []recordEntry) (*Trace, error) {
    m := NewDashboardModel(e.scanner, e.manager, e.profiles)
    m, _ = m.Update(scanResultMsg{...})

    // Replay the seed keystrokes
    for _, entry := range seed {
        if entry.T != "key" {
            continue
        }
        m, _ = m.Update(keyMsg(entry.Key))
    }

    // Start exploring from where the seed ended
    return e.exploreFrom(m), nil
}
```

The trace's `Origin` field is `seeded` instead of `enumerated`. Findings from seeded traces are tagged `discovered-via-recording` and get a 24-hour suppression window (FR-13).

---

## 10. Migration From v1

### 10.1 What v1 cases become

| v1 Matrix Row | v2 Treatment |
|---|---|
| Hand-written teatest case under `DM-P*` | Becomes a *named scenario* the explorer can reproduce. Stays in the test file; no rewrite. |
| Render/golden test | Stays. Tagged `Origin: manual` — explorer doesn't generate golden assertions. |
| Invariant assertion | Lifted into `pkg/uxexplore/invariants.go` if generally applicable. |
| One-off bug repro from a screenshot | Becomes a recording in `artifacts/journeys/` and replayed by the explorer. |

### 10.2 The v1 doc

`docs/specs/doctor-mode-phase12/ux-bug-hunt-protocol.md` gets a banner at the top:

```md
> **Superseded by Phase 14 — see [ux-bug-hunt-protocol-v2.md](../doctor-mode-phase14/ux-bug-hunt-protocol-v2.md)**
> Kept for history. The 15 invariants (I-01..I-15) and matrix dimensions are still authoritative;
> the *process* (manual screenshot → matrix row) is superseded by the explorer.
```

No content removed. Phase 12 work continues to be a valid historical reference.

---

## 11. Anti-Patterns

The protocol explicitly **rejects** these temptations:

1. **Don't fuzz arbitrary input sequences.** The dashboard keymap is finite. Deterministic enumeration over advertised keys + sampled unmapped keys is sufficient and reproducible.

2. **Don't special-case the explorer in production code.** If the explorer can't drive a screen, the screen has a defect. The explorer uses only the public `NewDashboardModel`, `Init`, `Update`, `View` surface.

3. **Don't generate matrix rows the human will rewrite.** The proposed-matrix-rows stubs are intentionally minimal — preconditions, keys, actual, invariants. `Expected` is human-filled because the human is the one who knows the desired contract.

4. **Don't gate on coverage-percentage.** Gate on cell-level coverage. 95% line coverage with a missing precondition class is worse than 70% line coverage with 100% cell coverage.

5. **Don't drop hand-written tests.** Render/golden tests, redaction guards, and Phase-12 named scenarios remain. The explorer covers behavior; goldens cover appearance.

6. **Don't make `findings.json` advisory.** It either passes CI or it doesn't. Advisory output trains the team to ignore output.

7. **Don't ship the explorer without recording.** Recording is how real-user paths re-enter the loop. Without it, the protocol is closed under what we predicted.

---

## 12. Pass / Fail Rules (v2)

A run fails when **any** of these are true:

- `findings.json` is non-empty AND not allowlisted.
- `coverage.json.gaps[]` is non-empty AND not in `exclusions.yaml`.
- Any invariant I-01..I-17 fails on any visited state.
- The action-bar parser returns zero keys for a non-terminal state.
- A `handleKey*` case exists for a key never advertised by any visited action bar (`unadvertised-key`).
- An action bar advertises a key that has no `handleKey*` case (`unreachable-key`).
- A run is non-deterministic (re-run produces different `findings.json` byte hash).
- The 30s runtime budget is exceeded by > 100%.

---

## 13. Glossary

- **Fixture** — a declarative `(scanner, manager, profiles)` configuration that seeds a starting model state.
- **State fingerprint** — `(Screen, PreconditionClass, BlockReason, HasError, InFlight)` tuple. Two states with identical fingerprints are considered semantically the same.
- **PreconditionClass** — coarse classification of why a screen is in its current state. Enum: `ok`, `missing-credentials`, `conflict-unresolved`, `no-targets-selected`, `scan-error`, `plan-error`, `apply-error`, `runtime-missing`, `network-failure`.
- **BlockReason** — user-visible explanation parsed from the rendered body. Stable text contract.
- **ViewDigest** — sha256 of the redacted `View()` output. Used with fingerprint to dedup identical visits.
- **Edge** — `(from_fingerprint, key, to_fingerprint, caused)`. The graph of state transitions.
- **Probe** — the per-state action that snapshots, checks invariants, fires advertised keys + unmapped keys + double-press.
- **Trace** — the complete recording of a fixture's run: visited states + edges + errors.
- **Finding** — an anomaly detected by analysis. Has a stable `MatrixID` derived from its canonical form.
- **Seed** — a `--record`ed user session used as a forced exploration path.
- **Coverage cell** — a `(screen, precondition_class)` pair. Coverage gates on cell-level.
- **Terminal state** — a state designed to have no progress edges (ApplyResult, post-quit). Explicitly enumerated.

---

## 14. References

- Spec: `docs/specs/doctor-mode-phase14/spec.md`
- Plan: `docs/specs/doctor-mode-phase14/plan.md`
- Tasks: `docs/specs/doctor-mode-phase14/tasks.md`
- v1 protocol (superseded): `docs/specs/doctor-mode-phase12/ux-bug-hunt-protocol.md`
- Phase 12 UX audit (anchor for UX-11): `docs/specs/doctor-mode-phase12/user-flow-audit.md`
- Original screen design: `docs/research/Doctor Mode, Batch Plan:Apply, and Credential Validation new spec.md` §10
- Phase 13 architecture review (deferred): `docs/specs/doctor-mode-phase13/architecture-review.md`
