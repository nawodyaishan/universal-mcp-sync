# Adding an MCP Provider

This guide walks through adding a new MCP server provider to `usync` without hard-coding provider-specific behavior into the TUI or apply flow.

## 1. Provider Shape
Before you begin, determine the shape of the provider:
- **Transport**: `stdio`, `http`, `sse`, or `streamable-http`.
- **Runtime**: For `stdio`, decide whether the command is installed by `npm`, `pypi`, `oci`, `mcpb`, or manually.
- **Authentication**: URL query parameters like Exa, HTTP headers like Context7, or environment variables like GitHub.
- **Credentials**: Single credential, multiple independent profiles, or a multi-key paste field.
- **Client support**: Which clients support the transport natively, need a bridge, or must be skipped?

Do this from the provider and client documentation. Do not infer a transport shape from one client if the provider publishes an official configuration format.

## 2. Step-by-step (Context7 Example)
Here is how we added Context7 (Remote StreamableHTTP, Header auth, single key):

1. **Scaffold helpers**: Create `pkg/context7/keys.go` and `pkg/context7/url.go`. Add prefix validation and key redaction rules.
2. **Implement Provider**: Create `pkg/provider/context7.go` implementing the `MCPProvider` interface. Use `BridgeOverride` only when a provider needs a bespoke bridge that the client matrix cannot express.
3. **Register**: Add it to `DefaultRegistry()` in `pkg/provider/registry.go`.
4. **Adaptation**: Add per-client bridges or header augmentation in `pkg/client/adapter.go` and transport support in `pkg/client/capabilities.go`.
5. **Persistence**: Update `pkg/config/` only if an existing JSON/TOML writer cannot persist the provider's transport shape safely.
6. **QA**: Write tests in `pkg/app/qa_scenarios_test.go` to ensure every supported client receives the correct configuration structure or a clear skip reason.

## 3. Required Tests

Add or update tests for each layer you touch:
- **Provider**: Registry inclusion, credential validation, generated `MCPConfig`, and optional `MultiValueParser` behavior.
- **Redaction**: Raw credentials, secret URLs, headers, environment values, and bridge args must not appear in user-facing output.
- **Client adaptation**: Native transport passthrough, bridge conversion, injected headers, and unsupported transport skips.
- **Config writers**: JSON/TOML root keys, special URL field names, headers, env vars, and file permissions.
- **QA scenarios**: End-to-end plans for representative clients, including at least one native remote target and one bridge target when applicable.

## 4. URL-auth Variant (Exa)
For URL-based auth, see `pkg/provider/exa.go`. It appends the API key directly to the URL query string.

Because the credential is embedded in the URL, tests must assert both the generated URL and the redacted output path. Never print the full URL when it contains secrets.

## 5. Multi-key Paste Variant (Exa)
If the provider accepts multiple credentials that a user might paste together, implement the `MultiValueParser` interface on the provider.

The TUI should create one `CredentialProfile` per parsed credential and show only a redacted, human-distinguishable label for each profile.

## 6. Stdio Variant (GitHub)
For stdio providers, return `Command`, `Args`, `Env`, and a non-nil `Runtime` when the package source is known.

Keep secrets in `Env` whenever the upstream server supports it. Avoid putting tokens in `Args`; command arguments are easier to leak through process listings, logs, and failed command reports.

## 7. User Experience Checklist

Before considering a provider ready:
- The provider name and description tell a non-expert what capability they are adding.
- Credential labels explain where to get the credential and what format is expected.
- Invalid credentials fail before writing configs.
- Unsupported clients are skipped with a clear reason instead of receiving malformed config.
- Dry-run output shows exact target files and redacted credentials.
- Apply output preserves rollback and verification behavior.
- Documentation tells users whether to restart their AI client after apply.

## 8. Architectural Context
See the technical specifications for deep dives:
- [Architecture Upgrade Plan](../specs/architecture-upgrade-plan.md)
- [Context7 Provider Spec](../specs/providers/add-context7-provider.md)

## 9. Spec-Driven Agentic Workflow

When using an AI coding assistant to add a new provider, we follow a strict Spec-Driven Development (SDD) workflow to ensure predictable, verifiable code generation:

1. **Draft the Spec**: Do not prompt the agent to "just build it." First, use the `agentic-sdd-spec` skill to draft a clear, scoped specification document (e.g., `docs/specs/providers/add-<name>-provider.md`) detailing the provider's transport, auth method, credentials, and any per-client adaptations.
2. **Review and Approve**: Ensure the specification correctly captures the architectural constraints before any code is generated. You can use the `agentic-sdd-architecture-review` skill to assist. No implementation should occur without an approved spec.
3. **Invoke the Skill**: Trigger the `.gemini/skills/agentic-sdd-implement` skill, explicitly binding it to your approved spec document:
   ```text
   Implement the docs/specs/add-<name>-provider.md spec using the agentic-sdd-implement skill.
   ```
4. **Execution and Validation**: The agent will follow the procedure defined in the skill, acting within the bounds of the spec. Alternatively, follow the generic SDD pipeline by triggering the `.gemini/skills/agentic-sdd-router` skill.

Validate the result with:
```bash
make fmt
make test
make lint
```

Run `make dry-run KEYS_FILE=...` for Exa-compatible non-interactive flows, or use the TUI for provider-neutral manual validation.
