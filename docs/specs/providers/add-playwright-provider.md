# Add Playwright Provider

**Last updated:** 2026-05-12
**Status:** Approved
**Audience:** Implementation engineers and AI agents

---

## How to use this document

This spec is the implementation contract for adding the Playwright MCP provider. Work phases in order. Run `make fmt`, `make test`, and `make gitignore-check`; run `make lint` when available.

---

## Context

Why this change: The user requested support for the official Playwright MCP server in `usync`, following the Spec-Driven Development router and the in-repo `add-provider` skill.

Intended outcome:
- A working Playwright provider registered in the provider registry.
- Playwright uses local `stdio` transport via the official npm package `@playwright/mcp@latest`.
- No credential prompt is shown because Playwright MCP does not require an API key for the standard local server.
- Stdio-capable clients receive the same provider-generated command shape through the existing generic config writers.
- Codex CLI receives its documented TOML stdio shape.

**Official docs confirmed:** The Microsoft Playwright MCP README documents the standard config as `command: "npx"` with args `["@playwright/mcp@latest"]`, and separately documents Codex TOML as:

```toml
[mcp_servers.playwright]
command = "npx"
args = ["@playwright/mcp@latest"]
```

---

## Architecture decision

### Decision: `TransportStdio` with npm runtime and no credentials

**Why:** Playwright MCP publishes a local stdio configuration as its standard install path across MCP clients. It does not require API-key, URL-query, header, or environment-variable authentication for the base server.

**Transport shape:**

```go
provider.MCPConfig{
	Type:    provider.TransportStdio,
	Command: "npx",
	Args:    []string{"@playwright/mcp@latest"},
	Runtime: &provider.PackageRuntime{Type: "npm"},
}
```

**Credential shape:** `RequiredCredentials()` returns nil. The existing TUI profile fallback creates one `Default` profile for providers with no credential specs.

**Client compatibility decision:** Enable Codex CLI stdio TOML writing because Playwright's official docs provide a native Codex stdio shape and `usync` already has a Codex TOML writer. Gemini CLI and Antigravity remain skipped for stdio until their file writers and client capability rules are explicitly updated for local subprocess servers.

**Claude Code decision:** For stdio providers, generate CLI args equivalent to:

```bash
claude mcp add -s user playwright -- npx @playwright/mcp@latest
```

Remote providers keep the existing `--transport <type> <name> <url>` CLI shape.

---

## Dependency graph

```text
Phase A (Provider)
 ├─ T-A1  Implement pkg/provider/playwright.go
 ├─ T-A2  Register provider and update registry tests
 └─ T-A3  Add provider unit tests
     └─ Phase B (Client persistence)
         ├─ T-B1  Support stdio in Codex TOML writer
         ├─ T-B2  Mark Codex CLI stdio-capable
         ├─ T-B3  Verify generic Codex TOML provider entries
         └─ T-B4  Build correct Claude Code CLI args for stdio
             └─ Phase C (QA + docs)
                 ├─ T-C1  Add Playwright QA scenario
                 └─ T-C2  Update README provider matrix
```

---

## Phase A - Provider

### T-A1 - Implement `PlaywrightProvider`

**Files:** `pkg/provider/playwright.go` - new

Implement `MCPProvider` with:
- ID: `playwright`
- Name: `Playwright`
- Description: browser automation for AI agents through structured accessibility snapshots.
- No required credentials.
- `GenerateConfig` returns stdio command `npx` and args `["@playwright/mcp@latest"]`.

### T-A2 - Register provider

**Files:** `pkg/provider/registry.go`, `pkg/provider/registry_test.go` - modified

Add `NewPlaywrightProvider()` to `DefaultRegistry()` after Tavily. Update count/order assertions.

### T-A3 - Provider tests

**Files:** `pkg/provider/playwright_test.go` - new

Assert metadata, zero credential specs, stdio config, npm runtime, and no env/header secrets.

---

## Phase B - Client persistence

### T-B1 - Codex TOML stdio support

**Files:** `pkg/config/toml_update.go`, `pkg/config/toml_update_test.go` - modified

Teach `UpdateCodexTOML` to write either:

```toml
[mcp_servers.<id>]
url = "..."
http_headers = { ... }
```

or:

```toml
[mcp_servers.<id>]
command = "npx"
args = ["@playwright/mcp@latest"]
```

When `Env` is present, write an `env = { ... }` inline table with sorted keys.

### T-B2 - Codex capability

**Files:** `pkg/client/capabilities.go`, `pkg/client/adapter_test.go` - modified

Set `AppCodexCLI` to `Stdio: true` and update capability tests.

### T-B3 - Generic Codex TOML verification

**Files:** `pkg/verify/verify.go`, `pkg/verify/verify_test.go` - modified

Allow `VerifyProviderFile` for non-special providers to validate Codex TOML entries by checking command for stdio and URL for remote transports.

### T-B4 - Claude Code stdio CLI args

**Files:** `pkg/app/app.go`, `pkg/app/app_test.go` - modified

Build `claude mcp add` args from transport:
- stdio: `mcp add -s user <providerID> -- <command> <args...>`
- remote: preserve existing URL-based transport shape.

---

## Phase C - QA and docs

### T-C1 - Playwright QA scenario

**Files:** `pkg/app/qa_scenarios_test.go` - modified

Add end-to-end coverage for Playwright across stdio-capable clients, including Codex TOML and Claude Code CLI planning when the fake `claude` binary is available. Verify no unsupported clients receive malformed config.

### T-C2 - README provider matrix

**Files:** `README.md` - modified

Add Playwright to the supported MCP table.

---

## Acceptance criteria

- `provider.DefaultRegistry()` includes Playwright.
- Playwright generates a no-auth stdio config with `npx @playwright/mcp@latest`.
- TUI setup can produce one default profile for Playwright without credential fields.
- Codex TOML can persist stdio providers.
- Claude Code CLI planning uses command/args for stdio providers.
- Gemini CLI and Antigravity are skipped for Playwright with clear unsupported-transport warnings.
- `make fmt`, `make test`, and `make gitignore-check` pass.
