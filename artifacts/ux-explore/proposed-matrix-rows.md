# Proposed Matrix Rows

Stubs emitted from `make ux-explore`. Fill in `Expected` and paste into the active phase's `ux-flow-matrix.md`.

## DM-P0af5d833 — silent-noop

- Fixture: `conflict-no-key`
- Preconditions: `CredentialEntry/conflict-unresolved` (block reason: conflict unresolved)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P11b4f31c — silent-noop

- Fixture: `plan-error`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P15c37e43 — silent-noop

- Fixture: `invalid-credentials-many-targets`
- Preconditions: `ProviderReady/ok`
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P164ebaca — silent-noop

- Fixture: `no-key-many-targets`
- Preconditions: `ProviderReady/ok`
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P18bff63e — silent-noop

- Fixture: `no-targets-deselected`
- Preconditions: `ProviderReady/ok`
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P2dde2986 — silent-noop

- Fixture: `apply-error-workspace`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P323917e0 — silent-noop

- Fixture: `runtime-missing`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P3804fd36 — silent-noop

- Fixture: `network-failure`
- Preconditions: `ProviderReady/network-failure` (block reason: validation error)
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P3e12e188 — silent-noop

- Fixture: `happy-path-no-key`
- Preconditions: `ProviderReady/ok`
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P3e27b589 — silent-noop

- Fixture: `runtime-missing`
- Preconditions: `ProviderReady/ok`
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P4079b214 — silent-noop

- Fixture: `happy-path-exa`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P40fb7af5 — silent-noop

- Fixture: `credential-workspace`
- Preconditions: `ProviderReady/ok`
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P44790794 — silent-noop

- Fixture: `apply-error`
- Preconditions: `ProviderReady/ok`
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P4a46eed9 — silent-noop

- Fixture: `many-targets`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P4d77de3b — silent-noop

- Fixture: `no-creds-anchor`
- Preconditions: `ProviderReady/ok`
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P4dc842b5 — silent-noop

- Fixture: `happy-path-no-key`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P50041362 — silent-noop

- Fixture: `many-conflicts`
- Preconditions: `CredentialEntry/conflict-unresolved` (block reason: conflict unresolved)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P5f281a6c — silent-noop

- Fixture: `runtime-missing`
- Preconditions: `ProviderReady/runtime-missing` (block reason: runtime missing)
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P61eb86ee — silent-noop

- Fixture: `invalid-credentials-many-targets`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P67cf9716 — invariant-violation

- Fixture: `credential-and-conflict`
- Preconditions: `TargetSelect/conflict-unresolved` (block reason: conflict unresolved)
- Actual: non-terminal state TargetSelect/conflict-unresolved has no progress edge in action bar
- Expected: _(human-filled)_
- Invariants: I-01

## DM-P6a581865 — silent-noop

- Fixture: `credential-and-conflict`
- Preconditions: `CredentialEntry/conflict-unresolved` (block reason: conflict unresolved)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P7c9e12e6 — silent-noop

- Fixture: `happy-path-exa`
- Preconditions: `ProviderReady/ok`
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P7f91f381 — silent-noop

- Fixture: `workspace-on`
- Preconditions: `ProviderReady/ok`
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P83720228 — silent-noop

- Fixture: `preflight-warning-many-targets`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P85afeff6 — silent-noop

- Fixture: `network-failure-no-key`
- Preconditions: `ProviderReady/network-failure` (block reason: validation error)
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P88504572 — silent-noop

- Fixture: `no-key-no-targets`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P8ad24c96 — silent-noop

- Fixture: `runtime-missing-with-conflict`
- Preconditions: `CredentialEntry/conflict-unresolved` (block reason: conflict unresolved)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P8c204b20 — invariant-violation

- Fixture: `runtime-missing-with-conflict`
- Preconditions: `TargetSelect/conflict-unresolved` (block reason: conflict unresolved)
- Actual: non-terminal state TargetSelect/conflict-unresolved has no progress edge in action bar
- Expected: _(human-filled)_
- Invariants: I-01

## DM-P8ca3d303 — silent-noop

- Fixture: `no-key-no-targets`
- Preconditions: `ProviderReady/ok`
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P908295c3 — silent-noop

- Fixture: `network-failure`
- Preconditions: `ProviderReady/network-failure` (block reason: validation error)
- Actual: advertised key "enter" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P91ebbc08 — silent-noop

- Fixture: `plan-error-no-key`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P9802a4b6 — silent-noop

- Fixture: `apply-error-workspace`
- Preconditions: `ProviderReady/ok`
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-P98dcce11 — invariant-violation

- Fixture: `conflict-no-key`
- Preconditions: `TargetSelect/conflict-unresolved` (block reason: conflict unresolved)
- Actual: non-terminal state TargetSelect/conflict-unresolved has no progress edge in action bar
- Expected: _(human-filled)_
- Invariants: I-01

## DM-P9ef2f421 — silent-noop

- Fixture: `credential-workspace`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pa3da293f — silent-noop

- Fixture: `network-failure-no-key`
- Preconditions: `CredentialEntry/network-failure` (block reason: validation error)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pa500d957 — silent-noop

- Fixture: `no-creds-anchor`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pab3b449b — silent-noop

- Fixture: `invalid-credentials`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pae906d8f — silent-noop

- Fixture: `preflight-warning`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pb28b4844 — silent-noop

- Fixture: `invalid-credentials`
- Preconditions: `ProviderReady/ok`
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pb39b847e — invariant-violation

- Fixture: `workspace-with-conflict`
- Preconditions: `TargetSelect/conflict-unresolved` (block reason: conflict unresolved)
- Actual: non-terminal state TargetSelect/conflict-unresolved has no progress edge in action bar
- Expected: _(human-filled)_
- Invariants: I-01

## DM-Pb612eddc — silent-noop

- Fixture: `plan-error-no-key`
- Preconditions: `ProviderReady/ok`
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pb6b470e7 — invariant-violation

- Fixture: `many-conflicts`
- Preconditions: `TargetSelect/conflict-unresolved` (block reason: conflict unresolved)
- Actual: non-terminal state TargetSelect/conflict-unresolved has no progress edge in action bar
- Expected: _(human-filled)_
- Invariants: I-01

## DM-Pc235250b — silent-noop

- Fixture: `no-targets-deselected`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pd6fe18b4 — silent-noop

- Fixture: `many-targets`
- Preconditions: `ProviderReady/ok`
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pd711ed3d — silent-noop

- Fixture: `network-failure`
- Preconditions: `CredentialEntry/network-failure` (block reason: validation error)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pd83eff83 — silent-noop

- Fixture: `network-failure-no-key`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pd9dd6759 — silent-noop

- Fixture: `preflight-warning`
- Preconditions: `ProviderReady/ok`
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pda1c2bff — silent-noop

- Fixture: `no-key-many-targets`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pda58a2b5 — invariant-violation

- Fixture: `conflict-then-resolve`
- Preconditions: `TargetSelect/conflict-unresolved` (block reason: conflict unresolved)
- Actual: non-terminal state TargetSelect/conflict-unresolved has no progress edge in action bar
- Expected: _(human-filled)_
- Invariants: I-01

## DM-Pdac2c57d — silent-noop

- Fixture: `apply-error`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pddc62740 — silent-noop

- Fixture: `network-failure`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pe47f5ab0 — silent-noop

- Fixture: `conflict-then-resolve`
- Preconditions: `CredentialEntry/conflict-unresolved` (block reason: conflict unresolved)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pe4bf14b6 — silent-noop

- Fixture: `workspace-on`
- Preconditions: `CredentialEntry/missing-credentials` (block reason: missing credentials)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pe4d9d1db — silent-noop

- Fixture: `plan-error`
- Preconditions: `ProviderReady/ok`
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pe788226d — silent-noop

- Fixture: `network-failure-no-key`
- Preconditions: `ProviderReady/network-failure` (block reason: validation error)
- Actual: advertised key "enter" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pe859ac18 — silent-noop

- Fixture: `preflight-warning-many-targets`
- Preconditions: `ProviderReady/ok`
- Actual: advertised key "v" does not change state on ProviderReady
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pe981a1c8 — silent-noop

- Fixture: `workspace-with-conflict`
- Preconditions: `CredentialEntry/conflict-unresolved` (block reason: conflict unresolved)
- Actual: advertised key "enter" does not change state on CredentialEntry
- Expected: _(human-filled)_
- Invariants: I-01, I-02

## DM-Pf585ad9c — invariant-violation

- Fixture: `runtime-missing-with-conflict`
- Preconditions: `TargetSelect/runtime-missing` (block reason: runtime missing)
- Actual: non-terminal state TargetSelect/runtime-missing has no progress edge in action bar
- Expected: _(human-filled)_
- Invariants: I-01

## DM-Pa98a32b9 — unadvertised-key

- Fixture: `keymap-audit`
- Preconditions: `ConflictResolve/`
- Actual: handler ConflictResolve accepts key "2" but no action bar advertises it
- Expected: _(human-filled)_
- Invariants: I-02

## DM-Pa13d84e3 — unadvertised-key

- Fixture: `keymap-audit`
- Preconditions: `ProviderReady/`
- Actual: handler ProviderReady accepts key "j" but no action bar advertises it
- Expected: _(human-filled)_
- Invariants: I-02

## DM-P2ed1bd5d — unadvertised-key

- Fixture: `keymap-audit`
- Preconditions: `TargetSelect/`
- Actual: handler TargetSelect accepts key "j" but no action bar advertises it
- Expected: _(human-filled)_
- Invariants: I-02

