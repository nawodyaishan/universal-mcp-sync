# Doctor Mode Phase 14 — Implementation Plan

**Spec:** `docs/specs/doctor-mode-phase14/spec.md`
**Protocol v2:** `docs/specs/doctor-mode-phase14/ux-bug-hunt-protocol-v2.md`
**Status:** Approved for scoped PR 14e in-memory credential-entry slice; full Phase 14 plan still requires the review blockers in `review.md` to be resolved.
**Last updated:** 2026-05-25

---

## 1. Goals This Plan Realizes

From the spec (§2):

- **Pillar A** — `pkg/uxexplore/` state-space explorer + CI gates.
- **Pillar B** — `screenCredentialEntry` restoring research §10.4 Screen 2.
- **Pillar C** — `--record` flag + `usync replay` subcommand + matrix emitter.

The PR sequence below front-loads Pillar A scaffolding so that Pillar B is *validated by* the explorer (the credential dead-end finding disappears from `findings.json` once 14e lands). Pillar C lands last because recording is most valuable once findings + replay are the canonical way to file bugs.

**Scoped approval note (2026-05-25):** User approved the recommended path from `review.md`: ship an in-memory-only credential-entry slice first. This slice covers `screenCredentialEntry`, `[k]` recovery from ProviderReady and TargetSelect, existing `pkg/validate` offline validation, readiness recomputation, and DM-P70..P75/DM-P77-style tests. It explicitly excludes `[s]` save-to-disk, recorder/replay, and full `make ux-explore` gating until the architecture review blockers are addressed.

**Implementation note (2026-05-25):** The scoped slice is implemented and verified with `go test ./pkg/tui`, `USYNC_UX_MATRIX=1 go test ./pkg/tui -run TestDashboardFlowMatrix -v`, `go test ./pkg/uxexplore/...`, `make ux-explore`, and `go test ./...`. `make ux-explore` is still the scaffold target; it does not yet emit findings or coverage artifacts.

---

## 2. PR Sequence

| PR | Scope | Pillar | Blast radius | LoC est. |
|---|---|---|---|---|
| **14a** | Explorer scaffold + fixture spec + enumerator | A | new pkg only | ~400 |
| **14b** | Driver + state fingerprint + action-bar parser | A | new pkg only | ~600 |
| **14c** | Probe + invariant pass (incl. I-16, I-17) | A | new pkg + invariant refs | ~500 |
| **14d** | Anomaly detector + findings + coverage gates + CI wiring | A | new pkg + Makefile + CI | ~700 |
| **14e** | `screenCredentialEntry` (Pillar B anchor) + footer guidance (FR-10) | B | `pkg/tui` + `cmd/usync` | ~900 |
| **14f** | Doctor screen guidance (FR-14) + `[s]` save-to-disk (FR-9.7) | B | `pkg/tui` | ~250 |
| **14g** | `--record` flag + recorder + replay subcommand + matrix emitter (FR-11..FR-13) | C | `pkg/tui` + `cmd/usync` | ~800 |
| **14h** | Protocol v2 doc finalization + v1 banner + matrix migration | A | docs only | ~50 |

**Total:** ~4,200 LoC across 8 PRs over ~3–4 weeks. Each PR is independently mergeable; only 14e depends on 14a–14d for its acceptance gate.

---

## 3. PR 14a — Explorer Scaffold

### Files

| File | Change |
|---|---|
| `pkg/uxexplore/doc.go` | package doc + import-discipline comment |
| `pkg/uxexplore/types.go` | `FixtureSpec`, `CredentialClass`, `ProviderClass`, `ConflictClass`, `TargetClass`, `StateFingerprint`, `Trace`, `Edge`, `Finding`, `FindingKind` |
| `pkg/uxexplore/enumerator.go` | `EnumerateFixtures() []FixtureSpec` + 24+ canonical entries |
| `pkg/uxexplore/exclusions.go` | reads `exclusions.yaml` |
| `pkg/uxexplore/exclusions.yaml` | initial empty list (cells filled in later PRs) |
| `pkg/uxexplore/enumerator_test.go` | `TestEnumerateFixtures_CoversAllPreconditionClasses` (UXE-01) |
| `Makefile` | `make ux-explore` target (stub — runs go test; full pipeline in 14d) |

### Pseudocode

```go
// pkg/uxexplore/enumerator.go
func EnumerateFixtures() []FixtureSpec {
    return []FixtureSpec{
        {Name: "happy-path-exa",          Credentials: CredentialsValid, Provider: ProviderRequiresCreds, Targets: TargetsOne},
        {Name: "happy-path-no-key",       Credentials: CredentialsNone,  Provider: ProviderNoKey,         Targets: TargetsOne},
        {Name: "no-creds-anchor",         Credentials: CredentialsNone,  Provider: ProviderRequiresCreds, Targets: TargetsOne},
        {Name: "conflict-then-resolve",   Credentials: CredentialsValid, Conflicts: ConflictsOne,         Targets: TargetsOne},
        {Name: "credential-and-conflict", Credentials: CredentialsNone,  Provider: ProviderRequiresCreds, Conflicts: ConflictsOne},
        {Name: "scan-error",              ScanError: true},
        {Name: "apply-error",             Credentials: CredentialsValid, Provider: ProviderNoKey, ApplyError: true},
        {Name: "no-targets-deselected",   Credentials: CredentialsValid, Targets: TargetsNone},
        {Name: "workspace-on",            Credentials: CredentialsValid, Targets: TargetsMixed, Workspace: true},
        // ...continues to ~28 entries
    }
}
```

### Verification

- `go test ./pkg/uxexplore/...` passes.
- `make ux-explore` runs without errors (does nothing meaningful yet — fills in across 14b–14d).
- `TestEnumerateFixtures_CoversAllPreconditionClasses` passes: every `PreconditionClass` constant is reachable from at least one fixture.

### Risks

- Defining the canonical fixture set wrong. **Mitigation:** start with cases from the Phase 12 protocol §"Matrix Dimensions"; extend in later PRs based on findings.

---

## 4. PR 14b — Driver + Fingerprint + Action-Bar Parser

### Files

| File | Change |
|---|---|
| `pkg/uxexplore/driver.go` | `NewDriver(spec)`, `(*Driver).Run(ctx)`, `buildScanner/buildManager/buildProfiles` |
| `pkg/uxexplore/fingerprint.go` | `Fingerprint(m DashboardModel) StateFingerprint`, `classifyPreconditionClass`, `extractBlockReason` |
| `pkg/uxexplore/actionbar.go` | `parseActionBar(view string) []string`, `normalizeKey` |
| `pkg/uxexplore/fakes.go` | shared fake scanner/manager — consolidates `FakeScanner`+`FakeDashboardManager` so the explorer reuses one set of fakes (groundwork for Phase 13 architecture A7 unification, doesn't refactor existing tests yet) |
| `pkg/uxexplore/driver_test.go` | `TestDriver_ReachesInitialStatePerFixture` (UXE-02), `TestFingerprint_StableAcrossEquivalentStates` |
| `pkg/uxexplore/actionbar_test.go` | `TestParseActionBar_AllScreenVariants` (UXE-03) |
| `pkg/tui/dashboard.go` | (no changes) — explorer uses public surface only |

### Key contracts (asserted by tests)

```go
// Fingerprint is stable: same model state → same fingerprint, across map iteration orders.
// classifyPreconditionClass returns one of the 9 PC* constants for every reachable state.
// parseActionBar handles: bracketed-key + action label, ↑↓ glyphs, multi-key tokens (ctrl+c).
```

### Pseudocode

```go
// pkg/uxexplore/fingerprint.go
func Fingerprint(m DashboardModel) StateFingerprint {
    return StateFingerprint{
        Screen:            m.screen,
        PreconditionClass: classifyPreconditionClass(m),
        BlockReason:       extractBlockReason(m),
        HasError:          m.err != nil || m.planErr != nil || m.applyErr != nil || m.validErr != nil,
        InFlight:          inflightLabel(m),
    }
}

func classifyPreconditionClass(m DashboardModel) string {
    if m.err != nil { return PCScanError }
    if m.applyErr != nil { return PCApplyError }
    if m.planErr != nil { return PCPlanError }
    if hasConflictClient(m.report) && len(m.resolvedConflicts) < countConflicts(m.report) {
        return PCConflictUnresolved
    }
    if m.selectedProviderNeedsCredentials() { return PCMissingCredentials }
    if m.screen == screenTargetSelect && m.selectedTargetCount() == 0 { return PCNoTargetsSelected }
    return PCOK
}
```

### Verification

- All `UXE-02` and `UXE-03` tests pass.
- `Fingerprint` runs in < 1ms per call (benchmark).
- The action-bar parser handles every variant in the codebase — extracted via grep and table-tested.

### Risks

- `extractBlockReason` is regex-based against rendered text → fragile. **Mitigation:** the parser is paired with `TestBlockReasonExtractor_StableAcrossRefactors`, which fails if the rendered text changes without parser update.

---

## 5. PR 14c — Probe + Invariants

### Files

| File | Change |
|---|---|
| `pkg/uxexplore/probe.go` | `(*Probe).Visit(m, trace)`, visited-set dedup, advertised-key pass, unmapped-key pass, double-press pass |
| `pkg/uxexplore/invariants.go` | `Invariant` interface + I-01..I-17 implementations |
| `pkg/uxexplore/terminals.go` | terminal state set: `{ApplyResult after apply, Doctor with scan-error+rescan-attempted}` |
| `pkg/uxexplore/invariants_test.go` | per-invariant unit tests (UXE-06, UXE-07) |
| `pkg/uxexplore/probe_test.go` | `TestProbe_VisitsAllReachableStatesFromHappyPath`, `TestProbe_DetectsDoublePressOnInFlight` |

### Key contracts

```go
// I-16: cursor on rendered row.
// I-17: progress edge exists from every non-terminal state.

// Probe terminates: visited-set dedup means each (Fingerprint, ViewDigest) pair
// is processed at most once per Trace. Worst case: |states| * |advertised keys|
// updates per fixture. Bounded by the dashboard's finite state space.
```

### Pseudocode

```go
// pkg/uxexplore/invariants.go
type Invariant interface {
    ID() string
    Check(m DashboardModel) error
}

type I17_ProgressEdge struct{}

func (I17_ProgressEdge) ID() string { return "I-17" }
func (I17_ProgressEdge) Check(m DashboardModel) error {
    if isTerminal(m) { return nil }
    fp := Fingerprint(m)
    // delegated: I-17 needs the full Trace to detect, runs in analyzer (Step 4)
    // per-visit check only flags states whose action bar is empty
    keys := parseActionBar(m.View())
    if len(keys) == 0 {
        return fmt.Errorf("non-terminal state has empty action bar: %v", fp)
    }
    return nil
}
```

### Verification

- All invariant unit tests pass.
- `UXE-06` (I-16 catches Phase 13 anchor pre-fix) passes by constructing a model with `providerCursor=0` pointing at a hidden row.
- `UXE-07` (I-17 catches credential dead-end pre-fix) passes by running the explorer against the `no-creds-anchor` fixture and asserting a finding with `Kind == dead-end`.
- `TestProbe_DetectsDoublePressOnInFlight` validates Phase 13 DM-P42/P43/P44 generalization.

### Risks

- I-17 must run after the trace is complete (it needs the edge graph). **Mitigation:** I-17's per-visit check is a *weak* version (empty action bar); the full check lives in the analyzer (PR 14d).

---

## 6. PR 14d — Analyzer + Findings + Coverage + CI

### Files

| File | Change |
|---|---|
| `pkg/uxexplore/analyze.go` | `Analyze(traces []*Trace) []Finding`, Tarjan SCC, dead-end detector, silent-noop detector, orphan detector, error-cycle detector |
| `pkg/uxexplore/keymap_audit.go` | `unadvertised-key` + `unreachable-key` detectors via go/ast walk of `handleKey*` |
| `pkg/uxexplore/report.go` | `WriteFindings(dir string, findings []Finding) error` — emits 5 artifact files |
| `pkg/uxexplore/coverage.go` | `Coverage(traces) Coverage`, `(*Coverage).Gaps() []Cell` |
| `pkg/uxexplore/findings-allowlist.yaml` | initial empty allowlist |
| `pkg/uxexplore/analyze_test.go` | `UXE-04` (finds credential dead-end), `TestAnalyze_DetectsSCCErrorCycles` |
| `pkg/uxexplore/coverage_test.go` | `UXE-05` (CI fails on uncovered cell) |
| `pkg/uxexplore/report_test.go` | `UXE-09` (matrix-row stub lint) |
| `Makefile` | full `make ux-explore` pipeline: enumerate → drive → probe → analyze → report → gate |
| `.github/workflows/ux-explore.yml` (or extend existing) | run `make ux-explore` on every PR; upload artifacts; comment on PR |

### Pseudocode

```go
// pkg/uxexplore/analyze.go
func Analyze(traces []*Trace) []Finding {
    var findings []Finding
    for _, t := range traces {
        findings = append(findings, detectDeadEnds(t)...)
        findings = append(findings, detectSilentNoops(t)...)
        findings = append(findings, detectOrphans(t)...)
        findings = append(findings, detectErrorCycles(t)...)
    }
    findings = append(findings, auditKeymapVsActionBars(traces)...)
    deduplicate(findings)
    return findings
}

func detectDeadEnds(t *Trace) []Finding {
    var out []Finding
    for _, s := range t.Visited {
        if isTerminal(s.Fingerprint) { continue }
        sameClass := true
        for _, e := range outboundEdges(t, s.Fingerprint) {
            if e.To.PreconditionClass != s.Fingerprint.PreconditionClass ||
               e.To.Screen != s.Fingerprint.Screen {
                sameClass = false
                break
            }
        }
        if sameClass {
            out = append(out, Finding{Kind: FindingDeadEnd, Fixture: t.Fixture.Name, State: s.Fingerprint, ...})
        }
    }
    return out
}
```

### CI artifact layout

```text
artifacts/ux-explore/
  findings.json
  findings.md
  proposed-matrix-rows.md
  coverage.json
  graph.dot
```

PR comment posted by the workflow:

```text
ux-explore: 1 new finding (DM-PE3F1A2C dead-end at TargetSelect/missing-credentials)
            0 cleared
            Coverage: 24/24 cells (100%)
            See: artifacts/ux-explore/findings.md (workflow artifact)
```

### Verification

- `make ux-explore` exits 1 when run on `main` *before* 14e lands (because the credential dead-end is a finding).
- After 14e lands, `make ux-explore` exits 0 (the finding clears).
- `UXE-04` flips from RED (asserts finding present) to GREEN (asserts finding absent) in lock-step with 14e.
- Coverage tests fail-closed: adding a new `PreconditionClass` constant without a fixture fails CI.

### Risks

- CI runtime. **Mitigation:** budget is 30s (per spec FR-8); shard fixtures across workers if exceeded; profile in 14d landing.
- False positives on dead-ends. **Mitigation:** terminal-state set is explicit (`pkg/uxexplore/terminals.go`); reviewer adds to it as needed.

---

## 7. PR 14e — Credential Entry Overlay (Pillar B Anchor)

### Files

| File | Change |
|---|---|
| `pkg/tui/dashboard.go` | New `screenCredentialEntry` constant; `credEntry *credentialEntryState`; `credReturnTo dashboardScreen`; `handleKeyCredentialEntry`; route `[k]` from `handleKeyProviderReady` and `handleKeyTargetSelect` |
| `pkg/tui/credential_entry.go` (new) | `credentialEntryState`, `credentialField`, field-cycle helpers, submit logic |
| `pkg/tui/credential_entry_view.go` (new) | `renderCredentialEntry`, action bar |
| `pkg/tui/dashboard_view.go` | wire `screenCredentialEntry` case in `View()`; update `actionBarProviderReady` + `actionBarTargetSelect` to advertise `[k]` when `selectedProviderNeedsCredentials()` |
| `pkg/tui/footer_guidance.go` (new) | data-driven `(screen, PC, dropped_key) → guidance` table per FR-10 |
| `pkg/tui/footer_guidance_test.go` (new) | table-driven `DM-P77` — every FR-10 row produces correct guidance |
| `pkg/tui/credential_entry_test.go` (new) | `DM-P72` cycle, `DM-P73` submit-validates, `DM-P74` Esc, `DM-P75` submit-adds-profile |
| `pkg/tui/dashboard_flow_matrix_test.go` | `DM-P70` anchor test, `DM-P71` `[k]` from ProviderReady |
| `pkg/uxexplore/findings-allowlist.yaml` | (delete the temporary allowlist entry from 14d if any) |

### Pseudocode

```go
// pkg/tui/credential_entry.go
type credentialEntryState struct {
    providerID string
    fields     []credentialField
    cursor     int
    submitErr  error
}

func (m DashboardModel) handleKeyCredentialEntry(key string) (tea.Model, tea.Cmd) {
    if m.credEntry == nil {
        m.screen = m.credReturnTo
        return m, nil
    }
    switch key {
    case "esc":
        m.credEntry = nil
        m.screen = m.credReturnTo
    case "tab":
        m.credEntry.cursor = (m.credEntry.cursor + 1) % len(m.credEntry.fields)
    case "shift+tab":
        m.credEntry.cursor = (m.credEntry.cursor - 1 + len(m.credEntry.fields)) % len(m.credEntry.fields)
    case "enter":
        return m.submitCredentialEntry()
    case "backspace":
        f := &m.credEntry.fields[m.credEntry.cursor]
        if len(f.Value) > 0 { f.Value = f.Value[:len(f.Value)-1] }
    default:
        if isPrintable(key) {
            f := &m.credEntry.fields[m.credEntry.cursor]
            f.Value += key
        }
    }
    return m, nil
}

func (m DashboardModel) submitCredentialEntry() (tea.Model, tea.Cmd) {
    values := map[string]string{}
    for _, f := range m.credEntry.fields {
        if f.Value == "" {
            m.credEntry.submitErr = fmt.Errorf("%s required", f.Spec.Key)
            return m, nil
        }
        values[f.Spec.Key] = f.Value
    }
    // Optional offline validation
    if v, ok := selectedProvider(m).(provider.OfflineValidator); ok {
        if checks := v.ValidateOffline(values); hasFailed(checks) {
            m.credEntry.submitErr = firstFailedCheck(checks)
            return m, nil
        }
    }
    m.profiles = append(m.profiles, provider.CredentialProfile{
        ProviderID: m.credEntry.providerID,
        Values:     values,
        Label:      "interactive",
    })
    m.credEntry = nil
    m.screen = m.credReturnTo
    return m, m.readinessCmd()  // recompute readiness with new profile
}
```

### Route entry

```go
// pkg/tui/dashboard.go (additions to existing handlers)
func (m DashboardModel) handleKeyProviderReady(key string) (tea.Model, tea.Cmd) {
    // ...existing...
    case "k":
        if m.selectedProviderNeedsCredentials() {
            m.credReturnTo = screenProviderReady
            m.credEntry = newCredentialEntryState(m.readiness[m.providerCursor].Meta.ID)
            m.screen = screenCredentialEntry
            return m, nil
        }
    }
    // ...existing...
}
```

### Verification gate (explorer-driven)

This PR's primary verification is **the explorer**:

```text
$ make ux-explore  # before this PR
findings.json: 1 finding (DM-PE3F1A2C dead-end)
EXIT 1

$ make ux-explore  # after this PR
findings.json: empty
EXIT 0
```

The teatest cases DM-P70..P75 are secondary — they assert specific UX expectations the explorer can't (e.g. "field is rendered with masking after submit"). The dead-end test is the explorer.

### Risks

- Bracketed paste support for long API keys. **Mitigation:** capture `tea.PasteMsg` in `handleKeyCredentialEntry` — paste appends to active field as one chunk.
- Inline credential exposure during typing. **Mitigation:** masked-display contract; `redact.Text` applied to the recorder; unit test `TestCredentialEntry_NoRawValueInView`.
- Adding `screenCredentialEntry` requires updating every existing screen-aware test (help overlay, action-bar table). **Mitigation:** explorer's `unadvertised-key` and `orphan-state` detectors catch any missed wiring.

---

## 8. PR 14f — Doctor Guidance + Save-to-Disk

### Files

| File | Change |
|---|---|
| `pkg/tui/dashboard_view.go` | `actionBarDoctor` adds guidance line on `m.err != nil` and `m.manager == nil` per FR-14 |
| `pkg/tui/credential_entry.go` | post-submit `[s]` flow: prompt → write to `~/.config/usync/credentials.toml` via existing TOML loader |
| `pkg/tui/credential_entry_save.go` (new) | atomic write + 0600 permissions per safety model §12.6 |
| `pkg/tui/dashboard_view_test.go` | `DM-P79` Doctor guidance |
| `pkg/tui/dashboard_fake_prod_matrix_test.go` | `DM-P76` `[s]` save-to-disk fs golden |

### Pseudocode

```go
// pkg/tui/dashboard_view.go
func actionBarDoctor(hasErr bool, hasManager bool) string {
    bar := "[r] rescan  [w] wizard  [?] help  [q] quit"
    if !hasManager {
        return "[r] rescan  [w] wizard  [?] help  [q] quit\nManager unavailable — press [w] for wizard"
    }
    if !hasErr {
        return "[p] providers  " + bar
    }
    return bar + "\nScan failed — press [r] to retry or [w] for wizard"
}
```

### Verification

- `DM-P76`, `DM-P79` pass.
- `make ux-explore` still passes (Doctor guidance is FR-14, validated by explorer for the `scan-error` fixture).
- Saved file is `0600`, parent dir is `0700` (per safety §12.6).

---

## 9. PR 14g — Recording + Replay + Matrix Emitter

### Files

| File | Change |
|---|---|
| `pkg/tui/recorder.go` (new) | `sessionRecorder`, JSONL writer, redaction pipeline |
| `pkg/tui/recorder_wrapper.go` (new) | wraps `Update(msg)` to capture and forward |
| `pkg/tui/dashboard.go` | `m.recorder` field; `Update` checks `m.recorder != nil` and forwards |
| `cmd/usync/main.go` | `--record [<path>]` flag; reject `+ --keys` / `+ --keys-file` / `+ --non-interactive` |
| `cmd/usync/replay_command.go` (new) | `usync replay <jsonl>` subcommand; `--emit-matrix`, `--realtime`, `--against-fixture` |
| `pkg/tui/recorder_test.go` (new) | `DM-P80` writes-per-keystroke, `DM-P82` clean-close |
| `pkg/tui/replay_test.go` (new) | `DM-P83` replay reproduces, `DM-P84` matrix emitter |
| `pkg/tui/redaction_regression_test.go` | extend with `DM-P81` recorder-redaction-guard |
| `cmd/usync/main_test.go` | `DM-P85` flag-conflict rejection |
| `pkg/uxexplore/seed.go` | `(*Explorer).RunWithSeed(seed []recordEntry)` per FR-13 |
| `pkg/uxexplore/seed_test.go` | `UXE-08` seeded exploration extends recorded path |

### Pseudocode

```go
// pkg/tui/recorder.go
type sessionRecorder struct {
    path     string
    enc      *json.Encoder
    file     *os.File
    redactor func(string) string
}

func newSessionRecorder(path string) (*sessionRecorder, error) {
    if path == "" {
        path = filepath.Join("artifacts", "journeys", fmt.Sprintf("usync-%s.jsonl", time.Now().UTC().Format("20060102T150405Z")))
    }
    if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil { return nil, err }
    f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
    if err != nil { return nil, err }
    return &sessionRecorder{path: path, enc: json.NewEncoder(f), file: f, redactor: redact.Text}, nil
}

func (r *sessionRecorder) Record(e recordEntry) {
    e.Message = r.redactor(e.Message)  // ALWAYS redact
    _ = r.enc.Encode(e)
}

func (r *sessionRecorder) Close() error { return r.file.Close() }
```

### Verification

- `DM-P80..P85` pass.
- `UXE-08` passes — explorer extends from a recording.
- Redaction regression: `TestRecorder_NoRawCredentialAcrossEveryFR9Flow` runs every UX-1..UX-6 flow with a UUID-style credential, asserts the UUID never appears in the JSONL output.

### Risks

- File I/O in hot path. **Mitigation:** recorder uses buffered writes; flush only on screen transitions + tea.Quit.
- Replay determinism. **Mitigation:** fakes are seeded; final-state digest is the contract; if digest mismatches, replay exits non-zero (this is a *feature* — it means the recorded behavior has changed).

---

## 10. PR 14h — Documentation Finalization

### Files

| File | Change |
|---|---|
| `docs/specs/doctor-mode-phase12/ux-bug-hunt-protocol.md` | add superseded banner at top |
| `docs/specs/doctor-mode-phase12/user-flow-audit.md` | flip UX-11 status from `Failing` to `Pass`; add Phase 14 PR reference |
| `docs/specs/doctor-mode-phase14/ux-flow-matrix.md` (new) | start the Phase 14 matrix; populate from explorer-generated stubs (initially: zero rows, because explorer is clean) |
| `docs/specs/doctor-mode-phase13/ux-flow-matrix.md` | mark Phase 13 rows DM-P40..P69 as `Origin: manual` (explorer doesn't generate goldens) |
| `Makefile` | update `make help` with `ux-explore` target description |
| `CLAUDE.md` (or `docs/contributing/test-runners.md`) | explain new testing hierarchy: unit → matrix → explorer → fake-prod |

### Verification

- Manual: every cross-reference between docs resolves.
- `make help` shows `ux-explore: run state-space explorer + coverage gates`.

---

## 11. Verification Gates Per PR

Every PR must pass before merge:

```text
1. NO_COLOR=1 TERM=xterm-256color go test ./...
2. make ux-fake-prod  (for PRs touching pkg/tui)
3. make ux-explore    (for PRs in 14a–14h; expected to PASS after 14e)
4. Goldens re-recorded if renderer changed.
5. CI workflow artifacts uploaded (findings.json, coverage.json).
```

---

## 12. Risks Per Phase

| Risk | Mitigation | Phase |
|---|---|---|
| Explorer non-determinism breaks CI | Seed everything; byte-hash `findings.json`; CI re-runs on mismatch | 14d |
| Adding `screenCredentialEntry` breaks Phase 13 goldens | Re-record goldens in 14e commit | 14e |
| `[k]` binding conflicts with future Phase 15 work | Reserve `k` namespace explicitly in `keymap_audit.go` | 14c |
| Recorder leaks credentials | Always-on redaction + `DM-P81` regression test | 14g |
| Recorder slows dashboard interaction | Buffered writes; benchmark with `BenchmarkUpdate_WithRecorder_Overhead < 5%` | 14g |
| Explorer fixture matrix grows beyond 30s budget | Shard across CI workers; per-fixture parallel `go test` | 14d, 14g |
| The credential overlay introduces *new* dead-ends | Explorer's `findings.json` catches them on 14e merge | 14e |
| Action-bar parser fails on a new screen renderer | `parseActionBar` fails-closed → orphan-state finding → CI blocks | 14b, 14e |

---

## 13. Out-Of-Band Notes

- This plan **does not** address Phase 13 architecture A1–A12. Those land in Phase 15. The only A4 mitigation that touches Phase 14 is the explorer's use of `context.Background()` inside its driver closures — same defect Phase 13 review noted. Phase 14 fixes this only inside `pkg/uxexplore/` (a clean-room package); existing production sites stay until Phase 15.
- The Phase 13 PR sequence ended at 13f. There is no 13g. Phase 14 starts at 14a.
- Plan is **revisable**. If 14a–14d find that the explorer is harder than estimated, 14e (the user-impacting credential fix) is allowed to land independently and the explorer slips to 14f-onwards. **Do not block the user-impacting fix on the engine.** The engine is the long-term win; the fix is the next-week win.

---

## 14. Order Of Operations Justification

Why **14a → 14d before 14e**:

- The explorer provides the *acceptance gate* for 14e. Without the explorer, "is the credential dead-end fixed?" is a manual judgment. With the explorer, it's a CI signal.
- 14a–14d are pure additions (new package). Zero risk of breaking Phase 11/12/13 tests.
- 14e is the first PR that touches `pkg/tui/dashboard.go` in Phase 14. Landing it after 14a–14d means the explorer guards the change.

Why **14e before 14g**:

- Recording is most valuable once there's a stable target to record against. Recording a buggy dead-end produces a transcript that's a bug report; recording a fixed flow produces a regression seed.
- Recording the credential dead-end (pre-fix) is the canonical demo of FR-13 (recording-as-seed). It works best when the fix exists in the next commit.

**Escape hatch:** if user pressure on the credential dead-end is high (week-1 priority), 14e ships in parallel with 14a (the scaffold), and 14b–14d play catch-up. The explorer will retroactively validate 14e once it lands.
