# Dogfooding with usync — MCP Providers for Contributors

As a contributor, you can use `usync` to configure your local AI tools to help build `usync`. This guide covers both providers that require API keys (Exa, Context7, GitHub) and providers that work immediately with no credentials at all (CodeGraph, Playwright).

## Getting Started

1. **Build `usync`**
   ```bash
   make build
   ```

2. **Choose a provider**

   | Provider   | Credential needed | Notes |
   |------------|-------------------|-------|
   | CodeGraph  | None              | Indexes your local repo; available immediately after `make build` |
   | Playwright | None              | Browser automation via `npx -y @playwright/mcp`; no sign-up required |
   | Exa        | Exa API key       | Get one from [exa.ai](https://exa.ai) |
   | Context7   | Context7 API key  | Get one from [context7.com/dashboard](https://context7.com/dashboard); keys look like `ctx7sk-...` |
   | GitHub     | GitHub PAT        | Create a token at [github.com/settings/tokens](https://github.com/settings/tokens) |

3. **Try a zero-credential provider first (no API key needed)**

   The fastest way to dogfood is with CodeGraph or Playwright — no sign-up, no key file:
   ```bash
   ./bin/usync
   ```
   Select **CodeGraph** or **Playwright** in the TUI, choose your target clients (Claude Code, Cursor, Windsurf, Zed, or Gemini CLI), and confirm.

4. **Preview credential-based providers from the CLI (optional)**
   ```bash
   ./bin/usync sync --keys-file ./exa_keys.txt --dry-run
   ```
   Confirm the target paths and redacted credentials look correct before applying.

5. **Apply providers through the TUI**
   ```bash
   ./bin/usync
   ```
   Follow the prompts to add any provider to your preferred clients.

6. **Restart clients**
   Restart your AI clients so they load the new MCP servers.

## Example Prompts After Configuring

- **CodeGraph**: "Use CodeGraph to explore the call graph for `pkg/app/app.go`."
- **Playwright**: "Use Playwright to open the Bubbletea GitHub releases page and extract the latest version."
- **Context7**: "Use Context7 to look up the Bubbletea documentation for creating a custom `tea.Cmd`."
- **Exa**: "Use Exa to search for recent discussions on Go 1.23 iterator patterns."

## Contributor Loop

When adding new providers, start with [Adding an MCP Provider](adding-a-provider.md), write or review the provider spec in `docs/specs/providers/add-<name>-provider.md`, then validate with:
```bash
make fmt
make vet
make test
make verify
```

If you use Gemini CLI in this repo, the `.gemini/skills/agentic-sdd-implement` skill can follow an approved provider spec, for example:
```text
Implement docs/specs/providers/add-<name>-provider.md using the agentic-sdd-implement skill.
```

> **Tip for first-time contributors**: Start by configuring **CodeGraph** — it requires no API key and lets you immediately experience the full `usync` flow (TUI selection → config write → client reload) right after `make build`.
