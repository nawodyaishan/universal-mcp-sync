# Doctor Mode Phase 14 UX Flow Matrix

**Source protocol:** `docs/specs/doctor-mode-phase14/ux-bug-hunt-protocol-v2.md`
**Explorer command:** `make ux-explore`
**Status:** Bootstrap matrix for the scoped Phase 14 credential-entry slice.

Phase 14 rows are generated from the explorer once the full probe/analyzer/report pipeline lands. The current in-repo `make ux-explore` target runs the `pkg/uxexplore` scaffold tests only; it does not yet emit `artifacts/ux-explore/proposed-matrix-rows.md`.

## Row Intake

New rows should come from `artifacts/ux-explore/proposed-matrix-rows.md` after a full explorer run. Until the analyzer/report writer is complete, scoped credential-entry coverage remains locked in Go tests:

- `TestDashboardFlowMatrix_CredentialDeadEndOffersRecovery`
- `TestCredentialEntry_KFromProviderReadyOpensOverlay`
- `TestCredentialEntry_EnterValidatesRequired`
- `TestCredentialEntry_SubmitAddsProfileAndMasksView`
- `TestCredentialEntry_EscRestoresPriorScreenUnchanged`
- `TestCredentialEntry_TabCyclesFields`
- `TestFooterGuidanceRows`

## Case Index

| ID | Family | Case | Preconditions | Keys | Expected | Invariants | Automation | Origin | Status |
|---|---|---|---|---|---|---|---|---|---|

