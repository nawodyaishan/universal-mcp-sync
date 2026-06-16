# Architecture Review — Doctor Mode Phase 8 (Revision 2)

**Decision: NEEDS CHANGES — one remaining blocking finding**  
**Reviewer:** Architecture gate (automated)  
**Date:** 2026-05-23  
**Previous review:** 2026-05-23 (Revision 1 — B1–B4 identified)

---

## Reviewed Artifacts

- `docs/specs/doctor-mode-phase8/plan.md` (revised, 2026-05-23)
- `pkg/app/plan_apply.go` — `buildOperationFromSavedPlan` (lines 278–352 read in full)
- `pkg/app/plan_v2.go` — `buildPlanOperation`, `PlanOperation`, `SavedPlanOptions`
- `pkg/config/json_update.go` — import list confirmed; no `pkg/app` import
- `pkg/app/plan_apply.go` — `AutoApprove bool` on `SavedPlanApplyOptions` confirmed

---

## Prior Findings Status

| Finding | Status |
|---|---|
| B1 — callsite updates in wrong task | ✅ Resolved — Task 6 updates callsites in same task as constructor change |
| B3 — validate.Service not injected | ✅ Resolved — `Validate` on `DashboardManager`; `dashboardManagerAdapter` in `cmd/usync` |
| B4 — `PreflightSavedPlan` blocking `Update` | ✅ Resolved — `preflightCmd()` + `preflightResultMsg` in Task 7 |
| N1 — alwaysApprover redundant | ✅ Resolved — `AutoApprove: true` used; confirmed field exists at `plan_apply.go:42` |
| N3 — provider registry access | ✅ Resolved — `provider.DefaultRegistry().Get(id)` is public |
| Circular import concern | ✅ Not a concern — `pkg/config` imports no `pkg/app`; `config.VSCodeInput` defined in `pkg/config`; `pkg/app → pkg/config` is the existing direction |
| **B2 — VS Code apply path** | ⚠️ **Partially resolved — one sub-gap remains (see below)** |

---

## Remaining Blocking Finding

### B2-A — Phase A header substitution does not survive to apply time

**Severity:** Blocking  
**File:** `pkg/app/plan_v2.go` Task 2 description; `pkg/app/plan_apply.go` missing change

**Root cause confirmed by reading `buildOperationFromSavedPlan` (plan_apply.go:278–352):**

The apply path always regenerates credentials from scratch:

```
ApplySavedPlan
  → prepareSavedPlan
    → buildOperationFromSavedPlan(plan, planOp, opts.Credentials)
        → prov.GenerateConfig(realCredentials)   // ← always real creds
        → op.Config = client.Adapt(appID, cfg)   // ← real headers
    → prepareFileOperation(opForWrite)
        → config.UpdateNamedServerJSON(..., op.Config, ...)  // ← real headers written to disk
```

The plan's Phase A description ("Replace credential header values with `${input:…}` literals in the `MCPConfig` *before* the plan is stored") modifies `op.Config.Headers` inside `buildPlanOperation` (called from `BuildSavedPlan`). However, `PlanOperation` does **not** store `MCPConfig` — it stores only metadata (`VSCodeInputs`, `Redacted`, `Transport`, `FilePath`, etc.). At apply time, `buildOperationFromSavedPlan` regenerates `op.Config` via `prov.GenerateConfig(realCredentials)`, discarding any substitution done during plan creation.

**Result without this fix:** VS Code config files will contain real credential values in headers (e.g. `"Authorization": "Bearer sk-real-key"`) even when `UseInputVariables: true`. `${input:id}` literals are never written. FR-17 and AC-6 are violated.

**Required fix:** Add header substitution in `buildOperationFromSavedPlan` (`pkg/app/plan_apply.go`), **after** `op.Config = client.Adapt(appID, cfg)` in the `PlanManagerFile` case:

```go
// When the plan records VSCodeInputs, replace credential header values
// with ${input:id} references so VS Code resolves them via its input
// variable mechanism rather than receiving raw secrets.
if len(planOp.VSCodeInputs) > 0 && op.Config.Type != provider.TransportStdio {
    subHeaders := make(map[string]string, len(op.Config.Headers))
    for k := range op.Config.Headers {
        subHeaders[k] = "${input:" + planOp.VSCodeInputs[0].ID + "}"
    }
    op.Config.Headers = subHeaders
}
```

**Task change:** Add this to Task 3 (which already modifies `plan_apply.go`) or as Task 3b. Remove the incorrect Phase A description from Task 2 (the `buildPlanOperation` modification of `op.Config.Headers` is a no-op for the apply path; it may be kept for the `Redacted` field update only).

**Test required:** `TestBuildOperationFromSavedPlan_VSCodeHeaderSubstituted` — assert that when `planOp.VSCodeInputs` is non-empty, the returned `op.Config.Headers` contains `${input:id}` values, not the raw credential string.

---

## Non-Blocking Findings (from this revision)

### N5 — `DashboardManager.HomeDir()` is a method; `app.Manager.HomeDir` is a field

`DashboardManager` declares `HomeDir() string` (method). `app.Manager` has `HomeDir string` (field). Embedding `*app.Manager` in `dashboardManagerAdapter` does **not** auto-promote a field to a method. The adapter needs an explicit implementation:

```go
func (a dashboardManagerAdapter) HomeDir() string { return a.Manager.HomeDir }
```

This is a one-line addition at implementation time. Non-blocking for approval; implementation note for Task 11.

### N6 — Assumption 5 references old Task numbering

Assumption 5 says "updated in **Task 4** (not Task 10)" but the task that fixes callsites is now Task 6. The body of Task 6 is correct. Non-blocking stale wording.

### N7 — `MergeVSCodeInputs` map iteration order is non-deterministic

Task 1's `MergeVSCodeInputs` implementation iterates over `existing map[string]VSCodeInput{}` when writing the merged slice. Go map iteration is unordered, so the `inputs` array order changes between runs. This matters for golden file tests but not for correctness. Recommended: maintain insertion order by using an ordered structure or sorting by `ID`. Non-blocking; note for implementer.

---

## Checklist Summary

| Item | Status |
|---|---|
| Scope clear | ✅ |
| All prior open questions resolved or explicitly deferred | ✅ |
| Follows existing architecture (Bubble Tea, saved-plan APIs) | ✅ |
| No new dependencies | ✅ |
| Data model changes backward-compatible | ✅ |
| Circular import: `pkg/config` ↛ `pkg/app` | ✅ Confirmed clean |
| Sensitive data: raw credentials not written to VS Code config | ❌ B2-A — credential still written at apply time |
| Auth/authorization boundaries explicit | ✅ |
| `AutoApprove` only after user confirmation | ✅ |
| All I/O in `tea.Cmd` closures (Bubble Tea contract) | ✅ (`preflightCmd` added) |
| Failure modes documented | ✅ |
| Rollback documented | ✅ |
| Tests cover core security boundary (header substitution) | ❌ Test for B2-A not in plan |
| Phase 7 regression handled in correct task | ✅ Task 6 |

---

## Approval Conditions

**Tasks 1–6:** Approved to proceed.

**Tasks 7–12:** Blocked until B2-A is resolved.

To resolve B2-A, update `plan.md` with:

1. Add to Task 3 (or a new Task 3b): modify `buildOperationFromSavedPlan` in `pkg/app/plan_apply.go` to substitute `${input:id}` for credential header values when `len(planOp.VSCodeInputs) > 0`.
2. Add test `TestBuildOperationFromSavedPlan_VSCodeHeaderSubstituted` to the task 4 test list.
3. Correct or clarify Task 2's Phase A note — the `op.Config.Headers` modification in `buildPlanOperation` only affects the `Redacted` field, not the apply path. The apply-time substitution happens in `buildOperationFromSavedPlan`.

Once those three changes are reflected in the plan, the plan may be approved and tasks may be generated.
