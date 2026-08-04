# Adding an MCP Provider

This guide walks through adding a new MCP server provider to `usync` without
hard-coding provider-specific behaviour into the TUI or apply flow.

It was last updated after adding CodeGraph (v1.3.8) — the first fully-local,
no-credential stdio provider — which revealed several layers the earlier
Context7-focused guide did not cover.

---

## 1. Provider Shape

Before you begin, determine the shape of the provider:

- **Transport**: `stdio`, `http`, `sse`, or `streamable-http`.
- **Runtime**: For `stdio`, decide whether the command is installed by `npm`,
  `pypi`, `oci`, `mcpb`, or manually (raw binary on `PATH`).
- **Authentication**: URL query parameters (Exa), HTTP headers (Context7),
  environment variables (GitHub), or none (Playwright, Kubernetes, CodeGraph).
- **Credentials**: Single credential, multiple independent profiles, a
  multi-key paste field, or no credentials at all.
- **Client support**: Which clients support the transport natively, need a
  bridge, or must be skipped?

Do this from the provider and client documentation. Do not infer a transport
shape from one client if the provider publishes an official configuration format.

---

## 2. Layers to touch — complete checklist

Every provider touches **six distinct layers**. Missing any one will cause test
failures or silent runtime gaps.

| # | Layer | Files |
|---|---|---|
| A | Provider implementation | `pkg/provider/<name>.go` |
| B | Provider registry | `pkg/provider/registry.go`, `pkg/provider/registry_test.go` |
| C | Dedicated provider test | `pkg/provider/<name>_test.go` |
| D | Manifest ID constant | `pkg/manifest/types.go` |
| E | Manifest provider data | `pkg/manifest/providers.go` |
| F | Manifest order test | `pkg/manifest/manifest_test.go` |
| G | QA scenario | `pkg/app/qa_scenarios_test.go` |
| H | E2E golden files | `tests/e2e/testdata/provider_<name>/` |
| I | README | `README.md` (table + mermaid diagram) |
| J | Provider spec | `docs/specs/providers/add-<name>-provider.md` |

> [!IMPORTANT]
> Layers D–F (the manifest) are easy to overlook for no-credential providers
> because they don't affect `GenerateConfig`. However the manifest drives the
> `usync providers` CLI command, Doctor Mode runtime checks, and the TUI
> readiness panel. A provider not in the manifest will show no runtime status
> and no docs URL in the UI.

---

## 3. Step-by-step walkthrough

### A — Implement the provider

Create `pkg/provider/<name>.go` implementing the `MCPProvider` interface:

```go
type MCPProvider interface {
    ID() string
    Name() string
    Description() string
    RequiredCredentials() []CredentialSpec
    GenerateConfig(credentials map[string]string) (MCPConfig, error)
}
```

**For no-credential stdio providers** (like Playwright, Kubernetes, CodeGraph):

```go
func (p *MyProvider) RequiredCredentials() []CredentialSpec { return nil }

func (p *MyProvider) GenerateConfig(_ map[string]string) (MCPConfig, error) {
    return MCPConfig{
        Type:    TransportStdio,
        Command: "npx",
        Args:    []string{"-y", "@publisher/package", "subcommand"},
        Runtime: &PackageRuntime{Type: "npm"},
    }, nil
}
```

**Keep secrets in `Env`** whenever the upstream server supports it.
Avoid putting tokens in `Args` — command arguments are easier to leak
through process listings, logs, and failed-command reports.

**Use `BridgeOverride`** only when a provider needs a bespoke bridge that the
client capability matrix cannot express (e.g. Context7 wraps a StreamableHTTP
server behind an npx stdio bridge for clients that do not support that transport).

### B — Register the provider

In `pkg/provider/registry.go`:

```go
r.register(NewMyProvider())
```

In `pkg/provider/registry_test.go`:
- Increment the count assertion (`7 → 8`, etc.).
- Add an `all[N].ID() == "<id>"` assertion for the new position.

### C — Dedicated provider test file

Create `pkg/provider/<name>_test.go` — **one file per provider** following the
`playwright_test.go` pattern, not inlined in `registry_test.go`.

Assert all of the following:
- `ID()`, `Name()`, `Description()` return expected values.
- `RequiredCredentials()` has the correct length (0 for no-credential providers).
- `GenerateConfig()` returns the right `Type`, `Command`, `Args`, `Env`,
  `Headers`, `URL`, and `Runtime`.
- For no-credential providers: `Env` is empty/nil, `Headers` is nil, `URL` is empty.

### D — Manifest ID constant

In `pkg/manifest/types.go`, add a named constant to the `ProviderID` block:

```go
const (
    ProviderExa       ProviderID = "exa"
    // ...
    ProviderMyProvider ProviderID = "myprovider"
)
```

This is required even if the provider has no credentials — the constant is
used by `ProviderByID()`, the TUI readiness panel, and the CLI `providers`
command.

### E — Manifest provider data

In `pkg/manifest/providers.go`, add a `ProviderMeta` entry to `allProviders`:

```go
{
    ID:      ProviderMyProvider,
    Name:    "My Provider",
    DocsURL: "https://example.com/docs",
    // RuntimeIDs drives Doctor Mode runtime checks.
    // Include every binary the provider needs on PATH.
    // Omit entirely for providers with no runtime dependency (e.g. remote HTTP).
    RuntimeIDs: []string{"node", "npx"},
    Sources: []SourceRef{
        {URL: "https://...", Title: "...", VerifiedAt: "YYYY-MM-DD", Confidence: "official"},
    },
},
```

For providers **with credentials**, add a `Credentials` slice:

```go
Credentials: []CredentialAcquisition{
    {
        Key:          "MY_API_KEY",
        EnvVar:       "MY_API_KEY",
        Required:     true,
        FormatHint:   "Format description",
        OfflineRegex: `^prefix-[A-Za-z0-9]+$`,
        GetURL:       "https://example.com/api-keys",
        DocsURL:      "https://example.com/docs",
    },
},
```

### F — Manifest order test

In `pkg/manifest/manifest_test.go`, add your provider ID to the `want` slice
in `TestAllProvidersIncludesSupportedProviderIDsInOrder`. The order must match
the order in `allProviders`.

### G — QA scenario

In `pkg/app/qa_scenarios_test.go`, add a `TestQA<Name>AllClients` function.

For a no-credential stdio provider, follow the `TestQAPlaywrightAllClients`
or `TestQACodeGraphAllClients` pattern:

1. Scaffold all client config files in a temp dir.
2. Call `PrepareProvider` with a `Default` profile (no credential values).
3. Assert stdio-skip warnings for Antigravity CLI and IDE.
4. Inspect `plan.Operations` to verify Claude Code CLI args:
   ```text
   mcp add -s user <id> -- <command> <args...>
   ```
5. Apply the plan and verify per-client output:
   - Claude Desktop / Cursor: correct package name in JSON.
   - VS Code: no `"type": "http"` for stdio providers.
   - Roo Code: `"type": "stdio"`.
   - OpenCode: `"type": "local"`.
   - Codex CLI: `[mcp_servers.<id>]` TOML section.
   - Antigravity CLI + IDE: untouched (`{}`).

### H — E2E golden files

Generate golden files after the provider is working:

```bash
go test ./tests/e2e -update -run TestProviders_Golden/<id>
```

Review the generated files in `tests/e2e/testdata/provider_<id>/` before
committing — they are ground truth for regression detection across all client
targets.

### I — README

In `README.md`:

1. Add a row to the **Supported MCPs** table:
   ```markdown
   | **My Provider** | Brief capability description | Transport | Auth | Status |
   ```
2. Add a node and edges to the **mermaid architecture diagram** in the
   `providerLayer` subgraph and the flow section below it.

### J — Provider spec document

Create `docs/specs/providers/add-<name>-provider.md` and add it to
`docs/specs/providers/README.md`. Use any existing spec as a template.
The spec should document:
- Why the provider was added and what it does.
- The transport + credential decision and why.
- The `MCPConfig` shape.
- The dependency graph of phases/tasks.
- Acceptance criteria.

---

## 4. Transport variants

### URL-auth (Exa)

See `pkg/provider/exa.go`. The API key is appended to the URL query string via
`pkg/exa/url.go`. Tests must assert both the generated URL and the redacted
output. Never print the full URL when it contains secrets.

### Header-auth with bridge (Context7)

See `pkg/provider/context7.go`. Uses `TransportStreamableHTTP` + a
`BridgeOverride` so clients that do not support StreamableHTTP natively
get an npx stdio proxy. The `CONTEXT7_API_KEY` header is injected via
`pkg/client/adapter.go`.

### Environment-variable auth (GitHub, Tavily)

See `pkg/provider/github.go`. The token is kept in `Env` on the `MCPConfig`
and written as an `env` block in client config files. For Codex TOML this
becomes an `env = { ... }` inline table.

### No-credential stdio (Playwright, Kubernetes, CodeGraph)

See `pkg/provider/playwright.go` or `pkg/provider/codegraph.go`. Return nil
from `RequiredCredentials()`. The TUI creates one `Default` profile
automatically. No `Env`, `Headers`, or `URL` are set.

> [!TIP]
> When the provider launches via `npx`, include `-y` in the args to avoid
> interactive prompts during first-time package downloads. Check whether the
> package needs a subcommand (e.g. CodeGraph uses `mcp` to start the MCP
> server rather than the interactive CLI).

---

## 5. Required tests — per layer

| Layer | What to cover |
|---|---|
| Provider | ID, name, description, credential count, config type/command/args/env/headers/URL/runtime |
| Registry | Count, insertion order |
| Manifest | ID constant reachable via `ProviderByID`, `AllProviders()` order |
| Redaction | No raw credentials, secret URLs, headers, env values, or bridge args in user-facing output |
| Client adaptation | Native transport passthrough, bridge conversion, injected headers, unsupported-transport skips |
| Config writers | JSON/TOML root keys, special URL field names, headers, env vars, file permissions |
| QA scenarios | End-to-end plans for all representative clients |
| E2E golden | Golden file per client target under `tests/e2e/testdata/provider_<id>/` |

---

## 6. Verification commands

Run in this order before opening a PR:

```bash
make fmt
make vet
make build
make test
make verify
make gitignore-check
```

All must exit 0. Run `make lint` as well when changing shared logic, client
compatibility, or release tooling.

---

## 7. User experience checklist

Before considering a provider ready:

- [ ] The provider name and description tell a non-expert what capability they
      are adding.
- [ ] Credential labels explain where to get the credential and what format
      is expected.
- [ ] Invalid credentials fail before writing configs.
- [ ] Unsupported clients are skipped with a clear reason instead of receiving
      malformed config.
- [ ] Dry-run output shows exact target files and redacted credentials.
- [ ] Apply output preserves rollback and verification behaviour.
- [ ] Documentation tells users whether to restart their AI client after apply.
- [ ] For no-credential providers: the TUI shows no credential form and creates
      one `Default` profile automatically.

---

## 8. Architectural context

See the technical specifications for deep dives:

- [Architecture Upgrade Plan](../specs/architecture-upgrade-plan.md)
- [Context7 Provider Spec](../specs/providers/add-context7-provider.md)
- [Playwright Provider Spec](../specs/providers/add-playwright-provider.md)
- [CodeGraph Provider Spec](../specs/providers/add-codegraph-provider.md)

---

## 9. Spec-Driven Agentic Workflow

When using an AI coding assistant to add a new provider, follow the
Spec-Driven Development (SDD) workflow:

1. **Draft the Spec**: Create `docs/specs/providers/add-<name>-provider.md`
   before writing any code. Define transport, auth method, credentials, and
   per-client adaptations. Reference the dependency graph from an existing
   spec (e.g. `add-playwright-provider.md` for no-credential stdio providers,
   `add-context7-provider.md` for remote transports).

2. **Review and approve**: Confirm the spec correctly captures all six layers
   listed in §2 before generating code. Pay particular attention to the
   manifest layer (D–F) — it is the most commonly missed for no-credential
   providers.

3. **Invoke the skill** (optional):
   ```text
   Implement the docs/specs/providers/add-<name>-provider.md spec.
   ```

4. **Validate**:
   ```bash
   make fmt && make test && make verify && make gitignore-check
   ```

5. **Generate E2E goldens** after all code is in place:
   ```bash
   go test ./tests/e2e -update -run TestProviders_Golden/<id>
   ```

6. **Commit** with a Conventional Commit subject:
   ```text
   feat: add <Name> MCP provider with full client matrix coverage
   ```
