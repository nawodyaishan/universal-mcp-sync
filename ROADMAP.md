# Project Roadmap

The Universal MCP Sync (`usync`) project is evolving from an Exa-specific configuration utility into a robust, provider-agnostic MCP sync engine.

## Vision
To provide a single source of truth for local AI toolchain configuration, allowing engineers to manage complex MCP environments across 12+ AI clients seamlessly.

## Milestones & Status

### ✅ Phase 1: Foundation (Completed)
- Established the `MCPProvider` and `MCPConfig` interface.
- Abstracted hardcoded Exa logic.
- Implemented provider-aware config mutators for JSON/TOML.

### ✅ Phase 2: Registry & TUI Wizard (Completed)
- Introduced an explicit `ProviderRegistry`.
- Implemented dynamic credential collection using `huh`.
- Migrated Exa to the new provider-driven planning path.
- Refactored `Operation` structures to be provider-neutral.

### ✅ Phase 3: stdio Engine & High-Value Providers (Completed — v1.3.x)
- [x] **GitHub Provider**: `npx -y @modelcontextprotocol/server-github` with personal access token auth.
- [x] **stdio Transport Implementation**: Robust `command`, `args`, and `env` handling across all client configurations.
- [x] **Capability Matrix**: Compatibility rules formalised for targets that only support HTTP vs. stdio.
- [x] **Playwright Provider**: `npx -y @playwright/mcp` — no API key required.
- [x] **Kubernetes Provider**: `npx -y @modelcontextprotocol/server-kubernetes` with kubeconfig path support.
- [x] **Terraform Provider**: `npx -y @modelcontextprotocol/server-terraform` with workspace configuration.
- [x] **CodeGraph Provider**: Local binary (`codegraph serve`) — no API key required, zero-credential stdio server.

### 🗺️ Phase 4: Safety & Fallbacks
- [ ] **Intelligent Validation**: Implement automatic fallbacks for clients that cannot support a selected provider.
- [ ] **Robust Error Remediation**: Context-aware error reporting (permissions, missing binaries, etc.).

### 🚀 Phase 5: Infinite Scale (Plugin System)
- [ ] **Dynamic Registry**: Fetch schemas directly from `registry.modelcontextprotocol.io`.
- [ ] **Custom Plugin Support**: Implement `hashicorp/go-plugin` for bespoke auth flows and complex setup logic.

---

## Strategic Direction
- **Provider-Agnostic Core**: Ensure the engine remains decoupled from specific MCP implementations.
- **Developer Experience First**: Maintain a "Zero-Leak" policy for secrets in the TUI/logs and ensure atomic, rollback-safe operations.
- **Standardized Verification**: The "Golden Path" test suite now covers both HTTP/SSE and stdio transports; focus is expanding scenario coverage across the full provider/client matrix.
- **Zero-Credential Providers**: Prioritise providers (like CodeGraph and Playwright) that need no API key, enabling contributors to dogfood `usync` immediately after `make build`.
