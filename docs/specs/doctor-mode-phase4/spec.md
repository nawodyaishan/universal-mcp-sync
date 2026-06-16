# Doctor Mode Phase 4: Credential Validation

## Problem Statement

Phase 2 and Phase 3 added saved plans and plan-file apply, but credentials are still mostly validated only when provider config generation runs. That makes bad keys show up late in the workflow and gives users little guidance about whether a credential is missing, malformed, duplicated, runtime-dependent, or live-verified.

Phase 4 adds a first-class credential validation layer for the `doctor -> validate -> plan -> apply` workflow. It should support offline validation for all credentialed providers, opt-in live validation only where a quota-safe endpoint exists, and a private cache for live results. It must not store raw credentials in saved plans, validation cache, logs, audit records, or user-facing output.

## Goals

- Add provider-level validation result types and optional live validation interfaces.
- Reuse existing `CredentialSpec.Validator` functions as the offline validation source of truth.
- Add a validation orchestration package that can validate one provider profile or a batch of profiles.
- Add live validation for GitHub and Tavily only.
- Do not ship live validation for Exa or Context7 because the research spec found no confirmed quota-safe endpoint.
- Add a private 24-hour live validation cache at `~/.usync/cache/credentials.json`.
- Add `usync validate --provider <id>` with human output and optional JSON output.
- Add `--live` as explicit opt-in; offline validation remains the default.
- Integrate offline validation into `usync plan` and saved-plan apply credential resolution so malformed credentials fail before config generation.
- Keep all output redacted by default.

## Non-Goals

- Do not add TUI credential validation UI in the first Phase 4 implementation PR.
- Do not perform live validation automatically during plan or apply.
- Do not validate Exa live.
- Do not validate Context7 live.
- Do not store raw keys, key hashes based on full raw keys, URLs containing secrets, or command args containing secrets.
- Do not change saved plan schema for this phase unless a future implementation finds an unavoidable compatibility issue.
- Do not add network calls to normal test runs; live validator tests must use mock HTTP clients or local test servers.

## Users or Actors

- CLI users who want to check keys before generating a plan.
- New users who need actionable "missing/malformed/get key here" status before setup.
- Automation that wants JSON validation output before running `usync plan` or `usync apply --plan`.
- Future TUI screens that need a provider-neutral validation API.

## Functional Requirements

- **FR-1:** Add validation statuses: `ok`, `warning`, `failed`, and `skipped`.
- **FR-2:** Add validation result fields for provider ID, credential key, redacted label, status, message, live/offline mode, cache status, and quota-cost flag.
- **FR-3:** Offline validation must run without network access.
- **FR-4:** Offline validation must use `CredentialSpec.Validator` where present.
- **FR-5:** Offline validation must report missing required credentials.
- **FR-6:** Offline validation must detect duplicate credentials in one provider batch when the same raw value appears more than once.
- **FR-7:** Offline validation must support Exa multi-key input by using the provider's `MultiValueParser` behavior or equivalent parsing.
- **FR-8:** Live validation must be opt-in with `--live`.
- **FR-9:** Live validation must be implemented for GitHub using `GET https://api.github.com/user`.
- **FR-10:** Live validation must be implemented for Tavily using the quota-safe usage endpoint from the research spec.
- **FR-11:** Live validation must run with a 5 second timeout.
- **FR-12:** Live timeout must return `skipped` with a redacted timeout message, not panic.
- **FR-13:** Network failures during live validation must return `skipped`, not exit code 1, when offline validation passed.
- **FR-14:** HTTP 200 from live validation must return `ok`.
- **FR-15:** HTTP 401 or 403 from live validation must return `failed`.
- **FR-16:** Unsupported live validation must return `skipped` with a clear message.
- **FR-17:** Cache entries must expire after 24 hours.
- **FR-18:** Cache files must be written with `0600`; cache parent directory must be `0700`.
- **FR-19:** Cache keys must not contain raw credentials.
- **FR-20:** `usync validate --provider <id> --keys-file <path>` must validate credentials from a file.
- **FR-21:** `usync validate --provider <id> --keys <value>` may be supported for Exa compatibility, but provider-neutral key-file parsing should be the preferred path.
- **FR-22:** `usync validate --provider <id> --json` must print machine-readable validation results only.
- **FR-23:** Human output must include get-key URLs or setup hints when credentials are missing, if those URLs are available from manifest/provider metadata.
- **FR-24:** `usync plan` must fail early on offline validation failures.
- **FR-25:** `usync apply --plan` must fail early on offline validation failures for supplied apply-time credentials.

## Credential Input Format

For provider-neutral validation, `--keys-file` should accept simple `KEY=value` lines.

Examples:

```text
EXA_API_KEY=11111111-1111-1111-1111-111111111111
GITHUB_PERSONAL_ACCESS_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
TAVILY_API_KEY=tvly-xxxxxxxxxxxxxxxxxxxx
CONTEXT7_API_KEY=ctx7sk-xxxxxxxxxxxxxxxxxxxx
```

Blank lines and `#` comments should be ignored. Quoted values may be supported if the existing key parser already supports them. Raw credential values must not be echoed back.

## Provider Validation Matrix

| Provider | Offline Validation | Live Validation | Notes |
|---|---|---|---|
| Exa | UUID parse via existing Exa parser | Not supported | Return live `skipped`; no confirmed safe endpoint. |
| GitHub | PAT format via existing validator | `GET https://api.github.com/user` | Uses `Authorization: Bearer <token>`. |
| Context7 | `ctx7sk-` / `ctx7sk_` parser via existing validator | Not supported | Return live `skipped`; no confirmed safe endpoint. |
| Tavily | `tvly-` parser via existing validator | Usage endpoint | Must use mock HTTP in tests. |
| Playwright | No credential required | Runtime checks deferred to doctor/runtime validation | Validation result may be `skipped` or `ok` with "no credentials required". |
| Kubernetes | No credential required | Runtime checks deferred to doctor/runtime validation | Validation result may be `skipped` or `ok` with "no credentials required". |
| Terraform | No required credential today | Runtime checks deferred to doctor/runtime validation | Optional `TFE_TOKEN` can be added later. |

## Cache Rules

Cache path:

```text
~/.usync/cache/credentials.json
```

Cache entry shape:

```go
type CredentialCache struct {
    Entries map[string]CacheEntry `json:"entries"`
}

type CacheEntry struct {
    Status     ValidationStatus `json:"status"`
    Message    string           `json:"message"`
    CachedAt   time.Time        `json:"cached_at"`
    ExpiresAt  time.Time        `json:"expires_at"`
    ProviderID string           `json:"provider_id"`
    KeyLabel   string           `json:"key_label"`
}
```

Cache key recommendation:

```text
sha256(provider_id + ":" + credential_key + ":" + redacted_label)
```

This follows the research spec's intent: key identity is stable enough for local UX, but the full raw key is never written to disk and never used as cache material.

## CLI Behavior

Commands:

```text
usync validate --provider exa --keys-file ./keys.env
usync validate --provider tavily --keys-file ./keys.env --live
usync validate --provider github --keys-file ./keys.env --live --json
```

Exit codes:

- `0`: validation completed and no `failed` results.
- `1`: invalid flags, unsupported provider, unreadable key file, or at least one offline/live `failed` result.
- `2`: reserved for future detailed-exitcode behavior; do not use in Phase 4 unless already required by shared CLI helpers.

Live network failure handling:

- If offline validation passes and live validation fails due to timeout/network, command exits `0` with `skipped` live result.
- If live endpoint returns auth failure, command exits `1` with `failed` live result.

## Acceptance Criteria

- Offline validation for Exa, GitHub, Context7, and Tavily is unit-tested with valid and invalid examples.
- Duplicate credential detection is unit-tested.
- Live GitHub validator is tested with a mock server or injected HTTP client.
- Live Tavily validator is tested with a mock server or injected HTTP client.
- Timeout returns `skipped` and does not hang tests.
- Cache hit avoids a live request inside the 24-hour TTL.
- Cache expiration triggers a new live request.
- Cache file and parent directory permissions are verified.
- `usync validate` works for offline validation.
- `usync validate --live` uses cache and mockable live validators.
- `usync validate --json` emits stable JSON with no raw credentials.
- `usync plan` rejects offline-invalid credentials before plan creation.
- `usync apply --plan` rejects offline-invalid apply-time credentials before mutation.
- `go test ./pkg/provider ./pkg/validate ./pkg/app ./cmd/usync` passes.
- `go test ./...` and `make test` pass before implementation is marked complete.

## Success Criteria

- Users can diagnose credential readiness before planning or applying.
- Phase 5 TUI work can call validation APIs without duplicating provider-specific logic.
- Plan/apply remains safer because malformed credentials fail before mutation code runs.
- Live validation is useful but never surprising; all network calls require explicit `--live`.

## Edge Cases

- Required credential missing.
- Unknown provider ID.
- Credential key exists but belongs to another provider.
- Key file has duplicate values.
- Key file has malformed lines.
- Provider has no required credentials.
- Provider supports offline validation but not live validation.
- Live endpoint times out.
- Live endpoint returns 401 or 403.
- Live endpoint returns 429.
- Cache file is missing.
- Cache file is corrupt.
- Cache file has permissive permissions.
- Cache directory cannot be created.
- `--json` output must stay valid when results include warnings or skipped statuses.

## Data Sensitivity and Compliance Notes

- Validation result messages must never include raw credential values.
- Cache keys must not use raw credential values directly.
- Cache entries must not contain raw credential values.
- Test fixtures must use obvious fake keys and assert those fake values are absent from output/cache.
- Live validator HTTP logs must not print headers.
- CLI errors must pass through redaction.

## Assumptions

- Phase 3 saved-plan apply is present.
- Existing provider `RequiredCredentials` metadata remains the source of required keys and offline validators.
- Provider-neutral key-file parsing can be implemented without changing TUI input immediately.
- Runtime validation for Node, Docker, kubectl, and CLIs remains doctor/runtime scope, not credential validation scope.

## Open Questions

- Should `usync validate` support provider-neutral `--env` mode that reads required keys directly from environment variables?
- Should cache permissive permissions be an error or should the tool repair permissions automatically? Recommendation: repair if owner-writable, otherwise error.
- Should live validation result messages include account metadata, such as GitHub username? Recommendation: defer until redaction and privacy expectations are explicit.
- Should `TFE_TOKEN` become an optional Terraform credential in this phase? Recommendation: defer.

## Human Approval Status

Approved to plan. Implementation approval pending.
