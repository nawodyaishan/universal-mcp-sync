# Doctor Mode Phase 14 — Architecture Review

**Reviewed:** 2026-05-25  
**Reviewer:** Codex  
**Status:** Needs changes for full Phase 14; scoped in-memory credential-entry slice approved by user on 2026-05-25

## Reviewed Artifacts

- `docs/specs/doctor-mode-phase14/spec.md`
- `docs/specs/doctor-mode-phase14/plan.md`
- `docs/specs/doctor-mode-phase14/tasks.md`
- `docs/specs/doctor-mode-phase14/ux-bug-hunt-protocol-v2.md`
- `docs/specs/doctor-mode-phase13/architecture-review.md`
- `docs/research/Doctor Mode, Batch Plan:Apply, and Credential Validation new spec.md`
- `AGENTS.md`
- Current `pkg/tui` dashboard implementation
- Current provider credential and validation contracts
- Current `Makefile`

## Decision

Phase 14 is not approved for implementation as currently written.

The product goal is sound: the credential dead-end is a real UX defect, and restoring in-flow credential entry is aligned with the original research. The explorer and recorder are reasonable long-term investments. However, the current plan has package-boundary contradictions and unresolved high-risk approval items that would force code generation to guess.

## Blocking Findings

### B1 — `pkg/uxexplore` cannot implement the specified fingerprint and invariant contract against current `pkg/tui`

Severity: High

The plan places the explorer in a new package, `pkg/uxexplore`, but the required APIs and pseudocode depend on unexported TUI internals:

- `dashboardScreen`
- `DashboardModel.screen`
- `DashboardModel.err`, `planErr`, `applyErr`, `validErr`
- `DashboardModel.scanning`, `validating`, `planning`, `applying`
- `DashboardModel.providerCursor`, `clientCursor`
- `DashboardModel.selectedProviderNeedsCredentials()`

Go package boundaries prevent `pkg/uxexplore` from reading these values. The plan must choose one of these before implementation:

- expose a narrow exported dashboard snapshot API from `pkg/tui`;
- move explorer internals under `pkg/tui` or an `internal` package with explicit access rules;
- reduce the explorer to rendered-view parsing only, and update FR-3/I-16/I-17 accordingly.

Recommended fix: add a small exported `tui.DashboardSnapshot` and `DashboardModel.Snapshot()` contract. Keep mutation through `Init`, `Update`, and `View`; expose only the read-only state the explorer needs.

### B2 — `provider.OfflineValidator` does not exist

Severity: High

FR-9 and PR 14e pseudocode call `provider.OfflineValidator`, but the current provider package exposes field-level `CredentialSpec.Validator` and package-level validation helpers in `pkg/validate`. There is no `provider.OfflineValidator` interface.

Recommended fix: revise credential-entry submission to use the existing validation path, preferably `validate.MissingRequiredCredentials` or `validate.OfflineProfiles`, and define exactly how warnings versus failures are handled.

### B3 — Credential persistence is high-risk and not explicitly approved

Severity: High

FR-9.7/PR 14f writes credentials to disk. This touches secrets and local file permissions, so it requires explicit approval under the SDD high-risk gate. The spec also leaves unresolved details:

- exact XDG path resolution versus hard-coded `~/.config/usync/credentials.toml`;
- whether multi-provider and multi-profile files merge or overwrite;
- rollback behavior if write, chmod, or TOML update fails;
- whether credential file content is ever rendered, recorded, or included in goldens.

Recommended fix: split `[s]` save-to-disk into its own approved sub-plan. Keep PR 14e in-memory only unless the user explicitly approves persistent credential writes.

### B4 — Phase status and approval items are still pending

Severity: High

Both `spec.md` and `plan.md` are marked `Draft — awaiting approval`, and `spec.md` lists six pending approval items in section 12. Implementation should not start until those are resolved or explicitly deferred.

Recommended fix: record an approval decision for Pillar A, Pillar B, Pillar C, protocol v2, Phase 13 architecture deferral, and CI runtime budget. A narrower approval for 14a or for an in-memory-only 14e slice is acceptable if documented.

### B5 — CI gating order is internally inconsistent before 14e

Severity: Medium

The plan says `make ux-explore` should run on every PR, but also says it must fail before 14e because the credential dead-end is expected. The allowlist strategy is mentioned later, but the merge rules and expected pass/fail state per PR are not clear enough for implementation.

Recommended fix: define per-PR gate semantics:

- 14a-14c: package tests only, no full findings gate.
- 14d: full gate allowed to pass only with a temporary, expiring credential-dead-end allowlist.
- 14e+: allowlist removed; full gate must pass.

### B6 — Replay and explorer fakes need an ownership decision

Severity: Medium

The plan creates fakes in `pkg/uxexplore` and then has `cmd/usync replay` depend on them. That is workable only if `pkg/uxexplore` remains a production package with stable fake-builder contracts. It also risks making test harness concepts part of CLI behavior.

Recommended fix: define whether replay fakes are production test fixtures, `internal` fixtures, or a dedicated replay fixture package. Avoid importing ad hoc test-only patterns into command code.

## Non-Blocking Suggestions

- Prioritize a narrow in-memory credential-entry slice if user impact is urgent. The plan already allows 14e to ship before the full explorer; make that explicit as an approved slice.
- Keep recorder implementation after redaction contracts are in place. The recorder is valuable, but it expands the secret-handling surface.
- Use existing `validate` package behavior for credential checks instead of adding a second validation abstraction.
- Add the snapshot API contract to the task list before PR 14b if the explorer remains outside `pkg/tui`.

## Required Changes Before Approval

1. Resolve package access for explorer fingerprinting and invariants.
2. Replace `provider.OfflineValidator` with the existing validation contract or define a new provider interface explicitly.
3. Obtain explicit approval for any credential persistence, or defer `[s]` save-to-disk out of the first implementation slice.
4. Update `spec.md` and `plan.md` approval status, or add a scoped approval note for the first implementation slice.
5. Clarify `make ux-explore` pass/fail behavior per PR.
6. Define ownership and import boundaries for replay fakes.

## Approved Slice Candidate

The following narrower slice can be approved after the changes above are reflected in the plan:

- Implement in-memory `screenCredentialEntry`.
- Route `[k]` from ProviderReady and TargetSelect when credentials are missing.
- Validate using existing `pkg/validate` helpers.
- Keep saved credentials and recorder out of scope.
- Add DM-P70..P75 and DM-P77 tests.
- Use manual verification instead of the full explorer until PR 14d exists.

This slice addresses the user-facing dead-end without taking on the unresolved persistence and recorder risks.
