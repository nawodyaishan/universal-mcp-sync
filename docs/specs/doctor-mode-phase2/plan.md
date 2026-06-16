# Doctor Mode Phase 2 Implementation Plan

## Summary

Phase 2 introduces saved batch plans. It should ship after Phase 1b because `usync plan` should consume doctor findings rather than repeat client discovery.

The scope is deliberately narrower than the research spec: Phase 2 creates, saves, loads, lists, cleans, and displays plans. It does not apply them. Phase 3 owns `apply --plan`, approval gates, rollback from saved plans, and audit logging.

## Inputs Reviewed

- `docs/research/Doctor Mode, Batch Plan:Apply, and Credential Validation new spec.md`
- `docs/specs/doctor-mode-phase1/spec.md`
- `docs/specs/doctor-mode-phase1/plan.md`
- `docs/specs/doctor-mode-phase1/tasks.md`
- `pkg/app/app.go`
- `cmd/usync/main.go`
- `pkg/redact/redact.go`
- `pkg/manifest/*`

## Key Design Corrections From The Research Spec

The research spec proposes a gob binary plan plus JSON sidecar. For Phase 2, use a single canonical JSON plan file.

Reasons:

- JSON is inspectable, diffable, and stable for tests.
- The plan must not contain secrets, so binary opacity gives little safety benefit.
- Gob binds the file format to Go struct details and is harder to evolve.
- A schema-versioned JSON document is enough for Phase 3 apply if it stores operation metadata, file hashes, provider ID, credential refs, and generated redacted summaries.

The research spec also mixes plan generation with apply validation. Phase 2 should compute hashes and expose verification helpers, but not mutate configs or run `apply --plan`.

## Architecture Approach

### 1. Package Layout

Add saved-plan code inside `pkg/app` first:

- `pkg/app/plan_v2.go`
- `pkg/app/plan_store.go`
- `pkg/app/plan_format.go`
- `pkg/app/plan_v2_test.go`

Keep `pkg/app/app.go` stable where practical. Existing `ExecutionPlan` remains the legacy in-memory plan used by `sync --dry-run`, TUI preview, and `sync --apply`.

### 2. Core Types

Use JSON tags from the start:

```go
type SavedPlan struct {
    SchemaVersion int             `json:"schema_version"`
    PlanID        string          `json:"plan_id"`
    CreatedAt     time.Time       `json:"created_at"`
    ExpiresAt     time.Time       `json:"expires_at"`
    UsyncVersion  string          `json:"usync_version"`
    ProviderID    string          `json:"provider_id"`
    Credentials   []CredentialRef `json:"credential_refs"`
    Operations    []PlanOperation `json:"operations"`
    Warnings      []string        `json:"warnings,omitempty"`
    DoctorSummary DoctorSummary   `json:"doctor_summary"`
}

type CredentialRef struct {
    Key    string `json:"key"`
    Label  string `json:"label"`
    EnvVar string `json:"env_var"`
}

type PlanOperation struct {
    TargetID      string   `json:"target_id"`
    TargetName    string   `json:"target_name"`
    Action        string   `json:"action"`
    FilePath      string   `json:"file_path,omitempty"`
    CurrentSHA    string   `json:"current_sha,omitempty"`
    Transport     string   `json:"transport"`
    Manager       string   `json:"manager"`
    CLICommand    []string `json:"cli_command,omitempty"`
    Redacted      string   `json:"redacted"`
    IsSymlink     bool     `json:"is_symlink"`
    ResolvedPath  string   `json:"resolved_path,omitempty"`
    Warnings      []string `json:"warnings,omitempty"`
}
```

Use string constants for schema version and actions:

- `SavedPlanSchemaVersion = 1`
- `PlanActionCreate`
- `PlanActionUpdate`
- `PlanActionSkip`
- `PlanActionConflict`

### 3. Plan Generation

Recommended API:

```go
type PlanBuildOptions struct {
    DoctorReport any
    Provider provider.MCPProvider
    Profiles []provider.CredentialProfile
    Selections []CandidateSelection
    Now time.Time
    PlanID string
    UsyncVersion string
}

type CandidateSelection struct {
    TargetID string
    TargetName string
    CandidateLabel string
    Path string
    Kind string
    Manager string
    Confidence string
    IsSymlink bool
    ResolvedPath string
    Warnings []string
}
```

The `DoctorReport any` placeholder should be replaced with the concrete Phase 1b `doctor.Report` once it exists. If Phase 2 is planned before Phase 1b implementation lands, keep the docs honest and do not invent a report type here.

Generation rules:

- Build provider config using existing `provider.MCPProvider.GenerateConfig`.
- Reuse `client.Adapt` when converting provider config to target-specific shape.
- Compute `CurrentSHA` from current file bytes for file-backed operations.
- Use a sentinel value such as `missing` for absent files.
- Set `ActionCreate` when target file does not exist.
- Set `ActionUpdate` when target file exists and selected provider entry will be written.
- Set `ActionSkip` for already-current targets only if equivalence can be checked without unsafe parsing.
- Set `ActionConflict` for doctor conflicts or unresolved symlink/write-path decisions.
- Add warnings for project/workspace writes, legacy paths, symlinks, missing runtimes, and skipped targets.

### 4. Plan Persistence

Recommended API:

```go
type PlanStore struct {
    HomeDir string
    PlanDir string
    Now func() time.Time
}

func DefaultPlanDir(home string) (string, error)
func (s PlanStore) Save(plan SavedPlan, outPath string) (string, error)
func (s PlanStore) Load(path string) (SavedPlan, error)
func (s PlanStore) List() ([]PlanFile, error)
func (s PlanStore) Clean(opts CleanOptions) ([]string, error)
```

Path rules:

- `$USYNC_PLAN_DIR` wins.
- Else `$XDG_CACHE_HOME/usync/plans`.
- Else `~/.cache/usync/plans`.
- Create plan directories with `0700`.
- Write plan files with `0600`.
- Use atomic write via existing config helpers only if the helper can be used without creating `.bak` files for plan saves. Otherwise add a plan-local atomic writer.
- Do not create backup files for plan saves.

### 5. Plan IDs And File Names

Use an injectable ID generator for tests.

File name:

```text
usync-plan-<YYYYMMDD>-<plan-id-prefix>.json
```

Plan ID should be random enough for local uniqueness. If no UUID dependency exists, use `crypto/rand` and hex encoding rather than adding a dependency.

### 6. CLI Integration

Add subcommand dispatch before existing legacy flag parsing:

```text
usync plan --provider exa --targets codex-cli,vscode --keys-file ./keys.txt
usync plan --provider exa --all-detected --keys-file ./keys.txt
usync plan --provider exa --targets codex-cli --out ./plan.json
usync plan list
usync plan clean [--expired] [--all]
usync show ./plan.json
usync show ./plan.json --json
```

Rules:

- Existing no-subcommand behavior still opens the TUI.
- Existing `sync` alias remains unchanged.
- Existing legacy `--dry-run` and `--apply` remain unchanged.
- `plan` requires a provider ID.
- `plan` requires `--targets` or `--all-detected`.
- `plan` writes path to stdout on success.
- `show --json` writes only JSON to stdout.

### 7. Provider And Credential Input

Phase 2 can start with the same credential entry paths as the existing CLI:

- `--keys`
- `--keys-file`

But the saved-plan model must be provider-neutral:

- store credential refs, not values
- support env var names from manifest/provider metadata
- do not assume Exa-only fields inside `SavedPlan`

If provider-neutral CLI credential loading is too large for Phase 2, document the limitation explicitly and keep plan generation for Exa first while the model remains provider-neutral.

### 8. Redaction

Add a dedicated redaction test for saved plans.

The JSON encoder should never receive raw credential values inside `SavedPlan`. Redaction should happen before `SavedPlan` construction, not as a last-minute serialization filter.

Use `redact.Text` for generated human summaries. Consider adding `redact.PlanJSON` only if tests show raw secrets can enter nested structures.

### 9. Dependency Rules

- `pkg/app` may import `pkg/doctor` after Phase 1b.
- `pkg/app` may import `pkg/manifest`, `pkg/provider`, `pkg/client`, `pkg/config`, `pkg/redact`, and `pkg/version`.
- `pkg/doctor` must not import `pkg/app`.
- `cmd/usync` may import `pkg/app` and `pkg/doctor`.
- `pkg/manifest` remains independent.

## Affected Modules

- `pkg/app/plan_v2.go` new
- `pkg/app/plan_store.go` new
- `pkg/app/plan_format.go` new
- `pkg/app/*_test.go`
- `cmd/usync/main.go`
- `cmd/usync/main_test.go`
- optional: `pkg/redact/redact.go`
- `docs/specs/doctor-mode-phase2/` planning docs

## Dependency Changes

No new external dependency by default.

Allowed only if justified:

- UUID package, only if `crypto/rand` local ID generation is rejected.

## Testing Strategy

App tests:

- saved plan schema JSON round trip
- no credentials in saved JSON
- `0600` plan file permissions
- default plan dir resolution
- plan list and clean behavior
- missing target SHA sentinel
- stable JSON with fixed time and plan ID
- stale/expired plan detection
- schema mismatch detection

CLI tests:

- `usync plan` requires provider
- `usync plan` requires explicit target selection
- `usync plan --out` writes a readable plan path
- `usync show --json` emits stable JSON
- `usync plan list` lists cache plans
- `usync plan clean --expired` removes only expired plans
- existing legacy CLI tests still pass

Security tests:

- raw API keys are absent from plan JSON
- token-like strings are absent from human show output
- plan files are `0600`
- plan directories are `0700`

## Risks And Mitigations

- **Risk:** Saved plans accidentally contain secrets.
  **Mitigation:** Store only credential refs and redacted labels; add tests that search serialized JSON for raw test keys.

- **Risk:** Phase 2 duplicates Phase 1 doctor selection logic.
  **Mitigation:** Depend on Phase 1b report types; keep fallback DTOs small and temporary if needed.

- **Risk:** JSON plan format becomes unstable.
  **Mitigation:** Add schema version and golden-style tests with fixed time and plan ID.

- **Risk:** Plan command broadens provider credential parsing too much.
  **Mitigation:** Keep model provider-neutral but allow Exa-first CLI support if needed.

- **Risk:** Users mistake plan generation for apply.
  **Mitigation:** Human output must clearly state that no config files were written.

## Human Architecture Approval Status

Pending approval to implement.
