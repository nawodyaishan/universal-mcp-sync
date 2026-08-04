# Contributing to Universal MCP Sync

Thank you for your interest in contributing. This project aims to make MCP configuration repeatable, reviewable, and safe across local AI assistants.

## Technical Vision
Universal MCP Sync started as an Exa-focused utility and now uses a **provider-based architecture**. New MCP servers should fit the generic provider/client pipeline instead of adding provider-specific branches to the TUI or apply flow.

## How to Contribute

### 1. Adding a New MCP Provider
If you want to add support for a new MCP server, we strictly follow **Spec-Driven Development (SDD)**.
1. **Draft a Spec**: Start by creating a specification document in `docs/specs/providers/add-<name>-provider.md` rather than writing code directly.
2. **Reference the Guide**: Use the [Adding a Provider Guide](docs/contributors/adding-a-provider.md) to understand the full 10-layer checklist:
   - `pkg/provider/<name>.go` — implement `MCPProvider` (`ID`, `Name`, `Description`, `RequiredCredentials`, `GenerateConfig`).
   - `pkg/provider/registry.go` — register in `DefaultRegistry()`.
   - `pkg/manifest/types.go` — add manifest constant.
   - `pkg/manifest/providers.go` — add display metadata entry.
   - `pkg/manifest/manifest_test.go` — assert new constant and entry presence.
   - `pkg/provider/<name>_test.go` — dedicated test file (registry inclusion, credential validation, config generation, redaction).
   - `pkg/app/app.go` (`configForTarget`) — add client compatibility rules when needed.
   - `pkg/app/qa_scenarios_test.go` — add QA scenario fixtures.
   - E2E golden files — generate or update for the new provider.
   - `make verify` — confirm the full local CI guard passes.
3. **Execute**: Once the spec is reviewed and approved, invoke the `.gemini/skills/agentic-sdd-implement` skill using the spec as the strict implementation contract. For a full SDD workflow, you can invoke the `agentic-sdd-router` from the `.gemini/skills/` directory.

### 2. Adding a New AI Client
If a new AI assistant with local MCP support is released:
1. **Add the app ID and path** in `pkg/config/paths.go`.
2. **Declare transport support** in `pkg/client/capabilities.go`.
3. **Add adaptation rules** in `pkg/client/adapter.go` when the client needs a bridge, headers, or transport-specific fields.
4. **Update config writers** in `pkg/config/` only when the client persists a new JSON/TOML shape.
5. **Add QA coverage** in `pkg/app/qa_scenarios_test.go` so existing providers keep working across the matrix.

## Development Workflow

### Requirements
- Go 1.23+
- macOS (for config path detection)
- `golangci-lint` for `make lint`
- [Lefthook](https://github.com/evilmartians/lefthook) (optional but recommended)

### First-Time Setup
```bash
go mod download
make build
./bin/usync --help
```

### Commands
```bash
make tidy          # sync module dependencies
make tidy-check    # verify go.mod and go.sum are already tidy
make mod-verify    # verify downloaded module checksums
make fmt           # format Go packages
make vet           # run go vet
make lint          # run golangci-lint
make test          # run all tests
make build         # build ./bin/usync
make dry-run KEYS_FILE=~/Downloads/exa_keys.txt
```

## Testing Standards
Reliability is our primary feature. We never ship a feature without verification:
- **Unit tests**: Cover credential parsing, generated `MCPConfig`, transport support, and config mutation.
- **Manifest layer tests**: Assert that every new manifest constant and display metadata entry is present and in the correct order in `pkg/manifest/manifest_test.go`.
- **Dedicated provider test file**: Each provider must have its own `pkg/provider/<name>_test.go` covering registry inclusion, credential validation, config generation, and redaction.
- **Scenario tests**: Update `pkg/app/qa_scenarios_test.go` so each provider/client shape remains compatible.
- **E2E golden files**: Generate or update golden comparison files for any new or changed provider so the golden gate passes.
- **Redaction tests**: Ensure UI output, logs, snapshots, and failures never leak raw credentials, secret URLs, or generated CLI args containing secrets.
- **Rollback tests**: Preserve backup and rollback behavior when touching apply logic or config writers.

Before opening a PR, run:
```bash
make fmt
make vet
make test
make build
make verify
make gitignore-check
```

Run `make lint` in addition when changing shared logic, release tooling, or provider/client compatibility.

## Pull Request Process
1. Create a branch with a Conventional Commit-style topic, for example `feat/context7-provider` or `fix/zed-headers`.
2. Keep provider, client, config writer, and TUI changes separated when practical.
3. Include or update tests for the behavior you changed.
4. Describe the problem, the change, and the verification commands you ran.
5. Include terminal output or screenshots only when TUI behavior changed.
