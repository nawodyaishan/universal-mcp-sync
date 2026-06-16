# Doctor Mode Phase 4 Implementation Plan

## Summary

Phase 4 adds credential validation as a reusable layer. It should ship as one focused PR if kept to CLI and app integration, or two PRs if live validation/cache work becomes noisy.

Recommended split:

- **PR 4a:** Validation types, offline orchestration, key-file parsing, `usync validate` offline.
- **PR 4b:** Live validators, cache, JSON output, plan/apply integration.

Keep TUI validation out of this phase unless the implementation is already trivial after the CLI API is stable.

## Inputs Reviewed

- `docs/research/Doctor Mode, Batch Plan:Apply, and Credential Validation new spec.md`
- `docs/specs/doctor-mode-phase2/spec.md`
- `docs/specs/doctor-mode-phase3/spec.md`
- `docs/specs/doctor-mode-phase3/plan.md`
- `pkg/provider/types.go`
- `pkg/provider/exa.go`
- `pkg/provider/github.go`
- `pkg/provider/context7.go`
- `pkg/provider/tavily.go`
- `pkg/app/plan_apply.go`
- `cmd/usync/apply_command.go`
- `cmd/usync/plan_commands.go`

## Key Design Corrections From The Research Spec

The research spec defines optional `OfflineValidator` and `LiveValidator` interfaces on providers. This project already has `CredentialSpec.Validator`, and several providers already implement useful offline validation through that field.

Use `CredentialSpec.Validator` for offline validation first. Add optional live validation behavior separately so offline format rules stay centralized in existing provider metadata.

The research spec also says the live cache key should use `provider_id + key_prefix_4_chars + key_last_4_chars`. This is acceptable, but Phase 4 should avoid writing even prefix/suffix as separate cache material. Use a hash of provider ID, credential key name, and redacted label. The redacted label is already allowed in plan files and user output.

## Architecture Approach

### 1. Validation Package

Add `pkg/validate` as the orchestration layer.

Suggested files:

- `pkg/validate/types.go`
- `pkg/validate/offline.go`
- `pkg/validate/live.go`
- `pkg/validate/cache.go`
- `pkg/validate/keys_file.go`

This package may import `pkg/provider`, `pkg/redact`, `pkg/exa`, `pkg/context7`, and `pkg/tavily` only if parsing reuse is necessary. Prefer using provider metadata first.

Core types:

```go
type Status string

const (
    StatusOK      Status = "ok"
    StatusWarning Status = "warning"
    StatusFailed  Status = "failed"
    StatusSkipped Status = "skipped"
)

type Mode string

const (
    ModeOffline Mode = "offline"
    ModeLive    Mode = "live"
)

type Result struct {
    ProviderID string `json:"provider_id"`
    Key        string `json:"key"`
    Label      string `json:"label"`
    Status     Status `json:"status"`
    Mode       Mode   `json:"mode"`
    Message    string `json:"message"`
    Cached     bool   `json:"cached,omitempty"`
    QuotaCost  bool   `json:"quota_cost,omitempty"`
}

type Request struct {
    Provider provider.MCPProvider
    Values   map[string]string
    Live     bool
    Now      time.Time
}
```

Keep raw credential values only in memory.

### 2. Offline Validation

Implement:

```go
func Offline(req Request) []Result
func OfflineBatch(provider provider.MCPProvider, profiles []provider.CredentialProfile) []Result
```

Rules:

- For every `RequiredCredentials` entry, check whether a value exists.
- If missing, return `failed`.
- If `Validator` is nil and value exists, return `ok`.
- If `Validator` returns an error, return `failed` with a redacted message.
- If `Validator` succeeds, return `ok`.
- For providers with no required credentials, return one `ok` or `skipped` result saying no credentials are required.
- Detect duplicate raw values within a batch and return `warning` or `failed`. Recommendation: `warning` in `validate`, `failed` in `plan` only if duplicate profiles would create ambiguous credential refs.

### 3. Key File Parser

Add provider-neutral parsing:

```go
func ParseKeyFile(data []byte) (map[string]string, error)
```

Rules:

- Ignore blank lines and `#` comments.
- Parse `KEY=value`.
- Trim surrounding whitespace.
- Support simple single or double quotes if needed.
- Return clear parse errors with line numbers.
- Never include values in parse error messages.

Do not remove the existing Exa `--keys` / `--keys-file` compatibility path in `plan` and legacy sync yet. Phase 4 can use the new parser for `validate` first, then migrate `plan/apply` carefully.

### 4. Live Validators

Add a small HTTP abstraction to avoid real network in tests:

```go
type HTTPDoer interface {
    Do(req *http.Request) (*http.Response, error)
}
```

Implement:

```go
func Live(ctx context.Context, req Request, cache Cache, httpClient HTTPDoer) []Result
```

Provider-specific behavior:

- GitHub: `GET https://api.github.com/user`, `Authorization: Bearer <token>`.
- Tavily: usage endpoint from the research spec, auth header per current Tavily API expectations.
- Exa: skipped, no supported live validator.
- Context7: skipped, no supported live validator.
- No-credential providers: skipped or ok, no live request.

Timeout handling:

- CLI creates `context.WithTimeout(ctx, 5*time.Second)`.
- Timeout and network errors return `skipped`.
- `401` and `403` return `failed`.
- `429` returns `warning` or `skipped`; recommendation: `skipped` with "rate limited".

### 5. Cache

Add:

```go
type Store struct {
    HomeDir string
    Path    string
    Now     func() time.Time
}

func DefaultCachePath(homeDir string) (string, error)
func (s Store) Load() (CredentialCache, error)
func (s Store) Save(cache CredentialCache) error
func (s Store) Get(key string) (CacheEntry, bool)
func (s Store) Put(key string, entry CacheEntry) error
```

Permissions:

- `~/.usync/cache` is `0700`.
- `credentials.json` is `0600`.

Corrupt cache handling:

- For CLI validation, corrupt cache should produce a warning and continue without cache.
- For tests, expose exact errors from lower-level load helpers.

### 6. CLI Integration

Add dispatch in `cmd/usync/main.go`:

```text
usync validate --provider exa --keys-file ./keys.env
usync validate --provider github --keys-file ./keys.env --live
usync validate --provider tavily --keys-file ./keys.env --live --json
```

Suggested file:

- `cmd/usync/validate_command.go`

Flags:

- `--provider`
- `--keys-file`
- `--keys`
- `--home-dir`
- `--live`
- `--json`

For Phase 4, `--keys` can remain Exa-oriented unless a provider-neutral `KEY=value` form is added. Prefer `--keys-file` for provider-neutral behavior.

### 7. Plan And Apply Integration

Plan integration:

- Before `manager.Prepare` or `PrepareProvider`, run offline validation.
- If any result is `failed`, exit before creating a saved plan.
- Do not run live validation from `plan`.

Apply integration:

- After apply-time credentials are loaded and before `ApplySavedPlan`, run offline validation for the saved plan provider.
- If any result is `failed`, exit before mutation.
- Do not run live validation from `apply`.

This preserves the Phase 3 guarantee that saved-plan apply gets credentials at apply time without storing secrets.

### 8. JSON Output

Human output should be compact:

```text
Credential validation
=====================
- EXA_API_KEY [ok] UUID format valid
- TAVILY_API_KEY [skipped] live validation not requested
```

JSON output should be stable and contain no raw credential values:

```json
{
  "provider_id": "tavily",
  "live": true,
  "results": []
}
```

### 9. Dependency Rules

- `pkg/validate` may import `pkg/provider` and `pkg/redact`.
- `pkg/provider` should not import `pkg/validate`.
- Provider-specific live endpoint code can live in `pkg/validate` to avoid provider packages taking HTTP dependencies.
- `cmd/usync` owns CLI flags and file loading.
- `pkg/app` may call `pkg/validate` only through small helper functions if needed. If that creates circular dependency risk, keep validation in `cmd/usync` for Phase 4 and move app integration later.
- `pkg/tui` remains unchanged in Phase 4.

## Affected Modules

- `pkg/validate/types.go` new
- `pkg/validate/offline.go` new
- `pkg/validate/live.go` new
- `pkg/validate/cache.go` new
- `pkg/validate/keys_file.go` new
- `pkg/validate/*_test.go` new
- `cmd/usync/main.go`
- `cmd/usync/validate_command.go` new
- `cmd/usync/main_test.go`
- `cmd/usync/plan_commands.go`
- `cmd/usync/apply_command.go`
- optional: `pkg/provider/types.go` only if shared validation interfaces are necessary

## Dependency Changes

No external Go dependencies are required.

## Testing Strategy

Validation package tests:

- offline success/failure for Exa, GitHub, Context7, and Tavily
- missing required credential
- provider with no required credentials
- duplicate detection
- key-file parsing with comments, blanks, and malformed lines
- live GitHub success, auth failure, timeout, network error
- live Tavily success, auth failure, timeout, network error
- cache hit, miss, expiry, permission checks, corrupt cache
- raw fake keys absent from result JSON and cache JSON

CLI tests:

- `usync validate` requires `--provider`
- unknown provider fails
- offline valid exits 0
- offline invalid exits 1
- `--json` emits valid JSON only
- `--live` uses mockable live path where possible
- `plan` rejects invalid offline credentials
- `apply --plan` rejects invalid offline credentials before writing

Regression tests:

- existing `go test ./pkg/app ./cmd/usync`
- existing `go test ./...`
- existing `make test`

## Rollout Notes

Keep the first implementation local-first:

- Offline validation is automatic and safe.
- Live validation is explicit.
- Cache is private and redacted.
- TUI integration waits until CLI behavior is stable.

## Implementation Start Gate

Do not start coding Phase 4 until `spec.md`, `plan.md`, and `tasks.md` are accepted.
