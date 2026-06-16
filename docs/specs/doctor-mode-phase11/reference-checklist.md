# Phase 11 Reference Verification Checklist

**Purpose:** Track currency of key claims usync makes about each client's config format.  
**Last reviewed:** 2026-05-23  
**How to use:** Verify each claim against the linked source before shipping. Mark pass/fail and update date.

---

## VS Code

| Claim | Source | Last checked | Status |
|---|---|---|---|
| MCP servers go in `mcp.json` under `"servers"` root | https://code.visualstudio.com/docs/copilot/reference/mcp-configuration | 2026-05-23 | ✅ pass |
| `"type": "http"` required for HTTP servers | Same | 2026-05-23 | ✅ pass |
| `"inputs"` array at JSON root for `${input:id}` secret injection | Same | 2026-05-23 | ✅ pass |
| `sandboxEnabled` and `sandbox` fields are preserved by usync (unknown-key safe) | Same | 2026-05-23 | ✅ pass |
| `chat.mcp.discovery.enabled` setting for auto-discovery | https://code.visualstudio.com/docs/copilot/customization/mcp-servers | 2026-05-23 | ✅ pass |

---

## Claude Code

| Claim | Source | Last checked | Status |
|---|---|---|---|
| Scope names: `local` (default), `project` (shared), `user` (cross-project) | https://code.claude.com/docs/en/mcp | 2026-05-23 | ✅ pass |
| `${VAR:-default}` expansion syntax supported in `.mcp.json` | Same | 2026-05-23 | ✅ pass |
| User-scope stored in `~/.claude.json`; project-scope in `.mcp.json` | Same | 2026-05-23 | ✅ pass |
| Managed config deployed at `/etc/claude-code/managed-settings.json` | https://code.claude.com/docs/en/configuration | 2026-05-23 | ✅ pass |
| `managed-mcp.json` enterprise feature for org-wide MCP policy | Same | 2026-05-23 | ✅ pass |

---

## Codex CLI

| Claim | Source | Last checked | Status |
|---|---|---|---|
| User-scope config: `~/.codex/config.toml` | https://developers.openai.com/codex/config-basic | 2026-05-23 | ✅ pass |
| Per-project `.codex/config.toml` is trust-gated; usync skips it | https://openai-codex.mintlify.app/configuration/mcp-servers | 2026-05-23 | ✅ pass |
| `bearer_token_env_var` field for HTTP credential | https://developers.openai.com/codex/config-advanced | 2026-05-23 | ✅ pass |
| `codex mcp add --url <url> --bearer-token-env-var <var> <name>` CLI shape | Same | 2026-05-23 | ✅ pass |
| `mcp_oauth_credentials_store` supports `"auto"`, `"file"`, `"keyring"` | Same | 2026-05-23 | ✅ pass |

---

## Antigravity CLI

| Claim | Source | Confidence | Last checked | Status |
|---|---|---|---|---|
| Canonical config path: `~/.gemini/antigravity-cli/mcp_config.json` | https://antigravity.google/docs/gcli-migration | official | 2026-05-23 | ✅ pass |
| Remote server URL field: `serverUrl` (not `url`) | Same | official | 2026-05-23 | ✅ pass |
| CLI binary name: `agy` | Same | official | 2026-05-23 | ✅ pass |
| Post-migration fallback: `~/.gemini/config/mcp_config.json` | https://github.com/google-antigravity/antigravity-cli/issues/60 | empirical | 2026-05-23 | ✅ pass |
| Old legacy path: `~/.gemini/antigravity/mcp_config.json` | Same issue, community observation | empirical | 2026-05-23 | ✅ pass |
| Workspace path: `.agents/mcp_config.json` | https://antigravity.google/docs/gcli-migration | official | 2026-05-23 | ✅ pass |

---

## Antigravity IDE

| Claim | Source | Confidence | Last checked | Status |
|---|---|---|---|---|
| Canonical config path: `~/.gemini/config/mcp_config.json` | https://antigravity.google/docs/mcp | official | 2026-05-23 | ✅ pass |
| Remote server URL field: `serverUrl` | Same | official | 2026-05-23 | ✅ pass |
| Config managed via IDE Settings → Customizations → Open MCP Config | Same | official | 2026-05-23 | ✅ pass |

---

## Zed

| Claim | Source | Last checked | Status |
|---|---|---|---|
| MCP servers under `"context_servers"` root in `~/.config/zed/settings.json` | https://zed.dev/docs/ai/mcp | 2026-05-23 | ✅ pass |
| Stdio servers: `command`/`args`/`env` fields | Same | 2026-05-23 | ✅ pass |
| Remote servers: `url`/`headers` fields | Same | 2026-05-23 | ✅ pass |
| OAuth flow supported for remote servers without `Authorization` header | Same | 2026-05-23 | ✅ pass |

---

## Verification Procedure

1. Open the source URL.
2. Search for the claim (e.g. field name, path, CLI syntax).
3. If confirmed: mark ✅ pass, update date.
4. If changed: update the claim in the manifest or code, mark ⚠️ updated, open a PR.
5. If unverifiable: mark ❓ unknown, add a note.
