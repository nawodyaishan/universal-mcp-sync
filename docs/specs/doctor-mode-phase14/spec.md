# Doctor Mode Phase 14: State-Space Explorer + Restored Credential Entry + Session Recording

**Type:** UX correctness + observability + protocol upgrade
**Status:** Approved for scoped PR 14e in-memory credential-entry slice; explorer, persistence, recorder, and replay remain pending.
**Last updated:** 2026-05-25
**Builds on:** Phase 11 (test infrastructure, audit/redaction), Phase 12 (5-screen dashboard, UX audit UX-01..UX-11, bug-hunt protocol v1), Phase 13 (key×screen matrix DM-P40..P69, apply-error recovery)
**Supersedes:** `docs/specs/doctor-mode-phase12/ux-bug-hunt-protocol.md` (kept for history; v2 is canonical from this phase forward)
**Anchor:** user-reported dead-end on **Select Targets** with no credential profile configured. Footer offers `[Esc] back` only; `Esc` loops to Provider Readiness where the same block exists. **Original Screen 2 design (research §10.4) included in-flow credential entry — never implemented.**
**Priority:** higher than Phase 13 architecture review (A1–A12). Deferred to Phase 15.

---

## 1. Problem Statement

Three structural issues, in order of severity:

### 1.1 The bug-hunt protocol is human-driven and reactive

The Phase 12 protocol (`ux-bug-hunt-protocol.md`) requires a human to:

1. Hit a bug while running the product manually.
2. Take a screenshot.
3. Hand-write a matrix-row stub.
4. Hand-write a teatest case.
5. Fix the code.

Every UX defect class costs the same human time — there is no compounding. Phase 13's matrix DM-P40..P69 (30 rows over 6 weeks) reflects the cost. The credential dead-end the user just hit is the latest example: it was filed as `UX-11 (Critical, Failing)` in Phase 12 nine months ago, the block landed (test passes — no plan call happens with zero profiles), but the recovery path was never implemented and no new bug-hunt iteration noticed.

**A protocol that requires a human to find each dead-end will keep producing dead-ends.**

### 1.2 The implementation never finished Screen 2 from the original research

The original research (`docs/research/Doctor Mode, Batch Plan:Apply, and Credential Validation new spec.md` §10.4) specified **Screen 2 — Provider + Credential Entry** with built-in credential input:

```text
Configure providers
...
Needs credentials
  [ ] GitHub        GITHUB_PERSONAL_ACCESS_TOKEN
                    Get key: https://github.com/settings/tokens
  [ ] Tavily        TAVILY_API_KEY
                    Get key: https://app.tavily.com/home

Enter credentials for selected providers:

EXA_API_KEY: [already loaded from env — not shown]

[Validate credentials (offline)]  [Continue to preview]  [Back]
```

The implementation collapsed Screen 2 to "Provider Readiness" (a read-only state view) and pushed credential entry out-of-band: `--keys`, `--keys-file`, or the legacy `[w] wizard`. The credential **entry** step was never built into the dashboard. Today, a first-launch user without `--keys` reaches **Select Targets**, sees the block, and has no in-flow path forward — exactly what the user reported. **The dead-end is a missing implementation, not a new feature ask.**

### 1.3 No mechanism to capture "I got stuck here" reproducibly

A stuck user must screenshot the terminal and describe in prose what they pressed. The `make ux-fake-prod` harness records its own runs but cannot ingest a real-user session. Real-user reports are the most signal-dense input the project gets, and they are currently lost on the way from terminal to issue tracker.

---

## 2. Goals

Three pillars, ordered by leverage:

### Pillar A — UX Explorer (the protocol upgrade)

Build `pkg/uxexplore/` — a Playwright-style state-space explorer that:

- Enumerates `(screen × precondition_class)` fixtures from a declarative spec.
- Drives the dashboard to every reachable state via the existing key handlers.
- Probes every advertised footer key (and a sample of unmapped keys) from every visited state.
- Snapshots model state + parses advertised keys from the rendered footer.
- Checks invariants I-01..I-17 on every visit.
- Runs graph analysis to detect dead-ends, silent no-ops, hidden-cursor states, repeating-error cycles, orphan states.
- Emits `artifacts/ux-explore/findings.json` + auto-generated matrix-row stubs.
- Fails CI on any new finding without a matching matrix row + justification.

The explorer is the **engine**. Pillars B and C are the first proof that the engine works.

### Pillar B — Restored Credential Entry (the anchor fix)

Implement **Screen 2 credential entry** as originally specified (research §10.4):

- New screen `screenCredentialEntry` reachable via `[k]` from ProviderReady or TargetSelect when `selectedProviderNeedsCredentials() == true`.
- Inline form per `RequiredCredentials()` entry — label + docs URL + masked input field.
- Submit adds an in-memory `provider.CredentialProfile` and returns to the prior screen.
- Optional `[s]` after submit prompts to persist via existing `--keys-file` mechanism.

The explorer (Pillar A) validates the fix: the credential dead-end finding disappears from `findings.json` once Pillar B lands, and no new findings appear from the new screen.

### Pillar C — Session Recording (seed for the explorer)

A `--record` flag on `usync dashboard` writes a JSONL transcript of keystrokes + screen transitions + redacted errors to `artifacts/journeys/<timestamp>.jsonl`. A `usync replay <file>` subcommand drives the dashboard from that transcript against fakes; `--emit-matrix` converts a recording into a Phase-12-format matrix-row stub.

Recordings are **seed input** for the explorer: a real-user path the explorer didn't predict becomes a forced exploration path the explorer extends from.

---

## 3. Non-Goals

- Architecture refactors (Phase 13 review A1/A2/A3/A8). **Phase 15.**
- Persisting credentials beyond the existing `--keys-file` TOML mechanism.
- Cross-session journey playback against *real* disk. Replay drives fakes only.
- Visual redesign (theme, color, layout). Same widgets, additive copy and one new overlay only.
- MCP server mode (`usync-mcp`). Out-of-scope per research §18.
- Property-based fuzzing of arbitrary input sequences. The dashboard keymap is finite; deterministic enumeration is enough.
- Replacing Phase 12 teatest cases. Existing tests remain as **named scenarios** the explorer can reproduce and verify.

---

## 4. Users / Actors

| Actor | Concern addressed | Pillar |
|---|---|---|
| First-launch user, no `--keys` | One-key recovery (`[k]`) to in-flow credential entry; no dashboard quit | B |
| Returning user | Footer explains *why* `[Enter] plan` is missing on a given screen state | B + protocol UX-3 |
| User filing a bug report | `usync dashboard --record` produces a JSONL transcript; one paste, no prose | C |
| Developer triaging the report | `usync replay <file>` reproduces the session locally; `--emit-matrix` drafts the row | C |
| QA running the bug-hunt loop | `make ux-explore` finds new defects without writing tests; coverage gates CI | A |
| Future contributor adding a screen | Adds one fixture entry; explorer covers the new (screen × precondition) cells automatically | A |
| Phase 12 audit follow-through | UX-11 (credential dead-end) closes; protocol v2 enforces no regression | A + B |

---

## 5. Alignment with Original Research

This phase **restores intent** more than it adds capability. Mapping:

| Original Research | Implemented Today | Phase 14 Fix |
|---|---|---|
| Research §10.4 Screen 2 has credential entry with form fields | Dashboard shows "Provider Readiness" — read-only; entry lives in `--keys`/wizard | Pillar B (`screenCredentialEntry`) |
| Research §10.1 flow: Dashboard → Conflict → Provider+Credential → Plan → Apply | Dashboard → Provider Ready → Target Select → Plan Preview → Apply (no Credential step) | Pillar B (inserts the missing step) |
| Research §13.1 three-phase validation: offline → live → batch | Offline + live exist; batch validation tied to plan | No change |
| Research §13.4 UI states: `ready`, `provided_unverified`, `verified_live`, `live_failed`, `validating`, `missing` | Today uses `ready`/`no-key-needed`/`missing-credentials`/`runtime-missing`/`conflict-blocked` | No change to states; Pillar B closes the `missing-credentials → provided` transition |
| Research §12.4 approval gates for project-scope writes | Workspace toggle exists; approval prompt missing for project-scope targets | Reported by explorer; **not fixed in 14** (Phase 15 candidate) |
| Bug-hunt protocol invariant I-04 "credential-required providers cannot plan with zero credential profiles" | Block enforced; no recovery | Pillar B + explorer guard |
| Bug-hunt protocol invariant I-13 "every error state must show a recovery action" | Several states violate (credential, scan-error, etc.) | Pillar A finds them; Pillar B fixes the credential one |

Phase 14 is the first phase that closes a gap *between the implementation and the original spec* rather than extending the spec.

---

## 6. Functional Requirements

### FR-1 — Explorer enumerator

`pkg/uxexplore/enumerator.go` defines the fixture matrix as data:

```go
type FixtureSpec struct {
    Name              string                  // "no-creds-with-conflicts"
    Credentials       CredentialClass         // none | valid | invalid
    Provider          ProviderClass           // requires-creds | no-key | runtime-missing
    Conflicts         ConflictClass           // none | one | many
    Targets           TargetClass             // none | one | many | mixed
    Workspace         bool
    ScanError         bool
    ApplyError        bool
    PreflightWarnings bool
}

func EnumerateFixtures() []FixtureSpec
```

`EnumerateFixtures()` returns the pruned cartesian product of meaningful intersections (per Phase 12 protocol §"Matrix Dimensions"). Initial set: 24–32 fixtures covering all `precondition_class` values in §FR-3 below. CI fails if a `FixtureSpec` field is added without a corresponding enumeration entry.

### FR-2 — Explorer driver

`pkg/uxexplore/driver.go` builds a `(scanner, manager, profiles)` triple from a `FixtureSpec`, instantiates `DashboardModel`, and walks the dashboard. It exposes:

```go
type Driver struct{ /* ... */ }

func NewDriver(spec FixtureSpec) (*Driver, error)
func (d *Driver) Run(ctx context.Context) (*Trace, error)

type Trace struct {
    Visited []VisitedState  // every fingerprint encountered
    Edges   []Edge          // (from, key, to, model_diff_summary)
    Errors  []ExplorerError // invariant violations, etc.
}
```

The driver uses the existing `keyMsg(s)` helper, the existing `Update(msg)` semantics, and the existing `View()` for snapshotting. **No production code path is special-cased for the explorer** — if the explorer can't drive a screen, that's a defect in the screen.

### FR-3 — State fingerprint and precondition classes

State is fingerprinted at a coarse-enough level that semantically identical states collapse:

```go
type StateFingerprint struct {
    Screen           dashboardScreen
    PreconditionClass string  // see enum below
    BlockReason      string   // empty if no block; otherwise the user-visible reason
    HasError         bool     // any of m.err, m.planErr, m.applyErr, m.validErr set
    InFlight         string   // empty | "scanning" | "validating" | "planning" | "applying"
}

// precondition_class values
const (
    PCOK                  = "ok"
    PCMissingCredentials  = "missing-credentials"
    PCConflictUnresolved  = "conflict-unresolved"
    PCNoTargetsSelected   = "no-targets-selected"
    PCScanError           = "scan-error"
    PCPlanError           = "plan-error"
    PCApplyError          = "apply-error"
    PCRuntimeMissing      = "runtime-missing"
    PCNetworkFailure      = "network-failure"
)
```

`BlockReason` is parsed from the rendered body (e.g. `"Credential profile required for Exa AI Search before planning"`) using a deterministic extractor. Free text changes break a regression test (`TestBlockReasonExtractor_StableAcrossRefactors`).

### FR-4 — Explorer probe

For each visited state, the probe:

1. Snapshots `View()` and `m` (deep-copied where pointers).
2. Parses the action bar to extract advertised keys via regex `\[([^\]]+)\]\s+(\w+)` against the trailing action-bar line.
3. For each advertised key: `Update(keyMsg(k))`, record the resulting fingerprint as an outbound edge.
4. For a fixed sample of unmapped keys (`x`, `5`, `F1`): `Update(keyMsg(k))`, assert no fingerprint change (I-01 / I-09).
5. For each in-flight boolean: send the primary key twice in a row, assert second press is a no-op (I-03, Phase 13 DM-P42/P43/P44 generalized).

### FR-5 — Invariant checker

Every visited state runs all applicable invariants. Invariants I-01..I-15 are inherited from Phase 12 protocol; two new ones land with this phase:

| ID | Invariant | Source |
|---|---|---|
| I-01 .. I-15 | (existing) | Phase 12 protocol §"Core Invariants" |
| **I-16** | **Cursor must point at a rendered row.** `m.providerCursor` ∈ `RenderedProviderIndices(...)`; `m.clientCursor` ∈ rendered-target-entry indices. | Phase 13 anchor |
| **I-17** | **Every reachable non-terminal state has at least one progress edge.** A progress edge is one whose target fingerprint has a *different* `PreconditionClass` *or* a *different* `Screen`. `Esc` to a same-class screen is not a progress edge. | Phase 14 anchor |

Terminal states (`ApplyResult` after apply, `Doctor` with scan-error post-rescan-attempt) are explicitly tagged in the explorer config — they are allowed to have zero progress edges.

### FR-6 — Anomaly detector

`pkg/uxexplore/analyze.go` runs graph algorithms over the `Trace`:

| Finding | Detection |
|---|---|
| **Dead-end** | State S where every outbound edge's target has the same `(Screen, PreconditionClass, BlockReason)` as S, and S is not a terminal state. |
| **Silent no-op on advertised key** | Edge (S, k, S') where `S == S'` (fingerprint and `View()` digest identical) and `k` was advertised in S's action bar. |
| **Hidden-cursor state** | Visit where I-16 fails. |
| **Orphan state** | State with zero outbound edges in the trace, not in the terminal set. |
| **Repeating-error cycle** | Cycle S → S' → S where both visits have `HasError == true` and `BlockReason` identical. |
| **Unadvertised reachable key** | Key handler exists in `handleKey*` for screen S but the action bar in S never renders it. |
| **Advertised unreachable key** | Action bar renders `[k]` but `handleKey*` for that screen has no case for `k`. |

Each finding produces a `Finding` struct that includes the reproducing fixture, key sequence, and a recommended matrix-row ID (`DM-P<auto-id>`).

### FR-7 — Findings → matrix rows

`pkg/uxexplore/report.go` emits:

1. `artifacts/ux-explore/findings.json` — machine-readable.
2. `artifacts/ux-explore/findings.md` — human-readable, grouped by family.
3. `artifacts/ux-explore/proposed-matrix-rows.md` — Phase-12-format stubs ready to paste into `ux-flow-matrix.md`.
4. `artifacts/ux-explore/coverage.json` — `{visited_cells, total_cells, gaps}`.

A finding without a matrix row is a CI failure. A matrix row without a corresponding test is a CI warning (becomes an error in Phase 15).

### FR-8 — Coverage contract

CI gates:

1. **Cell coverage:** every `(screen, precondition_class)` pair in the cartesian must be in `visited_cells` or explicitly excluded in `pkg/uxexplore/exclusions.yaml` with a reason.
2. **Edge coverage:** every advertised `(state, key)` edge probed at least once.
3. **Invariant coverage:** every I-01..I-17 evaluated at least once per `(screen, precondition_class)` cell.
4. **No new findings without matrix row:** see FR-7.

These gates run in `make ux-explore` and in the default `go test ./pkg/uxexplore/...`.

### FR-9 — Credential entry overlay (Pillar B anchor)

A new screen `screenCredentialEntry` is reachable via `[k]` from:

- **ProviderReady** when `m.readiness[providerCursor].State == ProviderStateMissingCredential`.
- **TargetSelect** when `selectedProviderNeedsCredentials() == true`.

The overlay:

1. Renders the provider name and one `credentialField` per entry in `RequiredCredentials()`. Each field shows the label and `DocsURL` from `provider.CredentialSpec`.
2. Field input semantics: typed characters append to `Value`; `Backspace` removes last byte; `Tab` / `Shift+Tab` cycle fields; `Enter` submits.
3. Display: each field renders as `[•••• supplied]` once non-empty (research §12.5 redaction rule; no chars shown).
4. Submit validates against `provider.OfflineValidator` if implemented. If validation fails, the screen stays open with a per-field error message.
5. On successful submit: append the new `provider.CredentialProfile{ProviderID, Values, Label: "interactive"}` to `m.profiles`, recompute readiness, return to `m.credReturnTo`.
6. `Esc` cancels — `m.profiles` unchanged.
7. After submit, an optional `[s]` key writes the profile to `~/.config/usync/credentials.toml` (XDG-respecting) using the existing `--keys-file` TOML loader. Prompts confirmation first.

Visual reference (matches research §10.4 layout):

```text
Add Credentials — Exa AI Search
================================

Exa requires the following credential:

  EXA_API_KEY
  Get key: https://dashboard.exa.ai/api-keys

> [                                              ]

[Enter] submit  [Tab] next field  [Esc] cancel  [?] help
```

### FR-10 — Footer self-documentation

Footer composition is extended with a `dropReason` slot. When a forward key is dropped because preconditions aren't met, a one-line guidance label takes its place:

| Screen | PreconditionClass | Dropped key | Guidance |
|---|---|---|---|
| ProviderReady | missing-credentials | `[Enter] select` | `Add credentials to enable [Enter] — press [k]` |
| TargetSelect | no-targets-selected | `[Enter] plan` | `Select at least one target to enable [Enter]` |
| TargetSelect | missing-credentials | `[Enter] plan` | `Credentials needed — press [k] to add` |
| TargetSelect | conflict-unresolved (focused row) | `[Enter] plan` | `Resolve conflicts first — press [r]` |
| TargetSelect | plan-error | `[Enter] plan` | `Plan failed — fix selection or press [Esc]` |
| PlanPreview | in-flight: applying | `[y] apply` | `Applying...` |
| ApplyResult | (terminal) | `[Enter]` | (none — terminal state per Pillar A FR-5) |

The mapping is data-driven (`pkg/tui/footer_guidance.go`); the explorer consumes the same table to assert the guidance label appears on the relevant state.

### FR-11 — Session recording

When `usync dashboard --record [<path>]`:

1. `DashboardModel.Update(msg)` is wrapped (when `m.recorder != nil`) to append one JSONL line per message of interest:
   - `tea.KeyMsg` → `{"t":"key","ts":<unix-ns>,"key":"<encoded>","screen":"<screen>"}`
   - Screen transitions → `{"t":"transition","ts":...,"from":"<from>","to":"<to>"}`
   - Errors → `{"t":"error","ts":...,"field":"planErr","message":"<redacted>"}`
   - Final → `{"t":"final","ts":...,"screen":"...","state_digest":"<sha256-of-redacted-View>"}`
2. Output path defaults to `artifacts/journeys/usync-<ISO8601>.jsonl`. The dashboard header shows `● rec` while active.
3. Off by default. Zero overhead when not enabled.
4. All recorded text passes `redact.Text`; a regression guard (`TestRecorder_NoRawCredentialAcrossEveryFR9Flow`) asserts no raw credential ever appears.
5. `--record` + `--keys` / `--keys-file` / `--non-interactive` is rejected with a clear error (recording is TUI-only).

### FR-12 — Session replay

`usync replay <jsonl-path>` reads the transcript and drives `DashboardModel.Update` against `pkg/uxexplore` fakes. Output is ANSI to stdout. Exit 0 iff the final-state digest matches; non-zero otherwise (the user's reported flow has not been reproduced — investigate).

Sub-flags:

- `--emit-matrix` → write a Phase-12-format matrix-row stub to stdout. The stub includes preconditions (inferred from the recorded initial state), keys (extracted from the transcript), expected (TBD — human fills in), actual (the recorded `final.state_digest`), invariants (suggested via static lookup), code-confirmation pointers (suggested via grep of recorded screen names).
- `--realtime` → honor the recorded timestamps (default: ignore).
- `--against-fixture <name>` → run against a different fixture than the one inferred. Useful for testing "the same keys against a different starting state."

### FR-13 — Explorer + recording feedback loop

The two interact intentionally:

- A `--record`ed session becomes a `*Trace` the explorer reads as a "seed path."
- The explorer extends the path: after the recording's last key, it probes every advertised key from that final state and continues normal exploration.
- A finding rooted in a recorded path is tagged `discovered-via-recording` in `findings.json`. CI is allowed to suppress these temporarily (24-hour window) to give a maintainer time to file a fix; after 24 hours, the suppression expires and CI fails.

### FR-14 — Doctor screen "next action" is always visible

Today the Doctor screen advertises `[p] providers` / `[r] rescan` / `[w] wizard` / `[?] help` / `[q] quit`. Phase 14 adds two state-conditional behaviors (validated by the explorer):

1. If `m.err != nil` (scan error), the action bar adds a one-line guidance `Scan failed — press [r] to retry or [w] for wizard`.
2. If `m.manager == nil` (smoke test or degraded mode), `[p]` is dropped and a guidance line `Manager unavailable — press [w] for wizard` replaces it.

These are the only Doctor-screen changes; everything else is downstream of the explorer's findings.

---

## 7. UX Requirements

**UX-1** TargetSelect, missing-credentials, conflict-free, eligible targets selected:

```text
Select Targets
====================

Credential profile required for Exa AI Search before planning.

> [ ] Antigravity CLI mcp-config
      /Users/.../mcp_config.json  (user)
  [ ] Claude Code user
      ...

[↑↓] navigate  [Space] toggle  [k] add credentials  [Esc] back  [q] quit
                                  Credentials needed — press [k] to add
```

**UX-2** Credential entry overlay (FR-9):

```text
Add Credentials — Exa AI Search
================================

Exa requires the following credential:

  EXA_API_KEY
  Get key: https://dashboard.exa.ai/api-keys

> [                                              ]

[Enter] submit  [Tab] next field  [Esc] cancel  [?] help
```

**UX-3** ProviderReady, selected provider missing-credentials:

```text
Provider Readiness
==================

> [missing-credentials] Exa AI Search
      get key: https://dashboard.exa.ai/api-keys
  [no-key-needed] Playwright
  ...

[↑↓] navigate  [k] add credentials  [Esc] back  [?] help  [q] quit
                Add credentials to enable [Enter] — press [k]
```

**UX-4** Header strip with recording active:

```text
[<>] MCP Config  @nawodyaishan  /  local config sync   ● rec
 1 Setup → 2 Assign → 3 Preview → 4 Results
```

**UX-5** ApplyResult terminal-state copy:

```text
Apply Result
====================

Updated (3):
  ...

Done. Press [r] to rescan or [q] to quit.

[r] rescan  [q] quit  [?] help
```

**UX-6** Doctor screen with scan error (FR-14):

```text
System Status
=============

Error scanning clients: permission denied: /Users/.../.codex/config.toml

[r] rescan  [w] wizard  [?] help  [q] quit
Scan failed — press [r] to retry or [w] for wizard
```

---

## 8. Data Model Requirements

### 8.1 `pkg/uxexplore` (new package)

```go
// pkg/uxexplore/types.go

type FixtureSpec struct {
    Name              string
    Credentials       CredentialClass
    Provider          ProviderClass
    Conflicts         ConflictClass
    Targets           TargetClass
    Workspace         bool
    ScanError         bool
    ApplyError        bool
    PreflightWarnings bool
}

type StateFingerprint struct {
    Screen            dashboardScreen
    PreconditionClass string
    BlockReason       string
    HasError          bool
    InFlight          string
}

type VisitedState struct {
    Fingerprint StateFingerprint
    ViewDigest  string  // sha256 of redacted View()
    ModelSnap   ModelSnap
}

type Edge struct {
    From   StateFingerprint
    Key    string
    To     StateFingerprint
    Caused string  // "screen-change" | "model-mutation" | "no-op" | "in-flight-cmd"
}

type Trace struct {
    Fixture FixtureSpec
    Visited []VisitedState
    Edges   []Edge
    Errors  []ExplorerError
}

type Finding struct {
    Kind          FindingKind
    Fixture       string
    Path          []string  // key sequence
    State         StateFingerprint
    Recommendation string
    MatrixID      string    // proposed DM-P<n>
}
```

### 8.2 `pkg/tui` extensions

```go
// dashboard.go — new fields
type DashboardModel struct {
    // ...existing...

    // FR-9: credential entry overlay
    credEntry    *credentialEntryState
    credReturnTo dashboardScreen

    // FR-11: recorder (nil unless --record)
    recorder *sessionRecorder
}

type credentialEntryState struct {
    providerID string
    fields     []credentialField
    cursor     int
    submitErr  error
}

type credentialField struct {
    Spec   provider.CredentialSpec
    Value  string  // never rendered raw; always redacted via "[•••• supplied]"
}

// dashboard.go — new screen enum
const (
    // ...existing...
    screenCredentialEntry dashboardScreen = iota + N
)

// footer_guidance.go (new)
type KeyOrGuidance struct {
    Key      string
    Action   string
    Reason   string  // empty if key is active; one-liner if dropped
    Disabled bool
}

var footerGuidance = map[guidanceKey]string{
    {ScreenProviderReady, PCMissingCredentials, "Enter"}: "Add credentials to enable [Enter] — press [k]",
    {ScreenTargetSelect, PCNoTargetsSelected, "Enter"}:   "Select at least one target to enable [Enter]",
    // ...
}

// recorder.go (new)
type sessionRecorder struct {
    path     string
    enc      *json.Encoder
    file     io.WriteCloser
    redactor func(string) string
}
```

### 8.3 CLI

```go
// cmd/usync/main.go
flags.StringVar(&recordPath, "record", "", "record dashboard session to JSONL (empty path → autogen)")

// cmd/usync/replay_command.go (new)
// 'usync replay <path>' with --emit-matrix, --realtime, --against-fixture flags
```

No new public types in `pkg/app`. The credential overlay reuses `provider.CredentialProfile` and `provider.CredentialSpec`.

---

## 9. Testing Requirements

| Row | Layer | File |
|---|---|---|
| **DM-P70 anchor** — credential-missing on TargetSelect routes to `[k]` overlay | teatest matrix | `pkg/tui/dashboard_flow_matrix_test.go` |
| DM-P71 — `[k]` from ProviderReady opens overlay | teatest matrix | same |
| DM-P72 — overlay Tab/Shift-Tab cycle fields | unit | `pkg/tui/credential_entry_test.go` (new) |
| DM-P73 — Enter on overlay validates required fields (calls `OfflineValidator` if present) | unit + teatest | same + matrix |
| DM-P74 — Esc on overlay restores prior screen unchanged (FR-9.6) | teatest matrix | matrix |
| DM-P75 — overlay submit adds profile + recomputes readiness | teatest matrix | matrix |
| DM-P76 — overlay `[s]` save-to-disk writes credentials.toml | teatest + fs golden | `dashboard_fake_prod_matrix_test.go` |
| DM-P77 — footer guidance label appears when key is dropped (all FR-10 rows) | table-driven unit + golden | `dashboard_view_test.go` |
| DM-P78 — help overlay surfaces dropped-key reasons | unit + golden | `dashboard_test.go` + goldens |
| DM-P79 — Doctor "[r] retry or [w] for wizard" guidance on scan error (FR-14) | unit + teatest | matrix |
| DM-P80 — `--record` writes one line per keystroke + transition + final | unit | `recorder_test.go` (new) |
| DM-P81 — recorder redaction guard (no raw credentials across every FR-9 flow) | regression | `redaction_regression_test.go` (extended) |
| DM-P82 — recorder closes cleanly on `tea.Quit` | unit | `recorder_test.go` |
| DM-P83 — `usync replay` reproduces a recorded session against fakes | integration | `replay_test.go` (new) |
| DM-P84 — `usync replay --emit-matrix` produces parseable markdown | unit | `replay_test.go` |
| DM-P85 — `--record` + `--keys` rejected | cmd test | `cmd/usync/main_test.go` |
| **UXE-01** — Explorer enumerates all `precondition_class` cells | unit | `pkg/uxexplore/enumerator_test.go` (new) |
| UXE-02 — Driver reaches every enumerated fixture's initial state | integration | `pkg/uxexplore/driver_test.go` |
| UXE-03 — Probe parses action bar correctly across all 6 screens | unit | `pkg/uxexplore/probe_test.go` |
| UXE-04 — Anomaly detector finds the credential dead-end before FR-9 lands; absence after | integration | `pkg/uxexplore/analyze_test.go` |
| UXE-05 — Coverage gates fail CI when a new precondition class lacks a fixture | CI test | `pkg/uxexplore/coverage_test.go` |
| UXE-06 — Invariant I-16 (cursor on rendered row) caught the Phase 13 anchor | regression | `pkg/uxexplore/invariants_test.go` |
| UXE-07 — Invariant I-17 (progress edge) fails on credential dead-end pre-fix | regression | same |
| UXE-08 — Recording-as-seed extends an exploration path | integration | `pkg/uxexplore/seed_test.go` |
| UXE-09 — Findings → matrix-row stubs format lints clean against `ux-flow-matrix.md` | unit | `pkg/uxexplore/report_test.go` |

---

## 10. Acceptance Criteria

| # | Criterion |
|---|---|
| **AC-1** | `make ux-explore` runs clean: `artifacts/ux-explore/findings.json` is empty (no dead-ends, no silent no-ops, no hidden-cursor states, no orphans, no repeating-error cycles) on the canonical fixture matrix. |
| AC-2 | `make ux-fake-prod` continues to run clean — Phase 13 contracts preserved. |
| AC-3 | A manual launch with no `--keys` reaches Select Targets, presses `[k]`, enters a key, presses `[Enter]`, lands on Plan Preview — no quit required. |
| AC-4 | `usync dashboard --record` produces a JSONL file with at least one `key`, one `transition`, and one `final` entry; **no raw credential appears in the file** across every UX-1..UX-6 flow. |
| AC-5 | `usync replay <file>` against fakes reproduces the recorded screen sequence and exits 0. |
| AC-6 | `usync replay --emit-matrix <file>` produces a markdown stub that satisfies the row-format lint in `pkg/uxexplore/report_test.go`. |
| AC-7 | `docs/specs/doctor-mode-phase14/ux-bug-hunt-protocol-v2.md` is the canonical protocol; the Phase 12 v1 document carries a `**Superseded by Phase 14 — see ux-bug-hunt-protocol-v2.md**` banner. |
| AC-8 | Coverage contract (FR-8) passes: every `(screen, precondition_class)` cell is either visited or excluded with a reason. |
| AC-9 | Phase 11/12/13 tests continue to pass unchanged. |
| AC-10 | UX-11 (credential dead-end) is moved from `Failing` to `Pass` in `docs/specs/doctor-mode-phase12/user-flow-audit.md`. |
| AC-11 | The Phase 12 matrix is regenerated with explorer findings — no row is hand-written that the explorer could have generated. Hand-written rows are explicitly marked `Origin: manual` (e.g. golden-only render tests). |

---

## 11. Open Questions

| OQ | Status |
|---|---|
| Where do saved credentials live? | **Proposed:** `~/.config/usync/credentials.toml` (XDG-respecting), opt-in via `[s]` after overlay submit. Reuses existing TOML loader from `--keys-file`. |
| Should the recorder capture *all* `tea.Msg` types? | **Proposed:** only `KeyMsg`+transitions+errors+final. Internal plumbing would noise the transcript. |
| Is `usync replay` a subcommand or a separate binary? | **Proposed:** subcommand of `usync` so flag/config plumbing applies. |
| Does the credential overlay support pasting (terminal bracketed-paste)? | **Proposed:** yes; `tea.PasteMsg` writes to the active field. Required for long API keys. |
| `[k]` on ApplyResult to retry with new credentials after auth failure? | **Proposed:** no. ApplyResult is terminal; user presses `[r] rescan` and re-enters flow. Revisit if explorer finds the case is common. |
| Recorder timing: capture or replay? | **Proposed:** capture timestamps; replay ignores by default; `--realtime` honors them. |
| Should `--emit-matrix` auto-suggest invariant IDs? | **Proposed:** yes, via static lookup against `(screen, precondition_class, dropped_key)`. Best-effort; human edits. |
| Explorer fixture matrix size | **Open:** start with 24–32 fixtures (pruned cartesian). Grows with each new precondition class. CI runtime budget: ≤ 30s. |
| Should the explorer run on every PR or only on PRs touching `pkg/tui`? | **Proposed:** every PR. Coverage gates are cheap; finding regressions early is the point. |
| Should the protocol v2 deprecate hand-written matrix rows entirely? | **Proposed:** no. Render tests and goldens remain hand-written (`Origin: manual` tag). Explorer covers the behavioral matrix only. |
| Where does `findings.json` live in CI? | **Proposed:** uploaded as an artifact; PR comment summarizes new findings; merge-blocking if non-empty without exclusion. |

---

## 12. Approval Status

Pending approval on:

1. **Pillar A scope** — explorer engine + coverage gates in this phase. Recommend yes; without the engine, Pillar B is one fix out of many and the next dead-end will arrive in Phase 15.
2. **Pillar B shape** — in-flow `[k]` overlay vs. routing to legacy `[w]` wizard. Recommend overlay — restores Screen 2 from research §10.4; wizard quits the dashboard mid-session.
3. **Pillar C scope** — recording + replay + `--emit-matrix` in this phase. Recommend ship together; recording without replay is half the value.
4. **Protocol v2 supersedes v1** — recommend yes; v1 remains for history with a superseded banner.
5. **Architecture review (Phase 13 A1–A12) deferred to Phase 15** — recommend explicit, time-boxed deferral.
6. **CI runtime budget** — `make ux-explore` ≤ 30s on a clean checkout. If the fixture matrix grows past that, fixtures sharded across CI workers; not a Phase 14 concern.

---

## 13. Cross-Reference Index

- Original product flow: `docs/research/Doctor Mode, Batch Plan:Apply, and Credential Validation new spec.md` §10
- Original screen 2 design: same doc §10.4
- Phase 12 bug-hunt protocol v1: `docs/specs/doctor-mode-phase12/ux-bug-hunt-protocol.md` (superseded)
- Phase 12 UX audit anchor: `docs/specs/doctor-mode-phase12/user-flow-audit.md` §UX-11
- Phase 13 matrix: `docs/specs/doctor-mode-phase13/ux-flow-matrix.md`
- Phase 13 architecture review (deferred): `docs/specs/doctor-mode-phase13/architecture-review.md`
- Protocol v2 (this phase): `docs/specs/doctor-mode-phase14/ux-bug-hunt-protocol-v2.md`
- Plan: `docs/specs/doctor-mode-phase14/plan.md`
- Tasks: `docs/specs/doctor-mode-phase14/tasks.md`
