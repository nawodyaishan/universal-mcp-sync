# Doctor Mode Phase 1 Implementation Plan

## Summary

Phase 1 adds the read-only discovery foundation for doctor mode. It should ship as two PRs:

- **PR 1a:** `pkg/manifest` static metadata and tests.
- **PR 1b:** `pkg/doctor` scanner plus `usync doctor` CLI.

This phase intentionally avoids changing existing apply behavior. `pkg/config.DetectAppConfigsForOS` remains the source used by `app.Manager` until doctor output is proven stable.

## Inputs Reviewed

- `docs/research/Doctor Mode, Batch Plan:Apply, and Credential Validation new spec.md`
- `docs/research/doctor-mode-batch-mcp-setup-research.md`
- `docs/specs/doctor-mode-phase0/spec.md`
- `docs/specs/doctor-mode-phase0/plan.md`
- `docs/specs/doctor-mode-phase0/tasks.md`
- `pkg/config/paths.go`
- `pkg/client/capabilities.go`
- `pkg/provider/types.go`
- `pkg/provider/registry.go`
- `cmd/usync/main.go`

## Key Design Corrections From The Research Spec

The research spec proposes `pkg/manifest` with no internal imports, while its sample types use `config.AppID`, `config.FileKind`, and `client.Capability`. That would violate the dependency rule and risks future import cycles.

Phase 1 should resolve this by keeping `pkg/manifest` pure:

- Client IDs are strings matching existing `config.AppID` values.
- Provider IDs are strings matching existing provider registry IDs.
- Config format is a manifest-local enum: `json`, `jsonc`, `toml`.
- Mutation shape is a manifest-local string matching current config behavior: `mcpServers`, `bareMCPServers`, `namedServer`, `codexTOML`, `claudeCodeCLI`.
- Client transport capability stays in `pkg/client` for Phase 1.
- Any future bridge from manifest to `pkg/config.AppConfig` should live in `pkg/config`, not in `pkg/manifest`.

This keeps metadata reusable by doctor, TUI, docs, and future planning without coupling it to current write implementation details.

## Architecture Approach

### 1. `pkg/manifest`

Create a new package with standard-library imports only.

Recommended files:

- `types.go`
- `clients.go`
- `providers.go`
- `runtimes.go`
- `paths.go`
- `manifest_test.go`

Core types:

```go
type ClientID string
type ProviderID string
type ConfigFormat string
type MutationKind string
type ScopeKind string
type ManagerKind string
type Confidence string

type ClientManifest struct {
    ID          ClientID
    Name        string
    Platforms   []string
    Candidates  []ConfigCandidate
    Manager     ManagerKind
    CLIName     string
    DocsURL     string
    Warnings    []ClientWarning
    Sources     []SourceRef
}

type ConfigCandidate struct {
    Label        string
    PathTemplate string
    Scope        ScopeKind
    Format       ConfigFormat
    MutationKind MutationKind
    RootKey      string
    URLField     string
    Creatable    bool
    Confidence   Confidence
    Precedence   int
    Deprecated   bool
    ReplacedBy   string
    SymlinkHint  bool
    GitWarning   bool
}
```

Provider metadata should describe user guidance, not generated configs:

```go
type ProviderMeta struct {
    ID          ProviderID
    Name        string
    DocsURL     string
    Credentials []CredentialAcquisition
    RuntimeIDs  []string
}
```

Runtime metadata:

```go
type RuntimeRequirement struct {
    ID          string
    Name        string
    Command     string
    Args        []string
    RequiredFor []string
    InstallURL  string
}
```

Helper functions:

- `AllClients() []ClientManifest`
- `ClientByID(id ClientID) (ClientManifest, bool)`
- `ForPlatform(clients []ClientManifest, goos string) []ClientManifest`
- `ExpandPath(template string, vars PathVars) (string, error)`
- `AllProviders() []ProviderMeta`
- `ProviderByID(id ProviderID) (ProviderMeta, bool)`
- `AllRuntimeRequirements() []RuntimeRequirement`

Path expansion should support:

- `{{.Home}}`
- `{{.Workspace}}`

Do not support arbitrary Go template execution in Phase 1. A small explicit replacer is safer and easier to test.

### 2. Manifest Content Scope

Start with the app IDs currently in `pkg/config.AppOrder`:

- `claude-desktop`
- `claude-code`
- `cursor`
- `vscode`
- `windsurf`
- `zed`
- `roocode`
- `opencode`
- `kiro`
- `gemini-cli`
- `antigravity-cli`
- `antigravity`
- `codex-cli`

The research spec says "12 clients", but the current repo has 13 app IDs because it separately models `antigravity-cli` and `antigravity`. Phase 1 should follow the repo, not the table count.

For Antigravity, do not hardcode final confidence until implementation begins. The current repo points Antigravity IDE to `~/.gemini/config/mcp_config.json`; the research spec claims `~/.gemini/antigravity/mcp_config.json` with symlink behavior. Phase 1 tasks must include a verification step before coding that entry.

### 3. `pkg/doctor`

Create a read-only scanner package.

Recommended files:

- `types.go`
- `doctor.go`
- `client_scan.go`
- `parse.go`
- `runtime.go`
- `providers.go`
- `report.go`
- `doctor_test.go`

Constructor:

```go
type Options struct {
    HomeDir      string
    WorkspaceDir string
    GOOS         string
    Now          time.Time
    CheckRuntime bool
    CommandTimeout time.Duration
}

type Doctor struct {
    options Options
}

func New(options Options) (*Doctor, error)
func (d *Doctor) Scan(ctx context.Context) (Report, error)
```

Report types should use manifest string IDs in Phase 1. Do not expose `config.AppID` in `pkg/doctor` unless needed later by `pkg/app`.

`Scan` behavior:

1. Load manifests for the requested platform.
2. Expand path templates.
3. `Lstat` each candidate.
4. If symlink, resolve target for reporting only.
5. If file exists, read and parse.
6. Check expected root key and object shape.
7. Detect configured provider IDs under the MCP root.
8. Check writability of existing file or creatable parent directory.
9. Compute effective candidate and confidence.
10. Generate migration hints and warnings.
11. Run runtime checks if enabled.

### 4. Parsing Strategy

JSON:

- Use `encoding/json`.
- Empty file is parse OK with an empty object only if existing behavior should create content there; otherwise report `root_key_ok=false`.

JSONC:

- For Phase 1, implement a small line-comment stripper that respects quoted strings, or add a narrowly justified parser dependency if local parsing becomes risky.
- JSONC tests must cover URLs with `https://` inside strings so comment stripping does not corrupt values.

TOML:

- Since the repo currently mutates Codex TOML using controlled string logic and has no TOML dependency, Phase 1 can start with targeted validation:
  - detect malformed obvious section headers,
  - detect `[mcp_servers.<provider>]` sections,
  - report unknown parse health as a warning if full TOML validation is deferred.
- If exact parse health is required in implementation, use a small established TOML parser and document the dependency in the PR.

### 5. CLI Integration

Extend `cmd/usync/main.go` with a narrow command dispatch before existing flags:

```text
usync doctor [--json] [--home-dir PATH] [--workspace PATH] [--no-runtimes]
```

Rules:

- Existing no-subcommand behavior still opens the TUI.
- Existing `sync` alias remains unchanged.
- Existing `--dry-run` and `--apply` behavior remains unchanged.
- `doctor --json` writes JSON to stdout and diagnostics to stderr.
- Human doctor output is redacted and concise.
- Exit code `2` means warnings/issues were found; this is not a crash.

### 6. Test Fixtures

Create fixture homes under:

```text
pkg/doctor/testdata/homes/
```

Minimum fixtures:

- `healthy_macos`
- `partial_macos`
- `malformed_json`
- `malformed_codex`
- `antigravity_conflict`
- `windsurf_legacy`
- `linux_paths`

Use `--home-dir` tests for CLI doctor. Avoid writing into the real user home.

### 7. Dependency Rules

- `pkg/manifest` imports only standard library packages.
- `pkg/doctor` may import `pkg/manifest` and `pkg/redact`.
- `pkg/doctor` must not import `pkg/app` or `pkg/tui`.
- `cmd/usync` may import `pkg/doctor`.
- `pkg/config` remains unchanged in Phase 1 unless a tiny compatibility helper is needed.

## Affected Modules

- `pkg/manifest/` new
- `pkg/doctor/` new
- `cmd/usync/main.go`
- `cmd/usync/main_test.go`
- `pkg/doctor/testdata/homes/` new
- `docs/specs/doctor-mode-phase1/` planning docs

## Dependency Changes

Default: no new external dependency.

Allowed only with explicit implementation note:

- A TOML parser if targeted Codex scanning cannot reliably report parse health.
- A JSONC parser if local comment stripping is demonstrably unsafe.

Do not add dependencies during the planning PR.

## Testing Strategy

Manifest tests:

- all repo app IDs represented
- all provider IDs represented
- all deprecated candidates have replacement or reason
- path expansion handles spaces and missing variables
- no duplicate expanded paths within one client/platform unless explicitly marked legacy/current conflict
- no internal imports

Doctor tests:

- healthy fixture returns no critical parse failures
- partial fixture reports missing candidates without failure
- malformed JSON/TOML reports parse failure without panic
- symlink fixture uses `lstat` and reports resolved path
- provider detection only reads expected root
- read-only fixture scan creates no files
- deterministic JSON output across two scans with fixed `Now`
- runtime checks can be faked or disabled for deterministic tests

CLI tests:

- `usync doctor --json --home-dir <fixture>` exits with expected code and redacted JSON
- existing `sync`, `--dry-run`, and `--apply` tests remain unchanged

## Risks and Mitigations

- **Risk:** Manifest duplicates existing `pkg/config` path data and drifts.
  **Mitigation:** Do not rewrite `pkg/config` yet; add tests comparing manifest IDs to `config.AppOrder`. Phase 2 can decide when to make config a wrapper.

- **Risk:** Antigravity path information remains unstable.
  **Mitigation:** Mark the disputed entry in tasks as requiring verification before implementation. Keep both candidate paths as current/legacy candidates if needed, with confidence levels.

- **Risk:** JSONC parsing accidentally strips URL content.
  **Mitigation:** Add tests with `https://` strings and quoted `//` content.

- **Risk:** Runtime checks slow doctor output.
  **Mitigation:** Set short timeouts and support `--no-runtimes`; default command timeout should be low.

- **Risk:** Exit code `2` breaks user expectations.
  **Mitigation:** Document it in command help and test it. It mirrors diagnostic CLI patterns where issues are distinct from crashes.

- **Risk:** Phase 1 grows into plan/apply.
  **Mitigation:** Keep doctor read-only and forbid `pkg/app` imports into `pkg/doctor`.

## Human Architecture Approval Status

Pending approval to implement.
