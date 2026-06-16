# Add Kubernetes Provider

**Last updated:** 2026-05-12
**Status:** Approved
**Audience:** Implementation engineers and AI agents

---

## How to use this document

This spec is the implementation contract for adding the Kubernetes MCP provider. Work phases in order. Run `make fmt`, `make test`, and `make gitignore-check`; run `make lint` and `make verify` because this touches provider/client compatibility.

---

## Context

Why this change: The user requested Kubernetes MCP support after Exa, Context7, GitHub, and Playwright so `usync` can cover runtime infrastructure state for platform, SRE, DevOps, and cloud-native engineers.

Intended outcome:
- A working Kubernetes provider registered in the provider registry.
- Kubernetes uses local `stdio` transport via the official npm package `kubernetes-mcp-server@latest`.
- The provider is read-only by default by always adding `--read-only`.
- No credential prompt is shown. The MCP server uses Kubernetes configuration resolution (`--kubeconfig`, default kubeconfig, or in-cluster config) on its side.
- Stdio-capable clients receive the same provider-generated command shape through existing generic config writers.
- Unsupported stdio clients are skipped with clear warnings instead of receiving malformed config.

**Official docs confirmed:** The `containers/kubernetes-mcp-server` README documents the Claude Desktop npm shape:

```json
{
  "mcpServers": {
    "kubernetes": {
      "command": "npx",
      "args": ["-y", "kubernetes-mcp-server@latest"]
    }
  }
}
```

It also documents `--read-only` as the mode that prevents write operations such as create, update, and delete on the Kubernetes cluster.

---

## Architecture decision

### Decision: `TransportStdio` with npm runtime, no credentials, and forced read-only flag

**Why:** The official server publishes an npm stdio startup path. It does not require a provider API key; access is controlled by Kubernetes credentials and RBAC outside `usync`. Because Kubernetes operations can change infrastructure state, `usync` must install the provider in read-only mode by default.

**Transport shape:**

```go
provider.MCPConfig{
	Type:    provider.TransportStdio,
	Command: "npx",
	Args:    []string{"-y", "kubernetes-mcp-server@latest", "--read-only"},
	Runtime: &provider.PackageRuntime{Type: "npm"},
}
```

**Credential shape:** `RequiredCredentials()` returns nil. The existing TUI profile fallback creates one `Default` profile for providers with no credential specs.

**Security decision:** `--read-only` is always present in generated config. `usync` will not expose a write-enabled Kubernetes provider option in this task. Users who need write mode must make a deliberate manual config change outside this default provider.

**Client compatibility decision:** Reuse the existing stdio client capability matrix. Codex CLI is supported because the TOML writer supports stdio providers. Gemini CLI and Antigravity remain skipped for stdio.

---

## Dependency graph

```text
Phase A (Provider)
 ├─ T-A1  Implement pkg/provider/kubernetes.go
 ├─ T-A2  Register provider and update registry tests
 └─ T-A3  Add provider unit tests
     └─ Phase B (QA + docs)
         ├─ T-B1  Add Kubernetes read-only QA scenario
         └─ T-B2  Update README provider matrix
```

---

## Phase A - Provider

### T-A1 - Implement `KubernetesProvider`

**Files:** `pkg/provider/kubernetes.go` - new

Implement `MCPProvider` with:
- ID: `kubernetes`
- Name: `Kubernetes`
- Description: read-only Kubernetes and OpenShift runtime state for AI agents.
- No required credentials.
- `GenerateConfig` returns stdio command `npx` and args `["-y", "kubernetes-mcp-server@latest", "--read-only"]`.

### T-A2 - Register provider

**Files:** `pkg/provider/registry.go`, `pkg/provider/registry_test.go` - modified

Add `NewKubernetesProvider()` to `DefaultRegistry()` after Playwright. Update count/order assertions.

### T-A3 - Provider tests

**Files:** `pkg/provider/kubernetes_test.go` - new

Assert metadata, zero credential specs, stdio config, npm runtime, and presence of `--read-only`.

---

## Phase B - QA and docs

### T-B1 - Kubernetes QA scenario

**Files:** `pkg/app/qa_scenarios_test.go` - modified

Add end-to-end coverage for Kubernetes across stdio-capable clients. Verify `--read-only` appears in JSON, Codex TOML, and Claude Code CLI planning. Verify unsupported stdio clients are skipped.

### T-B2 - README provider matrix

**Files:** `README.md` - modified

Add Kubernetes to the supported MCP table with read-only stdio auth shape.

---

## Acceptance criteria

- `provider.DefaultRegistry()` includes Kubernetes.
- Kubernetes generates a no-auth stdio config with `npx -y kubernetes-mcp-server@latest --read-only`.
- TUI setup can produce one default profile for Kubernetes without credential fields.
- Claude Code CLI planning includes `--read-only`.
- Codex TOML includes `--read-only`.
- Gemini CLI and Antigravity are skipped for Kubernetes with clear unsupported-transport warnings.
- `make fmt`, `make test`, `make gitignore-check`, `make lint`, and `make verify` pass.
