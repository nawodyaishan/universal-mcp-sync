# Documentation Index

Start here when looking for product, design, or process documentation.

## Top-level

- [`specification.md`](specification.md) — canonical product specification.

## Subdirectories

| Path | Purpose |
|---|---|
| [`architecture/`](architecture/) | System design docs (sync engine, scalability). |
| [`contributors/`](contributors/) | How-to guides for maintainers: adding a provider, test runners, E2E workflow, dogfooding. |
| [`research/`](research/) | External research outputs feeding into specs. Not authoritative; specs are. |
| [`roadmap/`](roadmap/) | Cross-cutting plans that span multiple phases or features (3-month plan, QA strategy). |
| [`specs/`](specs/) | Phase-by-phase feature specifications. Each subdir has `spec.md` / `plan.md` / `tasks.md`. |

## Specs layout

`docs/specs/` contains both individual feature specs and per-phase directories.

| Directory | Status |
|---|---|
| `doctor-mode-phase0` … `phase14` | Phased work on the doctor → plan → apply pipeline. Phase 14 (UX explorer + recorder/replay) is the most recent. |
| `e2e-testing/`, `e2e-testing-phase2/` | E2E test harness specs. |
| `linux-support/` | Linux platform support spec. |
| `providers/` | One-off provider integration specs (Context7, Kubernetes, Playwright, Tavily, Terraform, CodeGraph). |
| `architecture-upgrade-plan.md` | Cross-cutting architecture refactor plan. |
| `doctor-mode-remaining-implementation-plan.md` | Status sweep of leftover doctor-mode work. |
| `month-1-plan.md`, `3-month-roadmap.md`, `qa-usability-plan.md` | Historical planning docs. |

## How to find something

- **"What's the contract for X?"** → start in `specification.md` or the matching `specs/<phase>/spec.md`.
- **"How do I add a provider?"** → `contributors/adding-a-provider.md`.
- **"How do I record + replay a bug?"** → `contributors/e2e-testing-workflow.md`.
- **"What's the architecture?"** → `architecture/` plus the active phase's `plan.md`.
- **"Why was this decision made?"** → look for `review.md` or `gap-analysis.md` inside the phase directory.

## House style

- Each phase directory uses the `spec.md` / `plan.md` / `tasks.md` triad. Some also carry `review.md`, `gap-analysis.md`, or `ux-flow-matrix.md` when relevant.
- Status banners at the top of a doc supersede the body. The Phase 12 protocol carries a banner pointing to its v2 in Phase 14.
- Dates in plans are absolute (ISO-style) — relative dates rot fast.
