# E2E Testing Workflow with Claude Code

A practical guide for end-to-end bug-hunting and regression-coverage on `usync`, using the recorder + replay + explorer engine added in Phase 14.

**Audience:** maintainer driving a Claude Code session against this repo. Assumes you can run `make record`, `make replay`, `make ux-explore` locally.

## Overview

Two loops, sequenced.

| Loop | When | Evidence source | Best for |
|---|---|---|---|
| **A — Manual-driven** | You hit a bug in real use | A `--record` transcript + your prose description | One-off UX bugs, transcript-reproducible issues |
| **B — Automated baseline sweep** | Weekly hygiene, before allowlist expiry | `make ux-explore` findings + `proposed-matrix-rows.md` | Systematic bug-class elimination |

Both share the same shape: `record → analyze → fix → verify`. They differ in where the evidence comes from.

---

## Loop A — Manual-driven

Use this when you've experienced a bug yourself and want it root-caused.

### A.1 Capture evidence

```bash
make record                              # interactive, drive the failing flow, press q
ls -t artifacts/journeys/                # latest transcript on top
```

Note in plain English:
- **What you tried** (key sequence)
- **What broke** (error text, wrong screen)
- **What you expected**

Don't speculate about root cause. The transcript is your reproducer; your prose is your bug statement.

### A.2 Hand off to Claude Code

Open a **fresh** Claude Code session — do not reuse one with prior context. Paste exactly three things:

1. **Transcript path** — `artifacts/journeys/usync-<ts>.jsonl`
2. **Symptom** — 2–3 sentences, no cause speculation
3. **Expected contract** — one sentence

End with this instruction verbatim:

> Reproduce by replaying the transcript against the matching fixture. Write a regression test that fails without the fix. Then implement the fix. Don't refactor surrounding code.

This forces the agent to read the actual code path, not pattern-match on what looks similar.

#### Template prompt

```text
Bug: <transcript path>, line <N> shows <observed behavior>.

Symptom: <2-3 sentence factual description, no speculation>.

Expected: <one-sentence contract>.

Reproduce by replaying the transcript against the <fixture-name> fixture
(see EnumerateFixtures() in pkg/uxexplore/enumerator.go).
Add a teatest or unit regression that fails without the fix.
Then implement the fix. Don't refactor surrounding code.
```

### A.3 Verify

```bash
make replay TRANSCRIPT=artifacts/journeys/usync-<ts>.jsonl FIXTURE=happy-path-exa
go test ./pkg/tui ./pkg/uxexplore ./cmd/usync
make ux-explore
make record                              # re-record same flow; should now succeed
```

You're done when:
- `make replay` exits 0 (digest match if transcript had one)
- New regression test fails on `git stash` of the fix
- Re-recorded transcript reaches the success state

Commit on its own line. Reference the original transcript path in the message.

---

## Loop B — Automated baseline sweep

Use this after Loop A is quiet, or weekly, to keep `findings.json` near empty.

### B.1 Pull current findings

```bash
go run ./cmd/ux-explore --emit-allowlist > /tmp/proposed-allowlist.yaml
diff pkg/uxexplore/findings-allowlist.yaml /tmp/proposed-allowlist.yaml
cat artifacts/ux-explore/proposed-matrix-rows.md
```

`proposed-matrix-rows.md` gives a markdown stub per finding with `MatrixID`, `Recommendation`, fixture, screen, PC.

### B.2 Triage one finding at a time

For each finding stub, open a fresh Claude Code session:

```text
Finding DM-Pxxxx:
<paste the entire stub from proposed-matrix-rows.md>

Classify this finding as ONE of:
  (a) genuine product bug — the code is wrong
  (b) detector noise — the analyzer should be tuned
  (c) intended behavior — code is correct, document with a comment

Investigate the relevant code path. Don't fix anything yet — just classify
and explain in 5 sentences max.
```

After Claude classifies (don't take its first answer for granted; ask follow-ups if the reasoning seems thin), in the same session:

```text
Implement the fix you proposed. Add the regression test. Re-run make ux-explore.
Report new finding count and whether DM-Pxxxx is gone.
```

Working one finding at a time keeps diffs reviewable. **Resist batching.** The MatrixID is stable across runs, so resuming next session is cheap.

### B.3 Detector tuning (occasional)

If you classify three findings in a row as "detector noise of the same kind", that's a signal — tune the analyzer instead:

```text
The last three findings were classified as detector noise of the same kind:
<describe>. Propose a detector refinement in pkg/uxexplore/analyze.go
(heuristic change, additional silent-noop filter, or new exclusion category)
that eliminates all three without masking real bugs.
Add a test that locks in the new heuristic.
```

---

## Operational tips

| Pitfall | Mitigation |
|---|---|
| Transcript leaks credentials | Already fixed (Phase 14g): paste events become `<paste>`, `Key`/`Message`/`BlockReason` go through `redact.Text`. Sanity check: `grep -E '[0-9a-f]{8}-[0-9a-f]{4}' artifacts/journeys/*.jsonl` should return nothing. |
| Agent over-refactors when fixing a UX bug | Always end prompts with: "Don't refactor surrounding code. Don't add abstractions. The smallest diff that fixes the bug AND adds a regression test." |
| Multiple findings conflated in one diff | One finding per Claude session. Resist batching even when "they look related". |
| `make ux-explore` produces new findings after a fix | Expected — explorer doing its job. Add via `--emit-allowlist` if tangential; fix inline if related to what you just touched. |
| Goldens drift on action-bar edits | `go test ./pkg/tui -update` regenerates. Diff before committing to verify changes are intentional. |
| You forget to re-record after fix | Put transcript path in commit message: `Recorded verification: artifacts/journeys/usync-<ts>.jsonl`. Future you can replay it. |
| Agent reads stale memory of previous fix | Start with a fresh Claude Code session per bug. Memory is the enemy of clean reasoning here. |

---

## Concrete first move

The plan-error-after-conflict-resolve bug from the 2026-05-25 transcript is the highest-signal known issue. Suggested kickoff prompt for Loop A:

```text
Bug: artifacts/journeys/usync-20260525T132652Z.jsonl line 8 is a plan-error
after pressing Enter on TargetSelect. Lines 9-10 attempt recovery: open
ConflictResolve via [r], pick [1], return to TargetSelect, press Enter again.
Line 14 shows plan-error again despite the conflict being resolved.

Symptom: resolved-conflict state isn't propagating to the planner in
some code path.

Expected: after resolving a conflict via [r] → [1] → return → Enter on
TargetSelect, the plan should succeed (or at least surface a different
error from before).

Reproduce by replaying the transcript against the happy-path-exa fixture
extended with the conflict client (see matrixConflictClient() in
pkg/tui/dashboard_flow_matrix_test.go). Add a teatest regression that
fails without the fix. Then implement the fix. Don't refactor surrounding
code.
```

---

## Reference: tools used

| Command | Purpose |
|---|---|
| `make record [RECORD_PATH=…]` | Launch TUI with session recorder; writes to `artifacts/journeys/usync-<ts>.jsonl` (0600) |
| `make replay [TRANSCRIPT=…] [FIXTURE=…] [EMIT_MATRIX=1]` | Replay transcript against a uxexplore fixture; exits 1 on digest mismatch |
| `make ux-explore` | Full pipeline: enumerate → drive → probe → analyze → audit → report → gate. Exits 1 on unallowlisted findings or coverage gaps. |
| `go run ./cmd/ux-explore --emit-allowlist` | Print current findings as allowlist entries, ready to redirect into `findings-allowlist.yaml` |
| `go test ./pkg/tui -update` | Regenerate `.golden` files after intentional UI changes |
| `go test ./...` | Full unit + integration suite |

## Reference: artifacts

| Path | What it contains |
|---|---|
| `artifacts/journeys/usync-<ts>.jsonl` | Recorded session transcripts. 0600. Redacted at write time. |
| `artifacts/ux-explore/findings.json` | All analyzer findings keyed by MatrixID, sorted |
| `artifacts/ux-explore/findings.md` | Same findings as a human-readable table |
| `artifacts/ux-explore/proposed-matrix-rows.md` | One stub per finding with MatrixID, suggested invariants — paste into `ux-flow-matrix.md` after triage |
| `artifacts/ux-explore/coverage.json` | Reached `(Screen, PC)` cells + remaining gaps |
| `artifacts/ux-explore/graph.dot` | State graph from probe traces — render with `dot -Tpng` for visualization |
| `pkg/uxexplore/findings-allowlist.yaml` | MatrixID + `expires_at` entries that silence findings until the date passes |

## Reference: documents

- Protocol v2 (canonical): `docs/specs/doctor-mode-phase14/ux-bug-hunt-protocol-v2.md`
- Phase 14 spec: `docs/specs/doctor-mode-phase14/spec.md`
- Phase 14 plan: `docs/specs/doctor-mode-phase14/plan.md`
- Phase 14 tasks (completion gates): `docs/specs/doctor-mode-phase14/tasks.md`
- Test runners overview: `docs/contributors/test-runners.md`
