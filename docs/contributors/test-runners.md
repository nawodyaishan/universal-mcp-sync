# Test Runner Hierarchy

Use the narrowest runner that covers the risk, then move outward before merging.

## Unit And Package Tests

`go test ./pkg/<package>` is the default loop while editing one package. For dashboard work, prefer:

```sh
NO_COLOR=1 TERM=xterm-256color go test ./pkg/tui
```

## Matrix Tests

The dashboard matrix locks screen, key, and precondition behavior from the Phase 12 and Phase 13 UX matrices:

```sh
USYNC_UX_MATRIX=1 NO_COLOR=1 TERM=xterm-256color go test ./pkg/tui -run TestDashboardFlowMatrix -v
```

Use this whenever action bars, navigation, selection state, or recovery flows change.

## Explorer

`make ux-explore` is the Phase 14 state-space explorer entrypoint. In the current scoped implementation it runs the `pkg/uxexplore` scaffold tests. The full protocol v2 workflow will later enumerate fixtures, drive the dashboard, analyze findings, and write `artifacts/ux-explore/`.

Protocol reference: `docs/specs/doctor-mode-phase14/ux-bug-hunt-protocol-v2.md`.

## Fake Production

`make ux-fake-prod` runs container-backed dashboard scenarios against fake production config roots. Use it when changing apply behavior, target discovery, rollback, config file writes, or shell/PTY-facing behavior.

## Full Suite

Before handing off a Phase 14 dashboard change, run:

```sh
NO_COLOR=1 TERM=xterm-256color go test ./...
make ux-explore
```

Run `make ux-fake-prod` when the change touches file mutation or fake-production UX flows.

