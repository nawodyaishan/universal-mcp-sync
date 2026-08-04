# Add CodeGraph Provider

**Last updated:** 2026-08-04
**Status:** Implemented (v1.3.8)
**Audience:** Implementation engineers and AI agents

---

## How to use this document

This spec is the implementation record for adding the CodeGraph MCP provider.
It documents each layer touched so future no-credential stdio providers can be
added by analogy.

---

## Context

Why this change: CodeGraph provides semantic code intelligence (call graphs,
symbol maps, cross-file dependency trees) to AI agents through a fully local
MCP server. It was added so `usync` users can sync it into their AI clients
with a single command, eliminating the need for manual JSON edits.

Intended outcome:
- A working CodeGraph provider registered in the provider registry.
- CodeGraph uses local `stdio` transport via `npx -y @colbymchenry/codegraph mcp`.
- No credential prompt is shown — CodeGraph requires no API key; it reads
  the project's local `.codegraph/` index built by `codegraph init`.
- Stdio-capable clients receive the provider-generated command shape through
  the existing generic config writers.
- Antigravity CLI and Antigravity IDE are skipped (no stdio support).
- Codex CLI receives its documented TOML stdio shape.

**Official docs:** https://github.com/colbymchenry/codegraph

---

## Architecture decision

### Decision: `TransportStdio` with npm runtime and no credentials

**Why:** CodeGraph MCP is a local subprocess that communicates over stdio.
It reads a project-local `.codegraph/` index — there is no remote endpoint,
no API key, and no secret.

**Transport shape:**

```go
provider.MCPConfig{
    Type:    provider.TransportStdio,
    Command: "npx",
    Args:    []string{"-y", "@colbymchenry/codegraph", "mcp"},
    Runtime: &provider.PackageRuntime{Type: "npm"},
}
```

Note the `mcp` subcommand: it tells the CodeGraph CLI to start the MCP stdio
server rather than the interactive CLI. The `-y` flag avoids prompts when npx
downloads the package.

**Credential shape:** `RequiredCredentials()` returns nil. The existing TUI
profile fallback creates one `Default` profile for providers with no credential
specs.

**Client compatibility:** Same as Playwright — stdio-capable clients receive
the config; Antigravity CLI and IDE are skipped with clear warnings.

**Claude Code decision:** Generates CLI args equivalent to:

```bash
claude mcp add -s user codegraph -- npx -y @colbymchenry/codegraph mcp
```

---

## Dependency graph

```text
Phase A (Provider)
 ├─ T-A1  Implement pkg/provider/codegraph.go
 ├─ T-A2  Register provider and update registry tests
 └─ T-A3  Add provider unit tests (pkg/provider/codegraph_test.go)
     └─ Phase B (Manifest)
         ├─ T-B1  Add ProviderCodeGraph constant to pkg/manifest/types.go
         ├─ T-B2  Add CodeGraph ProviderMeta to pkg/manifest/providers.go
         └─ T-B3  Update pkg/manifest/manifest_test.go provider order list
             └─ Phase C (QA + docs)
                 ├─ T-C1  Add TestQACodeGraphAllClients QA scenario
                 ├─ T-C2  Generate E2E golden files (go test ./tests/e2e -update)
                 └─ T-C3  Update README provider table and mermaid diagram
```

---

## Phase A — Provider

### T-A1 — Implement `CodeGraphProvider`

**File:** `pkg/provider/codegraph.go` — new

Implement `MCPProvider` with:
- ID: `codegraph`
- Name: `CodeGraph`
- Description: semantic code intelligence for AI agents; 100% local.
- No required credentials.
- `GenerateConfig` returns stdio command `npx` and args
  `["-y", "@colbymchenry/codegraph", "mcp"]`.

### T-A2 — Register provider

**Files:** `pkg/provider/registry.go`, `pkg/provider/registry_test.go` — modified

Add `NewCodeGraphProvider()` to `DefaultRegistry()` after Terraform.
Update count/order assertions (7 → 8).

### T-A3 — Provider tests

**File:** `pkg/provider/codegraph_test.go` — new

Assert:
- `ID()` == `"codegraph"`
- `Name()` == `"CodeGraph"`
- `Description()` is non-empty
- `RequiredCredentials()` returns nil (zero-length)
- `GenerateConfig(nil)` returns `TransportStdio`, command `npx`,
  args `[-y @colbymchenry/codegraph mcp]`, no env, no headers, no URL,
  npm runtime.

---

## Phase B — Manifest

The manifest package is the source of truth for provider metadata used by the
`usync providers` CLI command, the TUI readiness panel, and the Doctor Mode
runtime check. Every provider **must** appear here — not just in the registry.

### T-B1 — Manifest ID constant

**File:** `pkg/manifest/types.go` — modified

```go
ProviderCodeGraph ProviderID = "codegraph"
```

### T-B2 — Manifest provider data

**File:** `pkg/manifest/providers.go` — modified

Add a `ProviderMeta` entry:

```go
{
    ID:         ProviderCodeGraph,
    Name:       "CodeGraph",
    DocsURL:    "https://github.com/colbymchenry/codegraph",
    RuntimeIDs: []string{"node", "npx"},
    Sources: []SourceRef{
        {URL: "https://github.com/colbymchenry/codegraph", Title: "CodeGraph",
         VerifiedAt: "2026-08-04", Confidence: "official"},
    },
},
```

**Note:** `RuntimeIDs` drives Doctor Mode runtime checks (`node` and `npx`
must be available for the provider to be usable). Even for no-credential
providers this field is mandatory when the provider depends on a runtime.

### T-B3 — Manifest test

**File:** `pkg/manifest/manifest_test.go` — modified

Add `ProviderCodeGraph` to the `want` slice in
`TestAllProvidersIncludesSupportedProviderIDsInOrder`.

---

## Phase C — QA and docs

### T-C1 — QA scenario

**File:** `pkg/app/qa_scenarios_test.go` — modified

Add `TestQACodeGraphAllClients`:
- Scaffolds all client config files.
- Calls `PrepareProvider` with a Default profile (no credential values).
- Asserts Antigravity CLI and IDE receive skip warnings.
- Asserts Claude Code CLI args == `"mcp add -s user codegraph -- npx -y @colbymchenry/codegraph mcp"`.
- Applies the plan and verifies per-client output:
  - Claude Desktop / Cursor: contain `"@colbymchenry/codegraph"` and `"command": "npx"`.
  - VS Code: no `"type": "http"` (stdio must not be coerced to HTTP).
  - Roo Code: `"type": "stdio"`.
  - OpenCode: `"type": "local"`.
  - Codex CLI TOML: `[mcp_servers.codegraph]`, `command = "npx"`, and the package name.
  - Antigravity CLI + IDE: untouched (`{}`).

### T-C2 — E2E golden files

```bash
go test ./tests/e2e -update -run TestProviders_Golden/codegraph
```

This generates `tests/e2e/testdata/provider_codegraph/*.golden` for all
12 client targets. Review diffs before committing — they are the ground
truth for regression detection.

### T-C3 — README

**File:** `README.md` — modified

- Add CodeGraph row to the Supported MCPs table.
- Add `codegraph` node and edges to the mermaid architecture diagram.

---

## Acceptance criteria

- `provider.DefaultRegistry()` includes CodeGraph at position 8.
- CodeGraph generates a no-auth stdio config with `npx -y @colbymchenry/codegraph mcp`.
- `manifest.ProviderByID("codegraph")` returns a valid `ProviderMeta` with
  `RuntimeIDs: ["node", "npx"]`.
- TUI setup produces one Default profile without showing credential fields.
- Antigravity CLI and IDE are skipped with clear unsupported-transport warnings.
- `TestQACodeGraphAllClients` passes for all stdio-capable clients.
- E2E golden files exist for all 12 client targets.
- `make fmt`, `make test`, `make verify`, and `make gitignore-check` pass.
