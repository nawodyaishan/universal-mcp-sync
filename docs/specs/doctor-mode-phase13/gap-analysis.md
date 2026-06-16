# Doctor Mode Phase 13 — Gap Analysis

**Last updated:** 2026-05-23
**Anchor incident:** user reports being stuck on Provider Readiness with "no response and no proceeding buttons" while manually walking the dashboard flow.
**Sources reviewed:**
- `pkg/tui/dashboard.go` — every key handler (`handleKey*`)
- `pkg/tui/dashboard_view.go` — every `render*` and action bar
- `pkg/tui/dashboard_readiness.go` — `ComputeReadiness` severity ladder
- `docs/specs/doctor-mode-phase12/ux-flow-matrix.md` — current locked rows
- `docs/specs/doctor-mode-phase12/user-flow-audit.md` — UX-01 … UX-11 already filed
- `artifacts/ux-fake-prod/{matrix.json,issues.json}` — current harness state (issues.json empty)

The Phase 12 matrix locks behavior for **13 scenarios** (DM-P05/10/12/14/15/19A/19B/20/31/32/33/34/35). Those rows assert specific contracts. They do **not** form an exhaustive (key × screen × precondition) grid. Phase 13 is the audit that closes that grid, with the user-reported dead-end as Scenario 0.

---

## A. The Anchor — Stuck On Provider Readiness

`ComputeReadiness` (`pkg/tui/dashboard_readiness.go:36`) sets every provider to `ProviderStateConflictBlocked` the moment **any** `doctor.ClientFinding` is `ConfidenceConflict`. `renderProviderReady` (`pkg/tui/dashboard_view.go:172`) then hides every conflict-blocked row from the list. The banner asks the user to press `[r]`. Three concrete dead-end shapes follow.

| Shape | What the user sees | What happens on the visible footer keys |
|---|---|---|
| A1. Conflict elsewhere, no ready provider visible | Banner only; zero provider rows | `[Enter]` calls `offlineValidationCmd` against `m.providerCursor=0` which points at a hidden row — silent advance into target select; `[v]` same; `[↑↓]` move an invisible cursor; `[r]` works but the connection to the missing provider list isn't obvious |
| A2. Banner persists after resolution | Conflict still listed even though it's resolved in `resolvedConflicts` | `[r]` re-enters target select where the row no longer appears as a conflict — feels circular; matches `UX-04` |
| A3. Validation in flight, second Enter | "Validating…" plus footer still advertising `[Enter] select` | Second `Enter`/`v` silently swallowed by the `if !m.validating` guard — no feedback that the action was ignored |

The shared root cause is **silent no-ops on advertised keys** plus **hidden cursor targets**. Stage-3 triage for these will live under DM-P40..P44.

---

## B. Coverage Audit — Key × Screen Grid

Phase 12 matrix rows lock the bold cells. Empty cells are Phase 13 candidates.

Legend: `L` = locked in Phase 12 matrix · `t` = covered only by unit tests · `_` = no coverage.

| Screen \ Key | `↑/k` | `↓/j` | `Enter` | `Space` | `Esc` | `r` | `v` | `w` | `i` | `1` | `2` | `s` | `y` | `n` | `?` | `q`/`ctrl+c` | unmapped |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| **Doctor** | — | — | t | — | — | t | — | t | — | — | — | — | — | — | `_` | t | `_` |
| **ProviderReady** | `_` | `_` | L(P31) | — | `_` | L(P10) | `_` | — | — | — | — | — | — | — | `_` | `_` | `_` |
| **TargetSelect** | t | t | L(P12,P19A,P19B,P20) | L(P12) | `_` | t | — | — | L(P14) | — | — | — | — | — | `_` | `_` | `_` |
| **ConflictResolve** | — | — | — | — | t | — | — | — | — | L(P19A) | L(P19B) | L(P20) | — | — | `_` | `_` | `_` |
| **PlanPreview** | — | — | t | — | t | — | — | — | — | — | — | — | t | t | `_` | `_` | `_` |
| **ApplyResult** | — | — | — | — | `_` | t | — | — | — | — | — | — | — | — | `_` | `_` | `_` |

The empty cells are 47 (key, screen) pairs with **no matrix locking**. Most are minor (unmapped keys must be no-ops, `q`/`ctrl+c` must quit from every screen). The high-value ones cluster in eight defect families below.

---

## C. Defect Families

### F1 — Hidden-cursor / hidden-row interactions on ProviderReady
The cursor advances past invisible rows because `m.providerCursor` is bounded by `len(m.readiness)`, not by the count of *rendered* rows. `Enter` then validates the wrong (invisible) provider. **No matrix row guards this today.**

### F2 — Action-bar advertised key with silent no-op
`[r]` on ProviderReady with no conflicts is a no-op (locked by DM-P10), but `[Enter]` while `m.validating==true`, `[Enter]` while `m.planning==true`, `[y]` while `m.applying==true`, and `[r]` rescan while `m.scanning==true` are all silent no-ops that match the same defect class.

### F3 — Esc semantics across the back-stack
Three Esc round-trips have no matrix coverage:
- ProviderReady → Esc → Doctor: scan result preserved? readiness state cleared?
- TargetSelect → Esc → ProviderReady: selections preserved? cursor restored?
- PlanPreview → Esc/n → TargetSelect: selection map and includeWorkspace preserved?

### F4 — State persistence across rescan
After apply success the model calls `scanCmd` (`pkg/tui/dashboard.go:680`). Whether `resolvedConflicts`, `selectedTargets`, `includeWorkspace`, and `providerCursor` survive that rescan is undocumented. The Phase 12 open question explicitly says "Yes — `resolvedConflicts` persists" but no automation locks it.

### F5 — Re-entrancy (double-press / debounce)
Every long-running operation (scan, validation, plan, apply) is guarded by a boolean. Double-press during the operation is silently dropped. The product invariant (I-03 in protocol) says repeating the primary action must not repeat the same unrecoverable error — but it also implies the second press must produce *visible feedback*. Neither outcome is tested.

### F6 — Help overlay (`?`)
`?` toggles `m.showHelp`, which short-circuits `View()` to `renderHelpOverlay()`. Nothing tests that:
- The overlay opens from each of the 6 screens.
- The overlay closes on second `?` and the prior screen content is intact (cursor, selections).
- Help text mentions the current-screen keymap rather than a static catalog.
- Keys other than `?` while the overlay is open are still routed correctly (today they appear to be — `handleKey` runs first, then `showHelp` is checked in `View()` only).

### F7 — Batch apply end-to-end
The current matrix focuses on single-target plans. A real "batch" run cuts across many targets (multi-app, multi-scope, mixed CLI-managed + file-write). No row covers:
- Plan preview rendering with N > 3 targets at width 80.
- Apply with one file-target success + one CLI-target success.
- Skip-on-identical (Phase 11 FR-2) surfaced in the TUI Apply Result as an "Unchanged" section.
- Sequential second pass: apply Exa, rescan, plan Context7, apply — does session state interfere?

### F8 — ConflictResolve edge cases
Single-candidate conflict, three-or-more-candidate conflict (overlay only exposes 1/2), unmapped digit keys (`0`, `3`), and sequential resolution of two conflict clients in one session are uncovered.

---

## D. Severity Classification

| Family | Severity | Justification |
|---|---|---|
| F1 hidden-cursor | **Blocker** | matches the user's reported "no response" experience |
| F2 silent no-op on advertised key | **Major** | breaks invariant I-03 / I-13 |
| F3 Esc round-trip state | **Major** | data loss feels like a bug, not a feature |
| F4 rescan persistence | **Major** | Phase 12 spec promises persistence; absence of test means regressions land silently |
| F5 re-entrancy | **Minor** | irritating but recoverable |
| F6 help overlay | **Minor** | visible-state issue only |
| F7 batch apply | **Major** | core promise of `usync` is the batch — must be tested at scale |
| F8 ConflictResolve digits | **Minor** | only triggers with malformed scenarios |

---

## E. Out Of Scope For Phase 13

- Refactoring `ComputeReadiness` to use unresolved conflicts (that's UX-04 — staying in Phase 12 backlog).
- Persisting conflict resolution to disk across sessions (Phase 12 explicit non-goal).
- Window resize / responsive layout audit (separate "inconsistent layout" lens, Phase 14 candidate).
- Performance budget enforcement (`>100ms keypress→render`, separate lens).

---

## F. From Gaps To Matrix Rows

The next document, `ux-flow-matrix.md`, materializes one row per gap above. Each row carries: preconditions, key sequence, expected behavior, invariant references, automation pointer, and status (`Proposed` → `Captured` → `Locked`).
