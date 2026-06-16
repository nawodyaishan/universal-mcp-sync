# Phase 11 Requirements Checklist

**Spec:** `docs/specs/doctor-mode-phase11/spec.md`  
**Last reviewed:** 2026-05-23

---

## Quality Gates

| Check | Status | Notes |
|---|---|---|
| No implementation details in spec | ✅ | Data model shows types/signatures, not algorithms |
| Goals and non-goals clear | ✅ | 13 goals; 10 explicit non-goals including Gemini, Windows, MCP Server Mode |
| Requirements testable and unambiguous | ✅ | Each FR maps to ≥1 test row in Testing Requirements table |
| Acceptance criteria complete | ✅ | 16 AC entries covering all FRs |
| Success criteria measurable | ✅ | All AC are binary pass/fail or exact output assertions |
| Edge cases identified | ✅ | 11 edge cases documented with expected behaviour |
| Data sensitivity noted | ✅ | `_usync` markers, `ContentHash`, audit rotation, managed-settings message scope |
| Open questions resolved or deferred | ✅ | All 6 OQs resolved |
| Spec ready for planning | ✅ | Plan exists at `plan.md`; plan tightening is next step |

---

## FR Coverage Map

| FR | Description | AC | Test row(s) |
|---|---|---|---|
| FR-1 | Audit log rotation at 5 MB | AC-5 | Rotation triggers; .1 backup; failure handled |
| FR-2 | Skip-on-identical apply | AC-6, AC-10 | skipped flag; SkippedTargets; no backup |
| FR-3 | Plan content integrity hash | AC-7 | ContentHash present; mismatch blocks preflight; empty skips |
| FR-4 | `_usync` markers in JSON | AC-8 | Present; absent for Antigravity; idempotent |
| FR-5 | `# managed-by=usync` in Codex TOML | AC-9 | Present; not duplicated |
| FR-6 | Codex CLI adapter | — | Args for stdio; args for HTTP; graceful skip |
| FR-7 | No-stdout-from-library guard | AC-2 | Guard test passes |
| FR-8 | Redaction regression suite | AC-11, AC-16 | All 7 surfaces × 4 patterns |
| FR-9 | TUI test hardening | AC-4, AC-12 | Color profile; helpers; golden 5 screens; NO_COLOR CI |
| FR-10 | VS Code sandbox preservation | AC-1 | sandboxEnabled + sandbox survive UpdateNamedServerJSON |
| FR-11 | Claude Code managed-settings warning | AC-13 | Warning in doctor report |
| FR-12 | `SourceRef.Confidence` field | AC-14 | Non-empty on all manifest entries |
| FR-13 | Reference-verification checklist | — | Artifact review only |

---

## Scope Boundary Verification

- [x] Gemini CLI references: none in spec
- [x] pkg/migrate references: none in spec  
- [x] Windows: listed in non-goals
- [x] Per-project Codex: listed in non-goals (FR-6 explicitly user-scope only)
- [x] usync reading /etc/claude-code/: listed in non-goals (FR-11 uses os.Stat only)
- [x] Credential storage: not mentioned (not in scope)
- [x] Network calls in tests: AC-15 explicitly prohibits
